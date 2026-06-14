package dedup

import (
	"container/list"
	"errors"
	"sync"
	"time"
)

var (
	ErrEmptyMessageID   = errors.New("dedup: empty message id")
	ErrDeduplicatorStop = errors.New("dedup: deduplicator is stopped")
	ErrInvalidConfig    = errors.New("dedup: invalid config")
)

type Deduplicator struct {
	cfg      Config
	mu       sync.Mutex
	idMap    map[string]*list.Element
	idList   *list.List
	running  bool
	stopped  bool
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

type idEntry struct {
	id        string
	createdAt time.Time
}

type Config struct {
	WindowSize    time.Duration
	CleanInterval time.Duration
}

func DefaultConfig() Config {
	return Config{
		WindowSize:    5 * time.Minute,
		CleanInterval: 1 * time.Minute,
	}
}

func NewDeduplicator() *Deduplicator {
	d, err := NewDeduplicatorWithConfig(DefaultConfig())
	if err != nil {
		panic("dedup: DefaultConfig is invalid: " + err.Error())
	}
	return d
}

func NewDeduplicatorWithConfig(cfg Config) (*Deduplicator, error) {
	if cfg.WindowSize < 0 {
		return nil, ErrInvalidConfig
	}
	if cfg.CleanInterval < 0 {
		return nil, ErrInvalidConfig
	}

	if cfg.WindowSize == 0 {
		cfg.WindowSize = 5 * time.Minute
	}
	if cfg.CleanInterval == 0 {
		cfg.CleanInterval = cfg.WindowSize / 5
		if cfg.CleanInterval <= 0 {
			cfg.CleanInterval = time.Second
		}
	}

	d := &Deduplicator{
		cfg:    cfg,
		idMap:  make(map[string]*list.Element),
		idList: list.New(),
		stopCh: make(chan struct{}),
	}
	return d, nil
}

func (d *Deduplicator) Start() {
	d.mu.Lock()
	if d.stopped {
		d.mu.Unlock()
		return
	}
	if d.running {
		d.mu.Unlock()
		return
	}
	d.running = true
	d.stopCh = make(chan struct{})
	d.mu.Unlock()

	d.wg.Add(1)
	go d.cleanLoop()
}

func (d *Deduplicator) Stop() {
	d.mu.Lock()
	if d.stopped {
		d.mu.Unlock()
		return
	}
	d.stopped = true
	if d.running {
		d.running = false
		close(d.stopCh)
	}
	d.mu.Unlock()

	d.wg.Wait()
}

func (d *Deduplicator) CheckAndMark(msgID string) (bool, error) {
	if msgID == "" {
		return false, ErrEmptyMessageID
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return false, ErrDeduplicatorStop
	}

	now := time.Now()
	cutoff := now.Add(-d.cfg.WindowSize)

	if elem, exists := d.idMap[msgID]; exists {
		entry := elem.Value.(*idEntry)
		if entry.createdAt.After(cutoff) {
			d.idList.MoveToBack(elem)
			entry.createdAt = now
			return false, nil
		}
		d.idList.MoveToBack(elem)
		entry.createdAt = now
		return true, nil
	}

	entry := &idEntry{
		id:        msgID,
		createdAt: now,
	}
	elem := d.idList.PushBack(entry)
	d.idMap[msgID] = elem
	return true, nil
}

func (d *Deduplicator) Contains(msgID string) (bool, error) {
	if msgID == "" {
		return false, ErrEmptyMessageID
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return false, ErrDeduplicatorStop
	}

	elem, exists := d.idMap[msgID]
	if !exists {
		return false, nil
	}

	entry := elem.Value.(*idEntry)
	cutoff := time.Now().Add(-d.cfg.WindowSize)
	return entry.createdAt.After(cutoff), nil
}

func (d *Deduplicator) Count() (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return 0, ErrDeduplicatorStop
	}

	cutoff := time.Now().Add(-d.cfg.WindowSize)
	expiredCount := 0

	for elem := d.idList.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*idEntry)
		if entry.createdAt.After(cutoff) {
			break
		}
		expiredCount++
	}

	return d.idList.Len() - expiredCount, nil
}

func (d *Deduplicator) Clear() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return ErrDeduplicatorStop
	}

	d.idMap = make(map[string]*list.Element)
	d.idList.Init()
	return nil
}

func (d *Deduplicator) CleanExpired() (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return 0, ErrDeduplicatorStop
	}

	return d.cleanExpiredLocked(), nil
}

func (d *Deduplicator) cleanExpiredLocked() int {
	cutoff := time.Now().Add(-d.cfg.WindowSize)
	cleaned := 0

	for {
		front := d.idList.Front()
		if front == nil {
			break
		}
		entry := front.Value.(*idEntry)
		if entry.createdAt.After(cutoff) {
			break
		}
		d.idList.Remove(front)
		delete(d.idMap, entry.id)
		cleaned++
	}

	return cleaned
}

func (d *Deduplicator) cleanLoop() {
	defer d.wg.Done()

	ticker := time.NewTicker(d.cfg.CleanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			_, _ = d.CleanExpired()
		}
	}
}
