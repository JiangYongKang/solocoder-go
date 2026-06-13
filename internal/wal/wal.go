package wal

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var (
	ErrClosed         = errors.New("wal is closed")
	ErrInvalidOffset  = errors.New("invalid offset")
	ErrCorruptedEntry = errors.New("corrupted log entry")
	ErrNotFound       = errors.New("entry not found")
	ErrEmptyData      = errors.New("empty data")
	ErrInvalidConfig  = errors.New("invalid wal config")
)

type OpType byte

const (
	OpPut OpType = iota + 1
	OpDelete
	OpCheckpoint
)

func (t OpType) String() string {
	switch t {
	case OpPut:
		return "PUT"
	case OpDelete:
		return "DELETE"
	case OpCheckpoint:
		return "CHECKPOINT"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", t)
	}
}

const (
	entryHeaderSize = 19
	magicNumber     = uint16(0x5741)
)

type Entry struct {
	Offset   int64
	Type     OpType
	Data     []byte
}

type CorruptedEntryWarning struct {
	SegmentID int
	Position  int64
	Reason    string
}

func (w *CorruptedEntryWarning) String() string {
	return fmt.Sprintf("corrupted entry in segment %d at position %d: %s", w.SegmentID, w.Position, w.Reason)
}

type Config struct {
	Dir          string
	MaxSegmentSize int64
	FSyncOnWrite bool
}

func DefaultConfig() *Config {
	return &Config{
		Dir:            "./wal",
		MaxSegmentSize: 64 * 1024 * 1024,
		FSyncOnWrite:   false,
	}
}

type segment struct {
	id        int
	path      string
	file      *os.File
	size      int64
	startOffset int64
	endOffset   int64
}

type WAL struct {
	mu         sync.RWMutex
	config     *Config
	segments   []*segment
	activeSeg  *segment
	nextOffset int64
	closed     bool
}

func encodeEntry(e *Entry) ([]byte, error) {
	dataLen := len(e.Data)
	totalLen := entryHeaderSize + dataLen
	buf := make([]byte, totalLen)

	binary.BigEndian.PutUint16(buf[0:2], magicNumber)

	binary.BigEndian.PutUint64(buf[6:14], uint64(e.Offset))
	buf[14] = byte(e.Type)
	binary.BigEndian.PutUint32(buf[15:19], uint32(dataLen))

	copy(buf[19:], e.Data)

	checksumPayload := make([]byte, 13+dataLen)
	copy(checksumPayload[0:8], buf[6:14])
	checksumPayload[8] = buf[14]
	copy(checksumPayload[9:13], buf[15:19])
	copy(checksumPayload[13:], e.Data)

	checksum := crc32.ChecksumIEEE(checksumPayload)
	binary.BigEndian.PutUint32(buf[2:6], checksum)

	return buf, nil
}

func decodeEntry(data []byte) (*Entry, int, error) {
	if len(data) < entryHeaderSize {
		return nil, 0, fmt.Errorf("data too short: %d bytes, need %d", len(data), entryHeaderSize)
	}

	magic := binary.BigEndian.Uint16(data[0:2])
	if magic != magicNumber {
		return nil, 0, fmt.Errorf("invalid magic number: 0x%x", magic)
	}

	checksum := binary.BigEndian.Uint32(data[2:6])

	dataLen := binary.BigEndian.Uint32(data[15:19])
	totalLen := entryHeaderSize + int(dataLen)

	if len(data) < totalLen {
		return nil, 0, io.ErrUnexpectedEOF
	}

	checksumPayload := make([]byte, 13+int(dataLen))
	copy(checksumPayload[0:8], data[6:14])
	checksumPayload[8] = data[14]
	copy(checksumPayload[9:13], data[15:19])
	copy(checksumPayload[13:], data[19:totalLen])

	actualChecksum := crc32.ChecksumIEEE(checksumPayload)
	if actualChecksum != checksum {
		return nil, 0, fmt.Errorf("checksum mismatch: expected 0x%x, got 0x%x", checksum, actualChecksum)
	}

	offset := int64(binary.BigEndian.Uint64(data[6:14]))
	opType := OpType(data[14])
	entryData := make([]byte, dataLen)
	copy(entryData, data[19:totalLen])

	return &Entry{
		Offset: offset,
		Type:   opType,
		Data:   entryData,
	}, totalLen, nil
}

func segmentFileName(id int) string {
	return fmt.Sprintf("wal_%08d.log", id)
}

func parseSegmentID(filename string) (int, bool) {
	if !strings.HasPrefix(filename, "wal_") || !strings.HasSuffix(filename, ".log") {
		return 0, false
	}
	numStr := strings.TrimPrefix(strings.TrimSuffix(filename, ".log"), "wal_")
	id, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, false
	}
	return id, true
}

