package barrier

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew_InvalidParticipants(t *testing.T) {
	_, err := New(0)
	if !errors.Is(err, ErrInvalidParticipants) {
		t.Fatalf("expected ErrInvalidParticipants, got %v", err)
	}

	_, err = New(-1)
	if !errors.Is(err, ErrInvalidParticipants) {
		t.Fatalf("expected ErrInvalidParticipants, got %v", err)
	}
}

func TestNew_ValidParticipants(t *testing.T) {
	b, err := New(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil Barrier")
	}
	if b.Participants() != 3 {
		t.Fatalf("expected 3 participants, got %d", b.Participants())
	}
}

func TestNew_WithCallback(t *testing.T) {
	called := false
	cb := func() error {
		called = true
		return nil
	}
	b, err := New(1, cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = b.Wait(0)
	if err != nil {
		t.Fatalf("unexpected wait error: %v", err)
	}
	if !called {
		t.Fatal("callback should have been called")
	}
}

func TestWait_NormalRelease(t *testing.T) {
	const numGoroutines = 5
	b, err := New(numGoroutines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var releasedCount int64
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			err := b.Wait(time.Second)
			if err != nil {
				t.Errorf("unexpected wait error: %v", err)
				return
			}
			atomic.AddInt64(&releasedCount, 1)
		}()
	}

	time.Sleep(50 * time.Millisecond)
	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("test timed out waiting for goroutines")
	}

	if atomic.LoadInt64(&releasedCount) != int64(numGoroutines) {
		t.Fatalf("expected %d released goroutines, got %d", numGoroutines, releasedCount)
	}
}

func TestWait_SimultaneousRelease(t *testing.T) {
	const numGoroutines = 3
	b, err := New(numGoroutines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var releaseTimes [numGoroutines]time.Time
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			if idx > 0 {
				time.Sleep(time.Duration(idx) * 50 * time.Millisecond)
			}
			err := b.Wait(time.Second)
			if err != nil {
				t.Errorf("unexpected wait error: %v", err)
				return
			}
			mu.Lock()
			releaseTimes[idx] = time.Now()
			mu.Unlock()
		}()
	}

	wg.Wait()

	var minTime, maxTime time.Time
	for i, rt := range releaseTimes {
		if i == 0 || rt.Before(minTime) {
			minTime = rt
		}
		if i == 0 || rt.After(maxTime) {
			maxTime = rt
		}
	}

	diff := maxTime.Sub(minTime)
	if diff > 100*time.Millisecond {
		t.Fatalf("goroutines released too far apart: %v", diff)
	}
}

func TestWait_Timeout(t *testing.T) {
	b, err := New(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := b.Wait(100 * time.Millisecond)
		if !errors.Is(err, ErrTimeout) {
			t.Errorf("expected ErrTimeout, got %v", err)
		}
	}()

	wg.Wait()

	if b.Waiting() != 0 {
		t.Fatalf("expected 0 waiting, got %d", b.Waiting())
	}
	if b.EffectiveNeeded() != 2 {
		t.Fatalf("expected effective needed 2, got %d", b.EffectiveNeeded())
	}
}

