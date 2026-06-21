package slametrics

import "errors"

var (
	ErrNoRequests          = errors.New("slametrics: no requests in time window")
	ErrNoLatencyData       = errors.New("slametrics: no latency data in time window")
	ErrInvalidDecimalPlaces = errors.New("slametrics: invalid decimal places, must be >= 0")
	ErrInvalidTimeRange    = errors.New("slametrics: invalid time range, start must be before end")
	ErrNilSLAConfig        = errors.New("slametrics: SLA config cannot be nil")
)
