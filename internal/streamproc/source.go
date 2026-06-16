package streamproc

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type ChannelSource struct {
	seqCounter int64

	name       string
	input      <-chan *Record
	output     chan *Record
	state      SourceState
	stateMu    sync.RWMutex
	bufferSize int

	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	pauseCh  chan struct{}
	resumeCh chan struct{}
}

func NewChannelSource(name string, input <-chan *Record, bufferSize int) *ChannelSource {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &ChannelSource{
		name:       name,
		input:      input,
		output:     make(chan *Record, bufferSize),
		bufferSize: bufferSize,
		state:      SourceStateIdle,
		pauseCh:    make(chan struct{}),
		resumeCh:   make(chan struct{}),
	}
}

func (s *ChannelSource) Name() string {
	return s.name
}

func (s *ChannelSource) State() SourceState {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.state
}

func (s *ChannelSource) setState(st SourceState) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.state = st
}

func (s *ChannelSource) Output() <-chan *Record {
	return s.output
}

func (s *ChannelSource) Start(ctx context.Context) error {
	s.stateMu.Lock()
	if s.state == SourceStateRunning {
		s.stateMu.Unlock()
		return ErrSourceAlreadyStarted
	}
	if s.state == SourceStateStopped {
		s.stateMu.Unlock()
		return fmt.Errorf("streamproc: cannot restart stopped source")
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.state = SourceStateRunning
	s.stateMu.Unlock()

	s.wg.Add(1)
	go s.run()

	return nil
}

func (s *ChannelSource) run() {
	defer s.wg.Done()
	defer close(s.output)

	for {
		state := s.State()
		if state == SourceStateStopped {
			return
		}

		if state == SourceStatePaused {
			select {
			case <-s.ctx.Done():
				return
			case <-s.resumeCh:
				continue
			}
		}

		select {
		case <-s.ctx.Done():
			return
		case rec, ok := <-s.input:
			if !ok {
				return
			}
			rec.SeqID = atomic.AddInt64(&s.seqCounter, 1)
			if rec.Timestamp.IsZero() {
				rec.Timestamp = time.Now()
			}
			select {
			case s.output <- rec:
			case <-s.ctx.Done():
				return
			}
		}
	}
}

func (s *ChannelSource) Pause() error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.state != SourceStateRunning {
		return ErrSourceNotStarted
	}
	s.state = SourceStatePaused
	return nil
}

func (s *ChannelSource) Resume() error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.state != SourceStatePaused {
		return fmt.Errorf("streamproc: source is not paused")
	}
	s.state = SourceStateRunning
	select {
	case s.resumeCh <- struct{}{}:
	default:
	}
	return nil
}

func (s *ChannelSource) Stop() error {
	s.stateMu.Lock()
	if s.state == SourceStateStopped {
		s.stateMu.Unlock()
		return nil
	}
	s.state = SourceStateStopped
	s.stateMu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	return nil
}

type SliceSource struct {
	seqCounter int64

	name       string
	data       []*Record
	output     chan *Record
	state      SourceState
	stateMu    sync.RWMutex
	bufferSize int

	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	interval   time.Duration
	currentIdx int
	idxMu      sync.Mutex
}

func NewSliceSource(name string, data []*Record, bufferSize int, interval time.Duration) *SliceSource {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	if interval < 0 {
		interval = 0
	}
	records := make([]*Record, len(data))
	for i, r := range data {
		if r != nil {
			records[i] = r.Clone()
		}
	}
	return &SliceSource{
		name:       name,
		data:       records,
		output:     make(chan *Record, bufferSize),
		bufferSize: bufferSize,
		state:      SourceStateIdle,
		interval:   interval,
	}
}

func (s *SliceSource) Name() string {
	return s.name
}

func (s *SliceSource) State() SourceState {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.state
}

func (s *SliceSource) setState(st SourceState) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.state = st
}

func (s *SliceSource) Output() <-chan *Record {
	return s.output
}

