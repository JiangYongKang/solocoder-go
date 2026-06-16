package retry

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var errTestRetryable = errors.New("test retryable error")
var errTestNonRetryable = errors.New("test non-retryable error")

func TestNewRetryer_DefaultConfig(t *testing.T) {
	r := NewRetryer()
	cfg := r.Config()

	if cfg.InitialInterval != 100*time.Millisecond {
		t.Errorf("expected InitialInterval 100ms, got %v", cfg.InitialInterval)
	}
	if cfg.MaxInterval != 10*time.Second {
		t.Errorf("expected MaxInterval 10s, got %v", cfg.MaxInterval)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", cfg.MaxRetries)
	}
	if cfg.JitterFactor != 0.1 {
		t.Errorf("expected JitterFactor 0.1, got %f", cfg.JitterFactor)
	}
	if cfg.IsRetryable == nil {
		t.Error("expected IsRetryable not nil")
	}
}

func TestNewRetryerWithConfig_InvalidValuesNormalized(t *testing.T) {
	cfg := Config{
		InitialInterval: -1 * time.Second,
		MaxInterval:     -1 * time.Second,
		MaxRetries:      -5,
		JitterFactor:    -0.5,
	}
	r := NewRetryerWithConfig(cfg)
	rcfg := r.Config()

	if rcfg.InitialInterval <= 0 {
		t.Errorf("expected positive InitialInterval, got %v", rcfg.InitialInterval)
	}
	if rcfg.MaxInterval < rcfg.InitialInterval {
		t.Errorf("expected MaxInterval >= InitialInterval, got MaxInterval=%v InitialInterval=%v",
			rcfg.MaxInterval, rcfg.InitialInterval)
	}
	if rcfg.MaxRetries != 0 {
		t.Errorf("expected MaxRetries 0, got %d", rcfg.MaxRetries)
	}
	if rcfg.JitterFactor != 0 {
		t.Errorf("expected JitterFactor 0, got %f", rcfg.JitterFactor)
	}
}

func TestNewRetryerWithConfig_JitterFactorClamped(t *testing.T) {
	cfg := Config{JitterFactor: 1.5}
	r := NewRetryerWithConfig(cfg)
	if r.Config().JitterFactor != 1.0 {
		t.Errorf("expected JitterFactor clamped to 1.0, got %f", r.Config().JitterFactor)
	}
}

func TestNewRetryerWithConfig_MaxIntervalLessThanInitial(t *testing.T) {
	cfg := Config{
		InitialInterval: 500 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
	}
	r := NewRetryerWithConfig(cfg)
	if r.Config().MaxInterval != 500*time.Millisecond {
		t.Errorf("expected MaxInterval adjusted to InitialInterval 500ms, got %v", r.Config().MaxInterval)
	}
}

func TestRetryer_Do_FirstTrySuccess(t *testing.T) {
	r := NewRetryerWithConfig(Config{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		MaxRetries:      3,
		JitterFactor:    0,
		IsRetryable:     func(err error) bool { return true },
	})

	var calls int
	err := r.Do(context.Background(), func(ctx context.Context) error {
		calls++
		return nil
	})

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
	if r.Attempts() != 0 {
		t.Errorf("expected 0 attempts (no failures), got %d", r.Attempts())
	}
	if len(r.Errors()) != 0 {
		t.Errorf("expected 0 errors, got %d", len(r.Errors()))
	}
}

