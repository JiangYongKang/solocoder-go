package etlpipe

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrSourceNil                = errors.New("etlpipe: source cannot be nil")
	ErrTargetNil                = errors.New("etlpipe: target cannot be nil")
	ErrSourceNotFound           = errors.New("etlpipe: source type not found")
	ErrSourceAlreadyRegistered  = errors.New("etlpipe: source type already registered")
	ErrBatchSizeInvalid         = errors.New("etlpipe: batch size must be greater than 0")
	ErrPipelineRunning          = errors.New("etlpipe: pipeline is already running")
	ErrPipelineNotRunning       = errors.New("etlpipe: pipeline is not running")
	ErrInvalidCursor            = errors.New("etlpipe: invalid cursor value")
	ErrIncrementalNoField       = errors.New("etlpipe: incremental field must be set for incremental mode")
	ErrTransformNil             = errors.New("etlpipe: transformer cannot be nil")
	ErrWriteTimeout             = errors.New("etlpipe: write batch timeout")
	ErrNoDataSources            = errors.New("etlpipe: no data sources configured")
)

type ExtractMode int

const (
	ExtractModeFull ExtractMode = iota
	ExtractModeTimestamp
	ExtractModeID
)

type Cursor struct {
	Mode       ExtractMode
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

func (r *Record) SetField(name string, value interface{}) {
	if r.Data == nil {
		r.Data = make(map[string]interface{})
	}
	r.Data[name] = value
}

func (r *Record) DeleteField(name string) {
	if r.Data == nil {
		return
	}
	delete(r.Data, name)
}

func (r *Record) Clone() *Record {
	data := make(map[string]interface{}, len(r.Data))
	for k, v := range r.Data {
		data[k] = v
	}
	return &Record{
		ID:        r.ID,
		Data:      data,
		Timestamp: r.Timestamp,
		SeqID:     r.SeqID,
	}
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

type SourceFactory func(config map[string]interface{}) (Source, error)

type SourceRegistry struct {
	mu        sync.RWMutex
	factories map[string]SourceFactory
}

func NewSourceRegistry() *SourceRegistry {
	return &SourceRegistry{
		factories: make(map[string]SourceFactory),
	}
}

func (r *SourceRegistry) Register(sourceType string, factory SourceFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.factories[sourceType]; exists {
		return ErrSourceAlreadyRegistered
	}
	r.factories[sourceType] = factory
	return nil
}

func (r *SourceRegistry) Unregister(sourceType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.factories, sourceType)
}

func (r *SourceRegistry) Create(sourceType string, config map[string]interface{}) (Source, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, exists := r.factories[sourceType]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrSourceNotFound, sourceType)
	}
	return factory(config)
}

func (r *SourceRegistry) Has(sourceType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.factories[sourceType]
	return exists
}

func (r *SourceRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	types := make([]string, 0, len(r.factories))
	for t := range r.factories {
		types = append(types, t)
	}
	return types
}

type TransformError struct {
	StageName   string
	StageIndex  int
	Record      *Record
	Err         error
	Timestamp   time.Time
}

func (e *TransformError) Error() string {
	return fmt.Sprintf("transform error at stage '%s' (index %d): %v", e.StageName, e.StageIndex, e.Err)
}

func (e *TransformError) Unwrap() error {
	return e.Err
}

type WriteError struct {
	Record    *Record
	Err       error
	Timestamp time.Time
}

func (e *WriteError) Error() string {
	return fmt.Sprintf("write error for record '%s': %v", e.Record.ID, e.Err)
}

func (e *WriteError) Unwrap() error {
	return e.Err
}

type TransformType int

const (
	TransformTypeFieldMap TransformType = iota
	TransformTypeTypeConvert
	TransformTypeValueReplace
	TransformTypeFieldFilter
	TransformTypeFieldCalculate
)

type FieldMapping struct {
	Source string
	Target string
}

type TypeConversion struct {
	Field    string
	TargetType string
}

type ValueReplacement struct {
	Field   string
	Old     interface{}
	New     interface{}
}

type FieldFilter struct {
	KeepFields   []string
	RemoveFields []string
}

type FieldCalculation struct {
	TargetField string
	Calculator  func(data map[string]interface{}) (interface{}, error)
}

