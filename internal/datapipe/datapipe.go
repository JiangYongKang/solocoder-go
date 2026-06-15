package datapipe

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrSourceNil          = errors.New("datapipe: source cannot be nil")
	ErrTargetNil          = errors.New("datapipe: target cannot be nil")
	ErrBatchSizeInvalid   = errors.New("datapipe: batch size must be greater than 0")
	ErrPipelineStopped    = errors.New("datapipe: pipeline is stopped")
	ErrPipelineRunning    = errors.New("datapipe: pipeline is already running")
	ErrInvalidCursor      = errors.New("datapipe: invalid cursor value")
	ErrProgressInterval   = errors.New("datapipe: progress interval must be non-negative")
	ErrIncrementalNoField = errors.New("datapipe: incremental field must be set for incremental mode")
)

type IncrementalMode int

const (
	IncrementalModeFull IncrementalMode = iota
	IncrementalModeTimestamp
	IncrementalModeID
)

type Cursor struct {
	Mode       IncrementalMode
	LastValue  interface{}
	LastOffset int64
	UpdateTime time.Time
}

type Record struct {
	ID        string
	Data      map[string]interface{}
	Timestamp time.Time
	SeqID     int64
}

func (r *Record) GetField(name string) (interface{}, bool) {
	if r.Data == nil {
		return nil, false
	}
	v, ok := r.Data[name]
	return v, ok
}

type Batch struct {
	ID       int64
	Records  []*Record
	FirstSeq int64
	LastSeq  int64
	StartTs  time.Time
	EndTs    time.Time
}

func (b *Batch) Size() int {
	return len(b.Records)
}

type Source interface {
	Fetch(ctx context.Context, cursor *Cursor, batchSize int) (*Batch, error)
	Count(ctx context.Context, cursor *Cursor) (int64, error)
	Close(ctx context.Context) error
}

type Target interface {
	Write(ctx context.Context, batch *Batch) error
	Close(ctx context.Context) error
}

type CheckpointStore interface {
	Save(ctx context.Context, cursor *Cursor) error
	Load(ctx context.Context) (*Cursor, error)
	Clear(ctx context.Context) error
}

type ProgressInfo struct {
	Processed   int64
	Total       int64
	Batches     int64
	RatePerSec  float64
	Percent     float64
	Elapsed     time.Duration
	Remaining   time.Duration
	CurrentBatch int64
}

type ProgressCallback func(info ProgressInfo)

type Config struct {
	BatchSize            int
	IncrementalMode      IncrementalMode
	IncrementalField     string
	EnableCheckpoint     bool
	ProgressInterval     time.Duration
	TimeoutPerBatch      time.Duration
	MaxRetryPerBatch     int
	RetryBackoff         time.Duration
}

func DefaultConfig() Config {
	return Config{
		BatchSize:        100,
		IncrementalMode:  IncrementalModeFull,
		EnableCheckpoint: true,
		ProgressInterval: 500 * time.Millisecond,
		TimeoutPerBatch:  30 * time.Second,
		MaxRetryPerBatch: 3,
		RetryBackoff:     100 * time.Millisecond,
	}
}

type PipelineStatus int

const (
	PipelineStatusIdle PipelineStatus = iota
	PipelineStatusRunning
	PipelineStatusPaused
	PipelineStatusCompleted
	PipelineStatusFailed
	PipelineStatusStopped
)

type Pipeline struct {
	cfg             Config
	source          Source
	target          Target
	checkpoint      CheckpointStore
	progressCb      ProgressCallback

	status          PipelineStatus
	mu              sync.Mutex

	processed       int64
	total           int64
	batches         int64
	startTime       time.Time

	cursor          *Cursor
	stopCh          chan struct{}
	stopOnce        sync.Once
}

type memoryCheckpointStore struct {
	mu     sync.RWMutex
	cursor *Cursor
}

func NewMemoryCheckpointStore() CheckpointStore {
	return &memoryCheckpointStore{}
}

func (m *memoryCheckpointStore) Save(_ context.Context, cursor *Cursor) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cursor == nil {
		return ErrInvalidCursor
	}
	m.cursor = &Cursor{
		Mode:       cursor.Mode,
		LastValue:  cursor.LastValue,
		LastOffset: cursor.LastOffset,
		UpdateTime: cursor.UpdateTime,
	}
	return nil
}

func (m *memoryCheckpointStore) Load(_ context.Context) (*Cursor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cursor == nil {
		return nil, nil
	}
	return &Cursor{
		Mode:       m.cursor.Mode,
		LastValue:  m.cursor.LastValue,
		LastOffset: m.cursor.LastOffset,
		UpdateTime: m.cursor.UpdateTime,
	}, nil
}

