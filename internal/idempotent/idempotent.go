package idempotent

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

var (
	ErrEmptyKey            = errors.New("idempotent: empty idempotent key")
	ErrIdempotentStopped   = errors.New("idempotent: idempotent is stopped")
	ErrInvalidConfig       = errors.New("idempotent: invalid config")
	ErrHandlerNil          = errors.New("idempotent: handler function is nil")
)

type Config struct {
	TTL           time.Duration
	CleanInterval time.Duration
	KeyHeader     string
}

func DefaultConfig() Config {
	return Config{
		TTL:           5 * time.Minute,
		CleanInterval: 1 * time.Minute,
		KeyHeader:     "X-Idempotency-Key",
	}
}

type Idempotent struct {
	cfg       Config
	mu        sync.Mutex
	cache     map[string]*cacheEntry
	pending   map[string]*pendingEntry
	running   bool
	stopped   bool
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

type cacheEntry struct {
	key       string
	statusCode int
	body       []byte
	expiresAt time.Time
}

type pendingEntry struct {
	key     string
	done    chan struct{}
	statusCode int
	body    []byte
}

func NewIdempotent() *Idempotent {
	i, err := NewIdempotentWithConfig(DefaultConfig())
	if err != nil {
		panic("idempotent: DefaultConfig is invalid: " + err.Error())
	}
	return i
}

func NewIdempotentWithConfig(cfg Config) (*Idempotent, error) {
	if cfg.TTL < 0 {
		return nil, ErrInvalidConfig
	}
	if cfg.CleanInterval < 0 {
		return nil, ErrInvalidConfig
	}

	if cfg.TTL == 0 {
		cfg.TTL = 5 * time.Minute
	}
	if cfg.CleanInterval == 0 {
		cfg.CleanInterval = cfg.TTL / 5
		if cfg.CleanInterval <= 0 {
			cfg.CleanInterval = time.Second
		}
	}
	if cfg.KeyHeader == "" {
		cfg.KeyHeader = "X-Idempotency-Key"
	}

	i := &Idempotent{
		cfg:     cfg,
		cache:   make(map[string]*cacheEntry),
		pending: make(map[string]*pendingEntry),
		stopCh:  make(chan struct{}),
	}
	return i, nil
}

func (i *Idempotent) Start() {
	i.mu.Lock()
	if i.stopped {
		i.mu.Unlock()
		return
	}
	if i.running {
		i.mu.Unlock()
		return
	}
	i.running = true
	i.stopCh = make(chan struct{})
	i.mu.Unlock()

	i.wg.Add(1)
	go i.cleanLoop()
}

func (i *Idempotent) Stop() {
	i.mu.Lock()
	if i.stopped {
		i.mu.Unlock()
		return
	}
	i.stopped = true
	if i.running {
		i.running = false
		close(i.stopCh)
	}
	i.mu.Unlock()

	i.wg.Wait()
}

func (i *Idempotent) Execute(key string, handler func() (int, []byte)) (int, []byte, bool, error) {
	if key == "" {
		return 0, nil, false, ErrEmptyKey
	}
	if handler == nil {
		return 0, nil, false, ErrHandlerNil
	}

	i.mu.Lock()
	if i.stopped {
		i.mu.Unlock()
		return 0, nil, false, ErrIdempotentStopped
	}

	if entry, ok := i.cache[key]; ok {
		if time.Now().Before(entry.expiresAt) {
			i.mu.Unlock()
			return entry.statusCode, entry.body, true, nil
		}
		delete(i.cache, key)
	}

	if pending, ok := i.pending[key]; ok {
		i.mu.Unlock()
		<-pending.done
		return pending.statusCode, pending.body, true, nil
	}

	pending := &pendingEntry{
		key:  key,
		done: make(chan struct{}),
	}
	i.pending[key] = pending
	i.mu.Unlock()

	statusCode, body := handler()

	i.mu.Lock()
	if i.stopped {
		delete(i.pending, key)
		close(pending.done)
		i.mu.Unlock()
		return 0, nil, false, ErrIdempotentStopped
	}

	pending.statusCode = statusCode
	pending.body = body

	entry := &cacheEntry{
		key:        key,
		statusCode: statusCode,
		body:       body,
		expiresAt:  time.Now().Add(i.cfg.TTL),
	}
	i.cache[key] = entry

	delete(i.pending, key)
	close(pending.done)
	i.mu.Unlock()

	return statusCode, body, false, nil
}

func (i *Idempotent) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get(i.cfg.KeyHeader)
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}

		rr := &responseRecorder{
			header: make(http.Header),
		}

		handler := func() (int, []byte) {
			next.ServeHTTP(rr, r)
			return rr.statusCode, rr.body
		}

		statusCode, body, fromCache, err := i.Execute(key, handler)
		if err != nil {
			if errors.Is(err, ErrIdempotentStopped) {
				http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		for k, vv := range rr.header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}

		if fromCache {
			w.Header().Set("X-Idempotent-Cache", "HIT")
		} else {
			w.Header().Set("X-Idempotent-Cache", "MISS")
		}

		w.WriteHeader(statusCode)
		w.Write(body)
	})
}

