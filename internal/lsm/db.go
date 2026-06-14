package lsm

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type DB struct {
	mu         sync.RWMutex
	config     Config
	memTable   *MemTable
	immutable  []*MemTable
	flushing   *MemTable
	levels     []*Level
	seqNum     int64
	closed     bool
	mergeMu    sync.Mutex
	merging    atomic.Bool
	flushCh    chan struct{}
	mergeCh    chan struct{}
	wg         sync.WaitGroup
	stopCh     chan struct{}
}

func NewDB(config Config) (*DB, error) {
	config.validate()

	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	levels := make([]*Level, config.MaxLevel)
	for i := 0; i < config.MaxLevel; i++ {
		levels[i] = NewLevel(i, config.LevelMaxFiles[i])
	}

	db := &DB{
		config:   config,
		memTable: NewMemTable(config.MemTableSize),
		levels:   levels,
		seqNum:   0,
		flushCh:  make(chan struct{}, 1),
		mergeCh:  make(chan struct{}, 1),
		stopCh:   make(chan struct{}),
	}

	if err := db.loadExistingSSTables(); err != nil {
		return nil, fmt.Errorf("failed to load existing SSTables: %w", err)
	}

	db.wg.Add(2)
	go db.flushLoop()
	go db.mergeLoop()

	return db, nil
}

func (db *DB) loadExistingSSTables() error {
	files, err := os.ReadDir(db.config.DataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	type tableInfo struct {
		level  int
		seqNum int
		path   string
	}

	var tables []tableInfo
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if !strings.HasSuffix(name, ".sst") {
			continue
		}
		parts := strings.Split(strings.TrimSuffix(name, ".sst"), "_")
		if len(parts) != 2 {
			continue
		}
		if !strings.HasPrefix(parts[0], "L") {
			continue
		}
		level, err := strconv.Atoi(strings.TrimPrefix(parts[0], "L"))
		if err != nil {
			continue
		}
		seqNum, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		if level < 0 || level >= db.config.MaxLevel {
			continue
		}
		tables = append(tables, tableInfo{
			level:  level,
			seqNum: seqNum,
			path:   filepath.Join(db.config.DataDir, name),
		})
		if int64(seqNum) > db.seqNum {
			db.seqNum = int64(seqNum)
		}
	}

	for _, ti := range tables {
		sst, err := LoadSSTable(ti.path, ti.level)
		if err != nil {
			return fmt.Errorf("failed to load SSTable %s: %w", ti.path, err)
		}
		db.levels[ti.level].AddTable(sst)
	}

	db.seqNum++

	return nil
}

func (db *DB) Put(key, value string) error {
	if key == "" {
		return ErrEmptyKey
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return ErrDBClosed
	}

	db.memTable.Put(key, value)
	needsFlush := db.memTable.ShouldFlush()

	if needsFlush {
		db.memTable.Freeze()
		db.immutable = append(db.immutable, db.memTable)
		db.memTable = NewMemTable(db.config.MemTableSize)

		select {
		case db.flushCh <- struct{}{}:
		default:
		}
	}

	return nil
}

func (db *DB) Delete(key string) error {
	if key == "" {
		return ErrEmptyKey
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return ErrDBClosed
	}

	db.memTable.Delete(key)
	needsFlush := db.memTable.ShouldFlush()

	if needsFlush {
		db.memTable.Freeze()
		db.immutable = append(db.immutable, db.memTable)
		db.memTable = NewMemTable(db.config.MemTableSize)

		select {
		case db.flushCh <- struct{}{}:
		default:
		}
	}

	return nil
}

