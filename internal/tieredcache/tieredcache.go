package tieredcache

import (
	"container/list"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

const (
	EvictionPolicyLRU       EvictionPolicy = "lru"
	WritePolicyWriteThrough WritePolicy    = "write_through"
	WritePolicyWriteBack    WritePolicy    = "write_back"
	CapacityModeCount       CapacityMode   = "count"
	CapacityModeBytes       CapacityMode   = "bytes"

	maxWriteBackRetries = 3
)

var (
	ErrKeyNotFound     = errors.New("key not found")
	ErrInvalidCapacity = errors.New("invalid capacity: must be positive")
	ErrInvalidPolicy   = errors.New("invalid write policy")
	ErrNilValue        = errors.New("value cannot be nil")
	ErrEmptyKey        = errors.New("key cannot be empty")
	ErrWriteBackFailed = errors.New("write-back failed after retries")
)

type WritePolicy string
type EvictionPolicy string
type CapacityMode string

type CacheEntry struct {
	Key         string
	Value       []byte
	Size        int
	Timestamp   int64
	Dirty       bool
	FailCount   int
}

type CacheLevelConfig struct {
	Capacity       int64
	CapacityMode   CapacityMode
	EvictionPolicy EvictionPolicy
}

type Config struct {
	L1Config          CacheLevelConfig
	L2Config          CacheLevelConfig
	WritePolicy       WritePolicy
	L2Dir             string
	WriteBackInterval time.Duration
}

func DefaultConfig() Config {
	return Config{
		L1Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       100 * 1024 * 1024,
			CapacityMode:   CapacityModeBytes,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy:       WritePolicyWriteThrough,
		L2Dir:             filepath.Join(os.TempDir(), "tieredcache"),
		WriteBackInterval: 5 * time.Second,
	}
}

type lruCache struct {
	capacity       int64
	capacityMode   CapacityMode
	evictionPolicy EvictionPolicy
	items          map[string]*list.Element
	orderList      *list.List
	mu             sync.RWMutex
	count          int64
	totalBytes     int64
	onEvict        func(*CacheEntry)
	evictAsync     bool
	evictQueueMu   sync.Mutex
	evictQueue     []*CacheEntry
	evictWg        sync.WaitGroup
	closed         atomic.Bool
}

func newLRUCache(cfg CacheLevelConfig, onEvict func(*CacheEntry)) *lruCache {
	if cfg.Capacity <= 0 {
		cfg.Capacity = 1000
	}
	if cfg.CapacityMode == "" {
		cfg.CapacityMode = CapacityModeCount
	}
	if cfg.EvictionPolicy == "" {
		cfg.EvictionPolicy = EvictionPolicyLRU
	}

	c := &lruCache{
		capacity:       cfg.Capacity,
		capacityMode:   cfg.CapacityMode,
		evictionPolicy: cfg.EvictionPolicy,
		items:          make(map[string]*list.Element),
		orderList:      list.New(),
		onEvict:        onEvict,
		evictAsync:     true,
		evictQueue:     make([]*CacheEntry, 0, 16),
	}

	if onEvict != nil {
		go c.processEvictQueue()
	}

	return c
}

func (c *lruCache) processEvictQueue() {
	for !c.closed.Load() {
		c.evictQueueMu.Lock()
		if len(c.evictQueue) == 0 {
			c.evictQueueMu.Unlock()
			time.Sleep(10 * time.Millisecond)
			continue
		}
		entries := c.evictQueue
		c.evictQueue = make([]*CacheEntry, 0, 16)
		c.evictQueueMu.Unlock()

		for _, entry := range entries {
			c.onEvict(entry)
		}
		c.evictWg.Add(-len(entries))
	}
}

func (c *lruCache) enqueueEvict(entry *CacheEntry) {
	if c.onEvict == nil {
		return
	}
	c.evictWg.Add(1)
	c.evictQueueMu.Lock()
	c.evictQueue = append(c.evictQueue, entry)
	c.evictQueueMu.Unlock()
}

func (c *lruCache) waitEvictions() {
	c.evictWg.Wait()
}

func (c *lruCache) Close() {
	c.closed.Store(true)
	c.waitEvictions()
}

func (c *lruCache) get(key string) (*CacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.orderList.MoveToFront(elem)
		entry := elem.Value.(*CacheEntry)
		return entry, true
	}
	return nil, false
}

