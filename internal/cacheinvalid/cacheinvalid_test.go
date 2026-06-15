package cacheinvalid

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewCacheInvalidManager(t *testing.T) {
	mgr := NewCacheInvalidManager()
	if mgr == nil {
		t.Fatal("NewCacheInvalidManager returned nil")
	}
	if mgr.Count() != 0 {
		t.Errorf("expected 0 entries, got %d", mgr.Count())
	}
}

func TestNewCacheInvalidManagerWithConfig(t *testing.T) {
	cfg := Config{
		DefaultTTL:      10 * time.Second,
		MaxEntries:      100,
		HotAccessThreshold: 50,
		PreloadSize:     50,
	}
	mgr := NewCacheInvalidManagerWithConfig(cfg)
	if mgr == nil {
		t.Fatal("NewCacheInvalidManagerWithConfig returned nil")
	}
	if mgr.config.DefaultTTL != 10*time.Second {
		t.Errorf("expected DefaultTTL 10s, got %v", mgr.config.DefaultTTL)
	}
	if mgr.config.MaxEntries != 100 {
		t.Errorf("expected MaxEntries 100, got %d", mgr.config.MaxEntries)
	}
}

func TestNewCacheInvalidManagerWithInvalidConfig(t *testing.T) {
	cfg := Config{
		DefaultTTL:      -1 * time.Second,
		MaxEntries:      -1,
		HotAccessThreshold: -1,
		PreloadSize:     -1,
	}
	mgr := NewCacheInvalidManagerWithConfig(cfg)
	if mgr.config.DefaultTTL <= 0 {
		t.Errorf("expected positive DefaultTTL, got %v", mgr.config.DefaultTTL)
	}
	if mgr.config.MaxEntries <= 0 {
		t.Errorf("expected positive MaxEntries, got %d", mgr.config.MaxEntries)
	}
	if mgr.config.HotAccessThreshold <= 0 {
		t.Errorf("expected positive HotAccessThreshold, got %d", mgr.config.HotAccessThreshold)
	}
	if mgr.config.PreloadSize < 0 {
		t.Errorf("expected non-negative PreloadSize, got %d", mgr.config.PreloadSize)
	}
}

func TestPutAndGet(t *testing.T) {
	mgr := NewCacheInvalidManager()

	err := mgr.Put("key1", "value1")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	value, ok := mgr.Get("key1")
	if !ok {
		t.Fatal("expected to find key1")
	}
	if value != "value1" {
		t.Errorf("expected value 'value1', got '%v'", value)
	}
}

func TestPutOverwrite(t *testing.T) {
	mgr := NewCacheInvalidManager()

	mgr.Put("key1", "value1")
	mgr.Put("key1", "value2")

	value, ok := mgr.Get("key1")
	if !ok {
		t.Fatal("expected to find key1")
	}
	if value != "value2" {
		t.Errorf("expected value 'value2', got '%v'", value)
	}
}

func TestGetNonExistent(t *testing.T) {
	mgr := NewCacheInvalidManager()

	_, ok := mgr.Get("nonexistent")
	if ok {
		t.Error("expected false for non-existent key")
	}
}

func TestPutWithTTL(t *testing.T) {
	mgr := NewCacheInvalidManager()

	err := mgr.PutWithTTL("key1", "value1", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("PutWithTTL failed: %v", err)
	}

	value, ok := mgr.Get("key1")
	if !ok {
		t.Fatal("expected to find key1")
	}
	if value != "value1" {
		t.Errorf("expected value 'value1', got '%v'", value)
	}
}

func TestPutWithInvalidTTL(t *testing.T) {
	mgr := NewCacheInvalidManager()

	err := mgr.PutWithTTL("key1", "value1", -1*time.Second)
	if !errors.Is(err, ErrInvalidTTL) {
		t.Errorf("expected ErrInvalidTTL, got %v", err)
	}
}

func TestTTLLazyExpiration(t *testing.T) {
	mgr := NewCacheInvalidManager()

	mgr.PutWithTTL("key1", "value1", 50*time.Millisecond)

	_, ok := mgr.Get("key1")
	if !ok {
		t.Fatal("expected key1 to exist before expiration")
	}

	time.Sleep(100 * time.Millisecond)

	_, ok = mgr.Get("key1")
	if ok {
		t.Error("expected key1 to be expired and removed")
	}

	if mgr.Count() != 0 {
		t.Errorf("expected 0 entries after lazy expiration, got %d", mgr.Count())
	}
}

func TestIsExpired(t *testing.T) {
	mgr := NewCacheInvalidManager()

	mgr.PutWithTTL("key1", "value1", 50*time.Millisecond)

	expired, err := mgr.IsExpired("key1")
	if err != nil {
		t.Fatalf("IsExpired failed: %v", err)
	}
	if expired {
		t.Error("expected key1 to not be expired yet")
	}

	time.Sleep(100 * time.Millisecond)

	expired, err = mgr.IsExpired("key1")
	if err != nil {
		t.Fatalf("IsExpired failed: %v", err)
	}
	if !expired {
		t.Error("expected key1 to be expired")
	}
}

