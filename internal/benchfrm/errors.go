package benchfrm

import "errors"

var (
	ErrNoGroupsRegistered = errors.New("benchfrm: no benchmark groups registered")
	ErrGroupNotFound      = errors.New("benchfrm: benchmark group not found")
	ErrInvalidIterations  = errors.New("benchfrm: invalid number of iterations")
	ErrInvalidWarmup      = errors.New("benchfrm: invalid number of warmup iterations")
	ErrInvalidThreshold   = errors.New("benchfrm: invalid regression threshold")
	ErrNilFunction        = errors.New("benchfrm: benchmark function cannot be nil")
	ErrDuplicateGroupName = errors.New("benchfrm: duplicate group name")
	ErrNoBaselineStore    = errors.New("benchfrm: no baseline store configured")
	ErrBaselineNotFound   = errors.New("benchfrm: baseline not found for group")
	ErrGroupEmptyResult   = errors.New("benchfrm: group has no valid results")
)
