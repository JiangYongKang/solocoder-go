package webhook

import (
	"container/heap"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockTransport struct {
	mu               sync.Mutex
	handlers         map[string]func(req *http.Request) (*http.Response, error)
	callCount        map[string]int
	delay            map[string]time.Duration
	removeDelayAfter map[string]int
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		handlers:         make(map[string]func(req *http.Request) (*http.Response, error)),
		callCount:        make(map[string]int),
		delay:            make(map[string]time.Duration),
		removeDelayAfter: make(map[string]int),
	}
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	key := req.URL.String()
	m.mu.Lock()
	m.callCount[key]++
	cur := m.callCount[key]
	if threshold, ok := m.removeDelayAfter[key]; ok && cur > threshold {
		m.delay[key] = 0
	}
	delay, hasDelay := m.delay[key]
	handler, hasHandler := m.handlers[key]
	m.mu.Unlock()

	if hasDelay && delay > 0 {
		ctx := req.Context()
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		}
	}

	if !hasHandler {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("OK")),
			Header:     make(http.Header),
		}, nil
	}
	return handler(req)
}

func (m *mockTransport) getCallCount(url string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount[url]
}

type mockHTTPClient struct {
	transport *mockTransport
}

func newMockHTTPClient() *mockHTTPClient {
	return &mockHTTPClient{transport: newMockTransport()}
}

func (c *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.transport.RoundTrip(req)
}

func okResponse() (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("OK")),
		Header:     make(http.Header),
	}, nil
}

func error500Response() (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader("Internal Server Error")),
		Header:     make(http.Header),
	}, nil
}

func error404Response() (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader("Not Found")),
		Header:     make(http.Header),
	}, nil
}