func (c *lruCache) put(key string, value []byte) *CacheEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	size := len(value)
	entry := &CacheEntry{
		Key:       key,
		Value:     value,
		Size:      size,
		Timestamp: time.Now().UnixNano(),
	}

	if elem, ok := c.items[key]; ok {
		oldEntry := elem.Value.(*CacheEntry)
		c.totalBytes -= int64(oldEntry.Size)
		elem.Value = entry
		c.orderList.MoveToFront(elem)
		c.totalBytes += int64(size)
		return entry
	}

	for c.isOverCapacity(size) {
		evicted := c.evictOneLocked()
		if evicted == nil {
			break
		}
	}

	elem := c.orderList.PushFront(entry)
	c.items[key] = elem
	c.count++
	c.totalBytes += int64(size)

	return entry
}

func (c *lruCache) putWithoutEvictCallback(key string, value []byte) (*CacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	size := len(value)
	entry := &CacheEntry{
		Key:       key,
		Value:     value,
		Size:      size,
		Timestamp: time.Now().UnixNano(),
	}

	if elem, ok := c.items[key]; ok {
		oldEntry := elem.Value.(*CacheEntry)
		c.totalBytes -= int64(oldEntry.Size)
		elem.Value = entry
		c.orderList.MoveToFront(elem)
		c.totalBytes += int64(size)
		return entry, true
	}

	if c.isOverCapacity(size) {
		return nil, false
	}

	elem := c.orderList.PushFront(entry)
	c.items[key] = elem
	c.count++
	c.totalBytes += int64(size)

	return entry, true
}

func (c *lruCache) delete(key string) bool {
	c.mu.Lock()

	if elem, ok := c.items[key]; ok {
		entry := elem.Value.(*CacheEntry)
		c.totalBytes -= int64(entry.Size)
		c.orderList.Remove(elem)
		delete(c.items, key)
		c.count--

		c.mu.Unlock()

		if c.onEvict != nil {
			c.enqueueEvict(entry)
		}
		return true
	}

	c.mu.Unlock()
	return false
}

func (c *lruCache) isOverCapacity(newSize int) bool {
	if c.capacityMode == CapacityModeCount {
		return c.count+1 > c.capacity
	}
	return c.totalBytes+int64(newSize) > c.capacity
}

func (c *lruCache) evictOneLocked() *CacheEntry {
	if c.orderList.Len() == 0 {
		return nil
	}

	elem := c.orderList.Back()
	if elem == nil {
		return nil
	}

	entry := elem.Value.(*CacheEntry)
	c.totalBytes -= int64(entry.Size)
	c.orderList.Remove(elem)
	delete(c.items, entry.Key)
	c.count--

	if c.onEvict != nil {
		c.enqueueEvict(entry)
	}

	return entry
}

func (c *lruCache) evictAll() []*CacheEntry {
	c.mu.Lock()

	entries := make([]*CacheEntry, 0, c.count)
	for elem := c.orderList.Back(); elem != nil; elem = c.orderList.Back() {
		if elem.Value == nil {
			c.orderList.Remove(elem)
			continue
		}
		entry := elem.Value.(*CacheEntry)
		entries = append(entries, entry)
		c.orderList.Remove(elem)
		delete(c.items, entry.Key)
		c.count--
		c.totalBytes -= int64(entry.Size)
	}

	c.mu.Unlock()

	if c.onEvict != nil {
		for _, entry := range entries {
			c.enqueueEvict(entry)
		}
	}

	return entries
}

func (c *lruCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.orderList = list.New()
	c.count = 0
	c.totalBytes = 0
}

func (c *lruCache) size() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.capacityMode == CapacityModeCount {
		return c.count
	}
	return c.totalBytes
}

func (c *lruCache) len() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.count
}

func (c *lruCache) getDirtyEntries() []*CacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entries := make([]*CacheEntry, 0)
	for _, elem := range c.items {
		entry := elem.Value.(*CacheEntry)
		if entry.Dirty {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (c *lruCache) clearDirty(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		entry := elem.Value.(*CacheEntry)
		entry.Dirty = false
		entry.FailCount = 0
	}
}

func (c *lruCache) incrementFailCount(key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		entry := elem.Value.(*CacheEntry)
		entry.FailCount++
		return entry.FailCount
	}
	return 0
}

func (c *lruCache) getAndRemoveDirtyLocked(key string) *CacheEntry {
	if elem, ok := c.items[key]; ok {
		entry := elem.Value.(*CacheEntry)
		entry.Dirty = false
		return entry
	}
	return nil
}

type TieredCache struct {
	l1              *lruCache
	l2              *lruCache
	l2Dir           string
	writePolicy     WritePolicy
	mu              sync.RWMutex
	writeBackTicker *time.Ticker
	writeBackStop   chan struct{}
	closed          atomic.Bool
	writeBackErrors atomic.Int64
}

func NewTieredCache() (*TieredCache, error) {
	return NewTieredCacheWithConfig(DefaultConfig())
}