func TestIsExpiredNonExistent(t *testing.T) {
	mgr := NewCacheInvalidManager()

	_, err := mgr.IsExpired("nonexistent")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	mgr := NewCacheInvalidManager()

	mgr.Put("key1", "value1")

	deleted := mgr.Delete("key1")
	if !deleted {
		t.Error("expected Delete to return true for existing key")
	}

	_, ok := mgr.Get("key1")
	if ok {
		t.Error("expected key1 to be deleted")
	}

	if mgr.Count() != 0 {
		t.Errorf("expected 0 entries after delete, got %d", mgr.Count())
	}
}

func TestDeleteNonExistent(t *testing.T) {
	mgr := NewCacheInvalidManager()

	deleted := mgr.Delete("nonexistent")
	if deleted {
		t.Error("expected Delete to return false for non-existent key")
	}
}

func TestClear(t *testing.T) {
	mgr := NewCacheInvalidManager()

	mgr.Put("key1", "value1")
	mgr.Put("key2", "value2")
	mgr.Put("key3", "value3")

	if mgr.Count() != 3 {
		t.Fatalf("expected 3 entries before clear, got %d", mgr.Count())
	}

	mgr.Clear()

	if mgr.Count() != 0 {
		t.Errorf("expected 0 entries after clear, got %d", mgr.Count())
	}
}

func TestCount(t *testing.T) {
	mgr := NewCacheInvalidManager()

	if mgr.Count() != 0 {
		t.Errorf("expected 0 entries, got %d", mgr.Count())
	}

	mgr.Put("key1", "value1")
	if mgr.Count() != 1 {
		t.Errorf("expected 1 entry, got %d", mgr.Count())
	}

	mgr.Put("key2", "value2")
	if mgr.Count() != 2 {
		t.Errorf("expected 2 entries, got %d", mgr.Count())
	}
}

func TestMarkHot(t *testing.T) {
	mgr := NewCacheInvalidManager()

	mgr.Put("key1", "value1")

	err := mgr.MarkHot("key1")
	if err != nil {
		t.Fatalf("MarkHot failed: %v", err)
	}

	isHot, err := mgr.IsHot("key1")
	if err != nil {
		t.Fatalf("IsHot failed: %v", err)
	}
	if !isHot {
		t.Error("expected key1 to be hot")
	}
}

