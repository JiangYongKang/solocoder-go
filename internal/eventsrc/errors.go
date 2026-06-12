package eventsrc

import "errors"

var (
	ErrAggregateNotFound    = errors.New("aggregate not found")
	ErrVersionConflict      = errors.New("version conflict")
	ErrInvalidEvent         = errors.New("invalid event")
	ErrSnapshotNotFound     = errors.New("snapshot not found")
	ErrAggregateNil         = errors.New("aggregate is nil")
	ErrEventNil             = errors.New("event is nil")
	ErrSnapshotNil          = errors.New("snapshot is nil")
	ErrInvalidAggregateID   = errors.New("invalid aggregate id")
	ErrAggregateIDMismatch  = errors.New("event aggregate id does not match target aggregate id")
)
