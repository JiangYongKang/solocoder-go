package streamproc

import (
	"context"
	"errors"
	"time"
)

var (
	ErrSourceNil             = errors.New("streamproc: source cannot be nil")
	ErrSourceAlreadyStarted  = errors.New("streamproc: source already started")
	ErrSourceNotStarted      = errors.New("streamproc: source not started")
	ErrOperatorNil           = errors.New("streamproc: operator cannot be nil")
	ErrPipelineRunning       = errors.New("streamproc: pipeline is already running")
	ErrPipelineNotRunning    = errors.New("streamproc: pipeline is not running")
	ErrPipelineStopped       = errors.New("streamproc: pipeline is stopped")
	ErrInvalidWindowSize     = errors.New("streamproc: window size must be greater than 0")
	ErrInvalidWindowSlide    = errors.New("streamproc: window slide must be greater than 0")
	ErrInvalidBackpressureThreshold = errors.New("streamproc: backpressure threshold must be greater than 0")
	ErrCheckpointNotFound    = errors.New("streamproc: checkpoint not found")
	ErrInvalidCheckpoint     = errors.New("streamproc: invalid checkpoint")
	ErrNoData                = errors.New("streamproc: no data available")
)

type Record struct {
	ID        string
	Data      interface{}
	Timestamp time.Time
	SeqID     int64
	Metadata  map[string]interface{}
}

func NewRecord(data interface{}) *Record {
	return &Record{
		Data:      data,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
}

func (r *Record) Clone() *Record {
	meta := make(map[string]interface{}, len(r.Metadata))
	for k, v := range r.Metadata {
		meta[k] = v
	}
	return &Record{
		ID:        r.ID,
		Data:      r.Data,
		Timestamp: r.Timestamp,
		SeqID:     r.SeqID,
		Metadata:  meta,
	}
}

func (r *Record) GetMeta(key string) (interface{}, bool) {
	if r.Metadata == nil {
		return nil, false
	}
	v, ok := r.Metadata[key]
	return v, ok
}

func (r *Record) SetMeta(key string, value interface{}) {
	if r.Metadata == nil {
		r.Metadata = make(map[string]interface{})
	}
	r.Metadata[key] = value
}

type SourceState int

const (
	SourceStateIdle SourceState = iota
	SourceStateRunning
	SourceStatePaused
	SourceStateStopped
)

func (s SourceState) String() string {
	switch s {
	case SourceStateIdle:
		return "idle"
	case SourceStateRunning:
		return "running"
	case SourceStatePaused:
		return "paused"
	case SourceStateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

type Source interface {
	Start(ctx context.Context) error
	Pause() error
	Resume() error
	Stop() error
	State() SourceState
	Output() <-chan *Record
	Name() string
}

type Operator interface {
	Name() string
	Process(ctx context.Context, input *Record) ([]*Record, error)
	SaveState() ([]byte, error)
	RestoreState(data []byte) error
}

type WindowType int

const (
	WindowTypeTumblingTime WindowType = iota
	WindowTypeSlidingTime
	WindowTypeTumblingCount
	WindowTypeSlidingCount
)

func (w WindowType) String() string {
	switch w {
	case WindowTypeTumblingTime:
		return "tumbling_time"
	case WindowTypeSlidingTime:
		return "sliding_time"
	case WindowTypeTumblingCount:
		return "tumbling_count"
	case WindowTypeSlidingCount:
		return "sliding_count"
	default:
		return "unknown"
	}
}

type AggregationType int

const (
	AggregationSum AggregationType = iota
	AggregationCount
	AggregationAvg
	AggregationMin
	AggregationMax
)

func (a AggregationType) String() string {
	switch a {
	case AggregationSum:
		return "sum"
	case AggregationCount:
		return "count"
	case AggregationAvg:
		return "avg"
	case AggregationMin:
		return "min"
	case AggregationMax:
		return "max"
	default:
		return "unknown"
	}
}

type WindowResult struct {
	WindowID       string
	WindowType     WindowType
	Start          time.Time
	End            time.Time
	StartSeq       int64
	EndSeq         int64
	Aggregation    AggregationType
	Value          float64
	Count          int64
	RecordIDs      []string
}

type BackpressureState int

const (
	BackpressureNormal BackpressureState = iota
	BackpressureWarning
	BackpressureCritical
)

func (b BackpressureState) String() string {
	switch b {
	case BackpressureNormal:
		return "normal"
	case BackpressureWarning:
		return "warning"
	case BackpressureCritical:
		return "critical"
	default:
		return "unknown"
	}
}

type BackpressureInfo struct {
	State          BackpressureState
	PendingCount   int
	Threshold      int
	WarningRatio   float64
	CriticalRatio  float64
}

type Checkpoint struct {
	ID           string
	Timestamp    time.Time
	SourceOffset int64
	OperatorStates map[string][]byte
	WindowStates  map[string][]byte
	Metadata     map[string]interface{}
}

type CheckpointStore interface {
	Save(ctx context.Context, cp *Checkpoint) error
	Load(ctx context.Context, id string) (*Checkpoint, error)
	Latest(ctx context.Context) (*Checkpoint, error)
	List(ctx context.Context) ([]string, error)
	Delete(ctx context.Context, id string) error
	Clear(ctx context.Context) error
}

type Sink interface {
	Consume(ctx context.Context, record *Record) error
	ConsumeWindow(ctx context.Context, result *WindowResult) error
	Close(ctx context.Context) error
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

type PipelineStats struct {
	RecordsIn       int64
	RecordsOut      int64
	RecordsDropped  int64
	WindowsClosed   int64
	CheckpointsMade int64
	Errors          int64
	SourceOffset    int64
	StartTime       time.Time
	Elapsed         time.Duration
}