func TestMarkHotNonExistent(t *testing.T) {
	mgr := NewCacheInvalidManager()

	err := mgr.MarkHot("nonexistent")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestUnmarkHot(t *testing.T) {
	mgr := NewCacheInvalidManager()

	mgr.Put("key1", "value1")
	mgr.MarkHot("key1")

	err := mgr.UnmarkHot("key1")
	if err != nil {
		t.Fatalf("UnmarkHot failed: %v", err)
	}

	isHot, err := mgr.IsHot("key1")
	if err != nil {
		t.Fatalf("IsHot failed: %v", err)
	}
	if isHot {
		t.Error("expected key1 to not be hot after unmark")
	}
}

func TestUnmarkHotNonExistent(t *testing.T) {
	mgr := NewCacheInvalidManager()

	err := mgr.UnmarkHot("nonexistent")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestIsHotNonExistent(t *testing.T) {
	mgr := NewCacheInvalidManager()

	_, err := mgr.IsHot("nonexistent")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestHotDataNeverExpires(t *testing.T) {
	mgr := NewCacheInvalidManager()

	mgr.PutWithTTL("key1", "value1", 50*time.Millisecond)
	mgr.MarkHot("key1")

	time.Sleep(100 * time.Millisecond)

	value, ok := mgr.Get("key1")
	if !ok {
		t.Fatal("expected hot key1 to still exist after TTL expired")
	}
	if value != "value1" {
		t.Errorf("expected value 'value1', got '%v'", value)
	}

	expired, err := mgr.IsExpired("key1")
	if err != nil {
		t.Fatalf("IsExpired failed: %v", err)
	}
	if expired {
		t.Error("expected hot key1 to not be expired")
	}
}

func TestHotCount(t *testing.T) {
	mgr := NewCacheInvalidManager()

	mgr.Put("key1", "value1")
	mgr.Put("key2", "value2")
	mgr.Put("key3", "value3")

	if mgr.HotCount() != 0 {
		t.Errorf("expected 0 hot entries, got %d", mgr.HotCount())
	}

	mgr.MarkHot("key1")
	mgr.MarkHot("key2")

	if mgr.HotCount() != 2 {
		t.Errorf("expected 2 hot entries, got %d", mgr.HotCount())
	}
}

func TestAutoMarkHotByAccess(t *testing.T) {
	cfg := Config{
		DefaultTTL:         1 * time.Minute,
		MaxEntries:         100,
		HotAccessThreshold: 5,
	}
	mgr := NewCacheInvalidManagerWithConfig(cfg)

	mgr.Put("key1", "value1")

	for i := 0; i < 4; i++ {
		mgr.Get("key1")
	}

	isHot, _ := mgr.IsHot("key1")
	if isHot {
		t.Error("expected key1 to not be hot yet")
	}

	mgr.Get("key1")

	isHot, _ = mgr.IsHot("key1")
	if !isHot {
		t.Error("expected key1 to be auto-marked as hot after threshold")
	}
}

func TestAddListener(t *testing.T) {
	mgr := NewCacheInvalidManager()

	listener := func(event InvalidationEvent) {}

	id, err := mgr.AddListener("test.event", listener)
	if err != nil {
		t.Fatalf("AddListener failed: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty listener id")
	}
}

func TestAddNilListener(t *testing.T) {
	mgr := NewCacheInvalidManager()

	_, err := mgr.AddListener("test.event", nil)
	if err == nil {
		t.Error("expected error for nil listener")
	}
}

func TestRemoveListener(t *testing.T) {
	mgr := NewCacheInvalidManager()

	listener := func(event InvalidationEvent) {}
	id, _ := mgr.AddListener("test.event", listener)

	err := mgr.RemoveListener(id)
	if err != nil {
		t.Fatalf("RemoveListener failed: %v", err)
	}
}

func TestRemoveListenerNotFound(t *testing.T) {
	mgr := NewCacheInvalidManager()

	err := mgr.RemoveListener("nonexistent")
	if !errors.Is(err, ErrListenerNotFound) {
		t.Errorf("expected ErrListenerNotFound, got %v", err)
	}
}

func TestPublishEvent(t *testing.T) {
	mgr := NewCacheInvalidManager()

	var callCount int32
	listener := func(event InvalidationEvent) {
		atomic.AddInt32(&callCount, 1)
	}

	mgr.AddListener("test.event", listener)

	event := InvalidationEvent{
		Key:       "key1",
		EventType: "test.event",
		Payload:   "test payload",
		Timestamp: time.Now(),
	}

	mgr.PublishEvent(event)

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected 1 listener call, got %d", callCount)
	}
}

func TestPublishEventNoListeners(t *testing.T) {
	mgr := NewCacheInvalidManager()

	event := InvalidationEvent{
		Key:       "key1",
		EventType: "nonexistent",
		Timestamp: time.Now(),
	}

	mgr.PublishEvent(event)
}

func TestPublishEventMultipleListeners(t *testing.T) {
	mgr := NewCacheInvalidManager()

	var callCount int32
	listener1 := func(event InvalidationEvent) {
		atomic.AddInt32(&callCount, 1)
	}
	listener2 := func(event InvalidationEvent) {
		atomic.AddInt32(&callCount, 1)
	}

	mgr.AddListener("test.event", listener1)
	mgr.AddListener("test.event", listener2)

	event := InvalidationEvent{
		Key:       "key1",
		EventType: "test.event",
		Timestamp: time.Now(),
	}

	mgr.PublishEvent(event)

	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("expected 2 listener calls, got %d", callCount)
	}
}

func TestPublishEventListenerPanicRecovery(t *testing.T) {
	mgr := NewCacheInvalidManager()

	var callCount int32
	panickingListener := func(event InvalidationEvent) {
		panic("test panic")
	}
	normalListener := func(event InvalidationEvent) {
		atomic.AddInt32(&callCount, 1)
	}

	mgr.AddListener("test.event", panickingListener)
	mgr.AddListener("test.event", normalListener)

	event := InvalidationEvent{
		Key:       "key1",
		EventType: "test.event",
		Timestamp: time.Now(),
	}

	mgr.PublishEvent(event)

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected 1 listener call after panic recovery, got %d", callCount)
	}
}

func TestInvalidate(t *testing.T) {
	mgr := NewCacheInvalidManager()

	var receivedEvent InvalidationEvent
	var wg sync.WaitGroup
	wg.Add(1)

	listener := func(event InvalidationEvent) {
		receivedEvent = event
		wg.Done()
	}

	mgr.AddListener("invalidate", listener)

	mgr.Put("key1", "value1")
	mgr.Invalidate("key1")

	wg.Wait()

	_, ok := mgr.Get("key1")
	if ok {
		t.Error("expected key1 to be invalidated")
	}

	if receivedEvent.Key != "key1" {
		t.Errorf("expected event key 'key1', got '%s'", receivedEvent.Key)
	}
	if receivedEvent.EventType != "invalidate" {
		t.Errorf("expected event type 'invalidate', got '%s'", receivedEvent.EventType)
	}
}

