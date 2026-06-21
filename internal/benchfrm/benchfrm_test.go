package benchfrm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

var errTest = errors.New("test error")

func TestNewBenchmarker(t *testing.T) {
	b := NewBenchmarker()
	if b == nil {
		t.Fatal("expected non-nil benchmarker")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Iterations != 100 {
		t.Errorf("expected 100 iterations, got %d", cfg.Iterations)
	}
	if cfg.WarmupIterations != 10 {
		t.Errorf("expected 10 warmup iterations, got %d", cfg.WarmupIterations)
	}
	if !cfg.CollectMemory {
		t.Error("expected memory collection enabled")
	}
	if cfg.Timeout != 0 {
		t.Errorf("expected 0 timeout, got %v", cfg.Timeout)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     RunConfig
		wantErr error
	}{
		{
			name:    "valid config",
			cfg:     RunConfig{Iterations: 10, WarmupIterations: 5},
			wantErr: nil,
		},
		{
			name:    "zero iterations",
			cfg:     RunConfig{Iterations: 0, WarmupIterations: 5},
			wantErr: ErrInvalidIterations,
		},
		{
			name:    "negative iterations",
			cfg:     RunConfig{Iterations: -1, WarmupIterations: 5},
			wantErr: ErrInvalidIterations,
		},
		{
			name:    "negative warmup",
			cfg:     RunConfig{Iterations: 10, WarmupIterations: -1},
			wantErr: ErrInvalidWarmup,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestRunOptionFunctions(t *testing.T) {
	cfg := DefaultConfig()

	WithIterations(50)(&cfg)
	if cfg.Iterations != 50 {
		t.Errorf("expected 50 iterations, got %d", cfg.Iterations)
	}

	WithWarmupIterations(20)(&cfg)
	if cfg.WarmupIterations != 20 {
		t.Errorf("expected 20 warmup iterations, got %d", cfg.WarmupIterations)
	}

	WithMemoryCollection(false)(&cfg)
	if cfg.CollectMemory {
		t.Error("expected memory collection disabled")
	}

	WithTimeout(5 * time.Second)(&cfg)
	if cfg.Timeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", cfg.Timeout)
	}
}

func TestAddGroup(t *testing.T) {
	b := NewBenchmarker()

	b.AddGroup("test1", func() error { return nil })
	b.AddGroup("test2", func() error { return nil }, WithIterations(10))

	bm, ok := b.(*benchmarker)
	if !ok {
		t.Fatal("expected *benchmarker type")
	}

	if len(bm.groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(bm.groups))
	}

	if bm.groups[0].config.Iterations != 100 {
		t.Errorf("expected default 100 iterations, got %d", bm.groups[0].config.Iterations)
	}
	if bm.groups[1].config.Iterations != 10 {
		t.Errorf("expected 10 iterations, got %d", bm.groups[1].config.Iterations)
	}
}

func TestAddGroup_NilFunction(t *testing.T) {
	b := NewBenchmarker()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil function")
		} else if !errors.Is(r.(error), ErrNilFunction) {
			t.Errorf("expected ErrNilFunction, got %v", r)
		}
	}()

	b.AddGroup("test", nil)
}

func TestAddGroup_DuplicateName(t *testing.T) {
	b := NewBenchmarker()

	b.AddGroup("test", func() error { return nil })

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for duplicate name")
		} else if !errors.Is(r.(error), ErrDuplicateGroupName) {
			t.Errorf("expected ErrDuplicateGroupName, got %v", r)
		}
	}()

	b.AddGroup("test", func() error { return nil })
}

func TestAddGroup_InvalidConfig(t *testing.T) {
	b := NewBenchmarker()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid config")
		} else if !errors.Is(r.(error), ErrInvalidIterations) {
			t.Errorf("expected ErrInvalidIterations, got %v", r)
		}
	}()

	b.AddGroup("test", func() error { return nil }, WithIterations(0))
}

func TestRunAll_NoGroups(t *testing.T) {
	b := NewBenchmarker()

	_, err := b.RunAll()
	if !errors.Is(err, ErrNoGroupsRegistered) {
		t.Errorf("expected ErrNoGroupsRegistered, got %v", err)
	}
}

func TestRunAll_SingleGroup(t *testing.T) {
	b := NewBenchmarker()

	var callCount int
	b.AddGroup("test", func() error {
		callCount++
		time.Sleep(1 * time.Microsecond)
		return nil
	}, WithIterations(5), WithWarmupIterations(2))

	results, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	stats := results[0]
	if stats.Name != "test" {
		t.Errorf("expected name 'test', got %s", stats.Name)
	}
	if stats.Iterations != 5 {
		t.Errorf("expected 5 iterations, got %d", stats.Iterations)
	}
	if callCount != 5+2 {
		t.Errorf("expected 7 calls (5 + 2 warmup), got %d", callCount)
	}
	if stats.MeanDuration <= 0 {
		t.Error("expected positive mean duration")
	}
	if stats.MinDuration > stats.MaxDuration {
		t.Error("min duration should be <= max duration")
	}
	if stats.StdDevDuration < 0 {
		t.Error("std dev should be non-negative")
	}
}

