package connpool

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockConn struct {
	id    int
	alive bool
}

func makeFactory() Factory {
	var counter int64
	return func() (Conn, error) {
		id := atomic.AddInt64(&counter, 1)
		return &mockConn{id: int(id), alive: true}, nil
	}
}

func makePing() PingFunc {
	return func(c Conn) error {
		mc := c.(*mockConn)
		if !mc.alive {
			return errors.New("connection dead")
		}
		return nil
	}
}

func makeClose(closed *[]int, mu *sync.Mutex) CloseFunc {
	return func(c Conn) error {
		mc := c.(*mockConn)
		if mu != nil {
			mu.Lock()
		}
		*closed = append(*closed, mc.id)
		if mu != nil {
			mu.Unlock()
		}
		mc.alive = false
		return nil
	}
}

func TestNewPool_InvalidConfig(t *testing.T) {
	_, err := NewPool(Config{})
	if err == nil {
		t.Fatal("expected error for missing factory")
	}

	_, err = NewPool(Config{
		Factory: makeFactory(),
		MaxCap:  0,
	})
	if err == nil {
		t.Fatal("expected error for MaxCap <= 0")
	}
}

func TestNewPool_InitialCap(t *testing.T) {
	var closed []int
	var mu sync.Mutex

	p, err := NewPool(Config{
		InitialCap: 3,
		MaxCap:     5,
		Factory:    makeFactory(),
		Close:      makeClose(&closed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	if p.Len() != 3 {
		t.Errorf("expected Len=3, got %d", p.Len())
	}
	if p.IdleCount() != 3 {
		t.Errorf("expected IdleCount=3, got %d", p.IdleCount())
	}
	if p.ActiveCount() != 0 {
		t.Errorf("expected ActiveCount=0, got %d", p.ActiveCount())
	}
}

func TestNewPool_InitialCapExceedsMaxCap(t *testing.T) {
	p, err := NewPool(Config{
		InitialCap: 10,
		MaxCap:     5,
		Factory:    makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	if p.Len() != 5 {
		t.Errorf("expected Len=5 (clamped to MaxCap), got %d", p.Len())
	}
}

func TestGetAndPut_Single(t *testing.T) {
	p, err := NewPool(Config{
		InitialCap: 2,
		MaxCap:     2,
		Factory:    makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	c, err := p.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil connection")
	}
	if p.ActiveCount() != 1 {
		t.Errorf("expected ActiveCount=1, got %d", p.ActiveCount())
	}
	if p.IdleCount() != 1 {
		t.Errorf("expected IdleCount=1, got %d", p.IdleCount())
	}

	err = p.Put(c)
	if err != nil {
		t.Fatalf("unexpected error on Put: %v", err)
	}
	if p.ActiveCount() != 0 {
		t.Errorf("expected ActiveCount=0 after Put, got %d", p.ActiveCount())
	}
	if p.IdleCount() != 2 {
		t.Errorf("expected IdleCount=2 after Put, got %d", p.IdleCount())
	}
}

func TestGetAndPut_Multiple(t *testing.T) {
	p, err := NewPool(Config{
		InitialCap: 0,
		MaxCap:     3,
		Factory:    makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	conns := make([]Conn, 3)
	for i := 0; i < 3; i++ {
		conns[i], err = p.Get()
		if err != nil {
			t.Fatalf("Get %d failed: %v", i, err)
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

	_, err = p.Get()
	if err != ErrPoolExhausted {
		t.Errorf("expected ErrPoolExhausted, got %v", err)
	}

	for i, c := range conns {
		if err := p.Put(c); err != nil {
			t.Fatalf("Put %d failed: %v", i, err)
		}
	}

	if p.IdleCount() != 3 {
		t.Errorf("expected IdleCount=3, got %d", p.IdleCount())
	}
}

func TestGet_WaitTimeout(t *testing.T) {
	p, err := NewPool(Config{
		InitialCap:  1,
		MaxCap:      1,
		WaitTimeout: 100 * time.Millisecond,
		Factory:     makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	c, err := p.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	start := time.Now()
	_, err = p.Get()
	elapsed := time.Since(start)
	if err != ErrPoolExhausted {
		t.Errorf("expected ErrPoolExhausted, got %v", err)
	}
	if elapsed < 80*time.Millisecond {
		t.Errorf("expected to wait ~100ms, waited %v", elapsed)
	}

	if err := p.Put(c); err != nil {
		t.Fatalf("Put failed: %v", err)
	}
}

func TestGet_WaitTimeout_Success(t *testing.T) {
	p, err := NewPool(Config{
		InitialCap:  1,
		MaxCap:      1,
		WaitTimeout: 500 * time.Millisecond,
		Factory:     makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	c1, err := p.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var c2 Conn
	var getErr error
	done := make(chan struct{})
	go func() {
		c2, getErr = p.Get()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	if err := p.Put(c1); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	<-done
	if getErr != nil {
		t.Fatalf("expected successful Get after Put, got: %v", getErr)
	}
	if c2 == nil {
		t.Fatal("expected non-nil connection")
	}
	if err := p.Put(c2); err != nil {
		t.Fatalf("Put failed: %v", err)
	}
}

func TestPut_InvalidConnection(t *testing.T) {
	p, err := NewPool(Config{
		InitialCap: 1,
		MaxCap:     1,
		Factory:    makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	err = p.Put(nil)
	if err == nil {
		t.Error("expected error for nil connection")
	}

	external := &mockConn{id: 999, alive: true}
	err = p.Put(external)
	if err == nil {
		t.Error("expected error for external connection")
	}

	c, _ := p.Get()
	if err := p.Put(c); err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	err = p.Put(c)
	if err == nil {
		t.Error("expected error for double-returned connection")
	}
}

func TestPool_Closed(t *testing.T) {
	var closed []int
	var mu sync.Mutex

	p, err := NewPool(Config{
		InitialCap: 3,
		MaxCap:     5,
		Factory:    makeFactory(),
		Close:      makeClose(&closed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c, err := p.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p.Close()
	p.Close()

	if len(closed) != 3 {
		t.Errorf("expected 3 connections closed, got %d", len(closed))
	}

	_, err = p.Get()
	if err != ErrPoolClosed {
		t.Errorf("expected ErrPoolClosed, got %v", err)
	}

	err = p.Put(c)
	if err != ErrPoolClosed {
		t.Errorf("expected ErrPoolClosed on Put, got %v", err)
	}
}

func TestHeartbeat_RemovesBadConns(t *testing.T) {
	var closed []int
	var mu sync.Mutex
	factory := makeFactory()
	pinger := makePing()
	closer := makeClose(&closed, &mu)

	p, err := NewPool(Config{
		InitialCap:        3,
		MaxCap:            3,
		HeartbeatInterval: 30 * time.Millisecond,
		Factory:           factory,
		Ping:              pinger,
		Close:             closer,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	c1, _ := p.Get()

	c2, _ := p.Get()
	mc2 := c2.(*mockConn)
	mc2.alive = false

	_ = p.Put(c1)
	_ = p.Put(c2)

	c3, _ := p.Get()
	mc3 := c3.(*mockConn)
	mc3.alive = false
	_ = p.Put(c3)

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	closedCount := len(closed)
	mu.Unlock()
	if closedCount < 2 {
		t.Errorf("expected at least 2 bad connections closed, got %d", closedCount)
	}
}

func TestIdleTimeout_ReclaimsConns(t *testing.T) {
	var closed []int
	var mu sync.Mutex

	p, err := NewPool(Config{
		InitialCap:  3,
		MaxCap:      3,
		IdleTimeout: 50 * time.Millisecond,
		Factory:     makeFactory(),
		Close:       makeClose(&closed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	c, _ := p.Get()
	time.Sleep(20 * time.Millisecond)
	_ = p.Put(c)

	time.Sleep(120 * time.Millisecond)

	mu.Lock()
	closedCount := len(closed)
	mu.Unlock()
	if closedCount != 3 {
		t.Errorf("expected 3 idle connections reclaimed, got %d", closedCount)
	}
	if p.Len() != 0 {
		t.Errorf("expected Len=0 after timeout, got %d", p.Len())
	}
}

func TestMaxLifetime_BorrowCheck(t *testing.T) {
	var closed []int
	var mu sync.Mutex

	p, err := NewPool(Config{
		InitialCap:  2,
		MaxCap:      2,
		MaxLifetime: 30 * time.Millisecond,
		Factory:     makeFactory(),
		Close:       makeClose(&closed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	time.Sleep(60 * time.Millisecond)

	c, err := p.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected new connection after expired ones removed")
	}

	mu.Lock()
	closedCount := len(closed)
	mu.Unlock()
	if closedCount < 2 {
		t.Errorf("expected at least 2 expired connections closed, got %d", closedCount)
	}
	_ = p.Put(c)
}

func TestMaxLifetime_PutCheck(t *testing.T) {
	var closed []int
	var mu sync.Mutex

	p, err := NewPool(Config{
		InitialCap:  1,
		MaxCap:      1,
		MaxLifetime: 80 * time.Millisecond,
		Factory:     makeFactory(),
		Close:       makeClose(&closed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	c, _ := p.Get()

	time.Sleep(120 * time.Millisecond)

	_ = p.Put(c)

	mu.Lock()
	closedCount := len(closed)
	mu.Unlock()
	if closedCount != 1 {
		t.Errorf("expected 1 expired connection closed on Put, got %d", closedCount)
	}
	if p.IdleCount() != 0 {
		t.Errorf("expected IdleCount=0, got %d", p.IdleCount())
	}
}

func TestMaxIdle_LRUReclaim(t *testing.T) {
	var closed []int
	var mu sync.Mutex

	p, err := NewPool(Config{
		InitialCap: 0,
		MaxCap:     5,
		MaxIdle:    2,
		Factory:    makeFactory(),
		Close:      makeClose(&closed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	conns := make([]Conn, 5)
	for i := 0; i < 5; i++ {
		conns[i], _ = p.Get()
	}

	for i := 0; i < 5; i++ {
		_ = p.Put(conns[i])
	}

	if p.IdleCount() != 2 {
		t.Errorf("expected IdleCount=2 (MaxIdle), got %d", p.IdleCount())
	}

	mu.Lock()
	closedCount := len(closed)
	mu.Unlock()
	if closedCount != 3 {
		t.Errorf("expected 3 excess idle connections reclaimed, got %d", closedCount)
	}

	mu.Lock()
	closedIds := append([]int{}, closed...)
	mu.Unlock()

	if len(closedIds) >= 3 {
		if closedIds[0] != 1 || closedIds[1] != 2 || closedIds[2] != 3 {
			t.Errorf("expected LRU order [1,2,3], got %v", closedIds)
		}
	}
}

func TestConcurrent_GetPut(t *testing.T) {
	p, err := NewPool(Config{
		InitialCap:  5,
		MaxCap:      10,
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
				c, err := p.Get()
				if err != nil {
					atomic.AddInt64(&fail, 1)
					continue
				}
				atomic.AddInt64(&success, 1)
				time.Sleep(time.Microsecond * 10)
				if err := p.Put(c); err != nil {
					t.Errorf("Put error: %v", err)
				}
			}
		}()
	}

	wg.Wait()

	expected := int64(numGoroutines * iterations)
	if success != expected {
		t.Errorf("expected %d successful Gets, got %d (failures: %d)", expected, success, fail)
	}
	if p.Len() > 10 {
		t.Errorf("expected Len <= MaxCap(10), got %d", p.Len())
	}
}

func TestGet_AllBadIdleCreatesNew(t *testing.T) {
	var created int64
	var closed []int
	var mu sync.Mutex

	factory := func() (Conn, error) {
		id := atomic.AddInt64(&created, 1)
		return &mockConn{id: int(id), alive: true}, nil
	}

	pinger := func(c Conn) error {
		mc := c.(*mockConn)
		if !mc.alive {
			return errors.New("dead")
		}
		return nil
	}

	p, err := NewPool(Config{
		InitialCap: 3,
		MaxCap:     5,
		Factory:    factory,
		Ping:       pinger,
		Close:      makeClose(&closed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	mu.Lock()
	initCreated := created
	mu.Unlock()

	if initCreated != 3 {
		t.Fatalf("expected 3 initial connections, got %d", initCreated)
	}

	conns := make([]*mockConn, 3)
	for i := 0; i < 3; i++ {
		c, _ := p.Get()
		conns[i] = c.(*mockConn)
		conns[i].alive = false
		_ = p.Put(c)
	}

	for i := 0; i < 2; i++ {
		c, err := p.Get()
		if err != nil {
			t.Fatalf("Get %d failed: %v", i, err)
		}
		mc := c.(*mockConn)
		if !mc.alive {
			t.Errorf("expected alive connection, got dead one (id=%d)", mc.id)
		}
		_ = p.Put(c)
	}
}

func TestNewPool_FactoryError(t *testing.T) {
	failOn := int64(2)
	var callCount int64
	factory := func() (Conn, error) {
		n := atomic.AddInt64(&callCount, 1)
		if n >= failOn {
			return nil, errors.New("factory failed")
		}
		return &mockConn{id: int(n), alive: true}, nil
	}

	var closed []int
	var mu sync.Mutex

	p, err := NewPool(Config{
		InitialCap: 5,
		MaxCap:     10,
		Factory:    factory,
		Close:      makeClose(&closed, &mu),
	})
	if err == nil {
		p.Close()
		t.Fatal("expected error from NewPool when factory fails")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(closed) != 1 {
		t.Errorf("expected 1 connection closed on init failure, got %d", len(closed))
	}
}

func TestGet_FactoryErrorCreatesSlot(t *testing.T) {
	var callCount int64
	factory := func() (Conn, error) {
		n := atomic.AddInt64(&callCount, 1)
		if n == 1 {
			return nil, errors.New("factory failed")
		}
		return &mockConn{id: int(n), alive: true}, nil
	}

	p, err := NewPool(Config{
		InitialCap: 0,
		MaxCap:     2,
		Factory:    factory,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	_, err = p.Get()
	if err == nil {
		t.Fatal("expected factory error on first Get")
	}

	c1, err := p.Get()
	if err != nil {
		t.Fatalf("expected success on second Get, got: %v", err)
	}
	if c1 == nil {
		t.Fatal("expected non-nil connection")
	}

	c2, err := p.Get()
	if err != nil {
		t.Fatalf("expected success on third Get, got: %v", err)
	}
	if c2 == nil {
		t.Fatal("expected non-nil connection")
	}

	_, err = p.Get()
	if err != ErrPoolExhausted {
		t.Errorf("expected ErrPoolExhausted, got %v", err)
	}

	_ = p.Put(c1)
	_ = p.Put(c2)
}

func TestLen_Accurate(t *testing.T) {
	p, err := NewPool(Config{
		InitialCap: 2,
		MaxCap:     4,
		Factory:    makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	if p.Len() != 2 {
		t.Errorf("expected 2, got %d", p.Len())
	}

	c1, _ := p.Get()
	c2, _ := p.Get()
	c3, _ := p.Get()

	if p.Len() != 3 {
		t.Errorf("expected 3, got %d", p.Len())
	}

	_ = p.Put(c1)
	_ = p.Put(c2)
	_ = p.Put(c3)

	if p.Len() != 3 {
		t.Errorf("expected 3, got %d", p.Len())
	}
}

func TestGet_PoolClosedDuringWait(t *testing.T) {
	p, err := NewPool(Config{
		InitialCap:  1,
		MaxCap:      1,
		WaitTimeout: 5 * time.Second,
		Factory:     makeFactory(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c, _ := p.Get()

	var getErr error
	done := make(chan struct{})
	go func() {
		_, getErr = p.Get()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	_ = p.Put(c)
	p.Close()

	<-done
	if getErr != ErrPoolClosed {
		t.Errorf("expected ErrPoolClosed, got %v", getErr)
	}
}

func TestIdleTimeout_ActiveConnNotReclaimed(t *testing.T) {
	var closed []int
	var mu sync.Mutex

	p, err := NewPool(Config{
		InitialCap:  2,
		MaxCap:      2,
		IdleTimeout: 50 * time.Millisecond,
		Factory:     makeFactory(),
		Close:       makeClose(&closed, &mu),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	c, _ := p.Get()

	time.Sleep(120 * time.Millisecond)

	mu.Lock()
	closedCount := len(closed)
	mu.Unlock()
	if closedCount != 1 {
		t.Errorf("expected 1 idle conn reclaimed, active should stay, got %d", closedCount)
	}

	_ = p.Put(c)

	mu.Lock()
	afterPut := len(closed)
	mu.Unlock()
	if afterPut != 1 {
		t.Errorf("expected still 1 after put active back, got %d", afterPut)
	}
}
