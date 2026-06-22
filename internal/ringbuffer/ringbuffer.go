package ringbuffer

import (
	"errors"
	"sync"
)

var (
	ErrInvalidCapacity  = errors.New("ringbuffer: invalid capacity")
	ErrInvalidHighWater = errors.New("ringbuffer: invalid high water mark")
)

type OverwriteStrategy int

const (
	NoOverwrite OverwriteStrategy = iota
	Overwrite
)

type RingBuffer[T any] struct {
	mu             sync.Mutex
	buf            []T
	capacity       int
	readPos        int
	writePos       int
	count          int
	strategy       OverwriteStrategy
	highWater      int
	highWaterAlarm bool
	onHighWater    func()
	onLowWater     func()
}

type Config struct {
	Capacity      int
	Strategy      OverwriteStrategy
	HighWaterMark int
}

func DefaultConfig() Config {
	return Config{
		Capacity:      1024,
		Strategy:      NoOverwrite,
		HighWaterMark: 0,
	}
}

func NewRingBuffer[T any](capacity int) (*RingBuffer[T], error) {
	cfg := DefaultConfig()
	cfg.Capacity = capacity
	return NewRingBufferWithConfig[T](cfg)
}

func NewRingBufferWithConfig[T any](cfg Config) (*RingBuffer[T], error) {
	if cfg.Capacity <= 0 {
		return nil, ErrInvalidCapacity
	}
	if cfg.HighWaterMark < 0 || cfg.HighWaterMark > cfg.Capacity {
		return nil, ErrInvalidHighWater
	}

	rb := &RingBuffer[T]{
		buf:       make([]T, cfg.Capacity),
		capacity:  cfg.Capacity,
		strategy:  cfg.Strategy,
		highWater: cfg.HighWaterMark,
	}

	return rb, nil
}

func (rb *RingBuffer[T]) SetHighWaterMark(mark int) error {
	var (
		needHigh bool
		needLow  bool
		err      error
	)

	func() {
		rb.mu.Lock()
		defer rb.mu.Unlock()

		if mark < 0 || mark > rb.capacity {
			err = ErrInvalidHighWater
			return
		}

		rb.highWater = mark

		if rb.highWater > 0 {
			if rb.count >= rb.highWater && !rb.highWaterAlarm {
				rb.highWaterAlarm = true
				needHigh = true
			} else if rb.count < rb.highWater && rb.highWaterAlarm {
				rb.highWaterAlarm = false
				needLow = true
			}
		}
	}()

	if err != nil {
		return err
	}

	rb.dispatchCallbacks(needHigh, needLow)

	return nil
}

func (rb *RingBuffer[T]) OnHighWater(fn func()) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.onHighWater = fn
}

func (rb *RingBuffer[T]) OnLowWater(fn func()) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.onLowWater = fn
}

func (rb *RingBuffer[T]) Write(value T) bool {
	var (
		needHigh bool
		needLow  bool
		result   bool
	)

	func() {
		rb.mu.Lock()
		defer rb.mu.Unlock()

		if rb.strategy == NoOverwrite && rb.count == rb.capacity {
			result = false
			return
		}

		overwrote := false
		var zero T
		if rb.strategy == Overwrite && rb.count == rb.capacity {
			rb.buf[rb.readPos] = zero
			rb.readPos = (rb.readPos + 1) % rb.capacity
			rb.count--
			overwrote = true
		}

		rb.buf[rb.writePos] = value
		rb.writePos = (rb.writePos + 1) % rb.capacity
		rb.count++

		needHigh, needLow = rb.checkWaterMarkLocked(overwrote)
		result = true
	}()

	rb.dispatchCallbacks(needHigh, needLow)

	return result
}

func (rb *RingBuffer[T]) Read() (T, bool) {
	var (
		needHigh bool
		needLow  bool
		value    T
		ok       bool
	)

	func() {
		rb.mu.Lock()
		defer rb.mu.Unlock()

		var zero T
		if rb.count == 0 {
			value = zero
			ok = false
			return
		}

		value = rb.buf[rb.readPos]
		rb.buf[rb.readPos] = zero
		rb.readPos = (rb.readPos + 1) % rb.capacity
		rb.count--

		needHigh, needLow = rb.checkWaterMarkLocked(false)
		ok = true
	}()

	rb.dispatchCallbacks(needHigh, needLow)

	return value, ok
}

func (rb *RingBuffer[T]) Peek() (T, bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	var zero T
	if rb.count == 0 {
		return zero, false
	}

	return rb.buf[rb.readPos], true
}

func (rb *RingBuffer[T]) Len() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.count
}

func (rb *RingBuffer[T]) Cap() int {
	return rb.capacity
}

func (rb *RingBuffer[T]) IsFull() bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.count == rb.capacity
}

func (rb *RingBuffer[T]) IsEmpty() bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.count == 0
}

func (rb *RingBuffer[T]) SetStrategy(strategy OverwriteStrategy) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.strategy = strategy
}

func (rb *RingBuffer[T]) GetStrategy() OverwriteStrategy {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.strategy
}

func (rb *RingBuffer[T]) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	var zero T
	for i := range rb.buf {
		rb.buf[i] = zero
	}
	rb.readPos = 0
	rb.writePos = 0
	rb.count = 0
	rb.highWaterAlarm = false
}

func (rb *RingBuffer[T]) checkWaterMarkLocked(overwrote bool) (needHigh bool, needLow bool) {
	if rb.highWater <= 0 {
		return false, false
	}

	if rb.count >= rb.highWater && !rb.highWaterAlarm {
		rb.highWaterAlarm = true
		return true, false
	} else if rb.count < rb.highWater && rb.highWaterAlarm && !overwrote {
		rb.highWaterAlarm = false
		return false, true
	}

	return false, false
}

func (rb *RingBuffer[T]) dispatchCallbacks(needHigh bool, needLow bool) {
	var (
		highCb func()
		lowCb  func()
	)

	if needHigh || needLow {
		rb.mu.Lock()
		if needHigh {
			highCb = rb.onHighWater
		}
		if needLow {
			lowCb = rb.onLowWater
		}
		rb.mu.Unlock()
	}

	if highCb != nil {
		highCb()
	}
	if lowCb != nil {
		lowCb()
	}
}
