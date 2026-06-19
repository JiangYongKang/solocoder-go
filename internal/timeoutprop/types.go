package timeoutprop

import (
	"context"
	"sync"
	"time"
)

type StageFunc func(ctx context.Context) error

type TimeoutType int

const (
	TimeoutTypeNone TimeoutType = iota
	TimeoutTypeTotal
	TimeoutTypeBudget
	TimeoutTypeMinThreshold
)

func (t TimeoutType) String() string {
	switch t {
	case TimeoutTypeNone:
		return "NONE"
	case TimeoutTypeTotal:
		return "TOTAL_TIMEOUT"
	case TimeoutTypeBudget:
		return "BUDGET_TIMEOUT"
	case TimeoutTypeMinThreshold:
		return "MIN_THRESHOLD_SKIP"
	default:
		return "UNKNOWN"
	}
}

type StageStatus int

const (
	StageStatusPending StageStatus = iota
	StageStatusRunning
	StageStatusCompleted
	StageStatusSkipped
	StageStatusTimedOut
	StageStatusFailed
)

func (s StageStatus) String() string {
	switch s {
	case StageStatusPending:
		return "PENDING"
	case StageStatusRunning:
		return "RUNNING"
	case StageStatusCompleted:
		return "COMPLETED"
	case StageStatusSkipped:
		return "SKIPPED"
	case StageStatusTimedOut:
		return "TIMED_OUT"
	case StageStatusFailed:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

type StageInfo struct {
	Name          string
	AllocatedBudget time.Duration
	UsedBudget    time.Duration
	RemainingBudget time.Duration
	Status        StageStatus
	TimeoutType   TimeoutType
	StartTime     time.Time
	EndTime       time.Time
	Error         error
}

type ChainReport struct {
	TotalTimeout  time.Duration
	TotalUsed     time.Duration
	RemainingTime time.Duration
	Stages        []*StageInfo
	Success       bool
	FailedStage   string
	TimeoutReason string
}

func (r *ChainReport) String() string {
	result := "=== Timeout Propagation Chain Report ===\n"
	result += "Total Timeout: " + r.TotalTimeout.String() + "\n"
	result += "Total Used: " + r.TotalUsed.String() + "\n"
	result += "Remaining: " + r.RemainingTime.String() + "\n"
	result += "Success: " + boolToString(r.Success) + "\n"
	if r.TimeoutReason != "" {
		result += "Timeout Reason: " + r.TimeoutReason + "\n"
	}
	if r.FailedStage != "" {
		result += "Failed Stage: " + r.FailedStage + "\n"
	}
	result += "\nStages:\n"
	for i, s := range r.Stages {
		result += "  [" + itoa(i) + "] " + s.Name + "\n"
		result += "      Status: " + s.Status.String() + "\n"
		result += "      Budget: " + s.AllocatedBudget.String() + " / Used: " + s.UsedBudget.String() + " / Remaining: " + s.RemainingBudget.String() + "\n"
		if s.TimeoutType != TimeoutTypeNone {
			result += "      Timeout Type: " + s.TimeoutType.String() + "\n"
		}
		if s.Error != nil {
			result += "      Error: " + s.Error.Error() + "\n"
		}
	}
	return result
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

type Stage struct {
	name           string
	fn             StageFunc
	allocatedBudget time.Duration
	mu             sync.Mutex
	info           *StageInfo
}

func newStage(name string, budget time.Duration, fn StageFunc) *Stage {
	return &Stage{
		name:           name,
		fn:             fn,
		allocatedBudget: budget,
		info: &StageInfo{
			Name:            name,
			AllocatedBudget: budget,
			Status:          StageStatusPending,
			TimeoutType:     TimeoutTypeNone,
		},
	}
}

type Propagator struct {
	mu           sync.Mutex
	stages       []*Stage
	stageMap     map[string]*Stage
	totalTimeout time.Duration
	minThreshold time.Duration
	rootCancel   context.CancelFunc
	report       *ChainReport
	executed     bool
}
