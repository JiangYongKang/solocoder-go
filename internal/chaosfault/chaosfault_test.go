package chaosfault

import (
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewFaultInjector(t *testing.T) {
	fi := NewFaultInjector()
	if fi == nil {
		t.Fatal("NewFaultInjector returned nil")
	}
	if fi.randSrc == nil {
		t.Error("randSrc should not be nil")
	}
	if fi.sleepFunc == nil {
		t.Error("sleepFunc should not be nil")
	}
	if fi.timeNowFunc == nil {
		t.Error("timeNowFunc should not be nil")
	}
}

func TestFaultInjectorOptions(t *testing.T) {
	customRand := rand.New(rand.NewSource(42))
	var sleepCalled bool
	customSleep := func(d time.Duration) {
		sleepCalled = true
	}
	var nowCalled bool
	customNow := func() time.Time {
		nowCalled = true
		return time.Now()
	}

	fi := NewFaultInjector(
		WithRandSource(customRand),
		WithSleepFunc(customSleep),
		WithTimeNowFunc(customNow),
	)

	if fi.randSrc != customRand {
		t.Error("randSrc should be custom rand")
	}

	fi.sleepFunc(10 * time.Millisecond)
	if !sleepCalled {
		t.Error("custom sleepFunc should be called")
	}

	fi.timeNowFunc()
	if !nowCalled {
		t.Error("custom timeNowFunc should be called")
	}
}

func TestSetDelayConfig_Fixed(t *testing.T) {
	fi := NewFaultInjector()
	cfg := DelayConfig{
		Enabled:     true,
		Mode:        DelayModeFixed,
		Fixed:       100 * time.Millisecond,
		TargetRatio: 1.0,
	}
	err := fi.SetDelayConfig(cfg)
	if err != nil {
		t.Fatalf("SetDelayConfig failed: %v", err)
	}

	savedCfg := fi.GetDelayConfig()
	if !savedCfg.Enabled {
		t.Error("delay should be enabled")
	}
	if savedCfg.Mode != DelayModeFixed {
		t.Errorf("mode should be DelayModeFixed, got %v", savedCfg.Mode)
	}
	if savedCfg.Fixed != 100*time.Millisecond {
		t.Errorf("fixed delay should be 100ms, got %v", savedCfg.Fixed)
	}
}

func TestSetDelayConfig_Random(t *testing.T) {
	fi := NewFaultInjector()
	cfg := DelayConfig{
		Enabled:     true,
		Mode:        DelayModeRandom,
		Min:         50 * time.Millisecond,
		Max:         200 * time.Millisecond,
		TargetRatio: 1.0,
	}
	err := fi.SetDelayConfig(cfg)
	if err != nil {
		t.Fatalf("SetDelayConfig failed: %v", err)
	}

	savedCfg := fi.GetDelayConfig()
	if savedCfg.Min != 50*time.Millisecond {
		t.Errorf("min delay should be 50ms, got %v", savedCfg.Min)
	}
	if savedCfg.Max != 200*time.Millisecond {
		t.Errorf("max delay should be 200ms, got %v", savedCfg.Max)
	}
}

func TestSetDelayConfig_Invalid(t *testing.T) {
	fi := NewFaultInjector()

	tests := []struct {
		name string
		cfg  DelayConfig
	}{
		{
			name: "fixed delay zero",
			cfg: DelayConfig{
				Enabled:     true,
				Mode:        DelayModeFixed,
				Fixed:       0,
				TargetRatio: 1.0,
			},
		},
		{
			name: "random delay min zero",
			cfg: DelayConfig{
				Enabled:     true,
				Mode:        DelayModeRandom,
				Min:         0,
				Max:         100 * time.Millisecond,
				TargetRatio: 1.0,
			},
		},
		{
			name: "random delay min >= max",
			cfg: DelayConfig{
				Enabled:     true,
				Mode:        DelayModeRandom,
				Min:         200 * time.Millisecond,
				Max:         100 * time.Millisecond,
				TargetRatio: 1.0,
			},
		},
		{
			name: "invalid target ratio negative",
			cfg: DelayConfig{
				Enabled:     true,
				Mode:        DelayModeFixed,
				Fixed:       100 * time.Millisecond,
				TargetRatio: -0.1,
			},
		},
		{
			name: "invalid target ratio over 1",
			cfg: DelayConfig{
				Enabled:     true,
				Mode:        DelayModeFixed,
				Fixed:       100 * time.Millisecond,
				TargetRatio: 1.5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fi.SetDelayConfig(tt.cfg)
			if err == nil {
				t.Error("expected error but got nil")
			}
		})
	}
}

func TestSetDelayConfig_InvalidTimeWindow(t *testing.T) {
	fi := NewFaultInjector()
	start := time.Now().Add(1 * time.Hour)
	end := time.Now()

	cfg := DelayConfig{
		Enabled:     true,
		Mode:        DelayModeFixed,
		Fixed:       100 * time.Millisecond,
		TargetRatio: 1.0,
		TimeWindow: &TimeWindow{
			StartTime: start,
			EndTime:   end,
		},
	}

	err := fi.SetDelayConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid time window but got nil")
	}
	if !errors.Is(err, ErrInvalidTimeWindow) {
		t.Errorf("expected ErrInvalidTimeWindow, got %v", err)
	}
}

