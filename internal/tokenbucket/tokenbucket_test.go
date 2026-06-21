package tokenbucket

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewBucket_InvalidCapacity(t *testing.T) {
	_, err := NewBucket(BucketConfig{Capacity: 0, Rate: 10})
	if err != ErrInvalidCapacity {
		t.Errorf("expected ErrInvalidCapacity, got %v", err)
	}

	_, err = NewBucket(BucketConfig{Capacity: -1, Rate: 10})
	if err != ErrInvalidCapacity {
		t.Errorf("expected ErrInvalidCapacity, got %v", err)
	}
}

func TestNewBucket_InvalidRate(t *testing.T) {
	_, err := NewBucket(BucketConfig{Capacity: 10, Rate: 0})
	if err != ErrInvalidRate {
		t.Errorf("expected ErrInvalidRate, got %v", err)
	}

	_, err = NewBucket(BucketConfig{Capacity: 10, Rate: -5})
	if err != ErrInvalidRate {
		t.Errorf("expected ErrInvalidRate, got %v", err)
	}
}

func TestNewBucket_InvalidWarmup(t *testing.T) {
	_, err := NewBucket(BucketConfig{
		Capacity:       10,
		Rate:           10,
		Warmup:         true,
		WarmupDuration: 0,
		WarmupStartRate: 1,
	})
	if err != ErrInvalidWarmupConfig {
		t.Errorf("expected ErrInvalidWarmupConfig for zero duration, got %v", err)
	}

	_, err = NewBucket(BucketConfig{
		Capacity:       10,
		Rate:           10,
		Warmup:         true,
		WarmupDuration: time.Second,
		WarmupStartRate: 0,
	})
	if err != ErrInvalidWarmupConfig {
		t.Errorf("expected ErrInvalidWarmupConfig for zero start rate, got %v", err)
	}

	_, err = NewBucket(BucketConfig{
		Capacity:       10,
		Rate:           10,
		Warmup:         true,
		WarmupDuration: time.Second,
		WarmupStartRate: 10,
	})
	if err != ErrInvalidWarmupConfig {
		t.Errorf("expected ErrInvalidWarmupConfig for start rate >= rate, got %v", err)
	}

	_, err = NewBucket(BucketConfig{
		Capacity:       10,
		Rate:           10,
		Warmup:         true,
		WarmupDuration: time.Second,
		WarmupStartRate: 15,
	})
	if err != ErrInvalidWarmupConfig {
		t.Errorf("expected ErrInvalidWarmupConfig for start rate > rate, got %v", err)
	}
}

func TestNewBucket_Success(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 100, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if b.Capacity() != 100 {
		t.Errorf("expected capacity 100, got %f", b.Capacity())
	}
	if b.Rate() != 10 {
		t.Errorf("expected rate 10, got %f", b.Rate())
	}
	if b.Tokens() != 100 {
		t.Errorf("expected initial tokens 100, got %f", b.Tokens())
	}
	if b.IsWarmingUp() {
		t.Error("expected no warmup")
	}
}

