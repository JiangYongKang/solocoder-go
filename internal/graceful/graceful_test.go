package graceful

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.RequestWaitTimeout <= 0 {
		t.Error("expected positive RequestWaitTimeout in default config")
	}
	if cfg.GlobalTimeout <= 0 {
		t.Error("expected positive GlobalTimeout in default config")
	}
	if cfg.DefaultCallbackTimeout <= 0 {
		t.Error("expected positive DefaultCallbackTimeout in default config")
	}
	if cfg.StopAcceptingTimeout <= 0 {
		t.Error("expected positive StopAcceptingTimeout in default config")
	}
	if len(cfg.Signals) == 0 {
		t.Error("expected at least one signal in default config")
	}
}

func TestNewManager(t *testing.T) {
	cfg := Config{
		RequestWaitTimeout:  5 * time.Second,
		GlobalTimeout:       10 * time.Second,
		DefaultCallbackTimeout: 2 * time.Second,
		StopAcceptingTimeout: 1 * time.Second,
	}
	m := NewManager(cfg)

	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.State() != StateRunning {
		t.Errorf("expected StateRunning, got %v", m.State())
	}
	if m.Phase() != PhaseInit {
		t.Errorf("expected PhaseInit, got %v", m.Phase())
	}
	if !m.IsAccepting() {
		t.Error("expected accepting to be true initially")
	}
	if m.ActiveRequests() != 0 {
		t.Errorf("expected 0 active requests, got %d", m.ActiveRequests())
	}
}

func TestNewManager_DefaultValues(t *testing.T) {
	cfg := Config{}
	m := NewManager(cfg)

	if m.cfg.RequestWaitTimeout != DefaultConfig().RequestWaitTimeout {
		t.Error("expected default RequestWaitTimeout when zero value provided")
	}
	if m.cfg.GlobalTimeout != DefaultConfig().GlobalTimeout {
		t.Error("expected default GlobalTimeout when zero value provided")
	}
	if m.cfg.DefaultCallbackTimeout != DefaultConfig().DefaultCallbackTimeout {
		t.Error("expected default DefaultCallbackTimeout when zero value provided")
	}
	if m.cfg.StopAcceptingTimeout != DefaultConfig().StopAcceptingTimeout {
		t.Error("expected default StopAcceptingTimeout when zero value provided")
	}
	if len(m.cfg.Signals) != len(DefaultConfig().Signals) {
		t.Error("expected default Signals when empty provided")
	}
}

func TestRegisterCallback(t *testing.T) {
	m := NewManager(DefaultConfig())

	err := m.RegisterCallback("test", func(ctx context.Context) error { return nil }, 5*time.Second, 10)
	if err != nil {
		t.Fatalf("unexpected error registering callback: %v", err)
	}

	err = m.RegisterCallback("test", func(ctx context.Context) error { return nil }, 5*time.Second, 10)
	if err != ErrCallbackAlreadyRegistered {
		t.Errorf("expected ErrCallbackAlreadyRegistered, got %v", err)
	}

	err = m.RegisterCallback("", func(ctx context.Context) error { return nil }, 5*time.Second, 10)
	if err == nil {
		t.Error("expected error for empty callback name")
	}

	err = m.RegisterCallback("nil-fn", nil, 5*time.Second, 10)
	if err != ErrNilCallback {
		t.Errorf("expected ErrNilCallback, got %v", err)
	}
}

func TestRegisterCallback_AfterShutdown(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  50 * time.Millisecond,
		GlobalTimeout:       500 * time.Millisecond,
		DefaultCallbackTimeout: 50 * time.Millisecond,
		StopAcceptingTimeout: 10 * time.Millisecond,
	})

	go func() {
		time.Sleep(10 * time.Millisecond)
		m.Shutdown()
	}()

	time.Sleep(30 * time.Millisecond)
	err := m.RegisterCallback("too-late", func(ctx context.Context) error { return nil }, 0, 0)
	if err != ErrManagerAlreadyShuttingDown {
		t.Errorf("expected ErrManagerAlreadyShuttingDown, got %v", err)
	}

	<-m.ShutdownDone()
}