func TestApplyDelay_Fixed(t *testing.T) {
	var slept time.Duration
	fi := NewFaultInjector(
		WithSleepFunc(func(d time.Duration) { slept = d }),
	)

	cfg := DelayConfig{
		Enabled:     true,
		Mode:        DelayModeFixed,
		Fixed:       150 * time.Millisecond,
		TargetRatio: 1.0,
	}
	err := fi.SetDelayConfig(cfg)
	if err != nil {
		t.Fatalf("SetDelayConfig failed: %v", err)
	}

	fi.ApplyDelay()
	if slept != 150*time.Millisecond {
		t.Errorf("expected sleep of 150ms, got %v", slept)
	}
}

func TestApplyDelay_Random(t *testing.T) {
	var slept time.Duration
	customRand := rand.New(rand.NewSource(42))
	fi := NewFaultInjector(
		WithSleepFunc(func(d time.Duration) { slept = d }),
		WithRandSource(customRand),
	)

	minDelay := 50 * time.Millisecond
	maxDelay := 200 * time.Millisecond
	cfg := DelayConfig{
		Enabled:     true,
		Mode:        DelayModeRandom,
		Min:         minDelay,
		Max:         maxDelay,
		TargetRatio: 1.0,
	}
	err := fi.SetDelayConfig(cfg)
	if err != nil {
		t.Fatalf("SetDelayConfig failed: %v", err)
	}

	for i := 0; i < 100; i++ {
		slept = 0
		fi.ApplyDelay()
		if slept < minDelay || slept > maxDelay {
			t.Errorf("delay %v is out of range [%v, %v]", slept, minDelay, maxDelay)
		}
	}
}

func TestApplyDelay_Disabled(t *testing.T) {
	var slept time.Duration
	fi := NewFaultInjector(
		WithSleepFunc(func(d time.Duration) { slept = d }),
	)

	cfg := DelayConfig{
		Enabled:     false,
		Mode:        DelayModeFixed,
		Fixed:       100 * time.Millisecond,
		TargetRatio: 1.0,
	}
	err := fi.SetDelayConfig(cfg)
	if err != nil {
		t.Fatalf("SetDelayConfig failed: %v", err)
	}

	fi.ApplyDelay()
	if slept != 0 {
		t.Errorf("expected no sleep when disabled, got %v", slept)
	}
}

func TestSetErrorConfig(t *testing.T) {
	fi := NewFaultInjector()
	customErr := errors.New("custom test error")

	cfg := ErrorConfig{
		Enabled:     true,
		Err:         customErr,
		Message:     "test message",
		TargetRatio: 1.0,
	}
	err := fi.SetErrorConfig(cfg)
	if err != nil {
		t.Fatalf("SetErrorConfig failed: %v", err)
	}

	savedCfg := fi.GetErrorConfig()
	if !savedCfg.Enabled {
		t.Error("error fault should be enabled")
	}
	if savedCfg.Err != customErr {
		t.Error("Err should match")
	}
	if savedCfg.Message != "test message" {
		t.Errorf("Message should be 'test message', got %s", savedCfg.Message)
	}
}

