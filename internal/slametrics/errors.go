package slametrics

import "errors"

var (
	ErrNoRequests          = errors.New("slametrics: no requests in time window")
	ErrInvalidPercentile   = errors.New("slametrics: invalid percentile value, must be in (0, 100]")
	ErrNoLatencyData       = errors.New("slametrics: no latency data in time window")
	ErrInvalidDecimalPlaces = errors.New("slametrics: invalid decimal places, must be >= 0")
	ErrInvalidTimeRange    = errors.New("slametrics: invalid time range, start must be before end")
	ErrEmptyErrorKey       = errors.New("slametrics: error key cannot be empty")
	ErrNilSLAConfig        = errors.New("slametrics: SLA config cannot be nil")
	ErrWindowNotFound      = errors.New("slametrics: window not found")
)
