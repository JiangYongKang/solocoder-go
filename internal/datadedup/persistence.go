package datadedup

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const (
	magicNumber           uint32 = 0x44445550
	version               uint16 = 1
	entryTypeFP           byte   = 1
	entryTypeCS           byte   = 2
	entryTypeEntriesDigest byte  = 3
	headerSize                   = 16
)

type persistIndex struct {
	mu sync.Mutex
}

func NewPersistIndex() PersistIndex {
	return &persistIndex{}
}

type persistHeader struct {
	Magic    uint32
	Version  uint16
	Reserved uint16
	Count    uint64
}

type persistEntry struct {
	Type      byte
	FP        Fingerprint
	Timestamp int64
}

func (p *persistIndex) Save(index FingerprintIndex, path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.saveLocked(index, path)
}

func (p *persistIndex) saveLocked(index FingerprintIndex, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	writer := bufio.NewWriter(f)

	header := persistHeader{
		Magic:    magicNumber,
		Version:  version,
		Reserved: 0,
		Count:    uint64(len(index)),
	}

	headerBytes := make([]byte, headerSize)
	binary.BigEndian.PutUint32(headerBytes[0:4], header.Magic)
	binary.BigEndian.PutUint16(headerBytes[4:6], header.Version)
	binary.BigEndian.PutUint16(headerBytes[6:8], header.Reserved)
	binary.BigEndian.PutUint64(headerBytes[8:16], header.Count)

	if _, err := writer.Write(headerBytes); err != nil {
		return err
	}

	entriesChecksum := sha256.New()
	for fp := range index {
		entry := persistEntry{
			Type:      entryTypeFP,
			FP:        fp,
			Timestamp: 0,
		}

		entryBytes := encodeEntry(entry)
		if _, err := entriesChecksum.Write(entryBytes); err != nil {
			return err
		}
		if _, err := writer.Write(entryBytes); err != nil {
			return err
		}
	}

	entriesDigest := entriesChecksum.Sum(nil)

	edEntry := persistEntry{
		Type:      entryTypeEntriesDigest,
		FP:        Fingerprint(entriesDigest),
		Timestamp: 0,
	}
	edEntryBytes := encodeEntry(edEntry)
	if _, err := writer.Write(edEntryBytes); err != nil {
		return err
	}

	fileChecksum := sha256.New()
	fileChecksum.Write(headerBytes)
	fileChecksum.Write(entriesDigest)
	fileCS := hex.EncodeToString(fileChecksum.Sum(nil))

	csEntry := persistEntry{
		Type:      entryTypeCS,
		FP:        Fingerprint(fileCS),
		Timestamp: 0,
	}

	csEntryBytes := encodeEntry(csEntry)
	if _, err := writer.Write(csEntryBytes); err != nil {
		return err
	}

	if err := writer.Flush(); err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}

func (p *persistIndex) Load(path string) (FingerprintIndex, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.loadLocked(path)
}

