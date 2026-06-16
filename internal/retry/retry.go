package retry

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

var (
	ErrMaxRetriesExceeded = errors.New("retry: max retries exceeded")
	ErrInvalidConfig      = errors.New("retry: invalid configuration")
	ErrNonRetryable       = errors.New("retry: non-retryable error")
)

type RetryableFunc func(ctx context.Context) error

type IsRetryableFunc func(err error) bool

type OnRetryFunc func(attempt int, err error)

type Config struct {
	InitialInterval time.Duration
	MaxInterval     time.Duration
	MaxRetries      int
	JitterFactor    float64
	IsRetryable     IsRetryableFunc
	OnRetryBefore   OnRetryFunc
	OnRetryAfter    OnRetryFunc
}

func DefaultConfig() Config {
	return Config{
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     10 * time.Second,
		MaxRetries:      3,
		JitterFactor:    0.1,
		IsRetryable:     DefaultIsRetryable,
	}
}

func DefaultIsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	return true
}

type AggregateError struct {
	Errors []error
}

func (e *AggregateError) Error() string {
	if len(e.Errors) == 0 {
		return "retry: no errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	return fmt.Sprintf("retry: %d attempts failed: %v", len(e.Errors), e.Errors)
}

func (e *AggregateError) Unwrap() []error {
	return e.Errors
}

type Retryer struct {
	cfg       Config
	attempts  int
	errs      []error
	randSrc   *rand.Rand
	sleepFunc func(time.Duration)
}

func NewRetryer() *Retryer {
	return NewRetryerWithConfig(DefaultConfig())
}

func NewRetryerWithConfig(cfg Config) *Retryer {
	if cfg.InitialInterval <= 0 {
		cfg.InitialInterval = DefaultConfig().InitialInterval
	}
	if cfg.MaxInterval <= 0 {
		cfg.MaxInterval = DefaultConfig().MaxInterval
	}
	if cfg.MaxInterval < cfg.InitialInterval {
		cfg.MaxInterval = cfg.InitialInterval
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.JitterFactor < 0 {
		cfg.JitterFactor = 0
	}
	if cfg.JitterFactor > 1 {
		cfg.JitterFactor = 1
	}
	if cfg.IsRetryable == nil {
		cfg.IsRetryable = DefaultIsRetryable
	}
	return &Retryer{
		cfg:     cfg,
		errs:    make([]error, 0),
		randSrc: rand.New(rand.NewSource(time.Now().UnixNano())),
		sleepFunc: func(d time.Duration) {
			time.Sleep(d)
		},
	}
}

func (r *Retryer) Do(ctx context.Context, fn RetryableFunc) error {
	r.attempts = 0
	r.errs = r.errs[:0]

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}

		r.attempts++
		r.errs = append(r.errs, err)

		if !r.cfg.IsRetryable(err) {
			return &AggregateError{Errors: append([]error{}, r.errs...)}
		}

		if r.attempts > r.cfg.MaxRetries {
			return &AggregateError{Errors: append([]error{}, r.errs...)}
		}

		if r.cfg.OnRetryBefore != nil {
			func() {
				defer func() { recover() }()
				r.cfg.OnRetryBefore(r.attempts, err)
			}()
		}

		interval := r.nextInterval(r.attempts - 1)

		if ctx.Err() != nil {
			return ctx.Err()
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}

		if r.cfg.OnRetryAfter != nil {
			func() {
				defer func() { recover() }()
				r.cfg.OnRetryAfter(r.attempts, err)
			}()
		}
	}
}

func (r *Retryer) nextInterval(attempt int) time.Duration {
	interval := r.cfg.InitialInterval
	for i := 0; i < attempt; i++ {
		interval *= 2
		if interval >= r.cfg.MaxInterval {
			interval = r.cfg.MaxInterval
			break
		}
	}

	if interval > r.cfg.MaxInterval {
		interval = r.cfg.MaxInterval
	}

	if r.cfg.JitterFactor > 0 {
		jitterRange := float64(interval) * r.cfg.JitterFactor
		jitter := r.randSrc.Float64()*jitterRange*2 - jitterRange
		interval = time.Duration(float64(interval) + jitter)
		if interval < 0 {
			interval = 0
		}
	}

	return interval
}

func (r *Retryer) Attempts() int {
	return r.attempts
}

func (r *Retryer) Errors() []error {
	return append([]error{}, r.errs...)
}

func (r *Retryer) Config() Config {
	return r.cfg
}

func Do(ctx context.Context, fn RetryableFunc, opts ...Option) error {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	r := NewRetryerWithConfig(cfg)
	return r.Do(ctx, fn)
}

type Option func(*Config)

func WithInitialInterval(d time.Duration) Option {
	return func(c *Config) {
		c.InitialInterval = d
	}
}

func WithMaxInterval(d time.Duration) Option {
	return func(c *Config) {
		c.MaxInterval = d
	}
}

func WithMaxRetries(n int) Option {
	return func(c *Config) {
		c.MaxRetries = n
	}
}

func WithJitterFactor(f float64) Option {
	return func(c *Config) {
		c.JitterFactor = f
	}
}

func WithIsRetryable(fn IsRetryableFunc) Option {
	return func(c *Config) {
		c.IsRetryable = fn
	}
}

func WithOnRetryBefore(fn OnRetryFunc) Option {
	return func(c *Config) {
		c.OnRetryBefore = fn
	}
}

func WithOnRetryAfter(fn OnRetryFunc) Option {
	return func(c *Config) {
		c.OnRetryAfter = fn
	}
}