func TestRetryPolicy_Validate(t *testing.T) {
	tests := []struct {
		name    string
		policy  RetryPolicy
		wantErr error
	}{
		{
			name: "valid fixed",
			policy: RetryPolicy{
				MaxRetries:  3,
				Interval:    time.Second,
				BackoffType: BackoffFixed,
			},
			wantErr: nil,
		},
		{
			name: "valid exponential",
			policy: RetryPolicy{
				MaxRetries:  5,
				Interval:    500 * time.Millisecond,
				BackoffType: BackoffExponential,
			},
			wantErr: nil,
		},
		{
			name: "zero retries valid",
			policy: RetryPolicy{
				MaxRetries:  0,
				Interval:    time.Second,
				BackoffType: BackoffFixed,
			},
			wantErr: nil,
		},
		{
			name: "negative retries",
			policy: RetryPolicy{
				MaxRetries:  -1,
				Interval:    time.Second,
				BackoffType: BackoffFixed,
			},
			wantErr: ErrInvalidMaxRetries,
		},
		{
			name: "zero interval",
			policy: RetryPolicy{
				MaxRetries:  3,
				Interval:    0,
				BackoffType: BackoffFixed,
			},
			wantErr: ErrInvalidInterval,
		},
		{
			name: "negative interval",
			policy: RetryPolicy{
				MaxRetries:  3,
				Interval:    -1,
				BackoffType: BackoffFixed,
			},
			wantErr: ErrInvalidInterval,
		},
		{
			name: "invalid backoff type",
			policy: RetryPolicy{
				MaxRetries:  3,
				Interval:    time.Second,
				BackoffType: 99,
			},
			wantErr: ErrInvalidBackoffType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRetryPolicy_BackoffDelay(t *testing.T) {
	t.Run("fixed backoff", func(t *testing.T) {
		p := RetryPolicy{
			MaxRetries:  5,
			Interval:    100 * time.Millisecond,
			BackoffType: BackoffFixed,
		}
		if d := p.BackoffDelay(0); d != 0 {
			t.Errorf("attempt 0 delay = %v, want 0", d)
		}
		if d := p.BackoffDelay(1); d != 100*time.Millisecond {
			t.Errorf("attempt 1 delay = %v, want 100ms", d)
		}
		if d := p.BackoffDelay(2); d != 100*time.Millisecond {
			t.Errorf("attempt 2 delay = %v, want 100ms", d)
		}
		if d := p.BackoffDelay(3); d != 100*time.Millisecond {
			t.Errorf("attempt 3 delay = %v, want 100ms", d)
		}
	})

	t.Run("exponential backoff", func(t *testing.T) {
		p := RetryPolicy{
			MaxRetries:  5,
			Interval:    100 * time.Millisecond,
			BackoffType: BackoffExponential,
		}
		if d := p.BackoffDelay(0); d != 0 {
			t.Errorf("attempt 0 delay = %v, want 0", d)
		}
		if d := p.BackoffDelay(1); d != 100*time.Millisecond {
			t.Errorf("attempt 1 delay = %v, want 100ms", d)
		}
		if d := p.BackoffDelay(2); d != 200*time.Millisecond {
			t.Errorf("attempt 2 delay = %v, want 200ms", d)
		}
		if d := p.BackoffDelay(3); d != 400*time.Millisecond {
			t.Errorf("attempt 3 delay = %v, want 400ms", d)
		}
		if d := p.BackoffDelay(4); d != 800*time.Millisecond {
			t.Errorf("attempt 4 delay = %v, want 800ms", d)
		}
	})
}

func TestDefaultRetryPolicy(t *testing.T) {
	p := DefaultRetryPolicy()
	if p.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", p.MaxRetries)
	}
	if p.Interval != time.Second {
		t.Errorf("Interval = %v, want 1s", p.Interval)
	}
	if p.BackoffType != BackoffExponential {
		t.Errorf("BackoffType = %v, want BackoffExponential", p.BackoffType)
	}
	if err := p.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestHMACSigner(t *testing.T) {
	secret := "my-secret-key"
	payload := []byte(`{"event":"test","data":"value"}`)
	timestamp := "1234567890"

	t.Run("generate and verify match", func(t *testing.T) {
		sig := GenerateHMACSHA256(secret, payload, timestamp)
		if !VerifyHMACSHA256(secret, payload, timestamp, sig) {
			t.Error("signature verification failed for matching data")
		}
	})

	t.Run("verify wrong secret", func(t *testing.T) {
		sig := GenerateHMACSHA256(secret, payload, timestamp)
		if VerifyHMACSHA256("wrong-secret", payload, timestamp, sig) {
			t.Error("signature should not verify with wrong secret")
		}
	})

	t.Run("verify wrong payload", func(t *testing.T) {
		sig := GenerateHMACSHA256(secret, payload, timestamp)
		if VerifyHMACSHA256(secret, []byte("wrong payload"), timestamp, sig) {
			t.Error("signature should not verify with wrong payload")
		}
	})

	t.Run("verify wrong timestamp", func(t *testing.T) {
		sig := GenerateHMACSHA256(secret, payload, timestamp)
		if VerifyHMACSHA256(secret, payload, "9999999999", sig) {
			t.Error("signature should not verify with wrong timestamp")
		}
	})

	t.Run("verify malformed signature", func(t *testing.T) {
		if VerifyHMACSHA256(secret, payload, timestamp, "invalid") {
			t.Error("should not verify malformed signature")
		}
	})

	t.Run("empty timestamp", func(t *testing.T) {
		sig := GenerateHMACSHA256(secret, payload, "")
		if !VerifyHMACSHA256(secret, payload, "", sig) {
			t.Error("signature verification failed with empty timestamp")
		}
	})

	t.Run("signature prefix", func(t *testing.T) {
		sig := GenerateHMACSHA256(secret, payload, timestamp)
		if !strings.HasPrefix(sig, "sha256=") {
			t.Errorf("signature should start with 'sha256=', got %s", sig)
		}
	})

	t.Run("empty payload", func(t *testing.T) {
		sig := GenerateHMACSHA256(secret, []byte{}, timestamp)
		if !VerifyHMACSHA256(secret, []byte{}, timestamp, sig) {
			t.Error("signature verification failed for empty payload")
		}
	})
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"http://example.com", true},
		{"https://example.com", true},
		{"https://example.com:8080/path", true},
		{"http://localhost", true},
		{"https://api.example.com/webhook?x=1", true},
		{"ftp://example.com", false},
		{"example.com", false},
		{"", false},
		{"://missing-scheme.com", false},
		{"http://", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := validateURL(tt.url); got != tt.want {
				t.Errorf("validateURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestValidateMethod(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"GET", true},
		{"POST", true},
		{"PUT", true},
		{"DELETE", true},
		{"PATCH", true},
		{"HEAD", true},
		{"OPTIONS", true},
		{"get", true},
		{"post", true},
		{"Post", true},
		{"INVALID", false},
		{"", false},
		{"CONNECT", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if got := validateMethod(tt.method); got != tt.want {
				t.Errorf("validateMethod(%q) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}

func TestNewScheduler(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		s := NewScheduler(SchedulerConfig{})
		if s == nil {
			t.Fatal("NewScheduler returned nil")
		}
		if s.workerCount != defaultWorkerCount {
			t.Errorf("workerCount = %d, want %d", s.workerCount, defaultWorkerCount)
		}
	})

	t.Run("custom worker count", func(t *testing.T) {
		s := NewScheduler(SchedulerConfig{WorkerCount: 8})
		if s.workerCount != 8 {
			t.Errorf("workerCount = %d, want 8", s.workerCount)
		}
	})

	t.Run("negative worker count uses default", func(t *testing.T) {
		s := NewScheduler(SchedulerConfig{WorkerCount: -1})
		if s.workerCount != defaultWorkerCount {
			t.Errorf("workerCount = %d, want %d", s.workerCount, defaultWorkerCount)
		}
	})
}

func TestRegister_Basic(t *testing.T) {
	mc := newMockHTTPClient()
	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	t.Run("valid registration", func(t *testing.T) {
		id, err := s.Register("https://example.com/webhook", "POST")
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if id == "" {
			t.Error("Register() returned empty ID")
		}

		cb, err := s.GetCallback(id)
		if err != nil {
			t.Fatalf("GetCallback() error = %v", err)
		}
		if cb.URL != "https://example.com/webhook" {
			t.Errorf("URL = %q, want https://example.com/webhook", cb.URL)
		}
		if cb.Method != "POST" {
			t.Errorf("Method = %q, want POST", cb.Method)
		}
		if cb.Status != CallbackStatusPending {
			t.Errorf("Status = %v, want CallbackStatusPending", cb.Status)
		}
	})

	t.Run("method normalized to uppercase", func(t *testing.T) {
		id, err := s.Register("https://example.com/webhook2", "post")
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		cb, _ := s.GetCallback(id)
		if cb.Method != "POST" {
			t.Errorf("Method = %q, want POST", cb.Method)
		}
	})

	t.Run("invalid url", func(t *testing.T) {
		_, err := s.Register("not-a-url", "POST")
		if err != ErrInvalidURL {
			t.Errorf("Register() error = %v, want ErrInvalidURL", err)
		}
	})

	t.Run("invalid method", func(t *testing.T) {
		_, err := s.Register("https://example.com/hook", "INVALID")
		if err != ErrInvalidMethod {
			t.Errorf("Register() error = %v, want ErrInvalidMethod", err)
		}
	})

	t.Run("with id duplicate", func(t *testing.T) {
		err := s.RegisterWithID("my-id", "https://example.com/a", "POST")
		if err != nil {
			t.Fatalf("RegisterWithID() error = %v", err)
		}
		err = s.RegisterWithID("my-id", "https://example.com/b", "POST")
		if err != ErrCallbackAlreadyExists {
			t.Errorf("RegisterWithID() error = %v, want ErrCallbackAlreadyExists", err)
		}
	})
}

func TestRegister_WithOptions(t *testing.T) {
	mc := newMockHTTPClient()
	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	headers := map[string]string{
		"X-Custom":     "value",
		"Content-Type": "application/json",
	}
	bodyTpl := `{"event":"{{.Event}}"}`
	policy := RetryPolicy{
		MaxRetries:  5,
		Interval:    200 * time.Millisecond,
		BackoffType: BackoffFixed,
	}

	id, err := s.Register(
		"https://example.com/hook",
		"POST",
		WithHeaders(headers),
		WithBodyTemplate(bodyTpl),
		WithRetryPolicy(policy),
		WithTimeout(5*time.Second),
		WithSecret("secret-key"),
	)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	cb, _ := s.GetCallback(id)
	if cb.Headers["X-Custom"] != "value" {
		t.Errorf("Headers[X-Custom] = %q, want value", cb.Headers["X-Custom"])
	}
	if cb.BodyTemplate != bodyTpl {
		t.Errorf("BodyTemplate mismatch")
	}
	if cb.RetryPolicy.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cb.RetryPolicy.MaxRetries)
	}
	if cb.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", cb.Timeout)
	}
	if cb.Secret != "secret-key" {
		t.Errorf("Secret mismatch")
	}
}

func TestRegister_InvalidOptions(t *testing.T) {
	mc := newMockHTTPClient()
	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	t.Run("invalid retry policy negative retries", func(t *testing.T) {
		_, err := s.Register("https://example.com/hook", "POST",
			WithRetryPolicy(RetryPolicy{MaxRetries: -1, Interval: time.Second}))
		if err != ErrInvalidMaxRetries {
			t.Errorf("error = %v, want ErrInvalidMaxRetries", err)
		}
	})

	t.Run("invalid timeout zero", func(t *testing.T) {
		_, err := s.Register("https://example.com/hook", "POST", WithTimeout(0))
		if err != ErrInvalidTimeout {
			t.Errorf("error = %v, want ErrInvalidTimeout", err)
		}
	})
}

func TestRegister_NotStarted(t *testing.T) {
	mc := newMockHTTPClient()
	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})

	_, err := s.Register("https://example.com/hook", "POST")
	if err != ErrSchedulerStopped {
		t.Errorf("Register() error = %v, want ErrSchedulerStopped", err)
	}
}

func TestTrigger_Success(t *testing.T) {
	mc := newMockHTTPClient()
	url := "https://example.com/success"
	mc.transport.handlers[url] = func(req *http.Request) (*http.Response, error) {
		return okResponse()
	}

	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	id, _ := s.Register(url, "POST")
	err := s.Trigger(id)
	if err != nil {
		t.Fatalf("Trigger() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := s.WaitForResult(ctx, id)
	if err != nil {
		t.Fatalf("WaitForResult() error = %v", err)
	}
	if !result.Final {
		t.Error("result should be final")
	}
	if result.Delivery.Status != DeliveryStatusSucceeded {
		t.Errorf("Delivery status = %v, want DeliveryStatusSucceeded", result.Delivery.Status)
	}
	if result.Delivery.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.Delivery.StatusCode)
	}
}

func TestTrigger_SignatureHeader(t *testing.T) {
	mc := newMockHTTPClient()
	url := "https://example.com/sig"
	secret := "test-secret-123"
	body := `{"key":"value"}`
	var capturedSig string
	var capturedTs string
	var capturedBody []byte
	var mu sync.Mutex

	mc.transport.handlers[url] = func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		capturedSig = req.Header.Get(SignatureHeader)
		capturedTs = req.Header.Get(TimestampHeader)
		capturedBody, _ = io.ReadAll(req.Body)
		mu.Unlock()
		return okResponse()
	}

	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	id, _ := s.Register(url, "POST",
		WithBodyTemplate(body),
		WithSecret(secret),
	)
	s.Trigger(id)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.WaitForResult(ctx, id)

	mu.Lock()
	defer mu.Unlock()

	if capturedSig == "" {
		t.Error("signature header not set")
	}
	if capturedTs == "" {
		t.Error("timestamp header not set")
	}
	if !VerifyHMACSHA256(secret, capturedBody, capturedTs, capturedSig) {
		t.Error("signature verification failed")
	}
}

func TestTrigger_NoSecret_NoSignature(t *testing.T) {
	mc := newMockHTTPClient()
	url := "https://example.com/no-sig"
	var capturedSig string
	var mu sync.Mutex

	mc.transport.handlers[url] = func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		capturedSig = req.Header.Get(SignatureHeader)
		mu.Unlock()
		return okResponse()
	}

	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	id, _ := s.Register(url, "POST", WithBodyTemplate(`{}`))
	s.Trigger(id)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.WaitForResult(ctx, id)

	mu.Lock()
	defer mu.Unlock()
	if capturedSig != "" {
		t.Error("signature header should not be set when no secret")
	}
}