func TestRunAll_MultipleGroups(t *testing.T) {
	b := NewBenchmarker()

	b.AddGroup("fast", func() error {
		time.Sleep(1 * time.Millisecond)
		return nil
	}, WithIterations(3), WithWarmupIterations(1))

	b.AddGroup("slow", func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}, WithIterations(3), WithWarmupIterations(1))

	results, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].MeanDuration >= results[1].MeanDuration {
		t.Error("expected fast group to be faster than slow group")
	}
}

func TestRunAll_FunctionError(t *testing.T) {
	b := NewBenchmarker()

	b.AddGroup("error", func() error {
		return errTest
	}, WithIterations(3), WithWarmupIterations(1))

	_, err := b.RunAll()
	if !errors.Is(err, ErrGroupEmptyResult) {
		t.Errorf("expected ErrGroupEmptyResult, got %v", err)
	}
	if !errors.Is(err, errTest) {
		t.Errorf("expected wrapped errTest, got %v", err)
	}
}

func TestRunAll_Timeout(t *testing.T) {
	b := NewBenchmarker()

	b.AddGroup("timeout", func() error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}, WithIterations(3), WithWarmupIterations(0), WithTimeout(10*time.Millisecond), WithMemoryCollection(false))

	_, err := b.RunAll()
	if !errors.Is(err, ErrGroupEmptyResult) {
		t.Errorf("expected ErrGroupEmptyResult, got %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected wrapped context.DeadlineExceeded, got %v", err)
	}
}

func TestRunAll_PartialErrors(t *testing.T) {
	b := NewBenchmarker()

	var callCount int
	b.AddGroup("partial-error", func() error {
		callCount++
		if callCount%2 == 0 {
			return errTest
		}
		time.Sleep(1 * time.Microsecond)
		return nil
	}, WithIterations(5), WithWarmupIterations(0), WithMemoryCollection(false))

	results, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 group result, got %d", len(results))
	}

	stats := results[0]
	if stats.Iterations != 3 {
		t.Errorf("expected 3 successful iterations (out of 5 with even failures), got %d", stats.Iterations)
	}
}

func TestRunAll_ErrGroupEmptyResult_AllFail(t *testing.T) {
	b := NewBenchmarker()

	b.AddGroup("all-fail", func() error {
		return errTest
	}, WithIterations(3), WithWarmupIterations(0))

	_, err := b.RunAll()
	if !errors.Is(err, ErrGroupEmptyResult) {
		t.Errorf("expected ErrGroupEmptyResult, got %v", err)
	}
	if !errors.Is(err, errTest) {
		t.Errorf("expected error to wrap original errTest, got %v", err)
	}
}

func TestErrGroupEmptyResult_Unwrap(t *testing.T) {
	b := NewBenchmarker()

	testErr := errors.New("custom failure")
	b.AddGroup("unwrap-test", func() error {
		return testErr
	}, WithIterations(2), WithWarmupIterations(0))

	_, err := b.RunAll()
	if !errors.Is(err, ErrGroupEmptyResult) {
		t.Errorf("expected ErrGroupEmptyResult, got %v", err)
	}
	if !errors.Is(err, testErr) {
		t.Errorf("expected error chain to contain testErr, got %v", err)
	}
}

func TestRunAll_NoTimeoutWorks(t *testing.T) {
	b := NewBenchmarker()

	b.AddGroup("no-timeout", func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}, WithIterations(3), WithWarmupIterations(1), WithTimeout(0))

	results, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Iterations != 3 {
		t.Errorf("expected 3 iterations, got %d", results[0].Iterations)
	}
}

