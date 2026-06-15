package writethrough

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockStorage struct {
	mu          sync.RWMutex
	data        map[string]string
	failCount   int
	failEvery   int
	putCount    int
	getCount    int
	deleteCount int
	alwaysFail  bool
	failGet     bool
	delay       time.Duration
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		data: make(map[string]string),
	}
}

func (ms *mockStorage) Get(key string) (string, error) {
	ms.mu.Lock()
	ms.getCount++
	ms.mu.Unlock()

	if ms.failGet {
		return "", ErrStorageUnavailable
	}

	ms.mu.RLock()
	defer ms.mu.RUnlock()
	val, ok := ms.data[key]
	if !ok {
		return "", ErrKeyNotFound
	}
	return val, nil
}

func (ms *mockStorage) Put(key string, value string) error {
	if ms.delay > 0 {
		time.Sleep(ms.delay)
	}

	ms.mu.Lock()
	ms.putCount++
	cnt := ms.putCount
	ms.mu.Unlock()

	if ms.alwaysFail {
		return ErrStorageUnavailable
	}

	if ms.failEvery > 0 && cnt%ms.failEvery == 0 {
		return ErrStorageUnavailable
	}

	ms.mu.Lock()
	ms.data[key] = value
	ms.mu.Unlock()

	return nil
}

func (ms *mockStorage) Delete(key string) error {
	ms.mu.Lock()
	ms.deleteCount++
	ms.mu.Unlock()

	if ms.alwaysFail {
		return ErrStorageUnavailable
	}

	ms.mu.Lock()
	delete(ms.data, key)
	ms.mu.Unlock()

	return nil
}

func (ms *mockStorage) GetPutCount() int {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.putCount
}

func (ms *mockStorage) GetGetCount() int {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.getCount
}

func (ms *mockStorage) SetAlwaysFail(fail bool) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.alwaysFail = fail
}

func (ms *mockStorage) GetData() map[string]string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	result := make(map[string]string)
	for k, v := range ms.data {
		result[k] = v
	}
	return result
}

func TestNewWriteThroughCache(t *testing.T) {
	storage := newMockStorage()
	cache, err := NewWriteThroughCache(storage)
	if err != nil {
		t.Fatalf("NewWriteThroughCache failed: %v", err)
	}
	if cache == nil {
		t.Fatal("NewWriteThroughCache returned nil")
	}
	defer cache.Close()

	if cache.Strategy() != WriteThroughStrategy {
		t.Errorf("expected default strategy WriteThrough, got %v", cache.Strategy())
	}
}

func TestNewWriteThroughCache_NilStorage(t *testing.T) {
	_, err := NewWriteThroughCache(nil)
	if err != ErrInvalidConfig {
		t.Errorf("expected ErrInvalidConfig for nil storage, got %v", err)
	}
}

func TestNewWriteThroughCacheWithConfig_Invalid(t *testing.T) {
	storage := newMockStorage()

	testCases := []struct {
		name string
		cfg  Config
	}{
		{"negative_max_retries", Config{MaxRetries: -1, DegradeThreshold: 5, RecoverThreshold: 3}},
		{"negative_retry_interval", Config{RetryInterval: -1 * time.Second, DegradeThreshold: 5, RecoverThreshold: 3}},
		{"zero_degrade_threshold", Config{DegradeThreshold: 0, RecoverThreshold: 3}},
		{"zero_recover_threshold", Config{DegradeThreshold: 5, RecoverThreshold: 0}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewWriteThroughCacheWithConfig(storage, tc.cfg)
			if err != ErrInvalidConfig {
				t.Errorf("expected ErrInvalidConfig for %s, got %v", tc.name, err)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("expected default MaxRetries 3, got %d", cfg.MaxRetries)
	}
	if cfg.RetryInterval != 100*time.Millisecond {
		t.Errorf("expected default RetryInterval 100ms, got %v", cfg.RetryInterval)
	}
	if cfg.DegradeThreshold != 5 {
		t.Errorf("expected default DegradeThreshold 5, got %d", cfg.DegradeThreshold)
	}
	if cfg.RecoverThreshold != 3 {
		t.Errorf("expected default RecoverThreshold 3, got %d", cfg.RecoverThreshold)
	}
	if cfg.RecoverWindow != 5*time.Second {
		t.Errorf("expected default RecoverWindow 5s, got %v", cfg.RecoverWindow)
	}
	if !cfg.EnableReadThrough {
		t.Error("expected default EnableReadThrough true")
	}
}

func TestWriteStrategy_String(t *testing.T) {
	testCases := []struct {
		strategy WriteStrategy
		expected string
	}{
		{WriteThroughStrategy, "WriteThrough"},
		{WriteAroundStrategy, "WriteAround"},
		{WriteStrategy(99), "Unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			if tc.strategy.String() != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, tc.strategy.String())
			}
		})
	}
}

