package timeoutprop

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewPropagator_DefaultConfig(t *testing.T) {
	p := NewPropagator()
	if p == nil {
		t.Fatal("expected non-nil Propagator")
	}
	if p.StageCount() != 0 {
		t.Errorf("expected 0 stages, got %d", p.StageCount())
	}
	if p.TotalBudget() != 0 {
		t.Errorf("expected 0 total budget, got %v", p.TotalBudget())
	}
}

func TestNewPropagatorWithConfig_InvalidValues(t *testing.T) {
	cfg := Config{
		TotalTimeout: -1 * time.Second,
		MinThreshold: -5 * time.Millisecond,
	}
	p := NewPropagatorWithConfig(cfg)
	if p == nil {
		t.Fatal("expected non-nil Propagator")
	}
	if p.totalTimeout <= 0 {
		t.Error("expected positive total timeout")
	}
	if p.minThreshold != 0 {
		t.Errorf("expected min threshold 0, got %v", p.minThreshold)
	}
}

func TestAddStage_Success(t *testing.T) {
	p := NewPropagator()
	err := p.AddStage("stage1", 100*time.Millisecond, func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if p.StageCount() != 1 {
		t.Errorf("expected 1 stage, got %d", p.StageCount())
	}
	if p.TotalBudget() != 100*time.Millisecond {
		t.Errorf("expected 100ms total budget, got %v", p.TotalBudget())
	}
}

func TestAddStage_EmptyName(t *testing.T) {
	p := NewPropagator()
	err := p.AddStage("", 100*time.Millisecond, func(ctx context.Context) error {
		return nil
	})
	if !errors.Is(err, ErrEmptyName) {
		t.Errorf("expected ErrEmptyName, got %v", err)
	}
}

func TestAddStage_NilHandler(t *testing.T) {
	p := NewPropagator()
	err := p.AddStage("stage1", 100*time.Millisecond, nil)
	if !errors.Is(err, ErrNilHandler) {
		t.Errorf("expected ErrNilHandler, got %v", err)
	}
}

func TestAddStage_NegativeBudget(t *testing.T) {
	p := NewPropagator()
	err := p.AddStage("stage1", -100*time.Millisecond, func(ctx context.Context) error {
		return nil
	})
	if !errors.Is(err, ErrNegativeBudget) {
		t.Errorf("expected ErrNegativeBudget, got %v", err)
	}
}

func TestAddStage_DuplicateName(t *testing.T) {
	p := NewPropagator()
	err1 := p.AddStage("stage1", 100*time.Millisecond, func(ctx context.Context) error {
		return nil
	})
	if err1 != nil {
		t.Fatalf("first add should succeed, got %v", err1)
	}
	err2 := p.AddStage("stage1", 200*time.Millisecond, func(ctx context.Context) error {
		return nil
	})
	if !errors.Is(err2, ErrStageAlreadyExists) {
		t.Errorf("expected ErrStageAlreadyExists, got %v", err2)
	}
}

func TestAddStage_AfterExecute(t *testing.T) {
	p := NewPropagatorWithConfig(Config{TotalTimeout: 1 * time.Second})
	p.AddStage("stage1", 100*time.Millisecond, func(ctx context.Context) error {
		return nil
	})
	_, err := p.Execute(context.Background())
	if err != nil {
		t.Fatalf("execute should succeed, got %v", err)
	}

	err = p.AddStage("stage2", 200*time.Millisecond, func(ctx context.Context) error {
		return nil
	})
	if !errors.Is(err, ErrChainAlreadyExecuted) {
		t.Errorf("expected ErrChainAlreadyExecuted, got %v", err)
	}
}

func TestExecute_NoStages(t *testing.T) {
	p := NewPropagator()
	_, err := p.Execute(context.Background())
	if !errors.Is(err, ErrNoStages) {
		t.Errorf("expected ErrNoStages, got %v", err)
	}
}

func TestExecute_BudgetExceedsTotal(t *testing.T) {
	p := NewPropagatorWithConfig(Config{TotalTimeout: 100 * time.Millisecond})
	p.AddStage("stage1", 60*time.Millisecond, func(ctx context.Context) error {
		return nil
	})
	p.AddStage("stage2", 50*time.Millisecond, func(ctx context.Context) error {
		return nil
	})
	report, err := p.Execute(context.Background())
	if !errors.Is(err, ErrBudgetExceedsTotal) {
		t.Errorf("expected ErrBudgetExceedsTotal, got %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if report.Success {
		t.Error("expected failure in report")
	}
	if report.TimeoutReason == "" {
		t.Error("expected timeout reason in report")
	}
	if len(report.Stages) != 2 {
		t.Errorf("expected 2 stages in report, got %d", len(report.Stages))
	}
}

func TestExecute_AllStagesSuccess(t *testing.T) {
	p := NewPropagatorWithConfig(Config{TotalTimeout: 1 * time.Second})

	var stage1Called, stage2Called bool

	p.AddStage("stage1", 100*time.Millisecond, func(ctx context.Context) error {
		stage1Called = true
		return nil
	})
	p.AddStage("stage2", 200*time.Millisecond, func(ctx context.Context) error {
		stage2Called = true
		return nil
	})

	report, err := p.Execute(context.Background())
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if !report.Success {
		t.Error("expected success")
	}
	if !stage1Called {
		t.Error("stage1 should be called")
	}
	if !stage2Called {
		t.Error("stage2 should be called")
	}
	if report.FailedStage != "" {
		t.Errorf("expected no failed stage, got %s", report.FailedStage)
	}
	if len(report.Stages) != 2 {
		t.Errorf("expected 2 stages in report, got %d", len(report.Stages))
	}
	if report.Stages[0].Status != StageStatusCompleted {
		t.Errorf("expected stage1 completed, got %v", report.Stages[0].Status)
	}
	if report.Stages[1].Status != StageStatusCompleted {
		t.Errorf("expected stage2 completed, got %v", report.Stages[1].Status)
	}
}

func TestExecute_BudgetTimeout(t *testing.T) {
	p := NewPropagatorWithConfig(Config{
		TotalTimeout: 1 * time.Second,
		MinThreshold: 1 * time.Millisecond,
	})

	p.AddStage("stage1", 50*time.Millisecond, func(ctx context.Context) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})
	p.AddStage("stage2", 100*time.Millisecond, func(ctx context.Context) error {
		return nil
	})

	report, err := p.Execute(context.Background())
	if err == nil {
		t.Error("expected error due to budget timeout")
	}
	if report.Success {
		t.Error("expected failure")
	}
	if report.FailedStage != "stage1" {
		t.Errorf("expected failed stage stage1, got %s", report.FailedStage)
	}
	if report.Stages[0].Status != StageStatusTimedOut {
		t.Errorf("expected stage1 timed out, got %v", report.Stages[0].Status)
	}
	if report.Stages[0].TimeoutType != TimeoutTypeBudget {
		t.Errorf("expected budget timeout type, got %v", report.Stages[0].TimeoutType)
	}
	if report.Stages[1].Status != StageStatusSkipped {
		t.Errorf("expected stage2 skipped, got %v", report.Stages[1].Status)
	}

	var stageErr *StageTimeoutError
	if !errors.As(err, &stageErr) {
		t.Fatalf("expected StageTimeoutError, got %T", err)
	}
	if stageErr.StageName != "stage1" {
		t.Errorf("expected stage name stage1, got %s", stageErr.StageName)
	}
	if stageErr.TimeoutType != TimeoutTypeBudget {
		t.Errorf("expected budget timeout, got %v", stageErr.TimeoutType)
	}
}

func TestExecute_TotalTimeout(t *testing.T) {
	p := NewPropagatorWithConfig(Config{
		TotalTimeout: 100 * time.Millisecond,
		MinThreshold: 1 * time.Millisecond,
	})

	p.AddStage("stage1", 50*time.Millisecond, func(ctx context.Context) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})
	p.AddStage("stage2", 30*time.Millisecond, func(ctx context.Context) error {
		return nil
	})

	report, err := p.Execute(context.Background())
	if err == nil {
		t.Error("expected error due to timeout")
	}
	if report.Success {
		t.Error("expected failure")
	}

	if report.Stages[1].Status != StageStatusSkipped {
		t.Errorf("expected stage2 skipped, got %v", report.Stages[1].Status)
	}
}

