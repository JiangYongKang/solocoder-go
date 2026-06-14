package webhook

import (
	"errors"
	"net/http"
	"time"
)

var (
	ErrCallbackNotFound     = errors.New("webhook: callback not found")
	ErrCallbackAlreadyExists = errors.New("webhook: callback already exists")
	ErrSchedulerStopped     = errors.New("webhook: scheduler is stopped")
	ErrInvalidURL           = errors.New("webhook: invalid URL")
	ErrInvalidMethod        = errors.New("webhook: invalid HTTP method")
	ErrInvalidBackoffType   = errors.New("webhook: invalid backoff type")
	ErrInvalidMaxRetries    = errors.New("webhook: max retries must be >= 0")
	ErrInvalidInterval      = errors.New("webhook: retry interval must be positive")
	ErrInvalidTimeout       = errors.New("webhook: timeout must be positive")
	ErrDeliveryNotFound  = errors.New("webhook: delivery not found")
	ErrCallbackCancelled = errors.New("webhook: callback is cancelled")
)

type BackoffType int

const (
	BackoffFixed    BackoffType = iota
	BackoffExponential BackoffType = iota
)

type CallbackStatus int

const (
	CallbackStatusPending   CallbackStatus = iota
	CallbackStatusSucceeded CallbackStatus = iota
	CallbackStatusFailed    CallbackStatus = iota
	CallbackStatusCancelled CallbackStatus = iota
)

type DeliveryStatus int

const (
	DeliveryStatusPending   DeliveryStatus = iota
	DeliveryStatusSucceeded DeliveryStatus = iota
	DeliveryStatusFailed    DeliveryStatus = iota
	DeliveryStatusTimeout   DeliveryStatus = iota
)

type RetryPolicy struct {
	MaxRetries  int
	Interval    time.Duration
	BackoffType BackoffType
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:  3,
		Interval:    1 * time.Second,
		BackoffType: BackoffExponential,
	}
}

func (p RetryPolicy) Validate() error {
	if p.MaxRetries < 0 {
		return ErrInvalidMaxRetries
	}
	if p.Interval <= 0 {
		return ErrInvalidInterval
	}
	if p.BackoffType != BackoffFixed && p.BackoffType != BackoffExponential {
		return ErrInvalidBackoffType
	}
	return nil
}

func (p RetryPolicy) BackoffDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	switch p.BackoffType {
	case BackoffFixed:
		return p.Interval
	case BackoffExponential:
		return p.Interval * time.Duration(1<<uint(attempt-1))
	default:
		return p.Interval
	}
}

type Callback struct {
	ID           string
	URL          string
	Method       string
	Headers      map[string]string
	BodyTemplate string
	RetryPolicy  RetryPolicy
	Timeout      time.Duration
	Secret       string
	Status       CallbackStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Delivery struct {
	ID           string
	CallbackID   string
	Attempt      int
	Status       DeliveryStatus
	StatusCode   int
	ResponseBody string
	Error        string
	StartedAt    time.Time
	FinishedAt   time.Time
	Duration     time.Duration
}

type DeliveryResult struct {
	Delivery *Delivery
	Final    bool
	Error    error
}

type CallbackOption func(*Callback)

func WithHeaders(headers map[string]string) CallbackOption {
	return func(c *Callback) {
		if c.Headers == nil {
			c.Headers = make(map[string]string)
		}
		for k, v := range headers {
			c.Headers[k] = v
		}
	}
}

func WithBodyTemplate(template string) CallbackOption {
	return func(c *Callback) {
		c.BodyTemplate = template
	}
}

func WithRetryPolicy(policy RetryPolicy) CallbackOption {
	return func(c *Callback) {
		c.RetryPolicy = policy
	}
}

func WithTimeout(timeout time.Duration) CallbackOption {
	return func(c *Callback) {
		c.Timeout = timeout
	}
}

func WithSecret(secret string) CallbackOption {
	return func(c *Callback) {
		c.Secret = secret
	}
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}
