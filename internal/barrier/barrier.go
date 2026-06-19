package barrier

import (
	"errors"
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

type CyclicCallbackFunc func(round uint64) error

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
	released        bool
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

func (b *Barrier) releaseWithCallbackLocked(cbErr error) []chan error {
	chans := make([]chan error, 0, len(b.waiters))
	for _, w := range b.waiters {
		chans = append(chans, w.done)
	}
	b.waiters = make(map[uint64]*waiter)
	b.waiting -= b.arrived
	b.arrived = 0
	b.effectiveNeeded = b.participants
	b.generation++
	b.released = true
	return chans
}

func (b *Barrier) doRelease() error {
	var cbErr error
	callback := b.callback
	chans := b.releaseWithCallbackLocked(cbErr)
	b.mu.Unlock()

	if callback != nil {
		cbErr = callback()
	}

	for _, ch := range chans {
		ch <- cbErr
		close(ch)
	}

	return cbErr
}

func (b *Barrier) Wait(timeout time.Duration) error {
	b.mu.Lock()

	if b.broken {
		b.mu.Unlock()
		return ErrBroken
	}

	b.released = false

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
		return b.doRelease()
	}

	var timer *time.Timer
	if timeout > 0 {
		timer = time.AfterFunc(timeout, func() {
			b.mu.Lock()

			if b.generation != gen {
				b.mu.Unlock()
				return
			}

			if _, ok := b.waiters[waiterID]; !ok {
				b.mu.Unlock()
				return
			}

			delete(b.waiters, waiterID)
			b.arrived--
			b.waiting--
			b.effectiveNeeded--

			if b.effectiveNeeded <= 0 {
				b.effectiveNeeded = 1
			}

			w.done <- ErrTimeout
			close(w.done)

			if b.arrived >= b.effectiveNeeded && b.arrived > 0 {
				b.doRelease()
				return
			}

			b.mu.Unlock()
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
	b.released = false

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
	b.released = false

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

func (b *Barrier) IsReleased() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.released
}

type CyclicBarrier struct {
	mu       sync.Mutex
	barrier  *Barrier
	round    uint64
	callback CyclicCallbackFunc
}

func NewCyclic(parties int, callback ...CyclicCallbackFunc) (*CyclicBarrier, error) {
	if parties <= 0 {
		return nil, ErrInvalidParticipants
	}

	cb := &CyclicBarrier{
		round: 0,
	}

	if len(callback) > 0 && callback[0] != nil {
		cb.callback = callback[0]
	}

	b, err := New(parties, cb.wrapCallback())
	if err != nil {
		return nil, err
	}
	cb.barrier = b

	return cb, nil
}

func (cb *CyclicBarrier) wrapCallback() CallbackFunc {
	return func() error {
		cb.mu.Lock()
		currentRound := cb.round
		cb.mu.Unlock()

		if cb.callback != nil {
			return cb.callback(currentRound)
		}
		return nil
	}
}

func (cb *CyclicBarrier) Await(timeout time.Duration) error {
	cb.mu.Lock()
	if cb.barrier.IsReleased() {
		cb.round++
		cb.barrier.Reset()
	}
	cb.mu.Unlock()

	return cb.barrier.Wait(timeout)
}

func (cb *CyclicBarrier) GetNumberWaiting() int {
	return cb.barrier.Waiting()
}

func (cb *CyclicBarrier) GetParties() int {
	return cb.barrier.Participants()
}

func (cb *CyclicBarrier) GetRound() uint64 {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.round
}

func (cb *CyclicBarrier) ResetBarrier(newParties ...int) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	err := cb.barrier.Reset(newParties...)
	if err != nil {
		return err
	}
	cb.round = 0
	return nil
}

func (cb *CyclicBarrier) ForceReset(newParties ...int) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	err := cb.barrier.ForceReset(newParties...)
	if err != nil {
		return err
	}
	cb.round = 0
	return nil
}

func (cb *CyclicBarrier) IsBroken() bool {
	return cb.barrier.IsBroken()
}

func (cb *CyclicBarrier) SetCallback(c CyclicCallbackFunc) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.callback = c
	cb.barrier.SetCallback(cb.wrapCallback())
}