func TestWait_TimeoutReducesParticipants(t *testing.T) {
	b, err := New(4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	var results [4]error
	started := make(chan struct{}, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		started <- struct{}{}
		results[0] = b.Wait(50 * time.Millisecond)
	}()

	<-started
	time.Sleep(80 * time.Millisecond)

	if b.EffectiveNeeded() != 3 {
		t.Fatalf("after timeout, expected effective needed 3, got %d", b.EffectiveNeeded())
	}

	wg.Add(3)
	for i := 1; i <= 3; i++ {
		idx := i
		go func() {
			defer wg.Done()
			results[idx] = b.Wait(time.Second)
		}()
	}

	wg.Wait()

	if !errors.Is(results[0], ErrTimeout) {
		t.Errorf("goroutine 0: expected ErrTimeout, got %v", results[0])
	}
	for i := 1; i <= 3; i++ {
		if results[i] != nil {
			t.Errorf("goroutine %d: expected no error, got %v", i, results[i])
		}
	}
}

func TestWait_AllTimeout(t *testing.T) {
	b, err := New(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	results := make([]error, 4)

	for i := 0; i < 4; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			results[idx] = b.Wait(time.Duration((idx+1)*40) * time.Millisecond)
		}()
	}

	wg.Wait()

	for i, r := range results {
		if !errors.Is(r, ErrTimeout) {
			t.Errorf("goroutine %d: expected ErrTimeout, got %v", i, r)
		}
	}

	if b.Waiting() != 0 {
		t.Fatalf("expected 0 waiting, got %d", b.Waiting())
	}
}

func TestWait_WithCallback(t *testing.T) {
	const numGoroutines = 3
	var callbackCalled int32
	var wg sync.WaitGroup

	cb := func() error {
		atomic.AddInt32(&callbackCalled, 1)
		return nil
	}

	b, err := New(numGoroutines, cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := b.Wait(time.Second)
			if err != nil {
				t.Errorf("unexpected wait error: %v", err)
			}
		}()
	}

	wg.Wait()

	if atomic.LoadInt32(&callbackCalled) != 1 {
		t.Fatalf("callback called %d times, expected 1", callbackCalled)
	}
}

func TestWait_CallbackErrorPropagated(t *testing.T) {
	const numGoroutines = 2
	expectedErr := errors.New("callback failed")

	cb := func() error {
		return expectedErr
	}

	b, err := New(numGoroutines, cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	var r1, r2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		r1 = b.Wait(time.Second)
	}()
	go func() {
		defer wg.Done()
		r2 = b.Wait(time.Second)
	}()

	wg.Wait()

	if !errors.Is(r1, expectedErr) {
		t.Errorf("goroutine 1: expected callback error, got %v", r1)
	}
	if !errors.Is(r2, expectedErr) {
		t.Errorf("goroutine 2: expected callback error, got %v", r2)
	}
}

func TestWait_CallbackErrorAfterTimeout(t *testing.T) {
	expectedErr := errors.New("callback failed")
	var mu sync.Mutex
	callCount := 0

	cb := func() error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return expectedErr
	}

	b, err := New(5, cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	var results [5]error
	started := make(chan struct{}, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		started <- struct{}{}
		results[0] = b.Wait(50 * time.Millisecond)
	}()

	<-started
	time.Sleep(100 * time.Millisecond)

	if b.EffectiveNeeded() != 4 {
		t.Fatalf("after timeout, expected effective needed 4, got %d", b.EffectiveNeeded())
	}

	for i := 1; i <= 4; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			results[idx] = b.Wait(time.Second)
		}()
	}

	wg.Wait()

	if !errors.Is(results[0], ErrTimeout) {
		t.Errorf("goroutine 0: expected ErrTimeout, got %v", results[0])
	}
	for i := 1; i <= 4; i++ {
		if !errors.Is(results[i], expectedErr) {
			t.Errorf("goroutine %d: expected callback error, got %v", i, results[i])
		}
	}

	mu.Lock()
	if callCount != 1 {
		t.Errorf("callback called %d times, expected 1", callCount)
	}
	mu.Unlock()
}