func TestPutAndGet_WriteThrough(t *testing.T) {
	storage := newMockStorage()
	cache, err := NewWriteThroughCache(storage)
	if err != nil {
		t.Fatalf("NewWriteThroughCache failed: %v", err)
	}
	defer cache.Close()

	err = cache.Put("key1", "value1")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	val, err := cache.Get("key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "value1" {
		t.Errorf("expected value1, got %s", val)
	}

	storageData := storage.GetData()
	if storageData["key1"] != "value1" {
		t.Error("storage should have the data after write-through")
	}

	if storage.GetPutCount() != 1 {
		t.Errorf("expected 1 storage Put call, got %d", storage.GetPutCount())
	}
}

func TestGet_CacheHit(t *testing.T) {
	storage := newMockStorage()
	cache, err := NewWriteThroughCache(storage)
	if err != nil {
		t.Fatalf("NewWriteThroughCache failed: %v", err)
	}
	defer cache.Close()

	err = cache.Put("key1", "value1")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	storageGetBefore := storage.GetGetCount()

	val, err := cache.Get("key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "value1" {
		t.Errorf("expected value1, got %s", val)
	}

	storageGetAfter := storage.GetGetCount()
	if storageGetAfter != storageGetBefore {
		t.Error("cache hit should not call storage.Get")
	}
}

