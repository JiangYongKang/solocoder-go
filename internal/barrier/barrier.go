package barrier

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrInvalidParticipants = errors.New("barrier: participants must be greater than 0")
	ErrTimeout             = errors.New("barrier: wait timed out")
	ErrBarrierReset        = errors.New("barrier: barrier was reset while waiting")
	ErrResetWhileWaiting   = errors.New("barrier: cannot reset while goroutines are waiting")
	ErrBroken              = errors.New("barrier: barrier is broken")
)

type CallbackFunc func() error

type waiter struct {
	id   uint64
	done chan error
}

type Barrier struct {
	mu              sync.Mutex
	participants    int
	effectiveNeeded int
	arrived         int
	waiting         int
	callback        CallbackFunc
	generation      uint64
	broken          bool
	waiters         map[uint64]*waiter
	nextWaiterID    uint64
}

func New(participants int, callback ...CallbackFunc) (*Barrier, error) {
	if participants <= 0 {
		return nil, ErrInvalidParticipants
	}

	b := &Barrier{
		participants:    participants,
		effectiveNeeded: participants,
		waiters:         make(map[uint64]*waiter),
	}

	if len(callback) > 0 && callback[0] != nil {
		b.callback = callback[0]
	}

	return b, nil
}

func (b *Barrier) releaseAllLocked(cbErr error) {
	for _, w := range b.waiters {
		w.done <- cbErr
		close(w.done)
	}
	b.waiters = make(map[uint64]*waiter)
	b.waiting -= b.arrived
	b.arrived = 0
	b.effectiveNeeded = b.participants
	b.generation++
}

func (b *Barrier) Wait(timeout time.Duration) error {
	b.mu.Lock()

	if b.broken {
		b.mu.Unlock()
		return ErrBroken
	}

	gen := b.generation
	waiterID := b.nextWaiterID
	b.nextWaiterID++

	w := &waiter{
		id:   waiterID,
		done: make(chan error, 1),
	}
	b.waiters[waiterID] = w
	b.arrived++
	b.waiting++

	if b.arrived >= b.effectiveNeeded {
		var cbErr error
		if b.callback != nil {
			cbErr = b.callback()
		}
		b.releaseAllLocked(cbErr)
		b.mu.Unlock()
		return cbErr
	}

	var timer *time.Timer
	if timeout > 0 {
		timer = time.AfterFunc(timeout, func() {
			b.mu.Lock()
			defer b.mu.Unlock()

			if b.generation != gen {
				return
			}

			if _, ok := b.waiters[waiterID]; !ok {
				return
			}

			delete(b.waiters, waiterID)
			b.arrived--
			b.waiting--
			b.effectiveNeeded--

			w.done <- ErrTimeout
			close(w.done)

			if b.effectiveNeeded <= 0 {
				b.effectiveNeeded = 1
			}

			if b.arrived >= b.effectiveNeeded && b.arrived > 0 {
				var cbErr error
				if b.callback != nil {
					cbErr = b.callback()
				}
				b.releaseAllLocked(cbErr)
			}
		})
	}

	b.mu.Unlock()

	err := <-w.done

	if timer != nil {
		timer.Stop()
	}

	return err
}

func (b *Barrier) Reset(newParticipants ...int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.waiting > 0 {
		return ErrResetWhileWaiting
	}

	if len(newParticipants) > 0 {
		np := newParticipants[0]
		if np <= 0 {
			return ErrInvalidParticipants
		}
		b.participants = np
	}

	b.effectiveNeeded = b.participants
	b.generation++
	b.arrived = 0
	b.broken = false

	return nil
}

func (b *Barrier) Break() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.broken = true
	b.generation++

	for _, w := range b.waiters {
		w.done <- ErrBroken
		close(w.done)
	}
	b.waiters = make(map[uint64]*waiter)
	b.waiting -= b.arrived
	b.arrived = 0
	b.effectiveNeeded = b.participants
}

func (b *Barrier) ForceReset(newParticipants ...int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(newParticipants) > 0 {
		np := newParticipants[0]
		if np <= 0 {
			return ErrInvalidParticipants
		}
		b.participants = np
	}

	b.effectiveNeeded = b.participants
	b.generation++

	for _, w := range b.waiters {
		w.done <- ErrBarrierReset
		close(w.done)
	}
	b.waiters = make(map[uint64]*waiter)
	b.waiting -= b.arrived
	b.arrived = 0
	b.broken = false

	return nil
}

func (b *Barrier) SetCallback(cb CallbackFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.callback = cb
}

func (b *Barrier) Participants() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.participants
}

func (b *Barrier) Arrived() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.arrived
}

func (b *Barrier) Waiting() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.waiting
}

func (b *Barrier) EffectiveNeeded() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.effectiveNeeded
}

func (b *Barrier) IsBroken() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.broken
}

type CyclicBarrier struct {
	*Barrier
}

func NewCyclic(parties int, callback ...CallbackFunc) (*CyclicBarrier, error) {
	b, err := New(parties, callback...)
	if err != nil {
		return nil, err
	}
	return &CyclicBarrier{Barrier: b}, nil
}

func (cb *CyclicBarrier) Await(timeout time.Duration) error {
	return cb.Barrier.Wait(timeout)
}

func (cb *CyclicBarrier) GetNumberWaiting() int {
	return cb.Waiting()
}

func (cb *CyclicBarrier) GetParties() int {
	return cb.Participants()
}

func (cb *CyclicBarrier) ResetBarrier(newParties ...int) error {
	return cb.Reset(newParties...)
}

var _ = fmt.Sprintf
