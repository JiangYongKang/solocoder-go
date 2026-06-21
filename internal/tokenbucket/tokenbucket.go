package tokenbucket

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrInvalidCapacity      = errors.New("tokenbucket: capacity must be greater than 0")
	ErrInvalidRate          = errors.New("tokenbucket: rate must be greater than 0")
	ErrInvalidTokenCount    = errors.New("tokenbucket: token count must be greater than 0")
	ErrInvalidWarmupConfig  = errors.New("tokenbucket: warmup requires positive duration and start rate less than rate")
	ErrBucketNotFound       = errors.New("tokenbucket: bucket not found")
	ErrEmptyKey             = errors.New("tokenbucket: key must not be empty")
)

type BucketConfig struct {
	Capacity       float64
	Rate           float64
	Warmup         bool
	WarmupStartRate float64
	WarmupDuration  time.Duration
}

type Result struct {
	Allowed    bool
	RetryAfter time.Duration
	Remaining  float64
}

type Bucket struct {
	mu         sync.Mutex
	tokens     float64
	capacity   float64
	rate       float64
	lastRefill time.Time

	warmup          bool
	warmupStartRate float64
	warmupDuration  time.Duration
	warmupStartTime time.Time
}

func NewBucket(cfg BucketConfig) (*Bucket, error) {
	if cfg.Capacity <= 0 {
		return nil, ErrInvalidCapacity
	}
	if cfg.Rate <= 0 {
		return nil, ErrInvalidRate
	}
	if cfg.Warmup {
		if cfg.WarmupDuration <= 0 || cfg.WarmupStartRate <= 0 || cfg.WarmupStartRate >= cfg.Rate {
			return nil, ErrInvalidWarmupConfig
		}
	}

	now := time.Now()

	return &Bucket{
		tokens:          cfg.Capacity,
		capacity:        cfg.Capacity,
		rate:            cfg.Rate,
		lastRefill:      now,
		warmup:          cfg.Warmup,
		warmupStartRate: cfg.WarmupStartRate,
		warmupDuration:  cfg.WarmupDuration,
		warmupStartTime: now,
	}, nil
}

func (b *Bucket) refill() {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill)
	if elapsed <= 0 {
		return
	}

	currentRate := b.currentRateLocked(now)
	added := elapsed.Seconds() * currentRate
	b.tokens += added
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	b.lastRefill = now
}

func (b *Bucket) currentRate(now time.Time) float64 {
	if !b.warmup {
		return b.rate
	}

	elapsed := now.Sub(b.warmupStartTime)
	if elapsed >= b.warmupDuration {
		return b.rate
	}

	progress := float64(elapsed) / float64(b.warmupDuration)
	return b.warmupStartRate + (b.rate-b.warmupStartRate)*progress
}

func (b *Bucket) currentRateLocked(now time.Time) float64 {
	if !b.warmup {
		return b.rate
	}

	elapsed := now.Sub(b.warmupStartTime)
	if elapsed >= b.warmupDuration {
		b.warmup = false
		return b.rate
	}

	progress := float64(elapsed) / float64(b.warmupDuration)
	return b.warmupStartRate + (b.rate-b.warmupStartRate)*progress
}

func (b *Bucket) Take(count float64) Result {
	b.mu.Lock()
	defer b.mu.Unlock()

	if count <= 0 {
		b.refill()
		return Result{Allowed: true, Remaining: b.tokens}
	}

	b.refill()

	if b.tokens >= count {
		b.tokens -= count
		return Result{Allowed: true, Remaining: b.tokens}
	}

	now := time.Now()
	currentRate := b.currentRateLocked(now)
	deficit := count - b.tokens
	var retryAfter time.Duration
	if currentRate > 0 {
		retryAfter = time.Duration(deficit / currentRate * float64(time.Second))
	}

	return Result{Allowed: false, RetryAfter: retryAfter, Remaining: b.tokens}
}

func (b *Bucket) SetRate(rate float64) error {
	if rate <= 0 {
		return ErrInvalidRate
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()
	b.rate = rate
	return nil
}

func (b *Bucket) SetCapacity(capacity float64) error {
	if capacity <= 0 {
		return ErrInvalidCapacity
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()
	b.capacity = capacity
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	return nil
}

func (b *Bucket) Tokens() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refill()
	return b.tokens
}

func (b *Bucket) Capacity() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.capacity
}

func (b *Bucket) Rate() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.rate
}

func (b *Bucket) CurrentRate() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.currentRateLocked(time.Now())
}

func (b *Bucket) IsWarmingUp() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.warmup {
		return false
	}
	elapsed := time.Now().Sub(b.warmupStartTime)
	return elapsed < b.warmupDuration
}

type Limiter struct {
	mu      sync.RWMutex
	buckets map[string]*Bucket
	config  BucketConfig
}