func TestInvalidateWithEvent(t *testing.T) {
	mgr := NewCacheInvalidManager()

	var receivedEvent InvalidationEvent
	var wg sync.WaitGroup
	wg.Add(1)

	listener := func(event InvalidationEvent) {
		receivedEvent = event
		wg.Done()
	}

	mgr.AddListener("data.updated", listener)

	mgr.Put("key1", "value1")
	mgr.InvalidateWithEvent("key1", "data.updated", "new value")

	wg.Wait()

	_, ok := mgr.Get("key1")
	if ok {
		t.Error("expected key1 to be invalidated")
	}

	if receivedEvent.EventType != "data.updated" {
		t.Errorf("expected event type 'data.updated', got '%s'", receivedEvent.EventType)
	}
	if receivedEvent.Payload != "new value" {
		t.Errorf("expected payload 'new value', got '%v'", receivedEvent.Payload)
	}
}

func TestSetPreloadLoader(t *testing.T) {
	mgr := NewCacheInvalidManager()

	loader := func() ([]CacheItem, error) {
		return []CacheItem{}, nil
	}

	err := mgr.SetPreloadLoader(loader)
	if err != nil {
		t.Fatalf("SetPreloadLoader failed: %v", err)
	}
}

func TestSetNilPreloadLoader(t *testing.T) {
	mgr := NewCacheInvalidManager()

	err := mgr.SetPreloadLoader(nil)
	if !errors.Is(err, ErrNilLoader) {
		t.Errorf("expected ErrNilLoader, got %v", err)
	}
}

func TestPreload(t *testing.T) {
	mgr := NewCacheInvalidManager()

	items := []CacheItem{
		{Key: "hot1", Value: "value1", IsHot: true},
		{Key: "hot2", Value: "value2", IsHot: true},
		{Key: "normal1", Value: "value3", IsHot: false},
	}

	loader := func() ([]CacheItem, error) {
		return items, nil
	}

	mgr.SetPreloadLoader(loader)

	err := mgr.Preload()
	if err != nil {
		t.Fatalf("Preload failed: %v", err)
	}

	if mgr.Count() != 3 {
		t.Errorf("expected 3 preloaded entries, got %d", mgr.Count())
	}

	if mgr.PreloadedCount() != 3 {
		t.Errorf("expected 3 preloaded entries, got %d", mgr.PreloadedCount())
	}

	if mgr.HotCount() != 2 {
		t.Errorf("expected 2 hot entries, got %d", mgr.HotCount())
	}
}

func TestPreloadWithoutLoader(t *testing.T) {
	mgr := NewCacheInvalidManager()

	err := mgr.Preload()
	if !errors.Is(err, ErrNilLoader) {
		t.Errorf("expected ErrNilLoader, got %v", err)
	}
}

func TestPreloadLoaderError(t *testing.T) {
	mgr := NewCacheInvalidManager()

	expectedErr := errors.New("loader error")
	loader := func() ([]CacheItem, error) {
		return nil, expectedErr
	}

	mgr.SetPreloadLoader(loader)

	err := mgr.Preload()
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected loader error, got %v", err)
	}
}

func TestPreloadDataNeverExpires(t *testing.T) {
	cfg := Config{
		DefaultTTL:  50 * time.Millisecond,
		MaxEntries:  100,
		PreloadSize: 10,
	}
	mgr := NewCacheInvalidManagerWithConfig(cfg)

	items := []CacheItem{
		{Key: "preloaded1", Value: "value1", IsHot: false},
	}

	loader := func() ([]CacheItem, error) {
		return items, nil
	}

	mgr.SetPreloadLoader(loader)
	mgr.Preload()

	time.Sleep(100 * time.Millisecond)

	value, ok := mgr.Get("preloaded1")
	if !ok {
		t.Fatal("expected preloaded key to still exist after default TTL expired")
	}
	if value != "value1" {
		t.Errorf("expected value 'value1', got '%v'", value)
	}

	expired, err := mgr.IsExpired("preloaded1")
	if err != nil {
		t.Fatalf("IsExpired failed: %v", err)
	}
	if expired {
		t.Error("expected preloaded key to not be expired")
	}
}

func TestPreloadRespectsSizeLimit(t *testing.T) {
	cfg := Config{
		DefaultTTL:  1 * time.Minute,
		MaxEntries:  100,
		PreloadSize: 3,
	}
	mgr := NewCacheInvalidManagerWithConfig(cfg)

	items := []CacheItem{
		{Key: "key1", Value: "value1", IsHot: false},
		{Key: "key2", Value: "value2", IsHot: false},
		{Key: "key3", Value: "value3", IsHot: false},
		{Key: "key4", Value: "value4", IsHot: false},
		{Key: "key5", Value: "value5", IsHot: false},
	}

	loader := func() ([]CacheItem, error) {
		return items, nil
	}

	mgr.SetPreloadLoader(loader)
	mgr.Preload()

	if mgr.PreloadedCount() != 3 {
		t.Errorf("expected 3 preloaded entries (limited by PreloadSize), got %d", mgr.PreloadedCount())
	}
}