func NewTieredCacheWithConfig(cfg Config) (*TieredCache, error) {
	validateConfig(&cfg)
	if err := validateConfigValues(cfg); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(cfg.L2Dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create L2 directory: %w", err)
	}

	tc := &TieredCache{
		l2Dir:         cfg.L2Dir,
		writePolicy:   cfg.WritePolicy,
		writeBackStop: make(chan struct{}),
	}

	tc.l2 = newLRUCache(cfg.L2Config, tc.handleL2Eviction)
	tc.l1 = newLRUCache(cfg.L1Config, tc.handleL1Eviction)

	if cfg.WritePolicy == WritePolicyWriteBack {
		interval := cfg.WriteBackInterval
		if interval <= 0 {
			interval = 5 * time.Second
		}
		tc.writeBackTicker = time.NewTicker(interval)
		go tc.runWriteBack()
	}

	if err := tc.loadL2FromDisk(); err != nil {
		return nil, fmt.Errorf("failed to load L2 from disk: %w", err)
	}

	return tc, nil
}

func validateConfig(cfg *Config) {
	if cfg.L1Config.EvictionPolicy == "" {
		cfg.L1Config.EvictionPolicy = EvictionPolicyLRU
	}
	if cfg.L2Config.EvictionPolicy == "" {
		cfg.L2Config.EvictionPolicy = EvictionPolicyLRU
	}
	if cfg.L1Config.CapacityMode == "" {
		cfg.L1Config.CapacityMode = CapacityModeCount
	}
	if cfg.L2Config.CapacityMode == "" {
		cfg.L2Config.CapacityMode = CapacityModeCount
	}
	if cfg.WritePolicy == "" {
		cfg.WritePolicy = WritePolicyWriteThrough
	}
}

func validateConfigValues(cfg Config) error {
	if cfg.L1Config.Capacity <= 0 {
		return fmt.Errorf("L1 %w", ErrInvalidCapacity)
	}
	if cfg.L2Config.Capacity <= 0 {
		return fmt.Errorf("L2 %w", ErrInvalidCapacity)
	}
	if cfg.WritePolicy != WritePolicyWriteThrough && cfg.WritePolicy != WritePolicyWriteBack {
		return ErrInvalidPolicy
	}
	if cfg.L1Config.EvictionPolicy != EvictionPolicyLRU {
		return fmt.Errorf("unsupported eviction policy: %s", cfg.L1Config.EvictionPolicy)
	}
	if cfg.L2Config.EvictionPolicy != EvictionPolicyLRU {
		return fmt.Errorf("unsupported eviction policy: %s", cfg.L2Config.EvictionPolicy)
	}
	return nil
}

func (tc *TieredCache) Get(key string) ([]byte, error) {
	if key == "" {
		return nil, ErrEmptyKey
	}

	tc.mu.RLock()
	defer tc.mu.RUnlock()

	if entry, ok := tc.l1.get(key); ok {
		return entry.Value, nil
	}

	if entry, ok := tc.l2.get(key); ok {
		tc.l1.put(key, entry.Value)
		return entry.Value, nil
	}

	return nil, ErrKeyNotFound
}

func (tc *TieredCache) Put(key string, value []byte) error {
	if key == "" {
		return ErrEmptyKey
	}
	if value == nil {
		return ErrNilValue
	}

	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.writePolicy == WritePolicyWriteThrough {
		entry := tc.l1.put(key, value)
		entry.Dirty = false
		if err := tc.writeToL2(key, value); err != nil {
			return err
		}
		l2Entry := tc.l2.put(key, value)
		l2Entry.Dirty = false
	} else {
		entry := tc.l1.put(key, value)
		entry.Dirty = true
		entry.FailCount = 0
	}

	return nil
}

func (tc *TieredCache) Delete(key string) error {
	if key == "" {
		return ErrEmptyKey
	}

	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.l1.clearDirty(key)
	tc.l1.delete(key)
	tc.l2.delete(key)
	tc.deleteFromL2(key)

	return nil
}

func (tc *TieredCache) handleL1Eviction(entry *CacheEntry) {
	if entry.Dirty {
		if err := tc.writeToL2(entry.Key, entry.Value); err != nil {
			tc.writeBackErrors.Add(1)
			return
		}
		l2Entry, ok := tc.l2.putWithoutEvictCallback(entry.Key, entry.Value)
		if ok {
			l2Entry.Dirty = false
		}
	}
}

func (tc *TieredCache) handleL2Eviction(entry *CacheEntry) {
	tc.deleteFromL2(entry.Key)
}

func (tc *TieredCache) writeToL2(key string, value []byte) error {
	filename := filepath.Join(tc.l2Dir, sanitizeKey(key))
	return os.WriteFile(filename, value, 0644)
}