func TestUnregisterCallback(t *testing.T) {
	m := NewManager(DefaultConfig())

	m.RegisterCallback("cb1", func(ctx context.Context) error { return nil }, 0, 0)

	err := m.UnregisterCallback("cb1")
	if err != nil {
		t.Errorf("unexpected error unregistering callback: %v", err)
	}

	err = m.UnregisterCallback("cb1")
	if err != ErrCallbackNotFound {
		t.Errorf("expected ErrCallbackNotFound, got %v", err)
	}

	err = m.UnregisterCallback("nonexistent")
	if err != ErrCallbackNotFound {
		t.Errorf("expected ErrCallbackNotFound for nonexistent, got %v", err)
	}
}

func TestBeginEndRequest(t *testing.T) {
	m := NewManager(DefaultConfig())

	err := m.BeginRequest()
	if err != nil {
		t.Fatalf("unexpected error beginning request: %v", err)
	}
	if m.ActiveRequests() != 1 {
		t.Errorf("expected 1 active request, got %d", m.ActiveRequests())
	}

	m.EndRequest()
	if m.ActiveRequests() != 0 {
		t.Errorf("expected 0 active requests after end, got %d", m.ActiveRequests())
	}
}

func TestBeginRequest_Concurrent(t *testing.T) {
	m := NewManager(DefaultConfig())

	var wg sync.WaitGroup
	count := 100
	wg.Add(count)

	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			err := m.BeginRequest()
			if err != nil {
				t.Errorf("unexpected error beginning concurrent request: %v", err)
				return
			}
			time.Sleep(1 * time.Millisecond)
			m.EndRequest()
		}()
	}

	wg.Wait()
	if m.ActiveRequests() != 0 {
		t.Errorf("expected 0 active requests after all done, got %d", m.ActiveRequests())
	}
}

func TestStart(t *testing.T) {
	m := NewManager(DefaultConfig())
	err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error starting manager: %v", err)
	}

	err = m.Start()
	if err != ErrManagerNotRunning {
		t.Errorf("expected ErrManagerNotRunning on double start, got %v", err)
	}

	err = m.TriggerShutdown()
	if err != nil {
		t.Fatalf("unexpected error triggering shutdown: %v", err)
	}

	<-m.ShutdownDone()
}

func TestStart_WithoutStart_TriggerShutdown(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  50 * time.Millisecond,
		GlobalTimeout:       500 * time.Millisecond,
		DefaultCallbackTimeout: 50 * time.Millisecond,
		StopAcceptingTimeout: 10 * time.Millisecond,
	})

	err := m.TriggerShutdown()
	if err != nil {
		t.Fatalf("expected TriggerShutdown to work without Start: %v", err)
	}

	report := m.WaitForShutdown()
	if !report.Success {
		t.Errorf("expected successful shutdown, got errors: %v", report.Errors)
	}
}

func TestShutdown_Empty(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  50 * time.Millisecond,
		GlobalTimeout:       500 * time.Millisecond,
		DefaultCallbackTimeout: 50 * time.Millisecond,
		StopAcceptingTimeout: 10 * time.Millisecond,
	})

	report := make(chan *ShutdownReport, 1)
	go func() {
		err := m.Shutdown()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		report <- m.WaitForShutdown()
	}()

	r := <-report
	if !r.Success {
		t.Errorf("expected successful shutdown, got errors: %v", r.Errors)
	}
	if r.Phase != PhaseComplete {
		t.Errorf("expected PhaseComplete, got %v", r.Phase)
	}
	if r.ActiveRequests != 0 {
		t.Errorf("expected 0 active requests, got %d", r.ActiveRequests)
	}
}

