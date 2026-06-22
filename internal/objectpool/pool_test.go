package objectpool

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testObj struct {
	id     int
	closed bool
}

func makeFactory() Factory[*testObj] {
	var counter int64
	return func() (*testObj, error) {
		id := atomic.AddInt64(&counter, 1)
		return &testObj{id: int(id), closed: false}, nil
	}
}

func makeDestroy(destroyed *[]int, mu *sync.Mutex) DestroyFunc[*testObj] {
	return func(obj *testObj) {
		if mu != nil {
			mu.Lock()
		}
		*destroyed = append(*destroyed, obj.id)
		if mu != nil {
			mu.Unlock()
		}
		obj.closed = true
	}
}

func TestNewPool_MissingFactory(t *testing.T) {
	_, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  5,
		Factory: nil,
	})
	if err == nil {
		t.Fatal("expected error for missing factory")
	}
}

func TestNewPool_InvalidMaxCap(t *testing.T) {
	_, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  0,
		Factory: makeFactory(),
	})
	if err == nil {
		t.Fatal("expected error for MaxCap <= 0")
	}

	_, err = NewPool[*testObj](Config[*testObj]{
		MaxCap:  -1,
		Factory: makeFactory(),
	})
	if err == nil {
		t.Fatal("expected error for MaxCap < 0")
	}
}

func TestNewPool_ValidConfig(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  5,
		Factory: makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	if p.Len() != 0 {
		t.Errorf("expected Len=0, got %d", p.Len())
	}
	if p.IdleCount() != 0 {
		t.Errorf("expected IdleCount=0, got %d", p.IdleCount())
	}
	if p.ActiveCount() != 0 {
		t.Errorf("expected ActiveCount=0, got %d", p.ActiveCount())
	}
}

