package streamproc

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type PipelineConfig struct {
	Name                     string
	BufferSize               int
	BackpressureThreshold    int
	BackpressureWarningRatio  float64
	BackpressureCriticalRatio float64
	EnableCheckpoint         bool
	CheckpointInterval       time.Duration
	CheckpointStore          CheckpointStore
	WindowAggregator         *WindowAggregator
	Sink                     Sink
}

func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		BufferSize:                100,
		BackpressureThreshold:    100,
		BackpressureWarningRatio:  0.7,
		BackpressureCriticalRatio: 0.9,
		EnableCheckpoint:         false,
		CheckpointInterval:   30 * time.Second,
	}
}

type Pipeline struct {
	pendingCount int64
	sourceOffset int64
	stats        PipelineStats

	cfg        PipelineConfig
	source     Source
	operators  *OperatorChain
	window     *WindowAggregator
	sink       Sink
	checkpoint CheckpointStore

	status   PipelineStatus
	statusMu sync.RWMutex

	statsMu sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	recordCh      chan *Record
	windowResults chan *WindowResult

	backpressureMu sync.RWMutex

	checkpointTicker *time.Ticker
	stopCh           chan struct{}
	stopOnce         sync.Once

	processedSeq map[int64]bool
	processedMu  sync.Mutex
}

func NewPipeline(cfg PipelineConfig, source Source) (*Pipeline, error) {
	if source == nil {
		return nil, ErrSourceNil
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 100
	}
	if cfg.BackpressureThreshold <= 0 {
		return nil, ErrInvalidBackpressureThreshold
	}
	if cfg.BackpressureWarningRatio <= 0 {
		cfg.BackpressureWarningRatio = 0.7
	}
	if cfg.BackpressureCriticalRatio <= 0 {
		cfg.BackpressureCriticalRatio = 0.9
	}
	if cfg.BackpressureWarningRatio >= cfg.BackpressureCriticalRatio {
		return nil, fmt.Errorf("streamproc: warning ratio must be less than critical ratio")
	}
	if cfg.CheckpointInterval < 0 {
		cfg.CheckpointInterval = 0
	}

	if cfg.CheckpointStore == nil {
		cfg.CheckpointStore = NewMemoryCheckpointStore()
	}

	return &Pipeline{
		cfg:          cfg,
		source:       source,
		operators:    NewOperatorChain(),
		window:       cfg.WindowAggregator,
		sink:         cfg.Sink,
		checkpoint:   cfg.CheckpointStore,
		status:       PipelineStatusIdle,
		recordCh:     make(chan *Record, cfg.BufferSize),
		windowResults: make(chan *WindowResult, cfg.BufferSize),
		processedSeq: make(map[int64]bool),
		stopCh:     make(chan struct{}),
	}, nil
}

func (p *Pipeline) AddOperator(op Operator) error {
	p.statusMu.RLock()
	if p.status == PipelineStatusRunning {
		p.statusMu.RUnlock()
		return ErrPipelineRunning
	}
	p.statusMu.RUnlock()
	return p.operators.Add(op)
}

func (p *Pipeline) SetSink(sink Sink) {
	p.statusMu.RLock()
	if p.status == PipelineStatusRunning {
		p.statusMu.RUnlock()
		return
	}
	p.statusMu.RUnlock()
	p.sink = sink
}

func (p *Pipeline) SetWindowAggregator(w *WindowAggregator) {
	p.statusMu.RLock()
	if p.status == PipelineStatusRunning {
		p.statusMu.RUnlock()
		return
	}
	p.statusMu.RUnlock()
	p.window = w
}

func (p *Pipeline) Status() PipelineStatus {
	p.statusMu.RLock()
	defer p.statusMu.RUnlock()
	return p.status
}

func (p *Pipeline) setStatus(s PipelineStatus) {
	p.statusMu.Lock()
	defer p.statusMu.Unlock()
	p.status = s
}

func (p *Pipeline) Stats() PipelineStats {
	p.statsMu.RLock()
	defer p.statsMu.RUnlock()
	stats := p.stats
	if !stats.StartTime.IsZero() {
		stats.Elapsed = time.Since(stats.StartTime)
	}
	return stats
}

