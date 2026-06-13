package deadletter

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func makeSuccessHandler() MessageHandler {
	return func(ctx context.Context, msg *DeadLetterMessage) error {
		return nil
	}
}

func makeFailHandler(maxFails int) MessageHandler {
	var mu sync.Mutex
	failCount := make(map[string]int)
	return func(ctx context.Context, msg *DeadLetterMessage) error {
		mu.Lock()
		defer mu.Unlock()
		failCount[msg.ID]++
		if failCount[msg.ID] <= maxFails {
			return fmt.Errorf("fail #%d for %s", failCount[msg.ID], msg.ID)
		}
		return nil
	}
}

func makeAlwaysFailHandler() MessageHandler {
	return func(ctx context.Context, msg *DeadLetterMessage) error {
		return errors.New("always fail")
	}
}

// ------------------------------ Config Validation Tests ------------------------------

func TestNewProcessor_InvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "negative MaxRetries",
			cfg: Config{
				MaxRetries: -1,
				DelayStrategy: DelayStrategy{
					Type: DelayStrategyFixed,
					Base: 100 * time.Millisecond,
					Max:  time.Second,
				},
			},
			wantErr: true,
		},
		{
			name: "negative AlertThreshold",
			cfg: Config{
				MaxRetries:     3,
				AlertThreshold: -1,
				DelayStrategy: DelayStrategy{
					Type: DelayStrategyFixed,
					Base: 100 * time.Millisecond,
					Max:  time.Second,
				},
			},
			wantErr: true,
		},
		{
			name: "zero Base delay",
			cfg: Config{
				MaxRetries: 3,
				DelayStrategy: DelayStrategy{
					Type: DelayStrategyFixed,
					Base: 0,
					Max:  time.Second,
				},
			},
			wantErr: true,
		},
		{
			name: "Max delay less than Base",
			cfg: Config{
				MaxRetries: 3,
				DelayStrategy: DelayStrategy{
					Type: DelayStrategyFixed,
					Base: 200 * time.Millisecond,
					Max:  100 * time.Millisecond,
				},
			},
			wantErr: true,
		},
		{
			name: "valid config",
			cfg: Config{
				MaxRetries:     3,
				AlertThreshold: 10,
				DelayStrategy: DelayStrategy{
					Type: DelayStrategyFixed,
					Base: 100 * time.Millisecond,
					Max:  time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "zero MaxRetries",
			cfg: Config{
				MaxRetries: 0,
				DelayStrategy: DelayStrategy{
					Type: DelayStrategyFixed,
					Base: 100 * time.Millisecond,
					Max:  time.Second,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProcessor(tt.cfg)
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ------------------------------ Lifecycle Tests ------------------------------

func TestNewProcessor(t *testing.T) {
	cfg := Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 100 * time.Millisecond,
			Max:  time.Second,
		},
	}
	p, err := NewProcessor(cfg)
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	if p == nil {
		t.Fatal("NewProcessor returned nil")
	}
	if p.PendingCount() != 0 {
		t.Errorf("expected 0 pending messages, got %d", p.PendingCount())
	}
}

func TestStart_HandlerNotSet(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 100 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()

	err = p.Start()
	if !errors.Is(err, ErrHandlerNotSet) {
		t.Errorf("expected ErrHandlerNotSet, got %v", err)
	}
}

func TestStart_Success(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 100 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()

	p.SetHandler(makeSuccessHandler())
	err = p.Start()
	if err != nil {
		t.Errorf("unexpected error on Start: %v", err)
	}

	err = p.Start()
	if !errors.Is(err, ErrAlreadyStarted) {
		t.Errorf("expected ErrAlreadyStarted on second Start, got %v", err)
	}
}

func TestStop_Idempotent(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 100 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	p.SetHandler(makeSuccessHandler())

	p.Stop()
	p.Stop()

	_ = p.Start()
	p.Stop()
	p.Stop()
}

// ------------------------------ MoveToDeadLetter Tests ------------------------------

func TestMoveToDeadLetter_StoppedProcessor(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 100 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	p.SetHandler(makeSuccessHandler())

	_, err = p.MoveToDeadLetter("topic", "payload", "reason", 3)
	if !errors.Is(err, ErrProcessorStopped) {
		t.Errorf("expected ErrProcessorStopped, got %v", err)
	}
}

func TestMoveToDeadLetter_Success(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 100 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeSuccessHandler())
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	before := time.Now()
	id, err := p.MoveToDeadLetter("test-topic", "test-payload", "test-reason", 2)
	if err != nil {
		t.Fatalf("MoveToDeadLetter failed: %v", err)
	}
	after := time.Now()

	if id == "" {
		t.Error("expected non-empty id")
	}

	msg, err := p.GetMessage(id)
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}

	if msg.OriginalTopic != "test-topic" {
		t.Errorf("expected OriginalTopic=test-topic, got %s", msg.OriginalTopic)
	}
	if msg.Payload != "test-payload" {
		t.Errorf("expected Payload=test-payload, got %v", msg.Payload)
	}
	if msg.FailureReason != "test-reason" {
		t.Errorf("expected FailureReason=test-reason, got %s", msg.FailureReason)
	}
	if msg.MaxRetries != 2 {
		t.Errorf("expected MaxRetries=2, got %d", msg.MaxRetries)
	}
	if msg.RetryCount != 0 {
		t.Errorf("expected RetryCount=0, got %d", msg.RetryCount)
	}
	if msg.Status != StatusPending {
		t.Errorf("expected Status=Pending, got %v", msg.Status)
	}
	if msg.TransferTime.Before(before) || msg.TransferTime.After(after) {
		t.Errorf("TransferTime not in expected range: %v", msg.TransferTime)
	}
	if msg.NextRetryAt.Before(msg.TransferTime) {
		t.Errorf("NextRetryAt should be after TransferTime")
	}
}

func TestMoveToDeadLetter_DefaultMaxRetries(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 5,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 100 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeSuccessHandler())
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	id, err := p.MoveToDeadLetter("topic", "payload", "reason", -1)
	if err != nil {
		t.Fatalf("MoveToDeadLetter failed: %v", err)
	}

	msg, err := p.GetMessage(id)
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}
	if msg.MaxRetries != 5 {
		t.Errorf("expected MaxRetries=5 (default), got %d", msg.MaxRetries)
	}
}