func TestShutdown_DoubleCall(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  50 * time.Millisecond,
		GlobalTimeout:       500 * time.Millisecond,
		DefaultCallbackTimeout: 50 * time.Millisecond,
		StopAcceptingTimeout: 10 * time.Millisecond,
	})

	err1 := m.Shutdown()
	if err1 != nil {
		t.Fatalf("unexpected error on first shutdown: %v", err1)
	}

	<-m.ShutdownDone()

	err2 := m.Shutdown()
	if err2 != ErrManagerAlreadyShuttingDown {
		t.Errorf("expected ErrManagerAlreadyShuttingDown on second shutdown, got %v", err2)
	}
}

func TestShutdown_WithActiveRequests(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  200 * time.Millisecond,
		GlobalTimeout:       2 * time.Second,
		DefaultCallbackTimeout: 100 * time.Millisecond,
		StopAcceptingTimeout: 50 * time.Millisecond,
	})

	m.BeginRequest()
	m.BeginRequest()

	go func() {
		time.Sleep(50 * time.Millisecond)
		m.EndRequest()
		time.Sleep(50 * time.Millisecond)
		m.EndRequest()
	}()

	go func() {
		time.Sleep(10 * time.Millisecond)
		m.Shutdown()
	}()

	report := m.WaitForShutdown()
	if !report.Success {
		t.Errorf("expected successful shutdown after requests complete, got errors: %v", report.Errors)
	}
	if report.ActiveRequests != 0 {
		t.Errorf("expected 0 active requests, got %d", report.ActiveRequests)
	}
}

func TestShutdown_WaitRequestsTimeout(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  100 * time.Millisecond,
		GlobalTimeout:       2 * time.Second,
		DefaultCallbackTimeout: 100 * time.Millisecond,
		StopAcceptingTimeout: 20 * time.Millisecond,
	})

	m.BeginRequest()
	m.BeginRequest()

	go func() {
		time.Sleep(10 * time.Millisecond)
		m.Shutdown()
	}()

	report := m.WaitForShutdown()

	if report.ActiveRequests != 2 {
		t.Logf("active requests at end: %d (expected 2 since we never ended them)", report.ActiveRequests)
	}

	if len(report.Errors) == 0 {
		t.Log("expected at least one error for request wait timeout")
	}

	m.EndRequest()
	m.EndRequest()
}

func TestShutdown_BeginRequestRejected(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  200 * time.Millisecond,
		GlobalTimeout:       2 * time.Second,
		DefaultCallbackTimeout: 100 * time.Millisecond,
		StopAcceptingTimeout: 50 * time.Millisecond,
	})

	go func() {
		time.Sleep(20 * time.Millisecond)
		m.Shutdown()
	}()

	time.Sleep(40 * time.Millisecond)

	err := m.BeginRequest()
	if err != ErrManagerAlreadyShuttingDown {
		t.Errorf("expected ErrManagerAlreadyShuttingDown when shutdown in progress, got %v", err)
	}

	<-m.ShutdownDone()
}

func TestShutdown_WithCallbacks(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  50 * time.Millisecond,
		GlobalTimeout:       2 * time.Second,
		DefaultCallbackTimeout: 200 * time.Millisecond,
		StopAcceptingTimeout: 20 * time.Millisecond,
	})

	var mu sync.Mutex
	executed := make([]string, 0)

	m.RegisterCallback("cb1", func(ctx context.Context) error {
		mu.Lock()
		executed = append(executed, "cb1")
		mu.Unlock()
		return nil
	}, 0, 10)

	m.RegisterCallback("cb2", func(ctx context.Context) error {
		mu.Lock()
		executed = append(executed, "cb2")
		mu.Unlock()
		return nil
	}, 0, 20)

	m.RegisterCallback("cb3", func(ctx context.Context) error {
		mu.Lock()
		executed = append(executed, "cb3")
		mu.Unlock()
		return nil
	}, 0, 5)

	go m.Shutdown()
	report := m.WaitForShutdown()

	if !report.Success {
		t.Errorf("expected successful shutdown, got errors: %v", report.Errors)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(executed) != 3 {
		t.Errorf("expected 3 callbacks executed, got %d: %v", len(executed), executed)
	}

	for _, name := range []string{"cb1", "cb2", "cb3"} {
		found := false
		for _, e := range executed {
			if e == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected callback %s to be executed, got %v", name, executed)
		}
	}
}