func (p *Pipeline) incrementRecordsIn() {
	atomic.AddInt64(&p.stats.RecordsIn, 1)
}

func (p *Pipeline) incrementRecordsOut() {
	atomic.AddInt64(&p.stats.RecordsOut, 1)
}

func (p *Pipeline) incrementRecordsDropped() {
	atomic.AddInt64(&p.stats.RecordsDropped, 1)
}

func (p *Pipeline) incrementWindowsClosed() {
	atomic.AddInt64(&p.stats.WindowsClosed, 1)
}

func (p *Pipeline) incrementCheckpointsMade() {
	atomic.AddInt64(&p.stats.CheckpointsMade, 1)
}

func (p *Pipeline) incrementErrors() {
	atomic.AddInt64(&p.stats.Errors, 1)
}

func (p *Pipeline) GetBackpressureInfo() BackpressureInfo {
	pending := atomic.LoadInt64(&p.pendingCount)
	threshold := int64(p.cfg.BackpressureThreshold)
	ratio := float64(pending) / float64(threshold)

	state := BackpressureNormal
	if ratio >= p.cfg.BackpressureCriticalRatio {
		state = BackpressureCritical
	} else if ratio >= p.cfg.BackpressureWarningRatio {
		state = BackpressureWarning
	}

	return BackpressureInfo{
		State:         state,
		PendingCount:  int(pending),
		Threshold:   p.cfg.BackpressureThreshold,
		WarningRatio:  p.cfg.BackpressureWarningRatio,
		CriticalRatio: p.cfg.BackpressureCriticalRatio,
	}
}

func (p *Pipeline) shouldApplyBackpressure() bool {
	info := p.GetBackpressureInfo()
	return info.State == BackpressureCritical
}

func (p *Pipeline) handleBackpressure() {
	for p.shouldApplyBackpressure() {
		state := p.source.State()
		if state == SourceStateRunning {
			_ = p.source.Pause()
		}
		select {
		case <-p.ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-time.After(10 * time.Millisecond):
		}
	}

	state := p.source.State()
	if state == SourceStatePaused {
		_ = p.source.Resume()
	}
}

func (p *Pipeline) Start(ctx context.Context) error {
	p.statusMu.Lock()
	if p.status == PipelineStatusRunning {
		p.statusMu.Unlock()
		return ErrPipelineRunning
	}
	if p.status == PipelineStatusStopped {
		p.statusMu.Unlock()
		return ErrPipelineStopped
	}
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = PipelineStatusRunning
	p.stats.StartTime = time.Now()
	p.stopCh = make(chan struct{})
	p.stopOnce = sync.Once{}
	p.statusMu.Unlock()

	if err := p.source.Start(p.ctx); err != nil {
		p.setStatus(PipelineStatusFailed)
		return fmt.Errorf("streamproc: start source: %w", err)
	}

	if p.window != nil {
		p.window.Start(p.ctx)
	}

	p.wg.Add(1)
	go p.sourceReader()

	p.wg.Add(1)
	go p.recordProcessor()

	if p.cfg.EnableCheckpoint && p.cfg.CheckpointInterval > 0 {
		p.wg.Add(1)
		go p.checkpointLoop()
	}

	if p.window != nil {
		p.wg.Add(1)
		go p.windowResultReader()
	}

	return nil
}

func (p *Pipeline) sourceReader() {
	defer p.wg.Done()

	sourceOutput := p.source.Output()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.stopCh:
			return
		case rec, ok := <-sourceOutput:
			if !ok {
				p.statusMu.Lock()
				if p.status != PipelineStatusStopped {
					p.status = PipelineStatusCompleted
				}
				p.statusMu.Unlock()
				return
			}
			if rec == nil {
				continue
			}

			p.incrementRecordsIn()

			select {
			case p.recordCh <- rec:
				atomic.AddInt64(&p.pendingCount, 1)
			case <-p.ctx.Done():
				return
			case <-p.stopCh:
				return
			}

			if p.shouldApplyBackpressure() {
				p.handleBackpressure()
			}
		}
	}
}