func TestRunAll_ShortTimeout(t *testing.T) {
	b := NewBenchmarker()

	b.AddGroup("short-timeout", func() error {
		return nil
	}, WithIterations(3), WithWarmupIterations(1), WithTimeout(5*time.Second))

	results, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestMemoryCollection(t *testing.T) {
	b := NewBenchmarker()

	var sink []byte
	b.AddGroup("allocate", func() error {
		buf := make([]byte, 1024)
		buf[0] = 1
		sink = buf
		return nil
	}, WithIterations(10), WithWarmupIterations(2), WithMemoryCollection(true))

	results, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = sink

	stats := results[0]
	if stats.MeanAllocBytes == 0 {
		t.Error("expected memory allocation to be counted")
	}
	if stats.MeanAllocCount == 0 {
		t.Error("expected allocation count to be counted")
	}
}

func TestMemoryCollectionDisabled(t *testing.T) {
	b := NewBenchmarker()

	b.AddGroup("no-mem", func() error {
		buf := make([]byte, 1024)
		buf[0] = 1
		return nil
	}, WithIterations(10), WithWarmupIterations(2), WithMemoryCollection(false))

	results, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := results[0]
	if stats.MeanAllocBytes != 0 {
		t.Errorf("expected 0 alloc bytes when disabled, got %d", stats.MeanAllocBytes)
	}
	if stats.MeanAllocCount != 0 {
		t.Errorf("expected 0 alloc count when disabled, got %d", stats.MeanAllocCount)
	}
}

func TestWarmupNotCounted(t *testing.T) {
	b := NewBenchmarker()

	var warmupCalls, actualCalls int
	b.AddGroup("test", func() error {
		if warmupCalls < 3 {
			warmupCalls++
		} else {
			actualCalls++
		}
		return nil
	}, WithIterations(5), WithWarmupIterations(3))

	results, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if results[0].Iterations != 5 {
		t.Errorf("expected 5 iterations counted, got %d", results[0].Iterations)
	}
	if actualCalls != 5 {
		t.Errorf("expected 5 actual calls, got %d", actualCalls)
	}
	if warmupCalls != 3 {
		t.Errorf("expected 3 warmup calls, got %d", warmupCalls)
	}
}

func TestCalculateStatistics(t *testing.T) {
	results := []RunResult{
		{Duration: 100 * time.Millisecond, AllocBytes: 100, AllocCount: 1},
		{Duration: 200 * time.Millisecond, AllocBytes: 200, AllocCount: 2},
		{Duration: 300 * time.Millisecond, AllocBytes: 300, AllocCount: 3},
	}

	stats := calculateStatistics("test", results)

	if stats.Name != "test" {
		t.Errorf("expected name 'test', got %s", stats.Name)
	}
	if stats.Iterations != 3 {
		t.Errorf("expected 3 iterations, got %d", stats.Iterations)
	}
	if stats.MeanDuration != 200*time.Millisecond {
		t.Errorf("expected 200ms mean, got %v", stats.MeanDuration)
	}
	if stats.MinDuration != 100*time.Millisecond {
		t.Errorf("expected 100ms min, got %v", stats.MinDuration)
	}
	if stats.MaxDuration != 300*time.Millisecond {
		t.Errorf("expected 300ms max, got %v", stats.MaxDuration)
	}
	if stats.MeanAllocBytes != 200 {
		t.Errorf("expected 200 mean alloc bytes, got %d", stats.MeanAllocBytes)
	}
	if stats.MeanAllocCount != 2 {
		t.Errorf("expected 2 mean alloc count, got %d", stats.MeanAllocCount)
	}
}

func TestCalculateStatistics_Empty(t *testing.T) {
	stats := calculateStatistics("test", nil)
	if stats.Name != "test" {
		t.Errorf("expected name 'test', got %s", stats.Name)
	}
	if stats.Iterations != 0 {
		t.Errorf("expected 0 iterations, got %d", stats.Iterations)
	}
}

func TestCompare(t *testing.T) {
	b := NewBenchmarker()

	b.AddGroup("baseline", func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}, WithIterations(3), WithWarmupIterations(1))

	b.AddGroup("faster", func() error {
		time.Sleep(5 * time.Millisecond)
		return nil
	}, WithIterations(3), WithWarmupIterations(1))

	b.AddGroup("slower", func() error {
		time.Sleep(20 * time.Millisecond)
		return nil
	}, WithIterations(3), WithWarmupIterations(1))

	_, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := b.Compare("baseline")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Baseline != "baseline" {
		t.Errorf("expected baseline 'baseline', got %s", report.Baseline)
	}
	if len(report.Items) != 3 {
		t.Errorf("expected 3 items, got %d", len(report.Items))
	}

	var fasterItem, slowerItem *ComparisonItem
	for i := range report.Items {
		if report.Items[i].Group == "faster" {
			fasterItem = &report.Items[i]
		} else if report.Items[i].Group == "slower" {
			slowerItem = &report.Items[i]
		}
	}

	if fasterItem == nil || slowerItem == nil {
		t.Fatal("expected both faster and slower items")
	}

	if fasterItem.VsBaselinePct >= 0 {
		t.Errorf("expected faster item to have negative duration pct, got %.2f", fasterItem.VsBaselinePct)
	}
	if slowerItem.VsBaselinePct <= 0 {
		t.Errorf("expected slower item to have positive duration pct, got %.2f", slowerItem.VsBaselinePct)
	}
}

func TestCompare_NoResults(t *testing.T) {
	b := NewBenchmarker()

	_, err := b.Compare("test")
	if !errors.Is(err, ErrNoGroupsRegistered) {
		t.Errorf("expected ErrNoGroupsRegistered, got %v", err)
	}
}