func TestNewPool_WithMaxIdleTime(t *testing.T) {
	var destroyed []int
	var mu sync.Mutex

	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:      5,
		MaxIdleTime: 50 * time.Millisecond,
		Factory:     makeFactory(),
		Destroy:     makeDestroy(&destroyed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	if p.cfg.CleanupInterval != 25*time.Millisecond {
		t.Errorf("expected CleanupInterval=25ms, got %v", p.cfg.CleanupInterval)
	}
}

func TestNewPool_DefaultDestroy(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  2,
		Factory: makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj, err := p.Acquire()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p.Close()

	_ = obj
}

func TestAcquireAndRelease_Single(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  3,
		Factory: makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	obj, err := p.Acquire()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj == nil {
		t.Fatal("expected non-nil object")
	}
	if obj.closed {
		t.Fatal("expected object to not be closed")
	}
	if p.ActiveCount() != 1 {
		t.Errorf("expected ActiveCount=1, got %d", p.ActiveCount())
	}
	if p.IdleCount() != 0 {
		t.Errorf("expected IdleCount=0, got %d", p.IdleCount())
	}
	if p.Len() != 1 {
		t.Errorf("expected Len=1, got %d", p.Len())
	}

	err = p.Release(obj)
	if err != nil {
		t.Fatalf("unexpected error on Release: %v", err)
	}
	if p.ActiveCount() != 0 {
		t.Errorf("expected ActiveCount=0 after Release, got %d", p.ActiveCount())
	}
	if p.IdleCount() != 1 {
		t.Errorf("expected IdleCount=1 after Release, got %d", p.IdleCount())
	}
}

func TestAcquireAndRelease_Multiple(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  3,
		Factory: makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	objs := make([]*testObj, 3)
	for i := 0; i < 3; i++ {
		objs[i], err = p.Acquire()
		if err != nil {
			t.Fatalf("Acquire %d failed: %v", i, err)
		}
	}

	if p.Len() != 3 {
		t.Errorf("expected Len=3, got %d", p.Len())
	}
	if p.ActiveCount() != 3 {
		t.Errorf("expected ActiveCount=3, got %d", p.ActiveCount())
	}
	if p.IdleCount() != 0 {
		t.Errorf("expected IdleCount=0, got %d", p.IdleCount())
	}

	_, err = p.Acquire()
	if err != ErrPoolExhausted {
		t.Errorf("expected ErrPoolExhausted, got %v", err)
	}

	for i, obj := range objs {
		if err := p.Release(obj); err != nil {
			t.Fatalf("Release %d failed: %v", i, err)
		}
	}

	if p.IdleCount() != 3 {
		t.Errorf("expected IdleCount=3, got %d", p.IdleCount())
	}
}

func TestAcquire_ReuseFromIdle(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  2,
		Factory: makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	obj1, _ := p.Acquire()
	id1 := obj1.id
	_ = p.Release(obj1)

	obj2, _ := p.Acquire()
	if obj2.id != id1 {
		t.Errorf("expected to reuse same object (id=%d), got id=%d", id1, obj2.id)
	}
	_ = p.Release(obj2)
}

func TestAcquire_PoolExhausted_NoWait(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:     1,
		WaitTimeout: 0,
		Factory:    makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	_, _ = p.Acquire()

	_, err = p.Acquire()
	if err != ErrPoolExhausted {
		t.Errorf("expected ErrPoolExhausted, got %v", err)
	}
}

func TestAcquire_WaitTimeout(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:      1,
		WaitTimeout: 100 * time.Millisecond,
		Factory:     makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	obj, _ := p.Acquire()

	start := time.Now()
	_, err = p.Acquire()
	elapsed := time.Since(start)
	if err != ErrPoolExhausted {
		t.Errorf("expected ErrPoolExhausted, got %v", err)
	}
	if elapsed < 80*time.Millisecond {
		t.Errorf("expected to wait ~100ms, waited %v", elapsed)
	}

	_ = p.Release(obj)
}

func TestAcquire_WaitTimeout_Success(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:      1,
		WaitTimeout: 500 * time.Millisecond,
		Factory:     makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	obj1, _ := p.Acquire()

	var obj2 *testObj
	var getErr error
	done := make(chan struct{})
	go func() {
		obj2, getErr = p.Acquire()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	_ = p.Release(obj1)

	<-done
	if getErr != nil {
		t.Fatalf("expected successful Acquire after Release, got: %v", getErr)
	}
	if obj2 == nil {
		t.Fatal("expected non-nil object")
	}
	if obj2.id != obj1.id {
		t.Errorf("expected to reuse same object, got different id")
	}
	_ = p.Release(obj2)
}

func TestAcquire_WaitTimeout_NewObjectCreated(t *testing.T) {
	var destroyed []int
	var mu sync.Mutex

	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:      1,
		WaitTimeout: 500 * time.Millisecond,
		MaxIdleTime: 1 * time.Hour,
		Factory:     makeFactory(),
		Destroy:     makeDestroy(&destroyed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	obj1, _ := p.Acquire()

	var obj2 *testObj
	var getErr error
	done := make(chan struct{})
	go func() {
		obj2, getErr = p.Acquire()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	p.mu.Lock()
	if p.idleList.Len() == 0 {
		p.mu.Unlock()
		_ = p.Release(obj1)
	} else {
		ic := p.idleList.Front().Value.(*idleEntry[*testObj])
		ic.lastUsed = time.Now().Add(-2 * time.Hour)

		var expired []*idleEntry[*testObj]
		now := time.Now()
		for e := p.idleList.Front(); e != nil; {
			entry := e.Value.(*idleEntry[*testObj])
			next := e.Next()
			if now.Sub(entry.lastUsed) > p.cfg.MaxIdleTime {
				p.idleList.Remove(e)
				expired = append(expired, entry)
				atomic.AddInt32(&p.count, -1)
			}
			e = next
		}
		if len(expired) > 0 {
			p.cond.Broadcast()
		}
		p.mu.Unlock()
		for _, entry := range expired {
			p.cfg.Destroy(entry.obj)
		}
	}

	<-done
	if getErr != nil {
		t.Fatalf("expected successful Acquire, got: %v", getErr)
	}
	if obj2 == nil {
		t.Fatal("expected non-nil object")
	}
	_ = p.Release(obj2)
}

func TestRelease_NilObject(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  2,
		Factory: makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	err = p.Release(nil)
	if err == nil {
		t.Error("expected error for nil object")
	}
}

func TestRelease_NotBorrowed(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  2,
		Factory: makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	external := &testObj{id: 999, closed: false}
	err = p.Release(external)
	if err != ErrNotBorrowed {
		t.Errorf("expected ErrNotBorrowed, got %v", err)
	}
}

func TestRelease_DoubleReturn(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  2,
		Factory: makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	obj, _ := p.Acquire()
	_ = p.Release(obj)

	err = p.Release(obj)
	if err != ErrNotBorrowed {
		t.Errorf("expected ErrNotBorrowed for double return, got %v", err)
	}
}

func TestRelease_OnClosedPool(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  2,
		Factory: makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj, _ := p.Acquire()
	p.Close()

	err = p.Release(obj)
	if err != ErrPoolClosed {
		t.Errorf("expected ErrPoolClosed, got %v", err)
	}
}

func TestAcquire_OnClosedPool(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  2,
		Factory: makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p.Close()

	_, err = p.Acquire()
	if err != ErrPoolClosed {
		t.Errorf("expected ErrPoolClosed, got %v", err)
	}
}

func TestClose_Idempotent(t *testing.T) {
	var destroyed []int
	var mu sync.Mutex

	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  3,
		Factory: makeFactory(),
		Destroy: makeDestroy(&destroyed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj, _ := p.Acquire()

	p.Close()
	p.Close()

	mu.Lock()
	count := len(destroyed)
	mu.Unlock()
	if count != 1 {
		t.Errorf("expected 1 object destroyed, got %d", count)
	}
	_ = obj
}

func TestClose_DestroysAllObjects(t *testing.T) {
	var destroyed []int
	var mu sync.Mutex

	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  5,
		Factory: makeFactory(),
		Destroy: makeDestroy(&destroyed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj1, _ := p.Acquire()
	obj2, _ := p.Acquire()
	obj3, _ := p.Acquire()
	_ = p.Release(obj3)

	p.Close()

	mu.Lock()
	count := len(destroyed)
	mu.Unlock()
	if count != 3 {
		t.Errorf("expected 3 objects destroyed, got %d", count)
	}
	if !obj1.closed || !obj2.closed || !obj3.closed {
		t.Error("all objects should be closed after pool close")
	}
}

func TestAcquire_FactoryError(t *testing.T) {
	var callCount int64
	factory := func() (*testObj, error) {
		n := atomic.AddInt64(&callCount, 1)
		if n == 1 {
			return nil, errors.New("factory failed")
		}
		return &testObj{id: int(n), closed: false}, nil
	}

	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  3,
		Factory: factory,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	_, err = p.Acquire()
	if err == nil {
		t.Fatal("expected factory error on first Acquire")
	}
	if err.Error() != "factory failed" {
		t.Errorf("expected factory error message, got: %v", err)
	}

	obj, err := p.Acquire()
	if err != nil {
		t.Fatalf("expected success on second Acquire, got: %v", err)
	}
	if obj == nil {
		t.Fatal("expected non-nil object")
	}
	_ = p.Release(obj)
}

func TestAcquire_FactoryErrorDoesNotLeakCount(t *testing.T) {
	var callCount int64
	factory := func() (*testObj, error) {
		n := atomic.AddInt64(&callCount, 1)
		if n <= 2 {
			return nil, errors.New("factory failed")
		}
		return &testObj{id: int(n), closed: false}, nil
	}

	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  3,
		Factory: factory,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	_, _ = p.Acquire()
	_, _ = p.Acquire()

	obj, err := p.Acquire()
	if err != nil {
		t.Fatalf("expected success after two failures, got: %v", err)
	}
	if p.Len() != 1 {
		t.Errorf("expected Len=1, got %d", p.Len())
	}
	_ = p.Release(obj)
}

func TestIdleReclaim_RemovesExpiredObjects(t *testing.T) {
	var destroyed []int
	var mu sync.Mutex

	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:      5,
		MaxIdleTime: 50 * time.Millisecond,
		Factory:     makeFactory(),
		Destroy:     makeDestroy(&destroyed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	obj1, _ := p.Acquire()
	obj2, _ := p.Acquire()
	obj3, _ := p.Acquire()
	_ = p.Release(obj1)
	_ = p.Release(obj2)
	_ = p.Release(obj3)

	if p.IdleCount() != 3 {
		t.Fatalf("expected IdleCount=3, got %d", p.IdleCount())
	}

	time.Sleep(120 * time.Millisecond)

	mu.Lock()
	count := len(destroyed)
	mu.Unlock()
	if count != 3 {
		t.Errorf("expected 3 idle objects reclaimed, got %d", count)
	}
	if p.Len() != 0 {
		t.Errorf("expected Len=0 after reclaim, got %d", p.Len())
	}
	if p.IdleCount() != 0 {
		t.Errorf("expected IdleCount=0 after reclaim, got %d", p.IdleCount())
	}
}

func TestIdleReclaim_ActiveNotReclaimed(t *testing.T) {
	var destroyed []int
	var mu sync.Mutex

	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:      3,
		MaxIdleTime: 50 * time.Millisecond,
		Factory:     makeFactory(),
		Destroy:     makeDestroy(&destroyed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	obj1, _ := p.Acquire()
	obj2, _ := p.Acquire()
	_ = p.Release(obj2)

	time.Sleep(120 * time.Millisecond)

	mu.Lock()
	count := len(destroyed)
	mu.Unlock()
	if count != 1 {
		t.Errorf("expected 1 idle object reclaimed, got %d", count)
	}
	if p.ActiveCount() != 1 {
		t.Errorf("expected ActiveCount=1, got %d", p.ActiveCount())
	}

	_ = p.Release(obj1)
}

func TestIdleReclaim_ReleasesCapacity(t *testing.T) {
	var destroyed []int
	var mu sync.Mutex

	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:      2,
		MaxIdleTime: 50 * time.Millisecond,
		WaitTimeout: 500 * time.Millisecond,
		Factory:     makeFactory(),
		Destroy:     makeDestroy(&destroyed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	obj1, _ := p.Acquire()
	obj2, _ := p.Acquire()
	_ = p.Release(obj1)
	_ = p.Release(obj2)

	if p.Len() != 2 {
		t.Fatalf("expected Len=2, got %d", p.Len())
	}

	time.Sleep(120 * time.Millisecond)

	mu.Lock()
	count := len(destroyed)
	mu.Unlock()
	if count != 2 {
		t.Errorf("expected 2 objects reclaimed, got %d", count)
	}
	if p.Len() != 0 {
		t.Errorf("expected Len=0 after reclaim, got %d", p.Len())
	}

	obj, err := p.Acquire()
	if err != nil {
		t.Fatalf("expected to create new object after reclaim, got: %v", err)
	}
	if obj == nil {
		t.Fatal("expected non-nil object")
	}
	_ = p.Release(obj)
}

func TestIdleReclaim_PartialExpiry(t *testing.T) {
	var destroyed []int
	var mu sync.Mutex

	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:      5,
		MaxIdleTime: 100 * time.Millisecond,
		Factory:     makeFactory(),
		Destroy:     makeDestroy(&destroyed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	obj1, _ := p.Acquire()
	obj2, _ := p.Acquire()
	_ = p.Release(obj1)

	time.Sleep(60 * time.Millisecond)

	_ = p.Release(obj2)

	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	count := len(destroyed)
	mu.Unlock()
	if count != 1 {
		t.Errorf("expected 1 object reclaimed (obj1), got %d", count)
	}

	if p.IdleCount() != 1 {
		t.Errorf("expected IdleCount=1 (obj2 still fresh), got %d", p.IdleCount())
	}
}

func TestConcurrent_AcquireRelease(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:      5,
		WaitTimeout: 2 * time.Second,
		Factory:     makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	var wg sync.WaitGroup
	numGoroutines := 20
	iterations := 100

	var success int64
	var fail int64

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				obj, err := p.Acquire()
				if err != nil {
					atomic.AddInt64(&fail, 1)
					continue
				}
				atomic.AddInt64(&success, 1)
				time.Sleep(time.Microsecond * 10)
				if err := p.Release(obj); err != nil {
					t.Errorf("Release error: %v", err)
				}
			}
		}()
	}

	wg.Wait()

	expected := int64(numGoroutines * iterations)
	if success != expected {
		t.Errorf("expected %d successful Acquires, got %d (failures: %d)", expected, success, fail)
	}
	if p.Len() > 5 {
		t.Errorf("expected Len <= MaxCap(5), got %d", p.Len())
	}
}

func TestLen_Accurate(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  4,
		Factory: makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	if p.Len() != 0 {
		t.Errorf("expected 0, got %d", p.Len())
	}

	obj1, _ := p.Acquire()
	obj2, _ := p.Acquire()
	obj3, _ := p.Acquire()

	if p.Len() != 3 {
		t.Errorf("expected 3, got %d", p.Len())
	}

	_ = p.Release(obj1)
	_ = p.Release(obj2)
	_ = p.Release(obj3)

	if p.Len() != 3 {
		t.Errorf("expected 3 after all Released, got %d", p.Len())
	}

	obj4, _ := p.Acquire()
	_ = obj4
}

func TestPoolClosed_DuringWait(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:      1,
		WaitTimeout: 5 * time.Second,
		Factory:     makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _ = p.Acquire()

	var getErr error
	done := make(chan struct{})
	go func() {
		_, getErr = p.Acquire()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	p.Close()

	select {
	case <-done:
		if getErr != ErrPoolClosed {
			t.Errorf("expected ErrPoolClosed, got %v", getErr)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Acquire should have returned after pool Close")
	}
}

func TestAcquire_PoolClosedAfterFactoryCall(t *testing.T) {
	var destroyed []int
	var mu sync.Mutex

	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  1,
		Factory: makeFactory(),
		Destroy: makeDestroy(&destroyed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _ = p.Acquire()
	go func() {
		time.Sleep(20 * time.Millisecond)
		p.Close()
	}()

	_, err = p.Acquire()
	if err != ErrPoolClosed && err != ErrPoolExhausted {
		t.Errorf("expected ErrPoolClosed or ErrPoolExhausted, got %v", err)
	}
}

func TestNewPool_CleanupIntervalCustom(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:          5,
		MaxIdleTime:     100 * time.Millisecond,
		CleanupInterval: 30 * time.Millisecond,
		Factory:         makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	if p.cfg.CleanupInterval != 30*time.Millisecond {
		t.Errorf("expected CleanupInterval=30ms, got %v", p.cfg.CleanupInterval)
	}
}

func TestGenericPool_IntType(t *testing.T) {
	var counter int64
	factory := func() (int, error) {
		return int(atomic.AddInt64(&counter, 1)), nil
	}

	p, err := NewPool[int](Config[int]{
		MaxCap:  3,
		Factory: factory,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	v1, err := p.Acquire()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v1 != 1 {
		t.Errorf("expected 1, got %d", v1)
	}

	err = p.Release(v1)
	if err != nil {
		t.Fatalf("unexpected error on Release: %v", err)
	}

	v2, err := p.Acquire()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v2 != 1 {
		t.Errorf("expected reused value 1, got %d", v2)
	}
	_ = p.Release(v2)
}

func TestGenericPool_StringType(t *testing.T) {
	var counter int64
	factory := func() (string, error) {
		n := atomic.AddInt64(&counter, 1)
		return "obj-" + string(rune('A'+n-1)), nil
	}

	p, err := NewPool[string](Config[string]{
		MaxCap:  2,
		Factory: factory,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	s1, _ := p.Acquire()
	s2, _ := p.Acquire()

	if s1 == s2 {
		t.Error("expected different objects")
	}

	_ = p.Release(s1)
	_ = p.Release(s2)

	if p.IdleCount() != 2 {
		t.Errorf("expected IdleCount=2, got %d", p.IdleCount())
	}
}

func TestIdleReclaim_BroadcastWakesBlockedAcquire(t *testing.T) {
	var destroyed []int
	var mu sync.Mutex

	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:      1,
		MaxIdleTime: 1 * time.Hour,
		WaitTimeout: 5 * time.Second,
		Factory:     makeFactory(),
		Destroy:     makeDestroy(&destroyed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	obj1, _ := p.Acquire()

	var obj2 *testObj
	var getErr error
	done := make(chan struct{})
	go func() {
		obj2, getErr = p.Acquire()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("Acquire should be blocked")
	default:
	}

	_ = p.Release(obj1)

	p.mu.Lock()
	if p.idleList.Len() == 1 {
		ic := p.idleList.Front().Value.(*idleEntry[*testObj])
		ic.lastUsed = time.Now().Add(-2 * time.Hour)
	}

	var expired []*idleEntry[*testObj]
	now := time.Now()
	for e := p.idleList.Front(); e != nil; {
		ic := e.Value.(*idleEntry[*testObj])
		next := e.Next()
		if now.Sub(ic.lastUsed) > p.cfg.MaxIdleTime {
			p.idleList.Remove(e)
			expired = append(expired, ic)
			atomic.AddInt32(&p.count, -1)
		}
		e = next
	}
	if len(expired) > 0 {
		p.cond.Broadcast()
	}
	p.mu.Unlock()

	for _, ic := range expired {
		p.cfg.Destroy(ic.obj)
	}

	select {
	case <-done:
		if getErr != nil {
			t.Fatalf("expected successful Acquire, got: %v", getErr)
		}
		if obj2 == nil {
			t.Fatal("expected non-nil object")
		}
		mu.Lock()
		closedCount := len(destroyed)
		mu.Unlock()
		if closedCount != 1 {
			t.Errorf("expected 1 object destroyed by reclaim, got %d", closedCount)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Acquire should have been woken up by reclaim Broadcast")
	}

	_ = p.Release(obj2)
}

func TestIdleReclaim_MultipleBlockedAcquires(t *testing.T) {
	var destroyed []int
	var mu sync.Mutex

	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:      2,
		MaxIdleTime: 1 * time.Hour,
		WaitTimeout: 5 * time.Second,
		Factory:     makeFactory(),
		Destroy:     makeDestroy(&destroyed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	obj1, _ := p.Acquire()
	obj2, _ := p.Acquire()

	var wg sync.WaitGroup
	var success int64

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			obj, err := p.Acquire()
			if err == nil && obj != nil {
				atomic.AddInt64(&success, 1)
				time.Sleep(10 * time.Millisecond)
				_ = p.Release(obj)
			}
		}()
	}

	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt64(&success) != 0 {
		t.Fatalf("no Acquire should succeed yet, got %d", atomic.LoadInt64(&success))
	}

	_ = p.Release(obj1)
	_ = p.Release(obj2)

	p.mu.Lock()
	for e := p.idleList.Front(); e != nil; e = e.Next() {
		ic := e.Value.(*idleEntry[*testObj])
		ic.lastUsed = time.Now().Add(-2 * time.Hour)
	}

	var expired []*idleEntry[*testObj]
	now := time.Now()
	for e := p.idleList.Front(); e != nil; {
		ic := e.Value.(*idleEntry[*testObj])
		next := e.Next()
		if now.Sub(ic.lastUsed) > p.cfg.MaxIdleTime {
			p.idleList.Remove(e)
			expired = append(expired, ic)
			atomic.AddInt32(&p.count, -1)
		}
		e = next
	}
	if len(expired) > 0 {
		p.cond.Broadcast()
	}
	p.mu.Unlock()

	for _, ic := range expired {
		p.cfg.Destroy(ic.obj)
	}

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
		if atomic.LoadInt64(&success) != 2 {
			t.Errorf("expected 2 successful Acquires, got %d", atomic.LoadInt64(&success))
		}
		mu.Lock()
		closedCount := len(destroyed)
		mu.Unlock()
		if closedCount != 2 {
			t.Errorf("expected 2 objects closed by reclaim, got %d", closedCount)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("all waiting Acquires should have completed after reclaim Broadcast")
	}
}

func TestWaitTimeout_AtomicCheck(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:      1,
		WaitTimeout: 100 * time.Millisecond,
		Factory:     makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	_, _ = p.Acquire()

	start := time.Now()
	_, err = p.Acquire()
	elapsed := time.Since(start)

	if err != ErrPoolExhausted {
		t.Errorf("expected ErrPoolExhausted, got %v", err)
	}
	if elapsed < 90*time.Millisecond || elapsed > 200*time.Millisecond {
		t.Errorf("expected wait ~100ms, waited %v", elapsed)
	}
}

func TestAcquire_Release_ResetByCaller(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  2,
		Factory: makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	obj, _ := p.Acquire()
	obj.closed = true
	_ = p.Release(obj)

	obj2, _ := p.Acquire()
	if obj2.id != obj.id {
		t.Errorf("expected same object reused, got different id")
	}
	_ = p.Release(obj2)
}

func TestPool_DestroyCalledOnReclaim(t *testing.T) {
	var destroyed []int
	var mu sync.Mutex

	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:      3,
		MaxIdleTime: 50 * time.Millisecond,
		Factory:     makeFactory(),
		Destroy:     makeDestroy(&destroyed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	obj1, _ := p.Acquire()
	_ = p.Release(obj1)

	time.Sleep(120 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(destroyed) != 1 {
		t.Errorf("expected 1 object destroyed on idle reclaim, got %d", len(destroyed))
	}
}

func TestPool_DestroyCalledOnClose(t *testing.T) {
	var destroyed []int
	var mu sync.Mutex

	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  3,
		Factory: makeFactory(),
		Destroy: makeDestroy(&destroyed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj1, _ := p.Acquire()
	obj2, _ := p.Acquire()
	_ = p.Release(obj2)

	p.Close()

	mu.Lock()
	defer mu.Unlock()
	if len(destroyed) != 2 {
		t.Errorf("expected 2 objects destroyed on close, got %d", len(destroyed))
	}
	if !obj1.closed || !obj2.closed {
		t.Error("all objects should be marked closed after pool close")
	}
}

func TestPool_NoIdleReclaim_WhenMaxIdleTimeZero(t *testing.T) {
	var destroyed []int
	var mu sync.Mutex

	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  3,
		Factory: makeFactory(),
		Destroy: makeDestroy(&destroyed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	obj1, _ := p.Acquire()
	_ = p.Release(obj1)

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := len(destroyed)
	mu.Unlock()
	if count != 0 {
		t.Errorf("expected 0 objects destroyed (no idle reclaim), got %d", count)
	}
}

func TestAcquire_CreatesUpToMaxCap(t *testing.T) {
	p, err := NewPool[*testObj](Config[*testObj]{
		MaxCap:  5,
		Factory: makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	objs := make([]*testObj, 5)
	for i := 0; i < 5; i++ {
		objs[i], err = p.Acquire()
		if err != nil {
			t.Fatalf("Acquire %d failed: %v", i, err)
		}
	}

	if p.Len() != 5 {
		t.Errorf("expected Len=5, got %d", p.Len())
	}

	for _, obj := range objs {
		_ = p.Release(obj)
	}
}