type TransformRule struct {
	Name           string
	Type           TransformType
	FieldMappings  []FieldMapping
	TypeConversions []TypeConversion
	Replacements   []ValueReplacement
	Filter         FieldFilter
	Calculation    *FieldCalculation
}

type Transformer interface {
	Name() string
	Transform(record *Record) (*Record, error)
}

type BaseTransformer struct {
	name string
	rules []TransformRule
}

func NewBaseTransformer(name string, rules []TransformRule) *BaseTransformer {
	return &BaseTransformer{
		name:  name,
		rules: rules,
	}
}

func (t *BaseTransformer) Name() string {
	return t.name
}

func (t *BaseTransformer) Transform(record *Record) (*Record, error) {
	if record == nil {
		return nil, errors.New("etlpipe: cannot transform nil record")
	}
	result := record.Clone()
	for i, rule := range t.rules {
		if err := t.applyRule(result, rule); err != nil {
			return nil, fmt.Errorf("rule '%s' (index %d): %w", rule.Name, i, err)
		}
	}
	return result, nil
}

func (t *BaseTransformer) applyRule(record *Record, rule TransformRule) error {
	switch rule.Type {
	case TransformTypeFieldMap:
		return t.applyFieldMap(record, rule.FieldMappings)
	case TransformTypeTypeConvert:
		return t.applyTypeConvert(record, rule.TypeConversions)
	case TransformTypeValueReplace:
		return t.applyValueReplace(record, rule.Replacements)
	case TransformTypeFieldFilter:
		return t.applyFieldFilter(record, rule.Filter)
	case TransformTypeFieldCalculate:
		return t.applyFieldCalculate(record, rule.Calculation)
	default:
		return fmt.Errorf("unknown transform type: %d", rule.Type)
	}
}

func (t *BaseTransformer) applyFieldMap(record *Record, mappings []FieldMapping) error {
	for _, m := range mappings {
		v, ok := record.GetField(m.Source)
		if ok {
			record.SetField(m.Target, v)
			if m.Source != m.Target {
				record.DeleteField(m.Source)
			}
		}
	}
	return nil
}

func (t *BaseTransformer) applyTypeConvert(record *Record, conversions []TypeConversion) error {
	for _, c := range conversions {
		v, ok := record.GetField(c.Field)
		if !ok {
			continue
		}
		converted, err := convertType(v, c.TargetType)
		if err != nil {
			return fmt.Errorf("field '%s' type conversion failed: %w", c.Field, err)
		}
		record.SetField(c.Field, converted)
	}
	return nil
}

func convertType(value interface{}, targetType string) (interface{}, error) {
	switch targetType {
	case "string":
		return fmt.Sprintf("%v", value), nil
	case "int":
		switch v := value.(type) {
		case int:
			return v, nil
		case int64:
			return int(v), nil
		case float64:
			return int(v), nil
		case string:
			i, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("cannot convert '%s' to int: %w", v, err)
			}
			return i, nil
		default:
			return nil, fmt.Errorf("cannot convert %T to int", value)
		}
	case "int64":
		switch v := value.(type) {
		case int64:
			return v, nil
		case int:
			return int64(v), nil
		case float64:
			return int64(v), nil
		case string:
			i, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot convert '%s' to int64: %w", v, err)
			}
			return i, nil
		default:
			return nil, fmt.Errorf("cannot convert %T to int64", value)
		}
	case "float64":
		switch v := value.(type) {
		case float64:
			return v, nil
		case int:
			return float64(v), nil
		case int64:
			return float64(v), nil
		case string:
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot convert '%s' to float64: %w", v, err)
			}
			return f, nil
		default:
			return nil, fmt.Errorf("cannot convert %T to float64", value)
		}
	case "bool":
		switch v := value.(type) {
		case bool:
			return v, nil
		case string:
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, fmt.Errorf("cannot convert '%s' to bool: %w", v, err)
			}
			return b, nil
		case int:
			return v != 0, nil
		default:
			return nil, fmt.Errorf("cannot convert %T to bool", value)
		}
	case "time":
		switch v := value.(type) {
		case time.Time:
			return v, nil
		case string:
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return nil, fmt.Errorf("cannot convert '%s' to time: %w", v, err)
			}
			return t, nil
		default:
			return nil, fmt.Errorf("cannot convert %T to time", value)
		}
	default:
		return nil, fmt.Errorf("unsupported target type: %s", targetType)
	}
}

