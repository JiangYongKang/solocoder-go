package connpool

import (
	"container/list"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrPoolClosed    = errors.New("connpool: pool is closed")
	ErrPoolExhausted = errors.New("connpool: pool exhausted")
)

type Factory func() (Conn, error)
type PingFunc func(Conn) error
type CloseFunc func(Conn) error

type Config struct {
	InitialCap        int
	MaxCap            int
	MaxIdle           int
	WaitTimeout       time.Duration
	IdleTimeout       time.Duration
	MaxLifetime       time.Duration
	HeartbeatInterval time.Duration
	Factory           Factory
	Ping              PingFunc
	Close             CloseFunc
}

type Conn interface{}

type idleConn struct {
	conn       Conn
	createTime time.Time
	lastUsed   time.Time
}

type Pool struct {
	cfg      Config
	mu       sync.Mutex
	cond     *sync.Cond
	idleList *list.List
	active   map[Conn]*idleConn
	count    int32
	closed   bool
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

func NewPool(cfg Config) (*Pool, error) {
	if cfg.Factory == nil {
		return nil, errors.New("connpool: factory function is required")
	}
	if cfg.MaxCap <= 0 {
		return nil, errors.New("connpool: MaxCap must be greater than 0")
	}
	if cfg.InitialCap < 0 {
		cfg.InitialCap = 0
	}
	if cfg.InitialCap > cfg.MaxCap {
		cfg.InitialCap = cfg.MaxCap
	}
	if cfg.MaxIdle <= 0 {
		cfg.MaxIdle = cfg.MaxCap
	}
	if cfg.MaxIdle > cfg.MaxCap {
		cfg.MaxIdle = cfg.MaxCap
	}
	if cfg.Close == nil {
		cfg.Close = func(Conn) error { return nil }
	}

	p := &Pool{
		cfg:      cfg,
		idleList: list.New(),
		active:   make(map[Conn]*idleConn),
		stopCh:   make(chan struct{}),
	}
	p.cond = sync.NewCond(&p.mu)

	for i := 0; i < cfg.InitialCap; i++ {
		c, err := cfg.Factory()
		if err != nil {
			p.Close()
			return nil, err
		}
		now := time.Now()
		p.idleList.PushBack(&idleConn{
			conn:       c,
			createTime: now,
			lastUsed:   now,
		})
		atomic.AddInt32(&p.count, 1)
	}

	if cfg.HeartbeatInterval > 0 {
		p.wg.Add(1)
		go p.heartbeatLoop()
	}
	if cfg.IdleTimeout > 0 {
		p.wg.Add(1)
		go p.idleTimeoutLoop()
	}

	return p, nil
}

func (p *Pool) getIdle() (*idleConn, bool) {
	for {
		e := p.idleList.Front()
		if e == nil {
			return nil, false
		}
		ic := e.Value.(*idleConn)
		p.idleList.Remove(e)

		if p.cfg.MaxLifetime > 0 && time.Since(ic.createTime) > p.cfg.MaxLifetime {
			atomic.AddInt32(&p.count, -1)
			p.mu.Unlock()
			_ = p.cfg.Close(ic.conn)
			p.mu.Lock()
			continue
		}

		if p.cfg.Ping != nil {
			p.mu.Unlock()
			err := p.cfg.Ping(ic.conn)
			p.mu.Lock()
			if err != nil {
				atomic.AddInt32(&p.count, -1)
				p.mu.Unlock()
				_ = p.cfg.Close(ic.conn)
				p.mu.Lock()
				continue
			}
		}

		return ic, true
	}
}

func (p *Pool) Get() (Conn, error) {
	for {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return nil, ErrPoolClosed
		}

		if ic, ok := p.getIdle(); ok {
			ic.lastUsed = time.Now()
			p.active[ic.conn] = ic
			p.mu.Unlock()
			return ic.conn, nil
		}

		currentCount := atomic.LoadInt32(&p.count)
		if currentCount < int32(p.cfg.MaxCap) {
			if atomic.CompareAndSwapInt32(&p.count, currentCount, currentCount+1) {
				p.mu.Unlock()
				c, err := p.cfg.Factory()
				if err != nil {
					atomic.AddInt32(&p.count, -1)
					return nil, err
				}
				p.mu.Lock()
				if p.closed {
					p.mu.Unlock()
					_ = p.cfg.Close(c)
					return nil, ErrPoolClosed
				}
				now := time.Now()
				ic := &idleConn{
					conn:       c,
					createTime: now,
					lastUsed:   now,
				}
				p.active[c] = ic
				p.mu.Unlock()
				return c, nil
			}
			continue
		}

		if p.cfg.WaitTimeout == 0 {
			p.mu.Unlock()
			return nil, ErrPoolExhausted
		}

		deadline := time.Now().Add(p.cfg.WaitTimeout)

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

		for {
			if p.closed {
				p.mu.Unlock()
				return nil, ErrPoolClosed
			}
			if time.Now().After(deadline) {
				p.mu.Unlock()
				return nil, ErrPoolExhausted
			}
			if p.idleList.Len() > 0 || atomic.LoadInt32(&p.count) < int32(p.cfg.MaxCap) {
				break
			}
			p.cond.Wait()
		}
		p.mu.Unlock()
	}
}

