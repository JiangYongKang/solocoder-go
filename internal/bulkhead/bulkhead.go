package bulkhead

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrBulkheadClosed     = errors.New("bulkhead: bulkhead is closed")
	ErrBulkheadFull       = errors.New("bulkhead: bulkhead is full")
	ErrBulkheadTimeout    = errors.New("bulkhead: wait timeout")
	ErrInvalidConcurrency = errors.New("bulkhead: max concurrency must be greater than 0")
	ErrInvalidQueueSize   = errors.New("bulkhead: max queue size must be greater than or equal to 0")
	ErrInvalidName        = errors.New("bulkhead: name must not be empty")
)

type Task func()

type FullError struct {
	Name           string
	ActiveCount    int
	MaxConcurrency int
	QueueLength    int
	MaxQueueSize   int
}

func (e *FullError) Error() string {
	return fmt.Sprintf(
		"bulkhead '%s' is full: active=%d/%d, queue=%d/%d",
		e.Name, e.ActiveCount, e.MaxConcurrency, e.QueueLength, e.MaxQueueSize,
	)
}

type Config struct {
	MaxConcurrency int
	MaxQueueSize   int
	WaitTimeout    time.Duration
}

type Bulkhead struct {
	name           string
	maxConcurrency int
	maxQueueSize   int
	waitTimeout    time.Duration

	mu             sync.Mutex
	cond           *sync.Cond
	taskQueue      []Task
	active         int
	idleWorkers    int
	closed         bool
	workerCnt      int
	shrinkCnt      int
	wg             sync.WaitGroup
}

func NewBulkhead(name string, cfg Config) (*Bulkhead, error) {
	if name == "" {
		return nil, ErrInvalidName
	}
	if cfg.MaxConcurrency <= 0 {
		return nil, ErrInvalidConcurrency
	}
	if cfg.MaxQueueSize < 0 {
		return nil, ErrInvalidQueueSize
	}

	b := &Bulkhead{
		name:           name,
		maxConcurrency: cfg.MaxConcurrency,
		maxQueueSize:   cfg.MaxQueueSize,
		waitTimeout:    cfg.WaitTimeout,
		taskQueue:      make([]Task, 0),
	}
	b.cond = sync.NewCond(&b.mu)

	for i := 0; i < cfg.MaxConcurrency; i++ {
		b.wg.Add(1)
		b.workerCnt++
		go b.worker()
	}

	b.mu.Lock()
	for b.idleWorkers < cfg.MaxConcurrency {
		b.cond.Wait()
	}
	b.mu.Unlock()

	return b, nil
}

func (b *Bulkhead) worker() {
	defer b.wg.Done()

	b.mu.Lock()
	defer b.mu.Unlock()

	for {
		for len(b.taskQueue) == 0 && !b.closed && b.shrinkCnt == 0 {
			b.idleWorkers++
			b.cond.Broadcast()
			b.cond.Wait()
			b.idleWorkers--
		}

		if len(b.taskQueue) == 0 {
			if b.closed {
				b.workerCnt--
				return
			}
			if b.shrinkCnt > 0 {
				b.shrinkCnt--
				b.workerCnt--
				return
			}
		}

		var task Task
		if len(b.taskQueue) > 0 {
			task = b.taskQueue[0]
			b.taskQueue = b.taskQueue[1:]
			b.active++
		}

		b.cond.Broadcast()
		b.mu.Unlock()

		if task != nil {
			task()
		}

		b.mu.Lock()
		if task != nil {
			b.active--
			b.cond.Broadcast()
		}
	}
}

func (b *Bulkhead) canSubmit() bool {
	if len(b.taskQueue) < b.maxQueueSize {
		return true
	}
	if b.idleWorkers > 0 {
		return true
	}
	return false
}