func TestCompare_BaselineNotFound(t *testing.T) {
	b := NewBenchmarker()

	b.AddGroup("test", func() error { return nil }, WithIterations(1), WithWarmupIterations(0))
	_, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = b.Compare("nonexistent")
	if !errors.Is(err, ErrGroupNotFound) {
		t.Errorf("expected ErrGroupNotFound, got %v", err)
	}
}

func TestCheckMetric(t *testing.T) {
	tests := []struct {
		name        string
		current     float64
		baseline    float64
		threshold   float64
		wantDegraded bool
		wantPct     float64
	}{
		{
			name:        "no degradation",
			current:     100,
			baseline:    100,
			threshold:   10,
			wantDegraded: false,
			wantPct:     0,
		},
		{
			name:        "below threshold",
			current:     105,
			baseline:    100,
			threshold:   10,
			wantDegraded: false,
			wantPct:     5,
		},
		{
			name:        "at threshold not degraded",
			current:     110,
			baseline:    100,
			threshold:   10,
			wantDegraded: false,
			wantPct:     10,
		},
		{
			name:        "above threshold",
			current:     115,
			baseline:    100,
			threshold:   10,
			wantDegraded: true,
			wantPct:     15,
		},
		{
			name:        "improvement",
			current:     90,
			baseline:    100,
			threshold:   10,
			wantDegraded: false,
			wantPct:     -10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := checkMetric("test", tt.current, tt.baseline, tt.threshold)
			if check.IsDegraded != tt.wantDegraded {
				t.Errorf("expected degraded=%v, got %v", tt.wantDegraded, check.IsDegraded)
			}
			if check.DegradationPct != tt.wantPct {
				t.Errorf("expected pct=%.2f, got %.2f", tt.wantPct, check.DegradationPct)
			}
		})
	}
}

func TestCheckRegression_NoStore(t *testing.T) {
	b := NewBenchmarker()

	b.AddGroup("test", func() error { return nil }, WithIterations(1), WithWarmupIterations(0))
	_, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = b.CheckRegression(10)
	if !errors.Is(err, ErrNoBaselineStore) {
		t.Errorf("expected ErrNoBaselineStore, got %v", err)
	}
}

func TestCheckRegression_InvalidThreshold(t *testing.T) {
	b := NewBenchmarker()

	b.AddGroup("test", func() error { return nil }, WithIterations(1), WithWarmupIterations(0))

	_, err := b.CheckRegression(0)
	if !errors.Is(err, ErrInvalidThreshold) {
		t.Errorf("expected ErrInvalidThreshold, got %v", err)
	}

	_, err = b.CheckRegression(-1)
	if !errors.Is(err, ErrInvalidThreshold) {
		t.Errorf("expected ErrInvalidThreshold, got %v", err)
	}
}

func TestCheckRegression_NoResults(t *testing.T) {
	b := NewBenchmarker()
	b.SetBaselineStore(NewMemoryStore())

	_, err := b.CheckRegression(10)
	if !errors.Is(err, ErrNoGroupsRegistered) {
		t.Errorf("expected ErrNoGroupsRegistered, got %v", err)
	}
}

func TestCheckRegression_BaselineNotFound(t *testing.T) {
	store := NewMemoryStore()
	b := NewBenchmarker()
	b.SetBaselineStore(store)

	b.AddGroup("test", func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}, WithIterations(3), WithWarmupIterations(1))

	_, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = b.CheckRegression(10)
	if !errors.Is(err, ErrBaselineNotFound) {
		t.Errorf("expected ErrBaselineNotFound, got %v", err)
	}
}

func TestCheckRegression_NoRegression(t *testing.T) {
	store := NewMemoryStore()
	b := NewBenchmarker()
	b.SetBaselineStore(store)

	b.AddGroup("test", func() error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}, WithIterations(3), WithWarmupIterations(1))

	_, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = b.SaveBaseline()
	if err != nil {
		t.Fatalf("unexpected error saving baseline: %v", err)
	}

	report, err := b.CheckRegression(50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.IsRegression {
		t.Error("expected no regression")
	}
}