func (t *BaseTransformer) applyValueReplace(record *Record, replacements []ValueReplacement) error {
	for _, r := range replacements {
		v, ok := record.GetField(r.Field)
		if !ok {
			continue
		}
		if v == r.Old || fmt.Sprintf("%v", v) == fmt.Sprintf("%v", r.Old) {
			record.SetField(r.Field, r.New)
		}
	}
	return nil
}

func (t *BaseTransformer) applyFieldFilter(record *Record, filter FieldFilter) error {
	if len(filter.KeepFields) > 0 {
		keepSet := make(map[string]struct{}, len(filter.KeepFields))
		for _, f := range filter.KeepFields {
			keepSet[f] = struct{}{}
		}
		for field := range record.Data {
			if _, keep := keepSet[field]; !keep {
				record.DeleteField(field)
			}
		}
	}
	for _, f := range filter.RemoveFields {
		record.DeleteField(f)
	}
	return nil
}

func (t *BaseTransformer) applyFieldCalculate(record *Record, calc *FieldCalculation) error {
	if calc == nil || calc.Calculator == nil {
		return errors.New("etlpipe: calculator function is nil")
	}
	result, err := calc.Calculator(record.Data)
	if err != nil {
		return fmt.Errorf("calculation failed: %w", err)
	}
	record.SetField(calc.TargetField, result)
	return nil
}

type TransformChain struct {
	transformers []Transformer
	mu           sync.RWMutex
}

func NewTransformChain() *TransformChain {
	return &TransformChain{
		transformers: make([]Transformer, 0),
	}
}

func (c *TransformChain) Add(t Transformer) error {
	if t == nil {
		return ErrTransformNil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.transformers = append(c.transformers, t)
	return nil
}

func (c *TransformChain) Insert(index int, t Transformer) error {
	if t == nil {
		return ErrTransformNil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if index < 0 || index > len(c.transformers) {
		return fmt.Errorf("etlpipe: transform index out of range: %d", index)
	}
	c.transformers = append(c.transformers[:index], append([]Transformer{t}, c.transformers[index:]...)...)
	return nil
}

func (c *TransformChain) Remove(index int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if index < 0 || index >= len(c.transformers) {
		return fmt.Errorf("etlpipe: transform index out of range: %d", index)
	}
	c.transformers = append(c.transformers[:index], c.transformers[index+1:]...)
	return nil
}

func (c *TransformChain) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.transformers)
}

func (c *TransformChain) List() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	names := make([]string, len(c.transformers))
	for i, t := range c.transformers {
		names[i] = t.Name()
	}
	return names
}

func (c *TransformChain) Process(record *Record) (*Record, error) {
	c.mu.RLock()
	transformers := make([]Transformer, len(c.transformers))
	copy(transformers, c.transformers)
	c.mu.RUnlock()

	current := record
	for i, t := range transformers {
		result, err := t.Transform(current)
		if err != nil {
			return nil, &TransformError{
				StageName:  t.Name(),
				StageIndex: i,
				Record:     record,
				Err:        err,
				Timestamp:  time.Now(),
			}
		}
		current = result
	}
	return current, nil
}

type ErrorQueue struct {
	mu              sync.RWMutex
	transformErrors []*TransformError
	writeErrors     []*WriteError
	maxSize         int
}

func NewErrorQueue(maxSize int) *ErrorQueue {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &ErrorQueue{
		transformErrors: make([]*TransformError, 0),
		writeErrors:     make([]*WriteError, 0),
		maxSize:         maxSize,
	}
}

func (q *ErrorQueue) AddTransformError(err *TransformError) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.transformErrors) >= q.maxSize {
		q.transformErrors = q.transformErrors[1:]
	}
	q.transformErrors = append(q.transformErrors, err)
}

func (q *ErrorQueue) AddWriteError(err *WriteError) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.writeErrors) >= q.maxSize {
		q.writeErrors = q.writeErrors[1:]
	}
	q.writeErrors = append(q.writeErrors, err)
}

func (q *ErrorQueue) TransformErrorCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.transformErrors)
}

