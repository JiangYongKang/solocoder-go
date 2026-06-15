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

type Response struct {
	StatusCode int
	Body       []byte
	Header     http.Header
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
	key        string
	statusCode int
	body       []byte
	header     http.Header
	expiresAt  time.Time
}

type pendingEntry struct {
	key        string
	done       chan struct{}
	statusCode int
	body       []byte
	header     http.Header
	err        error
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

	for _, pending := range i.pending {
		pending.err = ErrIdempotentStopped
		close(pending.done)
	}
	i.pending = make(map[string]*pendingEntry)

	if i.running {
		i.running = false
		close(i.stopCh)
	}
	i.mu.Unlock()

	i.wg.Wait()
}

func (i *Idempotent) Execute(key string, handler func() Response) (Response, bool, error) {
	if key == "" {
		return Response{}, false, ErrEmptyKey
	}
	if handler == nil {
		return Response{}, false, ErrHandlerNil
	}

	i.mu.Lock()
	if i.stopped {
		i.mu.Unlock()
		return Response{}, false, ErrIdempotentStopped
	}

	if entry, ok := i.cache[key]; ok {
		if time.Now().Before(entry.expiresAt) {
			resp := Response{
				StatusCode: entry.statusCode,
				Body:       entry.body,
				Header:     cloneHeader(entry.header),
			}
			i.mu.Unlock()
			return resp, true, nil
		}
		delete(i.cache, key)
	}

	if pending, ok := i.pending[key]; ok {
		i.mu.Unlock()
		<-pending.done
		if pending.err != nil {
			return Response{}, false, pending.err
		}
		resp := Response{
			StatusCode: pending.statusCode,
			Body:       pending.body,
			Header:     cloneHeader(pending.header),
		}
		return resp, true, nil
	}

	pending := &pendingEntry{
		key:  key,
		done: make(chan struct{}),
	}
	i.pending[key] = pending
	i.mu.Unlock()

	resp := handler()

	i.mu.Lock()
	if _, stillPending := i.pending[key]; !stillPending {
		i.mu.Unlock()
		return Response{}, false, ErrIdempotentStopped
	}
	if i.stopped {
		delete(i.pending, key)
		pending.err = ErrIdempotentStopped
		close(pending.done)
		i.mu.Unlock()
		return Response{}, false, ErrIdempotentStopped
	}

	pending.statusCode = resp.StatusCode
	pending.body = resp.Body
	pending.header = cloneHeader(resp.Header)

	entry := &cacheEntry{
		key:        key,
		statusCode: resp.StatusCode,
		body:       resp.Body,
		header:     cloneHeader(resp.Header),
		expiresAt:  time.Now().Add(i.cfg.TTL),
	}
	i.cache[key] = entry

	delete(i.pending, key)
	close(pending.done)
	i.mu.Unlock()

	return resp, false, nil
}

func cloneHeader(h http.Header) http.Header {
	if h == nil {
		return nil
	}
	cloned := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		cloned[k] = vv2
	}
	return cloned
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

		handler := func() Response {
			next.ServeHTTP(rr, r)
			return Response{
				StatusCode: rr.statusCode,
				Body:       rr.body,
				Header:     rr.header,
			}
		}

		resp, fromCache, err := i.Execute(key, handler)
		if err != nil {
			if rr.statusCode != 0 {
				writeResponse(w, rr.statusCode, rr.body, rr.header, "MISS")
				return
			}
			if errors.Is(err, ErrIdempotentStopped) {
				http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
				return
			}
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		writeResponse(w, resp.StatusCode, resp.Body, resp.Header, cacheFlag(fromCache))
	})
}

func cacheFlag(fromCache bool) string {
	if fromCache {
		return "HIT"
	}
	return "MISS"
}

func writeResponse(w http.ResponseWriter, statusCode int, body []byte, header http.Header, cacheStatus string) {
	if header != nil {
		for k, vv := range header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
	}
	w.Header().Set("X-Idempotent-Cache", cacheStatus)
	w.WriteHeader(statusCode)
	w.Write(body)
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

func (i *Idempotent) Get(key string) (Response, bool, error) {
	if key == "" {
		return Response{}, false, ErrEmptyKey
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	if i.stopped {
		return Response{}, false, ErrIdempotentStopped
	}

	entry, ok := i.cache[key]
	if !ok {
		return Response{}, false, nil
	}

	if time.Now().After(entry.expiresAt) {
		delete(i.cache, key)
		return Response{}, false, nil
	}

	resp := Response{
		StatusCode: entry.statusCode,
		Body:       entry.body,
		Header:     cloneHeader(entry.header),
	}
	return resp, true, nil
}

func (i *Idempotent) Set(key string, statusCode int, body []byte, header http.Header) error {
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
		header:     cloneHeader(header),
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
