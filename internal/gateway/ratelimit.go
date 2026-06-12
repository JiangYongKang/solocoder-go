package gateway

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

func newTokenBucket(rate, capacity int, refillRate time.Duration) *tokenBucket {
	return &tokenBucket{
		tokens:     capacity,
		capacity:   capacity,
		rate:       rate,
		lastRefill: time.Now(),
		refillRate: refillRate,
	}
}

func (tb *tokenBucket) take() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	refillTokens := int(elapsed / tb.refillRate)
	if refillTokens > 0 {
		tb.tokens = min(tb.tokens+refillTokens*tb.rate, tb.capacity)
		tb.lastRefill = tb.lastRefill.Add(time.Duration(refillTokens) * tb.refillRate)
	}

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	return false
}

func NewRateLimiter(rate, capacity int, refillRate time.Duration) *RateLimiter {
	return &RateLimiter{
		tokens:     make(map[string]*tokenBucket),
		rate:       rate,
		capacity:   capacity,
		refillRate: refillRate,
	}
}

func (rl *RateLimiter) getBucket(key string) *tokenBucket {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.tokens[key]
	if !exists {
		bucket = newTokenBucket(rl.rate, rl.capacity, rl.refillRate)
		rl.tokens[key] = bucket
	}
	return bucket
}

func (rl *RateLimiter) Allow(key string) bool {
	bucket := rl.getBucket(key)
	return bucket.take()
}

func (rl *RateLimiter) Middleware() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r)
			if !rl.Allow(ip) {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte("Too Many Requests: Rate limit exceeded"))
				return
			}
			next(w, r)
		}
	}
}

func extractIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	host := r.RemoteAddr
	colon := strings.LastIndex(host, ":")
	if colon != -1 {
		return host[:colon]
	}
	return host
}

func (rl *RateLimiter) Reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.tokens, key)
}

type RateLimiterStats struct {
	mu      sync.Mutex
	allowed map[string]int
	denied  map[string]int
}

func NewRateLimiterStats() *RateLimiterStats {
	return &RateLimiterStats{
		allowed: make(map[string]int),
		denied:  make(map[string]int),
	}
}

func (s *RateLimiterStats) RecordAllowed(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allowed[key]++
}

func (s *RateLimiterStats) RecordDenied(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.denied[key]++
}

func (s *RateLimiterStats) GetStats(key string) (allowed, denied int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.allowed[key], s.denied[key]
}

func extractPort(hostport string) int {
	colon := strings.LastIndex(hostport, ":")
	if colon == -1 {
		return 0
	}
	port, err := strconv.Atoi(hostport[colon+1:])
	if err != nil {
		return 0
	}
	return port
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
