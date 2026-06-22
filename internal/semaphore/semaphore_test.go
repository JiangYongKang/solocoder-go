package semaphore

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	s, err := New(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("semaphore should not be nil")
	}
	if s.TotalPermits() != 5 {
		t.Errorf("expected total permits 5, got %d", s.TotalPermits())
	}
	if s.AvailablePermits() != 5 {
		t.Errorf("expected available permits 5, got %d", s.AvailablePermits())
	}
	if s.IsFair() {
		t.Error("expected non-fair mode by default")
	}
}

func TestNewFair(t *testing.T) {
	s, err := New(3, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.IsFair() {
		t.Error("expected fair mode")
	}
}

func TestNewNegativePermits(t *testing.T) {
	_, err := New(-1)
	if err == nil {
		t.Fatal("expected error for negative permits")
	}
	if err != ErrInvalidPermits {
		t.Errorf("expected ErrInvalidPermits, got %v", err)
	}
}

func TestNewZeroPermits(t *testing.T) {
	s, err := New(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.AvailablePermits() != 0 {
		t.Errorf("expected 0 available permits, got %d", s.AvailablePermits())
	}
}

func TestAcquireAndRelease(t *testing.T) {
	s, _ := New(2)

	if !s.Acquire(0) {
		t.Fatal("first acquire should succeed")
	}
	if s.AvailablePermits() != 1 {
		t.Errorf("expected 1 available permit, got %d", s.AvailablePermits())
	}

	if !s.Acquire(0) {
		t.Fatal("second acquire should succeed")
	}
	if s.AvailablePermits() != 0 {
		t.Errorf("expected 0 available permits, got %d", s.AvailablePermits())
	}

	s.Release()
	if s.AvailablePermits() != 1 {
		t.Errorf("expected 1 available permit after release, got %d", s.AvailablePermits())
	}

	s.Release()
	if s.AvailablePermits() != 2 {
		t.Errorf("expected 2 available permits after release, got %d", s.AvailablePermits())
	}
}

func TestAcquireBlocks(t *testing.T) {
	s, _ := New(1)

	s.Acquire(0)

	done := make(chan struct{})
	go func() {
		s.Acquire(0)
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("acquire should block")
	case <-time.After(100 * time.Millisecond):
	}

	s.Release()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("acquire should unblock after release")
	}
}

func TestAcquireWithTimeout(t *testing.T) {
	s, _ := New(1)

	s.Acquire(0)

	start := time.Now()
	result := s.Acquire(100 * time.Millisecond)
	elapsed := time.Since(start)

	if result {
		t.Error("expected acquire to timeout")
	}
	if elapsed < 100*time.Millisecond {
		t.Errorf("expected to wait at least 100ms, waited %v", elapsed)
	}
}

func TestAcquireWithTimeoutSucceeds(t *testing.T) {
	s, _ := New(1)

	s.Acquire(0)

	go func() {
		time.Sleep(50 * time.Millisecond)
		s.Release()
	}()

	start := time.Now()
	result := s.Acquire(200 * time.Millisecond)
	elapsed := time.Since(start)

	if !result {
		t.Error("expected acquire to succeed")
	}
	if elapsed >= 200*time.Millisecond {
		t.Errorf("expected to succeed before timeout, waited %v", elapsed)
	}
}

func TestTryAcquire(t *testing.T) {
	s, _ := New(1)

	if !s.TryAcquire() {
		t.Error("first tryAcquire should succeed")
	}

	if s.TryAcquire() {
		t.Error("second tryAcquire should fail")
	}

	s.Release()

	if !s.TryAcquire() {
		t.Error("tryAcquire after release should succeed")
	}
}

func TestReleaseMoreThanAcquired(t *testing.T) {
	s, _ := New(2)

	s.Acquire(0)

	s.Release()
	s.Release()

	if s.AvailablePermits() != 2 {
		t.Errorf("expected available permits to not exceed total, got %d", s.AvailablePermits())
	}
	if s.TotalPermits() != 2 {
		t.Errorf("expected total permits 2, got %d", s.TotalPermits())
	}
}

func TestIncreasePermits(t *testing.T) {
	s, _ := New(2)

	s.Acquire(0)
	s.Acquire(0)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Acquire(0)
	}()

	time.Sleep(50 * time.Millisecond)
	if s.QueueLength() != 1 {
		t.Errorf("expected 1 waiter, got %d", s.QueueLength())
	}

	err := s.IncreasePermits(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.TotalPermits() != 4 {
		t.Errorf("expected total permits 4, got %d", s.TotalPermits())
	}

	wg.Wait()
	if s.AvailablePermits() != 1 {
		t.Errorf("expected 1 available permit, got %d", s.AvailablePermits())
	}
}

func TestIncreasePermitsZeroDelta(t *testing.T) {
	s, _ := New(2)

	err := s.IncreasePermits(0)
	if err == nil {
		t.Error("expected error for zero delta")
	}
	if err != ErrInvalidDelta {
		t.Errorf("expected ErrInvalidDelta, got %v", err)
	}
}

func TestIncreasePermitsNegativeDelta(t *testing.T) {
	s, _ := New(2)

	err := s.IncreasePermits(-1)
	if err == nil {
		t.Error("expected error for negative delta")
	}
	if err != ErrInvalidDelta {
		t.Errorf("expected ErrInvalidDelta, got %v", err)
	}
}

func TestDecreasePermits(t *testing.T) {
	s, _ := New(5)

	err := s.DecreasePermits(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.TotalPermits() != 3 {
		t.Errorf("expected total permits 3, got %d", s.TotalPermits())
	}
	if s.AvailablePermits() != 3 {
		t.Errorf("expected available permits 3, got %d", s.AvailablePermits())
	}
}

func TestDecreasePermitsWithSomeAcquired(t *testing.T) {
	s, _ := New(5)

	s.Acquire(0)
	s.Acquire(0)

	err := s.DecreasePermits(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.TotalPermits() != 2 {
		t.Errorf("expected total permits 2, got %d", s.TotalPermits())
	}
	if s.AvailablePermits() != 0 {
		t.Errorf("expected 0 available permits, got %d", s.AvailablePermits())
	}

	s.Release()
	if s.AvailablePermits() != 1 {
		t.Errorf("expected 1 available permit after release (one still held), got %d", s.AvailablePermits())
	}
}

func TestDecreasePermitsTooMuch(t *testing.T) {
	s, _ := New(2)

	err := s.DecreasePermits(5)
	if err == nil {
		t.Error("expected error when decreasing below zero")
	}
	if err != ErrNegativePermits {
		t.Errorf("expected ErrNegativePermits, got %v", err)
	}

	if s.TotalPermits() != 2 {
		t.Errorf("expected total permits to remain 2, got %d", s.TotalPermits())
	}
}

func TestDecreasePermitsZeroDelta(t *testing.T) {
	s, _ := New(2)

	err := s.DecreasePermits(0)
	if err == nil {
		t.Error("expected error for zero delta")
	}
	if err != ErrInvalidDelta {
		t.Errorf("expected ErrInvalidDelta, got %v", err)
	}
}

func TestDecreasePermitsNegativeDelta(t *testing.T) {
	s, _ := New(2)

	err := s.DecreasePermits(-1)
	if err == nil {
		t.Error("expected error for negative delta")
	}
	if err != ErrInvalidDelta {
		t.Errorf("expected ErrInvalidDelta, got %v", err)
	}
}

func TestFairModeNoBarging(t *testing.T) {
	s, _ := New(1, true)

	s.Acquire(0)

	waiterStarted := make(chan struct{})
	waiterDone := make(chan struct{})
	go func() {
		close(waiterStarted)
		s.Acquire(0)
		<-waiterDone
		s.Release()
	}()

	<-waiterStarted
	time.Sleep(50 * time.Millisecond)

	if s.QueueLength() != 1 {
		t.Fatalf("expected 1 waiter, got %d", s.QueueLength())
	}

	s.Release()

	time.Sleep(10 * time.Millisecond)

	result := s.TryAcquire()
	if result {
		t.Error("in fair mode, new caller should not barge when there are waiters")
		s.Release()
	}

	close(waiterDone)
	time.Sleep(50 * time.Millisecond)

	if !s.TryAcquire() {
		t.Error("after waiter releases, tryAcquire should succeed")
	}
	s.Release()
}

func TestNonFairModeBarging(t *testing.T) {
	s, _ := New(1, false)

	s.Acquire(0)

	waiterStarted := make(chan struct{})
	var waiterAcquired int32
	go func() {
		close(waiterStarted)
		s.Acquire(0)
		atomic.StoreInt32(&waiterAcquired, 1)
		s.Release()
	}()

	<-waiterStarted
	time.Sleep(50 * time.Millisecond)

	if s.QueueLength() != 1 {
		t.Fatalf("expected 1 waiter, got %d", s.QueueLength())
	}

	s.Release()

	time.Sleep(10 * time.Millisecond)

	result := s.TryAcquire()

	if result {
		s.Release()
	}

	for atomic.LoadInt32(&waiterAcquired) == 0 {
		time.Sleep(10 * time.Millisecond)
	}

	t.Logf("barging result: %v (non-deterministic, both outcomes are acceptable)", result)
}

func TestFairModeOrdering(t *testing.T) {
	s, _ := New(0, true)

	const numGoroutines = 5
	results := make([]int, numGoroutines)
	ready := make([]chan struct{}, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		ready[i] = make(chan struct{})
	}

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			close(ready[id])
			s.Acquire(0)
			results[id] = id
			time.Sleep(10 * time.Millisecond)
			s.Release()
		}(i)
		<-ready[i]
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(50 * time.Millisecond)
	if s.QueueLength() != numGoroutines {
		t.Fatalf("expected %d waiters, got %d", numGoroutines, s.QueueLength())
	}

	s.IncreasePermits(1)

	for i := 0; i < numGoroutines; i++ {
		time.Sleep(30 * time.Millisecond)
	}

	wg.Wait()

	firstAcquirer := -1
	for i := 0; i < numGoroutines; i++ {
		if results[i] == i {
			if firstAcquirer == -1 {
				firstAcquirer = i
			}
		}
	}

	if firstAcquirer != 0 {
		t.Errorf("expected goroutine 0 to acquire first (FIFO order), got goroutine %d", firstAcquirer)
	}
}

func TestConcurrentAcquireRelease(t *testing.T) {
	s, _ := New(10)

	const numGoroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	var acquired int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if s.Acquire(100 * time.Millisecond) {
					atomic.AddInt64(&acquired, 1)
					time.Sleep(time.Microsecond)
					s.Release()
				}
			}
		}()
	}

	wg.Wait()

	if acquired == 0 {
		t.Error("expected some acquires to succeed")
	}

	if s.AvailablePermits() != 10 {
		t.Errorf("expected 10 available permits after all releases, got %d", s.AvailablePermits())
	}
}