func (p *Pool) Put(c Conn) error {
	if c == nil {
		return errors.New("connpool: nil connection")
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return ErrPoolClosed
	}

	ic, ok := p.active[c]
	if !ok {
		p.mu.Unlock()
		return errors.New("connpool: connection not borrowed from this pool")
	}

	delete(p.active, c)

	if p.cfg.MaxLifetime > 0 && time.Since(ic.createTime) > p.cfg.MaxLifetime {
		atomic.AddInt32(&p.count, -1)
		p.cond.Signal()
		p.mu.Unlock()
		return p.cfg.Close(c)
	}

	ic.lastUsed = time.Now()
	p.idleList.PushFront(ic)

	for p.idleList.Len() > p.cfg.MaxIdle {
		e := p.idleList.Back()
		if e == nil {
			break
		}
		victim := e.Value.(*idleConn)
		p.idleList.Remove(e)
		atomic.AddInt32(&p.count, -1)
		p.mu.Unlock()
		_ = p.cfg.Close(victim.conn)
		p.mu.Lock()
	}

	p.cond.Signal()
	p.mu.Unlock()
	return nil
}

func (p *Pool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	close(p.stopCh)
	p.cond.Broadcast()

	var toClose []Conn

	for e := p.idleList.Front(); e != nil; e = e.Next() {
		ic := e.Value.(*idleConn)
		toClose = append(toClose, ic.conn)
	}
	p.idleList.Init()

	for c := range p.active {
		toClose = append(toClose, c)
	}
	p.active = make(map[Conn]*idleConn)
	atomic.StoreInt32(&p.count, 0)

	p.mu.Unlock()

	for _, c := range toClose {
		_ = p.cfg.Close(c)
	}

	p.wg.Wait()
}

func (p *Pool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return int(atomic.LoadInt32(&p.count))
}

func (p *Pool) IdleCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.idleList.Len()
}

func (p *Pool) ActiveCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.active)
}

func (p *Pool) heartbeatLoop() {
	defer p.wg.Done()
	ticker := time.NewTicker(p.cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.checkHeartbeats()
		}
	}
}

func (p *Pool) checkHeartbeats() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}

	var toCheck []*idleConn
	for e := p.idleList.Front(); e != nil; e = e.Next() {
		toCheck = append(toCheck, e.Value.(*idleConn))
	}
	p.idleList.Init()
	p.mu.Unlock()

	var good []*idleConn
	var bad []*idleConn

	for _, ic := range toCheck {
		err := p.cfg.Ping(ic.conn)
		if err == nil {
			good = append(good, ic)
		} else {
			bad = append(bad, ic)
			atomic.AddInt32(&p.count, -1)
		}
	}

	p.mu.Lock()
	if !p.closed {
		for _, ic := range good {
			if _, borrowed := p.active[ic.conn]; !borrowed {
				p.idleList.PushBack(ic)
			}
		}
		p.cond.Broadcast()
	} else {
		for _, ic := range good {
			bad = append(bad, ic)
		}
	}
	p.mu.Unlock()

	for _, ic := range bad {
		_ = p.cfg.Close(ic.conn)
	}
}

func (p *Pool) idleTimeoutLoop() {
	defer p.wg.Done()
	checkInterval := p.cfg.IdleTimeout / 2
	if checkInterval <= 0 {
		checkInterval = p.cfg.IdleTimeout
	}
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.reclaimIdleTimeout()
		}
	}
}

func (p *Pool) reclaimIdleTimeout() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}

	var expired []*idleConn
	now := time.Now()
	for e := p.idleList.Front(); e != nil; {
		ic := e.Value.(*idleConn)
		next := e.Next()
		if now.Sub(ic.lastUsed) > p.cfg.IdleTimeout {
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
		_ = p.cfg.Close(ic.conn)
	}
}
