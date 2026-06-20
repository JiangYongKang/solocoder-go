package perfsampler

import "errors"

var (
	ErrInvalidSampleRate       = errors.New("perfsampler: invalid sample rate, must be between 0 and 1")
	ErrProfilerNotStarted      = errors.New("perfsampler: profiler not started")
	ErrProfilerAlreadyStarted  = errors.New("perfsampler: profiler already started")
	ErrEmptyRequestID          = errors.New("perfsampler: request ID cannot be empty")
	ErrEmptyLabel              = errors.New("perfsampler: timing label cannot be empty")
	ErrSegmentAlreadyStarted   = errors.New("perfsampler: timing segment already started")
	ErrSegmentNotStarted       = errors.New("perfsampler: timing segment not started")
	ErrNilSampler              = errors.New("perfsampler: sampler cannot be nil")
	ErrInvalidCPUProfile       = errors.New("perfsampler: invalid CPU profile data")
	ErrNotSampled              = errors.New("perfsampler: request was not sampled")
)