func TestMoveToDeadLetter_ZeroMaxRetries(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 10 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeAlwaysFailHandler())
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	id, err := p.MoveToDeadLetter("topic", "payload", "reason", 0)
	if err != nil {
		t.Fatalf("MoveToDeadLetter failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	msg, err := p.GetMessage(id)
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}
	if msg.Status != StatusPermanentlyFailed {
		t.Errorf("expected Status=PermanentlyFailed, got %v", msg.Status)
	}
	if msg.RetryCount != 1 {
		t.Errorf("expected RetryCount=1, got %d", msg.RetryCount)
	}
}

// ------------------------------ Delay Strategy Tests ------------------------------

func TestDelayStrategy_Fixed(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 5,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 100 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeSuccessHandler())
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	id, err := p.MoveToDeadLetter("topic", "payload", "reason", 3)
	if err != nil {
		t.Fatalf("MoveToDeadLetter failed: %v", err)
	}

	msg, err := p.GetMessage(id)
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}

	expectedDelay := 100 * time.Millisecond
	actualDelay := msg.NextRetryAt.Sub(msg.TransferTime)
	if actualDelay < expectedDelay-10*time.Millisecond || actualDelay > expectedDelay+10*time.Millisecond {
		t.Errorf("expected delay ~%v, got %v", expectedDelay, actualDelay)
	}
}