func (q *ErrorQueue) WriteErrorCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.writeErrors)
}

func (q *ErrorQueue) TotalErrorCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.transformErrors) + len(q.writeErrors)
}

func (q *ErrorQueue) GetTransformErrors() []*TransformError {
	q.mu.RLock()
	defer q.mu.RUnlock()
	result := make([]*TransformError, len(q.transformErrors))
	copy(result, q.transformErrors)
	return result
}

func (q *ErrorQueue) GetWriteErrors() []*WriteError {
	q.mu.RLock()
	defer q.mu.RUnlock()
	result := make([]*WriteError, len(q.writeErrors))
	copy(result, q.writeErrors)
	return result
}

func (q *ErrorQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.transformErrors = q.transformErrors[:0]
	q.writeErrors = q.writeErrors[:0]
}

type Target interface {
	WriteBatch(ctx context.Context, records []*Record) ([]int, error)
	Close(ctx context.Context) error
}

type MemoryTarget struct {
	mu      sync.RWMutex
	records []*Record
	failIDs map[string]bool
}

func NewMemoryTarget() *MemoryTarget {
	return &MemoryTarget{
		records: make([]*Record, 0),
		failIDs: make(map[string]bool),
	}
}

func (t *MemoryTarget) SetFailRecord(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failIDs[id] = true
}

func (t *MemoryTarget) WriteBatch(_ context.Context, records []*Record) ([]int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	failedIndexes := make([]int, 0)
	for i, r := range records {
		if t.failIDs[r.ID] {
			failedIndexes = append(failedIndexes, i)
			continue
		}
		data := make(map[string]interface{}, len(r.Data))
		for k, v := range r.Data {
			data[k] = v
		}
		t.records = append(t.records, &Record{
			ID:        r.ID,
			Data:      data,
			Timestamp: r.Timestamp,
			SeqID:     r.SeqID,
		})
	}
	return failedIndexes, nil
}

func (t *MemoryTarget) Close(_ context.Context) error {
	return nil
}

func (t *MemoryTarget) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.records)
}

func (t *MemoryTarget) GetAll() []*Record {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]*Record, len(t.records))
	for i, r := range t.records {
		data := make(map[string]interface{}, len(r.Data))
		for k, v := range r.Data {
			data[k] = v
		}
		result[i] = &Record{
			ID:        r.ID,
			Data:      data,
			Timestamp: r.Timestamp,
			SeqID:     r.SeqID,
		}
	}
	return result
}

func (t *MemoryTarget) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = t.records[:0]
}

type Config struct {
	BatchSize            int
	WriteTimeout         time.Duration
	ExtractMode          ExtractMode
	IncrementalField     string
	MaxErrorQueueSize    int
}

func DefaultConfig() Config {
	return Config{
		BatchSize:         100,
		WriteTimeout:      30 * time.Second,
		ExtractMode:       ExtractModeFull,
		MaxErrorQueueSize: 10000,
	}
}

type PipelineStatus int

const (
	PipelineStatusIdle PipelineStatus = iota
	PipelineStatusRunning
	PipelineStatusCompleted
	PipelineStatusFailed
	PipelineStatusStopped
)