func (m *memoryCheckpointStore) Clear(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cursor = nil
	return nil
}

func NewPipeline(cfg Config, source Source, target Target, checkpoint CheckpointStore) (*Pipeline, error) {
	if source == nil {
		return nil, ErrSourceNil
	}
	if target == nil {
		return nil, ErrTargetNil
	}
	if cfg.BatchSize <= 0 {
		return nil, ErrBatchSizeInvalid
	}
	if cfg.IncrementalMode != IncrementalModeFull && cfg.IncrementalField == "" {
		return nil, ErrIncrementalNoField
	}
	if cfg.ProgressInterval < 0 {
		return nil, ErrProgressInterval
	}
	if cfg.MaxRetryPerBatch < 0 {
		cfg.MaxRetryPerBatch = 0
	}
	if cfg.RetryBackoff < 0 {
		cfg.RetryBackoff = 0
	}
	if cfg.TimeoutPerBatch <= 0 {
		cfg.TimeoutPerBatch = 30 * time.Second
	}
	if checkpoint == nil {
		checkpoint = NewMemoryCheckpointStore()
	}
	return &Pipeline{
		cfg:        cfg,
		source:     source,
		target:     target,
		checkpoint: checkpoint,
		status:     PipelineStatusIdle,
		stopCh:     make(chan struct{}),
	}, nil
}

func (p *Pipeline) SetProgressCallback(cb ProgressCallback) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.progressCb = cb
}

func (p *Pipeline) Status() PipelineStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

func (p *Pipeline) setStatus(s PipelineStatus) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = s
}

func (p *Pipeline) GetProcessed() int64 {
	return atomic.LoadInt64(&p.processed)
}

func (p *Pipeline) GetTotal() int64 {
	return atomic.LoadInt64(&p.total)
}

func (p *Pipeline) GetBatches() int64 {
	return atomic.LoadInt64(&p.batches)
}

func (p *Pipeline) GetCursor() *Cursor {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cursor == nil {
		return nil
	}
	return &Cursor{
		Mode:       p.cursor.Mode,
		LastValue:  p.cursor.LastValue,
		LastOffset: p.cursor.LastOffset,
		UpdateTime: p.cursor.UpdateTime,
	}
}

func (p *Pipeline) Stop() {
	p.stopOnce.Do(func() {
		close(p.stopCh)
	})
}

func (p *Pipeline) Run(ctx context.Context) error {
	p.mu.Lock()
	if p.status == PipelineStatusRunning {
		p.mu.Unlock()
		return ErrPipelineRunning
	}
	p.status = PipelineStatusRunning
	p.stopCh = make(chan struct{})
	p.stopOnce = sync.Once{}
	p.mu.Unlock()

	defer func() {
		_ = p.source.Close(ctx)
		_ = p.target.Close(ctx)
	}()

	if err := p.initCursor(ctx); err != nil {
		p.setStatus(PipelineStatusFailed)
		return err
	}

	total, err := p.source.Count(ctx, p.cursor)
	if err == nil {
		atomic.StoreInt64(&p.total, total)
	}

	p.startTime = time.Now()
	progressDone := make(chan struct{})
	if p.progressCb != nil && p.cfg.ProgressInterval > 0 {
		go p.progressReporter(progressDone)
	}

	err = p.runLoop(ctx)

	close(progressDone)
	p.reportProgress()

	if err != nil {
		p.setStatus(PipelineStatusFailed)
		return err
	}

	select {
	case <-p.stopCh:
		p.setStatus(PipelineStatusStopped)
	default:
		p.setStatus(PipelineStatusCompleted)
	}
	return nil
}

func (p *Pipeline) initCursor(ctx context.Context) error {
	if !p.cfg.EnableCheckpoint {
		p.cursor = &Cursor{
			Mode:       p.cfg.IncrementalMode,
			LastOffset: 0,
			UpdateTime: time.Now(),
		}
		return nil
	}

	loaded, err := p.checkpoint.Load(ctx)
	if err != nil {
		return fmt.Errorf("datapipe: failed to load checkpoint: %w", err)
	}
	if loaded != nil {
		p.cursor = loaded
		return nil
	}
	p.cursor = &Cursor{
		Mode:       p.cfg.IncrementalMode,
		LastOffset: 0,
		UpdateTime: time.Now(),
	}
	return nil
}

func (p *Pipeline) progressReporter(done chan struct{}) {
	if p.cfg.ProgressInterval <= 0 {
		return
	}
	ticker := time.NewTicker(p.cfg.ProgressInterval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			p.reportProgress()
		}
	}
}