func (b *Bulkhead) Submit(task Task) error {
	if task == nil {
		return errors.New("bulkhead: task must not be nil")
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return ErrBulkheadClosed
	}

	if b.canSubmit() {
		b.taskQueue = append(b.taskQueue, task)
		b.cond.Signal()
		b.mu.Unlock()
		return nil
	}

	if b.waitTimeout <= 0 {
		err := &FullError{
			Name:           b.name,
			ActiveCount:    b.active,
			MaxConcurrency: b.maxConcurrency,
			QueueLength:    len(b.taskQueue),
			MaxQueueSize:   b.maxQueueSize,
		}
		b.mu.Unlock()
		return err
	}

	deadline := time.Now().Add(b.waitTimeout)
	timer := time.AfterFunc(b.waitTimeout, func() {
		b.mu.Lock()
		b.cond.Broadcast()
		b.mu.Unlock()
	})
	defer timer.Stop()

	for !b.canSubmit() {
		if b.closed {
			b.mu.Unlock()
			return ErrBulkheadClosed
		}
		if time.Now().After(deadline) {
			err := &FullError{
				Name:           b.name,
				ActiveCount:    b.active,
				MaxConcurrency: b.maxConcurrency,
				QueueLength:    len(b.taskQueue),
				MaxQueueSize:   b.maxQueueSize,
			}
			b.mu.Unlock()
			return err
		}
		b.cond.Wait()
	}

	b.taskQueue = append(b.taskQueue, task)
	b.cond.Signal()
	b.mu.Unlock()
	return nil
}

func (b *Bulkhead) TrySubmit(task Task) (bool, error) {
	if task == nil {
		return false, errors.New("bulkhead: task must not be nil")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return false, ErrBulkheadClosed
	}

	if !b.canSubmit() {
		return false, nil
	}

	b.taskQueue = append(b.taskQueue, task)
	b.cond.Signal()
	return true, nil
}

func (b *Bulkhead) ActiveCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.active
}

func (b *Bulkhead) QueueLength() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.taskQueue)
}

func (b *Bulkhead) MaxConcurrency() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.maxConcurrency
}

func (b *Bulkhead) MaxQueueSize() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.maxQueueSize
}

func (b *Bulkhead) Name() string {
	return b.name
}

func (b *Bulkhead) Resize(maxConcurrency int, maxQueueSize int) error {
	if maxConcurrency <= 0 {
		return ErrInvalidConcurrency
	}
	if maxQueueSize < 0 {
		return ErrInvalidQueueSize
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return ErrBulkheadClosed
	}

	b.resizeConcurrency(maxConcurrency)
	b.resizeQueue(maxQueueSize)

	return nil
}

func (b *Bulkhead) resizeConcurrency(newMax int) {
	if newMax == b.maxConcurrency {
		return
	}

	if newMax > b.maxConcurrency {
		diff := newMax - b.maxConcurrency
		for i := 0; i < diff; i++ {
			b.wg.Add(1)
			b.workerCnt++
			go b.worker()
		}
	} else {
		diff := b.maxConcurrency - newMax
		b.shrinkCnt += diff
		b.cond.Broadcast()
	}

	b.maxConcurrency = newMax
}

func (b *Bulkhead) resizeQueue(newSize int) {
	if newSize == b.maxQueueSize {
		return
	}

	b.maxQueueSize = newSize

	if newSize > 0 {
		b.cond.Broadcast()
	}
}

func (b *Bulkhead) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	b.cond.Broadcast()
	b.mu.Unlock()

	b.wg.Wait()
}

func (b *Bulkhead) WorkerCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.workerCnt
}

type Registry struct {
	mu        sync.RWMutex
	bulkheads map[string]*Bulkhead
}

func NewRegistry() *Registry {
	return &Registry{
		bulkheads: make(map[string]*Bulkhead),
	}
}

func (r *Registry) NewBulkhead(name string, cfg Config) (*Bulkhead, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.bulkheads[name]; exists {
		return nil, fmt.Errorf("bulkhead: bulkhead '%s' already exists", name)
	}

	b, err := NewBulkhead(name, cfg)
	if err != nil {
		return nil, err
	}

	r.bulkheads[name] = b
	return b, nil
}

func (r *Registry) Get(name string) (*Bulkhead, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.bulkheads[name]
	return b, ok
}

func (r *Registry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if b, ok := r.bulkheads[name]; ok {
		b.Close()
		delete(r.bulkheads, name)
	}
}

func (r *Registry) CloseAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, b := range r.bulkheads {
		b.Close()
		delete(r.bulkheads, name)
	}
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.bulkheads))
	for name := range r.bulkheads {
		names = append(names, name)
	}
	return names
}