func TestNewBucket_WithWarmup(t *testing.T) {
	b, err := NewBucket(BucketConfig{
		Capacity:       100,
		Rate:           100,
		Warmup:         true,
		WarmupStartRate: 10,
		WarmupDuration:  10 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !b.IsWarmingUp() {
		t.Error("expected warmup to be active")
	}
	if b.Rate() != 100 {
		t.Errorf("expected target rate 100, got %f", b.Rate())
	}
	if b.CurrentRate() >= 100 {
		t.Errorf("expected current rate < 100 during warmup, got %f", b.CurrentRate())
	}
}

func TestBucket_Take_Success(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 10, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := b.Take(5)
	if !r.Allowed {
		t.Error("expected take to succeed")
	}
	if r.Remaining != 5 {
		t.Errorf("expected remaining 5, got %f", r.Remaining)
	}
	if r.RetryAfter != 0 {
		t.Errorf("expected no retry after, got %v", r.RetryAfter)
	}
}

func TestBucket_Take_ExactlyAllTokens(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 10, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := b.Take(10)
	if !r.Allowed {
		t.Error("expected take to succeed when taking exactly all tokens")
	}
	if r.Remaining != 0 {
		t.Errorf("expected remaining 0, got %f", r.Remaining)
	}
}

func TestBucket_Take_Rejected(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 5, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := b.Take(8)
	if r.Allowed {
		t.Error("expected take to be rejected")
	}
	if r.RetryAfter <= 0 {
		t.Errorf("expected positive retry after, got %v", r.RetryAfter)
	}
	if r.Remaining != 5 {
		t.Errorf("expected remaining 5 (unchanged), got %f", r.Remaining)
	}
}

func TestBucket_Take_ZeroCount(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 10, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := b.Take(0)
	if !r.Allowed {
		t.Error("expected zero take to succeed")
	}
}

func TestBucket_Take_NegativeCount(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 10, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := b.Take(-5)
	if !r.Allowed {
		t.Error("expected negative take to succeed")
	}
}

func TestBucket_Refill(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 100, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.Take(100)
	if b.Tokens() != 0 {
		t.Errorf("expected 0 tokens after taking all, got %f", b.Tokens())
	}

	b.mu.Lock()
	b.lastRefill = b.lastRefill.Add(-500 * time.Millisecond)
	b.mu.Unlock()

	tokens := b.Tokens()
	if tokens < 4.9 || tokens > 5.1 {
		t.Errorf("expected ~5 tokens after 500ms refill at 10/s, got %f", tokens)
	}
}

func TestBucket_Refill_CappedAtCapacity(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 10, Rate: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.Take(5)

	b.mu.Lock()
	b.lastRefill = b.lastRefill.Add(-10 * time.Second)
	b.mu.Unlock()

	tokens := b.Tokens()
	if tokens != 10 {
		t.Errorf("expected tokens capped at capacity 10, got %f", tokens)
	}
}

func TestBucket_BurstTraffic(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 100, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := b.Take(80)
	if !r.Allowed {
		t.Error("expected first burst take to succeed")
	}
	if r.Remaining != 20 {
		t.Errorf("expected remaining 20, got %f", r.Remaining)
	}

	r = b.Take(20)
	if !r.Allowed {
		t.Error("expected second burst take to succeed (using accumulated tokens)")
	}
	if r.Remaining != 0 {
		t.Errorf("expected remaining 0, got %f", r.Remaining)
	}

	r = b.Take(1)
	if r.Allowed {
		t.Error("expected take to be rejected after bucket is empty")
	}
}

func TestBucket_BurstAccumulation(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 50, Rate: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.Take(50)

	b.mu.Lock()
	b.lastRefill = b.lastRefill.Add(-2 * time.Second)
	b.mu.Unlock()

	tokens := b.Tokens()
	if tokens != 50 {
		t.Errorf("expected tokens capped at 50 after accumulation, got %f", tokens)
	}
}

func TestBucket_SetRate(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 100, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = b.SetRate(50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Rate() != 50 {
		t.Errorf("expected rate 50, got %f", b.Rate())
	}
}

func TestBucket_SetRate_Invalid(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 100, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = b.SetRate(0)
	if err != ErrInvalidRate {
		t.Errorf("expected ErrInvalidRate, got %v", err)
	}

	err = b.SetRate(-5)
	if err != ErrInvalidRate {
		t.Errorf("expected ErrInvalidRate, got %v", err)
	}

	if b.Rate() != 10 {
		t.Errorf("expected rate unchanged at 10, got %f", b.Rate())
	}
}

func TestBucket_SetRate_EffectiveImmediately(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 100, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.Take(100)

	err = b.SetRate(1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.mu.Lock()
	b.lastRefill = b.lastRefill.Add(-100 * time.Millisecond)
	b.mu.Unlock()

	tokens := b.Tokens()
	if tokens < 99 || tokens > 101 {
		t.Errorf("expected ~100 tokens with new rate after 100ms, got %f", tokens)
	}
}

func TestBucket_SetCapacity(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 100, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = b.SetCapacity(200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Capacity() != 200 {
		t.Errorf("expected capacity 200, got %f", b.Capacity())
	}
}

func TestBucket_SetCapacity_Invalid(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 100, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = b.SetCapacity(0)
	if err != ErrInvalidCapacity {
		t.Errorf("expected ErrInvalidCapacity, got %v", err)
	}

	err = b.SetCapacity(-1)
	if err != ErrInvalidCapacity {
		t.Errorf("expected ErrInvalidCapacity, got %v", err)
	}
}

func TestBucket_SetCapacity_TrimsExcess(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 100, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = b.SetCapacity(50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tokens := b.Tokens()
	if tokens != 50 {
		t.Errorf("expected tokens trimmed to new capacity 50, got %f", tokens)
	}
}

func TestBucket_SetCapacity_DoesNotAffectHeldTokens(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 100, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.Take(30)

	err = b.SetCapacity(50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tokens := b.Tokens()
	if tokens != 50 {
		t.Errorf("expected tokens trimmed to capacity 50 (not below), got %f", tokens)
	}
}

func TestBucket_PutBack(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 100, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.Take(50)
	b.PutBack(20)

	tokens := b.Tokens()
	if tokens != 70 {
		t.Errorf("expected 70 tokens after putback, got %f", tokens)
	}
}

func TestBucket_PutBack_CappedAtCapacity(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 100, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.Take(10)
	b.PutBack(200)

	tokens := b.Tokens()
	if tokens != 100 {
		t.Errorf("expected tokens capped at 100 after large putback, got %f", tokens)
	}
}

func TestBucket_PutBack_ZeroOrNegative(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 100, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.Take(50)
	tokensBefore := b.Tokens()

	b.PutBack(0)
	b.PutBack(-10)

	tokens := b.Tokens()
	if tokens != tokensBefore {
		t.Errorf("expected tokens unchanged after zero/negative putback, got %f", tokens)
	}
}

func TestBucket_Warmup_LinearRateIncrease(t *testing.T) {
	b, err := NewBucket(BucketConfig{
		Capacity:       1000,
		Rate:           100,
		Warmup:         true,
		WarmupStartRate: 10,
		WarmupDuration:  10 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !b.IsWarmingUp() {
		t.Error("expected warmup to be active initially")
	}

	initialRate := b.CurrentRate()
	if initialRate < 9 || initialRate > 11 {
		t.Errorf("expected initial rate ~10, got %f", initialRate)
	}

	b.mu.Lock()
	b.warmupStartTime = b.warmupStartTime.Add(-5 * time.Second)
	b.mu.Unlock()

	midRate := b.CurrentRate()
	if midRate < 50 || midRate > 60 {
		t.Errorf("expected mid warmup rate ~55, got %f", midRate)
	}

	b.mu.Lock()
	b.warmupStartTime = b.warmupStartTime.Add(-5 * time.Second)
	b.mu.Unlock()

	finalRate := b.CurrentRate()
	if finalRate != 100 {
		t.Errorf("expected final rate 100 after warmup, got %f", finalRate)
	}
	if b.IsWarmingUp() {
		t.Error("expected warmup to be finished")
	}
}

func TestBucket_Warmup_SlowRefill(t *testing.T) {
	b, err := NewBucket(BucketConfig{
		Capacity:       1000,
		Rate:           100,
		Warmup:         true,
		WarmupStartRate: 10,
		WarmupDuration:  10 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.Take(1000)

	b.mu.Lock()
	b.lastRefill = b.lastRefill.Add(-1 * time.Second)
	b.warmupStartTime = b.warmupStartTime.Add(-1 * time.Second)
	b.mu.Unlock()

	tokens := b.Tokens()
	if tokens > 25 {
		t.Errorf("expected slow refill during early warmup (<25 tokens at ~10-19/s), got %f", tokens)
	}
}

func TestBucket_RetryAfterCalculation(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 5, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.Take(5)

	r := b.Take(10)
	if r.Allowed {
		t.Error("expected rejection")
	}

	expectedRetryAfter := 1.0
	margin := 0.1
	retrySeconds := r.RetryAfter.Seconds()
	if retrySeconds < expectedRetryAfter-margin || retrySeconds > expectedRetryAfter+margin {
		t.Errorf("expected retry after ~1s (10 tokens needed / 10 rate), got %v (%f s)", r.RetryAfter, retrySeconds)
	}
}

func TestBucket_RetryAfter_DynamicRate(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 5, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.Take(5)

	b.SetRate(5)

	r := b.Take(10)
	if r.Allowed {
		t.Error("expected rejection")
	}

	expectedRetryAfter := 2.0
	margin := 0.2
	retrySeconds := r.RetryAfter.Seconds()
	if retrySeconds < expectedRetryAfter-margin || retrySeconds > expectedRetryAfter+margin {
		t.Errorf("expected retry after ~2s (10 tokens / 5 rate), got %v (%f s)", r.RetryAfter, retrySeconds)
	}
}

func TestLimiter_NewLimiter(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 10, Rate: 5})
	if l == nil {
		t.Fatal("expected non-nil limiter")
	}
}

func TestLimiter_Take_CreatesBucket(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 10, Rate: 5})

	r, err := l.Take("user:123", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.Allowed {
		t.Error("expected take to succeed")
	}
	if r.Remaining != 7 {
		t.Errorf("expected remaining 7, got %f", r.Remaining)
	}

	keys := l.Keys()
	if len(keys) != 1 {
		t.Errorf("expected 1 bucket, got %d", len(keys))
	}
}

func TestLimiter_Take_SameKeyReusesBucket(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 10, Rate: 5})

	l.Take("user:123", 3)
	l.Take("user:123", 3)

	_, ok := l.Bucket("user:123")
	if !ok {
		t.Error("expected bucket to exist")
	}

	keys := l.Keys()
	if len(keys) != 1 {
		t.Errorf("expected 1 bucket for same key, got %d", len(keys))
	}
}

func TestLimiter_Take_DifferentKeysIsolated(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 5, Rate: 5})

	r1, _ := l.Take("user:alice", 5)
	if !r1.Allowed {
		t.Error("expected alice to get 5 tokens")
	}

	r2, _ := l.Take("user:bob", 5)
	if !r2.Allowed {
		t.Error("expected bob to get 5 tokens (independent bucket)")
	}

	r3, _ := l.Take("user:alice", 1)
	if r3.Allowed {
		t.Error("expected alice to be rejected (bucket empty)")
	}
}

func TestLimiter_Take_EmptyKey(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 10, Rate: 5})

	_, err := l.Take("", 1)
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestLimiter_Take_ZeroCount(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 10, Rate: 5})

	r, err := l.Take("key", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.Allowed {
		t.Error("expected zero take to be allowed")
	}
}

func TestLimiter_TakeMulti_AllAllowed(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 10, Rate: 5})

	r, err := l.TakeMulti([]string{"user:alice", "ip:1.2.3.4", "path:/api"}, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.Allowed {
		t.Error("expected multi-take to succeed")
	}
	if r.Remaining != 7 {
		t.Errorf("expected remaining 7, got %f", r.Remaining)
	}
}

func TestLimiter_TakeMulti_OneRejectedAllRolledBack(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 10, Rate: 5})

	l.Take("ip:1.2.3.4", 8)

	r, err := l.TakeMulti([]string{"user:alice", "ip:1.2.3.4"}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Allowed {
		t.Error("expected multi-take to be rejected (ip bucket insufficient)")
	}

	aliceBucket, _ := l.Bucket("user:alice")
	if aliceBucket.Tokens() != 10 {
		t.Errorf("expected alice bucket to be rolled back to 10, got %f", aliceBucket.Tokens())
	}

	ipBucket, _ := l.Bucket("ip:1.2.3.4")
	if ipBucket.Tokens() != 2 {
		t.Errorf("expected ip bucket unchanged at 2 (take rejected, no deduction), got %f", ipBucket.Tokens())
	}
}

