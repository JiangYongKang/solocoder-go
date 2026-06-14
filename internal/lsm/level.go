package lsm

import (
	"fmt"
	"sort"
	"sync"
)

type Level struct {
	mu      sync.RWMutex
	level   int
	tables  []*SSTable
	maxSize int
}

func NewLevel(level int, maxSize int) *Level {
	return &Level{
		level:   level,
		tables:  make([]*SSTable, 0),
		maxSize: maxSize,
	}
}

func (l *Level) AddTable(sst *SSTable) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tables = append(l.tables, sst)
	l.sortTables()
}

func (l *Level) AddTables(tables []*SSTable) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tables = append(l.tables, tables...)
	l.sortTables()
}

func (l *Level) RemoveTable(sst *SSTable) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, t := range l.tables {
		if t.Filename() == sst.Filename() {
			l.tables = append(l.tables[:i], l.tables[i+1:]...)
			break
		}
	}
}

func (l *Level) RemoveTables(tables []*SSTable) {
	l.mu.Lock()
	defer l.mu.Unlock()
	removeSet := make(map[string]bool)
	for _, t := range tables {
		removeSet[t.Filename()] = true
	}
	remaining := make([]*SSTable, 0, len(l.tables))
	for _, t := range l.tables {
		if !removeSet[t.Filename()] {
			remaining = append(remaining, t)
		}
	}
	l.tables = remaining
}

func (l *Level) Tables() []*SSTable {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]*SSTable, len(l.tables))
	copy(result, l.tables)
	return result
}

func (l *Level) Size() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.tables)
}

func (l *Level) Level() int {
	return l.level
}

func (l *Level) MaxSize() int {
	return l.maxSize
}

func (l *Level) NeedsCompaction() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.tables) >= l.maxSize
}

func (l *Level) sortTables() {
	sort.Slice(l.tables, func(i, j int) bool {
		return l.tables[i].MinKey() < l.tables[j].MinKey()
	})
}

func (l *Level) Get(key string) (*Entry, bool, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.level == 0 {
		for i := len(l.tables) - 1; i >= 0; i-- {
			sst := l.tables[i]
			entry, found, err := sst.Get(key)
			if err != nil {
				return nil, false, err
			}
			if found {
				return entry, true, nil
			}
		}
	} else {
		idx := l.findTable(key)
		if idx >= 0 && idx < len(l.tables) {
			sst := l.tables[idx]
			if key >= sst.MinKey() && key <= sst.MaxKey() {
				return sst.Get(key)
			}
		}
	}
	return nil, false, nil
}

func (l *Level) Range(start, end string) ([]*Entry, error) {
	entries, err := l.rangeInternal(start, end)
	if err != nil {
		return nil, err
	}
	result := make([]*Entry, 0, len(entries))
	for _, e := range entries {
		if !e.Tombstone {
			result = append(result, e)
		}
	}
	return result, nil
}

func (l *Level) RangeWithTombstone(start, end string) ([]*Entry, error) {
	return l.rangeInternal(start, end)
}

func (l *Level) rangeInternal(start, end string) ([]*Entry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	resultMap := make(map[string]*Entry)

	if l.level == 0 {
		for i := len(l.tables) - 1; i >= 0; i-- {
			sst := l.tables[i]
			entries, err := sst.RangeWithTombstone(start, end)
			if err != nil {
				return nil, err
			}
			for _, e := range entries {
				existing, ok := resultMap[e.Key]
				if !ok || e.Timestamp > existing.Timestamp {
					resultMap[e.Key] = e
				}
			}
		}
	} else {
		for _, sst := range l.tables {
			if sst.MaxKey() < start || sst.MinKey() > end {
				continue
			}
			entries, err := sst.RangeWithTombstone(start, end)
			if err != nil {
				return nil, err
			}
			for _, e := range entries {
				existing, ok := resultMap[e.Key]
				if !ok || e.Timestamp > existing.Timestamp {
					resultMap[e.Key] = e
				}
			}
		}
	}

	result := make([]*Entry, 0, len(resultMap))
	for _, e := range resultMap {
		result = append(result, e)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})
	return result, nil
}

func (l *Level) findTable(key string) int {
	if len(l.tables) == 0 {
		return -1
	}
	low, high := 0, len(l.tables)-1
	for low <= high {
		mid := (low + high) / 2
		sst := l.tables[mid]
		if key < sst.MinKey() {
			high = mid - 1
		} else if key > sst.MaxKey() {
			low = mid + 1
		} else {
			return mid
		}
	}
	return -1
}

func (l *Level) FindOverlappingTables(minKey, maxKey string) []*SSTable {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var overlapping []*SSTable
	for _, sst := range l.tables {
		if sst.OverlapsWith(minKey, maxKey) {
			overlapping = append(overlapping, sst)
		}
	}
	return overlapping
}

func (l *Level) GetTableForCompaction() *SSTable {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(l.tables) == 0 {
		return nil
	}
	return l.tables[0]
}

func (l *Level) GetAllEntries() ([]*Entry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var allEntries []*Entry
	seen := make(map[string]*Entry)

	for _, sst := range l.tables {
		entries, err := sst.AllEntries()
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			existing, ok := seen[e.Key]
			if !ok || e.Timestamp > existing.Timestamp {
				seen[e.Key] = e
			}
		}
	}

	for _, e := range seen {
		allEntries = append(allEntries, e)
	}

	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Key < allEntries[j].Key
	})
	return allEntries, nil
}

func (l *Level) DebugInfo() string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	info := fmt.Sprintf("Level %d: %d tables, max %d\n", l.level, len(l.tables), l.maxSize)
	for i, sst := range l.tables {
		info += fmt.Sprintf("  Table %d: %s, keys [%s, %s], entries %d\n",
			i, sst.Filename(), sst.MinKey(), sst.MaxKey(), sst.EntryCount())
	}
	return info
}
