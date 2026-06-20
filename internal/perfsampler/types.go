package perfsampler

import (
	"encoding/json"
	"time"
)

type CPUStackNode struct {
	FunctionName string           `json:"function_name"`
	SelfTimeNs   int64            `json:"self_time_ns"`
	TotalTimeNs  int64            `json:"total_time_ns"`
	SampleCount  int              `json:"sample_count"`
	Children     []*CPUStackNode  `json:"children,omitempty"`
}

type MemoryFuncStat struct {
	FunctionName    string `json:"function_name"`
	AllocCount      int64  `json:"alloc_count"`
	AllocBytes      int64  `json:"alloc_bytes"`
	FreeCount       int64  `json:"free_count,omitempty"`
	FreeBytes       int64  `json:"free_bytes,omitempty"`
	InUseCount      int64  `json:"in_use_count,omitempty"`
	InUseBytes      int64  `json:"in_use_bytes,omitempty"`
}

type TimingSegment struct {
	Label      string        `json:"label"`
	StartTime  time.Time     `json:"start_time"`
	Duration   time.Duration `json:"duration_ns"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type ProfileResult struct {
	RequestID    string                `json:"request_id"`
	Sampled      bool                  `json:"sampled"`
	SampleRate   float64               `json:"sample_rate,omitempty"`
	StartTime    time.Time             `json:"start_time"`
	EndTime      time.Time             `json:"end_time"`
	Duration     time.Duration         `json:"duration_ns"`
	CPUProfile   *CPUStackNode         `json:"cpu_profile,omitempty"`
	MemoryStats  []*MemoryFuncStat     `json:"memory_stats,omitempty"`
	Timing       []*TimingSegment      `json:"timing_segments,omitempty"`
}

type FlameGraphEntry struct {
	Stack []string `json:"stack"`
	Value int      `json:"value"`
}

func (p *ProfileResult) JSON() ([]byte, error) {
	return json.Marshal(p)
}

func (p *ProfileResult) PrettyJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

type Sampler interface {
	ShouldSample(requestID string) bool
	Rate() float64
}