func TestSetErrorConfig_Invalid(t *testing.T) {
	fi := NewFaultInjector()

	tests := []struct {
		name string
		cfg  ErrorConfig
	}{
		{
			name: "no error and no message",
			cfg: ErrorConfig{
				Enabled:     true,
				TargetRatio: 1.0,
			},
		},
		{
			name: "invalid target ratio",
			cfg: ErrorConfig{
				Enabled:     true,
				Message:     "test",
				TargetRatio: -0.5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fi.SetErrorConfig(tt.cfg)
			if err == nil {
				t.Error("expected error but got nil")
			}
		})
	}
}

func TestCheckError_Enabled(t *testing.T) {
	fi := NewFaultInjector()
	customErr := errors.New("custom error")

	cfg := ErrorConfig{
		Enabled:     true,
		Err:         customErr,
		Message:     "injected error",
		TargetRatio: 1.0,
	}
	err := fi.SetErrorConfig(cfg)
	if err != nil {
		t.Fatalf("SetErrorConfig failed: %v", err)
	}

	result := fi.CheckError()
	if result == nil {
		t.Fatal("expected injected error, got nil")
	}

	var injErr *InjectedError
	if !errors.As(result, &injErr) {
		t.Fatal("error should be *InjectedError")
	}

	if injErr.Message != "injected error" {
		t.Errorf("expected message 'injected error', got '%s'", injErr.Message)
	}

	if !errors.Is(result, customErr) {
		t.Error("error should unwrap to customErr")
	}
}

func TestCheckError_Disabled(t *testing.T) {
	fi := NewFaultInjector()

	cfg := ErrorConfig{
		Enabled:     false,
		Message:     "test",
		TargetRatio: 1.0,
	}
	err := fi.SetErrorConfig(cfg)
	if err != nil {
		t.Fatalf("SetErrorConfig failed: %v", err)
	}

	result := fi.CheckError()
	if result != nil {
		t.Errorf("expected nil error when disabled, got %v", result)
	}
}

func TestCheckError_MessageOnly(t *testing.T) {
	fi := NewFaultInjector()

	cfg := ErrorConfig{
		Enabled:     true,
		Message:     "only message error",
		TargetRatio: 1.0,
	}
	err := fi.SetErrorConfig(cfg)
	if err != nil {
		t.Fatalf("SetErrorConfig failed: %v", err)
	}

	result := fi.CheckError()
	if result == nil {
		t.Fatal("expected error, got nil")
	}

	if result.Error() != "only message error" {
		t.Errorf("unexpected error message: %s", result.Error())
	}
}

func TestDisconnect_Manual(t *testing.T) {
	fi := NewFaultInjector()

	if fi.IsDisconnected() {
		t.Error("should not be disconnected initially")
	}

	fi.Disconnect()
	if !fi.IsDisconnected() {
		t.Error("should be disconnected after Disconnect()")
	}

	fi.Reconnect()
	if fi.IsDisconnected() {
		t.Error("should not be disconnected after Reconnect()")
	}
}

func TestSetDisconnectConfig(t *testing.T) {
	fi := NewFaultInjector()

	cfg := DisconnectConfig{
		Enabled:     true,
		TargetRatio: 1.0,
	}
	err := fi.SetDisconnectConfig(cfg)
	if err != nil {
		t.Fatalf("SetDisconnectConfig failed: %v", err)
	}

	savedCfg := fi.GetDisconnectConfig()
	if !savedCfg.Enabled {
		t.Error("disconnect should be enabled")
	}
}

func TestSetDisconnectConfig_InvalidRatio(t *testing.T) {
	fi := NewFaultInjector()

	cfg := DisconnectConfig{
		Enabled:     true,
		TargetRatio: 2.0,
	}
	err := fi.SetDisconnectConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid target ratio")
	}
}

func TestCheckDisconnect(t *testing.T) {
	fi := NewFaultInjector()

	cfg := DisconnectConfig{
		Enabled:     true,
		TargetRatio: 1.0,
	}
	err := fi.SetDisconnectConfig(cfg)
	if err != nil {
		t.Fatalf("SetDisconnectConfig failed: %v", err)
	}

	result := fi.CheckDisconnect()
	if result == nil {
		t.Fatal("expected connection broken error, got nil")
	}

	var connErr *ConnectionBrokenError
	if !errors.As(result, &connErr) {
		t.Fatal("error should be *ConnectionBrokenError")
	}

	if !errors.Is(result, ErrConnectionBroken) {
		t.Error("error should unwrap to ErrConnectionBroken")
	}
}