func TestLimiter_TakeMulti_EmptyKeys(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 10, Rate: 5})

	r, err := l.TakeMulti([]string{}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.Allowed {
		t.Error("expected empty keys to be allowed")
	}
}

func TestLimiter_TakeMulti_EmptyKeyInSlice(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 10, Rate: 5})

	_, err := l.TakeMulti([]string{"valid", ""}, 5)
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestLimiter_TakeMulti_ZeroCount(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 10, Rate: 5})

	r, err := l.TakeMulti([]string{"key1", "key2"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.Allowed {
		t.Error("expected zero count to be allowed")
	}
}

func TestLimiter_SetRate(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 100, Rate: 10})

	l.Take("key1", 1)

	err := l.SetRate("key1", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b, _ := l.Bucket("key1")
	if b.Rate() != 50 {
		t.Errorf("expected rate 50, got %f", b.Rate())
	}
}

func TestLimiter_SetRate_NotFound(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 100, Rate: 10})

	err := l.SetRate("nonexistent", 50)
	if err != ErrBucketNotFound {
		t.Errorf("expected ErrBucketNotFound, got %v", err)
	}
}

func TestLimiter_SetCapacity(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 100, Rate: 10})

	l.Take("key1", 1)

	err := l.SetCapacity("key1", 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b, _ := l.Bucket("key1")
	if b.Capacity() != 200 {
		t.Errorf("expected capacity 200, got %f", b.Capacity())
	}
}

