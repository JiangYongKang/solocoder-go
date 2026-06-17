package fallback

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewChain(t *testing.T) {
	chain := NewChain(nil)
	if chain == nil {
		t.Fatal("expected non-nil chain")
	}
	if chain.State() != ChainStateHealthy {
		t.Errorf("expected state HEALTHY, got %v", chain.State())
	}
	if chain.StrategyCount() != 0 {
		t.Errorf("expected 0 strategies, got %d", chain.StrategyCount())
	}
	if chain.IsRunning() {
		t.Error("expected chain not running")
	}
}

func TestRegisterStrategy(t *testing.T) {
	chain := NewChain(nil)

	handler := func(ctx context.Context) (interface{}, error) {
		return "success", nil
	}

	err := chain.RegisterStrategy("s1", "Strategy 1", 1, handler, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if chain.StrategyCount() != 1 {
		t.Errorf("expected 1 strategy, got %d", chain.StrategyCount())
	}

	s, exists := chain.GetStrategy("s1")
	if !exists {
		t.Fatal("expected strategy s1 to exist")
	}
	if s.Priority != 1 {
		t.Errorf("expected priority 1, got %d", s.Priority)
	}

	err = chain.RegisterStrategy("s1", "Duplicate", 2, handler, nil)
	if !errors.Is(err, ErrStrategyAlreadyExists) {
		t.Errorf("expected ErrStrategyAlreadyExists, got %v", err)
	}

	err = chain.RegisterStrategy("", "Empty ID", 1, handler, nil)
	if err == nil {
		t.Error("expected error for empty ID")
	}

	err = chain.RegisterStrategy("s2", "Negative Priority", -1, handler, nil)
	if !errors.Is(err, ErrInvalidPriority) {
		t.Errorf("expected ErrInvalidPriority, got %v", err)
	}

	err = chain.RegisterStrategy("s3", "Nil Handler", 1, nil, nil)
	if !errors.Is(err, ErrNilHandler) {
		t.Errorf("expected ErrNilHandler, got %v", err)
	}

	err = chain.RegisterStrategy("s2", "Strategy 2", 0, handler, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	strategies := chain.GetAllStrategies()
	if len(strategies) != 2 {
		t.Fatalf("expected 2 strategies, got %d", len(strategies))
	}
	if strategies[0].Priority != 0 || strategies[1].Priority != 1 {
		t.Errorf("expected strategies sorted by priority, got [%d, %d]", strategies[0].Priority, strategies[1].Priority)
	}
}

func TestStartStop(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	err := chain.Start(ctx)
	if !errors.Is(err, ErrNoStrategies) {
		t.Errorf("expected ErrNoStrategies, got %v", err)
	}

	handler := func(ctx context.Context) (interface{}, error) {
		return "success", nil
	}
	chain.RegisterStrategy("s1", "Strategy 1", 0, handler, nil)

	err = chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !chain.IsRunning() {
		t.Error("expected chain to be running")
	}

	err = chain.Start(ctx)
	if !errors.Is(err, ErrChainAlreadyRunning) {
		t.Errorf("expected ErrChainAlreadyRunning, got %v", err)
	}

	chain.Stop()
	if chain.IsRunning() {
		t.Error("expected chain to be stopped")
	}

	chain.Stop()
}

func TestExecuteSuccess(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	handler := func(ctx context.Context) (interface{}, error) {
		return "main success", nil
	}
	chain.RegisterStrategy("main", "Main Strategy", 0, handler, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	result, err := chain.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "main success" {
		t.Errorf("expected 'main success', got %v", result)
	}
	if chain.CurrentStrategyID() != "main" {
		t.Errorf("expected current strategy 'main', got %s", chain.CurrentStrategyID())
	}
	if chain.CurrentIndex() != 0 {
		t.Errorf("expected current index 0, got %d", chain.CurrentIndex())
	}

	metrics := chain.Metrics()
	if metrics.TotalExecutions != 1 {
		t.Errorf("expected 1 execution, got %d", metrics.TotalExecutions)
	}
	if metrics.TotalSuccesses != 1 {
		t.Errorf("expected 1 success, got %d", metrics.TotalSuccesses)
	}
}

func TestExecuteFallback(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	var mainCallCount int
	mainHandler := func(ctx context.Context) (interface{}, error) {
		mainCallCount++
		return nil, errors.New("main failed")
	}

	var fallbackCallCount int
	fallbackHandler := func(ctx context.Context) (interface{}, error) {
		fallbackCallCount++
		return "fallback success", nil
	}

	chain.RegisterStrategy("main", "Main Strategy", 0, mainHandler, nil)
	chain.RegisterStrategy("fallback", "Fallback Strategy", 1, fallbackHandler, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	result, err := chain.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "fallback success" {
		t.Errorf("expected 'fallback success', got %v", result)
	}
	if mainCallCount != 1 {
		t.Errorf("expected main called 1 time, got %d", mainCallCount)
	}
	if fallbackCallCount != 1 {
		t.Errorf("expected fallback called 1 time, got %d", fallbackCallCount)
	}
	if chain.CurrentStrategyID() != "fallback" {
		t.Errorf("expected current strategy 'fallback', got %s", chain.CurrentStrategyID())
	}
	if chain.CurrentIndex() != 1 {
		t.Errorf("expected current index 1, got %d", chain.CurrentIndex())
	}

	metrics := chain.Metrics()
	if metrics.FallbackCount != 1 {
		t.Errorf("expected fallback count 1, got %d", metrics.FallbackCount)
	}
}

func TestExecuteAllFailed(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	mainHandler := func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("main failed")
	}

	fallbackHandler := func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("fallback failed")
	}

	chain.RegisterStrategy("main", "Main Strategy", 0, mainHandler, nil)
	chain.RegisterStrategy("fallback", "Fallback Strategy", 1, fallbackHandler, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	result, err := chain.Execute(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}

	var aggErr *AggregateError
	if !errors.As(err, &aggErr) {
		t.Fatalf("expected AggregateError, got %T", err)
	}
	if len(aggErr.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(aggErr.Errors))
	}
	if aggErr.Errors[0].Error() != "main failed" {
		t.Errorf("expected first error 'main failed', got %v", aggErr.Errors[0])
	}
	if aggErr.Errors[1].Error() != "fallback failed" {
		t.Errorf("expected second error 'fallback failed', got %v", aggErr.Errors[1])
	}

	if chain.State() != ChainStateDegraded {
		t.Errorf("expected state DEGRADED, got %v", chain.State())
	}
}

func TestExecuteNotRunning(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	handler := func(ctx context.Context) (interface{}, error) {
		return "success", nil
	}
	chain.RegisterStrategy("s1", "Strategy 1", 0, handler, nil)

	_, err := chain.Execute(ctx)
	if !errors.Is(err, ErrChainNotRunning) {
		t.Errorf("expected ErrChainNotRunning, got %v", err)
	}
}

func TestTimeoutTriggerCondition(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	slowHandler := func(ctx context.Context) (interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		return "slow", nil
	}

	fastHandler := func(ctx context.Context) (interface{}, error) {
		return "fast", nil
	}

	timeoutCond := &TriggerCondition{
		Type:    TriggerConditionTimeout,
		Timeout: 100 * time.Millisecond,
	}

	chain.RegisterStrategy("slow", "Slow Strategy", 0, slowHandler, timeoutCond)
	chain.RegisterStrategy("fast", "Fast Strategy", 1, fastHandler, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	result, err := chain.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "fast" {
		t.Errorf("expected 'fast', got %v", result)
	}
}

func TestErrorTypeTriggerCondition(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	dbErr := errors.New("database connection failed")
	cacheErr := errors.New("cache miss")

	dbHandler := func(ctx context.Context) (interface{}, error) {
		return nil, dbErr
	}

	cacheHandler := func(ctx context.Context) (interface{}, error) {
		return nil, cacheErr
	}

	backupHandler := func(ctx context.Context) (interface{}, error) {
		return "backup", nil
	}

	cacheCond := &TriggerCondition{
		Type:       TriggerConditionErrorType,
		ErrorTypes: []error{cacheErr},
	}

	dbCond := &TriggerCondition{
		Type:       TriggerConditionErrorType,
		ErrorTypes: []error{dbErr},
	}

	chain.RegisterStrategy("db", "Database", 0, dbHandler, dbCond)
	chain.RegisterStrategy("cache", "Cache", 1, cacheHandler, cacheCond)
	chain.RegisterStrategy("backup", "Backup", 2, backupHandler, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	result, err := chain.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "backup" {
		t.Errorf("expected 'backup', got %v", result)
	}
}

func TestCustomTriggerCondition(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	customErr := fmt.Errorf("rate limit exceeded: 100")

	handler1 := func(ctx context.Context) (interface{}, error) {
		return nil, customErr
	}

	handler2 := func(ctx context.Context) (interface{}, error) {
		return "success", nil
	}

	customCond := &TriggerCondition{
		Type: TriggerConditionCustom,
		CustomCheck: func(err error) bool {
			return err != nil && errors.Is(err, customErr)
		},
	}

	chain.RegisterStrategy("s1", "Strategy 1", 0, handler1, customCond)
	chain.RegisterStrategy("s2", "Strategy 2", 1, handler2, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	result, err := chain.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "success" {
		t.Errorf("expected 'success', got %v", result)
	}
}

func TestTriggerConditionSkip(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	criticalErr := errors.New("critical failure")

	handler1 := func(ctx context.Context) (interface{}, error) {
		return nil, criticalErr
	}

	handler2 := func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("intermediate failed")
	}

	handler3 := func(ctx context.Context) (interface{}, error) {
		return "final success", nil
	}

	criticalCond := &TriggerCondition{
		Type:       TriggerConditionErrorType,
		ErrorTypes: []error{criticalErr},
	}

	chain.RegisterStrategy("s1", "Strategy 1", 0, handler1, nil)
	chain.RegisterStrategy("s2", "Strategy 2", 1, handler2, nil)
	chain.RegisterStrategy("s3", "Strategy 3", 2, handler3, criticalCond)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	result, err := chain.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "final success" {
		t.Errorf("expected 'final success', got %v", result)
	}
}

func TestActiveRecovery(t *testing.T) {
	cfg := DefaultChainConfig()
	cfg.Recovery.Mode = RecoveryModeActive
	cfg.Recovery.CheckInterval = 100 * time.Millisecond
	cfg.Recovery.ProbeSuccessThreshold = 2
	cfg.Recovery.ProbeFailureThreshold = 1
	cfg.Recovery.WarmUpDuration = 0

	chain := NewChain(cfg)
	ctx := context.Background()

	var mainShouldFail atomic.Bool
	mainShouldFail.Store(true)

	mainHandler := func(ctx context.Context) (interface{}, error) {
		if mainShouldFail.Load() {
			return nil, errors.New("main failed")
		}
		return "main success", nil
	}

	fallbackHandler := func(ctx context.Context) (interface{}, error) {
		return "fallback", nil
	}

	chain.RegisterStrategy("main", "Main", 0, mainHandler, nil)
	chain.RegisterStrategy("fallback", "Fallback", 1, fallbackHandler, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	result, err := chain.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "fallback" {
		t.Errorf("expected 'fallback', got %v", result)
	}

	mainShouldFail.Store(false)

	time.Sleep(500 * time.Millisecond)

	if chain.CurrentIndex() != 0 {
		t.Errorf("expected recovered to index 0, got %d", chain.CurrentIndex())
	}
	if chain.State() != ChainStateHealthy {
		t.Errorf("expected state HEALTHY, got %v", chain.State())
	}

	result, err = chain.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "main success" {
		t.Errorf("expected 'main success', got %v", result)
	}
}

func TestPassiveRecovery(t *testing.T) {
	cfg := DefaultChainConfig()
	cfg.Recovery.Mode = RecoveryModePassive
	cfg.Recovery.PassiveSuccessCount = 5
	cfg.Recovery.PassiveSuccessWindow = 30 * time.Second
	cfg.Recovery.WarmUpDuration = 0

	chain := NewChain(cfg)
	ctx := context.Background()

	var mainShouldFail atomic.Bool
	mainShouldFail.Store(false)

	mainHandler := func(ctx context.Context) (interface{}, error) {
		if mainShouldFail.Load() {
			return nil, errors.New("main failed")
		}
		return "main success", nil
	}

	fallbackHandler := func(ctx context.Context) (interface{}, error) {
		return "fallback", nil
	}

	chain.RegisterStrategy("main", "Main", 0, mainHandler, nil)
	chain.RegisterStrategy("fallback", "Fallback", 1, fallbackHandler, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	for i := 0; i < 5; i++ {
		chain.Execute(ctx)
	}

	mainShouldFail.Store(true)
	for i := 0; i < 3; i++ {
		chain.Execute(ctx)
	}

	if chain.CurrentIndex() == 0 {
		t.Errorf("expected to be on fallback strategy after failures")
	}

	mainStrategy, _ := chain.GetStrategy("main")
	mainStrategy.mu.Lock()
	now := time.Now()
	for i := 0; i < 10; i++ {
		mainStrategy.SuccessWindow = append(mainStrategy.SuccessWindow, successEntry{
			Time: now.Add(time.Duration(i) * time.Millisecond),
		})
	}
	mainStrategy.SuccessCount += 10
	mainStrategy.ConsecutiveFail = 0
	mainStrategy.mu.Unlock()

	_, err = chain.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mainShouldFail.Store(false)

	time.Sleep(200 * time.Millisecond)

	if chain.CurrentIndex() == 0 {
		t.Logf("successfully recovered to main strategy")
	}
}

func TestWarmUpPeriod(t *testing.T) {
	cfg := DefaultChainConfig()
	cfg.Recovery.Mode = RecoveryModeActive
	cfg.Recovery.CheckInterval = 50 * time.Millisecond
	cfg.Recovery.ProbeSuccessThreshold = 1
	cfg.Recovery.WarmUpDuration = 200 * time.Millisecond

	chain := NewChain(cfg)
	ctx := context.Background()

	var mainShouldFail atomic.Bool
	mainShouldFail.Store(true)

	var mainCallCount atomic.Int32
	mainHandler := func(ctx context.Context) (interface{}, error) {
		mainCallCount.Add(1)
		if mainShouldFail.Load() {
			return nil, errors.New("main failed")
		}
		return "main success", nil
	}

	fallbackHandler := func(ctx context.Context) (interface{}, error) {
		return "fallback", nil
	}

	chain.RegisterStrategy("main", "Main", 0, mainHandler, nil)
	chain.RegisterStrategy("fallback", "Fallback", 1, fallbackHandler, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	chain.Execute(ctx)
	mainShouldFail.Store(false)

	time.Sleep(500 * time.Millisecond)

	if chain.CurrentIndex() != 0 {
		t.Errorf("expected recovered to index 0, got %d", chain.CurrentIndex())
	}
}

func TestWarmUpFailure(t *testing.T) {
	cfg := DefaultChainConfig()
	cfg.Recovery.Mode = RecoveryModeActive
	cfg.Recovery.CheckInterval = 50 * time.Millisecond
	cfg.Recovery.ProbeSuccessThreshold = 1
	cfg.Recovery.ProbeFailureThreshold = 1
	cfg.Recovery.WarmUpDuration = 200 * time.Millisecond

	chain := NewChain(cfg)
	ctx := context.Background()

	var failDuringWarmUp atomic.Bool
	var callCount atomic.Int32

	mainHandler := func(ctx context.Context) (interface{}, error) {
		count := callCount.Add(1)
		if failDuringWarmUp.Load() && count > 3 {
			return nil, errors.New("warm up failed")
		}
		return "main success", nil
	}

	fallbackHandler := func(ctx context.Context) (interface{}, error) {
		return "fallback", nil
	}

	chain.RegisterStrategy("main", "Main", 0, mainHandler, nil)
	chain.RegisterStrategy("fallback", "Fallback", 1, fallbackHandler, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	failDuringWarmUp.Store(true)
	chain.ForceSwitchToStrategy("fallback")

	mainStrategy, _ := chain.GetStrategy("main")
	mainStrategy.mu.Lock()
	mainStrategy.State = StrategyStateDegraded
	mainStrategy.mu.Unlock()

	time.Sleep(300 * time.Millisecond)
}

func TestForceSwitch(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	chain.RegisterStrategy("s1", "Strategy 1", 0, func(ctx context.Context) (interface{}, error) { return "s1", nil }, nil)
	chain.RegisterStrategy("s2", "Strategy 2", 1, func(ctx context.Context) (interface{}, error) { return "s2", nil }, nil)
	chain.RegisterStrategy("s3", "Strategy 3", 2, func(ctx context.Context) (interface{}, error) { return "s3", nil }, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	err = chain.ForceSwitchToStrategy("s2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chain.CurrentStrategyID() != "s2" {
		t.Errorf("expected current strategy 's2', got %s", chain.CurrentStrategyID())
	}
	if chain.CurrentIndex() != 1 {
		t.Errorf("expected current index 1, got %d", chain.CurrentIndex())
	}

	err = chain.ForceSwitchToStrategy("nonexistent")
	if !errors.Is(err, ErrStrategyNotFound) {
		t.Errorf("expected ErrStrategyNotFound, got %v", err)
	}

	err = chain.ForceSwitchToMain()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chain.CurrentStrategyID() != "s1" {
		t.Errorf("expected current strategy 's1', got %s", chain.CurrentStrategyID())
	}
}

func TestResetMetrics(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	chain.RegisterStrategy("s1", "Strategy 1", 0, func(ctx context.Context) (interface{}, error) { return "ok", nil }, nil)
	chain.Start(ctx)
	defer chain.Stop()

	for i := 0; i < 5; i++ {
		chain.Execute(ctx)
	}

	metrics := chain.Metrics()
	if metrics.TotalExecutions != 5 {
		t.Errorf("expected 5 executions, got %d", metrics.TotalExecutions)
	}

	chain.ResetMetrics()
	metrics = chain.Metrics()
	if metrics.TotalExecutions != 0 {
		t.Errorf("expected 0 executions after reset, got %d", metrics.TotalExecutions)
	}
}

func TestCalculateErrorRate(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	var failCount int
	handler := func(ctx context.Context) (interface{}, error) {
		failCount++
		if failCount <= 3 {
			return nil, errors.New("failed")
		}
		return "ok", nil
	}

	chain.RegisterStrategy("s1", "Strategy 1", 0, handler, nil)
	chain.Start(ctx)
	defer chain.Stop()

	for i := 0; i < 5; i++ {
		chain.Execute(ctx)
	}

	rate, err := chain.CalculateErrorRate("s1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate != 0.6 {
		t.Errorf("expected error rate 0.6, got %f", rate)
	}

	rate, err = chain.CalculateErrorRate("nonexistent", 0)
	if !errors.Is(err, ErrStrategyNotFound) {
		t.Errorf("expected ErrStrategyNotFound, got %v", err)
	}
}

func TestStrategyStats(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	chain.RegisterStrategy("s1", "Strategy 1", 0, func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("fail")
	}, nil)
	chain.RegisterStrategy("s2", "Strategy 2", 1, func(ctx context.Context) (interface{}, error) {
		return "ok", nil
	}, nil)

	chain.Start(ctx)
	defer chain.Stop()

	for i := 0; i < 3; i++ {
		chain.Execute(ctx)
	}

	s1, _ := chain.GetStrategy("s1")
	s2, _ := chain.GetStrategy("s2")

	success1, fail1, consec1 := s1.Stats()
	if success1 != 0 {
		t.Errorf("expected s1 success 0, got %d", success1)
	}
	if fail1 != 1 {
		t.Errorf("expected s1 fail 1, got %d", fail1)
	}
	if consec1 != 1 {
		t.Errorf("expected s1 consecutive fail 1, got %d", consec1)
	}

	success2, fail2, consec2 := s2.Stats()
	if success2 != 3 {
		t.Errorf("expected s2 success 3, got %d", success2)
	}
	if fail2 != 0 {
		t.Errorf("expected s2 fail 0, got %d", fail2)
	}
	if consec2 != 0 {
		t.Errorf("expected s2 consecutive fail 0, got %d", consec2)
	}
}

func TestConcurrency(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	chain.RegisterStrategy("s1", "Strategy 1", 0, func(ctx context.Context) (interface{}, error) {
		return "ok", nil
	}, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	var wg sync.WaitGroup
	numGoroutines := 100
	numRequests := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numRequests; j++ {
				result, err := chain.Execute(ctx)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if result != "ok" {
					t.Errorf("expected 'ok', got %v", result)
					return
				}
			}
		}()
	}

	wg.Wait()

	metrics := chain.Metrics()
	expectedTotal := uint64(numGoroutines * numRequests)
	if metrics.TotalExecutions != expectedTotal {
		t.Errorf("expected %d executions, got %d", expectedTotal, metrics.TotalExecutions)
	}
}

func TestPanicRecovery(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	panicHandler := func(ctx context.Context) (interface{}, error) {
		panic("something went wrong")
	}

	normalHandler := func(ctx context.Context) (interface{}, error) {
		return "normal", nil
	}

	chain.RegisterStrategy("panic", "Panic Strategy", 0, panicHandler, nil)
	chain.RegisterStrategy("normal", "Normal Strategy", 1, normalHandler, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	result, err := chain.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "normal" {
		t.Errorf("expected 'normal', got %v", result)
	}
}

func TestContextCancellation(t *testing.T) {
	chain := NewChain(nil)

	slowHandler := func(ctx context.Context) (interface{}, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return "slow", nil
		}
	}

	chain.RegisterStrategy("slow", "Slow Strategy", 0, slowHandler, nil)

	ctx, cancel := context.WithCancel(context.Background())
	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err = chain.Execute(ctx)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestStrategyStateTransitions(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	chain.RegisterStrategy("s1", "Strategy 1", 0, func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("fail")
	}, nil)
	chain.RegisterStrategy("s2", "Strategy 2", 1, func(ctx context.Context) (interface{}, error) {
		return "ok", nil
	}, nil)

	chain.Start(ctx)
	defer chain.Stop()

	s1, _ := chain.GetStrategy("s1")
	s2, _ := chain.GetStrategy("s2")

	if s1.GetState() != StrategyStateActive {
		t.Errorf("expected s1 state ACTIVE, got %v", s1.GetState())
	}

	chain.Execute(ctx)

	if s1.GetState() != StrategyStateDegraded {
		t.Errorf("expected s1 state DEGRADED, got %v", s1.GetState())
	}
	if s2.GetState() != StrategyStateActive {
		t.Errorf("expected s2 state ACTIVE, got %v", s2.GetState())
	}
}

func TestAggregateErrorUnwrap(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	aggErr := &AggregateError{
		Errors: []error{err1, err2},
	}

	unwrapped := aggErr.Unwrap()
	if len(unwrapped) != 2 {
		t.Fatalf("expected 2 unwrapped errors, got %d", len(unwrapped))
	}
	if !errors.Is(unwrapped[0], err1) {
		t.Error("expected first unwrapped error to be err1")
	}
	if !errors.Is(unwrapped[1], err2) {
		t.Error("expected second unwrapped error to be err2")
	}

	msg := aggErr.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}

	emptyAgg := &AggregateError{}
	if emptyAgg.Error() != "all fallback strategies failed" {
		t.Errorf("unexpected empty aggregate error message: %s", emptyAgg.Error())
	}
}

func TestStrategyStateString(t *testing.T) {
	tests := []struct {
		state    StrategyState
		expected string
	}{
		{StrategyStateActive, "ACTIVE"},
		{StrategyStateDegraded, "DEGRADED"},
		{StrategyStateRecovering, "RECOVERING"},
		{StrategyStateWarmingUp, "WARMING_UP"},
		{StrategyState(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.state.String())
		}
	}
}

func TestChainStateString(t *testing.T) {
	tests := []struct {
		state    ChainState
		expected string
	}{
		{ChainStateHealthy, "HEALTHY"},
		{ChainStateDegraded, "DEGRADED"},
		{ChainStateRecovering, "RECOVERING"},
		{ChainState(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.state.String())
		}
	}
}

func TestErrIsType(t *testing.T) {
	baseErr := errors.New("base")
	wrappedErr := fmt.Errorf("wrapped: %w", baseErr)
	doubleWrapped := fmt.Errorf("double: %w", wrappedErr)
	multiWrapped := fmt.Errorf("multi: %w", baseErr)

	tests := []struct {
		err      error
		target   error
		expected bool
	}{
		{nil, baseErr, false},
		{baseErr, nil, false},
		{baseErr, baseErr, true},
		{wrappedErr, baseErr, true},
		{doubleWrapped, baseErr, true},
		{multiWrapped, baseErr, true},
		{baseErr, errors.New("other"), false},
	}

	for i, tt := range tests {
		result := errIsType(tt.err, tt.target)
		if result != tt.expected {
			t.Errorf("test %d: expected %v, got %v", i, tt.expected, result)
		}
	}
}

func TestPrioritySorting(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	handler := func(ctx context.Context) (interface{}, error) {
		return "ok", nil
	}

	chain.RegisterStrategy("s5", "Strategy 5", 5, handler, nil)
	chain.RegisterStrategy("s1", "Strategy 1", 1, handler, nil)
	chain.RegisterStrategy("s3", "Strategy 3", 3, handler, nil)
	chain.RegisterStrategy("s0", "Strategy 0", 0, handler, nil)
	chain.RegisterStrategy("s2", "Strategy 2", 2, handler, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	strategies := chain.GetAllStrategies()
	expectedOrder := []string{"s0", "s1", "s2", "s3", "s5"}

	if len(strategies) != len(expectedOrder) {
		t.Fatalf("expected %d strategies, got %d", len(expectedOrder), len(strategies))
	}

	for i, expectedID := range expectedOrder {
		if strategies[i].ID != expectedID {
			t.Errorf("expected strategy %s at index %d, got %s", expectedID, i, strategies[i].ID)
		}
	}

	result, err := chain.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %v", result)
	}
	if chain.CurrentStrategyID() != "s0" {
		t.Errorf("expected current strategy s0, got %s", chain.CurrentStrategyID())
	}
}

func TestMultipleFallbackLevels(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	chain.RegisterStrategy("l1", "Level 1", 0, func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("l1 failed")
	}, nil)
	chain.RegisterStrategy("l2", "Level 2", 1, func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("l2 failed")
	}, nil)
	chain.RegisterStrategy("l3", "Level 3", 2, func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("l3 failed")
	}, nil)
	chain.RegisterStrategy("l4", "Level 4", 3, func(ctx context.Context) (interface{}, error) {
		return "l4 success", nil
	}, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	result, err := chain.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "l4 success" {
		t.Errorf("expected 'l4 success', got %v", result)
	}
	if chain.CurrentIndex() != 3 {
		t.Errorf("expected current index 3, got %d", chain.CurrentIndex())
	}

	metrics := chain.Metrics()
	if metrics.FallbackCount != 3 {
		t.Errorf("expected fallback count 3, got %d", metrics.FallbackCount)
	}
}

func TestRecoveryCount(t *testing.T) {
	cfg := DefaultChainConfig()
	cfg.Recovery.Mode = RecoveryModeActive
	cfg.Recovery.CheckInterval = 50 * time.Millisecond
	cfg.Recovery.ProbeSuccessThreshold = 1
	cfg.Recovery.WarmUpDuration = 0

	chain := NewChain(cfg)
	ctx := context.Background()

	var mainShouldFail atomic.Bool
	mainShouldFail.Store(true)

	mainHandler := func(ctx context.Context) (interface{}, error) {
		if mainShouldFail.Load() {
			return nil, errors.New("main failed")
		}
		return "main success", nil
	}

	fallbackHandler := func(ctx context.Context) (interface{}, error) {
		return "fallback", nil
	}

	chain.RegisterStrategy("main", "Main", 0, mainHandler, nil)
	chain.RegisterStrategy("fallback", "Fallback", 1, fallbackHandler, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	chain.Execute(ctx)
	mainShouldFail.Store(false)

	time.Sleep(300 * time.Millisecond)

	metrics := chain.Metrics()
	if metrics.RecoveryCount != 1 {
		t.Errorf("expected recovery count 1, got %d", metrics.RecoveryCount)
	}
}

func TestErrorRateTriggerCondition(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	var failCount int
	mainHandler := func(ctx context.Context) (interface{}, error) {
		failCount++
		if failCount <= 6 {
			return nil, errors.New("failed")
		}
		return "ok", nil
	}

	fallbackHandler := func(ctx context.Context) (interface{}, error) {
		return "fallback ok", nil
	}

	errorRateCond := &TriggerCondition{
		Type:        TriggerConditionErrorRate,
		ErrorRate:   0.5,
		ErrorWindow: 10 * time.Second,
	}

	chain.RegisterStrategy("main", "Main", 0, mainHandler, errorRateCond)
	chain.RegisterStrategy("fallback", "Fallback", 1, fallbackHandler, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	for i := 0; i < 3; i++ {
		chain.Execute(ctx)
	}

	rate, _ := chain.CalculateErrorRate("main", 0)
	if rate < 0.99 {
		t.Errorf("expected error rate ~1.0, got %f", rate)
	}

	for i := 0; i < 3; i++ {
		chain.Execute(ctx)
	}

	mainStrategy, _ := chain.GetStrategy("main")
	if mainStrategy.GetState() != StrategyStateDegraded {
		t.Logf("main strategy state: %v", mainStrategy.GetState())
	}
}

func TestCalculateErrorRateWithWindow(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	var callCount int
	handler := func(ctx context.Context) (interface{}, error) {
		callCount++
		if callCount <= 4 {
			return nil, errors.New("failed")
		}
		return "ok", nil
	}

	chain.RegisterStrategy("s1", "Strategy 1", 0, handler, nil)
	chain.Start(ctx)
	defer chain.Stop()

	for i := 0; i < 10; i++ {
		chain.Execute(ctx)
	}

	rate, err := chain.CalculateErrorRate("s1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate != 0.4 {
		t.Errorf("expected error rate 0.4, got %f", rate)
	}

	rate, err = chain.CalculateErrorRate("s1", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate != 0.4 {
		t.Errorf("expected error rate 0.4 with window, got %f", rate)
	}
}

func TestCalculateErrorRateZeroWindow(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	handler := func(ctx context.Context) (interface{}, error) {
		return "ok", nil
	}

	chain.RegisterStrategy("s1", "Strategy 1", 0, handler, nil)
	chain.Start(ctx)
	defer chain.Stop()

	rate, err := chain.CalculateErrorRate("s1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate != 0 {
		t.Errorf("expected error rate 0 for no executions, got %f", rate)
	}

	for i := 0; i < 5; i++ {
		chain.Execute(ctx)
	}

	rate, _ = chain.CalculateErrorRate("s1", 0)
	if rate != 0 {
		t.Errorf("expected error rate 0 for all successes, got %f", rate)
	}
}

func TestSuccessWindowRecords(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	handler := func(ctx context.Context) (interface{}, error) {
		return "ok", nil
	}

	chain.RegisterStrategy("s1", "Strategy 1", 0, handler, nil)
	chain.Start(ctx)
	defer chain.Stop()

	for i := 0; i < 10; i++ {
		chain.Execute(ctx)
	}

	strategy, _ := chain.GetStrategy("s1")
	strategy.mu.RLock()
	successCount := len(strategy.SuccessWindow)
	strategy.mu.RUnlock()

	if successCount != 10 {
		t.Errorf("expected 10 success entries in window, got %d", successCount)
	}

	success, fail, _ := strategy.Stats()
	if success != 10 {
		t.Errorf("expected 10 successes, got %d", success)
	}
	if fail != 0 {
		t.Errorf("expected 0 failures, got %d", fail)
	}
}

func TestMixedSuccessFailureWindow(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	var callCount int
	handler := func(ctx context.Context) (interface{}, error) {
		callCount++
		if callCount%2 == 0 {
			return nil, errors.New("even failed")
		}
		return "odd ok", nil
	}

	chain.RegisterStrategy("s1", "Strategy 1", 0, handler, nil)
	chain.Start(ctx)
	defer chain.Stop()

	for i := 0; i < 10; i++ {
		chain.Execute(ctx)
	}

	strategy, _ := chain.GetStrategy("s1")
	strategy.mu.RLock()
	successCount := len(strategy.SuccessWindow)
	errorCount := len(strategy.ErrorWindow)
	strategy.mu.RUnlock()

	if successCount != 5 {
		t.Errorf("expected 5 success entries, got %d", successCount)
	}
	if errorCount != 5 {
		t.Errorf("expected 5 error entries, got %d", errorCount)
	}

	rate, _ := chain.CalculateErrorRate("s1", time.Hour)
	if rate != 0.5 {
		t.Errorf("expected error rate 0.5, got %f", rate)
	}
}

func TestErrorRateTriggerSkipsStrategy(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	mainHandler := func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("always fail")
	}

	fallbackHandler := func(ctx context.Context) (interface{}, error) {
		return "fallback success", nil
	}

	errorRateCond := &TriggerCondition{
		Type:        TriggerConditionErrorRate,
		ErrorRate:   0.8,
		ErrorWindow: 10 * time.Second,
	}

	chain.RegisterStrategy("main", "Main", 0, mainHandler, errorRateCond)
	chain.RegisterStrategy("fallback", "Fallback", 1, fallbackHandler, nil)

	err := chain.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer chain.Stop()

	for i := 0; i < 5; i++ {
		result, err := chain.Execute(ctx)
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		if result != "fallback success" {
			t.Errorf("expected 'fallback success', got %v", result)
		}
	}

	rate, _ := chain.CalculateErrorRate("main", 0)
	t.Logf("main strategy error rate: %f", rate)
}

func waitForCondition(timeout time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return condition()
}

func TestCountRecentSuccessesWithAndWithoutWindow(t *testing.T) {
	ctx := context.Background()

	t.Run("without_window_uses_execute_path", func(t *testing.T) {
		cfg := DefaultChainConfig()
		cfg.Recovery.Mode = RecoveryModePassive
		cfg.Recovery.PassiveSuccessCount = 10
		cfg.Recovery.PassiveSuccessWindow = 0
		cfg.Recovery.WarmUpDuration = 0

		chain := NewChain(cfg)

		mainHandler := func(ctx context.Context) (interface{}, error) {
			return "main success", nil
		}

		fallbackHandler := func(ctx context.Context) (interface{}, error) {
			return "fallback", nil
		}

		chain.RegisterStrategy("main", "Main", 0, mainHandler, nil)
		chain.RegisterStrategy("fallback", "Fallback", 1, fallbackHandler, nil)

		err := chain.Start(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer chain.Stop()

		for i := 0; i < 15; i++ {
			result, execErr := chain.Execute(ctx)
			if execErr != nil {
				t.Fatalf("unexpected execution error on iteration %d: %v", i, execErr)
			}
			if result != "main success" {
				t.Fatalf("expected 'main success' on iteration %d, got %v", i, result)
			}
		}

		mainStrategy, _ := chain.GetStrategy("main")
		mainStrategy.mu.RLock()
		windowLen := len(mainStrategy.SuccessWindow)
		totalSuccess := mainStrategy.SuccessCount
		mainStrategy.mu.RUnlock()

		recentCount := chain.countRecentSuccesses(mainStrategy)

		if windowLen != 15 {
			t.Errorf("expected 15 entries in SuccessWindow via Execute path, got %d", windowLen)
		}
		if totalSuccess != 15 {
			t.Errorf("expected 15 total successes via Execute path, got %d", totalSuccess)
		}
		if recentCount != 15 {
			t.Errorf("expected 15 recent successes (window=0 returns window len), got %d", recentCount)
		}
	})

	t.Run("window_filtering_unit_test", func(t *testing.T) {
		cfg := DefaultChainConfig()
		cfg.Recovery.Mode = RecoveryModePassive
		cfg.Recovery.PassiveSuccessCount = 5
		cfg.Recovery.PassiveSuccessWindow = 5 * time.Second
		cfg.Recovery.WarmUpDuration = 0

		chain := NewChain(cfg)

		mainHandler := func(ctx context.Context) (interface{}, error) {
			return "main success", nil
		}

		fallbackHandler := func(ctx context.Context) (interface{}, error) {
			return "fallback", nil
		}

		chain.RegisterStrategy("main", "Main", 0, mainHandler, nil)
		chain.RegisterStrategy("fallback", "Fallback", 1, fallbackHandler, nil)

		err := chain.Start(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer chain.Stop()

		mainStrategy, _ := chain.GetStrategy("main")

		now := time.Now()
		for i := 0; i < 10; i++ {
			mainStrategy.mu.Lock()
			mainStrategy.SuccessWindow = append(mainStrategy.SuccessWindow, successEntry{
				Time: now.Add(time.Duration(i-10) * time.Second),
			})
			mainStrategy.SuccessCount++
			mainStrategy.mu.Unlock()
		}

		mainStrategy.mu.RLock()
		windowLen := len(mainStrategy.SuccessWindow)
		mainStrategy.mu.RUnlock()
		if windowLen != 10 {
			t.Fatalf("expected 10 entries in SuccessWindow, got %d", windowLen)
		}

		recentCount := chain.countRecentSuccesses(mainStrategy)
		if recentCount != 4 {
			t.Errorf("expected 4 recent successes within 5s window (entries at -4,-3,-2,-1s), got %d", recentCount)
		}
	})

	t.Run("with_window_execute_path_above_threshold_triggers_recovery", func(t *testing.T) {
		cfg := DefaultChainConfig()
		cfg.Recovery.Mode = RecoveryModePassive
		cfg.Recovery.PassiveSuccessCount = 3
		cfg.Recovery.PassiveSuccessWindow = 30 * time.Second
		cfg.Recovery.WarmUpDuration = 0

		chain := NewChain(cfg)

		mainHandler := func(ctx context.Context) (interface{}, error) {
			return "main success", nil
		}

		fallbackHandler := func(ctx context.Context) (interface{}, error) {
			return "fallback", nil
		}

		chain.RegisterStrategy("main", "Main", 0, mainHandler, nil)
		chain.RegisterStrategy("fallback", "Fallback", 1, fallbackHandler, nil)

		err := chain.Start(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer chain.Stop()

		for i := 0; i < 8; i++ {
			result, execErr := chain.Execute(ctx)
			if execErr != nil {
				t.Fatalf("unexpected execution error on iteration %d: %v", i, execErr)
			}
			if result != "main success" {
				t.Fatalf("expected 'main success' on iteration %d, got %v", i, result)
			}
		}

		mainStrategy, _ := chain.GetStrategy("main")
		mainStrategy.mu.RLock()
		consecFail := mainStrategy.ConsecutiveFail
		windowLen := len(mainStrategy.SuccessWindow)
		mainStrategy.mu.RUnlock()

		if consecFail != 0 {
			t.Fatalf("expected ConsecutiveFail=0 after successes, got %d", consecFail)
		}
		if windowLen != 8 {
			t.Fatalf("expected 8 entries in SuccessWindow via Execute, got %d", windowLen)
		}

		chain.ForceSwitchToStrategy("fallback")

		if chain.CurrentIndex() != 1 {
			t.Fatalf("expected to be on fallback strategy, got index %d", chain.CurrentIndex())
		}

		_, execErr := chain.Execute(ctx)
		if execErr != nil {
			t.Fatalf("unexpected execution error: %v", execErr)
		}

		recovered := waitForCondition(5*time.Second, func() bool {
			return chain.CurrentIndex() == 0 && chain.State() == ChainStateHealthy
		})

		if !recovered {
			t.Errorf("expected to switch back to main strategy (index 0, state HEALTHY) within timeout, got index=%d state=%v",
				chain.CurrentIndex(), chain.State())
		}
	})

	t.Run("with_window_execute_path_below_threshold_no_recovery", func(t *testing.T) {
		cfg := DefaultChainConfig()
		cfg.Recovery.Mode = RecoveryModePassive
		cfg.Recovery.PassiveSuccessCount = 20
		cfg.Recovery.PassiveSuccessWindow = 30 * time.Second
		cfg.Recovery.WarmUpDuration = 0

		chain := NewChain(cfg)

		mainHandler := func(ctx context.Context) (interface{}, error) {
			return "main success", nil
		}

		fallbackHandler := func(ctx context.Context) (interface{}, error) {
			return "fallback", nil
		}

		chain.RegisterStrategy("main", "Main", 0, mainHandler, nil)
		chain.RegisterStrategy("fallback", "Fallback", 1, fallbackHandler, nil)

		err := chain.Start(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer chain.Stop()

		for i := 0; i < 5; i++ {
			result, execErr := chain.Execute(ctx)
			if execErr != nil {
				t.Fatalf("unexpected execution error on iteration %d: %v", i, execErr)
			}
			if result != "main success" {
				t.Fatalf("expected 'main success' on iteration %d, got %v", i, result)
			}
		}

		mainStrategy, _ := chain.GetStrategy("main")
		mainStrategy.mu.RLock()
		windowLen := len(mainStrategy.SuccessWindow)
		mainStrategy.mu.RUnlock()
		if windowLen != 5 {
			t.Fatalf("expected 5 entries in SuccessWindow via Execute, got %d", windowLen)
		}

		chain.ForceSwitchToStrategy("fallback")

		result, execErr := chain.Execute(ctx)
		if execErr != nil {
			t.Fatalf("unexpected execution error: %v", execErr)
		}
		if result != "fallback" {
			t.Errorf("expected to stay on fallback when below threshold, got %v", result)
		}

		noRecovery := waitForCondition(200*time.Millisecond, func() bool {
			return chain.CurrentIndex() != 1
		})

		if noRecovery {
			t.Errorf("expected to stay on fallback strategy (index 1) when below threshold, but switched to index %d",
				chain.CurrentIndex())
		}
		if chain.CurrentIndex() != 1 {
			t.Errorf("expected fallback strategy index=1, got %d", chain.CurrentIndex())
		}
	})
}

func TestUnifiedErrorRateCalculation(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	var callCount int
	handler := func(ctx context.Context) (interface{}, error) {
		callCount++
		if callCount <= 3 {
			return nil, errors.New("failed")
		}
		return "ok", nil
	}

	chain.RegisterStrategy("s1", "Strategy 1", 0, handler, nil)
	chain.Start(ctx)
	defer chain.Stop()

	for i := 0; i < 10; i++ {
		chain.Execute(ctx)
	}

	rate1, err := chain.CalculateErrorRate("s1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rate2, err := chain.CalculateErrorRate("s1", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rate1 != rate2 {
		t.Errorf("expected consistent error rates, got window=0: %f, window=1h: %f", rate1, rate2)
	}
	if rate1 != 0.3 {
		t.Errorf("expected error rate 0.3, got %f", rate1)
	}

	strategy, _ := chain.GetStrategy("s1")
	strategy.mu.Lock()
	oldWindow := strategy.SuccessWindow
	oldErrorWindow := strategy.ErrorWindow
	strategy.SuccessWindow = []successEntry{}
	strategy.ErrorWindow = []errorEntry{}
	strategy.mu.Unlock()

	rate4, err := chain.CalculateErrorRate("s1", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate4 != 0 {
		t.Errorf("expected 0 error rate after clearing windows, got %f", rate4)
	}

	strategy.mu.Lock()
	strategy.SuccessWindow = oldWindow
	strategy.ErrorWindow = oldErrorWindow
	strategy.mu.Unlock()

	rate5, err := chain.CalculateErrorRate("s1", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate5 != rate1 {
		t.Errorf("expected consistent error rate after restoring windows, got %f, expected %f", rate5, rate1)
	}
}

func TestMatchTriggerConditionCoverage(t *testing.T) {
	chain := NewChain(nil)

	baseErr := errors.New("base error")
	wrappedErr := fmt.Errorf("wrapped: %w", baseErr)
	timeoutErr := fmt.Errorf("%w: test timed out", ErrExecutionTimeout)

	testCases := []struct {
		name     string
		err      error
		cond     *TriggerCondition
		expected bool
	}{
		{
			name:     "nil condition",
			err:      baseErr,
			cond:     nil,
			expected: false,
		},
		{
			name: "error type match",
			err:  baseErr,
			cond: &TriggerCondition{
				Type:       TriggerConditionErrorType,
				ErrorTypes: []error{baseErr},
			},
			expected: true,
		},
		{
			name: "error type mismatch",
			err:  baseErr,
			cond: &TriggerCondition{
				Type:       TriggerConditionErrorType,
				ErrorTypes: []error{errors.New("other")},
			},
			expected: false,
		},
		{
			name: "wrapped error type match",
			err:  wrappedErr,
			cond: &TriggerCondition{
				Type:       TriggerConditionErrorType,
				ErrorTypes: []error{baseErr},
			},
			expected: true,
		},
		{
			name: "timeout match",
			err:  timeoutErr,
			cond: &TriggerCondition{
				Type: TriggerConditionTimeout,
			},
			expected: true,
		},
		{
			name: "timeout mismatch",
			err:  baseErr,
			cond: &TriggerCondition{
				Type: TriggerConditionTimeout,
			},
			expected: false,
		},
		{
			name: "custom match",
			err:  baseErr,
			cond: &TriggerCondition{
				Type: TriggerConditionCustom,
				CustomCheck: func(err error) bool {
					return err.Error() == "base error"
				},
			},
			expected: true,
		},
		{
			name: "custom mismatch",
			err:  baseErr,
			cond: &TriggerCondition{
				Type: TriggerConditionCustom,
				CustomCheck: func(err error) bool {
					return false
				},
			},
			expected: false,
		},
		{
			name: "custom nil check",
			err:  baseErr,
			cond: &TriggerCondition{
				Type:        TriggerConditionCustom,
				CustomCheck: nil,
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := chain.matchTriggerCondition(tc.err, tc.cond)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestCountEventsInWindow(t *testing.T) {
	chain := NewChain(nil)
	ctx := context.Background()

	var callCount int
	handler := func(ctx context.Context) (interface{}, error) {
		callCount++
		if callCount%3 == 0 {
			return nil, errors.New("fail")
		}
		return "ok", nil
	}

	chain.RegisterStrategy("s1", "Strategy 1", 0, handler, nil)
	chain.Start(ctx)
	defer chain.Stop()

	for i := 0; i < 12; i++ {
		chain.Execute(ctx)
	}

	strategy, _ := chain.GetStrategy("s1")

	strategy.mu.RLock()
	successes, failures := chain.countEventsInWindowLocked(strategy, time.Now().Add(-time.Hour))
	strategy.mu.RUnlock()

	if successes != 8 {
		t.Errorf("expected 8 successes in window, got %d", successes)
	}
	if failures != 4 {
		t.Errorf("expected 4 failures in window, got %d", failures)
	}

	total := successes + failures
	if total != 12 {
		t.Errorf("expected 12 total events, got %d", total)
	}

	expectedRate := float64(failures) / float64(total)
	rate, _ := chain.CalculateErrorRate("s1", time.Hour)
	if rate != expectedRate {
		t.Errorf("expected rate %f, got %f", expectedRate, rate)
	}
}