func TestCheckDisconnect_Disabled(t *testing.T) {
	fi := NewFaultInjector()

	cfg := DisconnectConfig{
		Enabled:     false,
		TargetRatio: 1.0,
	}
	err := fi.SetDisconnectConfig(cfg)
	if err != nil {
		t.Fatalf("SetDisconnectConfig failed: %v", err)
	}

	result := fi.CheckDisconnect()
	if result != nil {
		t.Errorf("expected nil when disabled, got %v", result)
	}
}

func TestTimeWindow_IsActive(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		tw       *TimeWindow
		expected bool
	}{
		{
			name:     "nil time window",
			tw:       nil,
			expected: true,
		},
		{
			name: "within window",
			tw: &TimeWindow{
				StartTime: now.Add(-1 * time.Hour),
				EndTime:   now.Add(1 * time.Hour),
			},
			expected: true,
		},
		{
			name: "before start",
			tw: &TimeWindow{
				StartTime: now.Add(1 * time.Hour),
				EndTime:   now.Add(2 * time.Hour),
			},
			expected: false,
		},
		{
			name: "after end",
			tw: &TimeWindow{
				StartTime: now.Add(-2 * time.Hour),
				EndTime:   now.Add(-1 * time.Hour),
			},
			expected: false,
		},
		{
			name: "only start time, after start",
			tw: &TimeWindow{
				StartTime: now.Add(-1 * time.Hour),
			},
			expected: true,
		},
		{
			name: "only end time, before end",
			tw: &TimeWindow{
				EndTime: now.Add(1 * time.Hour),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.tw.IsActive()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDelayWithTimeWindow(t *testing.T) {
	now := time.Now()
	var slept time.Duration
	var currentTime time.Time = now

	fi := NewFaultInjector(
		WithSleepFunc(func(d time.Duration) { slept = d }),
		WithTimeNowFunc(func() time.Time { return currentTime }),
	)

	cfg := DelayConfig{
		Enabled:     true,
		Mode:        DelayModeFixed,
		Fixed:       100 * time.Millisecond,
		TargetRatio: 1.0,
		TimeWindow: &TimeWindow{
			StartTime: now.Add(-1 * time.Hour),
			EndTime:   now.Add(1 * time.Hour),
		},
	}
	err := fi.SetDelayConfig(cfg)
	if err != nil {
		t.Fatalf("SetDelayConfig failed: %v", err)
	}

	slept = 0
	fi.ApplyDelay()
	if slept != 100*time.Millisecond {
		t.Errorf("expected delay within window, got %v", slept)
	}

	currentTime = now.Add(2 * time.Hour)
	slept = 0
	fi.ApplyDelay()
	if slept != 0 {
		t.Errorf("expected no delay outside window, got %v", slept)
	}
}

func TestErrorWithTimeWindow(t *testing.T) {
	now := time.Now()
	var currentTime time.Time = now

	fi := NewFaultInjector(
		WithTimeNowFunc(func() time.Time { return currentTime }),
	)

	cfg := ErrorConfig{
		Enabled:     true,
		Message:     "test error",
		TargetRatio: 1.0,
		TimeWindow: &TimeWindow{
			StartTime: now.Add(-1 * time.Hour),
			EndTime:   now.Add(1 * time.Hour),
		},
	}
	err := fi.SetErrorConfig(cfg)
	if err != nil {
		t.Fatalf("SetErrorConfig failed: %v", err)
	}

	result := fi.CheckError()
	if result == nil {
		t.Error("expected error within window")
	}

	currentTime = now.Add(2 * time.Hour)
	result = fi.CheckError()
	if result != nil {
		t.Errorf("expected no error outside window, got %v", result)
	}
}

func TestDisconnectWithTimeWindow(t *testing.T) {
	now := time.Now()
	var currentTime time.Time = now

	fi := NewFaultInjector(
		WithTimeNowFunc(func() time.Time { return currentTime }),
	)

	cfg := DisconnectConfig{
		Enabled:     true,
		TargetRatio: 1.0,
		TimeWindow: &TimeWindow{
			StartTime: now.Add(-1 * time.Hour),
			EndTime:   now.Add(1 * time.Hour),
		},
	}
	err := fi.SetDisconnectConfig(cfg)
	if err != nil {
		t.Fatalf("SetDisconnectConfig failed: %v", err)
	}

	if !fi.IsDisconnected() {
		t.Error("expected disconnected within window")
	}

	currentTime = now.Add(2 * time.Hour)
	if fi.IsDisconnected() {
		t.Error("expected connected outside window")
	}
}

func TestTargetRatio_Delay(t *testing.T) {
	customRand := rand.New(rand.NewSource(42))
	var sleepCount int
	fi := NewFaultInjector(
		WithRandSource(customRand),
		WithSleepFunc(func(d time.Duration) { sleepCount++ }),
	)

	cfg := DelayConfig{
		Enabled:     true,
		Mode:        DelayModeFixed,
		Fixed:       10 * time.Millisecond,
		TargetRatio: 0.5,
	}
	err := fi.SetDelayConfig(cfg)
	if err != nil {
		t.Fatalf("SetDelayConfig failed: %v", err)
	}

	total := 10000
	sleepCount = 0
	for i := 0; i < total; i++ {
		fi.ApplyDelay()
	}

	ratio := float64(sleepCount) / float64(total)
	if ratio < 0.4 || ratio > 0.6 {
		t.Errorf("expected ratio ~0.5, got %f (count: %d)", ratio, sleepCount)
	}
}

func TestTargetRatio_Zero(t *testing.T) {
	customRand := rand.New(rand.NewSource(42))
	var sleepCount int
	fi := NewFaultInjector(
		WithRandSource(customRand),
		WithSleepFunc(func(d time.Duration) { sleepCount++ }),
	)

	cfg := DelayConfig{
		Enabled:     true,
		Mode:        DelayModeFixed,
		Fixed:       10 * time.Millisecond,
		TargetRatio: 0.0,
	}
	err := fi.SetDelayConfig(cfg)
	if err != nil {
		t.Fatalf("SetDelayConfig failed: %v", err)
	}

	for i := 0; i < 1000; i++ {
		fi.ApplyDelay()
	}
	if sleepCount != 0 {
		t.Errorf("expected 0 sleeps with ratio 0, got %d", sleepCount)
	}
}

func TestTargetRatio_One(t *testing.T) {
	customRand := rand.New(rand.NewSource(42))
	var sleepCount int
	fi := NewFaultInjector(
		WithRandSource(customRand),
		WithSleepFunc(func(d time.Duration) { sleepCount++ }),
	)

	cfg := DelayConfig{
		Enabled:     true,
		Mode:        DelayModeFixed,
		Fixed:       10 * time.Millisecond,
		TargetRatio: 1.0,
	}
	err := fi.SetDelayConfig(cfg)
	if err != nil {
		t.Fatalf("SetDelayConfig failed: %v", err)
	}

	for i := 0; i < 100; i++ {
		fi.ApplyDelay()
	}
	if sleepCount != 100 {
		t.Errorf("expected 100 sleeps with ratio 1, got %d", sleepCount)
	}
}

func TestInject_DisconnectFirst(t *testing.T) {
	fi := NewFaultInjector()
	fi.Disconnect()

	var fnCalled bool
	err := fi.Inject(func() error {
		fnCalled = true
		return nil
	})

	if err == nil {
		t.Error("expected error from disconnect")
	}
	if fnCalled {
		t.Error("function should not be called when disconnected")
	}
	if !errors.Is(err, ErrConnectionBroken) {
		t.Errorf("expected ErrConnectionBroken, got %v", err)
	}
}

func TestInject_ErrorInjection(t *testing.T) {
	fi := NewFaultInjector()
	testErr := errors.New("test injected error")

	cfg := ErrorConfig{
		Enabled:     true,
		Err:         testErr,
		Message:     "injected",
		TargetRatio: 1.0,
	}
	err := fi.SetErrorConfig(cfg)
	if err != nil {
		t.Fatalf("SetErrorConfig failed: %v", err)
	}

	var fnCalled bool
	err = fi.Inject(func() error {
		fnCalled = true
		return nil
	})

	if err == nil {
		t.Error("expected injected error")
	}
	if fnCalled {
		t.Error("function should not be called when error is injected")
	}
}

func TestInject_DelayAndSuccess(t *testing.T) {
	var slept time.Duration
	fi := NewFaultInjector(
		WithSleepFunc(func(d time.Duration) { slept = d }),
	)

	cfg := DelayConfig{
		Enabled:     true,
		Mode:        DelayModeFixed,
		Fixed:       50 * time.Millisecond,
		TargetRatio: 1.0,
	}
	err := fi.SetDelayConfig(cfg)
	if err != nil {
		t.Fatalf("SetDelayConfig failed: %v", err)
	}

	var fnCalled bool
	err = fi.Inject(func() error {
		fnCalled = true
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !fnCalled {
		t.Error("function should be called")
	}
	if slept != 50*time.Millisecond {
		t.Errorf("expected 50ms delay, got %v", slept)
	}
}

func TestInject_FunctionError(t *testing.T) {
	fi := NewFaultInjector()
	expectedErr := errors.New("function error")

	err := fi.Inject(func() error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("expected function error, got %v", err)
	}
}

func TestEnableDisableHelpers(t *testing.T) {
	fi := NewFaultInjector()

	err := fi.EnableDelay(DelayModeFixed, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("EnableDelay failed: %v", err)
	}
	if !fi.GetDelayConfig().Enabled {
		t.Error("delay should be enabled")
	}

	fi.DisableDelay()
	if fi.GetDelayConfig().Enabled {
		t.Error("delay should be disabled")
	}

	testErr := errors.New("test")
	err = fi.EnableError(testErr, "test message")
	if err != nil {
		t.Fatalf("EnableError failed: %v", err)
	}
	if !fi.GetErrorConfig().Enabled {
		t.Error("error should be enabled")
	}

	fi.DisableError()
	if fi.GetErrorConfig().Enabled {
		t.Error("error should be disabled")
	}

	err = fi.EnableDisconnect()
	if err != nil {
		t.Fatalf("EnableDisconnect failed: %v", err)
	}
	if !fi.GetDisconnectConfig().Enabled {
		t.Error("disconnect should be enabled")
	}

	fi.DisableDisconnect()
	if fi.GetDisconnectConfig().Enabled {
		t.Error("disconnect should be disabled")
	}
}

func TestReset(t *testing.T) {
	fi := NewFaultInjector()

	fi.EnableDelay(DelayModeFixed, 100*time.Millisecond)
	fi.EnableError(errors.New("test"), "msg")
	fi.EnableDisconnect()
	fi.Disconnect()

	fi.Reset()

	if fi.GetDelayConfig().Enabled {
		t.Error("delay should be disabled after reset")
	}
	if fi.GetErrorConfig().Enabled {
		t.Error("error should be disabled after reset")
	}
	if fi.GetDisconnectConfig().Enabled {
		t.Error("disconnect should be disabled after reset")
	}
	if fi.IsDisconnected() {
		t.Error("should not be disconnected after reset")
	}
}

func TestFaultType_String(t *testing.T) {
	tests := []struct {
		ft       FaultType
		expected string
	}{
		{FaultTypeDelay, "delay"},
		{FaultTypeError, "error"},
		{FaultTypeDisconnect, "disconnect"},
		{FaultType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.ft.String() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.ft.String())
			}
		})
	}
}

func TestInjectedError_Error(t *testing.T) {
	err := &InjectedError{
		Message: "test message",
		Cause:   errors.New("cause error"),
	}
	expected := "test message: cause error"
	if err.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, err.Error())
	}

	err2 := &InjectedError{
		Message: "only message",
	}
	if err2.Error() != "only message" {
		t.Errorf("expected 'only message', got '%s'", err2.Error())
	}
}