func TestLimiter_SetCapacity_NotFound(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 100, Rate: 10})

	err := l.SetCapacity("nonexistent", 200)
	if err != ErrBucketNotFound {
		t.Errorf("expected ErrBucketNotFound, got %v", err)
	}
}

func TestLimiter_SetAllRates(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 100, Rate: 10})

	l.Take("key1", 1)
	l.Take("key2", 1)

	err := l.SetAllRates(50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b1, _ := l.Bucket("key1")
	b2, _ := l.Bucket("key2")
	if b1.Rate() != 50 {
		t.Errorf("expected key1 rate 50, got %f", b1.Rate())
	}
	if b2.Rate() != 50 {
		t.Errorf("expected key2 rate 50, got %f", b2.Rate())
	}
}

func TestLimiter_SetAllRates_Invalid(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 100, Rate: 10})

	err := l.SetAllRates(0)
	if err != ErrInvalidRate {
		t.Errorf("expected ErrInvalidRate, got %v", err)
	}
}

func TestLimiter_SetAllCapacities(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 100, Rate: 10})

	l.Take("key1", 1)
	l.Take("key2", 1)

	err := l.SetAllCapacities(200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b1, _ := l.Bucket("key1")
	b2, _ := l.Bucket("key2")
	if b1.Capacity() != 200 {
		t.Errorf("expected key1 capacity 200, got %f", b1.Capacity())
	}
	if b2.Capacity() != 200 {
		t.Errorf("expected key2 capacity 200, got %f", b2.Capacity())
	}
}

