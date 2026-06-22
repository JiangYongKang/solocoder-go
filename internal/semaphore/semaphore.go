package semaphore

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrInvalidPermits    = errors.New("semaphore: permits must be non-negative")
	ErrNegativePermits   = errors.New("semaphore: total permits cannot be negative")
	ErrInvalidDelta      = errors.New("semaphore: delta must be greater than 0")
)

type waiter struct {
	ch chan bool
}

type Semaphore struct {
	mu           sync.Mutex
	held         int
	totalPermits int
	fair         bool
	waiters      []*waiter
}

func New(permits int, fair ...bool) (*Semaphore, error) {
	if permits < 0 {
		return nil, ErrInvalidPermits
	}

	isFair := false
	if len(fair) > 0 {
		isFair = fair[0]
	}

	return &Semaphore{
		held:         0,
		totalPermits: permits,
		fair:         isFair,
		waiters:      make([]*waiter, 0),
	}, nil
}

func (s *Semaphore) available() int {
	if s.held >= s.totalPermits {
		return 0
	}
	return s.totalPermits - s.held
}

func (s *Semaphore) Acquire(timeout time.Duration) bool {
	s.mu.Lock()

	if s.available() > 0 && (!s.fair || len(s.waiters) == 0) {
		s.held++
		s.mu.Unlock()
		return true
	}

	w := &waiter{
		ch: make(chan bool, 1),
	}
	s.waiters = append(s.waiters, w)

	var timer *time.Timer
	if timeout > 0 {
		timer = time.AfterFunc(timeout, func() {
			s.mu.Lock()
			defer s.mu.Unlock()

			for i, waiter := range s.waiters {
				if waiter == w {
					s.waiters = append(s.waiters[:i], s.waiters[i+1:]...)
					w.ch <- false
					return
				}
			}
		})
	}

	s.mu.Unlock()

	result := <-w.ch

	if timer != nil {
		timer.Stop()
	}

	return result
}

func (s *Semaphore) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.held <= 0 {
		return
	}

	s.held--
	s.dispatchWaiters()
}

func (s *Semaphore) dispatchWaiters() {
	for s.available() > 0 && len(s.waiters) > 0 {
		w := s.waiters[0]
		s.waiters = s.waiters[1:]
		s.held++
		w.ch <- true
	}
}

func (s *Semaphore) TryAcquire() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.available() > 0 && (!s.fair || len(s.waiters) == 0) {
		s.held++
		return true
	}
	return false
}

func (s *Semaphore) IncreasePermits(delta int) error {
	if delta <= 0 {
		return ErrInvalidDelta
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.totalPermits += delta

	s.dispatchWaiters()

	return nil
}

func (s *Semaphore) DecreasePermits(delta int) error {
	if delta <= 0 {
		return ErrInvalidDelta
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.totalPermits-delta < 0 {
		return ErrNegativePermits
	}

	s.totalPermits -= delta

	return nil
}

func (s *Semaphore) AvailablePermits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.available()
}

func (s *Semaphore) TotalPermits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.totalPermits
}

func (s *Semaphore) QueueLength() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.waiters)
}

func (s *Semaphore) IsFair() bool {
	return s.fair
}