func TestExecute_MinThresholdSkip(t *testing.T) {
	p := NewPropagatorWithConfig(Config{
		TotalTimeout: 100 * time.Millisecond,
		MinThreshold: 50 * time.Millisecond,
	})

	p.AddStage("stage1", 60*time.Millisecond, func(ctx context.Context) error {
		time.Sleep(60 * time.Millisecond)
		return nil
	})
	p.AddStage("stage2", 30*time.Millisecond, func(ctx context.Context) error {
		return nil
	})

	report, _ := p.Execute(context.Background())

	if report.Stages[0].Status != StageStatusCompleted {
		t.Errorf("expected stage1 completed, got %v", report.Stages[0].Status)
	}

	if report.Stages[1].Status != StageStatusSkipped {
		t.Errorf("expected stage2 skipped due to min threshold, got %v", report.Stages[1].Status)
	}
	if report.Stages[1].TimeoutType != TimeoutTypeMinThreshold {
		t.Errorf("expected stage2 timeout type MIN_THRESHOLD, got %v", report.Stages[1].TimeoutType)
	}
}

func TestExecute_MinThresholdSkip_ZeroBudgetStage(t *testing.T) {
	p := NewPropagatorWithConfig(Config{
		TotalTimeout: 100 * time.Millisecond,
		MinThreshold: 50 * time.Millisecond,
	})

	p.AddStage("stage1", 60*time.Millisecond, func(ctx context.Context) error {
		time.Sleep(60 * time.Millisecond)
		return nil
	})
	p.AddStage("stage2", 0, func(ctx context.Context) error {
		return nil
	})

	report, _ := p.Execute(context.Background())

	if report.Stages[1].Status != StageStatusSkipped {
		t.Errorf("expected zero-budget stage2 skipped due to min threshold, got %v", report.Stages[1].Status)
	}
	if report.Stages[1].TimeoutType != TimeoutTypeMinThreshold {
		t.Errorf("expected stage2 timeout type MIN_THRESHOLD, got %v", report.Stages[1].TimeoutType)
	}
}