func (s *SliceSource) Start(ctx context.Context) error {
	s.stateMu.Lock()
	if s.state == SourceStateRunning {
		s.stateMu.Unlock()
		return ErrSourceAlreadyStarted
	}
	if s.state == SourceStateStopped {
		s.stateMu.Unlock()
		return fmt.Errorf("streamproc: cannot restart stopped source")
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.state = SourceStateRunning
	s.stateMu.Unlock()

	s.wg.Add(1)
	go s.run()

	return nil
}

func (s *SliceSource) run() {
	defer s.wg.Done()
	defer close(s.output)

	var ticker *time.Ticker
	if s.interval > 0 {
		ticker = time.NewTicker(s.interval)
		defer ticker.Stop()
	}

	for {
		state := s.State()
		if state == SourceStateStopped {
			return
		}
		if state == SourceStatePaused {
			select {
			case <-s.ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
				continue
			}
		}

		s.idxMu.Lock()
		idx := s.currentIdx
		if idx >= len(s.data) {
			s.idxMu.Unlock()
			return
		}
		rec := s.data[idx]
		s.currentIdx++
		s.idxMu.Unlock()

		if rec != nil {
			rec.SeqID = atomic.AddInt64(&s.seqCounter, 1)
			if rec.Timestamp.IsZero() {
				rec.Timestamp = time.Now()
			}
			select {
			case s.output <- rec:
			case <-s.ctx.Done():
				return
			}
		}

		if s.interval > 0 {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}
}

func (s *SliceSource) Pause() error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.state != SourceStateRunning {
		return ErrSourceNotStarted
	}
	s.state = SourceStatePaused
	return nil
}

func (s *SliceSource) Resume() error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.state != SourceStatePaused {
		return fmt.Errorf("streamproc: source is not paused")
	}
	s.state = SourceStateRunning
	return nil
}

func (s *SliceSource) Stop() error {
	s.stateMu.Lock()
	if s.state == SourceStateStopped {
		s.stateMu.Unlock()
		return nil
	}
	s.state = SourceStateStopped
	s.stateMu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	return nil
}

func (s *SliceSource) Reset() {
	s.idxMu.Lock()
	s.currentIdx = 0
	s.idxMu.Unlock()
	atomic.StoreInt64(&s.seqCounter, 0)
}

func (s *SliceSource) CurrentIndex() int {
	s.idxMu.Lock()
	defer s.idxMu.Unlock()
	return s.currentIdx
}

type GeneratorSource struct {
	seqCounter int64
	maxCount   int64

	name       string
	generator  func(seq int64) *Record
	output     chan *Record
	state      SourceState
	stateMu    sync.RWMutex
	bufferSize int

	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	interval time.Duration
}

func NewGeneratorSource(name string, generator func(seq int64) *Record, maxCount int64, bufferSize int, interval time.Duration) *GeneratorSource {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	if interval < 0 {
		interval = 0
	}
	return &GeneratorSource{
		name:       name,
		generator:  generator,
		maxCount:   maxCount,
		output:     make(chan *Record, bufferSize),
		bufferSize: bufferSize,
		state:      SourceStateIdle,
		interval:   interval,
	}
}

func (s *GeneratorSource) Name() string {
	return s.name
}

func (s *GeneratorSource) State() SourceState {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.state
}

func (s *GeneratorSource) Output() <-chan *Record {
	return s.output
}

func (s *GeneratorSource) Start(ctx context.Context) error {
	s.stateMu.Lock()
	if s.state == SourceStateRunning {
		s.stateMu.Unlock()
		return ErrSourceAlreadyStarted
	}
	if s.state == SourceStateStopped {
		s.stateMu.Unlock()
		return fmt.Errorf("streamproc: cannot restart stopped source")
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.state = SourceStateRunning
	s.stateMu.Unlock()

	s.wg.Add(1)
	go s.run()

	return nil
}

func (s *GeneratorSource) run() {
	defer s.wg.Done()
	defer close(s.output)

	var ticker *time.Ticker
	if s.interval > 0 {
		ticker = time.NewTicker(s.interval)
		defer ticker.Stop()
	}

	for {
		state := s.State()
		if state == SourceStateStopped {
			return
		}
		if state == SourceStatePaused {
			select {
			case <-s.ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
				continue
			}
		}

		seq := atomic.AddInt64(&s.seqCounter, 1)
		if s.maxCount > 0 && seq > s.maxCount {
			return
		}

		rec := s.generator(seq)
		if rec == nil {
			continue
		}
		rec.SeqID = seq
		if rec.Timestamp.IsZero() {
			rec.Timestamp = time.Now()
		}

		select {
		case s.output <- rec:
		case <-s.ctx.Done():
			return
		}

		if s.interval > 0 {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}
}

func (s *GeneratorSource) Pause() error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.state != SourceStateRunning {
		return ErrSourceNotStarted
	}
	s.state = SourceStatePaused
	return nil
}

func (s *GeneratorSource) Resume() error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.state != SourceStatePaused {
		return fmt.Errorf("streamproc: source is not paused")
	}
	s.state = SourceStateRunning
	return nil
}

func (s *GeneratorSource) Stop() error {
	s.stateMu.Lock()
	if s.state == SourceStateStopped {
		s.stateMu.Unlock()
		return nil
	}
	s.state = SourceStateStopped
	s.stateMu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	return nil
}
