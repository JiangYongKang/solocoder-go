package fallback

import (
	"context"
	"sync"
	"time"
)

type HandlerFunc func(ctx context.Context) (interface{}, error)

type TriggerConditionType int

const (
	TriggerConditionTimeout TriggerConditionType = iota
	TriggerConditionErrorType
	TriggerConditionErrorRate
	TriggerConditionCustom
)

type TriggerCondition struct {
	Type         TriggerConditionType
	Timeout      time.Duration
	ErrorTypes   []error
	ErrorRate    float64
	ErrorWindow  time.Duration
	CustomCheck  func(err error) bool
}

type StrategyState int

const (
	StrategyStateActive StrategyState = iota
	StrategyStateDegraded
	StrategyStateRecovering
	StrategyStateWarmingUp
)

func (s StrategyState) String() string {
	switch s {
	case StrategyStateActive:
		return "ACTIVE"
	case StrategyStateDegraded:
		return "DEGRADED"
	case StrategyStateRecovering:
		return "RECOVERING"
	case StrategyStateWarmingUp:
		return "WARMING_UP"
	default:
		return "UNKNOWN"
	}
}

type RecoveryMode int

const (
	RecoveryModePassive RecoveryMode = iota
	RecoveryModeActive
)

type Strategy struct {
	ID              string
	Name            string
	Priority        int
	Handler         HandlerFunc
	TriggerCond     *TriggerCondition
	State           StrategyState
	LastUsedAt      time.Time
	SuccessCount    uint64
	FailureCount    uint64
	ConsecutiveFail uint64
	ErrorWindow     []errorEntry
	mu              sync.RWMutex
}

type errorEntry struct {
	Time time.Time
	Err  error
}

type executionResult struct {
	Result      interface{}
	Err         error
	StrategyID  string
	StrategyName string
	Duration    time.Duration
}

type ChainState int

const (
	ChainStateHealthy ChainState = iota
	ChainStateDegraded
	ChainStateRecovering
)

func (s ChainState) String() string {
	switch s {
	case ChainStateHealthy:
		return "HEALTHY"
	case ChainStateDegraded:
		return "DEGRADED"
	case ChainStateRecovering:
		return "RECOVERING"
	default:
		return "UNKNOWN"
	}
}

type ChainMetrics struct {
	TotalExecutions    uint64
	TotalSuccesses     uint64
	TotalFailures      uint64
	FallbackCount      uint64
	RecoveryCount      uint64
	AvgResponseTime    time.Duration
}

type RecoveryConfig struct {
	Mode                  RecoveryMode
	CheckInterval         time.Duration
	ProbeSuccessThreshold int
	ProbeFailureThreshold int
	WarmUpDuration        time.Duration
	PassiveSuccessWindow  time.Duration
	PassiveSuccessCount   int
}

type Chain struct {
	mu               sync.RWMutex
	strategies       []*Strategy
	strategyMap      map[string]*Strategy
	currentIndex     int
	state            ChainState
	metrics          ChainMetrics
	recoveryCfg      RecoveryConfig
	stopCh           chan struct{}
	wg               sync.WaitGroup
	running          bool
	activeStrategyID string
}

type AggregateError struct {
	Errors []error
}

func (e *AggregateError) Error() string {
	if len(e.Errors) == 0 {
		return "all fallback strategies failed"
	}
	msg := "all fallback strategies failed: "
	for i, err := range e.Errors {
		if i > 0 {
			msg += "; "
		}
		msg += err.Error()
	}
	return msg
}

func (e *AggregateError) Unwrap() []error {
	return e.Errors
}
