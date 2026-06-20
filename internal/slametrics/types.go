package slametrics

import "time"

type RequestRecord struct {
	Timestamp  time.Time
	Success    bool
	Latency    float64
	ErrorKey   string
}

type AvailabilityResult struct {
	TotalRequests   int
	SuccessRequests int
	FailedRequests  int
	Availability    float64
}

type LatencyPercentiles struct {
	P50    float64
	P90    float64
	P99    float64
	Count  int
	Min    float64
	Max    float64
}

type ErrorStat struct {
	Count    int
	ErrorRate float64
}

type ErrorRateResult struct {
	TotalRequests int
	TotalErrors   int
	TotalErrorRate float64
	ByErrorKey     map[string]ErrorStat
}

type SLAConfig struct {
	MinAvailability   float64
	MaxP99Latency     float64
	MaxP90Latency     float64
	MaxP50Latency     float64
	MaxTotalErrorRate float64
}

type SLAEvaluation struct {
	WindowStart   time.Time
	WindowEnd     time.Time
	Compliant     bool
	Violations    []ViolationDetail
	Availability  float64
	LatencyStats  LatencyPercentiles
	ErrorStats    ErrorRateResult
}

type ViolationDetail struct {
	MetricName string
	Actual     float64
	Target     float64
}

type ViolationEvent struct {
	ID          string
	WindowStart time.Time
	WindowEnd   time.Time
	MetricName  string
	Actual      float64
	Target      float64
	RecordedAt  time.Time
}

type TimeWindow struct {
	Start time.Time
	End   time.Time
}
