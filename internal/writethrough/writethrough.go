package writethrough

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrKeyNotFound        = errors.New("key not found")
	ErrStorageUnavailable = errors.New("storage unavailable")
	ErrInvalidConfig      = errors.New("invalid configuration")
)

type WriteStrategy int

const (
	WriteThroughStrategy WriteStrategy = iota
	WriteAroundStrategy
)

func (s WriteStrategy) String() string {
	switch s {
	case WriteThroughStrategy:
		return "WriteThrough"
	case WriteAroundStrategy:
		return "WriteAround"
	default:
		return "Unknown"
	}
}

type Storage interface {
	Get(key string) (string, error)
	Put(key string, value string) error
	Delete(key string) error
}

type Cache interface {
	Get(key string) (string, bool)
	Put(key string, value string)
	Delete(key string) bool
}

type Config struct {
	MaxRetries         int
	RetryInterval      time.Duration
	DegradeThreshold   int
	RecoverThreshold   int
	RecoverWindow      time.Duration
	EnableReadThrough  bool
}

func DefaultConfig() Config {
	return Config{
		MaxRetries:        3,
		RetryInterval:     100 * time.Millisecond,
		DegradeThreshold:  5,
		RecoverThreshold:  3,
		RecoverWindow:     5 * time.Second,
		EnableReadThrough: true,
	}
}

type pendingItem struct {
	key      string
	value    string
	retryCnt int
	nextTry  time.Time
}

type WriteThroughCache struct {
	cache   Cache
	storage Storage
	config  Config

	strategy    atomic.Int32
	failureCnt  atomic.Int64
	successCnt  atomic.Int64
	lastSuccess time.Time
	lastFail    time.Time

	pendingMu   sync.Mutex
	pendingList []*pendingItem

	recoveryMu      sync.Mutex
	recoverySuccess []time.Time

	wg       sync.WaitGroup
	stopCh   chan struct{}
	stopOnce sync.Once
	mu       sync.RWMutex
}

type memoryCache struct {
	data map[string]string
	mu   sync.RWMutex
}

func newMemoryCache() *memoryCache {
	return &memoryCache{
		data: make(map[string]string),
	}
}

func (mc *memoryCache) Get(key string) (string, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	val, ok := mc.data[key]
	return val, ok
}

func (mc *memoryCache) Put(key string, value string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.data[key] = value
}

func (mc *memoryCache) Delete(key string) bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	_, ok := mc.data[key]
	if ok {
		delete(mc.data, key)
	}
	return ok
}

func NewWriteThroughCache(storage Storage) (*WriteThroughCache, error) {
	return NewWriteThroughCacheWithConfig(storage, DefaultConfig())
}

func NewWriteThroughCacheWithConfig(storage Storage, cfg Config) (*WriteThroughCache, error) {
	if storage == nil {
		return nil, ErrInvalidConfig
	}
	if cfg.MaxRetries < 0 {
		return nil, ErrInvalidConfig
	}
	if cfg.RetryInterval < 0 {
		return nil, ErrInvalidConfig
	}
	if cfg.DegradeThreshold <= 0 {
		return nil, ErrInvalidConfig
	}
	if cfg.RecoverThreshold <= 0 {
		return nil, ErrInvalidConfig
	}

	cache := &WriteThroughCache{
		cache:    newMemoryCache(),
		storage:  storage,
		config:   cfg,
		stopCh:   make(chan struct{}),
		lastSuccess: time.Now(),
	}

	cache.strategy.Store(int32(WriteThroughStrategy))

	cache.wg.Add(1)
	go cache.retryLoop()

	return cache, nil
}

func (wtc *WriteThroughCache) Get(key string) (string, error) {
	if val, ok := wtc.cache.Get(key); ok {
		return val, nil
	}

	if !wtc.config.EnableReadThrough {
		return "", ErrKeyNotFound
	}

	val, err := wtc.storage.Get(key)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return "", ErrKeyNotFound
		}
		return "", err
	}

	wtc.cache.Put(key, val)

	return val, nil
}

func (wtc *WriteThroughCache) Put(key string, value string) error {
	strategy := WriteStrategy(wtc.strategy.Load())

	if strategy == WriteAroundStrategy {
		return wtc.putWriteAround(key, value)
	}

	return wtc.putWriteThrough(key, value)
}

func (wtc *WriteThroughCache) putWriteThrough(key string, value string) error {
	wtc.cache.Put(key, value)

	err := wtc.tryWriteStorage(key, value)
	if err == nil {
		wtc.recordSuccess()
		return nil
	}

	wtc.recordFailure()
	wtc.addPending(key, value)

	if wtc.shouldDegrade() {
		wtc.degradeToWriteAround()
	}

	return err
}

func (wtc *WriteThroughCache) putWriteAround(key string, value string) error {
	err := wtc.storage.Put(key, value)
	if err != nil {
		wtc.recordFailure()
		return err
	}

	wtc.recordSuccess()

	if wtc.shouldRecover() {
		wtc.recoverToWriteThrough()
	}

	return nil
}

func (wtc *WriteThroughCache) tryWriteStorage(key string, value string) error {
	var lastErr error
	for i := 0; i <= wtc.config.MaxRetries; i++ {
		err := wtc.storage.Put(key, value)
		if err == nil {
			return nil
		}
		lastErr = err
		if i < wtc.config.MaxRetries {
			time.Sleep(wtc.config.RetryInterval)
		}
	}
	return lastErr
}