type responseRecorder struct {
	header     http.Header
	statusCode int
	body       []byte
}

func (rr *responseRecorder) Header() http.Header {
	return rr.header
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	if rr.statusCode == 0 {
		rr.statusCode = http.StatusOK
	}
	rr.body = append(rr.body, b...)
	return len(b), nil
}

func (rr *responseRecorder) WriteHeader(statusCode int) {
	rr.statusCode = statusCode
}

func (i *Idempotent) Get(key string) (int, []byte, bool, error) {
	if key == "" {
		return 0, nil, false, ErrEmptyKey
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	if i.stopped {
		return 0, nil, false, ErrIdempotentStopped
	}

	entry, ok := i.cache[key]
	if !ok {
		return 0, nil, false, nil
	}

	if time.Now().After(entry.expiresAt) {
		delete(i.cache, key)
		return 0, nil, false, nil
	}

	return entry.statusCode, entry.body, true, nil
}

func (i *Idempotent) Set(key string, statusCode int, body []byte) error {
	if key == "" {
		return ErrEmptyKey
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	if i.stopped {
		return ErrIdempotentStopped
	}

	entry := &cacheEntry{
		key:        key,
		statusCode: statusCode,
		body:       body,
		expiresAt:  time.Now().Add(i.cfg.TTL),
	}
	i.cache[key] = entry
	return nil
}

func (i *Idempotent) Delete(key string) error {
	if key == "" {
		return ErrEmptyKey
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	if i.stopped {
		return ErrIdempotentStopped
	}

	delete(i.cache, key)
	return nil
}

func (i *Idempotent) Contains(key string) (bool, error) {
	if key == "" {
		return false, ErrEmptyKey
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	if i.stopped {
		return false, ErrIdempotentStopped
	}

	entry, ok := i.cache[key]
	if !ok {
		return false, nil
	}

	if time.Now().After(entry.expiresAt) {
		delete(i.cache, key)
		return false, nil
	}

	return true, nil
}

func (i *Idempotent) Count() (int, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.stopped {
		return 0, ErrIdempotentStopped
	}

	count := 0
	now := time.Now()
	for _, entry := range i.cache {
		if now.Before(entry.expiresAt) {
			count++
		}
	}
	return count, nil
}

func (i *Idempotent) Clear() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.stopped {
		return ErrIdempotentStopped
	}

	i.cache = make(map[string]*cacheEntry)
	return nil
}

func (i *Idempotent) CleanExpired() (int, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.stopped {
		return 0, ErrIdempotentStopped
	}

	return i.cleanExpiredLocked(), nil
}

func (i *Idempotent) cleanExpiredLocked() int {
	now := time.Now()
	cleaned := 0

	for key, entry := range i.cache {
		if now.After(entry.expiresAt) {
			delete(i.cache, key)
			cleaned++
		}
	}

	return cleaned
}

func (i *Idempotent) cleanLoop() {
	defer i.wg.Done()

	ticker := time.NewTicker(i.cfg.CleanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-i.stopCh:
			return
		case <-ticker.C:
			_, _ = i.CleanExpired()
		}
	}
}