func (p *persistIndex) loadLocked(path string) (FingerprintIndex, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, ErrPersistFileNotExist
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if len(data) < headerSize {
		return nil, ErrPersistCorrupted
	}

	header := persistHeader{
		Magic:    binary.BigEndian.Uint32(data[0:4]),
		Version:  binary.BigEndian.Uint16(data[4:6]),
		Reserved: binary.BigEndian.Uint16(data[6:8]),
		Count:    binary.BigEndian.Uint64(data[8:16]),
	}

	if header.Magic != magicNumber {
		return nil, ErrPersistCorrupted
	}

	if header.Version != version {
		return nil, ErrPersistCorrupted
	}

	headerBytes := data[0:headerSize]

	offset := headerSize
	entriesChecksum := sha256.New()
	var storedEntriesDigest Fingerprint

	index := make(FingerprintIndex, header.Count)
	var lastChecksum string
	fpCount := uint64(0)

	for offset < len(data) {
		if offset+1 > len(data) {
			return nil, ErrPersistCorrupted
		}
		entryType := data[offset]

		if offset+5 > len(data) {
			return nil, ErrPersistCorrupted
		}
		fpLen := binary.BigEndian.Uint32(data[offset+1 : offset+5])

		if fpLen > 1024 {
			return nil, ErrPersistCorrupted
		}

		if offset+5+int(fpLen)+8 > len(data) {
			return nil, ErrPersistCorrupted
		}

		fpBytes := data[offset+5 : offset+5+int(fpLen)]

		entryLen := 1 + 4 + int(fpLen) + 8

		if entryType == entryTypeCS {
			lastChecksum = string(fpBytes)
			offset += entryLen
			break
		}

		if entryType == entryTypeEntriesDigest {
			storedEntriesDigest = Fingerprint(fpBytes)
			offset += entryLen
			continue
		}

		if entryType == entryTypeFP {
			entriesChecksum.Write(data[offset : offset+entryLen])
			index[Fingerprint(fpBytes)] = true
			fpCount++
		}

		offset += entryLen
	}

	if lastChecksum == "" {
		return nil, ErrPersistCorrupted
	}

	if fpCount != header.Count {
		return nil, ErrPersistCorrupted
	}

	entriesDigest := entriesChecksum.Sum(nil)

	if storedEntriesDigest != "" {
		if !bytes.Equal(entriesDigest, []byte(storedEntriesDigest)) {
			return nil, ErrChecksumMismatch
		}
	}

	fileChecksum := sha256.New()
	fileChecksum.Write(headerBytes)
	fileChecksum.Write(entriesDigest)
	calculated := hex.EncodeToString(fileChecksum.Sum(nil))

	if calculated != lastChecksum {
		return nil, ErrChecksumMismatch
	}

	return index, nil
}

func (p *persistIndex) Append(fp Fingerprint, path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return p.saveLocked(FingerprintIndex{fp: true}, path)
	}

	return p.appendLocked(fp, path)
}

