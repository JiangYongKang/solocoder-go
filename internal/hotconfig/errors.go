package hotconfig

import (
	"errors"
	"fmt"
)

var (
	ErrFileNotFound        = errors.New("config file not found")
	ErrUnsupportedFormat   = errors.New("unsupported config format")
	ErrParseFailed         = errors.New("failed to parse config")
	ErrValidationFailed    = errors.New("validation failed")
	ErrFieldRequired       = errors.New("field is required")
	ErrFieldOutOfRange     = errors.New("field value out of range")
	ErrFieldInvalidFormat  = errors.New("field has invalid format")
	ErrFieldTypeMismatch   = errors.New("field type mismatch")
	ErrWatcherNotStarted   = errors.New("watcher not started")
	ErrWatcherAlreadyRunning = errors.New("watcher already running")
	ErrInvalidConfigPath   = errors.New("invalid config path")
	ErrNilCallback         = errors.New("callback cannot be nil")
)

type ValidationError struct {
	Field   string
	Message string
	Err     error
}

func (e *ValidationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("field %q validation failed: %s (%v)", e.Field, e.Message, e.Err)
	}
	return fmt.Sprintf("field %q validation failed: %s", e.Field, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

type AggregateValidationError struct {
	Errors []*ValidationError
}

func (e *AggregateValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	msg := fmt.Sprintf("validation failed with %d error(s): ", len(e.Errors))
	for i, err := range e.Errors {
		if i > 0 {
			msg += "; "
		}
		msg += err.Error()
	}
	return msg
}

func (e *AggregateValidationError) Unwrap() []error {
	errs := make([]error, len(e.Errors))
	for i, err := range e.Errors {
		errs[i] = err
	}
	return errs
}

type ParseError struct {
	Format string
	Path   string
	Err    error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("failed to parse %s file %q: %v", e.Format, e.Path, e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}
