package cacheinvalid

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrKeyNotFound         = errors.New("cache key not found")
	ErrNilLoader           = errors.New("preload loader cannot be nil")
	ErrInvalidTTL          = errors.New("invalid TTL: must be non-negative")
	ErrListenerExists      = errors.New("listener already exists")
	ErrListenerNotFound    = errors.New("listener not found")
	ErrInvalidPreloadSize  = errors.New("invalid preload size: must be non-negative")
	ErrCapacityExhausted   = errors.New("cache capacity exhausted: all entries are protected")
)

type CacheEntry struct {
	Key         string
	Value       interface{}
	ExpiresAt   time.Time
	TTL         time.Duration
	IsHot       atomic.Bool
	IsPreloaded bool
	CreateTime  time.Time
	AccessCount atomic.Int64
}

type InvalidationEvent struct {
	Key       string
	EventType string
	Payload   interface{}
	Timestamp time.Time
}

type InvalidationListener func(event InvalidationEvent)

type PreloadLoader func() ([]CacheItem, error)

type CacheItem struct {
	Key   string
	Value interface{}
	IsHot bool
}

type Config struct {
	DefaultTTL      time.Duration
	MaxEntries      int
	HotAccessThreshold int64
	PreloadSize     int
	PreloadOnStart  bool
}

func DefaultConfig() Config {
	return Config{
		DefaultTTL:         5 * time.Minute,
		MaxEntries:         10000,
		HotAccessThreshold: 100,
		PreloadSize:        100,
		PreloadOnStart:     false,
	}
}

type listenerEntry struct {
	id       string
	listener InvalidationListener
}

type CacheInvalidManager struct {
	mu           sync.RWMutex
	entries      map[string]*CacheEntry
	config       Config
	listeners    map[string][]*listenerEntry
	listenerMap  map[string]*listenerEntry
	listenerEventType map[string]string
	nextListenerID uint64
	idMu         sync.Mutex
	preloadLoader PreloadLoader
}

func NewCacheInvalidManager() *CacheInvalidManager {
	return NewCacheInvalidManagerWithConfig(DefaultConfig())
}

func NewCacheInvalidManagerWithConfig(cfg Config) *CacheInvalidManager {
	if cfg.DefaultTTL < 0 {
		cfg.DefaultTTL = 5 * time.Minute
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = 10000
	}
	if cfg.HotAccessThreshold <= 0 {
		cfg.HotAccessThreshold = 100
	}
	if cfg.PreloadSize < 0 {
		cfg.PreloadSize = 100
	}

	mgr := &CacheInvalidManager{
		entries:           make(map[string]*CacheEntry),
		config:            cfg,
		listeners:         make(map[string][]*listenerEntry),
		listenerMap:       make(map[string]*listenerEntry),
		listenerEventType: make(map[string]string),
	}

	return mgr
}

func (m *CacheInvalidManager) generateListenerID() string {
	m.idMu.Lock()
	defer m.idMu.Unlock()
	m.nextListenerID++
	return "listener-" + itoa(int(m.nextListenerID))
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}

func (m *CacheInvalidManager) Put(key string, value interface{}) error {
	return m.PutWithTTL(key, value, m.config.DefaultTTL)
}

