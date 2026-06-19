package timeoutprop

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidConfig       = errors.New("timeoutprop: invalid configuration")
	ErrStageNotFound       = errors.New("timeoutprop: stage not found")
	ErrStageAlreadyExists  = errors.New("timeoutprop: stage already exists")
	ErrEmptyName           = errors.New("timeoutprop: stage name cannot be empty")
	ErrNilHandler          = errors.New("timeoutprop: stage handler cannot be nil")
	ErrBudgetExceedsTotal  = errors.New("timeoutprop: total budget exceeds total timeout")
	ErrNegativeBudget      = errors.New("timeoutprop: budget cannot be negative")
	ErrChainAlreadyExecuted = errors.New("timeoutprop: chain already executed")
	ErrNoStages            = errors.New("timeoutprop: no stages registered")
)

type StageTimeoutError struct {
	StageName   string
	TimeoutType TimeoutType
	Allocated   time.Duration
	Used        time.Duration
}

func (e *StageTimeoutError) Error() string {
	return fmt.Sprintf("stage %s timed out (type: %s, allocated: %v, used: %v)",
		e.StageName, e.TimeoutType, e.Allocated, e.Used)
}

func (e *StageTimeoutError) Unwrap() error {
	switch e.TimeoutType {
	case TimeoutTypeTotal:
		return contextDeadlineExceeded
	case TimeoutTypeBudget:
		return contextDeadlineExceeded
	default:
		return nil
	}
}

var contextDeadlineExceeded = deadlineExceededError{}

type deadlineExceededError struct{}

func (deadlineExceededError) Error() string   { return "context deadline exceeded" }
func (deadlineExceededError) Timeout() bool   { return true }
func (deadlineExceededError) Temporary() bool { return true }
