package quotamgr

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidTenantID       = errors.New("quotamgr: tenant ID must not be empty")
	ErrInvalidQuota         = errors.New("quotamgr: quota values must be non-negative")
	ErrInvalidSoftThreshold = errors.New("quotamgr: soft threshold must be between 1.0 and 2.0")
	ErrTenantNotFound      = errors.New("quotamgr: tenant not found")
	ErrInvalidResourceType = errors.New("quotamgr: invalid resource type")
	ErrInvalidAmount       = errors.New("quotamgr: amount must be positive")
	ErrReleaseTooLarge        = errors.New("quotamgr: release amount exceeds current usage")
)

func wrapError(err error, msg string) error {
	return fmt.Errorf("%s: %w", msg, err)
}