func TestGet_KeyNotFound(t *testing.T) {
	storage := newMockStorage()
	cache, err := NewWriteThroughCache(storage)
	if err != nil {
		t.Fatalf("NewWriteThroughCache failed: %v", err)
	}
	defer cache.Close()

	_, err = cache.Get("nonexistent")
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestGet_ReadThroughCacheBackfill(t *testing.T) {
	storage := newMockStorage()
	storage.Put("key1", "value1")

	cfg := Config{
		MaxRetries:       3,
		RetryInterval:    10 * time.Millisecond,
		DegradeThreshold: 5,
		RecoverThreshold: 3,
		RecoverWindow:    5 * time.Second,
		EnableReadThrough: true,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	val, err := cache.Get("key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "value1" {
		t.Errorf("expected value1, got %s", val)
	}

	if storage.GetGetCount() != 1 {
		t.Errorf("expected 1 storage Get call for cache miss, got %d", storage.GetGetCount())
	}

	val, err = cache.Get("key1")
	if err != nil {
		t.Fatalf("second Get failed: %v", err)
	}
	if val != "value1" {
		t.Errorf("expected value1 on second get, got %s", val)
	}

	if storage.GetGetCount() != 1 {
		t.Errorf("expected 1 storage Get call after cache hit, got %d (data should be backfilled)", storage.GetGetCount())
	}
}

func TestGet_DisabledReadThrough(t *testing.T) {
	storage := newMockStorage()
	storage.Put("key1", "value1")

	cfg := Config{
		MaxRetries:        3,
		RetryInterval:     10 * time.Millisecond,
		DegradeThreshold:  5,
		RecoverThreshold:  3,
		RecoverWindow:     5 * time.Second,
		EnableReadThrough: false,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	_, err = cache.Get("key1")
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound with disabled read-through, got %v", err)
	}
}

func TestGet_StorageError(t *testing.T) {
	storage := newMockStorage()
	storage.failGet = true

	cache, err := NewWriteThroughCache(storage)
	if err != nil {
		t.Fatalf("NewWriteThroughCache failed: %v", err)
	}
	defer cache.Close()

	_, err = cache.Get("key1")
	if err == nil {
		t.Error("expected error when storage fails on Get")
	}
	if !errors.Is(err, ErrStorageUnavailable) {
		t.Errorf("expected ErrStorageUnavailable, got %v", err)
	}
}

func TestPut_RetryOnFailure(t *testing.T) {
	storage := newMockStorage()
	storage.alwaysFail = true

	cfg := Config{
		MaxRetries:       2,
		RetryInterval:    5 * time.Millisecond,
		DegradeThreshold: 10,
		RecoverThreshold: 3,
		RecoverWindow:    5 * time.Second,
		EnableReadThrough: true,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	err = cache.Put("key1", "value1")
	if err == nil {
		t.Error("expected error after max retries exhausted")
	}

	expectedCalls := cfg.MaxRetries + 1
	actualCalls := storage.GetPutCount()
	if actualCalls != expectedCalls {
		t.Errorf("expected %d storage Put calls (1 initial + %d retries), got %d",
			expectedCalls, cfg.MaxRetries, actualCalls)
	}

	if cache.FailureCount() < 1 {
		t.Error("failure count should be incremented")
	}
}

func TestPut_PendingItems(t *testing.T) {
	storage := newMockStorage()
	storage.alwaysFail = true

	cfg := Config{
		MaxRetries:       0,
		RetryInterval:    10 * time.Millisecond,
		DegradeThreshold: 100,
		RecoverThreshold: 3,
		RecoverWindow:    5 * time.Second,
		EnableReadThrough: true,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	err = cache.Put("key1", "value1")
	if err == nil {
		t.Error("expected error")
	}

	if cache.PendingCount() != 1 {
		t.Errorf("expected 1 pending item, got %d", cache.PendingCount())
	}

	err = cache.Put("key1", "value2")
	if err == nil {
		t.Error("expected error on second put")
	}

	if cache.PendingCount() != 1 {
		t.Errorf("expected still 1 pending item (same key should update), got %d", cache.PendingCount())
	}
}

func TestDegradeToWriteAround(t *testing.T) {
	storage := newMockStorage()
	storage.alwaysFail = true

	cfg := Config{
		MaxRetries:       0,
		RetryInterval:    1 * time.Millisecond,
		DegradeThreshold: 3,
		RecoverThreshold: 2,
		RecoverWindow:    5 * time.Second,
		EnableReadThrough: true,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	for i := 0; i < cfg.DegradeThreshold; i++ {
		cache.Put(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
	}

	if cache.Strategy() != WriteAroundStrategy {
		t.Errorf("expected strategy to degrade to WriteAround after %d failures, got %v",
			cfg.DegradeThreshold, cache.Strategy())
	}
}

func TestRecoverToWriteThrough(t *testing.T) {
	storage := newMockStorage()

	cfg := Config{
		MaxRetries:       0,
		RetryInterval:    1 * time.Millisecond,
		DegradeThreshold: 2,
		RecoverThreshold: 3,
		RecoverWindow:    5 * time.Second,
		EnableReadThrough: true,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	storage.alwaysFail = true
	for i := 0; i < cfg.DegradeThreshold; i++ {
		cache.Put(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
	}
	if cache.Strategy() != WriteAroundStrategy {
		t.Fatal("expected degraded state")
	}

	storage.alwaysFail = false
	for i := 0; i < cfg.RecoverThreshold; i++ {
		err := cache.Put(fmt.Sprintf("recover_key%d", i), fmt.Sprintf("value%d", i))
		if err != nil {
			t.Fatalf("Put %d failed: %v", i, err)
		}
	}

	if cache.Strategy() != WriteThroughStrategy {
		t.Errorf("expected strategy to recover to WriteThrough after %d successes, got %v",
			cfg.RecoverThreshold, cache.Strategy())
	}
}

func TestWriteAround_PutOnlyUpdatesStorage(t *testing.T) {
	storage := newMockStorage()

	cfg := Config{
		MaxRetries:       0,
		RetryInterval:    1 * time.Millisecond,
		DegradeThreshold: 1,
		RecoverThreshold: 100,
		RecoverWindow:    5 * time.Second,
		EnableReadThrough: false,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	storage.alwaysFail = true
	cache.Put("fail_key", "fail_value")
	if cache.Strategy() != WriteAroundStrategy {
		t.Fatal("expected degraded to WriteAround")
	}
	storage.alwaysFail = false

	err = cache.Put("key1", "value1")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	storageData := storage.GetData()
	if storageData["key1"] != "value1" {
		t.Error("storage should have the data in WriteAround mode")
	}

	_, ok := cache.cache.Get("key1")
	if ok {
		t.Error("cache should not have the data directly after Put in WriteAround mode")
	}
}

func TestDelete_WriteThrough(t *testing.T) {
	storage := newMockStorage()
	cache, err := NewWriteThroughCache(storage)
	if err != nil {
		t.Fatalf("NewWriteThroughCache failed: %v", err)
	}
	defer cache.Close()

	cache.Put("key1", "value1")

	err = cache.Delete("key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = cache.Get("key1")
	if err != ErrKeyNotFound {
		t.Error("key should be deleted from cache")
	}

	storageData := storage.GetData()
	if _, ok := storageData["key1"]; ok {
		t.Error("key should be deleted from storage")
	}
}

func TestDelete_WriteThrough_StorageFail(t *testing.T) {
	storage := newMockStorage()

	cfg := Config{
		MaxRetries:        0,
		RetryInterval:     1 * time.Millisecond,
		DegradeThreshold:  100,
		RecoverThreshold:  3,
		RecoverWindow:     5 * time.Second,
		EnableReadThrough: false,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	cache.Put("key1", "value1")

	storage.alwaysFail = true
	err = cache.Delete("key1")
	if err == nil {
		t.Error("expected error when storage Delete fails")
	}

	storage.alwaysFail = false
	storageData := storage.GetData()
	if _, ok := storageData["key1"]; !ok {
		t.Error("data should still be in storage since delete failed")
	}

	val, err := cache.Get("key1")
	if err != nil {
		t.Fatalf("cache should still have the data after storage delete failed, got error: %v", err)
	}
	if val != "value1" {
		t.Errorf("cache should have value1, got %s", val)
	}
}

func TestDelete_Consistency_FirstDeleteStorage(t *testing.T) {
	storage := newMockStorage()

	cfg := Config{
		MaxRetries:        0,
		RetryInterval:     1 * time.Millisecond,
		DegradeThreshold:  100,
		RecoverThreshold:  3,
		RecoverWindow:     5 * time.Second,
		EnableReadThrough: false,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	cache.Put("key1", "value1")

	storage.alwaysFail = true
	err = cache.Delete("key1")
	if err == nil {
		t.Fatal("expected error when storage Delete fails")
	}

	_, ok := cache.cache.Get("key1")
	if !ok {
		t.Error("cache should still have the data because storage delete failed (cache is not deleted)")
	}

	storage.alwaysFail = false

	err = cache.Delete("key1")
	if err != nil {
		t.Fatalf("Delete should succeed now, got: %v", err)
	}

	_, ok = cache.cache.Get("key1")
	if ok {
		t.Error("cache should be deleted after successful storage delete")
	}

	storageData := storage.GetData()
	if _, ok := storageData["key1"]; ok {
		t.Error("storage should have the data deleted")
	}
}

func TestDelete_WriteAround(t *testing.T) {
	storage := newMockStorage()

	cfg := Config{
		MaxRetries:       0,
		RetryInterval:    1 * time.Millisecond,
		DegradeThreshold: 1,
		RecoverThreshold: 100,
		RecoverWindow:    5 * time.Second,
		EnableReadThrough: true,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	storage.alwaysFail = true
	cache.Put("key1", "value1")
	storage.alwaysFail = false

	storage.Put("key1", "value1")

	cache.cache.Put("key1", "value1")

	err = cache.Delete("key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	storageData := storage.GetData()
	if _, ok := storageData["key1"]; ok {
		t.Error("key should be deleted from storage in WriteAround mode")
	}

	_, ok := cache.cache.Get("key1")
	if ok {
		t.Error("key should be deleted from cache in WriteAround mode")
	}
}

func TestBackgroundRetry_Success(t *testing.T) {
	storage := newMockStorage()
	storage.alwaysFail = true

	cfg := Config{
		MaxRetries:       3,
		RetryInterval:    10 * time.Millisecond,
		DegradeThreshold: 100,
		RecoverThreshold: 3,
		RecoverWindow:    5 * time.Second,
		EnableReadThrough: true,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	cache.Put("key1", "value1")
	if cache.PendingCount() != 1 {
		t.Fatalf("expected 1 pending item, got %d", cache.PendingCount())
	}

	storage.alwaysFail = false

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cache.PendingCount() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if cache.PendingCount() != 0 {
		t.Errorf("expected pending items to be retried successfully, still %d pending", cache.PendingCount())
	}

	storageData := storage.GetData()
	if storageData["key1"] != "value1" {
		t.Error("storage should have the data after background retry")
	}

	cachedVal, ok := cache.cache.Get("key1")
	if !ok || cachedVal != "value1" {
		t.Error("cache should have the data after successful retry")
	}
}

func TestBackgroundRetry_LimitExceeded(t *testing.T) {
	storage := newMockStorage()
	storage.alwaysFail = true

	cfg := Config{
		MaxRetries:       1,
		RetryInterval:    10 * time.Millisecond,
		DegradeThreshold: 100,
		RecoverThreshold: 3,
		RecoverWindow:    5 * time.Second,
		EnableReadThrough: true,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	cache.Put("key1", "value1")

	time.Sleep(100 * time.Millisecond)

	if cache.PendingCount() != 0 {
		t.Errorf("expected pending item to be dropped after max retries, got %d pending", cache.PendingCount())
	}
}

func TestConcurrentPut(t *testing.T) {
	storage := newMockStorage()
	cache, err := NewWriteThroughCache(storage)
	if err != nil {
		t.Fatalf("NewWriteThroughCache failed: %v", err)
	}
	defer cache.Close()

	var wg sync.WaitGroup
	numGoroutines := 20
	numOps := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numOps; i++ {
				key := fmt.Sprintf("g%d_k%d", id, i)
				val := fmt.Sprintf("v%d_%d", id, i)
				err := cache.Put(key, val)
				if err != nil {
					t.Errorf("Put failed for key %s: %v", key, err)
				}
			}
		}(g)
	}

	wg.Wait()

	expected := numGoroutines * numOps
	actualPuts := storage.GetPutCount()
	if actualPuts != expected {
		t.Errorf("expected %d storage puts, got %d", expected, actualPuts)
	}
}

func TestConcurrentGet(t *testing.T) {
	storage := newMockStorage()
	cache, err := NewWriteThroughCache(storage)
	if err != nil {
		t.Fatalf("NewWriteThroughCache failed: %v", err)
	}
	defer cache.Close()

	numKeys := 100
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key%d", i)
		val := fmt.Sprintf("val%d", i)
		cache.Put(key, val)
	}

	var wg sync.WaitGroup
	numReaders := 20
	iterations := 100
	var errors atomic.Int64

	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				keyIdx := i % numKeys
				key := fmt.Sprintf("key%d", keyIdx)
				expectedVal := fmt.Sprintf("val%d", keyIdx)
				val, err := cache.Get(key)
				if err != nil {
					errors.Add(1)
					t.Errorf("Get failed for key %s: %v", key, err)
					continue
				}
				if val != expectedVal {
					errors.Add(1)
					t.Errorf("value mismatch for key %s: expected %s, got %s", key, expectedVal, val)
				}
			}
		}()
	}

	wg.Wait()

	if errors.Load() > 0 {
		t.Errorf("got %d errors during concurrent gets", errors.Load())
	}
}

func TestConcurrentPutAndGet(t *testing.T) {
	storage := newMockStorage()
	cache, err := NewWriteThroughCache(storage)
	if err != nil {
		t.Fatalf("NewWriteThroughCache failed: %v", err)
	}
	defer cache.Close()

	numKeys := 50
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key%d", i)
		val := fmt.Sprintf("val%d", i)
		cache.Put(key, val)
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			key := fmt.Sprintf("key%d", i%numKeys)
			val := fmt.Sprintf("updated_%d", i)
			cache.Put(key, val)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			key := fmt.Sprintf("key%d", i%numKeys)
			_, err := cache.Get(key)
			if err != nil {
				t.Errorf("Get failed for %s: %v", key, err)
			}
		}
	}()

	wg.Wait()
}

func TestClose(t *testing.T) {
	storage := newMockStorage()
	cache, err := NewWriteThroughCache(storage)
	if err != nil {
		t.Fatalf("NewWriteThroughCache failed: %v", err)
	}

	cache.Close()
	cache.Close()
}

func TestFailureCount(t *testing.T) {
	storage := newMockStorage()
	storage.alwaysFail = true

	cfg := Config{
		MaxRetries:        0,
		RetryInterval:     1 * time.Millisecond,
		DegradeThreshold:  100,
		RecoverThreshold:  3,
		RecoverWindow:     5 * time.Second,
		EnableReadThrough: true,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	cache.Put("key1", "value1")
	if cache.FailureCount() < 1 {
		t.Errorf("expected failure count >= 1, got %d", cache.FailureCount())
	}

	storage.alwaysFail = false
	cache.Put("key2", "value2")
	if cache.FailureCount() != 0 {
		t.Errorf("expected failure count to reset to 0 after Put success, got %d", cache.FailureCount())
	}
}

func TestDelete_Failure_NotAffectDegradeCounter(t *testing.T) {
	storage := newMockStorage()

	cfg := Config{
		MaxRetries:        0,
		RetryInterval:     1 * time.Millisecond,
		DegradeThreshold:  3,
		RecoverThreshold:  3,
		RecoverWindow:     5 * time.Second,
		EnableReadThrough: false,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	cache.Put("key1", "value1")

	if cache.FailureCount() != 0 {
		t.Fatalf("expected failure count 0 initially, got %d", cache.FailureCount())
	}

	storage.alwaysFail = true
	for i := 0; i < 10; i++ {
		cache.Delete("key1")
	}

	if cache.FailureCount() != 0 {
		t.Errorf("expected failure count to remain 0 after Delete failures, got %d", cache.FailureCount())
	}

	if cache.Strategy() != WriteThroughStrategy {
		t.Errorf("expected WriteThrough strategy, Delete failures should not cause degradation, got %v", cache.Strategy())
	}
}

func TestPut_FailureInterruptedByBackgroundSuccess(t *testing.T) {
	storage := newMockStorage()

	cfg := Config{
		MaxRetries:        0,
		RetryInterval:     50 * time.Millisecond,
		DegradeThreshold:  5,
		RecoverThreshold:  3,
		RecoverWindow:     5 * time.Second,
		EnableReadThrough: false,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	storage.alwaysFail = true

	for i := 0; i < 3; i++ {
		cache.Put(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
	}

	if cache.FailureCount() != 3 {
		t.Fatalf("expected 3 failures, got %d", cache.FailureCount())
	}

	storage.alwaysFail = false
	time.Sleep(150 * time.Millisecond)

	pending := cache.PendingCount()
	if pending != 0 {
		t.Logf("pending items: %d (should be retried)", pending)
	}

	if cache.FailureCount() != 3 {
		t.Errorf("expected failure count to remain 3 after background retry success, got %d", cache.FailureCount())
	}

	storage.alwaysFail = true
	for i := 3; i < 5; i++ {
		cache.Put(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
	}

	if cache.FailureCount() >= int64(cfg.DegradeThreshold) {
		t.Logf("failure count: %d, threshold: %d, strategy: %v",
			cache.FailureCount(), cfg.DegradeThreshold, cache.Strategy())
	}
}

func TestPut_FailureAccumulationAndDegrade(t *testing.T) {
	storage := newMockStorage()

	cfg := Config{
		MaxRetries:        0,
		RetryInterval:     1 * time.Millisecond,
		DegradeThreshold:  3,
		RecoverThreshold:  3,
		RecoverWindow:     5 * time.Second,
		EnableReadThrough: false,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	storage.alwaysFail = true

	cache.Put("key1", "value1")
	if cache.FailureCount() != 1 {
		t.Errorf("expected 1 failure after first Put failure, got %d", cache.FailureCount())
	}
	if cache.Strategy() != WriteThroughStrategy {
		t.Error("should still be WriteThrough after 1 failure")
	}

	storage.alwaysFail = false
	cache.Delete("key1")

	storage.alwaysFail = true
	cache.Put("key2", "value2")
	if cache.FailureCount() != 2 {
		t.Errorf("expected 2 failures, Delete success should not reset Put failure count, got %d", cache.FailureCount())
	}

	cache.Put("key3", "value3")
	if cache.FailureCount() != 3 {
		t.Errorf("expected 3 failures, got %d", cache.FailureCount())
	}
	if cache.Strategy() != WriteAroundStrategy {
		t.Errorf("expected degraded to WriteAround after %d failures, got %v", cfg.DegradeThreshold, cache.Strategy())
	}
}

func TestPut_UpdateExistingKey(t *testing.T) {
	storage := newMockStorage()
	cache, err := NewWriteThroughCache(storage)
	if err != nil {
		t.Fatalf("NewWriteThroughCache failed: %v", err)
	}
	defer cache.Close()

	cache.Put("key1", "old_value")
	cache.Put("key1", "new_value")

	val, err := cache.Get("key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "new_value" {
		t.Errorf("expected new_value, got %s", val)
	}

	storageData := storage.GetData()
	if storageData["key1"] != "new_value" {
		t.Errorf("storage should have new_value, got %s", storageData["key1"])
	}
}

func TestErrors_Variables(t *testing.T) {
	if ErrKeyNotFound == nil {
		t.Error("ErrKeyNotFound should not be nil")
	}
	if ErrStorageUnavailable == nil {
		t.Error("ErrStorageUnavailable should not be nil")
	}
	if ErrInvalidConfig == nil {
		t.Error("ErrInvalidConfig should not be nil")
	}
}

func TestReadThrough_StorageUnavailable(t *testing.T) {
	storage := newMockStorage()
	storage.failGet = true

	cache, err := NewWriteThroughCache(storage)
	if err != nil {
		t.Fatalf("NewWriteThroughCache failed: %v", err)
	}
	defer cache.Close()

	_, err = cache.Get("key1")
	if err == nil {
		t.Error("expected error from storage")
	}
	if !errors.Is(err, ErrStorageUnavailable) {
		t.Errorf("expected ErrStorageUnavailable, got %v", err)
	}
}

func TestWriteAround_StorageFail(t *testing.T) {
	storage := newMockStorage()
	storage.alwaysFail = true

	cfg := Config{
		MaxRetries:       0,
		RetryInterval:    1 * time.Millisecond,
		DegradeThreshold: 1,
		RecoverThreshold: 100,
		RecoverWindow:    5 * time.Second,
		EnableReadThrough: true,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	cache.Put("key1", "value1")
	if cache.Strategy() != WriteAroundStrategy {
		t.Fatal("expected WriteAround strategy")
	}

	err = cache.Put("key2", "value2")
	if err == nil {
		t.Error("expected error when storage fails in WriteAround mode")
	}
}

func TestPendingItems_MultipleKeys(t *testing.T) {
	storage := newMockStorage()
	storage.alwaysFail = true

	cfg := Config{
		MaxRetries:       0,
		RetryInterval:    10 * time.Millisecond,
		DegradeThreshold: 100,
		RecoverThreshold: 3,
		RecoverWindow:    5 * time.Second,
		EnableReadThrough: true,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	numKeys := 10
	for i := 0; i < numKeys; i++ {
		cache.Put(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
	}

	if cache.PendingCount() != numKeys {
		t.Errorf("expected %d pending items, got %d", numKeys, cache.PendingCount())
	}
}

func TestStrategy_NoChangeWhenAlreadyInState(t *testing.T) {
	storage := newMockStorage()
	cache, err := NewWriteThroughCache(storage)
	if err != nil {
		t.Fatalf("NewWriteThroughCache failed: %v", err)
	}
	defer cache.Close()

	if cache.Strategy() != WriteThroughStrategy {
		t.Fatal("expected initial WriteThrough strategy")
	}

	if cache.shouldDegrade() {
		t.Error("should not degrade when already degraded (no, this checks shouldDegrade)")
	}

	cache.degradeToWriteAround()
	if cache.Strategy() != WriteAroundStrategy {
		t.Fatal("expected WriteAround strategy after degrade")
	}

	if cache.shouldDegrade() {
		t.Error("shouldDegrade should return false when already in WriteAround mode")
	}

	cache.recoverToWriteThrough()
	if cache.Strategy() != WriteThroughStrategy {
		t.Fatal("expected WriteThrough strategy after recover")
	}
}

func TestRecovery_WindowExpiry(t *testing.T) {
	storage := newMockStorage()

	cfg := Config{
		MaxRetries:       0,
		RetryInterval:    1 * time.Second,
		DegradeThreshold: 2,
		RecoverThreshold: 3,
		RecoverWindow:    50 * time.Millisecond,
		EnableReadThrough: false,
	}
	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	cache.degradeToWriteAround()
	if cache.Strategy() != WriteAroundStrategy {
		t.Fatal("expected degraded state")
	}

	cache.Put("k1", "v1")
	cache.Put("k2", "v2")

	cache.recoveryMu.Lock()
	beforeCount := len(cache.recoverySuccess)
	cache.recoveryMu.Unlock()
	if beforeCount != 2 {
		t.Fatalf("expected 2 recovery success records before sleep, got %d", beforeCount)
	}

	time.Sleep(100 * time.Millisecond)

	cache.Put("k3", "v3")

	cache.recoveryMu.Lock()
	afterCount := len(cache.recoverySuccess)
	cache.recoveryMu.Unlock()

	if afterCount >= 3 {
		t.Errorf("expected less than 3 recovery success records after window expiry, got %d", afterCount)
	}

	if cache.Strategy() != WriteAroundStrategy {
		t.Errorf("should not recover because first two successes are outside the recovery window, got %v records", afterCount)
	}
}

func TestCustomConfig(t *testing.T) {
	storage := newMockStorage()

	cfg := Config{
		MaxRetries:       5,
		RetryInterval:    50 * time.Millisecond,
		DegradeThreshold: 10,
		RecoverThreshold: 5,
		RecoverWindow:    10 * time.Second,
		EnableReadThrough: false,
	}

	cache, err := NewWriteThroughCacheWithConfig(storage, cfg)
	if err != nil {
		t.Fatalf("NewWriteThroughCacheWithConfig failed: %v", err)
	}
	defer cache.Close()

	if cache.config.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", cache.config.MaxRetries)
	}
	if cache.config.EnableReadThrough {
		t.Error("expected EnableReadThrough to be false")
	}
}