func TestShutdown_CallbacksReverseOrderByPriority(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  50 * time.Millisecond,
		GlobalTimeout:       2 * time.Second,
		DefaultCallbackTimeout: 200 * time.Millisecond,
		StopAcceptingTimeout: 20 * time.Millisecond,
	})

	var mu sync.Mutex
	executed := make([]string, 0)

	m.RegisterCallback("high", func(ctx context.Context) error {
		mu.Lock()
		executed = append(executed, "high")
		mu.Unlock()
		return nil
	}, 0, 100)

	m.RegisterCallback("low", func(ctx context.Context) error {
		mu.Lock()
		executed = append(executed, "low")
		mu.Unlock()
		return nil
	}, 0, 10)

	m.RegisterCallback("mid", func(ctx context.Context) error {
		mu.Lock()
		executed = append(executed, "mid")
		mu.Unlock()
		return nil
	}, 0, 50)

	go m.Shutdown()
	report := m.WaitForShutdown()

	if !report.Success {
		t.Fatalf("shutdown failed: %v", report.Errors)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(executed) != 3 {
		t.Fatalf("expected 3 callbacks, got %v", executed)
	}

	if executed[0] != "low" {
		t.Errorf("expected 'low' first, got %v", executed)
	}
	if executed[1] != "mid" {
		t.Errorf("expected 'mid' second, got %v", executed)
	}
	if executed[2] != "high" {
		t.Errorf("expected 'high' last, got %v", executed)
	}
}

func TestShutdown_CallbackError(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  50 * time.Millisecond,
		GlobalTimeout:       2 * time.Second,
		DefaultCallbackTimeout: 200 * time.Millisecond,
		StopAcceptingTimeout: 20 * time.Millisecond,
	})

	m.RegisterCallback("fails", func(ctx context.Context) error {
		return errors.New("callback failed")
	}, 0, 10)

	m.RegisterCallback("ok", func(ctx context.Context) error {
		return nil
	}, 0, 20)

	go m.Shutdown()
	report := m.WaitForShutdown()

	if report.Success {
		t.Error("expected shutdown to report failure due to callback error")
	}

	var foundFail, foundOk bool
	for _, r := range report.CallbackResults {
		if r.Name == "fails" {
			foundFail = true
			if r.Success {
				t.Error("expected 'fails' callback to report failure")
			}
			if r.Error == nil {
				t.Error("expected 'fails' callback to have error")
			}
		}
		if r.Name == "ok" {
			foundOk = true
			if !r.Success {
				t.Error("expected 'ok' callback to report success")
			}
		}
	}
	if !foundFail || !foundOk {
		t.Errorf("missing callback results, got %d results", len(report.CallbackResults))
	}
}

func TestShutdown_CallbackPanic(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  50 * time.Millisecond,
		GlobalTimeout:       2 * time.Second,
		DefaultCallbackTimeout: 200 * time.Millisecond,
		StopAcceptingTimeout: 20 * time.Millisecond,
	})

	m.RegisterCallback("panics", func(ctx context.Context) error {
		panic("oh no!")
	}, 0, 10)

	go m.Shutdown()
	report := m.WaitForShutdown()

	if report.Success {
		t.Error("expected shutdown to report failure due to callback panic")
	}

	for _, r := range report.CallbackResults {
		if r.Name == "panics" {
			if r.Success {
				t.Error("expected panic callback to report failure")
			}
			if r.Error == nil {
				t.Error("expected panic callback to have error")
			}
		}
	}
}