func TestCheckRegression_WithRegression(t *testing.T) {
	store := NewMemoryStore()
	b := NewBenchmarker()
	b.SetBaselineStore(store)

	sleepDuration := 50 * time.Millisecond
	b.AddGroup("test", func() error {
		time.Sleep(sleepDuration)
		return nil
	}, WithIterations(3), WithWarmupIterations(1), WithMemoryCollection(false))

	_, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = b.SaveBaseline()
	if err != nil {
		t.Fatalf("unexpected error saving baseline: %v", err)
	}

	bm, ok := b.(*benchmarker)
	if !ok {
		t.Fatal("expected *benchmarker type")
	}

	bm.mu.Lock()
	for _, g := range bm.groups {
		if g.name == "test" {
			g.fn = func() error {
				time.Sleep(sleepDuration * 3)
				return nil
			}
		}
	}
	bm.mu.Unlock()

	_, err = b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := b.CheckRegression(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !report.IsRegression {
		t.Error("expected regression detected")
	}

	foundDegraded := false
	for _, check := range report.Checks {
		if check.IsDegraded {
			foundDegraded = true
			break
		}
	}
	if !foundDegraded {
		t.Error("expected at least one degraded check")
	}
}

func TestMemoryStore(t *testing.T) {
	store := NewMemoryStore()

	stats := GroupStatistics{Name: "test", MeanDuration: 100 * time.Millisecond}

	err := store.Save("test", stats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, found, err := store.Load("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected to find saved stats")
	}
	if loaded.Name != stats.Name || loaded.MeanDuration != stats.MeanDuration {
		t.Errorf("loaded stats mismatch")
	}

	_, found, _ = store.Load("nonexistent")
	if found {
		t.Error("expected not to find nonexistent stats")
	}
}

func TestFileStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "benchfrm-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create file store: %v", err)
	}

	stats := GroupStatistics{
		Name:           "test",
		Iterations:     100,
		MeanDuration:   150 * time.Millisecond,
		MinDuration:    100 * time.Millisecond,
		MaxDuration:    200 * time.Millisecond,
		StdDevDuration: 25 * time.Millisecond,
		MeanAllocBytes: 1024,
		MeanAllocCount: 5,
	}

	err = store.Save("test", stats)
	if err != nil {
		t.Fatalf("unexpected error saving: %v", err)
	}

	loaded, found, err := store.Load("test")
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if !found {
		t.Fatal("expected to find saved stats")
	}
	if loaded.Name != stats.Name {
		t.Errorf("expected name %s, got %s", stats.Name, loaded.Name)
	}
	if loaded.MeanDuration != stats.MeanDuration {
		t.Errorf("expected mean %v, got %v", stats.MeanDuration, loaded.MeanDuration)
	}
	if loaded.MeanAllocBytes != stats.MeanAllocBytes {
		t.Errorf("expected alloc bytes %d, got %d", stats.MeanAllocBytes, loaded.MeanAllocBytes)
	}

	_, found, _ = store.Load("nonexistent")
	if found {
		t.Error("expected not to find nonexistent stats")
	}

	expectedFile := filepath.Join(tmpDir, "test.json")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist", expectedFile)
	}
}

func TestFileStore_CreateDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "benchfrm-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nestedDir := filepath.Join(tmpDir, "nested", "dir")
	_, err = NewFileStore(nestedDir)
	if err != nil {
		t.Fatalf("failed to create nested dir store: %v", err)
	}

	if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
		t.Errorf("expected nested dir to be created")
	}
}

func TestTextReporter(t *testing.T) {
	reporter := NewTextReporter()

	stats := []GroupStatistics{
		{
			Name:           "test1",
			Iterations:     100,
			MeanDuration:   150 * time.Millisecond,
			MinDuration:    100 * time.Millisecond,
			MaxDuration:    200 * time.Millisecond,
			StdDevDuration: 25 * time.Millisecond,
			MeanAllocBytes: 1024,
			MeanAllocCount: 5,
		},
	}

	report := reporter.Report(stats)
	if report == "" {
		t.Error("expected non-empty report")
	}
	if !contains(report, "test1") {
		t.Error("report should contain group name")
	}
	if !contains(report, "100") {
		t.Error("report should contain iterations")
	}
	if !contains(report, "Mean Alloc Bytes") {
		t.Error("report should contain new memory field labels")
	}
	if contains(report, "Allocs/Op") {
		t.Error("report should not contain removed Allocs/Op field")
	}
	if contains(report, "Bytes/Op") {
		t.Error("report should not contain removed Bytes/Op field")
	}

	comparison := ComparisonReport{
		Baseline: "test1",
		Items: []ComparisonItem{
			{Group: "test1", MeanDuration: 100 * time.Millisecond, MeanAllocBytes: 1000, MeanAllocCount: 10, VsBaselinePct: 0, AllocBytesPct: 0, AllocCountPct: 0},
			{Group: "test2", MeanDuration: 150 * time.Millisecond, MeanAllocBytes: 1500, MeanAllocCount: 15, VsBaselinePct: 50, AllocBytesPct: 50, AllocCountPct: 50},
		},
		GeneratedAt: time.Now(),
	}

	compReport := reporter.ReportComparison(comparison)
	if compReport == "" {
		t.Error("expected non-empty comparison report")
	}
	if !contains(compReport, "test1") || !contains(compReport, "test2") {
		t.Error("comparison report should contain group names")
	}
	if !contains(compReport, "Duration Δ%") {
		t.Error("comparison report should contain Duration Δ% column")
	}
	if !contains(compReport, "Bytes Δ%") {
		t.Error("comparison report should contain Bytes Δ% column")
	}
	if !contains(compReport, "Allocs Δ%") {
		t.Error("comparison report should contain Allocs Δ% column")
	}

	regression := RegressionReport{
		IsRegression: true,
		Checks: []RegressionCheck{
			{MetricName: "MeanDuration", CurrentValue: 150, BaselineValue: 100, DegradationPct: 50, ThresholdPct: 10, IsDegraded: true},
		},
		GeneratedAt: time.Now(),
	}

	regReport := reporter.ReportRegression(regression)
	if regReport == "" {
		t.Error("expected non-empty regression report")
	}
	if !contains(regReport, "REGRESSION DETECTED") {
		t.Error("regression report should contain warning")
	}
}