func TestLimiter_SetAllCapacities_Invalid(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 100, Rate: 10})

	err := l.SetAllCapacities(-1)
	if err != ErrInvalidCapacity {
		t.Errorf("expected ErrInvalidCapacity, got %v", err)
	}
}

func TestLimiter_Remove(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 10, Rate: 5})

	l.Take("key1", 1)
	l.Remove("key1")

	_, ok := l.Bucket("key1")
	if ok {
		t.Error("expected bucket to be removed")
	}
}

func TestLimiter_Keys(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 10, Rate: 5})

	l.Take("alpha", 1)
	l.Take("beta", 1)
	l.Take("gamma", 1)

	keys := l.Keys()
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}
}

func TestLimiter_Bucket_NotFound(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 10, Rate: 5})

	_, ok := l.Bucket("nonexistent")
	if ok {
		t.Error("expected bucket not to exist")
	}
}

func TestResult_RetryAfterSeconds(t *testing.T) {
	r := Result{Allowed: false, RetryAfter: 1500 * time.Millisecond}
	if r.RetryAfterSeconds() != 2 {
		t.Errorf("expected 2 seconds (ceiling), got %d", r.RetryAfterSeconds())
	}

	r = Result{Allowed: false, RetryAfter: 1000 * time.Millisecond}
	if r.RetryAfterSeconds() != 1 {
		t.Errorf("expected 1 second (exact), got %d", r.RetryAfterSeconds())
	}

	r = Result{Allowed: true, RetryAfter: 0}
	if r.RetryAfterSeconds() != 0 {
		t.Errorf("expected 0 seconds (allowed), got %d", r.RetryAfterSeconds())
	}

	r = Result{Allowed: false, RetryAfter: 100 * time.Millisecond}
	if r.RetryAfterSeconds() != 1 {
		t.Errorf("expected 1 second (fraction rounds up), got %d", r.RetryAfterSeconds())
	}
}

func TestResult_String(t *testing.T) {
	r := Result{Allowed: true, Remaining: 5.5}
	s := r.String()
	if s == "" {
		t.Error("expected non-empty string")
	}

	r = Result{Allowed: false, RetryAfter: 2 * time.Second, Remaining: 0}
	s = r.String()
	if s == "" {
		t.Error("expected non-empty string")
	}
}

func TestBucket_ConcurrentAccess(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 1000, Rate: 10000})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	successCount := int64(0)
	failCount := int64(0)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := b.Take(1)
			if r.Allowed {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&failCount, 1)
			}
		}()
	}

	wg.Wait()

	total := successCount + failCount
	if total != 100 {
		t.Errorf("expected 100 total operations, got %d", total)
	}
	if successCount != 1000 {
		if successCount > 1000 {
			t.Errorf("expected at most 1000 successes, got %d", successCount)
		}
	}
}

func TestLimiter_ConcurrentAccess(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 100, Rate: 1000})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("user:%d", id%5)
			l.Take(key, 1)
		}(i)
	}
	wg.Wait()

	keys := l.Keys()
	if len(keys) != 5 {
		t.Errorf("expected 5 buckets, got %d", len(keys))
	}
}