func New(config *Config) (*WAL, error) {
	if config == nil {
		config = DefaultConfig()
	}
	if config.Dir == "" {
		return nil, ErrInvalidConfig
	}
	if config.MaxSegmentSize <= 0 {
		config.MaxSegmentSize = DefaultConfig().MaxSegmentSize
	}

	if err := os.MkdirAll(config.Dir, 0755); err != nil {
		return nil, fmt.Errorf("create wal dir failed: %w", err)
	}

	wal := &WAL{
		config:     config,
		segments:   make([]*segment, 0),
		nextOffset: 0,
	}

	if err := wal.loadExistingSegments(); err != nil {
		return nil, err
	}

	if len(wal.segments) == 0 {
		if err := wal.createSegment(1); err != nil {
			return nil, err
		}
	} else {
		lastSeg := wal.segments[len(wal.segments)-1]
		if err := lastSeg.file.Close(); err != nil {
			return nil, fmt.Errorf("close last segment failed: %w", err)
		}
		f, err := os.OpenFile(lastSeg.path, os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("reopen last segment with append failed: %w", err)
		}
		lastSeg.file = f
		wal.activeSeg = lastSeg
		wal.nextOffset = wal.activeSeg.endOffset
		if wal.nextOffset >= 0 {
			wal.nextOffset++
		} else {
			wal.nextOffset = 0
		}
	}

	return wal, nil
}

func (w *WAL) loadExistingSegments() error {
	entries, err := os.ReadDir(w.config.Dir)
	if err != nil {
		return fmt.Errorf("read wal dir failed: %w", err)
	}

	var segIDs []int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		id, ok := parseSegmentID(e.Name())
		if ok {
			segIDs = append(segIDs, id)
		}
	}

	sort.Ints(segIDs)

	for _, id := range segIDs {
		seg, err := w.openSegment(id)
		if err != nil {
			return err
		}
		w.segments = append(w.segments, seg)
	}

	return nil
}

func (w *WAL) openSegment(id int) (*segment, error) {
	path := filepath.Join(w.config.Dir, segmentFileName(id))
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open segment %d failed: %w", id, err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stat segment %d failed: %w", id, err)
	}

	seg := &segment{
		id:          id,
		path:        path,
		file:        f,
		size:        info.Size(),
		startOffset: -1,
		endOffset:   -1,
	}

	if seg.size > 0 {
		if err := w.scanSegmentOffsets(seg); err != nil {
			f.Close()
			return nil, err
		}
	}

	return seg, nil
}

func (w *WAL) scanSegmentOffsets(seg *segment) error {
	_, err := seg.file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	rawData, err := io.ReadAll(seg.file)
	if err != nil {
		return err
	}

	firstOffset := int64(-1)
	lastOffset := int64(-1)
	pos := 0

	for pos < len(rawData) {
		remaining := rawData[pos:]
		if len(remaining) < entryHeaderSize {
			break
		}

		magic := binary.BigEndian.Uint16(remaining[0:2])
		if magic != magicNumber {
			pos++
			continue
		}

		dataLen := binary.BigEndian.Uint32(remaining[15:19])
		totalLen := entryHeaderSize + int(dataLen)
		if len(remaining) < totalLen {
			break
		}

		checksum := binary.BigEndian.Uint32(remaining[2:6])
		checksumPayload := make([]byte, 13+int(dataLen))
		copy(checksumPayload[0:8], remaining[6:14])
		checksumPayload[8] = remaining[14]
		copy(checksumPayload[9:13], remaining[15:19])
		copy(checksumPayload[13:], remaining[19:totalLen])
		actualChecksum := crc32.ChecksumIEEE(checksumPayload)

		if actualChecksum != checksum {
			pos++
			continue
		}

		offset := int64(binary.BigEndian.Uint64(remaining[6:14]))
		if firstOffset == -1 {
			firstOffset = offset
		}
		lastOffset = offset
		pos += totalLen
	}

	seg.startOffset = firstOffset
	seg.endOffset = lastOffset
	return nil
}

func (w *WAL) createSegment(id int) error {
	path := filepath.Join(w.config.Dir, segmentFileName(id))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("create segment %d failed: %w", id, err)
	}

	seg := &segment{
		id:        id,
		path:      path,
		file:      f,
		size:      0,
		startOffset: -1,
		endOffset:   -1,
	}

	w.segments = append(w.segments, seg)
	w.activeSeg = seg
	return nil
}

func (w *WAL) Append(opType OpType, data []byte) (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return -1, ErrClosed
	}

	if opType != OpCheckpoint && len(data) == 0 {
		return -1, ErrEmptyData
	}

	offset := w.nextOffset
	entry := &Entry{
		Offset: offset,
		Type:   opType,
		Data:   data,
	}

	encoded, err := encodeEntry(entry)
	if err != nil {
		return -1, fmt.Errorf("encode entry failed: %w", err)
	}

	if w.config.MaxSegmentSize > 0 && w.activeSeg.size+int64(len(encoded)) > w.config.MaxSegmentSize {
		if err := w.rotateSegment(); err != nil {
			return -1, err
		}
	}

	n, err := w.activeSeg.file.Write(encoded)
	if err != nil {
		return -1, fmt.Errorf("write entry failed: %w", err)
	}

	if w.config.FSyncOnWrite {
		if err := w.activeSeg.file.Sync(); err != nil {
			return -1, fmt.Errorf("fsync failed: %w", err)
		}
	}

	w.activeSeg.size += int64(n)
	if w.activeSeg.startOffset == -1 {
		w.activeSeg.startOffset = offset
	}
	w.activeSeg.endOffset = offset
	w.nextOffset = offset + 1

	return offset, nil
}