func TestTrigger_RetryThenSuccess_Fixed(t *testing.T) {
	mc := newMockHTTPClient()
	url := "https://example.com/retry-fixed"
	var mu sync.Mutex
	attempts := 0

	mc.transport.handlers[url] = func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		attempts++
		cur := attempts
		mu.Unlock()
		if cur < 3 {
			return error500Response()
		}
		return okResponse()
	}

	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	policy := RetryPolicy{
		MaxRetries:  4,
		Interval:    50 * time.Millisecond,
		BackoffType: BackoffFixed,
	}
	id, _ := s.Register(url, "POST", WithRetryPolicy(policy))
	s.Trigger(id)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := s.WaitForResult(ctx, id)
	if err != nil {
		t.Fatalf("WaitForResult() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
	if result.Delivery.Status != DeliveryStatusSucceeded {
		t.Errorf("status = %v, want success", result.Delivery.Status)
	}
}

func TestTrigger_RetryThenSuccess_Exponential(t *testing.T) {
	mc := newMockHTTPClient()
	url := "https://example.com/retry-exp"
	var mu sync.Mutex
	attempts := 0

	mc.transport.handlers[url] = func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		attempts++
		cur := attempts
		mu.Unlock()
		if cur < 2 {
			return error500Response()
		}
		return okResponse()
	}

	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	policy := RetryPolicy{
		MaxRetries:  3,
		Interval:    30 * time.Millisecond,
		BackoffType: BackoffExponential,
	}
	id, _ := s.Register(url, "POST", WithRetryPolicy(policy))
	s.Trigger(id)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := s.WaitForResult(ctx, id)
	if err != nil {
		t.Fatalf("WaitForResult() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
	if result.Delivery.Status != DeliveryStatusSucceeded {
		t.Errorf("status = %v, want success", result.Delivery.Status)
	}
}

func TestTrigger_RetryExhausted_Failed(t *testing.T) {
	mc := newMockHTTPClient()
	url := "https://example.com/exhausted"
	var mu sync.Mutex
	attempts := 0

	mc.transport.handlers[url] = func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		attempts++
		mu.Unlock()
		return error500Response()
	}

	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	policy := RetryPolicy{
		MaxRetries:  2,
		Interval:    20 * time.Millisecond,
		BackoffType: BackoffFixed,
	}
	id, _ := s.Register(url, "POST", WithRetryPolicy(policy))
	s.Trigger(id)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := s.WaitForResult(ctx, id)
	if err != nil {
		t.Fatalf("WaitForResult() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3 (1 initial + 2 retries)", attempts)
	}
	if result.Delivery.Status != DeliveryStatusFailed {
		t.Errorf("status = %v, want DeliveryStatusFailed", result.Delivery.Status)
	}
	if !result.Final {
		t.Error("result should be final")
	}

	cb, _ := s.GetCallback(id)
	if cb.Status != CallbackStatusFailed {
		t.Errorf("callback status = %v, want CallbackStatusFailed", cb.Status)
	}
}

func TestTrigger_RetryExhausted_ZeroRetries(t *testing.T) {
	mc := newMockHTTPClient()
	url := "https://example.com/zero-retry"
	var mu sync.Mutex
	attempts := 0

	mc.transport.handlers[url] = func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		attempts++
		mu.Unlock()
		return error404Response()
	}

	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	policy := RetryPolicy{
		MaxRetries:  0,
		Interval:    100 * time.Millisecond,
		BackoffType: BackoffFixed,
	}
	id, _ := s.Register(url, "POST", WithRetryPolicy(policy))
	s.Trigger(id)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := s.WaitForResult(ctx, id)
	if err != nil {
		t.Fatalf("WaitForResult() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (no retries)", attempts)
	}
	if result.Delivery.Status != DeliveryStatusFailed {
		t.Errorf("status = %v, want failed", result.Delivery.Status)
	}
}