func (p *Pipeline) recordProcessor() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.stopCh:
			return
		case rec, ok := <-p.recordCh:
			if !ok {
				return
			}
			atomic.AddInt64(&p.pendingCount, -1)

			if rec == nil {
				continue
			}

			p.processedMu.Lock()
			p.processedSeq[rec.SeqID] = true
			p.processedMu.Unlock()

			atomic.StoreInt64(&p.sourceOffset, rec.SeqID)

			results, err := p.operators.Process(p.ctx, rec)
			if err != nil {
				p.incrementErrors()
				p.incrementRecordsDropped()
				continue
			}

			if results == nil || len(results) == 0 {
				p.incrementRecordsDropped()
				continue
			}

			for _, r := range results {
				if p.window != nil {
					_, err := p.window.Process(p.ctx, r)
					if err != nil {
						p.incrementErrors()
						continue
					}
				}

				if p.sink != nil {
					if err := p.sink.Consume(p.ctx, r); err != nil {
						p.incrementErrors()
					}
				}

				p.incrementRecordsOut()
			}
		}
	}
}

func (p *Pipeline) windowResultReader() {
	defer p.wg.Done()

	if p.window == nil {
		return
	}

	results := p.window.Results()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.stopCh:
			return
		case result, ok := <-results:
			if !ok {
				return
			}
			if result == nil {
				continue
			}

			p.incrementWindowsClosed()

			if p.sink != nil {
				if err := p.sink.ConsumeWindow(p.ctx, result); err != nil {
					p.incrementErrors()
				}
			}
		}
	}
}

func (p *Pipeline) checkpointLoop() {
	defer p.wg.Done()

	if p.cfg.CheckpointInterval <= 0 {
		return
	}

	p.checkpointTicker = time.NewTicker(p.cfg.CheckpointInterval)
	defer p.checkpointTicker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-p.checkpointTicker.C:
			_ = p.SaveCheckpoint()
		}
	}
}

func (p *Pipeline) SaveCheckpoint() error {
	if !p.cfg.EnableCheckpoint {
		return nil
	}

	opStates, err := p.operators.SaveStates()
	if err != nil {
		return fmt.Errorf("streamproc: save operator states: %w", err)
	}

	windowStates := make(map[string][]byte)
	if p.window != nil {
		ws, err := p.window.SaveState()
		if err != nil {
			return fmt.Errorf("streamproc: save window state: %w", err)
		}
		windowStates[p.window.Name()] = ws
	}

	cp := &Checkpoint{
		ID:             GenerateCheckpointID(),
		Timestamp:      time.Now(),
		SourceOffset:   atomic.LoadInt64(&p.sourceOffset),
		OperatorStates: opStates,
		WindowStates:   windowStates,
		Metadata:       make(map[string]interface{}),
	}

	if err := p.checkpoint.Save(p.ctx, cp); err != nil {
		return fmt.Errorf("streamproc: save checkpoint: %w", err)
	}

	p.incrementCheckpointsMade()
	return nil
}

func (p *Pipeline) RestoreFromCheckpoint(id string) error {
	p.statusMu.RLock()
	if p.status == PipelineStatusRunning {
		p.statusMu.RUnlock()
		return ErrPipelineRunning
	}
	p.statusMu.RUnlock()

	cp, err := p.checkpoint.Load(p.ctx, id)
	if err != nil {
		return fmt.Errorf("streamproc: load checkpoint: %w", err)
	}

	return p.restoreCheckpoint(cp)
}

func (p *Pipeline) RestoreFromLatestCheckpoint() error {
	p.statusMu.RLock()
	if p.status == PipelineStatusRunning {
		p.statusMu.RUnlock()
		return ErrPipelineRunning
	}
	p.statusMu.RUnlock()

	cp, err := p.checkpoint.Latest(p.ctx)
	if err != nil {
		return fmt.Errorf("streamproc: load latest checkpoint: %w", err)
	}

	return p.restoreCheckpoint(cp)
}