func (m *CacheInvalidManager) PutWithTTL(key string, value interface{}, ttl time.Duration) error {
	if ttl < 0 {
		return ErrInvalidTTL
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if entry, exists := m.entries[key]; exists {
		entry.Value = value
		entry.TTL = ttl
		entry.ExpiresAt = time.Now().Add(ttl)
		entry.CreateTime = time.Now()
		entry.IsHot.Store(false)
		entry.AccessCount.Store(0)
		return nil
	}

	if len(m.entries) >= m.config.MaxEntries {
		evicted := m.evictOne()
		if !evicted {
			return ErrCapacityExhausted
		}
	}

	entry := &CacheEntry{
		Key:         key,
		Value:       value,
		ExpiresAt:   time.Now().Add(ttl),
		TTL:         ttl,
		IsPreloaded: false,
		CreateTime:  time.Now(),
	}
	entry.IsHot.Store(false)
	entry.AccessCount.Store(0)
	m.entries[key] = entry

	return nil
}

func (m *CacheInvalidManager) Get(key string) (interface{}, bool) {
	m.mu.RLock()
	entry, exists := m.entries[key]
	if !exists {
		m.mu.RUnlock()
		return nil, false
	}

	isHot := entry.IsHot.Load()
	isPreloaded := entry.IsPreloaded
	value := entry.Value
	expiresAt := entry.ExpiresAt
	hotThreshold := m.config.HotAccessThreshold
	m.mu.RUnlock()

	if isHot || isPreloaded {
		entry.AccessCount.Add(1)
		return value, true
	}

	if time.Now().After(expiresAt) {
		m.mu.Lock()
		entry2, exists2 := m.entries[key]
		if exists2 && !entry2.IsHot.Load() && !entry2.IsPreloaded && time.Now().After(entry2.ExpiresAt) {
			delete(m.entries, key)
		}
		m.mu.Unlock()
		return nil, false
	}

	count := entry.AccessCount.Add(1)
	if count >= hotThreshold && !isHot {
		entry.IsHot.CompareAndSwap(false, true)
	}

	return value, true
}

func (m *CacheInvalidManager) Delete(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.entries[key]
	if exists {
		delete(m.entries, key)
	}
	return exists
}

func (m *CacheInvalidManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = make(map[string]*CacheEntry)
}

func (m *CacheInvalidManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

func (m *CacheInvalidManager) HotCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, entry := range m.entries {
		if entry.IsHot.Load() {
			count++
		}
	}
	return count
}

func (m *CacheInvalidManager) IsExpired(key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.entries[key]
	if !exists {
		return false, ErrKeyNotFound
	}

	if entry.IsHot.Load() || entry.IsPreloaded {
		return false, nil
	}

	return time.Now().After(entry.ExpiresAt), nil
}

func (m *CacheInvalidManager) MarkHot(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[key]
	if !exists {
		return ErrKeyNotFound
	}

	entry.IsHot.Store(true)
	return nil
}

func (m *CacheInvalidManager) UnmarkHot(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[key]
	if !exists {
		return ErrKeyNotFound
	}

	entry.IsHot.Store(false)
	return nil
}

func (m *CacheInvalidManager) IsHot(key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.entries[key]
	if !exists {
		return false, ErrKeyNotFound
	}

	return entry.IsHot.Load(), nil
}

func (m *CacheInvalidManager) AddListener(eventType string, listener InvalidationListener) (string, error) {
	if listener == nil {
		return "", errors.New("listener cannot be nil")
	}

	id := m.generateListenerID()
	le := &listenerEntry{
		id:       id,
		listener: listener,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.listeners[eventType] = append(m.listeners[eventType], le)
	m.listenerMap[id] = le
	m.listenerEventType[id] = eventType

	return id, nil
}

func (m *CacheInvalidManager) RemoveListener(listenerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	eventType, exists := m.listenerEventType[listenerID]
	if !exists {
		return ErrListenerNotFound
	}

	listeners := m.listeners[eventType]
	for i, le := range listeners {
		if le.id == listenerID {
			m.listeners[eventType] = append(listeners[:i], listeners[i+1:]...)
			break
		}
	}

	if len(m.listeners[eventType]) == 0 {
		delete(m.listeners, eventType)
	}

	delete(m.listenerMap, listenerID)
	delete(m.listenerEventType, listenerID)
	return nil
}

func (m *CacheInvalidManager) PublishEvent(event InvalidationEvent) {
	m.mu.RLock()
	listenerEntries := m.listeners[event.EventType]
	m.mu.RUnlock()

	if len(listenerEntries) == 0 {
		return
	}

	for _, le := range listenerEntries {
		func(l InvalidationListener) {
			defer func() {
				recover()
			}()
			l(event)
		}(le.listener)
	}
}

func (m *CacheInvalidManager) Invalidate(key string) {
	m.Delete(key)
	m.PublishEvent(InvalidationEvent{
		Key:       key,
		EventType: "invalidate",
		Timestamp: time.Now(),
	})
}

func (m *CacheInvalidManager) InvalidateWithEvent(key string, eventType string, payload interface{}) {
	m.Delete(key)
	m.PublishEvent(InvalidationEvent{
		Key:       key,
		EventType: eventType,
		Payload:   payload,
		Timestamp: time.Now(),
	})
}

func (m *CacheInvalidManager) SetPreloadLoader(loader PreloadLoader) error {
	if loader == nil {
		return ErrNilLoader
	}
	m.preloadLoader = loader

	if m.config.PreloadOnStart {
		return m.Preload()
	}

	return nil
}

func (m *CacheInvalidManager) Preload() error {
	if m.preloadLoader == nil {
		return ErrNilLoader
	}

	items, err := m.preloadLoader()
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	preloadCount := 0
	for _, item := range items {
		if preloadCount >= m.config.PreloadSize {
			break
		}

		if len(m.entries) >= m.config.MaxEntries {
			break
		}

		entry := &CacheEntry{
			Key:         item.Key,
			Value:       item.Value,
			ExpiresAt:   time.Time{},
			TTL:         0,
			IsPreloaded: true,
			CreateTime:  time.Now(),
		}
		entry.IsHot.Store(item.IsHot)
		entry.AccessCount.Store(0)
		m.entries[item.Key] = entry
		preloadCount++
	}

	return nil
}

func (m *CacheInvalidManager) UnmarkPreloaded(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[key]
	if !exists {
		return ErrKeyNotFound
	}

	entry.IsPreloaded = false
	entry.ExpiresAt = time.Now().Add(m.config.DefaultTTL)
	entry.TTL = m.config.DefaultTTL

	return nil
}

func (m *CacheInvalidManager) IsPreloaded(key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.entries[key]
	if !exists {
		return false, ErrKeyNotFound
	}

	return entry.IsPreloaded, nil
}

func (m *CacheInvalidManager) PreloadedCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, entry := range m.entries {
		if entry.IsPreloaded {
			count++
		}
	}
	return count
}

func (m *CacheInvalidManager) evictOne() bool {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range m.entries {
		if entry.IsHot.Load() || entry.IsPreloaded {
			continue
		}
		if oldestKey == "" || entry.CreateTime.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreateTime
		}
	}

	if oldestKey != "" {
		delete(m.entries, oldestKey)
		return true
	}

	return false
}

func (m *CacheInvalidManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	now := time.Now()
	for key, entry := range m.entries {
		if entry.IsHot.Load() || entry.IsPreloaded {
			continue
		}
		if now.After(entry.ExpiresAt) {
			delete(m.entries, key)
			count++
		}
	}
	return count
}

func (m *CacheInvalidManager) GetEntry(key string) (*CacheEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.entries[key]
	if !exists {
		return nil, false
	}

	result := &CacheEntry{
		Key:         entry.Key,
		Value:       entry.Value,
		ExpiresAt:   entry.ExpiresAt,
		TTL:         entry.TTL,
		IsPreloaded: entry.IsPreloaded,
		CreateTime:  entry.CreateTime,
	}
	result.IsHot.Store(entry.IsHot.Load())
	result.AccessCount.Store(entry.AccessCount.Load())
	return result, true
}
