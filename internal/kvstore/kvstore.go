package kvstore

import (
	"errors"
	"hash/fnv"
	"sort"
	"sync"
)

var (
	ErrKeyNotFound   = errors.New("key not found")
	ErrEmptyBatch    = errors.New("empty batch")
	ErrInvalidRange  = errors.New("invalid range: start > end")
	ErrInvalidLimit  = errors.New("invalid limit: must be positive")
	ErrNilSnapshot   = errors.New("nil snapshot")
)

type KVStore struct {
	segments     []*segment
	segmentCount int
	bloomFilter  *BloomFilter
	bloomMu      sync.RWMutex
}

type segment struct {
	data map[string]string
	mu   sync.RWMutex
}

type Snapshot struct {
	Data map[string]string
}

type RangeResult struct {
	Items    []KVItem
	HasMore  bool
	NextKey  string
	Total    int
}

type KVItem struct {
	Key   string
	Value string
}

type Config struct {
	SegmentCount      int
	BloomCapacity     uint
	BloomFalseRate    float64
}

func DefaultConfig() Config {
	return Config{
		SegmentCount:   16,
		BloomCapacity:  10000,
		BloomFalseRate: 0.01,
	}
}

func NewKVStore() *KVStore {
	return NewKVStoreWithConfig(DefaultConfig())
}

func NewKVStoreWithConfig(cfg Config) *KVStore {
	if cfg.SegmentCount <= 0 {
		cfg.SegmentCount = 16
	}
	if cfg.BloomCapacity == 0 {
		cfg.BloomCapacity = 10000
	}
	if cfg.BloomFalseRate <= 0 || cfg.BloomFalseRate >= 1 {
		cfg.BloomFalseRate = 0.01
	}

	segments := make([]*segment, cfg.SegmentCount)
	for i := 0; i < cfg.SegmentCount; i++ {
		segments[i] = &segment{
			data: make(map[string]string),
		}
	}

	return &KVStore{
		segments:     segments,
		segmentCount: cfg.SegmentCount,
		bloomFilter:  NewBloomFilter(cfg.BloomCapacity, cfg.BloomFalseRate),
	}
}

func (kv *KVStore) getSegmentIndex(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() % uint32(kv.segmentCount))
}

func (kv *KVStore) getSegment(key string) *segment {
	return kv.segments[kv.getSegmentIndex(key)]
}

func (kv *KVStore) Put(key string, value string) {
	kv.bloomMu.Lock()
	kv.bloomFilter.Add(key)
	kv.bloomMu.Unlock()

	seg := kv.getSegment(key)
	seg.mu.Lock()
	seg.data[key] = value
	seg.mu.Unlock()
}

func (kv *KVStore) Get(key string) (string, bool) {
	kv.bloomMu.RLock()
	mayExist := kv.bloomFilter.MightContain(key)
	kv.bloomMu.RUnlock()

	if !mayExist {
		return "", false
	}

	seg := kv.getSegment(key)
	seg.mu.RLock()
	value, ok := seg.data[key]
	seg.mu.RUnlock()

	return value, ok
}

func (kv *KVStore) Delete(key string) bool {
	seg := kv.getSegment(key)
	seg.mu.Lock()
	_, exists := seg.data[key]
	if exists {
		delete(seg.data, key)
	}
	seg.mu.Unlock()

	return exists
}

func (kv *KVStore) BatchPut(pairs map[string]string) error {
	if len(pairs) == 0 {
		return ErrEmptyBatch
	}

	kv.bloomMu.Lock()
	for key := range pairs {
		kv.bloomFilter.Add(key)
	}
	kv.bloomMu.Unlock()

	segmentKeys := make(map[int][]string)
	for key := range pairs {
		idx := kv.getSegmentIndex(key)
		segmentKeys[idx] = append(segmentKeys[idx], key)
	}

	sortedIndices := make([]int, 0, len(segmentKeys))
	for idx := range segmentKeys {
		sortedIndices = append(sortedIndices, idx)
	}
	sort.Ints(sortedIndices)

	for _, idx := range sortedIndices {
		kv.segments[idx].mu.Lock()
	}

	defer func() {
		for i := len(sortedIndices) - 1; i >= 0; i-- {
			kv.segments[sortedIndices[i]].mu.Unlock()
		}
	}()

	for key, value := range pairs {
		idx := kv.getSegmentIndex(key)
		kv.segments[idx].data[key] = value
	}

	return nil
}

func (kv *KVStore) RangeScan(start, end string, limit int) (*RangeResult, error) {
	if start > end {
		return nil, ErrInvalidRange
	}
	if limit <= 0 {
		return nil, ErrInvalidLimit
	}

	var allItems []KVItem

	for i := 0; i < kv.segmentCount; i++ {
		seg := kv.segments[i]
		seg.mu.RLock()
		for key, value := range seg.data {
			if key >= start && key <= end {
				allItems = append(allItems, KVItem{Key: key, Value: value})
			}
		}
		seg.mu.RUnlock()
	}

	sort.Slice(allItems, func(i, j int) bool {
		return allItems[i].Key < allItems[j].Key
	})

	total := len(allItems)
	hasMore := total > limit
	nextKey := ""

	if hasMore {
		nextKey = allItems[limit].Key
		allItems = allItems[:limit]
	}

	return &RangeResult{
		Items:   allItems,
		HasMore: hasMore,
		NextKey: nextKey,
		Total:   total,
	}, nil
}

func (kv *KVStore) Snapshot() *Snapshot {
	data := make(map[string]string)

	for i := 0; i < kv.segmentCount; i++ {
		seg := kv.segments[i]
		seg.mu.RLock()
		for k, v := range seg.data {
			data[k] = v
		}
		seg.mu.RUnlock()
	}

	return &Snapshot{Data: data}
}

func (kv *KVStore) Restore(snapshot *Snapshot) error {
	if snapshot == nil {
		return ErrNilSnapshot
	}

	for i := 0; i < kv.segmentCount; i++ {
		kv.segments[i].mu.Lock()
	}

	defer func() {
		for i := kv.segmentCount - 1; i >= 0; i-- {
			kv.segments[i].mu.Unlock()
		}
	}()

	for i := 0; i < kv.segmentCount; i++ {
		kv.segments[i].data = make(map[string]string)
	}

	kv.bloomMu.Lock()
	kv.bloomFilter.Reset()

	for key, value := range snapshot.Data {
		idx := kv.getSegmentIndex(key)
		kv.segments[idx].data[key] = value
		kv.bloomFilter.Add(key)
	}
	kv.bloomMu.Unlock()

	return nil
}

func (kv *KVStore) Count() int {
	count := 0
	for i := 0; i < kv.segmentCount; i++ {
		seg := kv.segments[i]
		seg.mu.RLock()
		count += len(seg.data)
		seg.mu.RUnlock()
	}
	return count
}

func (s *Snapshot) Count() int {
	return len(s.Data)
}

func (s *Snapshot) Get(key string) (string, bool) {
	value, ok := s.Data[key]
	return value, ok
}