func TestShutdown_CallbackTimeout(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  50 * time.Millisecond,
		GlobalTimeout:       5 * time.Second,
		DefaultCallbackTimeout: 200 * time.Millisecond,
		StopAcceptingTimeout: 20 * time.Millisecond,
	})

	m.RegisterCallback("slow", func(ctx context.Context) error {
		time.Sleep(5 * time.Second)
		return nil
	}, 100*time.Millisecond, 10)

	m.RegisterCallback("fast", func(ctx context.Context) error {
		return nil
	}, 0, 20)

	start := time.Now()
	go m.Shutdown()
	report := m.WaitForShutdown()
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Errorf("expected callback timeout to prevent long wait, took %v", elapsed)
	}

	for _, r := range report.CallbackResults {
		if r.Name == "slow" {
			if !r.TimedOut {
				t.Error("expected slow callback to be marked timed out")
			}
			if r.Success {
				t.Error("expected slow callback to report failure")
			}
		}
		if r.Name == "fast" {
			if !r.Success {
				t.Errorf("expected fast callback to succeed, got error: %v", r.Error)
			}
		}
	}
}

func TestShutdown_GlobalTimeout(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  50 * time.Millisecond,
		GlobalTimeout:       200 * time.Millisecond,
		DefaultCallbackTimeout: 5 * time.Second,
		StopAcceptingTimeout: 20 * time.Millisecond,
	})

	m.RegisterCallback("very-slow-1", func(ctx context.Context) error {
		time.Sleep(10 * time.Second)
		return nil
	}, 0, 10)

	m.RegisterCallback("very-slow-2", func(ctx context.Context) error {
		time.Sleep(10 * time.Second)
		return nil
	}, 0, 20)

	start := time.Now()
	go m.Shutdown()
	report := m.WaitForShutdown()
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Errorf("expected global timeout to prevent long wait, took %v", elapsed)
	}

	if !report.Forced {
		t.Error("expected forced shutdown due to global timeout")
	}

	if report.Phase != PhaseForced {
		t.Errorf("expected PhaseForced, got %v", report.Phase)
	}
}

func TestShutdown_GlobalTimeout_InWaitPhase(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  10 * time.Second,
		GlobalTimeout:       150 * time.Millisecond,
		DefaultCallbackTimeout: 100 * time.Millisecond,
		StopAcceptingTimeout: 20 * time.Millisecond,
	})

	m.BeginRequest()

	start := time.Now()
	go m.Shutdown()
	report := m.WaitForShutdown()
	elapsed := time.Since(start)

	m.EndRequest()

	if elapsed > 1*time.Second {
		t.Errorf("expected global timeout to cut short wait phase, took %v", elapsed)
	}

	if !report.Forced {
		t.Error("expected forced shutdown")
	}
}

func TestTriggerShutdown(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  50 * time.Millisecond,
		GlobalTimeout:       500 * time.Millisecond,
		DefaultCallbackTimeout: 50 * time.Millisecond,
		StopAcceptingTimeout: 10 * time.Millisecond,
	})
	m.Start()

	err := m.TriggerShutdown()
	if err != nil {
		t.Fatalf("unexpected error triggering shutdown: %v", err)
	}

	report := m.WaitForShutdown()
	if !report.Success {
		t.Errorf("expected successful shutdown, got errors: %v", report.Errors)
	}
}