func (p *Pipeline) reportProgress() {
	p.mu.Lock()
	cb := p.progressCb
	p.mu.Unlock()
	if cb == nil {
		return
	}

	processed := atomic.LoadInt64(&p.processed)
	total := atomic.LoadInt64(&p.total)
	batches := atomic.LoadInt64(&p.batches)
	elapsed := time.Since(p.startTime)

	var rate float64
	var remaining time.Duration
	var percent float64
	if elapsed > 0 {
		rate = float64(processed) / elapsed.Seconds()
	}
	if total > 0 {
		percent = float64(processed) / float64(total) * 100
		if rate > 0 {
			remaining = time.Duration(float64(total-processed)/rate) * time.Second
		}
	}

	info := ProgressInfo{
		Processed:    processed,
		Total:        total,
		Batches:      batches,
		RatePerSec:   rate,
		Percent:      percent,
		Elapsed:      elapsed,
		Remaining:    remaining,
		CurrentBatch: batches,
	}
	cb(info)
}

func (p *Pipeline) runLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.stopCh:
			return nil
		default:
		}

		batch, err := p.fetchNextBatch(ctx)
		if err != nil {
			return fmt.Errorf("datapipe: fetch batch failed: %w", err)
		}
		if batch == nil || batch.Size() == 0 {
			return nil
		}

		if err := p.writeBatchWithRetry(ctx, batch); err != nil {
			if errors.Is(err, ErrPipelineStopped) {
				return nil
			}
			return fmt.Errorf("datapipe: write batch failed: %w", err)
		}

		if err := p.updateCursor(ctx, batch); err != nil {
			return fmt.Errorf("datapipe: update checkpoint failed: %w", err)
		}

		atomic.AddInt64(&p.processed, int64(batch.Size()))
		atomic.AddInt64(&p.batches, 1)
	}
}

func (p *Pipeline) fetchNextBatch(ctx context.Context) (*Batch, error) {
	batchCtx, cancel := context.WithTimeout(ctx, p.cfg.TimeoutPerBatch)
	defer cancel()

	done := make(chan struct{})
	var batch *Batch
	var err error

	go func() {
		batch, err = p.source.Fetch(batchCtx, p.cursor, p.cfg.BatchSize)
		close(done)
	}()

	select {
	case <-done:
		if batch != nil {
			batch.ID = atomic.LoadInt64(&p.batches) + 1
		}
		return batch, err
	case <-p.stopCh:
		cancel()
		<-done
		return nil, nil
	}
}

func (p *Pipeline) writeBatchWithRetry(ctx context.Context, batch *Batch) error {
	var lastErr error
	for attempt := 0; attempt <= p.cfg.MaxRetryPerBatch; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-p.stopCh:
				return ErrPipelineStopped
			case <-time.After(p.cfg.RetryBackoff * time.Duration(1<<uint(attempt-1))):
			}
		}

		writeCtx, cancel := context.WithTimeout(ctx, p.cfg.TimeoutPerBatch)
		done := make(chan error, 1)

		go func() {
			done <- p.target.Write(writeCtx, batch)
		}()

		select {
		case err := <-done:
			cancel()
			if err == nil {
				return nil
			}
			lastErr = err
		case <-p.stopCh:
			cancel()
			<-done
			return ErrPipelineStopped
		case <-ctx.Done():
			cancel()
			<-done
			return ctx.Err()
		}
	}
	return lastErr
}

func (p *Pipeline) updateCursor(ctx context.Context, batch *Batch) error {
	p.mu.Lock()
	if p.cfg.IncrementalMode == IncrementalModeTimestamp {
		p.cursor.LastValue = batch.EndTs
	} else if p.cfg.IncrementalMode == IncrementalModeID {
		p.cursor.LastValue = batch.LastSeq
	}
	p.cursor.LastOffset += int64(batch.Size())
	p.cursor.UpdateTime = time.Now()
	cursorCopy := &Cursor{
		Mode:       p.cursor.Mode,
		LastValue:  p.cursor.LastValue,
		LastOffset: p.cursor.LastOffset,
		UpdateTime: p.cursor.UpdateTime,
	}
	p.mu.Unlock()

	if p.cfg.EnableCheckpoint {
		return p.checkpoint.Save(ctx, cursorCopy)
	}
	return nil
}

func (s PipelineStatus) String() string {
	switch s {
	case PipelineStatusIdle:
		return "idle"
	case PipelineStatusRunning:
		return "running"
	case PipelineStatusPaused:
		return "paused"
	case PipelineStatusCompleted:
		return "completed"
	case PipelineStatusFailed:
		return "failed"
	case PipelineStatusStopped:
		return "stopped"
	default:
		return "unknown"
	}
}