func TestTrigger_Timeout(t *testing.T) {
	mc := newMockHTTPClient()
	url := "https://example.com/timeout"
	mc.transport.delay[url] = 500 * time.Millisecond
	mc.transport.handlers[url] = func(req *http.Request) (*http.Response, error) {
		return okResponse()
	}

	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	policy := RetryPolicy{
		MaxRetries:  0,
		Interval:    50 * time.Millisecond,
		BackoffType: BackoffFixed,
	}
	id, _ := s.Register(url, "POST",
		WithTimeout(50*time.Millisecond),
		WithRetryPolicy(policy),
	)
	s.Trigger(id)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := s.WaitForResult(ctx, id)
	if err != nil {
		t.Fatalf("WaitForResult() error = %v", err)
	}

	if result.Delivery.Status != DeliveryStatusTimeout {
		t.Errorf("status = %v, want DeliveryStatusTimeout", result.Delivery.Status)
	}
	if !strings.Contains(result.Delivery.Error, "timeout") {
		t.Errorf("error = %q, want to contain 'timeout'", result.Delivery.Error)
	}
}

func TestTrigger_TimeoutWithRetry(t *testing.T) {
	mc := newMockHTTPClient()
	url := "https://example.com/timeout-retry"

	mc.transport.delay[url] = 500 * time.Millisecond
	mc.transport.removeDelayAfter[url] = 2
	mc.transport.handlers[url] = func(req *http.Request) (*http.Response, error) {
		return okResponse()
	}

	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	policy := RetryPolicy{
		MaxRetries:  3,
		Interval:    30 * time.Millisecond,
		BackoffType: BackoffFixed,
	}
	id, _ := s.Register(url, "POST",
		WithTimeout(80*time.Millisecond),
		WithRetryPolicy(policy),
	)
	s.Trigger(id)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := s.WaitForResult(ctx, id)
	if err != nil {
		t.Fatalf("WaitForResult() error = %v", err)
	}

	attempts := mc.transport.getCallCount(url)
	if attempts < 3 {
		t.Errorf("attempts = %d, want at least 3", attempts)
	}
	if result.Delivery.Status != DeliveryStatusSucceeded {
		t.Errorf("final status = %v, want success", result.Delivery.Status)
	}
}