func (db *DB) Get(key string) (string, error) {
	if key == "" {
		return "", ErrEmptyKey
	}

	db.mu.RLock()
	if db.closed {
		db.mu.RUnlock()
		return "", ErrDBClosed
	}

	entry, found := db.memTable.GetWithTombstone(key)
	if found {
		if entry.Tombstone {
			db.mu.RUnlock()
			return "", ErrKeyNotFound
		}
		db.mu.RUnlock()
		return entry.Value, nil
	}

	for i := len(db.immutable) - 1; i >= 0; i-- {
		entry, found = db.immutable[i].GetWithTombstone(key)
		if found {
			if entry.Tombstone {
				db.mu.RUnlock()
				return "", ErrKeyNotFound
			}
			db.mu.RUnlock()
			return entry.Value, nil
		}
	}

	if db.flushing != nil {
		entry, found = db.flushing.GetWithTombstone(key)
		if found {
			if entry.Tombstone {
				db.mu.RUnlock()
				return "", ErrKeyNotFound
			}
			db.mu.RUnlock()
			return entry.Value, nil
		}
	}
	db.mu.RUnlock()

	for level := 0; level < db.config.MaxLevel; level++ {
		db.mu.RLock()
		if db.closed {
			db.mu.RUnlock()
			return "", ErrDBClosed
		}
		db.mu.RUnlock()

		entry, found, err := db.levels[level].Get(key)
		if err != nil {
			return "", err
		}
		if found {
			if entry.Tombstone {
				return "", ErrKeyNotFound
			}
			return entry.Value, nil
		}
	}

	return "", ErrKeyNotFound
}

func (db *DB) Range(start, end string) ([]*Entry, error) {
	if start > end {
		return nil, ErrInvalidRange
	}

	resultMap := make(map[string]*Entry)

	db.mu.RLock()
	if db.closed {
		db.mu.RUnlock()
		return nil, ErrDBClosed
	}

	memEntries := db.memTable.RangeWithTombstone(start, end)
	for _, e := range memEntries {
		if existing, ok := resultMap[e.Key]; !ok || e.Timestamp > existing.Timestamp {
			resultMap[e.Key] = e
		}
	}

	for i := len(db.immutable) - 1; i >= 0; i-- {
		immEntries := db.immutable[i].RangeWithTombstone(start, end)
		for _, e := range immEntries {
			if existing, ok := resultMap[e.Key]; !ok || e.Timestamp > existing.Timestamp {
				resultMap[e.Key] = e
			}
		}
	}

	if db.flushing != nil {
		flushEntries := db.flushing.RangeWithTombstone(start, end)
		for _, e := range flushEntries {
			if existing, ok := resultMap[e.Key]; !ok || e.Timestamp > existing.Timestamp {
				resultMap[e.Key] = e
			}
		}
	}
	db.mu.RUnlock()

	for level := 0; level < db.config.MaxLevel; level++ {
		db.mu.RLock()
		if db.closed {
			db.mu.RUnlock()
			return nil, ErrDBClosed
		}
		db.mu.RUnlock()

		levelEntries, err := db.levels[level].RangeWithTombstone(start, end)
		if err != nil {
			return nil, err
		}
		for _, e := range levelEntries {
			if existing, ok := resultMap[e.Key]; !ok || e.Timestamp > existing.Timestamp {
				resultMap[e.Key] = e
			}
		}
	}

	result := make([]*Entry, 0, len(resultMap))
	for _, e := range resultMap {
		if !e.Tombstone {
			result = append(result, e)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})

	return result, nil
}

func (db *DB) flushLoop() {
	defer db.wg.Done()

	for {
		select {
		case <-db.flushCh:
			db.flushImmutable()
		case <-db.stopCh:
			return
		}
	}
}

func (db *DB) flushImmutable() {
	for {
		db.mu.Lock()
		if len(db.immutable) == 0 {
			db.mu.Unlock()
			return
		}
		mt := db.immutable[0]
		db.immutable = db.immutable[1:]
		db.flushing = mt
		db.mu.Unlock()

		if err := db.flushMemTable(mt, 0); err != nil {
			fmt.Printf("Failed to flush memtable: %v\n", err)
		}

		db.mu.Lock()
		db.flushing = nil
		db.mu.Unlock()
	}
}