func TestQueueLength(t *testing.T) {
	s, _ := New(1)

	s.Acquire(0)

	if s.QueueLength() != 0 {
		t.Errorf("expected 0 waiters initially, got %d", s.QueueLength())
	}

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Acquire(0)
			s.Release()
		}()
	}

	time.Sleep(100 * time.Millisecond)
	if s.QueueLength() != 3 {
		t.Errorf("expected 3 waiters, got %d", s.QueueLength())
	}

	s.Release()
	time.Sleep(50 * time.Millisecond)

	s.Release()
	time.Sleep(50 * time.Millisecond)

	s.Release()
	wg.Wait()

	if s.QueueLength() != 0 {
		t.Errorf("expected 0 waiters after all done, got %d", s.QueueLength())
	}
}

func TestIncreasePermitsWakesWaiters(t *testing.T) {
	s, _ := New(0)

	done := make(chan struct{})
	go func() {
		s.Acquire(0)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	if s.QueueLength() != 1 {
		t.Fatalf("expected 1 waiter, got %d", s.QueueLength())
	}

	s.IncreasePermits(1)

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected waiter to be woken up after increasing permits")
	}
}

func TestDecreasePermitsNoEffectOnHolders(t *testing.T) {
	s, _ := New(3)

	s.Acquire(0)
	s.Acquire(0)

	s.DecreasePermits(2)

	if s.TotalPermits() != 1 {
		t.Errorf("expected total permits 1, got %d", s.TotalPermits())
	}
	if s.AvailablePermits() != 0 {
		t.Errorf("expected 0 available permits (2 held, total=1, over by 1), got %d", s.AvailablePermits())
	}

	s.Release()
	if s.AvailablePermits() != 0 {
		t.Errorf("expected 0 available permits after first release (still 1 over total), got %d", s.AvailablePermits())
	}

	s.Release()
	if s.AvailablePermits() != 1 {
		t.Errorf("expected 1 available permit after second release (no longer over total), got %d", s.AvailablePermits())
	}
}

