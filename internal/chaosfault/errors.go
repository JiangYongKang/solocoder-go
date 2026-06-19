package chaosfault

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidConfig      = errors.New("chaosfault: invalid configuration")
	ErrConnectionBroken   = errors.New("chaosfault: connection is broken")
	ErrInvalidTargetRatio = errors.New("chaosfault: invalid target ratio")
	ErrInvalidTimeWindow  = errors.New("chaosfault: invalid time window")
)

func wrapError(err error, msg string) error {
	return fmt.Errorf("%s: %w", msg, err)
}

type InjectedError struct {
	Message string
	Cause   error
}

func (e *InjectedError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *InjectedError) Unwrap() error {
	return e.Cause
}

type ConnectionBrokenError struct {
	Message string
}

func (e *ConnectionBrokenError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return ErrConnectionBroken.Error()
}

func (e *ConnectionBrokenError) Unwrap() error {
	return ErrConnectionBroken
}