func TestConnectionBrokenError_Error(t *testing.T) {
	err := &ConnectionBrokenError{
		Message: "custom broken message",
	}
	if err.Error() != "custom broken message" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	err2 := &ConnectionBrokenError{}
	if err2.Error() != ErrConnectionBroken.Error() {
		t.Errorf("expected default message, got '%s'", err2.Error())
	}
}

func TestConcurrency(t *testing.T) {
	fi := NewFaultInjector()
	fi.EnableDelay(DelayModeFixed, 1*time.Millisecond)

	var wg sync.WaitGroup
	var counter int64
	total := 100

	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fi.ApplyDelay()
			fi.CheckError()
			fi.CheckDisconnect()
			_ = fi.IsDisconnected()
			_ = fi.GetDelayConfig()
			_ = fi.GetErrorConfig()
			_ = fi.GetDisconnectConfig()
			atomic.AddInt64(&counter, 1)
		}()
	}

	wg.Wait()
	if atomic.LoadInt64(&counter) != int64(total) {
		t.Errorf("expected %d goroutines to complete, got %d", total, counter)
	}
}

func TestConcurrency_WithWrites(t *testing.T) {
	fi := NewFaultInjector()

	var wg sync.WaitGroup
	total := 50

	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				fi.EnableDelay(DelayModeFixed, time.Duration(idx+1)*time.Millisecond)
			} else {
				fi.DisableDelay()
			}
			fi.ApplyDelay()
		}(i)
	}

	wg.Wait()
}