func (p *Pipeline) restoreCheckpoint(cp *Checkpoint) error {
	if cp == nil {
		return ErrInvalidCheckpoint
	}

	if err := p.operators.RestoreStates(cp.OperatorStates); err != nil {
		return fmt.Errorf("streamproc: restore operator states: %w", err)
	}

	if p.window != nil {
		if ws, ok := cp.WindowStates[p.window.Name()]; ok {
			if err := p.window.RestoreState(ws); err != nil {
				return fmt.Errorf("streamproc: restore window state: %w", err)
			}
		}
	}

	atomic.StoreInt64(&p.sourceOffset, cp.SourceOffset)

	p.processedMu.Lock()
	p.processedSeq = make(map[int64]bool)
	p.processedMu.Unlock()

	p.statsMu.Lock()
	p.stats.SourceOffset = cp.SourceOffset
	p.statsMu.Unlock()

	return nil
}

func (p *Pipeline) Pause() error {
	p.statusMu.Lock()
	if p.status != PipelineStatusRunning {
		p.statusMu.Unlock()
		return ErrPipelineNotRunning
	}
	p.status = PipelineStatusPaused
	p.statusMu.Unlock()

	return p.source.Pause()
}

func (p *Pipeline) Resume() error {
	p.statusMu.Lock()
	if p.status != PipelineStatusPaused {
		p.statusMu.Unlock()
		return fmt.Errorf("streamproc: pipeline is not paused")
	}
	p.status = PipelineStatusRunning
	p.statusMu.Unlock()

	return p.source.Resume()
}

func (p *Pipeline) Stop() error {
	p.stopOnce.Do(func() {
		close(p.stopCh)
	})

	p.statusMu.Lock()
	if p.status == PipelineStatusStopped || p.status == PipelineStatusCompleted {
		p.statusMu.Unlock()
		return nil
	}
	prevStatus := p.status
	p.status = PipelineStatusStopped
	p.statusMu.Unlock()

	if prevStatus == PipelineStatusRunning || prevStatus == PipelineStatusPaused {
		if err := p.source.Stop(); err != nil {
			return err
		}
	}

	if p.window != nil {
		p.window.Stop()
	}

	if p.cancel != nil {
		p.cancel()
	}

	p.wg.Wait()

	if p.sink != nil {
		_ = p.sink.Close(p.ctx)
	}

	if p.cfg.EnableCheckpoint && p.stats.CheckpointsMade == 0 {
		_ = p.SaveCheckpoint()
	}

	return nil
}

func (p *Pipeline) ListCheckpoints() ([]string, error) {
	return p.checkpoint.List(p.ctx)
}

func (p *Pipeline) DeleteCheckpoint(id string) error {
	return p.checkpoint.Delete(p.ctx, id)
}

func (p *Pipeline) ClearCheckpoints() error {
	return p.checkpoint.Clear(p.ctx)
}

func (p *Pipeline) SourceOffset() int64 {
	return atomic.LoadInt64(&p.sourceOffset)
}

type CollectSink struct {
	Records []*Record
	Results []*WindowResult
	mu      sync.Mutex
}

func NewCollectSink() *CollectSink {
	return &CollectSink{
		Records: make([]*Record, 0),
		Results: make([]*WindowResult, 0),
	}
}

func (s *CollectSink) Consume(_ context.Context, record *Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if record != nil {
		s.Records = append(s.Records, record)
	}
	return nil
}

func (s *CollectSink) ConsumeWindow(_ context.Context, result *WindowResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if result != nil {
		s.Results = append(s.Results, result)
	}
	return nil
}

func (s *CollectSink) Close(_ context.Context) error {
	return nil
}

func (s *CollectSink) GetRecords() []*Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*Record, len(s.Records))
	copy(result, s.Records)
	return result
}

func (s *CollectSink) GetResults() []*WindowResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*WindowResult, len(s.Results))
	copy(result, s.Results)
	return result
}

func (s *CollectSink) Count() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Records), len(s.Results)
}

func (s *CollectSink) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Records = make([]*Record, 0)
	s.Results = make([]*WindowResult, 0)
}