func TestTrigger_CallbackNotFound(t *testing.T) {
	mc := newMockHTTPClient()
	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	err := s.Trigger("nonexistent-id")
	if err != ErrCallbackNotFound {
		t.Errorf("Trigger() error = %v, want ErrCallbackNotFound", err)
	}
}

func TestTrigger_SchedulerStopped(t *testing.T) {
	mc := newMockHTTPClient()
	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})

	err := s.Trigger("any-id")
	if err != ErrCallbackNotFound {
		t.Errorf("Trigger() error = %v, want ErrCallbackNotFound", err)
	}
}

func TestCancel(t *testing.T) {
	mc := newMockHTTPClient()
	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	id, _ := s.Register("https://example.com/cancel", "POST")

	t.Run("cancel existing", func(t *testing.T) {
		err := s.Cancel(id)
		if err != nil {
			t.Fatalf("Cancel() error = %v", err)
		}
		cb, _ := s.GetCallback(id)
		if cb.Status != CallbackStatusCancelled {
			t.Errorf("status = %v, want CallbackStatusCancelled", cb.Status)
		}
	})

	t.Run("cancel nonexistent", func(t *testing.T) {
		err := s.Cancel("no-id")
		if err != ErrCallbackNotFound {
			t.Errorf("Cancel() error = %v, want ErrCallbackNotFound", err)
		}
	})
}