func TestTextReporter_NoRegression(t *testing.T) {
	reporter := NewTextReporter()

	regression := RegressionReport{
		IsRegression: false,
		Checks: []RegressionCheck{
			{MetricName: "MeanDuration", CurrentValue: 105, BaselineValue: 100, DegradationPct: 5, ThresholdPct: 10, IsDegraded: false},
		},
		GeneratedAt: time.Now(),
	}

	report := reporter.ReportRegression(regression)
	if !contains(report, "No performance regression") {
		t.Error("report should indicate no regression")
	}
}

func TestConcurrentAccess(t *testing.T) {
	b := NewBenchmarker()
	store := NewMemoryStore()
	b.SetBaselineStore(store)

	b.AddGroup("concurrent", func() error {
		time.Sleep(1 * time.Millisecond)
		return nil
	}, WithIterations(5), WithWarmupIterations(2))

	var wg sync.WaitGroup
	const goroutines = 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			bm, ok := b.(*benchmarker)
			if !ok {
				return
			}

			bm.mu.RLock()
			groupName := bm.groups[0].name
			bm.mu.RUnlock()

			stats := GroupStatistics{Name: groupName, MeanDuration: time.Duration(id) * time.Millisecond}
			_ = store.Save(groupName, stats)

			_, _, _ = store.Load(groupName)
		}(i)
	}

	_, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wg.Wait()
}

func TestZeroWarmupIterations(t *testing.T) {
	b := NewBenchmarker()

	var calls int
	b.AddGroup("zero-warmup", func() error {
		calls++
		return nil
	}, WithIterations(5), WithWarmupIterations(0))

	_, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls != 5 {
		t.Errorf("expected 5 calls (no warmup), got %d", calls)
	}
}

func TestSingleIteration(t *testing.T) {
	b := NewBenchmarker()

	b.AddGroup("single", func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}, WithIterations(1), WithWarmupIterations(0))

	results, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := results[0]
	if stats.Iterations != 1 {
		t.Errorf("expected 1 iteration, got %d", stats.Iterations)
	}
	if stats.MinDuration != stats.MaxDuration || stats.MinDuration != stats.MeanDuration {
		t.Error("min, max, and mean should be equal for single iteration")
	}
	if stats.StdDevDuration != 0 {
		t.Errorf("expected 0 std dev for single iteration, got %v", stats.StdDevDuration)
	}
}

func TestSetReporterAndStore(t *testing.T) {
	b := NewBenchmarker()

	reporter := NewTextReporter()
	store := NewMemoryStore()

	b.SetReporter(reporter)
	b.SetBaselineStore(store)

	bm, ok := b.(*benchmarker)
	if !ok {
		t.Fatal("expected *benchmarker type")
	}

	if bm.reporter != reporter {
		t.Error("reporter not set correctly")
	}
	if bm.baselineStore != store {
		t.Error("baseline store not set correctly")
	}
}

func TestRegressionCheck_Fields(t *testing.T) {
	check := RegressionCheck{
		MetricName:    "MeanDuration",
		CurrentValue:  150,
		BaselineValue: 100,
		DegradationPct: 50,
		ThresholdPct:  10,
		IsDegraded:    true,
	}

	if check.MetricName != "MeanDuration" {
		t.Errorf("expected MetricName 'MeanDuration', got %s", check.MetricName)
	}
	if check.CurrentValue != 150 {
		t.Errorf("expected CurrentValue 150, got %f", check.CurrentValue)
	}
	if check.BaselineValue != 100 {
		t.Errorf("expected BaselineValue 100, got %f", check.BaselineValue)
	}
	if check.DegradationPct != 50 {
		t.Errorf("expected DegradationPct 50, got %f", check.DegradationPct)
	}
	if check.ThresholdPct != 10 {
		t.Errorf("expected ThresholdPct 10, got %f", check.ThresholdPct)
	}
	if !check.IsDegraded {
		t.Error("expected IsDegraded true")
	}
}