func TestUnmarkPreloaded(t *testing.T) {
	mgr := NewCacheInvalidManager()

	items := []CacheItem{
		{Key: "preloaded1", Value: "value1", IsHot: false},
	}

	loader := func() ([]CacheItem, error) {
		return items, nil
	}

	mgr.SetPreloadLoader(loader)
	mgr.Preload()

	err := mgr.UnmarkPreloaded("preloaded1")
	if err != nil {
		t.Fatalf("UnmarkPreloaded failed: %v", err)
	}

	isPreloaded, err := mgr.IsPreloaded("preloaded1")
	if err != nil {
		t.Fatalf("IsPreloaded failed: %v", err)
	}
	if isPreloaded {
		t.Error("expected key to not be preloaded after unmark")
	}
}

func TestUnmarkPreloadedNonExistent(t *testing.T) {
	mgr := NewCacheInvalidManager()

	err := mgr.UnmarkPreloaded("nonexistent")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestIsPreloaded(t *testing.T) {
	mgr := NewCacheInvalidManager()

	mgr.Put("key1", "value1")

	isPreloaded, err := mgr.IsPreloaded("key1")
	if err != nil {
		t.Fatalf("IsPreloaded failed: %v", err)
	}
	if isPreloaded {
		t.Error("expected normal key to not be preloaded")
	}
}

func TestIsPreloadedNonExistent(t *testing.T) {
	mgr := NewCacheInvalidManager()

	_, err := mgr.IsPreloaded("nonexistent")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestPreloadedCount(t *testing.T) {
	mgr := NewCacheInvalidManager()

	if mgr.PreloadedCount() != 0 {
		t.Errorf("expected 0 preloaded entries, got %d", mgr.PreloadedCount())
	}
}

func TestGetEntry(t *testing.T) {
	mgr := NewCacheInvalidManager()

	mgr.Put("key1", "value1")

	entry, ok := mgr.GetEntry("key1")
	if !ok {
		t.Fatal("expected to get entry for key1")
	}
	if entry.Key != "key1" {
		t.Errorf("expected key 'key1', got '%s'", entry.Key)
	}
	if entry.Value != "value1" {
		t.Errorf("expected value 'value1', got '%v'", entry.Value)
	}
}

func TestGetEntryNonExistent(t *testing.T) {
	mgr := NewCacheInvalidManager()

	_, ok := mgr.GetEntry("nonexistent")
	if ok {
		t.Error("expected false for non-existent key")
	}
}

func TestMaxEntriesEviction(t *testing.T) {
	cfg := Config{
		DefaultTTL: 1 * time.Minute,
		MaxEntries: 3,
	}
	mgr := NewCacheInvalidManagerWithConfig(cfg)

	mgr.Put("key1", "value1")
	mgr.Put("key2", "value2")
	mgr.Put("key3", "value3")

	if mgr.Count() != 3 {
		t.Fatalf("expected 3 entries, got %d", mgr.Count())
	}

	mgr.Put("key4", "value4")

	if mgr.Count() != 3 {
		t.Errorf("expected 3 entries after eviction, got %d", mgr.Count())
	}
}

func TestMaxEntriesHotProtection(t *testing.T) {
	cfg := Config{
		DefaultTTL: 1 * time.Minute,
		MaxEntries: 3,
	}
	mgr := NewCacheInvalidManagerWithConfig(cfg)

	mgr.Put("hot1", "value1")
	mgr.MarkHot("hot1")
	mgr.Put("normal1", "value2")
	mgr.Put("normal2", "value3")

	mgr.Put("newkey", "value4")

	_, ok := mgr.Get("hot1")
	if !ok {
		t.Error("expected hot key to not be evicted")
	}
}

func TestCleanupExpired(t *testing.T) {
	mgr := NewCacheInvalidManager()

	mgr.PutWithTTL("key1", "value1", 50*time.Millisecond)
	mgr.PutWithTTL("key2", "value2", 50*time.Millisecond)
	mgr.Put("key3", "value3")

	time.Sleep(100 * time.Millisecond)

	cleaned := mgr.CleanupExpired()
	if cleaned != 2 {
		t.Errorf("expected 2 expired entries cleaned up, got %d", cleaned)
	}

	if mgr.Count() != 1 {
		t.Errorf("expected 1 entry remaining, got %d", mgr.Count())
	}
}

func TestCleanupExpiredHotProtection(t *testing.T) {
	mgr := NewCacheInvalidManager()

	mgr.PutWithTTL("hot1", "value1", 50*time.Millisecond)
	mgr.MarkHot("hot1")

	time.Sleep(100 * time.Millisecond)

	cleaned := mgr.CleanupExpired()
	if cleaned != 0 {
		t.Errorf("expected 0 expired entries (hot protected), got %d", cleaned)
	}
}

func TestConcurrentAccess(t *testing.T) {
	mgr := NewCacheInvalidManager()

	var wg sync.WaitGroup
	numGoroutines := 10
	numOps := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := "key-" + itoa(id*numOps + j)
				mgr.Put(key, "value")
				mgr.Get(key)
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentGetAndDelete(t *testing.T) {
	mgr := NewCacheInvalidManager()

	for i := 0; i < 1000; i++ {
		mgr.Put("key-"+itoa(i), "value")
	}

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			mgr.Get("key-" + itoa(i))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i += 2 {
			mgr.Delete("key-" + itoa(i))
		}
	}()

	wg.Wait()
}

func TestInvalidationEventFields(t *testing.T) {
	mgr := NewCacheInvalidManager()

	var receivedEvent InvalidationEvent
	var wg sync.WaitGroup
	wg.Add(1)

	listener := func(event InvalidationEvent) {
		receivedEvent = event
		wg.Done()
	}

	mgr.AddListener("test", listener)

	beforeTime := time.Now()
	mgr.InvalidateWithEvent("key1", "test", map[string]string{"field": "value"})
	wg.Wait()
	afterTime := time.Now()

	if receivedEvent.Key != "key1" {
		t.Errorf("expected Key 'key1', got '%s'", receivedEvent.Key)
	}
	if receivedEvent.EventType != "test" {
		t.Errorf("expected EventType 'test', got '%s'", receivedEvent.EventType)
	}
	if receivedEvent.Timestamp.Before(beforeTime) || receivedEvent.Timestamp.After(afterTime) {
		t.Error("expected Timestamp to be within expected range")
	}
}

func TestCacheEntryFields(t *testing.T) {
	mgr := NewCacheInvalidManager()

	mgr.PutWithTTL("key1", "value1", 2*time.Minute)

	entry, ok := mgr.GetEntry("key1")
	if !ok {
		t.Fatal("expected to get entry")
	}

	if entry.Key != "key1" {
		t.Errorf("expected Key 'key1', got '%s'", entry.Key)
	}
	if entry.Value != "value1" {
		t.Errorf("expected Value 'value1', got '%v'", entry.Value)
	}
	if entry.TTL != 2*time.Minute {
		t.Errorf("expected TTL 2m, got %v", entry.TTL)
	}
	if entry.IsHot.Load() {
		t.Error("expected IsHot false")
	}
	if entry.IsPreloaded {
		t.Error("expected IsPreloaded false")
	}
	if entry.AccessCount.Load() != 0 {
		t.Errorf("expected AccessCount 0, got %d", entry.AccessCount.Load())
	}
	if entry.CreateTime.IsZero() {
		t.Error("expected CreateTime to be set")
	}
	if entry.ExpiresAt.IsZero() {
		t.Error("expected ExpiresAt to be set")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DefaultTTL <= 0 {
		t.Error("expected DefaultTTL to be positive")
	}
	if cfg.MaxEntries <= 0 {
		t.Error("expected MaxEntries to be positive")
	}
	if cfg.HotAccessThreshold <= 0 {
		t.Error("expected HotAccessThreshold to be positive")
	}
	if cfg.PreloadSize < 0 {
		t.Error("expected PreloadSize to be non-negative")
	}
}

func TestListenerIDsAreUnique(t *testing.T) {
	mgr := NewCacheInvalidManager()

	listener := func(event InvalidationEvent) {}

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, _ := mgr.AddListener("event", listener)
		if ids[id] {
			t.Errorf("duplicate listener ID: %s", id)
		}
		ids[id] = true
	}
}

func TestMultipleEventTypes(t *testing.T) {
	mgr := NewCacheInvalidManager()

	var count1, count2 int32

	mgr.AddListener("type1", func(event InvalidationEvent) {
		atomic.AddInt32(&count1, 1)
	})
	mgr.AddListener("type2", func(event InvalidationEvent) {
		atomic.AddInt32(&count2, 1)
	})

	mgr.PublishEvent(InvalidationEvent{
		Key:       "k",
		EventType: "type1",
		Timestamp: time.Now(),
	})

	if atomic.LoadInt32(&count1) != 1 {
		t.Errorf("expected type1 count 1, got %d", count1)
	}
	if atomic.LoadInt32(&count2) != 0 {
		t.Errorf("expected type2 count 0, got %d", count2)
	}
}

func TestRemoveListenerCleanup(t *testing.T) {
	mgr := NewCacheInvalidManager()

	listener := func(event InvalidationEvent) {}
	id, _ := mgr.AddListener("test.event", listener)

	mgr.RemoveListener(id)

	mgr.PublishEvent(InvalidationEvent{
		Key:       "k",
		EventType: "test.event",
		Timestamp: time.Now(),
	})
}

func TestHotAndPreloaded(t *testing.T) {
	mgr := NewCacheInvalidManager()

	items := []CacheItem{
		{Key: "hot_preloaded", Value: "value", IsHot: true},
	}

	loader := func() ([]CacheItem, error) {
		return items, nil
	}

	mgr.SetPreloadLoader(loader)
	mgr.Preload()

	isHot, _ := mgr.IsHot("hot_preloaded")
	if !isHot {
		t.Error("expected key to be hot")
	}

	isPreloaded, _ := mgr.IsPreloaded("hot_preloaded")
	if !isPreloaded {
		t.Error("expected key to be preloaded")
	}
}

func TestUnmarkPreloadedStartsTTL(t *testing.T) {
	mgr := NewCacheInvalidManager()

	items := []CacheItem{
		{Key: "p1", Value: "v1", IsHot: false},
	}

	loader := func() ([]CacheItem, error) {
		return items, nil
	}

	mgr.SetPreloadLoader(loader)
	mgr.Preload()

	mgr.UnmarkPreloaded("p1")

	expired, _ := mgr.IsExpired("p1")
	if expired {
		t.Error("expected unmarked preloaded entry to not be immediately expired")
	}
}

func TestAllProtectedEntriesEvictionFails(t *testing.T) {
	cfg := Config{
		DefaultTTL: 1 * time.Minute,
		MaxEntries: 2,
	}
	mgr := NewCacheInvalidManagerWithConfig(cfg)

	mgr.Put("hot1", "value1")
	mgr.MarkHot("hot1")
	mgr.Put("hot2", "value2")
	mgr.MarkHot("hot2")

	if mgr.Count() != 2 {
		t.Fatalf("expected 2 entries, got %d", mgr.Count())
	}

	err := mgr.Put("newkey", "value3")
	if !errors.Is(err, ErrCapacityExhausted) {
		t.Errorf("expected ErrCapacityExhausted, got %v", err)
	}

	if mgr.Count() != 2 {
		t.Errorf("expected 2 entries (protected entries should not be evicted), got %d", mgr.Count())
	}

	_, ok1 := mgr.Get("hot1")
	_, ok2 := mgr.Get("hot2")
	if !ok1 || !ok2 {
		t.Error("expected both protected entries to still exist")
	}
}

func TestAllPreloadedEntriesEvictionFails(t *testing.T) {
	cfg := Config{
		DefaultTTL:  1 * time.Minute,
		MaxEntries:  2,
		PreloadSize: 2,
	}
	mgr := NewCacheInvalidManagerWithConfig(cfg)

	items := []CacheItem{
		{Key: "p1", Value: "v1", IsHot: false},
		{Key: "p2", Value: "v2", IsHot: false},
	}
	loader := func() ([]CacheItem, error) {
		return items, nil
	}
	mgr.SetPreloadLoader(loader)
	mgr.Preload()

	if mgr.Count() != 2 {
		t.Fatalf("expected 2 preloaded entries, got %d", mgr.Count())
	}

	err := mgr.Put("newkey", "value3")
	if !errors.Is(err, ErrCapacityExhausted) {
		t.Errorf("expected ErrCapacityExhausted, got %v", err)
	}

	if mgr.Count() != 2 {
		t.Errorf("expected 2 entries (preloaded entries should not be evicted), got %d", mgr.Count())
	}
}

func TestPreloadOnStartAutoTrigger(t *testing.T) {
	cfg := Config{
		DefaultTTL:     1 * time.Minute,
		MaxEntries:     100,
		PreloadSize:    10,
		PreloadOnStart: true,
	}
	mgr := NewCacheInvalidManagerWithConfig(cfg)

	items := []CacheItem{
		{Key: "auto1", Value: "v1", IsHot: true},
		{Key: "auto2", Value: "v2", IsHot: false},
		{Key: "auto3", Value: "v3", IsHot: true},
	}

	loadCalled := false
	loader := func() ([]CacheItem, error) {
		loadCalled = true
		return items, nil
	}

	err := mgr.SetPreloadLoader(loader)
	if err != nil {
		t.Fatalf("SetPreloadLoader failed: %v", err)
	}

	if !loadCalled {
		t.Error("expected preload to be triggered automatically when PreloadOnStart is true")
	}

	if mgr.PreloadedCount() != 3 {
		t.Errorf("expected 3 preloaded entries, got %d", mgr.PreloadedCount())
	}

	if mgr.HotCount() != 2 {
		t.Errorf("expected 2 hot entries, got %d", mgr.HotCount())
	}
}

func TestPreloadOnStartFalseDoesNotAutoTrigger(t *testing.T) {
	cfg := Config{
		DefaultTTL:     1 * time.Minute,
		MaxEntries:     100,
		PreloadSize:    10,
		PreloadOnStart: false,
	}
	mgr := NewCacheInvalidManagerWithConfig(cfg)

	loadCalled := false
	loader := func() ([]CacheItem, error) {
		loadCalled = true
		return []CacheItem{}, nil
	}

	err := mgr.SetPreloadLoader(loader)
	if err != nil {
		t.Fatalf("SetPreloadLoader failed: %v", err)
	}

	if loadCalled {
		t.Error("expected preload NOT to be triggered when PreloadOnStart is false")
	}

	if mgr.PreloadedCount() != 0 {
		t.Errorf("expected 0 preloaded entries, got %d", mgr.PreloadedCount())
	}
}

func TestPreloadOnStartWithLoaderError(t *testing.T) {
	cfg := Config{
		DefaultTTL:     1 * time.Minute,
		MaxEntries:     100,
		PreloadSize:    10,
		PreloadOnStart: true,
	}
	mgr := NewCacheInvalidManagerWithConfig(cfg)

	expectedErr := errors.New("preload failed")
	loader := func() ([]CacheItem, error) {
		return nil, expectedErr
	}

	err := mgr.SetPreloadLoader(loader)
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected preload error to propagate, got %v", err)
	}
}

func TestConcurrentReadPerformance(t *testing.T) {
	mgr := NewCacheInvalidManager()

	for i := 0; i < 100; i++ {
		mgr.Put("key-"+itoa(i), "value-"+itoa(i))
	}

	var wg sync.WaitGroup
	numGoroutines := 50
	readsPerGoroutine := 1000

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < readsPerGoroutine; j++ {
				key := "key-" + itoa(j%100)
				mgr.Get(key)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	totalReads := numGoroutines * readsPerGoroutine
	t.Logf("Completed %d reads in %v (%.0f reads/sec)", totalReads, elapsed, float64(totalReads)/elapsed.Seconds())

	if elapsed > 5*time.Second {
		t.Errorf("concurrent reads took too long: %v", elapsed)
	}
}

func TestConcurrentReadWithWrite(t *testing.T) {
	mgr := NewCacheInvalidManager()

	mgr.Put("shared-key", "initial-value")

	var wg sync.WaitGroup
	readGoroutines := 20
	writeGoroutines := 2
	opsPerGoroutine := 500

	var readErrors int32
	var writeErrors int32

	for i := 0; i < readGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				_, ok := mgr.Get("shared-key")
				if !ok {
					atomic.AddInt32(&readErrors, 1)
				}
				time.Sleep(time.Microsecond)
			}
		}()
	}

	for i := 0; i < writeGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				err := mgr.Put("key-"+itoa(id*opsPerGoroutine+j), "value")
				if err != nil {
					atomic.AddInt32(&writeErrors, 1)
				}
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	wg.Wait()

	if readErrors > 0 {
		t.Errorf("got %d read errors", readErrors)
	}
	if writeErrors > 0 {
		t.Errorf("got %d write errors", writeErrors)
	}
}