func (db *DB) flushMemTable(mt *MemTable, level int) error {
	entries := mt.AllEntries()
	if len(entries) == 0 {
		return nil
	}

	db.mu.Lock()
	seqNum := db.seqNum
	db.seqNum++
	db.mu.Unlock()

	filename := GenerateSSTableFilename(db.config.DataDir, level, int(seqNum))
	sst, err := NewSSTable(filename, level, entries)
	if err != nil {
		return fmt.Errorf("failed to create SSTable: %w", err)
	}

	db.levels[level].AddTable(sst)

	select {
	case db.mergeCh <- struct{}{}:
	default:
	}

	return nil
}

func (db *DB) mergeLoop() {
	defer db.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-db.mergeCh:
			db.runCompaction()
		case <-ticker.C:
			db.checkAndTriggerCompaction()
		case <-db.stopCh:
			return
		}
	}
}

func (db *DB) checkAndTriggerCompaction() {
	for level := 0; level < db.config.MaxLevel-1; level++ {
		if db.levels[level].NeedsCompaction() {
			select {
			case db.mergeCh <- struct{}{}:
			default:
			}
			return
		}
	}
}

func (db *DB) runCompaction() {
	if !db.merging.CompareAndSwap(false, true) {
		return
	}
	defer db.merging.Store(false)

	for level := 0; level < db.config.MaxLevel-1; level++ {
		for db.levels[level].NeedsCompaction() {
			if err := db.compactLevel(level); err != nil {
				fmt.Printf("Failed to compact level %d: %v\n", level, err)
				return
			}
		}
	}
}

func (db *DB) compactLevel(level int) error {
	db.mergeMu.Lock()
	defer db.mergeMu.Unlock()

	var tablesToMerge []*SSTable
	var minKey, maxKey string

	if level == 0 {
		sst := db.levels[level].GetTableForCompaction()
		if sst == nil {
			return nil
		}
		tablesToMerge = []*SSTable{sst}
		minKey = sst.MinKey()
		maxKey = sst.MaxKey()

		for _, t := range db.levels[level].Tables() {
			if t.Filename() != sst.Filename() && t.OverlapsWith(minKey, maxKey) {
				tablesToMerge = append(tablesToMerge, t)
				if t.MinKey() < minKey {
					minKey = t.MinKey()
				}
				if t.MaxKey() > maxKey {
					maxKey = t.MaxKey()
				}
			}
		}
	} else {
		sst := db.levels[level].GetTableForCompaction()
		if sst == nil {
			return nil
		}
		tablesToMerge = []*SSTable{sst}
		minKey = sst.MinKey()
		maxKey = sst.MaxKey()
	}

	nextLevel := level + 1
	overlapping := db.levels[nextLevel].FindOverlappingTables(minKey, maxKey)
	tablesToMerge = append(tablesToMerge, overlapping...)

	allEntries := make(map[string]*Entry)
	for _, t := range tablesToMerge {
		entries, err := t.AllEntries()
		if err != nil {
			return fmt.Errorf("failed to get entries from %s: %w", t.Filename(), err)
		}
		for _, e := range entries {
			existing, ok := allEntries[e.Key]
			if !ok || e.Timestamp > existing.Timestamp {
				allEntries[e.Key] = e
			}
		}
	}

	hasDeeperOverlap := false
	for deeperLevel := nextLevel + 1; deeperLevel < db.config.MaxLevel; deeperLevel++ {
		if len(db.levels[deeperLevel].FindOverlappingTables(minKey, maxKey)) > 0 {
			hasDeeperOverlap = true
			break
		}
	}

	mergedEntries := make([]*Entry, 0, len(allEntries))
	for _, e := range allEntries {
		if e.Tombstone {
			if hasDeeperOverlap {
				mergedEntries = append(mergedEntries, e)
			}
		} else {
			mergedEntries = append(mergedEntries, e)
		}
	}
	sort.Slice(mergedEntries, func(i, j int) bool {
		return mergedEntries[i].Key < mergedEntries[j].Key
	})

	if len(mergedEntries) == 0 {
		db.levels[level].RemoveTables(tablesToMerge[:len(tablesToMerge)-len(overlapping)])
		db.levels[nextLevel].RemoveTables(overlapping)
		for _, t := range tablesToMerge {
			_ = t.DeleteFile()
		}
		return nil
	}

	targetSize := db.config.TargetFileSize
	var newTables []*SSTable
	var currentEntries []*Entry
	currentSize := 0

	for _, e := range mergedEntries {
		entrySize := e.Size()
		if currentSize+entrySize > targetSize && len(currentEntries) > 0 {
			db.mu.Lock()
			seqNum := db.seqNum
			db.seqNum++
			db.mu.Unlock()

			filename := GenerateSSTableFilename(db.config.DataDir, nextLevel, int(seqNum))
			newSST, err := NewSSTable(filename, nextLevel, currentEntries)
			if err != nil {
				return fmt.Errorf("failed to create new SSTable: %w", err)
			}
			newTables = append(newTables, newSST)
			currentEntries = nil
			currentSize = 0
		}
		currentEntries = append(currentEntries, e)
		currentSize += entrySize
	}

	if len(currentEntries) > 0 {
		db.mu.Lock()
		seqNum := db.seqNum
		db.seqNum++
		db.mu.Unlock()

		filename := GenerateSSTableFilename(db.config.DataDir, nextLevel, int(seqNum))
		newSST, err := NewSSTable(filename, nextLevel, currentEntries)
		if err != nil {
			return fmt.Errorf("failed to create new SSTable: %w", err)
		}
		newTables = append(newTables, newSST)
	}

	currentLevelTables := tablesToMerge[:len(tablesToMerge)-len(overlapping)]
	db.levels[level].RemoveTables(currentLevelTables)
	db.levels[nextLevel].RemoveTables(overlapping)
	db.levels[nextLevel].AddTables(newTables)

	for _, t := range tablesToMerge {
		_ = t.DeleteFile()
	}

	return nil
}