func NewLimiter(cfg BucketConfig) *Limiter {
	return &Limiter{
		buckets: make(map[string]*Bucket),
		config:  cfg,
	}
}

func (l *Limiter) getOrCreateBucket(key string) (*Bucket, error) {
	l.mu.RLock()
	b, exists := l.buckets[key]
	l.mu.RUnlock()

	if exists {
		return b, nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	b, exists = l.buckets[key]
	if exists {
		return b, nil
	}

	bucket, err := NewBucket(l.config)
	if err != nil {
		return nil, err
	}
	l.buckets[key] = bucket
	return bucket, nil
}

func (l *Limiter) Take(key string, count float64) (Result, error) {
	if key == "" {
		return Result{}, ErrEmptyKey
	}

	b, err := l.getOrCreateBucket(key)
	if err != nil {
		return Result{}, err
	}

	return b.Take(count), nil
}

func (l *Limiter) TakeMulti(keys []string, count float64) (Result, error) {
	if len(keys) == 0 {
		return Result{Allowed: true}, nil
	}

	for _, key := range keys {
		if key == "" {
			return Result{}, ErrEmptyKey
		}
	}

	seen := make(map[string]struct{}, len(keys))
	uniqueKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			uniqueKeys = append(uniqueKeys, key)
		}
	}

	buckets := make([]*Bucket, 0, len(uniqueKeys))
	for _, key := range uniqueKeys {
		b, err := l.getOrCreateBucket(key)
		if err != nil {
			return Result{}, err
		}
		buckets = append(buckets, b)
	}

	if count <= 0 {
		var worstResult Result
		worstResult.Allowed = true
		for i, b := range buckets {
			r := b.Take(count)
			if i == 0 || r.Remaining < worstResult.Remaining {
				worstResult.Remaining = r.Remaining
			}
		}
		return worstResult, nil
	}

	var worstResult Result
	worstResult.Allowed = true

	for i, b := range buckets {
		r := b.Take(count)
		if !r.Allowed {
			for j := 0; j < i; j++ {
				buckets[j].PutBack(count)
			}
			if r.RetryAfter > worstResult.RetryAfter {
				worstResult = r
			}
			worstResult.Allowed = false
			return worstResult, nil
		}
		if r.Remaining < worstResult.Remaining || i == 0 {
			worstResult.Remaining = r.Remaining
		}
	}

	return worstResult, nil
}

func (b *Bucket) PutBack(count float64) {
	if count <= 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tokens += count
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
}

func (l *Limiter) SetRate(key string, rate float64) error {
	l.mu.RLock()
	b, exists := l.buckets[key]
	l.mu.RUnlock()

	if !exists {
		return ErrBucketNotFound
	}
	return b.SetRate(rate)
}

func (l *Limiter) SetCapacity(key string, capacity float64) error {
	l.mu.RLock()
	b, exists := l.buckets[key]
	l.mu.RUnlock()

	if !exists {
		return ErrBucketNotFound
	}
	return b.SetCapacity(capacity)
}

func (l *Limiter) SetAllRates(rate float64) error {
	if rate <= 0 {
		return ErrInvalidRate
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.config.Rate = rate

	buckets := make([]*Bucket, 0, len(l.buckets))
	for _, b := range l.buckets {
		buckets = append(buckets, b)
	}

	for _, b := range buckets {
		if err := b.SetRate(rate); err != nil {
			return err
		}
	}
	return nil
}

func (l *Limiter) SetAllCapacities(capacity float64) error {
	if capacity <= 0 {
		return ErrInvalidCapacity
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.config.Capacity = capacity

	buckets := make([]*Bucket, 0, len(l.buckets))
	for _, b := range l.buckets {
		buckets = append(buckets, b)
	}

	for _, b := range buckets {
		if err := b.SetCapacity(capacity); err != nil {
			return err
		}
	}
	return nil
}

func (l *Limiter) Bucket(key string) (*Bucket, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	b, ok := l.buckets[key]
	return b, ok
}

func (l *Limiter) Remove(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, key)
}

func (l *Limiter) Keys() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	keys := make([]string, 0, len(l.buckets))
	for k := range l.buckets {
		keys = append(keys, k)
	}
	return keys
}

func (r Result) RetryAfterSeconds() int {
	if r.RetryAfter <= 0 {
		return 0
	}
	s := int(r.RetryAfter.Seconds())
	rem := r.RetryAfter - time.Duration(s)*time.Second
	if rem > 0 {
		s++
	}
	return s
}

func (r Result) String() string {
	if r.Allowed {
		return fmt.Sprintf("allowed (remaining=%.2f)", r.Remaining)
	}
	return fmt.Sprintf("rejected (retryAfter=%v, remaining=%.2f)", r.RetryAfter, r.Remaining)
}