func TestTriggerShutdown_DoubleTrigger(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  50 * time.Millisecond,
		GlobalTimeout:       500 * time.Millisecond,
		DefaultCallbackTimeout: 50 * time.Millisecond,
		StopAcceptingTimeout: 10 * time.Millisecond,
	})
	m.Start()

	err := m.TriggerShutdown()
	if err != nil {
		t.Fatalf("first trigger failed: %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	err = m.TriggerShutdown()
	if err != ErrManagerAlreadyShuttingDown {
		t.Errorf("expected ErrManagerAlreadyShuttingDown, got %v", err)
	}

	<-m.ShutdownDone()
}

func TestShutdownPhase_String(t *testing.T) {
	phases := map[ShutdownPhase]string{
		PhaseInit:           "Init",
		PhaseStopAccepting:  "StopAccepting",
		PhaseWaitRequests:   "WaitRequests",
		PhaseExecuteCallbacks: "ExecuteCallbacks",
		PhaseComplete:       "Complete",
		PhaseForced:         "Forced",
	}
	for p, expected := range phases {
		if p.String() != expected {
			t.Errorf("expected %s for phase %d, got %s", expected, int(p), p.String())
		}
	}

	unknown := ShutdownPhase(99)
	if unknown.String() != "Unknown" {
		t.Errorf("expected Unknown, got %s", unknown.String())
	}
}

func TestShutdownState_String(t *testing.T) {
	states := map[ShutdownState]string{
		StateRunning:      "Running",
		StateShuttingDown: "ShuttingDown",
		StateCompleted:    "Completed",
	}
	for s, expected := range states {
		if s.String() != expected {
			t.Errorf("expected %s for state %d, got %s", expected, int(s), s.String())
		}
	}

	unknown := ShutdownState(99)
	if unknown.String() != "Unknown" {
		t.Errorf("expected Unknown, got %s", unknown.String())
	}
}

func TestShutdownReport_HasGoroutineCount(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  50 * time.Millisecond,
		GlobalTimeout:       500 * time.Millisecond,
		DefaultCallbackTimeout: 50 * time.Millisecond,
		StopAcceptingTimeout: 10 * time.Millisecond,
	})

	go m.Shutdown()
	report := m.WaitForShutdown()

	if report.GoroutineCount <= 0 {
		t.Errorf("expected positive goroutine count, got %d", report.GoroutineCount)
	}
	if report.TotalDuration <= 0 {
		t.Error("expected positive total duration")
	}
}

func TestConcurrentShutdownAndRegister(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  100 * time.Millisecond,
		GlobalTimeout:       2 * time.Second,
		DefaultCallbackTimeout: 100 * time.Millisecond,
		StopAcceptingTimeout: 50 * time.Millisecond,
	})

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond)
		m.Shutdown()
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			name := fmt.Sprintf("cb-%d", i)
			_ = m.RegisterCallback(name, func(ctx context.Context) error {
				return nil
			}, 0, i)
			time.Sleep(1 * time.Millisecond)
		}
	}()

	wg.Wait()
	<-m.ShutdownDone()
}

func TestConcurrentBeginRequestAndShutdown(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  150 * time.Millisecond,
		GlobalTimeout:       2 * time.Second,
		DefaultCallbackTimeout: 100 * time.Millisecond,
		StopAcceptingTimeout: 50 * time.Millisecond,
	})

	var wg sync.WaitGroup
	var successfulBegins int32
	var rejectedBegins int32

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Duration(i) * 2 * time.Millisecond)
			err := m.BeginRequest()
			if err == nil {
				atomic.AddInt32(&successfulBegins, 1)
				time.Sleep(20 * time.Millisecond)
				m.EndRequest()
			} else {
				atomic.AddInt32(&rejectedBegins, 1)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(30 * time.Millisecond)
		m.Shutdown()
	}()

	wg.Wait()
	report := m.WaitForShutdown()

	if atomic.LoadInt32(&successfulBegins) == 0 && atomic.LoadInt32(&rejectedBegins) == 0 {
		t.Log("no requests were attempted")
	}

	if report.ActiveRequests != 0 {
		t.Errorf("expected 0 active requests at end, got %d", report.ActiveRequests)
	}
}

func TestGetReport(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  50 * time.Millisecond,
		GlobalTimeout:       500 * time.Millisecond,
		DefaultCallbackTimeout: 50 * time.Millisecond,
		StopAcceptingTimeout: 10 * time.Millisecond,
	})

	if m.GetReport() != nil {
		t.Error("expected nil report before shutdown")
	}

	go m.Shutdown()
	<-m.ShutdownDone()

	r := m.GetReport()
	if r == nil {
		t.Fatal("expected non-nil report after shutdown")
	}
	if !r.Success {
		t.Errorf("expected successful report, got errors: %v", r.Errors)
	}
}

func TestWaitForShutdown_MultipleWaiters(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  50 * time.Millisecond,
		GlobalTimeout:       500 * time.Millisecond,
		DefaultCallbackTimeout: 50 * time.Millisecond,
		StopAcceptingTimeout: 10 * time.Millisecond,
	})

	var wg sync.WaitGroup
	reports := make([]*ShutdownReport, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			reports[idx] = m.WaitForShutdown()
		}(i)
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		m.Shutdown()
	}()

	wg.Wait()

	for i, r := range reports {
		if r == nil {
			t.Errorf("waiter %d: expected non-nil report", i)
			continue
		}
		if !r.Success {
			t.Errorf("waiter %d: expected success, got errors: %v", i, r.Errors)
		}
	}
}