func (wtc *WriteThroughCache) Delete(key string) error {
	strategy := WriteStrategy(wtc.strategy.Load())

	if strategy == WriteAroundStrategy {
		err := wtc.storage.Delete(key)
		if err != nil {
			wtc.recordFailure()
			return err
		}
		wtc.recordSuccess()
		wtc.cache.Delete(key)
		return nil
	}

	wtc.cache.Delete(key)

	err := wtc.storage.Delete(key)
	if err != nil {
		wtc.recordFailure()
		return err
	}

	wtc.recordSuccess()
	return nil
}

func (wtc *WriteThroughCache) addPending(key string, value string) {
	wtc.pendingMu.Lock()
	defer wtc.pendingMu.Unlock()

	for _, item := range wtc.pendingList {
		if item.key == key {
			item.value = value
			item.retryCnt = 0
			item.nextTry = time.Now().Add(wtc.config.RetryInterval)
			return
		}
	}

	wtc.pendingList = append(wtc.pendingList, &pendingItem{
		key:     key,
		value:   value,
		nextTry: time.Now().Add(wtc.config.RetryInterval),
	})
}

func (wtc *WriteThroughCache) retryLoop() {
	defer wtc.wg.Done()

	ticker := time.NewTicker(wtc.config.RetryInterval / 2)
	if ticker.C == nil {
		ticker = time.NewTicker(50 * time.Millisecond)
	}
	defer ticker.Stop()

	for {
		select {
		case <-wtc.stopCh:
			return
		case <-ticker.C:
			wtc.processPendingRetries()
		}
	}
}

func (wtc *WriteThroughCache) processPendingRetries() {
	wtc.pendingMu.Lock()
	if len(wtc.pendingList) == 0 {
		wtc.pendingMu.Unlock()
		return
	}

	var readyItems []*pendingItem
	var remainingItems []*pendingItem

	now := time.Now()
	for _, item := range wtc.pendingList {
		if now.After(item.nextTry) || now.Equal(item.nextTry) {
			readyItems = append(readyItems, item)
		} else {
			remainingItems = append(remainingItems, item)
		}
	}
	wtc.pendingList = remainingItems
	wtc.pendingMu.Unlock()

	for _, item := range readyItems {
		err := wtc.storage.Put(item.key, item.value)
		if err == nil {
			wtc.recordSuccess()
			wtc.cache.Put(item.key, item.value)

			if wtc.shouldRecover() {
				wtc.recoverToWriteThrough()
			}
		} else {
			item.retryCnt++
			if item.retryCnt < wtc.config.MaxRetries {
				item.nextTry = time.Now().Add(wtc.config.RetryInterval)
				wtc.pendingMu.Lock()
				wtc.pendingList = append(wtc.pendingList, item)
				wtc.pendingMu.Unlock()
			}
		}
	}
}

func (wtc *WriteThroughCache) recordSuccess() {
	wtc.failureCnt.Store(0)
	wtc.successCnt.Add(1)
	wtc.mu.Lock()
	wtc.lastSuccess = time.Now()
	wtc.mu.Unlock()

	wtc.recoveryMu.Lock()
	defer wtc.recoveryMu.Unlock()

	wtc.recoverySuccess = append(wtc.recoverySuccess, time.Now())

	cutoff := time.Now().Add(-wtc.config.RecoverWindow)
	valid := 0
	for _, t := range wtc.recoverySuccess {
		if t.After(cutoff) {
			wtc.recoverySuccess[valid] = t
			valid++
		}
	}
	wtc.recoverySuccess = wtc.recoverySuccess[:valid]
}

func (wtc *WriteThroughCache) recordFailure() {
	wtc.failureCnt.Add(1)
	wtc.successCnt.Store(0)
	wtc.mu.Lock()
	wtc.lastFail = time.Now()
	wtc.mu.Unlock()

	wtc.recoveryMu.Lock()
	wtc.recoverySuccess = nil
	wtc.recoveryMu.Unlock()
}

func (wtc *WriteThroughCache) shouldDegrade() bool {
	if wtc.strategy.Load() != int32(WriteThroughStrategy) {
		return false
	}
	return wtc.failureCnt.Load() >= int64(wtc.config.DegradeThreshold)
}

func (wtc *WriteThroughCache) shouldRecover() bool {
	if wtc.strategy.Load() != int32(WriteAroundStrategy) {
		return false
	}

	wtc.recoveryMu.Lock()
	defer wtc.recoveryMu.Unlock()

	return len(wtc.recoverySuccess) >= wtc.config.RecoverThreshold
}

func (wtc *WriteThroughCache) degradeToWriteAround() {
	wtc.strategy.Store(int32(WriteAroundStrategy))
}

func (wtc *WriteThroughCache) recoverToWriteThrough() {
	wtc.strategy.Store(int32(WriteThroughStrategy))
	wtc.failureCnt.Store(0)

	wtc.recoveryMu.Lock()
	wtc.recoverySuccess = nil
	wtc.recoveryMu.Unlock()
}

func (wtc *WriteThroughCache) Strategy() WriteStrategy {
	return WriteStrategy(wtc.strategy.Load())
}

func (wtc *WriteThroughCache) FailureCount() int64 {
	return wtc.failureCnt.Load()
}

func (wtc *WriteThroughCache) PendingCount() int {
	wtc.pendingMu.Lock()
	defer wtc.pendingMu.Unlock()
	return len(wtc.pendingList)
}

func (wtc *WriteThroughCache) Close() {
	wtc.stopOnce.Do(func() {
		close(wtc.stopCh)
		wtc.wg.Wait()
	})
}