func TestBucket_Warmup_FullCycle(t *testing.T) {
	b, err := NewBucket(BucketConfig{
		Capacity:       1000,
		Rate:           200,
		Warmup:         true,
		WarmupStartRate: 20,
		WarmupDuration:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !b.IsWarmingUp() {
		t.Error("expected warmup active at start")
	}

	b.mu.Lock()
	b.lastRefill = b.lastRefill.Add(-2500 * time.Millisecond)
	b.warmupStartTime = b.warmupStartTime.Add(-2500 * time.Millisecond)
	b.mu.Unlock()

	currentRate := b.CurrentRate()
	if currentRate < 100 || currentRate > 120 {
		t.Errorf("expected mid warmup rate ~110, got %f", currentRate)
	}
	if !b.IsWarmingUp() {
		t.Error("expected warmup still active at midpoint")
	}

	b.mu.Lock()
	b.lastRefill = b.lastRefill.Add(-2500 * time.Millisecond)
	b.warmupStartTime = b.warmupStartTime.Add(-2500 * time.Millisecond)
	b.mu.Unlock()

	if b.IsWarmingUp() {
		t.Error("expected warmup finished after duration")
	}
	if b.CurrentRate() != 200 {
		t.Errorf("expected full rate 200 after warmup, got %f", b.CurrentRate())
	}
}

func TestLimiter_MultiDimension_SameRequestMultipleConstraints(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 100, Rate: 10})

	l.Take("user:alice", 95)

	r, err := l.TakeMulti([]string{"user:alice", "ip:10.0.0.1"}, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Allowed {
		t.Error("expected rejection because user:alice has only 5 tokens")
	}

	aliceBucket, _ := l.Bucket("user:alice")
	if aliceBucket.Tokens() != 5 {
		t.Errorf("expected alice bucket unchanged at 5 (take rejected, no deduction), got %f", aliceBucket.Tokens())
	}

	ipBucket, _ := l.Bucket("ip:10.0.0.1")
	if ipBucket.Tokens() != 100 {
		t.Errorf("expected ip bucket untouched at 100 (take never attempted), got %f", ipBucket.Tokens())
	}
}

func TestLimiter_MultiDimension_AllPassThenReject(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 100, Rate: 10})

	r, err := l.TakeMulti([]string{"user:alice", "ip:10.0.0.1"}, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.Allowed {
		t.Error("expected first multi-take to succeed")
	}

	r, err = l.TakeMulti([]string{"user:alice", "ip:10.0.0.1"}, 60)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Allowed {
		t.Error("expected second multi-take to fail (both buckets low)")
	}
}

func TestBucket_DynamicRateChange_PreservesTokens(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 100, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.Take(30)

	err = b.SetRate(50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tokens := b.Tokens()
	if tokens < 69.5 || tokens > 70.5 {
		t.Errorf("expected ~70 tokens after rate change (preserved), got %f", tokens)
	}
}

func TestBucket_DynamicCapacityIncrease_PreservesTokens(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 50, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.Take(20)

	err = b.SetCapacity(200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tokens := b.Tokens()
	if tokens != 30 {
		t.Errorf("expected 30 tokens preserved after capacity increase, got %f", tokens)
	}
}

func TestBucket_FractionalTokens(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 1000, Rate: 1000})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.Take(1000)

	b.mu.Lock()
	b.lastRefill = b.lastRefill.Add(-500 * time.Millisecond)
	b.mu.Unlock()

	tokens := b.Tokens()
	if tokens < 499 || tokens > 501 {
		t.Errorf("expected ~500 tokens after 500ms at 1000/s, got %f", tokens)
	}

	r := b.Take(0.5)
	if !r.Allowed {
		t.Error("expected fractional take to succeed")
	}
}

func TestBucket_Warmup_RetryAfter(t *testing.T) {
	b, err := NewBucket(BucketConfig{
		Capacity:       10,
		Rate:           100,
		Warmup:         true,
		WarmupStartRate: 10,
		WarmupDuration:  10 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.Take(10)

	r := b.Take(10)
	if r.Allowed {
		t.Error("expected rejection")
	}
	if r.RetryAfter <= 0 {
		t.Error("expected positive retry after during warmup")
	}
}

func TestLimiter_RemoveNonExistent(t *testing.T) {
	l := NewLimiter(BucketConfig{Capacity: 10, Rate: 5})
	l.Remove("nonexistent")
}

func TestBucket_Take_Sequential(t *testing.T) {
	b, err := NewBucket(BucketConfig{Capacity: 10, Rate: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := 0; i < 10; i++ {
		r := b.Take(1)
		if !r.Allowed {
			t.Errorf("expected take %d to succeed", i+1)
		}
	}

	r := b.Take(1)
	if r.Allowed {
		t.Error("expected 11th take to fail")
	}
}
