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
	rb.mu.Lock()

	if mark < 0 || mark > rb.capacity {
		rb.mu.Unlock()
		return ErrInvalidHighWater
	}

	rb.highWater = mark

	var callback func()
	if rb.highWater > 0 {
		if rb.count >= rb.highWater && !rb.highWaterAlarm {
			rb.highWaterAlarm = true
			callback = rb.onHighWater
		} else if rb.count < rb.highWater && rb.highWaterAlarm {
			rb.highWaterAlarm = false
			callback = rb.onLowWater
		}
	}

	rb.mu.Unlock()

	if callback != nil {
		callback()
	}

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
	rb.mu.Lock()

	if rb.strategy == NoOverwrite && rb.count == rb.capacity {
		rb.mu.Unlock()
		return false
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

	callback := rb.checkWaterMarkLocked(overwrote)

	rb.mu.Unlock()

	if callback != nil {
		callback()
	}

	return true
}

func (rb *RingBuffer[T]) Read() (T, bool) {
	rb.mu.Lock()

	var zero T
	if rb.count == 0 {
		rb.mu.Unlock()
		return zero, false
	}

	value := rb.buf[rb.readPos]
	rb.buf[rb.readPos] = zero
	rb.readPos = (rb.readPos + 1) % rb.capacity
	rb.count--

	callback := rb.checkWaterMarkLocked(false)

	rb.mu.Unlock()

	if callback != nil {
		callback()
	}

	return value, true
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

	var zero T
	for i := range rb.buf {
		rb.buf[i] = zero
	}
	rb.readPos = 0
	rb.writePos = 0
	rb.count = 0
	rb.highWaterAlarm = false

	rb.mu.Unlock()
}

func (rb *RingBuffer[T]) checkWaterMarkLocked(overwrote bool) func() {
	if rb.highWater <= 0 {
		return nil
	}

	if rb.count >= rb.highWater && !rb.highWaterAlarm {
		rb.highWaterAlarm = true
		return rb.onHighWater
	} else if rb.count < rb.highWater && rb.highWaterAlarm && !overwrote {
		rb.highWaterAlarm = false
		return rb.onLowWater
	}

	return nil
}