func (s PipelineStatus) String() string {
	switch s {
	case PipelineStatusIdle:
		return "idle"
	case PipelineStatusRunning:
		return "running"
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

type PipelineStats struct {
	ExtractedCount     int64
	TransformedCount   int64
	WrittenCount       int64
	TransformErrorCount int64
	WriteErrorCount    int64
	BatchCount         int64
	ElapsedTime        time.Duration
	StartTime          time.Time
}

type Pipeline struct {
	cfg          Config
	source       Source
	target       Target
	chain        *TransformChain
	errorQueue   *ErrorQueue

	status       PipelineStatus
	mu           sync.Mutex

	stats        PipelineStats
	startTime    time.Time

	cursor       *Cursor
	stopCh       chan struct{}
	stopOnce     sync.Once
}

func NewPipeline(cfg Config, source Source, target Target) (*Pipeline, error) {
	if source == nil {
		return nil, ErrSourceNil
	}
	if target == nil {
		return nil, ErrTargetNil
	}
	if cfg.BatchSize <= 0 {
		return nil, ErrBatchSizeInvalid
	}
	if cfg.ExtractMode != ExtractModeFull && cfg.IncrementalField == "" {
		return nil, ErrIncrementalNoField
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = 30 * time.Second
	}
	if cfg.MaxErrorQueueSize <= 0 {
		cfg.MaxErrorQueueSize = 10000
	}

	return &Pipeline{
		cfg:        cfg,
		source:     source,
		target:     target,
		chain:      NewTransformChain(),
		errorQueue: NewErrorQueue(cfg.MaxErrorQueueSize),
		status:     PipelineStatusIdle,
		cursor: &Cursor{
			Mode:       cfg.ExtractMode,
			LastOffset: 0,
			UpdateTime: time.Now(),
		},
		stopCh: make(chan struct{}),
	}, nil
}

func (p *Pipeline) AddTransformer(t Transformer) error {
	return p.chain.Add(t)
}

func (p *Pipeline) InsertTransformer(index int, t Transformer) error {
	return p.chain.Insert(index, t)
}

func (p *Pipeline) RemoveTransformer(index int) error {
	return p.chain.Remove(index)
}

func (p *Pipeline) ListTransformers() []string {
	return p.chain.List()
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

func (p *Pipeline) Stats() PipelineStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	stats := p.stats
	if p.status == PipelineStatusRunning {
		stats.ElapsedTime = time.Since(p.startTime)
	}
	return stats
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

func (p *Pipeline) GetErrorQueue() *ErrorQueue {
	return p.errorQueue
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
	p.startTime = time.Now()
	p.stats = PipelineStats{StartTime: p.startTime}
	p.mu.Unlock()

	defer func() {
		_ = p.source.Close(ctx)
		_ = p.target.Close(ctx)
	}()

	err := p.runLoop(ctx)

	p.mu.Lock()
	p.stats.ElapsedTime = time.Since(p.startTime)
	p.mu.Unlock()

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
			return fmt.Errorf("etlpipe: fetch batch failed: %w", err)
		}
		if batch == nil || batch.Size() == 0 {
			return nil
		}

		atomic.AddInt64(&p.stats.ExtractedCount, int64(batch.Size()))

		transformedRecords := make([]*Record, 0, batch.Size())
		for _, rec := range batch.Records {
			transformed, err := p.chain.Process(rec)
			if err != nil {
				if te, ok := err.(*TransformError); ok {
					p.errorQueue.AddTransformError(te)
				} else {
					p.errorQueue.AddTransformError(&TransformError{
						StageName:  "unknown",
						StageIndex: -1,
						Record:     rec,
						Err:        err,
						Timestamp:  time.Now(),
					})
				}
				atomic.AddInt64(&p.stats.TransformErrorCount, 1)
				continue
			}
			transformedRecords = append(transformedRecords, transformed)
		}

		atomic.AddInt64(&p.stats.TransformedCount, int64(len(transformedRecords)))

		if len(transformedRecords) > 0 {
			if err := p.writeBatch(ctx, transformedRecords); err != nil {
				return fmt.Errorf("etlpipe: write batch failed: %w", err)
			}
		}

		p.updateCursor(batch)
		atomic.AddInt64(&p.stats.BatchCount, 1)
	}
}

func (p *Pipeline) fetchNextBatch(ctx context.Context) (*Batch, error) {
	done := make(chan struct{})
	var batch *Batch
	var err error

	go func() {
		defer close(done)
		batch, err = p.source.Fetch(ctx, p.cursor, p.cfg.BatchSize)
	}()

	select {
	case <-done:
		return batch, err
	case <-p.stopCh:
		<-done
		return nil, nil
	}
}

func (p *Pipeline) writeBatch(ctx context.Context, records []*Record) error {
	writeCtx, cancel := context.WithTimeout(ctx, p.cfg.WriteTimeout)
	defer cancel()

	done := make(chan struct{})
	var failedIndexes []int
	var writeErr error

	go func() {
		defer close(done)
		failedIndexes, writeErr = p.target.WriteBatch(writeCtx, records)
	}()

	select {
	case <-done:
	case <-p.stopCh:
		<-done
		return nil
	case <-writeCtx.Done():
		<-done
		if writeErr == nil {
			writeErr = ErrWriteTimeout
		}
	}

	if writeErr != nil {
		for i, r := range records {
			_ = i
			p.errorQueue.AddWriteError(&WriteError{
				Record:    r,
				Err:       writeErr,
				Timestamp: time.Now(),
			})
			atomic.AddInt64(&p.stats.WriteErrorCount, 1)
		}
		return nil
	}

	failedSet := make(map[int]struct{})
	for _, idx := range failedIndexes {
		failedSet[idx] = struct{}{}
		if idx >= 0 && idx < len(records) {
			p.errorQueue.AddWriteError(&WriteError{
				Record:    records[idx],
				Err:       fmt.Errorf("single record write failed"),
				Timestamp: time.Now(),
			})
			atomic.AddInt64(&p.stats.WriteErrorCount, 1)
		}
	}

	writtenCount := int64(len(records) - len(failedSet))
	atomic.AddInt64(&p.stats.WrittenCount, writtenCount)

	return nil
}

func (p *Pipeline) updateCursor(batch *Batch) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch p.cfg.ExtractMode {
	case ExtractModeTimestamp:
		if !batch.EndTs.IsZero() {
			p.cursor.LastValue = batch.EndTs
		}
	case ExtractModeID:
		if batch.LastSeq > 0 {
			p.cursor.LastValue = batch.LastSeq
		}
	}
	p.cursor.LastOffset += int64(batch.Size())
	p.cursor.UpdateTime = time.Now()
}

