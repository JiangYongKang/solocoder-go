package rwlocker

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrLockTimeout       = errors.New("rwlocker: lock acquisition timed out")
	ErrDeadlockDetected  = errors.New("rwlocker: deadlock detected - goroutine already holds the lock")
	ErrUpgradeFailed     = errors.New("rwlocker: failed to upgrade read lock to write lock")
	ErrNotHeld           = errors.New("rwlocker: lock is not held by current goroutine")
	ErrInvalidTimeout    = errors.New("rwlocker: timeout must be non-negative")
	ErrHoldDurationExceeded = errors.New("rwlocker: lock hold duration exceeded threshold")
)

type TimeoutError struct {
	LockType string
	Timeout  time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("rwlocker: %s lock acquisition timed out after %v", e.LockType, e.Timeout)
}

func (e *TimeoutError) Unwrap() error {
	return ErrLockTimeout
}

type DeadlockError struct {
	LockType     string
	GoroutineID  int64
	AlreadyHeld  string
}

func (e *DeadlockError) Error() string {
	return fmt.Sprintf(
		"rwlocker: deadlock detected - goroutine %d attempting to acquire %s lock while already holding %s lock",
		e.GoroutineID, e.LockType, e.AlreadyHeld,
	)
}

func (e *DeadlockError) Unwrap() error {
	return ErrDeadlockDetected
}

type UpgradeError struct {
	Reason    string
	ReaderCount int
	Blocking  bool
}

func (e *UpgradeError) Error() string {
	if e.Blocking {
		return fmt.Sprintf(
			"rwlocker: failed to upgrade read lock to write lock - %s (current readers: %d)",
			e.Reason, e.ReaderCount,
		)
	}
	return fmt.Sprintf(
		"rwlocker: failed to upgrade read lock to write lock (non-blocking) - %s (current readers: %d)",
		e.Reason, e.ReaderCount,
	)
}

func (e *UpgradeError) Unwrap() error {
	return ErrUpgradeFailed
}

type HoldDurationWarning struct {
	LockType      string
	HoldDuration  time.Duration
	Threshold     time.Duration
	GoroutineID   int64
}

func (e *HoldDurationWarning) Error() string {
	return fmt.Sprintf(
		"rwlocker: %s lock held for %v by goroutine %d, exceeding threshold %v",
		e.LockType, e.HoldDuration, e.GoroutineID, e.Threshold,
	)
}

func (e *HoldDurationWarning) Unwrap() error {
	return ErrHoldDurationExceeded
}