func TestGetCallback_NotFound(t *testing.T) {
	mc := newMockHTTPClient()
	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	_, err := s.GetCallback("missing")
	if err != ErrCallbackNotFound {
		t.Errorf("GetCallback() error = %v, want ErrCallbackNotFound", err)
	}
}

func TestGetDeliveries(t *testing.T) {
	mc := newMockHTTPClient()
	url := "https://example.com/deliveries"
	var mu sync.Mutex
	attempts := 0
	mc.transport.handlers[url] = func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		attempts++
		cur := attempts
		mu.Unlock()
		if cur < 2 {
			return error500Response()
		}
		return okResponse()
	}

	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	policy := RetryPolicy{
		MaxRetries:  2,
		Interval:    30 * time.Millisecond,
		BackoffType: BackoffFixed,
	}
	id, _ := s.Register(url, "POST", WithRetryPolicy(policy))
	s.Trigger(id)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.WaitForResult(ctx, id)

	dels, err := s.GetDeliveries(id)
	if err != nil {
		t.Fatalf("GetDeliveries() error = %v", err)
	}
	if len(dels) != 2 {
		t.Errorf("deliveries count = %d, want 2", len(dels))
	}
	if dels[0].Attempt != 1 {
		t.Errorf("first delivery attempt = %d, want 1", dels[0].Attempt)
	}
	if dels[len(dels)-1].Status != DeliveryStatusSucceeded {
		t.Errorf("last delivery status = %v, want success", dels[len(dels)-1].Status)
	}
}