func (tc *TieredCache) deleteFromL2(key string) {
	filename := filepath.Join(tc.l2Dir, sanitizeKey(key))
	os.Remove(filename)
}

func (tc *TieredCache) runWriteBack() {
	for {
		select {
		case <-tc.writeBackTicker.C:
			tc.flushWriteBack()
		case <-tc.writeBackStop:
			return
		}
	}
}

func (tc *TieredCache) flushWriteBack() {
	tc.mu.Lock()

	dirtyEntries := tc.l1.getDirtyEntries()

	for _, entry := range dirtyEntries {
		failCount := tc.l1.incrementFailCount(entry.Key)
		if failCount > maxWriteBackRetries {
			tc.l1.clearDirty(entry.Key)
			tc.writeBackErrors.Add(1)
			continue
		}

		if err := tc.writeToL2(entry.Key, entry.Value); err != nil {
			continue
		}

		l2Entry, ok := tc.l2.putWithoutEvictCallback(entry.Key, entry.Value)
		if !ok {
			continue
		}
		l2Entry.Dirty = false
		tc.l1.clearDirty(entry.Key)
	}

	tc.mu.Unlock()
}

func (tc *TieredCache) Flush() error {
	if tc.writePolicy == WritePolicyWriteBack {
		tc.flushWriteBack()
		errCount := tc.writeBackErrors.Load()
		if errCount > 0 {
			return fmt.Errorf("%w: %d entries permanently failed", ErrWriteBackFailed, errCount)
		}
	}
	return nil
}

func (tc *TieredCache) Close() error {
	if !tc.closed.CompareAndSwap(false, true) {
		return nil
	}

	if tc.writeBackTicker != nil {
		tc.writeBackTicker.Stop()
		close(tc.writeBackStop)
	}

	var flushErr error
	if tc.writePolicy == WritePolicyWriteBack {
		flushErr = tc.Flush()
	}

	tc.l1.waitEvictions()
	tc.l2.waitEvictions()

	tc.l1.Close()
	tc.l2.Close()

	return flushErr
}

func (tc *TieredCache) loadL2FromDisk() error {
	files, err := os.ReadDir(tc.l2Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	loadedCount := 0
	skippedCount := 0

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := filepath.Join(tc.l2Dir, file.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			continue
		}

		key := unsanitizeKey(file.Name())
		entry, ok := tc.l2.putWithoutEvictCallback(key, data)
		if ok {
			entry.Dirty = false
			loadedCount++
		} else {
			skippedCount++
		}
	}

	if skippedCount > 0 {
		_ = skippedCount
	}
	_ = loadedCount

	return nil
}

func (tc *TieredCache) L1Size() int64 {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.l1.size()
}

func (tc *TieredCache) L2Size() int64 {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.l2.size()
}

func (tc *TieredCache) L1Count() int64 {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.l1.len()
}

func (tc *TieredCache) L2Count() int64 {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.l2.len()
}

func (tc *TieredCache) ContainsL1(key string) bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	_, ok := tc.l1.get(key)
	return ok
}

func (tc *TieredCache) ContainsL2(key string) bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	_, ok := tc.l2.get(key)
	return ok
}

func (tc *TieredCache) WriteBackErrorCount() int64 {
	return tc.writeBackErrors.Load()
}

func (tc *TieredCache) Clear() error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.l1.clear()
	tc.l2.clear()

	files, err := os.ReadDir(tc.l2Dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			os.Remove(filepath.Join(tc.l2Dir, file.Name()))
		}
	}

	return nil
}

func sanitizeKey(key string) string {
	keyBytes := []byte(key)
	sanitized := make([]byte, 0, len(keyBytes)*4)
	for _, c := range keyBytes {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '.' {
			sanitized = append(sanitized, c)
		} else if c == '_' {
			sanitized = append(sanitized, '_', '_')
		} else {
			sanitized = append(sanitized, '_')
			sanitized = append(sanitized, []byte(fmt.Sprintf("%02x", c))...)
		}
	}
	return string(sanitized)
}

func unsanitizeKey(filename string) string {
	result := make([]byte, 0, len(filename))
	i := 0
	for i < len(filename) {
		if filename[i] == '_' {
			if i+1 < len(filename) && filename[i+1] == '_' {
				result = append(result, '_')
				i += 2
				continue
			}
			if i+2 < len(filename) {
				hex := filename[i+1 : i+3]
				var b byte
				if n, _ := fmt.Sscanf(hex, "%02x", &b); n == 1 {
					result = append(result, b)
					i += 3
					continue
				}
			}
		}
		result = append(result, filename[i])
		i++
	}
	return string(result)
}