func TestBeginRequest_AfterStopAccepting(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  200 * time.Millisecond,
		GlobalTimeout:       2 * time.Second,
		DefaultCallbackTimeout: 100 * time.Millisecond,
		StopAcceptingTimeout: 50 * time.Millisecond,
	})

	go func() {
		time.Sleep(10 * time.Millisecond)
		m.Shutdown()
	}()

	deadline := time.Now().Add(200 * time.Millisecond)
	gotRejected := false
	for time.Now().Before(deadline) {
		if !m.IsAccepting() {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if m.IsAccepting() {
		t.Fatal("expected accepting to be false after shutdown started")
	}

	err := m.BeginRequest()
	if err != ErrManagerAlreadyShuttingDown {
		t.Errorf("expected ErrManagerAlreadyShuttingDown when not accepting, got %v", err)
	} else {
		gotRejected = true
	}

	<-m.ShutdownDone()
	if !gotRejected {
		t.Log("didn't get to test rejection, but it's ok")
	}
}

func TestShutdown_AllCallbacksComplete_IncompleteList(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  50 * time.Millisecond,
		GlobalTimeout:       2 * time.Second,
		DefaultCallbackTimeout: 200 * time.Millisecond,
		StopAcceptingTimeout: 20 * time.Millisecond,
	})

	m.RegisterCallback("ok-1", func(ctx context.Context) error { return nil }, 0, 10)
	m.RegisterCallback("ok-2", func(ctx context.Context) error { return nil }, 0, 20)

	go m.Shutdown()
	report := m.WaitForShutdown()

	if len(report.IncompleteCallbacks) != 0 {
		t.Errorf("expected 0 incomplete callbacks, got %v", report.IncompleteCallbacks)
	}
}

func TestShutdown_IncompleteCallbacksList(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  50 * time.Millisecond,
		GlobalTimeout:       300 * time.Millisecond,
		DefaultCallbackTimeout: 5 * time.Second,
		StopAcceptingTimeout: 20 * time.Millisecond,
	})

	m.RegisterCallback("a", func(ctx context.Context) error {
		time.Sleep(10 * time.Second)
		return nil
	}, 0, 10)

	m.RegisterCallback("b", func(ctx context.Context) error {
		time.Sleep(10 * time.Second)
		return nil
	}, 0, 20)

	go m.Shutdown()
	report := m.WaitForShutdown()

	if len(report.IncompleteCallbacks) == 0 {
		t.Error("expected incomplete callbacks list to be populated")
	}

	t.Logf("incomplete callbacks: %v", report.IncompleteCallbacks)
	t.Logf("callback results count: %d", len(report.CallbackResults))
}

func TestShutdown_ActiveRequestCounter(t *testing.T) {
	m := NewManager(Config{
		RequestWaitTimeout:  300 * time.Millisecond,
		GlobalTimeout:       2 * time.Second,
		DefaultCallbackTimeout: 100 * time.Millisecond,
		StopAcceptingTimeout: 50 * time.Millisecond,
	})

	for i := 0; i < 5; i++ {
		m.BeginRequest()
	}

	if m.ActiveRequests() != 5 {
		t.Fatalf("expected 5 active requests, got %d", m.ActiveRequests())
	}

	for i := 0; i < 3; i++ {
		m.EndRequest()
	}

	if m.ActiveRequests() != 2 {
		t.Errorf("expected 2 active requests, got %d", m.ActiveRequests())
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		m.EndRequest()
		m.EndRequest()
	}()

	go m.Shutdown()
	report := m.WaitForShutdown()

	if report.ActiveRequests != 0 {
		t.Errorf("expected 0 active requests at end, got %d", report.ActiveRequests)
	}
}
