package objectpool

import (
	"container/list"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrPoolClosed    = errors.New("objectpool: pool is closed")
	ErrPoolExhausted = errors.New("objectpool: pool exhausted")
	ErrNotBorrowed   = errors.New("objectpool: object was not borrowed from this pool")
)

type Factory[T any] func() (T, error)

type DestroyFunc[T any] func(T)

type Config[T any] struct {
	MaxCap       int
	MaxIdleTime  time.Duration
	WaitTimeout  time.Duration
	CleanupInterval time.Duration
	Factory      Factory[T]
	Destroy      DestroyFunc[T]
}

type idleEntry[T any] struct {
	obj      T
	lastUsed time.Time
}

type Pool[T any] struct {
	cfg      Config[T]
	mu       sync.Mutex
	cond     *sync.Cond
	idleList *list.List
	active   map[any]*idleEntry[T]
	count    int32
	closed   bool
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

func NewPool[T any](cfg Config[T]) (*Pool[T], error) {
	if cfg.Factory == nil {
		return nil, errors.New("objectpool: factory function is required")
	}
	if cfg.MaxCap <= 0 {
		return nil, errors.New("objectpool: MaxCap must be greater than 0")
	}
	if cfg.MaxIdleTime > 0 && cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = cfg.MaxIdleTime / 2
		if cfg.CleanupInterval <= 0 {
			cfg.CleanupInterval = cfg.MaxIdleTime
		}
	}
	if cfg.Destroy == nil {
		cfg.Destroy = func(T) {}
	}

	p := &Pool[T]{
		cfg:      cfg,
		idleList: list.New(),
		active:   make(map[any]*idleEntry[T]),
		stopCh:   make(chan struct{}),
	}
	p.cond = sync.NewCond(&p.mu)

	if cfg.MaxIdleTime > 0 {
		p.wg.Add(1)
		go p.cleanupLoop()
	}

	return p, nil
}

func (p *Pool[T]) Acquire() (T, error) {
	var deadline time.Time

	for {
		p.mu.Lock()
		if p.closed {
			var zero T
			p.mu.Unlock()
			return zero, ErrPoolClosed
		}

		if ic, ok := p.getIdle(); ok {
			ic.lastUsed = time.Now()
			p.active[any(ic.obj)] = ic
			p.mu.Unlock()
			return ic.obj, nil
		}

		currentCount := atomic.LoadInt32(&p.count)
		if currentCount < int32(p.cfg.MaxCap) {
			if atomic.CompareAndSwapInt32(&p.count, currentCount, currentCount+1) {
				p.mu.Unlock()
				obj, err := p.cfg.Factory()
				if err != nil {
					atomic.AddInt32(&p.count, -1)
					var zero T
					return zero, err
				}
				p.mu.Lock()
				if p.closed {
					p.mu.Unlock()
					p.cfg.Destroy(obj)
					var zero T
					return zero, ErrPoolClosed
				}
				ic := &idleEntry[T]{
					obj:      obj,
					lastUsed: time.Now(),
				}
				p.active[any(obj)] = ic
				p.mu.Unlock()
				return obj, nil
			}
			p.mu.Unlock()
			continue
		}

		if p.cfg.WaitTimeout <= 0 {
			p.mu.Unlock()
			var zero T
			return zero, ErrPoolExhausted
		}

		if deadline.IsZero() {
			deadline = time.Now().Add(p.cfg.WaitTimeout)
			go func(d time.Time) {
				select {
				case <-time.After(time.Until(d)):
					p.mu.Lock()
					p.cond.Broadcast()
					p.mu.Unlock()
				case <-p.stopCh:
					return
				}
			}(deadline)
		}

		for {
			if p.closed {
				p.mu.Unlock()
				var zero T
				return zero, ErrPoolClosed
			}
			if time.Now().After(deadline) {
				p.mu.Unlock()
				var zero T
				return zero, ErrPoolExhausted
			}
			if p.idleList.Len() > 0 || atomic.LoadInt32(&p.count) < int32(p.cfg.MaxCap) {
				break
			}
			p.cond.Wait()
		}
		p.mu.Unlock()
	}
}

func (p *Pool[T]) getIdle() (*idleEntry[T], bool) {
	for {
		e := p.idleList.Front()
		if e == nil {
			return nil, false
		}
		ic := e.Value.(*idleEntry[T])
		p.idleList.Remove(e)
		return ic, true
	}
}

func (p *Pool[T]) Release(obj T) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return ErrPoolClosed
	}

	key := any(obj)

	ic, ok := p.active[key]
	if !ok {
		p.mu.Unlock()
		return ErrNotBorrowed
	}

	delete(p.active, key)

	ic.lastUsed = time.Now()
	p.idleList.PushFront(ic)

	p.cond.Signal()
	p.mu.Unlock()
	return nil
}

func (p *Pool[T]) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	close(p.stopCh)
	p.cond.Broadcast()

	var toDestroy []T

	for e := p.idleList.Front(); e != nil; e = e.Next() {
		ic := e.Value.(*idleEntry[T])
		toDestroy = append(toDestroy, ic.obj)
	}
	p.idleList.Init()

	for _, ic := range p.active {
		toDestroy = append(toDestroy, ic.obj)
	}
	p.active = make(map[any]*idleEntry[T])
	atomic.StoreInt32(&p.count, 0)

	p.mu.Unlock()

	for _, obj := range toDestroy {
		p.cfg.Destroy(obj)
	}

	p.wg.Wait()
}

func (p *Pool[T]) Len() int {
	return int(atomic.LoadInt32(&p.count))
}

func (p *Pool[T]) IdleCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.idleList.Len()
}

func (p *Pool[T]) ActiveCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.active)
}

func (p *Pool[T]) cleanupLoop() {
	defer p.wg.Done()
	ticker := time.NewTicker(p.cfg.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.reclaimIdle()
		}
	}
}

func (p *Pool[T]) reclaimIdle() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}

	var expired []*idleEntry[T]
	now := time.Now()
	for e := p.idleList.Front(); e != nil; {
		ic := e.Value.(*idleEntry[T])
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
}
