package lsm

import "errors"

var (
	ErrKeyNotFound    = errors.New("key not found")
	ErrInvalidRange   = errors.New("invalid range: start > end")
	ErrInvalidLimit   = errors.New("invalid limit: must be positive")
	ErrDBClosed       = errors.New("database is closed")
	ErrEmptyKey       = errors.New("key cannot be empty")
	ErrMergeInProgress = errors.New("merge is already in progress")
)