func TestGetDeliveries_NotFound(t *testing.T) {
	mc := newMockHTTPClient()
	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	_, err := s.GetDeliveries("missing-id")
	if err != ErrCallbackNotFound {
		t.Errorf("GetDeliveries() error = %v, want ErrCallbackNotFound", err)
	}
}

func TestWaitForResult_Cancelled(t *testing.T) {
	mc := newMockHTTPClient()
	url := "https://example.com/long"
	mc.transport.delay[url] = 500 * time.Millisecond
	mc.transport.handlers[url] = func(req *http.Request) (*http.Response, error) {
		return okResponse()
	}

	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	id, _ := s.Register(url, "POST", WithTimeout(500*time.Millisecond))
	s.Trigger(id)
	time.Sleep(10 * time.Millisecond)
	s.Cancel(id)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := s.WaitForResult(ctx, id)
	if err != nil {
		t.Fatalf("WaitForResult() error = %v", err)
	}
	if !result.Final {
		t.Error("result should be final after cancel")
	}
}

func TestWaitForResult_ContextTimeout(t *testing.T) {
	mc := newMockHTTPClient()
	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	id, _ := s.Register("https://example.com/pending", "POST")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := s.WaitForResult(ctx, id)
	if err != context.DeadlineExceeded {
		t.Errorf("WaitForResult() error = %v, want DeadlineExceeded", err)
	}
}