func TestReset_NoWaiters(t *testing.T) {
	b, err := New(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = b.Reset()
	if err != nil {
		t.Fatalf("unexpected reset error: %v", err)
	}

	if b.Participants() != 3 {
		t.Fatalf("expected 3 participants, got %d", b.Participants())
	}
}

func TestReset_WithNewParticipants(t *testing.T) {
	b, err := New(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = b.Reset(5)
	if err != nil {
		t.Fatalf("unexpected reset error: %v", err)
	}

	if b.Participants() != 5 {
		t.Fatalf("expected 5 participants, got %d", b.Participants())
	}
}

func TestReset_InvalidParticipants(t *testing.T) {
	b, err := New(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = b.Reset(0)
	if !errors.Is(err, ErrInvalidParticipants) {
		t.Fatalf("expected ErrInvalidParticipants, got %v", err)
	}

	err = b.Reset(-1)
	if !errors.Is(err, ErrInvalidParticipants) {
		t.Fatalf("expected ErrInvalidParticipants, got %v", err)
	}
}

func TestReset_WhileWaiting(t *testing.T) {
	b, err := New(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.Wait(200 * time.Millisecond)
	}()

	time.Sleep(20 * time.Millisecond)

	err = b.Reset()
	if !errors.Is(err, ErrResetWhileWaiting) {
		t.Fatalf("expected ErrResetWhileWaiting, got %v", err)
	}

	wg.Wait()
}

func TestReset_Reuse(t *testing.T) {
	b, err := New(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := b.Wait(time.Second); err != nil {
			t.Errorf("round 1: unexpected error: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := b.Wait(time.Second); err != nil {
			t.Errorf("round 1: unexpected error: %v", err)
		}
	}()
	wg.Wait()

	err = b.Reset()
	if err != nil {
		t.Fatalf("reset error: %v", err)
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := b.Wait(time.Second); err != nil {
			t.Errorf("round 2: unexpected error: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := b.Wait(time.Second); err != nil {
			t.Errorf("round 2: unexpected error: %v", err)
		}
	}()
	wg.Wait()
}

func TestReset_ChangeParticipants(t *testing.T) {
	b, err := New(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		b.Wait(time.Second)
	}()
	go func() {
		defer wg.Done()
		b.Wait(time.Second)
	}()
	wg.Wait()

	err = b.Reset(3)
	if err != nil {
		t.Fatalf("reset error: %v", err)
	}

	var r1, r2 error
	wg.Add(2)
	go func() {
		defer wg.Done()
		r1 = b.Wait(100 * time.Millisecond)
	}()
	go func() {
		defer wg.Done()
		r2 = b.Wait(100 * time.Millisecond)
	}()
	wg.Wait()

	if !errors.Is(r1, ErrTimeout) {
		t.Errorf("expected timeout for r1, got %v", r1)
	}
	if !errors.Is(r2, ErrTimeout) {
		t.Errorf("expected timeout for r2, got %v", r2)
	}
}

func TestForceReset(t *testing.T) {
	b, err := New(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	var r1, r2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		r1 = b.Wait(time.Second)
	}()
	go func() {
		defer wg.Done()
		r2 = b.Wait(time.Second)
	}()

	time.Sleep(20 * time.Millisecond)

	err = b.ForceReset()
	if err != nil {
		t.Fatalf("unexpected force reset error: %v", err)
	}

	wg.Wait()

	if !errors.Is(r1, ErrBarrierReset) {
		t.Errorf("goroutine 1: expected ErrBarrierReset, got %v", r1)
	}
	if !errors.Is(r2, ErrBarrierReset) {
		t.Errorf("goroutine 2: expected ErrBarrierReset, got %v", r2)
	}

	if b.Waiting() != 0 {
		t.Fatalf("expected 0 waiting, got %d", b.Waiting())
	}
}

func TestBreak(t *testing.T) {
	b, err := New(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	var r1, r2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		r1 = b.Wait(time.Second)
	}()
	go func() {
		defer wg.Done()
		r2 = b.Wait(time.Second)
	}()

	time.Sleep(20 * time.Millisecond)

	b.Break()

	wg.Wait()

	if !errors.Is(r1, ErrBroken) {
		t.Errorf("goroutine 1: expected ErrBroken, got %v", r1)
	}
	if !errors.Is(r2, ErrBroken) {
		t.Errorf("goroutine 2: expected ErrBroken, got %v", r2)
	}

	if !b.IsBroken() {
		t.Fatal("expected barrier to be broken")
	}

	r3 := b.Wait(time.Second)
	if !errors.Is(r3, ErrBroken) {
		t.Errorf("goroutine 3: expected ErrBroken, got %v", r3)
	}
}

func TestSetCallback(t *testing.T) {
	b, err := New(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	called := false
	b.SetCallback(func() error {
		called = true
		return nil
	})

	err = b.Wait(0)
	if err != nil {
		t.Fatalf("unexpected wait error: %v", err)
	}

	if !called {
		t.Fatal("callback should have been called")
	}
}

func TestSingleParticipant(t *testing.T) {
	b, err := New(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = b.Wait(time.Second)
	if err != nil {
		t.Fatalf("unexpected wait error: %v", err)
	}
}

func TestWait_NoTimeout(t *testing.T) {
	const numGoroutines = 3
	b, err := New(numGoroutines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	var count int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := b.Wait(0)
			if err != nil {
				t.Errorf("unexpected wait error: %v", err)
			}
			atomic.AddInt64(&count, 1)
		}()
	}

	wg.Wait()

	if atomic.LoadInt64(&count) != int64(numGoroutines) {
		t.Fatalf("expected %d, got %d", numGoroutines, count)
	}
}

func TestWait_CallbackOrder(t *testing.T) {
	const numGoroutines = 3
	var callbackDone bool
	var mu sync.Mutex

	cb := func() error {
		mu.Lock()
		callbackDone = true
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	b, err := New(numGoroutines, cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	var goroutinesBeforeCallback int32

	checkReleased := func() {
		defer wg.Done()
		err := b.Wait(time.Second)
		if err != nil {
			t.Errorf("unexpected wait error: %v", err)
			return
		}
		mu.Lock()
		if !callbackDone {
			atomic.AddInt32(&goroutinesBeforeCallback, 1)
		}
		mu.Unlock()
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go checkReleased()
	}

	wg.Wait()

	if atomic.LoadInt32(&goroutinesBeforeCallback) > 0 {
		t.Fatalf("%d goroutines released before callback completed", goroutinesBeforeCallback)
	}
}

func TestHighConcurrency(t *testing.T) {
	const numGoroutines = 100
	b, err := New(numGoroutines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	var released int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := b.Wait(5 * time.Second)
			if err != nil {
				t.Errorf("unexpected wait error: %v", err)
				return
			}
			atomic.AddInt64(&released, 1)
		}()
	}

	wg.Wait()

	if atomic.LoadInt64(&released) != int64(numGoroutines) {
		t.Fatalf("expected %d released, got %d", numGoroutines, released)
	}
}

func TestArrivedAndWaiting(t *testing.T) {
	b, err := New(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if b.Arrived() != 0 {
		t.Fatalf("expected 0 arrived, got %d", b.Arrived())
	}
	if b.Waiting() != 0 {
		t.Fatalf("expected 0 waiting, got %d", b.Waiting())
	}

	var wg sync.WaitGroup
	wg.Add(2)

	startWaiters := make(chan struct{})
	go func() {
		defer wg.Done()
		<-startWaiters
		b.Wait(200 * time.Millisecond)
	}()
	go func() {
		defer wg.Done()
		<-startWaiters
		b.Wait(200 * time.Millisecond)
	}()

	close(startWaiters)
	time.Sleep(30 * time.Millisecond)

	if b.Arrived() != 2 {
		t.Fatalf("expected 2 arrived, got %d", b.Arrived())
	}
	if b.Waiting() != 2 {
		t.Fatalf("expected 2 waiting, got %d", b.Waiting())
	}

	wg.Wait()

	if b.Arrived() != 0 {
		t.Fatalf("expected 0 arrived after timeout, got %d", b.Arrived())
	}
	if b.Waiting() != 0 {
		t.Fatalf("expected 0 waiting after timeout, got %d", b.Waiting())
	}
}

func TestCyclicBarrier_Basic(t *testing.T) {
	cb, err := NewCyclic(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for round := 0; round < 3; round++ {
		var wg sync.WaitGroup
		var count int64

		wg.Add(2)
		go func() {
			defer wg.Done()
			err := cb.Await(time.Second)
			if err != nil {
				t.Errorf("round %d: unexpected error: %v", round, err)
			}
			atomic.AddInt64(&count, 1)
		}()
		go func() {
			defer wg.Done()
			err := cb.Await(time.Second)
			if err != nil {
				t.Errorf("round %d: unexpected error: %v", round, err)
			}
			atomic.AddInt64(&count, 1)
		}()

		wg.Wait()

		if atomic.LoadInt64(&count) != 2 {
			t.Fatalf("round %d: expected 2, got %d", round, count)
		}
	}
}

func TestCyclicBarrier_WithCallback(t *testing.T) {
	var callbackCount int32
	var lastRound uint64
	var mu sync.Mutex

	cbFunc := func(round uint64) error {
		atomic.AddInt32(&callbackCount, 1)
		mu.Lock()
		lastRound = round
		mu.Unlock()
		return nil
	}

	cb, err := NewCyclic(2, cbFunc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for round := 0; round < 3; round++ {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			cb.Await(time.Second)
		}()
		go func() {
			defer wg.Done()
			cb.Await(time.Second)
		}()
		wg.Wait()
	}

	if atomic.LoadInt32(&callbackCount) != 3 {
		t.Fatalf("expected 3 callback invocations, got %d", callbackCount)
	}

	mu.Lock()
	if lastRound != 2 {
		t.Fatalf("expected last round 2, got %d", lastRound)
	}
	mu.Unlock()
}

func TestCyclicBarrier_Timeout(t *testing.T) {
	cb, err := NewCyclic(4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	var results [4]error
	started := make(chan struct{}, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		started <- struct{}{}
		results[0] = cb.Await(50 * time.Millisecond)
	}()

	<-started
	time.Sleep(100 * time.Millisecond)

	for i := 1; i <= 3; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			results[idx] = cb.Await(time.Second)
		}()
	}

	wg.Wait()

	if !errors.Is(results[0], ErrTimeout) {
		t.Errorf("goroutine 0: expected ErrTimeout, got %v", results[0])
	}
	for i := 1; i <= 3; i++ {
		if results[i] != nil {
			t.Errorf("goroutine %d: expected no error, got %v", i, results[i])
		}
	}
}

func TestCyclicBarrier_GetPartiesAndWaiting(t *testing.T) {
	cb, err := NewCyclic(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cb.GetParties() != 5 {
		t.Fatalf("expected 5 parties, got %d", cb.GetParties())
	}
	if cb.GetNumberWaiting() != 0 {
		t.Fatalf("expected 0 waiting, got %d", cb.GetNumberWaiting())
	}
}

func TestReset_AfterBroken(t *testing.T) {
	b, err := New(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b.Break()

	if !b.IsBroken() {
		t.Fatal("expected barrier to be broken")
	}

	err = b.Reset()
	if err != nil {
		t.Fatalf("unexpected reset error: %v", err)
	}

	if b.IsBroken() {
		t.Fatal("barrier should not be broken after reset")
	}

	var wg sync.WaitGroup
	var count int64
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := b.Wait(time.Second); err == nil {
				atomic.AddInt64(&count, 1)
			}
		}()
	}
	wg.Wait()

	if count != 3 {
		t.Fatalf("expected 3, got %d", count)
	}
}

func TestMultipleTimeoutsCascadeRelease(t *testing.T) {
	b, err := New(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	results := make([]error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		idx := i
		timeout := time.Duration((idx+1)*30) * time.Millisecond
		go func() {
			defer wg.Done()
			results[idx] = b.Wait(timeout)
		}()
	}

	wg.Wait()

	timeoutCount := 0
	successCount := 0
	for _, r := range results {
		if errors.Is(r, ErrTimeout) {
			timeoutCount++
		} else if r == nil {
			successCount++
		}
	}

	if timeoutCount+successCount != 5 {
		t.Fatalf("unexpected results: timeouts=%d, successes=%d", timeoutCount, successCount)
	}

	if successCount == 0 {
		t.Fatal("at least one goroutine should have succeeded")
	}
}

func TestCallback_CallsBarrierMethods_NoDeadlock(t *testing.T) {
	b, err := New(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var participants int
	var arrived int
	var waiting int

	cb := func() error {
		participants = b.Participants()
		arrived = b.Arrived()
		waiting = b.Waiting()
		return nil
	}
	b.SetCallback(cb)

	done := make(chan struct{})
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				b.Wait(time.Second)
			}()
		}
		wg.Wait()
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("callback calling barrier methods caused deadlock")
	}

	if participants != 2 {
		t.Errorf("expected participants=2 in callback, got %d", participants)
	}
	_ = arrived
	_ = waiting
}

func TestIsReleased(t *testing.T) {
	b, err := New(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if b.IsReleased() {
		t.Fatal("expected IsReleased should be false initially")
	}

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Wait(time.Second)
		}()
	}
	wg.Wait()

	if !b.IsReleased() {
		t.Fatal("expected IsReleased to be true after release")
	}

	b.Reset()

	if b.IsReleased() {
		t.Fatal("expected IsReleased to be false after reset")
	}
}

func TestCyclicBarrier_GetRound(t *testing.T) {
	cb, err := NewCyclic(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cb.GetRound() != 0 {
		t.Fatalf("expected round 0, got %d", cb.GetRound())
	}

	for round := 0; round < 3; round++ {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			cb.Await(time.Second)
		}()
		go func() {
			defer wg.Done()
			cb.Await(time.Second)
		}()
		wg.Wait()

		if cb.GetRound() != uint64(round) {
			t.Fatalf("round %d: expected round %d, got %d", round, round, cb.GetRound())
		}
	}
}

func TestCyclicBarrier_ResetBarrier(t *testing.T) {
	cb, err := NewCyclic(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := 0; i < 3; i++ {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			cb.Await(time.Second)
		}()
		go func() {
			defer wg.Done()
			cb.Await(time.Second)
		}()
		wg.Wait()
	}

	if cb.GetRound() != 2 {
		t.Fatalf("expected round 2, got %d", cb.GetRound())
	}

	err = cb.ResetBarrier(3)
	if err != nil {
		t.Fatalf("unexpected reset error: %v", err)
	}

	if cb.GetRound() != 0 {
		t.Fatalf("expected round 0 after reset, got %d", cb.GetRound())
	}

	if cb.GetParties() != 3 {
		t.Fatalf("expected 3 parties after reset, got %d", cb.GetParties())
	}
}

func TestCyclicBarrier_ResetWhileWaiting(t *testing.T) {
	cb, err := NewCyclic(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		cb.Await(200 * time.Millisecond)
	}()

	time.Sleep(50 * time.Millisecond)

	err = cb.ResetBarrier()
	if !errors.Is(err, ErrResetWhileWaiting) {
		t.Fatalf("expected ErrResetWhileWaiting, got %v", err)
	}

	wg.Wait()
}

func TestCyclicBarrier_SetCallback(t *testing.T) {
	cb, err := NewCyclic(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var lastRound uint64 = 999
	cb.SetCallback(func(round uint64) error {
		lastRound = round
		return nil
	})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		cb.Await(time.Second)
	}()
	go func() {
		defer wg.Done()
		cb.Await(time.Second)
	}()
	wg.Wait()

	if lastRound != 0 {
		t.Fatalf("expected lastRound 0, got %d", lastRound)
	}
}