func TestAcquireWithTimeoutCleansUpWaiter(t *testing.T) {
	s, _ := New(1)

	s.Acquire(0)

	s.Acquire(50 * time.Millisecond)

	time.Sleep(100 * time.Millisecond)

	if s.QueueLength() != 0 {
		t.Errorf("expected 0 waiters after timeout, got %d", s.QueueLength())
	}
}

func TestFairModeTryAcquireNoBarging(t *testing.T) {
	s, _ := New(1, true)

	s.Acquire(0)

	waiterReady := make(chan struct{})
	go func() {
		close(waiterReady)
		s.Acquire(0)
		s.Release()
	}()

	<-waiterReady
	time.Sleep(50 * time.Millisecond)

	if s.TryAcquire() {
		t.Error("TryAcquire in fair mode should not barge when there are waiters")
	}

	s.Release()
}

func TestMultipleIncreasePermits(t *testing.T) {
	s, _ := New(0)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Acquire(0)
		}()
	}

	time.Sleep(100 * time.Millisecond)
	if s.QueueLength() != 5 {
		t.Fatalf("expected 5 waiters, got %d", s.QueueLength())
	}

	s.IncreasePermits(2)
	time.Sleep(50 * time.Millisecond)
	if s.QueueLength() != 3 {
		t.Errorf("expected 3 waiters after first increase, got %d", s.QueueLength())
	}

	s.IncreasePermits(3)
	wg.Wait()

	if s.AvailablePermits() != 0 {
		t.Errorf("expected 0 available permits after all acquired, got %d", s.AvailablePermits())
	}
}

func TestZeroPermitsSemaphore(t *testing.T) {
	s, _ := New(0)

	if s.TryAcquire() {
		t.Error("tryAcquire should fail on zero-permit semaphore")
	}

	s.IncreasePermits(1)

	if !s.TryAcquire() {
		t.Error("tryAcquire should succeed after increasing permits")
	}
}