func TestConcurrentTriggers(t *testing.T) {
	mc := newMockHTTPClient()
	url := "https://example.com/concurrent"
	mc.transport.handlers[url] = func(req *http.Request) (*http.Response, error) {
		return okResponse()
	}

	s := NewScheduler(SchedulerConfig{WorkerCount: 10, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	const n = 50
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		id, err := s.Register(fmt.Sprintf("%s/%d", url, i), "POST")
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		ids[i] = id
	}

	var wg sync.WaitGroup
	var successCount int64
	wg.Add(n)
	for _, id := range ids {
		go func(cbID string) {
			defer wg.Done()
			if err := s.Trigger(cbID); err != nil {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			r, err := s.WaitForResult(ctx, cbID)
			if err == nil && r.Delivery != nil && r.Delivery.Status == DeliveryStatusSucceeded {
				atomic.AddInt64(&successCount, 1)
			}
		}(id)
	}
	wg.Wait()

	if successCount != n {
		t.Errorf("successCount = %d, want %d", successCount, n)
	}
}

func TestMultipleTriggers_SameCallback(t *testing.T) {
	mc := newMockHTTPClient()
	url := "https://example.com/multi"
	var mu sync.Mutex
	requests := 0
	mc.transport.handlers[url] = func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		requests++
		mu.Unlock()
		return okResponse()
	}

	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	id, _ := s.Register(url, "POST")
	for i := 0; i < 3; i++ {
		if err := s.Trigger(id); err != nil {
			t.Fatalf("Trigger() error = %v", err)
		}
	}

	time.Sleep(2 * time.Second)

	mu.Lock()
	defer mu.Unlock()
	if requests < 3 {
		t.Errorf("requests = %d, want at least 3", requests)
	}
}

func TestCallbackCount(t *testing.T) {
	mc := newMockHTTPClient()
	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	if c := s.CallbackCount(); c != 0 {
		t.Errorf("initial count = %d, want 0", c)
	}

	for i := 0; i < 5; i++ {
		s.Register(fmt.Sprintf("https://example.com/%d", i), "POST")
	}

	if c := s.CallbackCount(); c != 5 {
		t.Errorf("after register count = %d, want 5", c)
	}
}

func TestPendingCount(t *testing.T) {
	mc := newMockHTTPClient()
	url := "https://example.com/pending-count"
	mc.transport.handlers[url] = func(req *http.Request) (*http.Response, error) {
		time.Sleep(100 * time.Millisecond)
		return okResponse()
	}

	s := NewScheduler(SchedulerConfig{WorkerCount: 1, HTTPClient: mc})
	s.Start()
	defer s.Stop()

	id1, _ := s.Register(url, "POST")
	id2, _ := s.Register(url, "POST")

	s.Trigger(id1)
	time.Sleep(10 * time.Millisecond)
	s.Trigger(id2)

	time.Sleep(50 * time.Millisecond)
	pending := s.PendingCount()
	if pending < 1 {
		t.Errorf("PendingCount = %d, want at least 1", pending)
	}
}

func TestStop_Idempotent(t *testing.T) {
	mc := newMockHTTPClient()
	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	s.Stop()
	s.Stop()
}

func TestStart_Idempotent(t *testing.T) {
	mc := newMockHTTPClient()
	s := NewScheduler(SchedulerConfig{WorkerCount: 2, HTTPClient: mc})
	s.Start()
	s.Start()
	defer s.Stop()
}

func TestDeliveryHeap(t *testing.T) {
	h := &deliveryHeap{}
	cb1 := &Callback{ID: "cb1"}
	cb2 := &Callback{ID: "cb2"}
	cb3 := &Callback{ID: "cb3"}

	heap.Push(h, &pendingDelivery{callback: cb1, scheduledAt: time.Now().Add(3 * time.Second)})
	heap.Push(h, &pendingDelivery{callback: cb2, scheduledAt: time.Now().Add(1 * time.Second)})
	heap.Push(h, &pendingDelivery{callback: cb3, scheduledAt: time.Now().Add(2 * time.Second)})

	if h.Len() != 3 {
		t.Fatalf("heap len = %d, want 3", h.Len())
	}

	first := heap.Pop(h).(*pendingDelivery)
	if first.callback.ID != "cb2" {
		t.Errorf("first pop = %s, want cb2", first.callback.ID)
	}

	second := heap.Pop(h).(*pendingDelivery)
	if second.callback.ID != "cb3" {
		t.Errorf("second pop = %s, want cb3", second.callback.ID)
	}

	third := heap.Pop(h).(*pendingDelivery)
	if third.callback.ID != "cb1" {
		t.Errorf("third pop = %s, want cb1", third.callback.ID)
	}
}