func TestGroupStatistics_Fields(t *testing.T) {
	stats := GroupStatistics{
		Name:           "test",
		Iterations:     100,
		MeanDuration:   150 * time.Millisecond,
		MinDuration:    100 * time.Millisecond,
		MaxDuration:    200 * time.Millisecond,
		StdDevDuration: 25 * time.Millisecond,
		MeanAllocBytes: 1024,
		MeanAllocCount: 5,
	}

	if stats.Name != "test" {
		t.Errorf("expected Name 'test', got %s", stats.Name)
	}
	if stats.Iterations != 100 {
		t.Errorf("expected Iterations 100, got %d", stats.Iterations)
	}
	if stats.MeanDuration != 150*time.Millisecond {
		t.Errorf("expected MeanDuration 150ms, got %v", stats.MeanDuration)
	}
	if stats.MeanAllocBytes != 1024 {
		t.Errorf("expected MeanAllocBytes 1024, got %d", stats.MeanAllocBytes)
	}
	if stats.MeanAllocCount != 5 {
		t.Errorf("expected MeanAllocCount 5, got %d", stats.MeanAllocCount)
	}
}

func TestRunResult_Fields(t *testing.T) {
	result := RunResult{
		Duration:   100 * time.Millisecond,
		AllocBytes: 512,
		AllocCount: 3,
		Error:      nil,
	}

	if result.Duration != 100*time.Millisecond {
		t.Errorf("expected Duration 100ms, got %v", result.Duration)
	}
	if result.AllocBytes != 512 {
		t.Errorf("expected AllocBytes 512, got %d", result.AllocBytes)
	}
	if result.AllocCount != 3 {
		t.Errorf("expected AllocCount 3, got %d", result.AllocCount)
	}
	if result.Error != nil {
		t.Errorf("expected nil Error, got %v", result.Error)
	}
}

func TestComparisonReport_Fields(t *testing.T) {
	now := time.Now()
	report := ComparisonReport{
		Baseline:    "base",
		GeneratedAt: now,
		Items: []ComparisonItem{
			{Group: "item1", MeanDuration: 100 * time.Millisecond, MeanAllocBytes: 1000, MeanAllocCount: 10, VsBaselinePct: 0, AllocBytesPct: 0, AllocCountPct: 0},
		},
	}

	if report.Baseline != "base" {
		t.Errorf("expected Baseline 'base', got %s", report.Baseline)
	}
	if !report.GeneratedAt.Equal(now) {
		t.Errorf("expected GeneratedAt %v, got %v", now, report.GeneratedAt)
	}
	if len(report.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(report.Items))
	}
	if report.Items[0].AllocBytesPct != 0 {
		t.Errorf("expected AllocBytesPct 0, got %f", report.Items[0].AllocBytesPct)
	}
	if report.Items[0].AllocCountPct != 0 {
		t.Errorf("expected AllocCountPct 0, got %f", report.Items[0].AllocCountPct)
	}
}

func TestSaveBaseline(t *testing.T) {
	store := NewMemoryStore()
	b := NewBenchmarker()
	b.SetBaselineStore(store)

	b.AddGroup("test", func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}, WithIterations(3), WithWarmupIterations(1))

	_, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = b.SaveBaseline()
	if err != nil {
		t.Fatalf("unexpected error saving baseline: %v", err)
	}

	stats, found, err := store.Load("test")
	if err != nil {
		t.Fatalf("unexpected error loading baseline: %v", err)
	}
	if !found {
		t.Fatal("expected baseline to be found")
	}
	if stats.Name != "test" {
		t.Errorf("expected name 'test', got %s", stats.Name)
	}
}

func TestSaveBaseline_NoStore(t *testing.T) {
	b := NewBenchmarker()

	b.AddGroup("test", func() error { return nil }, WithIterations(1), WithWarmupIterations(0))
	_, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = b.SaveBaseline()
	if !errors.Is(err, ErrNoBaselineStore) {
		t.Errorf("expected ErrNoBaselineStore, got %v", err)
	}
}

func TestSaveBaseline_NoResults(t *testing.T) {
	store := NewMemoryStore()
	b := NewBenchmarker()
	b.SetBaselineStore(store)

	b.AddGroup("test", func() error { return nil }, WithIterations(1), WithWarmupIterations(0))

	err := b.SaveBaseline()
	if !errors.Is(err, ErrNoGroupsRegistered) {
		t.Errorf("expected ErrNoGroupsRegistered, got %v", err)
	}
}