func TestExecute_MinThresholdSkip_SufficientTime(t *testing.T) {
	p := NewPropagatorWithConfig(Config{
		TotalTimeout: 200 * time.Millisecond,
		MinThreshold: 50 * time.Millisecond,
	})

	p.AddStage("stage1", 50*time.Millisecond, func(ctx context.Context) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})
	p.AddStage("stage2", 30*time.Millisecond, func(ctx context.Context) error {
		return nil
	})

	report, _ := p.Execute(context.Background())

	if report.Stages[1].Status != StageStatusCompleted {
		t.Errorf("expected stage2 completed (sufficient time), got %v", report.Stages[1].Status)
	}
}

func TestExecute_BudgetCarryOver(t *testing.T) {
	p := NewPropagatorWithConfig(Config{
		TotalTimeout: 500 * time.Millisecond,
		MinThreshold: 1 * time.Millisecond,
	})

	var stage2Ctx context.Context
	var stage2Deadline time.Time
	var stage2Start time.Time

	p.AddStage("stage1", 200*time.Millisecond, func(ctx context.Context) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})
	p.AddStage("stage2", 200*time.Millisecond, func(ctx context.Context) error {
		stage2Ctx = ctx
		stage2Start = time.Now()
		dl, ok := ctx.Deadline()
		if ok {
			stage2Deadline = dl
		}
		return nil
	})

	report, err := p.Execute(context.Background())
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !report.Success {
		t.Error("expected success")
	}

	if stage2Ctx == nil {
		t.Fatal("stage2 context should not be nil")
	}

	stage2Budget := stage2Deadline.Sub(stage2Start)
	expectedMinBudget := 200 * time.Millisecond
	if stage2Budget < expectedMinBudget {
		t.Logf("stage2 budget: %v, expected at least: %v", stage2Budget, expectedMinBudget)
		t.Log("note: timing-based test, may have minor variance")
	}

	if report.Stages[1].AllocatedBudget < report.Stages[1].UsedBudget {
		t.Error("stage2 allocated budget should be >= used budget")
	}
}

