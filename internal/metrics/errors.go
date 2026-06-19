package metrics

import "errors"

var (
	ErrMetricNotFound     = errors.New("metrics: metric not found")
	ErrMetricExists       = errors.New("metrics: metric already exists")
	ErrInvalidMetricName  = errors.New("metrics: invalid metric name")
	ErrInvalidLabelName   = errors.New("metrics: invalid label name")
	ErrNegativeCounter    = errors.New("metrics: counter cannot be decremented")
	ErrInvalidBucket      = errors.New("metrics: invalid bucket configuration")
	ErrInvalidQuantile    = errors.New("metrics: invalid quantile value")
	ErrTypeMismatch       = errors.New("metrics: metric type mismatch")
	ErrEmptyBuckets       = errors.New("metrics: buckets cannot be empty")
	ErrEmptyQuantiles     = errors.New("metrics: quantiles cannot be empty")
)