func (w *WAL) rotateSegment() error {
	if err := w.activeSeg.file.Sync(); err != nil {
		return fmt.Errorf("sync active segment failed: %w", err)
	}

	newID := w.activeSeg.id + 1
	return w.createSegment(newID)
}

func (w *WAL) ReadFrom(startOffset int64) ([]*Entry, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.closed {
		return nil, ErrClosed
	}
	if startOffset < 0 {
		return nil, ErrInvalidOffset
	}

	var result []*Entry
	var startSeg *segment

	for _, seg := range w.segments {
		if seg.startOffset == -1 {
			continue
		}
		if startOffset >= seg.startOffset && (seg.endOffset == -1 || startOffset <= seg.endOffset) {
			startSeg = seg
			break
		}
		if startOffset < seg.startOffset {
			startSeg = seg
			break
		}
	}

	if startSeg == nil {
		return result, nil
	}

	startIdx := -1
	for i, seg := range w.segments {
		if seg.id == startSeg.id {
			startIdx = i
			break
		}
	}

	for i := startIdx; i < len(w.segments); i++ {
		seg := w.segments[i]
		entries, err := w.readSegmentEntries(seg, startOffset)
		if err != nil {
			return result, err
		}
		result = append(result, entries...)
	}

	return result, nil
}

func (w *WAL) readSegmentEntries(seg *segment, minOffset int64) ([]*Entry, error) {
	_, err := seg.file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	var entries []*Entry
	rawData, err := io.ReadAll(seg.file)
	if err != nil {
		return nil, err
	}

	pos := 0
	for pos < len(rawData) {
		entry, consumed, err := decodeEntry(rawData[pos:])
		if err != nil {
			pos++
			continue
		}
		if entry.Offset >= minOffset {
			entries = append(entries, entry)
		}
		pos += consumed
	}

	return entries, nil
}

type RecoverCallback func(entry *Entry) error

func (w *WAL) RecoverFrom(startOffset int64, cb RecoverCallback) ([]*CorruptedEntryWarning, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.closed {
		return nil, ErrClosed
	}
	if startOffset < 0 {
		return nil, ErrInvalidOffset
	}
	if cb == nil {
		return nil, errors.New("callback is nil")
	}

	var warnings []*CorruptedEntryWarning

	var startSeg *segment
	for _, seg := range w.segments {
		if seg.startOffset == -1 {
			continue
		}
		if startOffset <= seg.endOffset || seg.endOffset == -1 {
			if startSeg == nil {
				startSeg = seg
			}
			break
		}
		if startOffset < seg.startOffset {
			startSeg = seg
			break
		}
	}

	if startSeg == nil {
		return warnings, nil
	}

	startIdx := -1
	for i, seg := range w.segments {
		if seg.id == startSeg.id {
			startIdx = i
			break
		}
	}

	for i := startIdx; i < len(w.segments); i++ {
		seg := w.segments[i]
		segWarnings, err := w.recoverSegment(seg, startOffset, cb)
		warnings = append(warnings, segWarnings...)
		if err != nil {
			return warnings, err
		}
	}

	return warnings, nil
}

func (w *WAL) recoverSegment(seg *segment, minOffset int64, cb RecoverCallback) ([]*CorruptedEntryWarning, error) {
	_, err := seg.file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	var warnings []*CorruptedEntryWarning
	rawData, err := io.ReadAll(seg.file)
	if err != nil {
		return nil, err
	}

	pos := 0
	for pos < len(rawData) {
		entry, consumed, err := decodeEntry(rawData[pos:])
		if err != nil {
			warnings = append(warnings, &CorruptedEntryWarning{
				SegmentID: seg.id,
				Position:  int64(pos),
				Reason:    err.Error(),
			})
			pos++
			continue
		}
		if entry.Offset >= minOffset {
			if err := cb(entry); err != nil {
				return warnings, err
			}
		}
		pos += consumed
	}

	return warnings, nil
}

func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	if w.activeSeg != nil {
		return w.activeSeg.file.Sync()
	}
	return nil
}

func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	var firstErr error
	for _, seg := range w.segments {
		if err := seg.file.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func (w *WAL) SegmentCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.segments)
}

func (w *WAL) LastOffset() int64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.nextOffset == 0 {
		return -1
	}
	return w.nextOffset - 1
}

func (w *WAL) ActiveSegmentSize() int64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.activeSeg == nil {
		return 0
	}
	return w.activeSeg.size
}