func TestValidateTargetRatio(t *testing.T) {
	tests := []struct {
		ratio   float64
		wantErr bool
	}{
		{0.0, false},
		{0.5, false},
		{1.0, false},
		{-0.1, true},
		{1.1, true},
	}

	for _, tt := range tests {
		err := validateTargetRatio(tt.ratio)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateTargetRatio(%f) error = %v, wantErr %v", tt.ratio, err, tt.wantErr)
		}
	}
}

func TestValidateTimeWindow(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		tw      *TimeWindow
		wantErr bool
	}{
		{"nil", nil, false},
		{"valid", &TimeWindow{StartTime: now, EndTime: now.Add(1 * time.Hour)}, false},
		{"start after end", &TimeWindow{StartTime: now.Add(1 * time.Hour), EndTime: now}, true},
		{"only start", &TimeWindow{StartTime: now}, false},
		{"only end", &TimeWindow{EndTime: now}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTimeWindow(tt.tw)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTimeWindow() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInject_AllFaultsCombined(t *testing.T) {
	var slept time.Duration
	fi := NewFaultInjector(
		WithSleepFunc(func(d time.Duration) { slept = d }),
	)

	fi.EnableDelay(DelayModeFixed, 10*time.Millisecond)
	testErr := errors.New("injected error")
	fi.EnableError(testErr, "error msg")
	fi.EnableDisconnect()

	var fnCalled bool
	err := fi.Inject(func() error {
		fnCalled = true
		return nil
	})

	if err == nil {
		t.Error("expected error (disconnect)")
	}
	if fnCalled {
		t.Error("function should not be called")
	}
	if !errors.Is(err, ErrConnectionBroken) {
		t.Errorf("expected disconnect error, got %v", err)
	}

	fi.Reconnect()
	fi.DisableDisconnect()

	slept = 0
	err = fi.Inject(func() error {
		fnCalled = true
		return nil
	})

	if err == nil {
		t.Error("expected error injection")
	}
	if fnCalled {
		t.Error("function should not be called due to error injection")
	}
	if !errors.Is(err, testErr) {
		t.Errorf("expected injected error, got %v", err)
	}

	fi.DisableError()

	slept = 0
	fnCalled = false
	err = fi.Inject(func() error {
		fnCalled = true
		return nil
	})

	if err != nil {
		t.Errorf("expected success, got %v", err)
	}
	if !fnCalled {
		t.Error("function should be called")
	}
	if slept != 10*time.Millisecond {
		t.Errorf("expected 10ms delay, got %v", slept)
	}
}

func TestManualDisconnectOverridesConfig(t *testing.T) {
	fi := NewFaultInjector()

	cfg := DisconnectConfig{
		Enabled:     false,
		TargetRatio: 0.0,
	}
	fi.SetDisconnectConfig(cfg)

	fi.Disconnect()

	if !fi.IsDisconnected() {
		t.Error("manual disconnect should work even when config is disabled")
	}

	result := fi.CheckDisconnect()
	if result == nil {
		t.Error("CheckDisconnect should return error after manual disconnect")
	}
}