func (p *persistIndex) appendLocked(fp Fingerprint, path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	fileInfo, err := f.Stat()
	if err != nil {
		return err
	}

	if fileInfo.Size() < int64(headerSize) {
		return ErrPersistCorrupted
	}

	headerBytes := make([]byte, headerSize)
	if _, err := io.ReadFull(f, headerBytes); err != nil {
		return err
	}

	magic := binary.BigEndian.Uint32(headerBytes[0:4])
	ver := binary.BigEndian.Uint16(headerBytes[4:6])
	count := binary.BigEndian.Uint64(headerBytes[8:16])

	if magic != magicNumber || ver != version {
		return ErrPersistCorrupted
	}

	entriesChecksum := sha256.New()

	var csEntryOffset int64 = -1
	var edEntryOffset int64 = -1
	var fpCount uint64
	duplicateFound := false

	for {
		curOffset, _ := f.Seek(0, io.SeekCurrent)
		if curOffset >= fileInfo.Size() {
			break
		}

		entryHeader := make([]byte, 5)
		if _, err := io.ReadFull(f, entryHeader); err != nil {
			if err == io.EOF {
				break
			}
			return ErrPersistCorrupted
		}

		entryType := entryHeader[0]
		fpLen := binary.BigEndian.Uint32(entryHeader[1:5])

		if fpLen > 1024 {
			return ErrPersistCorrupted
		}

		restLen := int(fpLen) + 8
		rest := make([]byte, restLen)
		if _, err := io.ReadFull(f, rest); err != nil {
			return ErrPersistCorrupted
		}

		fpBytes := rest[:int(fpLen)]

		if entryType == entryTypeCS {
			csEntryOffset = curOffset
			break
		}

		if entryType == entryTypeEntriesDigest {
			edEntryOffset = curOffset
			continue
		}

		if entryType == entryTypeFP {
			fpCount++
			entriesChecksum.Write(entryHeader)
			entriesChecksum.Write(rest)
			if Fingerprint(fpBytes) == fp {
				duplicateFound = true
			}
		}
	}

	if csEntryOffset < 0 {
		return ErrPersistCorrupted
	}

	if fpCount != count {
		return ErrPersistCorrupted
	}

	if duplicateFound {
		return nil
	}

	fpEntry := persistEntry{
		Type:      entryTypeFP,
		FP:        fp,
		Timestamp: 0,
	}
	fpEntryBytes := encodeEntry(fpEntry)

	entriesChecksum.Write(fpEntryBytes)
	newEntriesDigest := entriesChecksum.Sum(nil)

	edEntry := persistEntry{
		Type:      entryTypeEntriesDigest,
		FP:        Fingerprint(newEntriesDigest),
		Timestamp: 0,
	}
	edEntryBytes := encodeEntry(edEntry)

	newCount := count + 1
	newHeaderBytes := make([]byte, headerSize)
	binary.BigEndian.PutUint32(newHeaderBytes[0:4], magicNumber)
	binary.BigEndian.PutUint16(newHeaderBytes[4:6], version)
	binary.BigEndian.PutUint16(newHeaderBytes[6:8], 0)
	binary.BigEndian.PutUint64(newHeaderBytes[8:16], newCount)

	fileChecksum := sha256.New()
	fileChecksum.Write(newHeaderBytes)
	fileChecksum.Write(newEntriesDigest)
	newCS := hex.EncodeToString(fileChecksum.Sum(nil))

	newCSEntry := persistEntry{
		Type:      entryTypeCS,
		FP:        Fingerprint(newCS),
		Timestamp: 0,
	}
	newCSEntryBytes := encodeEntry(newCSEntry)

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := f.Write(newHeaderBytes); err != nil {
		return err
	}

	if edEntryOffset >= 0 {
		if _, err := f.Seek(edEntryOffset, io.SeekStart); err != nil {
			return err
		}
		if _, err := f.Write(edEntryBytes); err != nil {
			return err
		}
	}

	if _, err := f.Seek(csEntryOffset, io.SeekStart); err != nil {
		return err
	}
	if _, err := f.Write(fpEntryBytes); err != nil {
		return err
	}
	if _, err := f.Write(newCSEntryBytes); err != nil {
		return err
	}

	newFileSize := csEntryOffset + int64(len(fpEntryBytes)) + int64(len(newCSEntryBytes))
	if err := f.Truncate(newFileSize); err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}

	return nil
}

func (p *persistIndex) Verify(path string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, ErrPersistFileNotExist
	}

	_, err := p.loadLocked(path)
	if err != nil {
		return false, nil
	}

	return true, nil
}

func encodeEntry(entry persistEntry) []byte {
	fpBytes := []byte(entry.FP)
	fpLen := uint32(len(fpBytes))

	buf := make([]byte, 1+4+int(fpLen)+8)
	buf[0] = entry.Type
	binary.BigEndian.PutUint32(buf[1:5], fpLen)
	copy(buf[5:5+int(fpLen)], fpBytes)
	binary.BigEndian.PutUint64(buf[5+int(fpLen):], uint64(entry.Timestamp))

	return buf
}

type indexPersister interface {
	GetIndex() FingerprintIndex
	SetIndex(index FingerprintIndex)
	AddFingerprint(fp Fingerprint)
	Count() int
	Close() error
}

func saveIndex(persister indexPersister, persisterImpl PersistIndex, path string) error {
	index := persister.GetIndex()
	return persisterImpl.Save(index, path)
}

func loadIndex(persister indexPersister, persisterImpl PersistIndex, path string) error {
	index, err := persisterImpl.Load(path)
	if err != nil {
		return err
	}
	persister.SetIndex(index)
	return nil
}

func appendFingerprint(persister indexPersister, persisterImpl PersistIndex, fp Fingerprint, path string) error {
	if err := persisterImpl.Append(fp, path); err != nil {
		return err
	}
	persister.AddFingerprint(fp)
	return nil
}

func verifyIndex(persisterImpl PersistIndex, path string) (bool, error) {
	return persisterImpl.Verify(path)
}
