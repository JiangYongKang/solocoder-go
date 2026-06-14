package lsm

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type SSTable struct {
	mu          sync.RWMutex
	filename    string
	level       int
	index       map[string]*IndexEntry
	indexOffset int64
	indexLen    int32
	minKey      string
	maxKey      string
	entryCount  int
	fileSize    int64
}

func NewSSTable(filename string, level int, entries []*Entry) (*SSTable, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("cannot create SSTable with empty entries")
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})

	seen := make(map[string]*Entry)
	for _, e := range entries {
		existing, ok := seen[e.Key]
		if !ok || e.Timestamp > existing.Timestamp {
			seen[e.Key] = e
		}
	}

	sortedEntries := make([]*Entry, 0, len(seen))
	for _, e := range seen {
		sortedEntries = append(sortedEntries, e)
	}
	sort.Slice(sortedEntries, func(i, j int) bool {
		return sortedEntries[i].Key < sortedEntries[j].Key
	})

	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSTable file: %w", err)
	}
	defer file.Close()

	index := make(map[string]*IndexEntry)
	var offset int64 = 0

	for _, entry := range sortedEntries {
		encoded := entry.Encode()
		entryLen := int32(len(encoded))

		n, err := file.Write(encoded)
		if err != nil {
			return nil, fmt.Errorf("failed to write entry: %w", err)
		}

		index[entry.Key] = &IndexEntry{
			Key:      entry.Key,
			Offset:   offset,
			EntryLen: entryLen,
		}

		offset += int64(n)
	}

	indexOffset := offset
	var indexLen int32 = 0

	sortedKeys := make([]string, 0, len(index))
	for k := range index {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, key := range sortedKeys {
		ie := index[key]
		encoded := ie.Encode()
		n, err := file.Write(encoded)
		if err != nil {
			return nil, fmt.Errorf("failed to write index entry: %w", err)
		}
		indexLen += int32(n)
	}

	if err := binary.Write(file, binary.LittleEndian, indexOffset); err != nil {
		return nil, fmt.Errorf("failed to write index offset: %w", err)
	}
	if err := binary.Write(file, binary.LittleEndian, indexLen); err != nil {
		return nil, fmt.Errorf("failed to write index length: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file stat: %w", err)
	}

	minKey := sortedKeys[0]
	maxKey := sortedKeys[len(sortedKeys)-1]

	return &SSTable{
		filename:    filename,
		level:       level,
		index:       index,
		indexOffset: indexOffset,
		indexLen:    indexLen,
		minKey:      minKey,
		maxKey:      maxKey,
		entryCount:  len(sortedEntries),
		fileSize:    stat.Size(),
	}, nil
}

func LoadSSTable(filename string, level int) (*SSTable, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open SSTable file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file stat: %w", err)
	}
	fileSize := stat.Size()

	if fileSize < 12 {
		return nil, fmt.Errorf("SSTable file too small")
	}

	var indexOffset int64
	var indexLen int32

	_, err = file.Seek(fileSize-12, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to footer: %w", err)
	}

	if err := binary.Read(file, binary.LittleEndian, &indexOffset); err != nil {
		return nil, fmt.Errorf("failed to read index offset: %w", err)
	}
	if err := binary.Read(file, binary.LittleEndian, &indexLen); err != nil {
		return nil, fmt.Errorf("failed to read index length: %w", err)
	}

	_, err = file.Seek(indexOffset, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to index: %w", err)
	}

	indexData := make([]byte, indexLen)
	_, err = file.Read(indexData)
	if err != nil {
		return nil, fmt.Errorf("failed to read index data: %w", err)
	}

	index := make(map[string]*IndexEntry)
	var minKey, maxKey string
	read := 0
	for read < len(indexData) {
		ie, n, err := DecodeIndexEntry(indexData[read:])
		if err != nil {
			return nil, fmt.Errorf("failed to decode index entry: %w", err)
		}
		index[ie.Key] = ie
		read += n

		if minKey == "" || ie.Key < minKey {
			minKey = ie.Key
		}
		if maxKey == "" || ie.Key > maxKey {
			maxKey = ie.Key
		}
	}

	return &SSTable{
		filename:    filename,
		level:       level,
		index:       index,
		indexOffset: indexOffset,
		indexLen:    indexLen,
		minKey:      minKey,
		maxKey:      maxKey,
		entryCount:  len(index),
		fileSize:    fileSize,
	}, nil
}

func (sst *SSTable) Get(key string) (*Entry, bool, error) {
	sst.mu.RLock()
	defer sst.mu.RUnlock()

	ie, ok := sst.index[key]
	if !ok {
		return nil, false, nil
	}

	if key < sst.minKey || key > sst.maxKey {
		return nil, false, nil
	}

	file, err := os.Open(sst.filename)
	if err != nil {
		return nil, false, fmt.Errorf("failed to open SSTable: %w", err)
	}
	defer file.Close()

	_, err = file.Seek(ie.Offset, 0)
	if err != nil {
		return nil, false, fmt.Errorf("failed to seek to entry: %w", err)
	}

	entryData := make([]byte, ie.EntryLen)
	_, err = file.Read(entryData)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read entry data: %w", err)
	}

	entry, _, err := DecodeEntry(entryData)
	if err != nil {
		return nil, false, fmt.Errorf("failed to decode entry: %w", err)
	}

	return entry, true, nil
}