func TestDelayStrategy_Exponential(t *testing.T) {
	var retryDelays []time.Duration
	var mu sync.Mutex
	lastTime := time.Now()

	handler := func(ctx context.Context, msg *DeadLetterMessage) error {
		mu.Lock()
		now := time.Now()
		delay := now.Sub(lastTime)
		retryDelays = append(retryDelays, delay)
		lastTime = now
		mu.Unlock()
		return errors.New("fail")
	}

	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyExponential,
			Base: 10 * time.Millisecond,
			Max:  500 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(handler)
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	_, err = p.MoveToDeadLetter("topic", "payload", "reason", 3)
	if err != nil {
		t.Fatalf("MoveToDeadLetter failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	delays := retryDelays
	mu.Unlock()

	if len(delays) < 4 {
		t.Fatalf("expected at least 4 delays, got %d", len(delays))
	}

	expected := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		40 * time.Millisecond,
		80 * time.Millisecond,
	}

	for i, exp := range expected {
		if i >= len(delays) {
			break
		}
		if delays[i] < exp-5*time.Millisecond || delays[i] > exp+50*time.Millisecond {
			t.Errorf("delay %d: expected ~%v, got %v", i, exp, delays[i])
		}
	}

	for i := 1; i < len(delays) && i < 4; i++ {
		if delays[i] <= delays[i-1] {
			t.Errorf("exponential delays should increase: delay[%d]=%v <= delay[%d]=%v",
				i, delays[i], i-1, delays[i-1])
		}
	}
}

func TestDelayStrategy_Exponential_MaxCap(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 10,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyExponential,
			Base: 10 * time.Millisecond,
			Max:  100 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}

	next1 := p.calculateNextRetry(0)
	next5 := p.calculateNextRetry(5)
	next10 := p.calculateNextRetry(10)

	delay1 := next1.Sub(time.Now())
	delay5 := next5.Sub(time.Now())
	delay10 := next10.Sub(time.Now())

	if delay1 > 100*time.Millisecond {
		t.Errorf("delay1 should be <= 100ms, got %v", delay1)
	}
	if delay5 > 100*time.Millisecond+10*time.Millisecond {
		t.Errorf("delay5 should be capped at ~100ms, got %v", delay5)
	}
	if delay10 > 100*time.Millisecond+10*time.Millisecond {
		t.Errorf("delay10 should be capped at ~100ms, got %v", delay10)
	}
}

// ------------------------------ Retry Processing Tests ------------------------------