func (db *DB) Close() error {
	db.mu.Lock()
	if db.closed {
		db.mu.Unlock()
		return nil
	}
	db.closed = true
	db.mu.Unlock()

	close(db.stopCh)
	db.wg.Wait()

	db.mu.Lock()
	if db.memTable.Len() > 0 {
		db.memTable.Freeze()
		db.immutable = append(db.immutable, db.memTable)
		db.memTable = NewMemTable(db.config.MemTableSize)
	}
	db.mu.Unlock()

	db.flushImmutable()

	return nil
}

func (db *DB) DebugInfo() string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	info := fmt.Sprintf("LSM Tree DB:\n")
	info += fmt.Sprintf("  MemTable: %d entries, size %d bytes\n", db.memTable.Len(), db.memTable.Size())
	info += fmt.Sprintf("  Immutable MemTables: %d\n", len(db.immutable))
	for i, mt := range db.immutable {
		info += fmt.Sprintf("    Immutable %d: %d entries\n", i, mt.Len())
	}
	if db.flushing != nil {
		info += fmt.Sprintf("  Flushing MemTable: %d entries\n", db.flushing.Len())
	}
	for _, level := range db.levels {
		info += level.DebugInfo()
	}
	return info
}

func (db *DB) SeqNum() int64 {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.seqNum
}

func (db *DB) Levels() []*Level {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.levels
}

func (db *DB) MemTable() *MemTable {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.memTable
}

func (db *DB) Immutable() []*MemTable {
	db.mu.RLock()
	defer db.mu.RUnlock()
	result := make([]*MemTable, len(db.immutable))
	copy(result, db.immutable)
	return result
}

func (db *DB) IsClosed() bool {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.closed
}