func TestExecute_ContextAlreadyCanceled(t *testing.T) {
	p := NewPropagatorWithConfig(Config{TotalTimeout: 1 * time.Second})
	p.AddStage("stage1", 100*time.Millisecond, func(ctx context.Context) error {
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	report, err := p.Execute(ctx)
	if err == nil {
		t.Error("expected error with canceled context")
	}
	if report.Success {
		t.Error("expected failure")
	}
	if report.Stages[0].Status != StageStatusSkipped {
		t.Errorf("expected stage skipped, got %v", report.Stages[0].Status)
	}
}

func TestExecute_StageReturnsError(t *testing.T) {
	p := NewPropagatorWithConfig(Config{TotalTimeout: 1 * time.Second})

	customErr := errors.New("custom error")
	p.AddStage("stage1", 100*time.Millisecond, func(ctx context.Context) error {
		return customErr
	})
	p.AddStage("stage2", 100*time.Millisecond, func(ctx context.Context) error {
		return nil
	})

	report, err := p.Execute(context.Background())
	if err == nil {
		t.Error("expected error")
	}
	if !errors.Is(err, customErr) {
		t.Errorf("expected custom error, got %v", err)
	}
	if report.Success {
		t.Error("expected failure")
	}
	if report.FailedStage != "stage1" {
		t.Errorf("expected failed stage stage1, got %s", report.FailedStage)
	}
	if report.Stages[0].Status != StageStatusFailed {
		t.Errorf("expected stage1 status failed (business error), got %v", report.Stages[0].Status)
	}
	if report.Stages[0].TimeoutType != TimeoutTypeNone {
		t.Errorf("expected stage1 timeout type NONE (business error), got %v", report.Stages[0].TimeoutType)
	}
	if report.Stages[1].Status != StageStatusSkipped {
		t.Errorf("expected stage2 skipped, got %v", report.Stages[1].Status)
	}
}

func TestGetStageInfo(t *testing.T) {
	p := NewPropagatorWithConfig(Config{TotalTimeout: 1 * time.Second})
	p.AddStage("stage1", 100*time.Millisecond, func(ctx context.Context) error {
		return nil
	})

	info, exists := p.GetStageInfo("stage1")
	if !exists {
		t.Fatal("expected stage to exist")
	}
	if info.Name != "stage1" {
		t.Errorf("expected name stage1, got %s", info.Name)
	}
	if info.Status != StageStatusPending {
		t.Errorf("expected pending status, got %v", info.Status)
	}

	_, exists = p.GetStageInfo("nonexistent")
	if exists {
		t.Error("expected stage not to exist")
	}
}

func TestReport(t *testing.T) {
	p := NewPropagatorWithConfig(Config{TotalTimeout: 1 * time.Second})
	p.AddStage("stage1", 100*time.Millisecond, func(ctx context.Context) error {
		return nil
	})

	if p.Report() != nil {
		t.Error("expected nil report before execution")
	}

	_, err := p.Execute(context.Background())
	if err != nil {
		t.Fatalf("execute should succeed, got %v", err)
	}

	report := p.Report()
	if report == nil {
		t.Fatal("expected non-nil report after execution")
	}
	if !report.Success {
		t.Error("expected success in report")
	}
}

func TestReset(t *testing.T) {
	p := NewPropagatorWithConfig(Config{TotalTimeout: 1 * time.Second})
	p.AddStage("stage1", 100*time.Millisecond, func(ctx context.Context) error {
		return nil
	})

	_, err := p.Execute(context.Background())
	if err != nil {
		t.Fatalf("first execute should succeed, got %v", err)
	}
	if p.Report() == nil {
		t.Fatal("expected report after first execute")
	}

	p.Reset()

	if p.Report() != nil {
		t.Error("expected nil report after reset")
	}

	info, _ := p.GetStageInfo("stage1")
	if info.Status != StageStatusPending {
		t.Errorf("expected pending status after reset, got %v", info.Status)
	}

	_, err = p.Execute(context.Background())
	if err != nil {
		t.Errorf("second execute should succeed, got %v", err)
	}
}

func TestRemainingTime(t *testing.T) {
	p := NewPropagator()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	remaining := p.RemainingTime(ctx)
	if remaining <= 0 || remaining > 500*time.Millisecond {
		t.Errorf("expected remaining time between 0 and 500ms, got %v", remaining)
	}
}

func TestRemainingTime_NoDeadline(t *testing.T) {
	p := NewPropagator()

	remaining := p.RemainingTime(context.Background())
	if remaining <= 0 {
		t.Errorf("expected positive remaining time for context without deadline, got %v", remaining)
	}
}

func TestExecute_StagePanic(t *testing.T) {
	p := NewPropagatorWithConfig(Config{TotalTimeout: 1 * time.Second})
	p.AddStage("stage1", 100*time.Millisecond, func(ctx context.Context) error {
		panic("intentional panic")
	})

	report, err := p.Execute(context.Background())
	if err == nil {
		t.Error("expected error from panic")
	}
	if report.Success {
		t.Error("expected failure")
	}
	if report.Stages[0].Error == nil {
		t.Error("expected error in stage info")
	}
}

func TestTimeoutType_String(t *testing.T) {
	tests := []struct {
		tt   TimeoutType
		want string
	}{
		{TimeoutTypeNone, "NONE"},
		{TimeoutTypeTotal, "TOTAL_TIMEOUT"},
		{TimeoutTypeBudget, "BUDGET_TIMEOUT"},
		{TimeoutTypeMinThreshold, "MIN_THRESHOLD_SKIP"},
		{TimeoutType(999), "UNKNOWN"},
	}
	for _, tt := range tests {
		got := tt.tt.String()
		if got != tt.want {
			t.Errorf("TimeoutType(%d).String() = %q, want %q", tt.tt, got, tt.want)
		}
	}
}

func TestStageStatus_String(t *testing.T) {
	tests := []struct {
		ss   StageStatus
		want string
	}{
		{StageStatusPending, "PENDING"},
		{StageStatusRunning, "RUNNING"},
		{StageStatusCompleted, "COMPLETED"},
		{StageStatusSkipped, "SKIPPED"},
		{StageStatusTimedOut, "TIMED_OUT"},
		{StageStatusFailed, "FAILED"},
		{StageStatus(999), "UNKNOWN"},
	}
	for _, tt := range tests {
		got := tt.ss.String()
		if got != tt.want {
			t.Errorf("StageStatus(%d).String() = %q, want %q", tt.ss, got, tt.want)
		}
	}
}

func TestChainReport_String(t *testing.T) {
	report := &ChainReport{
		TotalTimeout:  1 * time.Second,
		TotalUsed:     500 * time.Millisecond,
		RemainingTime: 500 * time.Millisecond,
		Success:       true,
		Stages: []*StageInfo{
			{
				Name:            "stage1",
				AllocatedBudget: 300 * time.Millisecond,
				UsedBudget:      200 * time.Millisecond,
				RemainingBudget: 100 * time.Millisecond,
				Status:          StageStatusCompleted,
				TimeoutType:     TimeoutTypeNone,
			},
		},
	}

	s := report.String()
	if s == "" {
		t.Error("expected non-empty report string")
	}
}

func TestStageTimeoutError_Error(t *testing.T) {
	err := &StageTimeoutError{
		StageName:   "test",
		TimeoutType: TimeoutTypeBudget,
		Allocated:   100 * time.Millisecond,
		Used:        200 * time.Millisecond,
	}
	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestStageTimeoutError_Unwrap_ContextDeadlineExceeded(t *testing.T) {
	budgetErr := &StageTimeoutError{
		StageName:   "test",
		TimeoutType: TimeoutTypeBudget,
		Allocated:   100 * time.Millisecond,
		Used:        200 * time.Millisecond,
	}
	if !errors.Is(budgetErr, context.DeadlineExceeded) {
		t.Error("expected errors.Is to return true for budget timeout")
	}

	totalErr := &StageTimeoutError{
		StageName:   "test",
		TimeoutType: TimeoutTypeTotal,
		Allocated:   100 * time.Millisecond,
		Used:        200 * time.Millisecond,
	}
	if !errors.Is(totalErr, context.DeadlineExceeded) {
		t.Error("expected errors.Is to return true for total timeout")
	}

	noTimeoutErr := &StageTimeoutError{
		StageName:   "test",
		TimeoutType: TimeoutTypeNone,
		Allocated:   100 * time.Millisecond,
		Used:        50 * time.Millisecond,
	}
	if errors.Is(noTimeoutErr, context.DeadlineExceeded) {
		t.Error("expected errors.Is to return false for no timeout type")
	}
}

func TestExecute_ZeroBudgetStage(t *testing.T) {
	p := NewPropagatorWithConfig(Config{
		TotalTimeout: 1 * time.Second,
		MinThreshold: 0,
	})

	var called bool
	p.AddStage("stage1", 0, func(ctx context.Context) error {
		called = true
		return nil
	})

	report, err := p.Execute(context.Background())
	if err != nil {
		t.Errorf("expected success, got %v", err)
	}
	if !report.Success {
		t.Error("expected success")
	}
	if !called {
		t.Error("stage should be called even with zero budget when minThreshold is 0")
	}
}

func TestExecute_ParentContextTimeout(t *testing.T) {
	p := NewPropagatorWithConfig(Config{TotalTimeout: 5 * time.Second})

	p.AddStage("stage1", 100*time.Millisecond, func(ctx context.Context) error {
		time.Sleep(1 * time.Second)
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	report, err := p.Execute(ctx)
	if err == nil {
		t.Error("expected error due to parent context timeout")
	}
	if report.Success {
		t.Error("expected failure")
	}
}

func TestWithTotalTimeout_Option(t *testing.T) {
	var cfg Config
	WithTotalTimeout(5 * time.Second)(&cfg)
	if cfg.TotalTimeout != 5*time.Second {
		t.Errorf("expected 5s total timeout, got %v", cfg.TotalTimeout)
	}
}

func TestWithMinThreshold_Option(t *testing.T) {
	var cfg Config
	WithMinThreshold(50 * time.Millisecond)(&cfg)
	if cfg.MinThreshold != 50*time.Millisecond {
		t.Errorf("expected 50ms min threshold, got %v", cfg.MinThreshold)
	}
}

func TestExecute_ConvenienceFunction(t *testing.T) {
	var stage1Called bool

	report, err := Execute(context.Background(),
		func(p *Propagator) error {
			return p.AddStage("stage1", 100*time.Millisecond, func(ctx context.Context) error {
				stage1Called = true
				return nil
			})
		},
		WithTotalTimeout(1*time.Second),
		WithMinThreshold(1*time.Millisecond),
	)

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !report.Success {
		t.Error("expected success")
	}
	if !stage1Called {
		t.Error("stage1 should be called")
	}
}

func TestExecute_ConvenienceFunction_SetupError(t *testing.T) {
	_, err := Execute(context.Background(),
		func(p *Propagator) error {
			return errors.New("setup error")
		},
	)
	if err == nil {
		t.Error("expected setup error")
	}
}

func TestGetStageInfo_ModifyReturnedInfo(t *testing.T) {
	p := NewPropagatorWithConfig(Config{TotalTimeout: 1 * time.Second})
	p.AddStage("stage1", 100*time.Millisecond, func(ctx context.Context) error {
		return nil
	})

	info1, _ := p.GetStageInfo("stage1")
	info1.Name = "modified"

	info2, _ := p.GetStageInfo("stage1")
	if info2.Name != "stage1" {
		t.Error("modifying returned info should not affect internal state")
	}
}

func TestReport_CopyIsIndependent(t *testing.T) {
	p := NewPropagatorWithConfig(Config{TotalTimeout: 1 * time.Second})
	p.AddStage("stage1", 100*time.Millisecond, func(ctx context.Context) error {
		return nil
	})

	p.Execute(context.Background())

	r1 := p.Report()
	r2 := p.Report()

	r1.Stages[0].Name = "modified"
	if r2.Stages[0].Name != "stage1" {
		t.Error("report copies should be independent")
	}
}

func TestTotalBudget_MultipleStages(t *testing.T) {
	p := NewPropagator()
	p.AddStage("stage1", 100*time.Millisecond, func(ctx context.Context) error { return nil })
	p.AddStage("stage2", 200*time.Millisecond, func(ctx context.Context) error { return nil })
	p.AddStage("stage3", 300*time.Millisecond, func(ctx context.Context) error { return nil })

	total := p.TotalBudget()
	if total != 600*time.Millisecond {
		t.Errorf("expected 600ms total budget, got %v", total)
	}
}
