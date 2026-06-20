package benchfrm

import "time"

type RunConfig struct {
	Iterations       int
	WarmupIterations int
	CollectMemory    bool
	Timeout          time.Duration
}

type RunOption func(*RunConfig)

func DefaultConfig() RunConfig {
	return RunConfig{
		Iterations:       100,
		WarmupIterations: 10,
		CollectMemory:    true,
		Timeout:          0,
	}
}

func WithIterations(n int) RunOption {
	return func(c *RunConfig) {
		c.Iterations = n
	}
}

func WithWarmupIterations(n int) RunOption {
	return func(c *RunConfig) {
		c.WarmupIterations = n
	}
}

func WithMemoryCollection(enabled bool) RunOption {
	return func(c *RunConfig) {
		c.CollectMemory = enabled
	}
}

func WithTimeout(d time.Duration) RunOption {
	return func(c *RunConfig) {
		c.Timeout = d
	}
}

func (c RunConfig) Validate() error {
	if c.Iterations <= 0 {
		return ErrInvalidIterations
	}
	if c.WarmupIterations < 0 {
		return ErrInvalidWarmup
	}
	return nil
}