func TestLoadBaseline(t *testing.T) {
	store := NewMemoryStore()
	b := NewBenchmarker()
	b.SetBaselineStore(store)

	b.AddGroup("test1", func() error { return nil }, WithIterations(1), WithWarmupIterations(0))
	b.AddGroup("test2", func() error { return nil }, WithIterations(1), WithWarmupIterations(0))

	baseline1 := GroupStatistics{Name: "test1", MeanDuration: 100 * time.Millisecond}
	baseline2 := GroupStatistics{Name: "test2", MeanDuration: 200 * time.Millisecond}
	_ = store.Save("test1", baseline1)
	_ = store.Save("test2", baseline2)

	baselines, err := b.LoadBaseline()
	if err != nil {
		t.Fatalf("unexpected error loading baselines: %v", err)
	}

	if len(baselines) != 2 {
		t.Errorf("expected 2 baselines, got %d", len(baselines))
	}

	if b1, ok := baselines["test1"]; !ok || b1.MeanDuration != baseline1.MeanDuration {
		t.Errorf("baseline test1 mismatch")
	}
	if b2, ok := baselines["test2"]; !ok || b2.MeanDuration != baseline2.MeanDuration {
		t.Errorf("baseline test2 mismatch")
	}
}

func TestLoadBaseline_NoStore(t *testing.T) {
	b := NewBenchmarker()

	b.AddGroup("test", func() error { return nil }, WithIterations(1), WithWarmupIterations(0))

	_, err := b.LoadBaseline()
	if !errors.Is(err, ErrNoBaselineStore) {
		t.Errorf("expected ErrNoBaselineStore, got %v", err)
	}
}

func TestErrorVariables(t *testing.T) {
	errTests := []struct {
		name     string
		err      error
		wantMsg  string
	}{
		{"ErrNoGroupsRegistered", ErrNoGroupsRegistered, "no benchmark groups registered"},
		{"ErrGroupNotFound", ErrGroupNotFound, "benchmark group not found"},
		{"ErrInvalidIterations", ErrInvalidIterations, "invalid number of iterations"},
		{"ErrInvalidWarmup", ErrInvalidWarmup, "invalid number of warmup iterations"},
		{"ErrInvalidThreshold", ErrInvalidThreshold, "invalid regression threshold"},
		{"ErrNilFunction", ErrNilFunction, "benchmark function cannot be nil"},
		{"ErrDuplicateGroupName", ErrDuplicateGroupName, "duplicate group name"},
		{"ErrNoBaselineStore", ErrNoBaselineStore, "no baseline store configured"},
		{"ErrBaselineNotFound", ErrBaselineNotFound, "baseline not found for group"},
		{"ErrGroupEmptyResult", ErrGroupEmptyResult, "group has no valid results"},
	}

	for _, tt := range errTests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Error("error should not be nil")
			}
			if !contains(tt.err.Error(), tt.wantMsg) {
				t.Errorf("expected error message to contain %q, got %q", tt.wantMsg, tt.err.Error())
			}
		})
	}
}

func TestTimeoutOptionIntegration(t *testing.T) {
	b := NewBenchmarker()

	b.AddGroup("with-timeout", func() error {
		time.Sleep(1 * time.Millisecond)
		return nil
	}, WithIterations(5), WithWarmupIterations(2), WithTimeout(1*time.Second), WithMemoryCollection(false))

	bm, ok := b.(*benchmarker)
	if !ok {
		t.Fatal("expected *benchmarker type")
	}

	if len(bm.groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(bm.groups))
	}

	if bm.groups[0].config.Timeout != 1*time.Second {
		t.Errorf("expected 1s timeout, got %v", bm.groups[0].config.Timeout)
	}
	if bm.groups[0].config.Iterations != 5 {
		t.Errorf("expected 5 iterations, got %d", bm.groups[0].config.Iterations)
	}
	if bm.groups[0].config.WarmupIterations != 2 {
		t.Errorf("expected 2 warmup iterations, got %d", bm.groups[0].config.WarmupIterations)
	}
	if bm.groups[0].config.CollectMemory != false {
		t.Errorf("expected CollectMemory false, got %v", bm.groups[0].config.CollectMemory)
	}
}

func TestCompare_Percentages(t *testing.T) {
	b := NewBenchmarker()

	var sink1, sink2 []byte
	b.AddGroup("baseline", func() error {
		buf := make([]byte, 1000)
		buf[0] = 1
		sink1 = buf
		time.Sleep(10 * time.Millisecond)
		return nil
	}, WithIterations(3), WithWarmupIterations(1))

	b.AddGroup("compare", func() error {
		buf := make([]byte, 1500)
		buf[0] = 1
		sink2 = buf
		time.Sleep(15 * time.Millisecond)
		return nil
	}, WithIterations(3), WithWarmupIterations(1))

	_, err := b.RunAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = sink1
	_ = sink2

	report, err := b.Compare("baseline")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, item := range report.Items {
		if item.Group == "compare" {
			if item.VsBaselinePct <= 0 {
				t.Errorf("expected positive duration pct, got %.2f", item.VsBaselinePct)
			}
			if item.AllocBytesPct <= 0 {
				t.Errorf("expected positive alloc bytes pct, got %.2f", item.AllocBytesPct)
			}
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