func TestCapacityExhaustedError(t *testing.T) {
	cfg := Config{
		DefaultTTL: 1 * time.Minute,
		MaxEntries: 1,
	}
	mgr := NewCacheInvalidManagerWithConfig(cfg)

	mgr.Put("hot1", "value1")
	mgr.MarkHot("hot1")

	err := mgr.Put("key2", "value2")
	if err == nil {
		t.Fatal("expected error when capacity is exhausted with all protected entries")
	}
	if !errors.Is(err, ErrCapacityExhausted) {
		t.Errorf("expected ErrCapacityExhausted, got %v", err)
	}

	errMsg := err.Error()
	if errMsg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestMixedProtectionEviction(t *testing.T) {
	cfg := Config{
		DefaultTTL: 1 * time.Minute,
		MaxEntries: 3,
	}
	mgr := NewCacheInvalidManagerWithConfig(cfg)

	mgr.Put("hot1", "value1")
	mgr.MarkHot("hot1")
	mgr.Put("normal1", "value2")
	mgr.Put("normal2", "value3")

	err := mgr.Put("newkey", "value4")
	if err != nil {
		t.Fatalf("expected successful Put with eviction of normal entry, got error: %v", err)
	}

	if mgr.Count() != 3 {
		t.Errorf("expected 3 entries after eviction, got %d", mgr.Count())
	}

	_, ok := mgr.Get("hot1")
	if !ok {
		t.Error("expected hot entry to be protected from eviction")
	}
}

func TestGetEntryReturnsCopy(t *testing.T) {
	mgr := NewCacheInvalidManager()

	mgr.Put("key1", "value1")

	entry1, ok1 := mgr.GetEntry("key1")
	if !ok1 {
		t.Fatal("expected to get entry")
	}

	entry2, ok2 := mgr.GetEntry("key1")
	if !ok2 {
		t.Fatal("expected to get entry")
	}

	if entry1 == entry2 {
		t.Error("expected GetEntry to return different copies")
	}

	if entry1.Key != entry2.Key {
		t.Error("expected entries to have same key")
	}
}