func TestProcessMessage_SuccessOnFirstRetry(t *testing.T) {
	var processed int32
	handler := func(ctx context.Context, msg *DeadLetterMessage) error {
		atomic.AddInt32(&processed, 1)
		return nil
	}

	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 10 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(handler)
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	id, err := p.MoveToDeadLetter("topic", "payload", "reason", 3)
	if err != nil {
		t.Fatalf("MoveToDeadLetter failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&processed) != 1 {
		t.Errorf("expected 1 processing, got %d", processed)
	}

	_, err = p.GetMessage(id)
	if !errors.Is(err, ErrMessageNotFound) {
		t.Errorf("expected ErrMessageNotFound after successful processing, got %v", err)
	}

	if p.PendingCount() != 0 {
		t.Errorf("expected 0 pending, got %d", p.PendingCount())
	}
}

func TestProcessMessage_SuccessAfterMultipleRetries(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 10 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeFailHandler(2))
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	id, err := p.MoveToDeadLetter("topic", "payload", "reason", 3)
	if err != nil {
		t.Fatalf("MoveToDeadLetter failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	_, err = p.GetMessage(id)
	if !errors.Is(err, ErrMessageNotFound) {
		t.Errorf("message should be removed after success, got err=%v", err)
	}
}

func TestProcessMessage_PermanentlyFailed(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 2,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 10 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeAlwaysFailHandler())
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	id, err := p.MoveToDeadLetter("topic", "payload", "reason", 2)
	if err != nil {
		t.Fatalf("MoveToDeadLetter failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	msg, err := p.GetMessage(id)
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}

	if msg.Status != StatusPermanentlyFailed {
		t.Errorf("expected Status=PermanentlyFailed, got %v", msg.Status)
	}
	if msg.RetryCount != 3 {
		t.Errorf("expected RetryCount=3, got %d", msg.RetryCount)
	}
	if msg.LastError == "" {
		t.Error("LastError should not be empty")
	}

	if p.PermanentlyFailedCount() != 1 {
		t.Errorf("expected 1 permanently failed, got %d", p.PermanentlyFailedCount())
	}
}

func TestProcessMessage_HandlerPanic(t *testing.T) {
	var firstCall int32
	handler := func(ctx context.Context, msg *DeadLetterMessage) error {
		if atomic.CompareAndSwapInt32(&firstCall, 0, 1) {
			panic("intentional panic")
		}
		return nil
	}

	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 10 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(handler)
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	_, err = p.MoveToDeadLetter("topic", "payload", "reason", 3)
	if err != nil {
		t.Fatalf("MoveToDeadLetter failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if p.PendingCount() != 0 {
		t.Errorf("processor should recover from panic and process successfully, pending=%d", p.PendingCount())
	}
}

// ------------------------------ Manual Retry Tests ------------------------------

func TestRetryMessage_Success(t *testing.T) {
	var processed int32
	handler := func(ctx context.Context, msg *DeadLetterMessage) error {
		atomic.AddInt32(&processed, 1)
		return nil
	}

	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: time.Hour,
			Max:  time.Hour,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(handler)
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	id, err := p.MoveToDeadLetter("topic", "payload", "reason", 3)
	if err != nil {
		t.Fatalf("MoveToDeadLetter failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&processed) != 0 {
		t.Fatal("message should not be processed yet due to long delay")
	}

	err = p.RetryMessage(id)
	if err != nil {
		t.Fatalf("RetryMessage failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if atomic.LoadInt32(&processed) != 1 {
		t.Errorf("expected 1 processing after manual retry, got %d", processed)
	}
}

func TestRetryMessage_NotFound(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 100 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeSuccessHandler())
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	err = p.RetryMessage("nonexistent")
	if !errors.Is(err, ErrMessageNotFound) {
		t.Errorf("expected ErrMessageNotFound, got %v", err)
	}
}

func TestRetryMessage_PermanentlyFailed(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 0,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 10 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeAlwaysFailHandler())
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	id, err := p.MoveToDeadLetter("topic", "payload", "reason", 0)
	if err != nil {
		t.Fatalf("MoveToDeadLetter failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	err = p.RetryMessage(id)
	if !errors.Is(err, ErrMaxRetriesExceeded) {
		t.Errorf("expected ErrMaxRetriesExceeded, got %v", err)
	}
}

// ------------------------------ Alert Tests ------------------------------

func TestAlertThreshold_Exceeded(t *testing.T) {
	var alertCount int32
	var lastAlert AlertInfo
	var mu sync.Mutex

	alertCb := func(info AlertInfo) {
		mu.Lock()
		atomic.AddInt32(&alertCount, 1)
		lastAlert = info
		mu.Unlock()
	}

	p, err := NewProcessor(Config{
		MaxRetries:     3,
		AlertThreshold: 3,
		AlertCallback:  alertCb,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: time.Hour,
			Max:  time.Hour,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeSuccessHandler())
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 0; i < 2; i++ {
		_, err := p.MoveToDeadLetter("topic", "payload", fmt.Sprintf("reason-%d", i%2), 3)
		if err != nil {
			t.Fatalf("MoveToDeadLetter failed: %v", err)
		}
	}

	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&alertCount) != 0 {
		t.Errorf("alert should not fire yet, count=%d", alertCount)
	}

	_, err = p.MoveToDeadLetter("topic", "payload", "reason-0", 3)
	if err != nil {
		t.Fatalf("MoveToDeadLetter failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	alert := lastAlert
	mu.Unlock()

	if atomic.LoadInt32(&alertCount) < 1 {
		t.Fatalf("alert should have fired, count=%d", alertCount)
	}
	if alert.TotalCount < 3 {
		t.Errorf("expected TotalCount >= 3, got %d", alert.TotalCount)
	}
	if alert.Threshold != 3 {
		t.Errorf("expected Threshold=3, got %d", alert.Threshold)
	}
	if alert.ReasonStats["reason-0"] != 2 {
		t.Errorf("expected reason-0 count=2, got %d", alert.ReasonStats["reason-0"])
	}
	if alert.ReasonStats["reason-1"] != 1 {
		t.Errorf("expected reason-1 count=1, got %d", alert.ReasonStats["reason-1"])
	}
	if alert.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestAlertThreshold_NotFiredTwice(t *testing.T) {
	var alertCount int32

	alertCb := func(info AlertInfo) {
		atomic.AddInt32(&alertCount, 1)
	}

	p, err := NewProcessor(Config{
		MaxRetries:     3,
		AlertThreshold: 2,
		AlertCallback:  alertCb,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: time.Hour,
			Max:  time.Hour,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeSuccessHandler())
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		_, err := p.MoveToDeadLetter("topic", "payload", "reason", 3)
		if err != nil {
			t.Fatalf("MoveToDeadLetter failed: %v", err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&alertCount) != 1 {
		t.Errorf("alert should fire exactly once, got %d", alertCount)
	}
}

func TestAlertThreshold_ResetAfterProcessing(t *testing.T) {
	var alertCount int32
	var mu sync.Mutex
	var handler MessageHandler

	handler = func(ctx context.Context, msg *DeadLetterMessage) error {
		return nil
	}

	alertCb := func(info AlertInfo) {
		atomic.AddInt32(&alertCount, 1)
	}

	p, err := NewProcessor(Config{
		MaxRetries:     3,
		AlertThreshold: 2,
		AlertCallback:  alertCb,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 10 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(handler)
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		_, err := p.MoveToDeadLetter("topic", "payload", "reason", 3)
		if err != nil {
			t.Fatalf("MoveToDeadLetter failed: %v", err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	firstAlertCount := atomic.LoadInt32(&alertCount)
	if firstAlertCount < 1 {
		t.Fatalf("alert should have fired at least once, count=%d", firstAlertCount)
	}

	if p.PendingCount() != 0 {
		t.Fatalf("all messages should be processed, pending=%d", p.PendingCount())
	}

	mu.Lock()
	p.SetHandler(makeSuccessHandler())
	mu.Unlock()

	for i := 0; i < 2; i++ {
		_, err := p.MoveToDeadLetter("topic", "payload", "reason", 3)
		if err != nil {
			t.Fatalf("MoveToDeadLetter failed: %v", err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&alertCount) <= firstAlertCount {
		t.Errorf("alert should fire again after reset, before=%d, after=%d",
			firstAlertCount, atomic.LoadInt32(&alertCount))
	}
}

func TestAlert_NoCallback(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries:     3,
		AlertThreshold: 1,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: time.Hour,
			Max:  time.Hour,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeSuccessHandler())
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	_, err = p.MoveToDeadLetter("topic", "payload", "reason", 3)
	if err != nil {
		t.Fatalf("MoveToDeadLetter failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
}

func TestAlert_ZeroThreshold(t *testing.T) {
	var alertCount int32
	alertCb := func(info AlertInfo) {
		atomic.AddInt32(&alertCount, 1)
	}

	p, err := NewProcessor(Config{
		MaxRetries:     3,
		AlertThreshold: 0,
		AlertCallback:  alertCb,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: time.Hour,
			Max:  time.Hour,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeSuccessHandler())
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 0; i < 10; i++ {
		_, err := p.MoveToDeadLetter("topic", "payload", "reason", 3)
		if err != nil {
			t.Fatalf("MoveToDeadLetter failed: %v", err)
		}
	}

	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&alertCount) != 0 {
		t.Errorf("alert should not fire with threshold=0, count=%d", alertCount)
	}
}

// ------------------------------ Query & Management Tests ------------------------------

func TestGetMessage_NotFound(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 100 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()

	_, err = p.GetMessage("nonexistent")
	if !errors.Is(err, ErrMessageNotFound) {
		t.Errorf("expected ErrMessageNotFound, got %v", err)
	}
}

func TestGetAllMessages(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: time.Hour,
			Max:  time.Hour,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeSuccessHandler())
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		_, err := p.MoveToDeadLetter("topic", "payload", "reason", 3)
		if err != nil {
			t.Fatalf("MoveToDeadLetter failed: %v", err)
		}
	}

	msgs := p.GetAllMessages()
	if len(msgs) != 5 {
		t.Errorf("expected 5 messages, got %d", len(msgs))
	}
}

func TestGetMessagesByStatus(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 0,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 10 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeAlwaysFailHandler())
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		_, err := p.MoveToDeadLetter("topic", "payload", "reason", 0)
		if err != nil {
			t.Fatalf("MoveToDeadLetter failed: %v", err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	failedMsgs := p.GetMessagesByStatus(StatusPermanentlyFailed)
	if len(failedMsgs) != 3 {
		t.Errorf("expected 3 permanently failed messages, got %d", len(failedMsgs))
	}

	pendingMsgs := p.GetMessagesByStatus(StatusPending)
	if len(pendingMsgs) != 0 {
		t.Errorf("expected 0 pending messages, got %d", len(pendingMsgs))
	}
}

func TestRemoveMessage(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: time.Hour,
			Max:  time.Hour,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeSuccessHandler())
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	id, err := p.MoveToDeadLetter("topic", "payload", "reason", 3)
	if err != nil {
		t.Fatalf("MoveToDeadLetter failed: %v", err)
	}

	if p.PendingCount() != 1 {
		t.Fatalf("expected 1 pending, got %d", p.PendingCount())
	}

	err = p.RemoveMessage(id)
	if err != nil {
		t.Fatalf("RemoveMessage failed: %v", err)
	}

	if p.PendingCount() != 0 {
		t.Errorf("expected 0 pending after remove, got %d", p.PendingCount())
	}

	_, err = p.GetMessage(id)
	if !errors.Is(err, ErrMessageNotFound) {
		t.Errorf("expected ErrMessageNotFound after remove, got %v", err)
	}
}

func TestRemoveMessage_NotFound(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 100 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()

	err = p.RemoveMessage("nonexistent")
	if !errors.Is(err, ErrMessageNotFound) {
		t.Errorf("expected ErrMessageNotFound, got %v", err)
	}
}

func TestClearPermanentlyFailed(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 0,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 10 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeAlwaysFailHandler())
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		_, err := p.MoveToDeadLetter("topic", "payload", "reason", 0)
		if err != nil {
			t.Fatalf("MoveToDeadLetter failed: %v", err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	if p.PermanentlyFailedCount() != 3 {
		t.Fatalf("expected 3 permanently failed, got %d", p.PermanentlyFailedCount())
	}

	cleared := p.ClearPermanentlyFailed()
	if cleared != 3 {
		t.Errorf("expected 3 cleared, got %d", cleared)
	}

	if p.PermanentlyFailedCount() != 0 {
		t.Errorf("expected 0 permanently failed after clear, got %d", p.PermanentlyFailedCount())
	}
}

// ------------------------------ Concurrent Tests ------------------------------

func TestConcurrent_MoveToDeadLetter(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: time.Hour,
			Max:  time.Hour,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeSuccessHandler())
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	var wg sync.WaitGroup
	n := 100

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = p.MoveToDeadLetter("topic", fmt.Sprintf("payload-%d", i), "reason", 3)
		}(i)
	}
	wg.Wait()

	if p.PendingCount() != n {
		t.Errorf("expected %d pending, got %d", n, p.PendingCount())
	}

	msgs := p.GetAllMessages()
	if len(msgs) != n {
		t.Errorf("expected %d messages, got %d", n, len(msgs))
	}

	ids := make(map[string]bool)
	for _, msg := range msgs {
		if ids[msg.ID] {
			t.Errorf("duplicate ID: %s", msg.ID)
		}
		ids[msg.ID] = true
	}
}

func TestConcurrent_ProcessAndQuery(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 10 * time.Millisecond,
			Max:  50 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeFailHandler(1))
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	var wg sync.WaitGroup
	n := 20

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = p.MoveToDeadLetter("topic", fmt.Sprintf("payload-%d", i), "reason", 3)
		}(i)
	}

	go func() {
		for {
			_ = p.PendingCount()
			_ = p.PermanentlyFailedCount()
			_ = p.GetAllMessages()
		}
	}()

	wg.Wait()
	time.Sleep(300 * time.Millisecond)

	if p.PendingCount() != 0 {
		t.Errorf("expected 0 pending after processing, got %d", p.PendingCount())
	}
}

// ------------------------------ Integration Tests ------------------------------

func TestFullFlow_MessageRecovery(t *testing.T) {
	var attempt int32
	handler := func(ctx context.Context, msg *DeadLetterMessage) error {
		n := atomic.AddInt32(&attempt, 1)
		if n < 3 {
			return fmt.Errorf("temporary failure #%d", n)
		}
		return nil
	}

	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 20 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(handler)
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	id, err := p.MoveToDeadLetter("order-events", `{"order_id":123}`, "database timeout", 3)
	if err != nil {
		t.Fatalf("MoveToDeadLetter failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt32(&attempt) != 3 {
		t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempt))
	}

	_, err = p.GetMessage(id)
	if !errors.Is(err, ErrMessageNotFound) {
		t.Errorf("message should be removed after successful processing")
	}

	if p.PendingCount() != 0 {
		t.Errorf("pending count should be 0")
	}
	if p.PermanentlyFailedCount() != 0 {
		t.Errorf("permanently failed count should be 0")
	}
}

func TestFullFlow_PermanentFailure(t *testing.T) {
	p, err := NewProcessor(Config{
		MaxRetries: 2,
		AlertThreshold: 5,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyExponential,
			Base: 10 * time.Millisecond,
			Max:  100 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	defer p.Stop()
	p.SetHandler(makeAlwaysFailHandler())
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	id, err := p.MoveToDeadLetter("payment-events", `{"txn_id":456}`, "invalid payment data", 2)
	if err != nil {
		t.Fatalf("MoveToDeadLetter failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	msg, err := p.GetMessage(id)
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}

	if msg.Status != StatusPermanentlyFailed {
		t.Errorf("expected Status=PermanentlyFailed, got %v", msg.Status)
	}
	if msg.RetryCount != 3 {
		t.Errorf("expected RetryCount=3, got %d", msg.RetryCount)
	}
	if msg.OriginalTopic != "payment-events" {
		t.Errorf("expected OriginalTopic=payment-events, got %s", msg.OriginalTopic)
	}
	if msg.FailureReason != "invalid payment data" {
		t.Errorf("expected FailureReason, got %s", msg.FailureReason)
	}
	if msg.LastError == "" {
		t.Error("LastError should not be empty")
	}
	if msg.TransferTime.IsZero() {
		t.Error("TransferTime should not be zero")
	}

	cleared := p.ClearPermanentlyFailed()
	if cleared != 1 {
		t.Errorf("expected 1 cleared, got %d", cleared)
	}

	if p.PermanentlyFailedCount() != 0 {
		t.Errorf("permanently failed count should be 0 after clear")
	}
}

func TestStop_WaitsForNoRunningTasks(t *testing.T) {
	var started int32
	release := make(chan struct{})

	handler := func(ctx context.Context, msg *DeadLetterMessage) error {
		atomic.StoreInt32(&started, 1)
		<-release
		return nil
	}

	p, err := NewProcessor(Config{
		MaxRetries: 3,
		DelayStrategy: DelayStrategy{
			Type: DelayStrategyFixed,
			Base: 10 * time.Millisecond,
			Max:  time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}
	p.SetHandler(handler)
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	_, err = p.MoveToDeadLetter("topic", "payload", "reason", 3)
	if err != nil {
		t.Fatalf("MoveToDeadLetter failed: %v", err)
	}

	for atomic.LoadInt32(&started) == 0 {
		time.Sleep(10 * time.Millisecond)
	}

	stopped := make(chan struct{})
	go func() {
		p.Stop()
		close(stopped)
	}()

	time.Sleep(50 * time.Millisecond)

	close(release)

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop did not complete in time")
	}
}