func (sst *SSTable) Range(start, end string) ([]*Entry, error) {
	sst.mu.RLock()
	defer sst.mu.RUnlock()

	if end < sst.minKey || start > sst.maxKey {
		return nil, nil
	}

	file, err := os.Open(sst.filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open SSTable: %w", err)
	}
	defer file.Close()

	sortedKeys := make([]string, 0, len(sst.index))
	for k := range sst.index {
		if k >= start && k <= end {
			sortedKeys = append(sortedKeys, k)
		}
	}
	sort.Strings(sortedKeys)

	var result []*Entry
	for _, key := range sortedKeys {
		ie := sst.index[key]

		_, err = file.Seek(ie.Offset, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to seek to entry: %w", err)
		}

		entryData := make([]byte, ie.EntryLen)
		_, err = file.Read(entryData)
		if err != nil {
			return nil, fmt.Errorf("failed to read entry data: %w", err)
		}

		entry, _, err := DecodeEntry(entryData)
		if err != nil {
			return nil, fmt.Errorf("failed to decode entry: %w", err)
		}

		if !entry.Tombstone {
			result = append(result, entry)
		}
	}

	return result, nil
}

func (sst *SSTable) RangeWithTombstone(start, end string) ([]*Entry, error) {
	sst.mu.RLock()
	defer sst.mu.RUnlock()

	if end < sst.minKey || start > sst.maxKey {
		return nil, nil
	}

	file, err := os.Open(sst.filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open SSTable: %w", err)
	}
	defer file.Close()

	sortedKeys := make([]string, 0, len(sst.index))
	for k := range sst.index {
		if k >= start && k <= end {
			sortedKeys = append(sortedKeys, k)
		}
	}
	sort.Strings(sortedKeys)

	var result []*Entry
	for _, key := range sortedKeys {
		ie := sst.index[key]

		_, err = file.Seek(ie.Offset, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to seek to entry: %w", err)
		}

		entryData := make([]byte, ie.EntryLen)
		_, err = file.Read(entryData)
		if err != nil {
			return nil, fmt.Errorf("failed to read entry data: %w", err)
		}

		entry, _, err := DecodeEntry(entryData)
		if err != nil {
			return nil, fmt.Errorf("failed to decode entry: %w", err)
		}

		result = append(result, entry)
	}

	return result, nil
}

func (sst *SSTable) AllEntries() ([]*Entry, error) {
	sst.mu.RLock()
	defer sst.mu.RUnlock()

	file, err := os.Open(sst.filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open SSTable: %w", err)
	}
	defer file.Close()

	dataLen := sst.indexOffset
	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to start: %w", err)
	}

	data := make([]byte, dataLen)
	_, err = file.Read(data)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	var entries []*Entry
	read := 0
	for read < int(dataLen) {
		entry, n, err := DecodeEntry(data[read:])
		if err != nil {
			return nil, fmt.Errorf("failed to decode entry: %w", err)
		}
		entries = append(entries, entry)
		read += n
	}

	return entries, nil
}

func (sst *SSTable) MinKey() string {
	sst.mu.RLock()
	defer sst.mu.RUnlock()
	return sst.minKey
}

func (sst *SSTable) MaxKey() string {
	sst.mu.RLock()
	defer sst.mu.RUnlock()
	return sst.maxKey
}

func (sst *SSTable) Filename() string {
	sst.mu.RLock()
	defer sst.mu.RUnlock()
	return sst.filename
}

func (sst *SSTable) Level() int {
	sst.mu.RLock()
	defer sst.mu.RUnlock()
	return sst.level
}

func (sst *SSTable) EntryCount() int {
	sst.mu.RLock()
	defer sst.mu.RUnlock()
	return sst.entryCount
}

func (sst *SSTable) FileSize() int64 {
	sst.mu.RLock()
	defer sst.mu.RUnlock()
	return sst.fileSize
}

func (sst *SSTable) DeleteFile() error {
	sst.mu.Lock()
	defer sst.mu.Unlock()
	return os.Remove(sst.filename)
}

func (sst *SSTable) OverlapsWith(minKey, maxKey string) bool {
	sst.mu.RLock()
	defer sst.mu.RUnlock()
	return !(sst.maxKey < minKey || sst.minKey > maxKey)
}

func GenerateSSTableFilename(dataDir string, level, seqNum int) string {
	return filepath.Join(dataDir, fmt.Sprintf("L%d_%06d.sst", level, seqNum))
}
