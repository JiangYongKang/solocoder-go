package lsm

import (
	"sync"
	"time"
)

type MemTable struct {
	mu        sync.RWMutex
	data      *SkipList
	size      int
	maxSize   int
	frozen    bool
}

func NewMemTable(maxSize int) *MemTable {
	return &MemTable{
		data:    NewSkipList(),
		maxSize: maxSize,
		frozen:  false,
	}
}

func (mt *MemTable) Put(key, value string) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	entry := &Entry{
		Key:       key,
		Value:     value,
		Tombstone: false,
		Timestamp: time.Now().UnixNano(),
	}

	oldEntry, exists := mt.data.Get(key)
	if exists {
		mt.size -= oldEntry.Size()
	}

	mt.data.Insert(entry)
	mt.size += entry.Size()
}

func (mt *MemTable) Delete(key string) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	oldEntry, exists := mt.data.Get(key)
	if exists {
		mt.size -= oldEntry.Size()
	}

	entry := &Entry{
		Key:       key,
		Value:     "",
		Tombstone: true,
		Timestamp: time.Now().UnixNano(),
	}

	mt.data.Insert(entry)
	mt.size += entry.Size()
}

func (mt *MemTable) Get(key string) (*Entry, bool) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	entry, exists := mt.data.Get(key)
	if !exists {
		return nil, false
	}

	if entry.Tombstone {
		return nil, false
	}

	return entry, true
}

func (mt *MemTable) GetWithTombstone(key string) (*Entry, bool) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.data.Get(key)
}

func (mt *MemTable) ShouldFlush() bool {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.size >= mt.maxSize
}

func (mt *MemTable) Freeze() {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	mt.frozen = true
}

func (mt *MemTable) IsFrozen() bool {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.frozen
}

func (mt *MemTable) Size() int {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.size
}

func (mt *MemTable) Len() int {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.data.Len()
}

func (mt *MemTable) Range(start, end string) []*Entry {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	entries := mt.data.Range(start, end)
	result := make([]*Entry, 0, len(entries))
	for _, e := range entries {
		if !e.Tombstone {
			result = append(result, e)
		}
	}
	return result
}

func (mt *MemTable) RangeWithTombstone(start, end string) []*Entry {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.data.Range(start, end)
}

func (mt *MemTable) AllEntries() []*Entry {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.data.AllEntries()
}

func (mt *MemTable) Iterator() *SkipListIterator {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.data.Iterator()
}
