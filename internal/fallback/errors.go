package fallback

import (
	"errors"
	"fmt"
)

var (
	ErrNoStrategies          = errors.New("no strategies registered")
	ErrStrategyAlreadyExists = errors.New("strategy already exists")
	ErrStrategyNotFound      = errors.New("strategy not found")
	ErrNilHandler            = errors.New("handler cannot be nil")
	ErrInvalidPriority       = errors.New("invalid priority")
	ErrChainNotRunning       = errors.New("fallback chain is not running")
	ErrChainAlreadyRunning   = errors.New("fallback chain is already running")
	ErrInvalidRecoveryConfig = errors.New("invalid recovery configuration")
	ErrAllStrategiesFailed   = errors.New("all fallback strategies failed")
	ErrExecutionTimeout      = errors.New("execution timed out")
	ErrProbeFailed           = errors.New("probe execution failed")
)

func wrapError(err error, msg string) error {
	return fmt.Errorf("%s: %w", msg, err)
}