type MemorySource struct {
	mu             sync.RWMutex
	records        []*Record
	fetchDelay     time.Duration
	fetchError     error
	fetchErrorCount int
	fetchErrorN    int
	currentOffset  int
	timestampField string
	idField        string
}

func NewMemorySource(records []*Record) *MemorySource {
	return &MemorySource{
		records:       records,
		currentOffset: 0,
	}
}

func (s *MemorySource) SetFetchDelay(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fetchDelay = d
}

func (s *MemorySource) SetFetchError(err error, times int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fetchError = err
	s.fetchErrorN = times
	s.fetchErrorCount = 0
}

func (s *MemorySource) SetIncrementalFields(timestampField, idField string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timestampField = timestampField
	s.idField = idField
}

func (s *MemorySource) Fetch(_ context.Context, cursor *Cursor, batchSize int) (*Batch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.fetchDelay > 0 {
		time.Sleep(s.fetchDelay)
	}

	if s.fetchError != nil && s.fetchErrorCount < s.fetchErrorN {
		s.fetchErrorCount++
		return nil, s.fetchError
	}

	offset := int(cursor.LastOffset)
	if offset > s.currentOffset {
		s.currentOffset = offset
	}
	if s.currentOffset >= len(s.records) {
		return &Batch{Records: []*Record{}}, nil
	}

	end := s.currentOffset + batchSize
	if end > len(s.records) {
		end = len(s.records)
	}

	records := s.records[s.currentOffset:end]
	batchRecords := make([]*Record, len(records))
	for i, r := range records {
		data := make(map[string]interface{}, len(r.Data))
		for k, v := range r.Data {
			data[k] = v
		}
		batchRecords[i] = &Record{
			ID:        r.ID,
			Data:      data,
			Timestamp: r.Timestamp,
			SeqID:     r.SeqID,
		}
	}

	batch := &Batch{
		Records:  batchRecords,
		FirstSeq: records[0].SeqID,
		LastSeq:  records[len(records)-1].SeqID,
		StartTs:  records[0].Timestamp,
		EndTs:    records[len(records)-1].Timestamp,
	}

	s.currentOffset = end
	return batch, nil
}

func (s *MemorySource) Count(_ context.Context, _ *Cursor) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int64(len(s.records)), nil
}

func (s *MemorySource) Close(_ context.Context) error {
	return nil
}

func (s *MemorySource) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentOffset = 0
	s.fetchErrorCount = 0
}

func NewMemorySourceFactory() SourceFactory {
	return func(config map[string]interface{}) (Source, error) {
		records, _ := config["records"].([]*Record)
		source := NewMemorySource(records)
		if tf, ok := config["timestamp_field"].(string); ok {
			if idf, ok2 := config["id_field"].(string); ok2 {
				source.SetIncrementalFields(tf, idf)
			}
		}
		return source, nil
	}
}