func TestRetryer_Do_RetryThenSuccess(t *testing.T) {
	r := NewRetryerWithConfig(Config{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		MaxRetries:      3,
		JitterFactor:    0,
		IsRetryable:     func(err error) bool { return true },
	})

	var calls int
	start := time.Now()
	err := r.Do(context.Background(), func(ctx context.Context) error {
		calls++
		if calls < 3 {
			return errTestRetryable
		}
		return nil
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
	if r.Attempts() != 2 {
		t.Errorf("expected 2 failed attempts, got %d", r.Attempts())
	}
	if len(r.Errors()) != 2 {
		t.Errorf("expected 2 errors, got %d", len(r.Errors()))
	}
	minExpected := 10*time.Millisecond + 20*time.Millisecond
	if elapsed < minExpected {
		t.Errorf("expected elapsed >= %v, got %v", minExpected, elapsed)
	}
}

func TestRetryer_Do_MaxRetriesExceeded(t *testing.T) {
	cfg := Config{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     50 * time.Millisecond,
		MaxRetries:      2,
		JitterFactor:    0,
		IsRetryable:     func(err error) bool { return true },
	}
	r := NewRetryerWithConfig(cfg)

	var calls int
	err := r.Do(context.Background(), func(ctx context.Context) error {
		calls++
		return errTestRetryable
	})

	if calls != 3 {
		t.Errorf("expected 3 calls (1 initial + 2 retries), got %d", calls)
	}
	if r.Attempts() != 3 {
		t.Errorf("expected 3 failed attempts, got %d", r.Attempts())
	}
	if len(r.Errors()) != 3 {
		t.Errorf("expected 3 errors, got %d", len(r.Errors()))
	}

	var aggErr *AggregateError
	if !errors.As(err, &aggErr) {
		t.Fatalf("expected AggregateError, got %T", err)
	}
	if len(aggErr.Errors) != 3 {
		t.Errorf("expected 3 errors in aggregate, got %d", len(aggErr.Errors))
	}
	for i, e := range aggErr.Errors {
		if !errors.Is(e, errTestRetryable) {
			t.Errorf("error[%d]: expected errTestRetryable, got %v", i, e)
		}
	}
}

func TestRetryer_Do_MaxRetriesZero(t *testing.T) {
	cfg := Config{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     50 * time.Millisecond,
		MaxRetries:      0,
		JitterFactor:    0,
		IsRetryable:     func(err error) bool { return true },
	}
	r := NewRetryerWithConfig(cfg)

	var calls int
	err := r.Do(context.Background(), func(ctx context.Context) error {
		calls++
		return errTestRetryable
	})

	if calls != 1 {
		t.Errorf("expected 1 call (no retries), got %d", calls)
	}
	if r.Attempts() != 1 {
		t.Errorf("expected 1 attempt, got %d", r.Attempts())
	}
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestRetryer_Do_NonRetryableError(t *testing.T) {
	cfg := Config{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		MaxRetries:      5,
		JitterFactor:    0,
		IsRetryable: func(err error) bool {
			return !errors.Is(err, errTestNonRetryable)
		},
	}
	r := NewRetryerWithConfig(cfg)

	var calls int
	err := r.Do(context.Background(), func(ctx context.Context) error {
		calls++
		if calls == 2 {
			return errTestNonRetryable
		}
		return errTestRetryable
	})

	if calls != 2 {
		t.Errorf("expected 2 calls (stopped by non-retryable), got %d", calls)
	}
	if r.Attempts() != 2 {
		t.Errorf("expected 2 attempts, got %d", r.Attempts())
	}

	var aggErr *AggregateError
	if !errors.As(err, &aggErr) {
		t.Fatalf("expected AggregateError, got %T", err)
	}
	if len(aggErr.Errors) != 2 {
		t.Errorf("expected 2 errors in aggregate, got %d", len(aggErr.Errors))
	}
	if !errors.Is(aggErr.Errors[1], errTestNonRetryable) {
		t.Errorf("last error should be non-retryable, got %v", aggErr.Errors[1])
	}
}

func TestDefaultIsRetryable(t *testing.T) {
	if DefaultIsRetryable(nil) {
		t.Error("nil error should not be retryable")
	}
	if !DefaultIsRetryable(context.DeadlineExceeded) {
		t.Error("DeadlineExceeded should be retryable")
	}
	if DefaultIsRetryable(context.Canceled) {
		t.Error("Canceled should not be retryable")
	}
	if !DefaultIsRetryable(errors.New("some error")) {
		t.Error("generic error should be retryable by default")
	}
}

func TestRetryer_Do_ContextCanceled(t *testing.T) {
	r := NewRetryerWithConfig(Config{
		InitialInterval: 500 * time.Millisecond,
		MaxInterval:     1 * time.Second,
		MaxRetries:      10,
		JitterFactor:    0,
		IsRetryable:     func(err error) bool { return true },
	})

	ctx, cancel := context.WithCancel(context.Background())

	var calls int32
	done := make(chan error, 1)

	go func() {
		done <- r.Do(ctx, func(ctx context.Context) error {
			atomic.AddInt32(&calls, 1)
			return errTestRetryable
		})
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Do did not return after context cancellation")
	}
}

func TestRetryer_Do_ContextDeadlineExceeded(t *testing.T) {
	r := NewRetryerWithConfig(Config{
		InitialInterval: 500 * time.Millisecond,
		MaxInterval:     1 * time.Second,
		MaxRetries:      10,
		JitterFactor:    0,
		IsRetryable:     func(err error) bool { return true },
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var calls int32
	done := make(chan error, 1)

	go func() {
		done <- r.Do(ctx, func(ctx context.Context) error {
			atomic.AddInt32(&calls, 1)
			return errTestRetryable
		})
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context.DeadlineExceeded, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Do did not return after context deadline")
	}
}

func TestRetryer_Do_ContextAlreadyCanceled(t *testing.T) {
	r := NewRetryer()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var called bool
	err := r.Do(ctx, func(ctx context.Context) error {
		called = true
		return nil
	})

	if called {
		t.Error("function should not be called with canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestRetryer_nextInterval_ExponentialGrowth(t *testing.T) {
	r := NewRetryerWithConfig(Config{
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     10 * time.Second,
		MaxRetries:      10,
		JitterFactor:    0,
	})

	expected := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		1600 * time.Millisecond,
		3200 * time.Millisecond,
		6400 * time.Millisecond,
		10 * time.Second,
		10 * time.Second,
		10 * time.Second,
	}

	for i, want := range expected {
		got := r.nextInterval(i)
		if got != want {
			t.Errorf("attempt %d: expected %v, got %v", i, want, got)
		}
	}
}

func TestRetryer_nextInterval_MaxIntervalCap(t *testing.T) {
	r := NewRetryerWithConfig(Config{
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     500 * time.Millisecond,
		MaxRetries:      10,
		JitterFactor:    0,
	})

	for attempt := 0; attempt < 20; attempt++ {
		interval := r.nextInterval(attempt)
		if interval > 500*time.Millisecond {
			t.Errorf("attempt %d: interval %v exceeds MaxInterval 500ms", attempt, interval)
		}
	}
}

func TestRetryer_nextInterval_JitterInRange(t *testing.T) {
	r := NewRetryerWithConfig(Config{
		InitialInterval: 1 * time.Second,
		MaxInterval:     10 * time.Second,
		MaxRetries:      5,
		JitterFactor:    0.2,
	})

	baseInterval := 1 * time.Second
	jitterRange := float64(baseInterval) * 0.2
	minInterval := time.Duration(float64(baseInterval) - jitterRange)
	maxInterval := time.Duration(float64(baseInterval) + jitterRange)

	samples := 1000
	for i := 0; i < samples; i++ {
		got := r.nextInterval(0)
		if got < minInterval || got > maxInterval {
			t.Fatalf("sample %d: interval %v outside expected range [%v, %v]",
				i, got, minInterval, maxInterval)
		}
	}
}

func TestRetryer_nextInterval_JitterZero(t *testing.T) {
	r := NewRetryerWithConfig(Config{
		InitialInterval: 500 * time.Millisecond,
		MaxInterval:     10 * time.Second,
		MaxRetries:      5,
		JitterFactor:    0,
	})

	for i := 0; i < 10; i++ {
		got := r.nextInterval(0)
		if got != 500*time.Millisecond {
			t.Errorf("iteration %d: expected exactly 500ms, got %v", i, got)
		}
	}
}

func TestRetryer_Do_OnRetryBeforeCallback(t *testing.T) {
	var beforeAttempts []int
	var beforeErrors []error

	cfg := Config{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		MaxRetries:      3,
		JitterFactor:    0,
		IsRetryable:     func(err error) bool { return true },
		OnRetryBefore: func(attempt int, err error) {
			beforeAttempts = append(beforeAttempts, attempt)
			beforeErrors = append(beforeErrors, err)
		},
	}
	r := NewRetryerWithConfig(cfg)

	var calls int
	r.Do(context.Background(), func(ctx context.Context) error {
		calls++
		if calls < 4 {
			return fmt.Errorf("error %d", calls)
		}
		return nil
	})

	if len(beforeAttempts) != 3 {
		t.Fatalf("expected 3 OnRetryBefore calls, got %d", len(beforeAttempts))
	}
	expectedAttempts := []int{1, 2, 3}
	for i, want := range expectedAttempts {
		if beforeAttempts[i] != want {
			t.Errorf("OnRetryBefore[%d]: expected attempt %d, got %d", i, want, beforeAttempts[i])
		}
	}
}

func TestRetryer_Do_OnRetryAfterCallback(t *testing.T) {
	var afterAttempts []int
	var afterErrors []error

	cfg := Config{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		MaxRetries:      2,
		JitterFactor:    0,
		IsRetryable:     func(err error) bool { return true },
		OnRetryAfter: func(attempt int, err error) {
			afterAttempts = append(afterAttempts, attempt)
			afterErrors = append(afterErrors, err)
		},
	}
	r := NewRetryerWithConfig(cfg)

	var calls int
	r.Do(context.Background(), func(ctx context.Context) error {
		calls++
		return fmt.Errorf("error %d", calls)
	})

	if len(afterAttempts) != 2 {
		t.Fatalf("expected 2 OnRetryAfter calls (not after final failure), got %d", len(afterAttempts))
	}
	expectedAttempts := []int{1, 2}
	for i, want := range expectedAttempts {
		if afterAttempts[i] != want {
			t.Errorf("OnRetryAfter[%d]: expected attempt %d, got %d", i, want, afterAttempts[i])
		}
	}
}

func TestRetryer_Do_CallbackPanicDoesNotStopRetry(t *testing.T) {
	cfg := Config{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		MaxRetries:      3,
		JitterFactor:    0,
		IsRetryable:     func(err error) bool { return true },
		OnRetryBefore: func(attempt int, err error) {
			panic("intentional panic in before callback")
		},
		OnRetryAfter: func(attempt int, err error) {
			panic("intentional panic in after callback")
		},
	}
	r := NewRetryerWithConfig(cfg)

	var calls int
	err := r.Do(context.Background(), func(ctx context.Context) error {
		calls++
		if calls < 2 {
			return errTestRetryable
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected nil error (callback panic should not affect retry), got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestAggregateError_ErrorSingle(t *testing.T) {
	agg := &AggregateError{Errors: []error{errTestRetryable}}
	want := errTestRetryable.Error()
	if agg.Error() != want {
		t.Errorf("expected %q, got %q", want, agg.Error())
	}
}

func TestAggregateError_ErrorMultiple(t *testing.T) {
	e1 := errors.New("err1")
	e2 := errors.New("err2")
	agg := &AggregateError{Errors: []error{e1, e2}}
	msg := agg.Error()
	if msg == e1.Error() {
		t.Error("multi-error should not return single error message")
	}
	if len(msg) == 0 {
		t.Error("error message should not be empty")
	}
}

func TestAggregateError_ErrorEmpty(t *testing.T) {
	agg := &AggregateError{Errors: []error{}}
	if agg.Error() == "" {
		t.Error("empty AggregateError should return non-empty message")
	}
}

func TestAggregateError_Unwrap(t *testing.T) {
	errs := []error{errors.New("a"), errors.New("b")}
	agg := &AggregateError{Errors: errs}
	unwrapped := agg.Unwrap()
	if len(unwrapped) != 2 {
		t.Fatalf("expected 2 unwrapped errors, got %d", len(unwrapped))
	}
	for i, e := range unwrapped {
		if e != errs[i] {
			t.Errorf("unwrapped[%d] mismatch", i)
		}
	}
}

func TestRetryer_ErrorsReturnsCopy(t *testing.T) {
	r := NewRetryerWithConfig(Config{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		MaxRetries:      2,
		JitterFactor:    0,
		IsRetryable:     func(err error) bool { return true },
	})

	r.Do(context.Background(), func(ctx context.Context) error {
		return errTestRetryable
	})

	errs1 := r.Errors()
	errs2 := r.Errors()

	errs1[0] = errors.New("modified")

	if errors.Is(errs2[0], errors.New("modified")) {
		t.Error("Errors() should return a copy, modifications should not affect internal state")
	}
	if !errors.Is(r.Errors()[0], errTestRetryable) {
		t.Error("internal errors should remain unchanged")
	}
}

func TestDo_ConvenienceFunction(t *testing.T) {
	var calls int
	err := Do(context.Background(),
		func(ctx context.Context) error {
			calls++
			if calls < 2 {
				return errTestRetryable
			}
			return nil
		},
		WithInitialInterval(10*time.Millisecond),
		WithMaxInterval(100*time.Millisecond),
		WithMaxRetries(3),
		WithJitterFactor(0),
	)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestDo_ConvenienceFunction_NonRetryable(t *testing.T) {
	err := Do(context.Background(),
		func(ctx context.Context) error {
			return errTestNonRetryable
		},
		WithMaxRetries(5),
		WithIsRetryable(func(err error) bool {
			return !errors.Is(err, errTestNonRetryable)
		}),
	)

	var aggErr *AggregateError
	if !errors.As(err, &aggErr) {
		t.Fatalf("expected AggregateError, got %T", err)
	}
	if len(aggErr.Errors) != 1 {
		t.Errorf("expected 1 error (stopped by non-retryable), got %d", len(aggErr.Errors))
	}
}

func TestRetryer_Do_Concurrent(t *testing.T) {
	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	var successCount int32

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			r := NewRetryerWithConfig(Config{
				InitialInterval: 5 * time.Millisecond,
				MaxInterval:     20 * time.Millisecond,
				MaxRetries:      3,
				JitterFactor:    0,
				IsRetryable:     func(err error) bool { return true },
			})

			var localCalls int
			err := r.Do(context.Background(), func(ctx context.Context) error {
				localCalls++
				if localCalls <= id%2 {
					return errTestRetryable
				}
				return nil
			})

			if err == nil {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	if successCount != goroutines {
		t.Errorf("expected %d successes, got %d", goroutines, successCount)
	}
}

func TestRetryer_Do_ResetStateBetweenCalls(t *testing.T) {
	r := NewRetryerWithConfig(Config{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		MaxRetries:      3,
		JitterFactor:    0,
		IsRetryable:     func(err error) bool { return true },
	})

	r.Do(context.Background(), func(ctx context.Context) error {
		return errTestRetryable
	})
	if r.Attempts() != 4 {
		t.Fatalf("precondition: expected 4 attempts after first Do, got %d", r.Attempts())
	}

	var calls int
	err := r.Do(context.Background(), func(ctx context.Context) error {
		calls++
		return nil
	})

	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
	if r.Attempts() != 0 {
		t.Errorf("expected attempts reset to 0, got %d", r.Attempts())
	}
	if len(r.Errors()) != 0 {
		t.Errorf("expected errors reset to empty, got %d", len(r.Errors()))
	}
}

func TestDo_WithAllOptions(t *testing.T) {
	var beforeCalled, afterCalled bool
	var isRetryableCalled bool

	err := Do(context.Background(),
		func(ctx context.Context) error {
			return nil
		},
		WithInitialInterval(50*time.Millisecond),
		WithMaxInterval(500*time.Millisecond),
		WithMaxRetries(5),
		WithJitterFactor(0.15),
		WithIsRetryable(func(err error) bool {
			isRetryableCalled = true
			return true
		}),
		WithOnRetryBefore(func(attempt int, err error) {
			beforeCalled = true
		}),
		WithOnRetryAfter(func(attempt int, err error) {
			afterCalled = true
		}),
	)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	_ = isRetryableCalled
	_ = beforeCalled
	_ = afterCalled
}

func TestRetryer_Do_TimingExponentialBackoff(t *testing.T) {
	r := NewRetryerWithConfig(Config{
		InitialInterval: 20 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		MaxRetries:      4,
		JitterFactor:    0,
		IsRetryable:     func(err error) bool { return true },
	})

	var calls int
	start := time.Now()
	r.Do(context.Background(), func(ctx context.Context) error {
		calls++
		return errTestRetryable
	})
	elapsed := time.Since(start)

	expectedMin := 20*time.Millisecond + 40*time.Millisecond + 80*time.Millisecond + 100*time.Millisecond
	if elapsed < expectedMin {
		t.Errorf("expected elapsed >= %v, got %v", expectedMin, elapsed)
	}
}

func TestRetryer_nextInterval_FirstAttemptIsInitial(t *testing.T) {
	r := NewRetryerWithConfig(Config{
		InitialInterval: 250 * time.Millisecond,
		MaxInterval:     10 * time.Second,
		JitterFactor:    0,
	})

	got := r.nextInterval(0)
	if got != 250*time.Millisecond {
		t.Errorf("first interval (attempt 0) should be InitialInterval 250ms, got %v", got)
	}
}

func TestDo_WithContextCanceledDuringSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- Do(ctx,
			func(ctx context.Context) error {
				return errTestRetryable
			},
			WithInitialInterval(500*time.Millisecond),
			WithMaxRetries(10),
			WithJitterFactor(0),
		)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Do did not return promptly after cancellation")
	}
}
