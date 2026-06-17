package windowagg

import (
	"sync"
	"testing"
	"time"
)

func TestCountAggregator(t *testing.T) {
	agg := NewCountAggregator()
	if agg.Name() != "count" {
		t.Errorf("expected name 'count', got %s", agg.Name())
	}

	result, err := agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 0 {
		t.Errorf("expected initial result 0, got %f", result)
	}

	agg.Add(1.0)
	agg.Add(2.0)
	agg.Add(3.0)
	result, err = agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 3 {
		t.Errorf("expected count 3, got %f", result)
	}

	agg.Remove(1.0)
	result, err = agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 2 {
		t.Errorf("expected count 2 after remove, got %f", result)
	}

	agg.Reset()
	result, err = agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 0 {
		t.Errorf("expected count 0 after reset, got %f", result)
	}
}

func TestSumAggregator(t *testing.T) {
	agg := NewSumAggregator()
	if agg.Name() != "sum" {
		t.Errorf("expected name 'sum', got %s", agg.Name())
	}

	result, err := agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 0 {
		t.Errorf("expected initial sum 0, got %f", result)
	}

	agg.Add(1.5)
	agg.Add(2.5)
	agg.Add(3.0)
	result, err = agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 7.0 {
		t.Errorf("expected sum 7.0, got %f", result)
	}

	agg.Remove(2.5)
	result, err = agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 4.5 {
		t.Errorf("expected sum 4.5 after remove, got %f", result)
	}

	agg.Reset()
	result, err = agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 0 {
		t.Errorf("expected sum 0 after reset, got %f", result)
	}
}

func TestAvgAggregator(t *testing.T) {
	agg := NewAvgAggregator()
	if agg.Name() != "avg" {
		t.Errorf("expected name 'avg', got %s", agg.Name())
	}

	_, err := agg.Result()
	if err != ErrEmptyWindow {
		t.Errorf("expected ErrEmptyWindow for empty aggregator, got %v", err)
	}

	agg.Add(2.0)
	agg.Add(4.0)
	agg.Add(6.0)
	result, err := agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 4.0 {
		t.Errorf("expected avg 4.0, got %f", result)
	}

	agg.Remove(6.0)
	result, err = agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 3.0 {
		t.Errorf("expected avg 3.0 after remove, got %f", result)
	}

	agg.Reset()
	_, err = agg.Result()
	if err != ErrEmptyWindow {
		t.Errorf("expected ErrEmptyWindow after reset, got %v", err)
	}
}

func TestMaxAggregator(t *testing.T) {
	agg := NewMaxAggregator()
	if agg.Name() != "max" {
		t.Errorf("expected name 'max', got %s", agg.Name())
	}

	_, err := agg.Result()
	if err != ErrEmptyWindow {
		t.Errorf("expected ErrEmptyWindow for empty aggregator, got %v", err)
	}

	agg.Add(3.0)
	agg.Add(1.0)
	agg.Add(5.0)
	agg.Add(2.0)
	result, err := agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 5.0 {
		t.Errorf("expected max 5.0, got %f", result)
	}

	agg.Remove(5.0)
	result, err = agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 3.0 {
		t.Errorf("expected max 3.0 after removing max, got %f", result)
	}

	agg.Reset()
	_, err = agg.Result()
	if err != ErrEmptyWindow {
		t.Errorf("expected ErrEmptyWindow after reset, got %v", err)
	}
}

func TestMinAggregator(t *testing.T) {
	agg := NewMinAggregator()
	if agg.Name() != "min" {
		t.Errorf("expected name 'min', got %s", agg.Name())
	}

	_, err := agg.Result()
	if err != ErrEmptyWindow {
		t.Errorf("expected ErrEmptyWindow for empty aggregator, got %v", err)
	}

	agg.Add(3.0)
	agg.Add(1.0)
	agg.Add(5.0)
	agg.Add(2.0)
	result, err := agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 1.0 {
		t.Errorf("expected min 1.0, got %f", result)
	}

	agg.Remove(1.0)
	result, err = agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 2.0 {
		t.Errorf("expected min 2.0 after removing min, got %f", result)
	}

	agg.Reset()
	_, err = agg.Result()
	if err != ErrEmptyWindow {
		t.Errorf("expected ErrEmptyWindow after reset, got %v", err)
	}
}

func TestNewAggregator(t *testing.T) {
	tests := []struct {
		aggType     AggregatorType
		expectedErr error
	}{
		{AggregatorCount, nil},
		{AggregatorSum, nil},
		{AggregatorAvg, nil},
		{AggregatorMax, nil},
		{AggregatorMin, nil},
		{AggregatorType(99), ErrInvalidAggregator},
	}

	for _, tt := range tests {
		t.Run(tt.aggType.String(), func(t *testing.T) {
			agg, err := NewAggregator(tt.aggType)
			if tt.expectedErr != nil {
				if err != tt.expectedErr {
					t.Errorf("expected error %v, got %v", tt.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if agg == nil {
					t.Error("expected non-nil aggregator")
				}
			}
		})
	}
}

func TestAggregatorTypeString(t *testing.T) {
	tests := []struct {
		aggType  AggregatorType
		expected string
	}{
		{AggregatorCount, "count"},
		{AggregatorSum, "sum"},
		{AggregatorAvg, "avg"},
		{AggregatorMax, "max"},
		{AggregatorMin, "min"},
		{AggregatorType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.aggType.String() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.aggType.String())
			}
		})
	}
}

func TestNewSlidingWindow(t *testing.T) {
	tests := []struct {
		name        string
		cfg         WindowConfig
		expectError bool
	}{
		{
			"valid count window",
			WindowConfig{WindowType: WindowTypeCount, AggregatorType: AggregatorSum, Size: 10, Slide: 5},
			false,
		},
		{
			"valid time window",
			WindowConfig{WindowType: WindowTypeTime, AggregatorType: AggregatorAvg, Size: 1000, Slide: 500},
			false,
		},
		{
			"slide equals size",
			WindowConfig{WindowType: WindowTypeCount, AggregatorType: AggregatorCount, Size: 10, Slide: 10},
			false,
		},
		{
			"invalid window type",
			WindowConfig{WindowType: WindowType(99), AggregatorType: AggregatorSum, Size: 10, Slide: 5},
			true,
		},
		{
			"invalid size zero",
			WindowConfig{WindowType: WindowTypeCount, AggregatorType: AggregatorSum, Size: 0, Slide: 5},
			true,
		},
		{
			"invalid size negative",
			WindowConfig{WindowType: WindowTypeCount, AggregatorType: AggregatorSum, Size: -1, Slide: 5},
			true,
		},
		{
			"invalid slide zero",
			WindowConfig{WindowType: WindowTypeCount, AggregatorType: AggregatorSum, Size: 10, Slide: 0},
			true,
		},
		{
			"slide greater than size",
			WindowConfig{WindowType: WindowTypeCount, AggregatorType: AggregatorSum, Size: 5, Slide: 10},
			true,
		},
		{
			"invalid aggregator type",
			WindowConfig{WindowType: WindowTypeCount, AggregatorType: AggregatorType(99), Size: 10, Slide: 5},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, err := NewSlidingWindow("test", tt.cfg)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if w == nil {
					t.Error("expected non-nil window")
				}
				if w.Name() != "test" {
					t.Errorf("expected name 'test', got %s", w.Name())
				}
			}
		})
	}
}

func TestCountWindowTumbling(t *testing.T) {
	w, err := NewSlidingWindow("test", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorSum,
		Size:           3,
		Slide:          3,
	})
	if err != nil {
		t.Fatalf("NewSlidingWindow failed: %v", err)
	}

	now := time.Now()

	w.AddValue(1.0, now)
	w.AddValue(2.0, now)
	result, err := w.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 3.0 {
		t.Errorf("expected sum 3.0, got %f", result)
	}
	if w.Count() != 2 {
		t.Errorf("expected count 2, got %d", w.Count())
	}

	w.AddValue(3.0, now)
	result, err = w.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 6.0 {
		t.Errorf("expected sum 6.0, got %f", result)
	}
	if w.Count() != 3 {
		t.Errorf("expected count 3, got %d", w.Count())
	}

	w.AddValue(4.0, now)
	result, err = w.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Count() != 1 {
		t.Errorf("expected count 1 after slide, got %d", w.Count())
	}
}

func TestCountWindowSliding(t *testing.T) {
	w, err := NewSlidingWindow("test", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorSum,
		Size:           3,
		Slide:          1,
	})
	if err != nil {
		t.Fatalf("NewSlidingWindow failed: %v", err)
	}

	now := time.Now()

	w.AddValue(1.0, now)
	w.AddValue(2.0, now)
	w.AddValue(3.0, now)
	result, err := w.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 6.0 {
		t.Errorf("expected sum 6.0, got %f", result)
	}
	if w.Count() != 3 {
		t.Errorf("expected count 3, got %d", w.Count())
	}

	w.AddValue(4.0, now)
	result, err = w.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 9.0 {
		t.Errorf("expected sum 9.0 (2+3+4), got %f", result)
	}
	if w.Count() != 3 {
		t.Errorf("expected count 3, got %d", w.Count())
	}

	w.AddValue(5.0, now)
	result, err = w.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 12.0 {
		t.Errorf("expected sum 12.0 (3+4+5), got %f", result)
	}
}

func TestCountWindowSlidingOverlap(t *testing.T) {
	w, err := NewSlidingWindow("test", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorCount,
		Size:           4,
		Slide:          2,
	})
	if err != nil {
		t.Fatalf("NewSlidingWindow failed: %v", err)
	}

	now := time.Now()

	w.AddValue(1.0, now)
	w.AddValue(2.0, now)
	if w.Count() != 2 {
		t.Errorf("expected count 2, got %d", w.Count())
	}

	w.AddValue(3.0, now)
	w.AddValue(4.0, now)
	if w.Count() != 4 {
		t.Errorf("expected count 4, got %d", w.Count())
	}

	w.AddValue(5.0, now)
	if w.Count() != 3 {
		t.Errorf("expected count 3, got %d", w.Count())
	}

	w.AddValue(6.0, now)
	if w.Count() != 4 {
		t.Errorf("expected count 4, got %d", w.Count())
	}
}

func TestTimeWindow(t *testing.T) {
	w, err := NewSlidingWindow("test", WindowConfig{
		WindowType:     WindowTypeTime,
		AggregatorType: AggregatorSum,
		Size:           100,
		Slide:          100,
	})
	if err != nil {
		t.Fatalf("NewSlidingWindow failed: %v", err)
	}

	w.AddValue(1.0, time.Now())
	w.AddValue(2.0, time.Now())
	result, err := w.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 3.0 {
		t.Errorf("expected sum 3.0, got %f", result)
	}
	if w.Count() != 2 {
		t.Errorf("expected count 2, got %d", w.Count())
	}

	w.AddValue(3.0, time.Now())
	w.AddValue(4.0, time.Now())
	if w.Count() != 4 {
		t.Errorf("expected count 4, got %d", w.Count())
	}

	time.Sleep(120 * time.Millisecond)

	result, err = w.Result()
	if err != nil {
		t.Fatalf("unexpected error after sleep: %v", err)
	}
	if result != 0.0 {
		t.Errorf("expected sum 0.0 after real time expiry, got %f", result)
	}
	if w.Count() != 0 {
		t.Errorf("expected count 0 after real time expiry, got %d", w.Count())
	}
}

func TestTimeWindowAllItemsEvicted(t *testing.T) {
	w, err := NewSlidingWindow("test", WindowConfig{
		WindowType:     WindowTypeTime,
		AggregatorType: AggregatorSum,
		Size:           50,
		Slide:          50,
	})
	if err != nil {
		t.Fatalf("NewSlidingWindow failed: %v", err)
	}

	w.AddValue(1.0, time.Now())
	w.AddValue(2.0, time.Now())

	time.Sleep(60 * time.Millisecond)

	w.AddValue(3.0, time.Now())
	result, err := w.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 3.0 {
		t.Errorf("expected sum 3.0 (only newest), got %f", result)
	}
	if w.Count() != 1 {
		t.Errorf("expected count 1, got %d", w.Count())
	}

	time.Sleep(60 * time.Millisecond)

	result, err = w.Result()
	if err != nil {
		t.Fatalf("unexpected error after sleep: %v", err)
	}
	if result != 0.0 {
		t.Errorf("expected sum 0.0 after real time expiry, got %f", result)
	}
	if w.Count() != 0 {
		t.Errorf("expected count 0 after real time expiry, got %d", w.Count())
	}
}

func TestTimeWindowResultEvictsExpiredData(t *testing.T) {
	w, err := NewSlidingWindow("test", WindowConfig{
		WindowType:     WindowTypeTime,
		AggregatorType: AggregatorSum,
		Size:           50,
		Slide:          50,
	})
	if err != nil {
		t.Fatalf("NewSlidingWindow failed: %v", err)
	}

	now := time.Now()
	w.AddValue(10.0, now)
	w.AddValue(20.0, now)

	result, err := w.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 30.0 {
		t.Errorf("expected sum 30.0 before expiry, got %f", result)
	}
	if w.Count() != 2 {
		t.Errorf("expected count 2 before expiry, got %d", w.Count())
	}

	time.Sleep(60 * time.Millisecond)

	result, err = w.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 0.0 {
		t.Errorf("expected sum 0.0 after expiry, got %f", result)
	}
	if w.Count() != 0 {
		t.Errorf("expected count 0 after expiry, got %d", w.Count())
	}
}

func TestTimeWindowSlidingPartialExpiry(t *testing.T) {
	w, err := NewSlidingWindow("test", WindowConfig{
		WindowType:     WindowTypeTime,
		AggregatorType: AggregatorSum,
		Size:           200,
		Slide:          50,
	})
	if err != nil {
		t.Fatalf("NewSlidingWindow failed: %v", err)
	}

	w.AddValue(1.0, time.Now())
	w.AddValue(2.0, time.Now())

	result, err := w.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 3.0 {
		t.Errorf("expected sum 3.0 after first batch, got %f", result)
	}
	if w.Count() != 2 {
		t.Errorf("expected count 2 after first batch, got %d", w.Count())
	}

	time.Sleep(80 * time.Millisecond)

	w.AddValue(10.0, time.Now())

	result, err = w.Result()
	if err != nil {
		t.Fatalf("unexpected error after second batch: %v", err)
	}
	if result != 13.0 {
		t.Errorf("expected sum 13.0 after second batch (all data in window), got %f", result)
	}
	if w.Count() != 3 {
		t.Errorf("expected count 3 after second batch, got %d", w.Count())
	}

	time.Sleep(150 * time.Millisecond)

	result, err = w.Result()
	if err != nil {
		t.Fatalf("unexpected error after sleep: %v", err)
	}
	if result != 10.0 {
		t.Errorf("expected sum 10.0 after partial expiry (only 10.0 remains), got %f", result)
	}
	if w.Count() != 1 {
		t.Errorf("expected count 1 after partial expiry, got %d", w.Count())
	}
}

func TestSlidingWindowReset(t *testing.T) {
	w, err := NewSlidingWindow("test", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorAvg,
		Size:           5,
		Slide:          5,
	})
	if err != nil {
		t.Fatalf("NewSlidingWindow failed: %v", err)
	}

	now := time.Now()
	w.AddValue(1.0, now)
	w.AddValue(2.0, now)

	if w.Count() != 2 {
		t.Errorf("expected count 2, got %d", w.Count())
	}

	w.Reset()

	if w.Count() != 0 {
		t.Errorf("expected count 0 after reset, got %d", w.Count())
	}

	_, err = w.Result()
	if err != ErrEmptyWindow {
		t.Errorf("expected ErrEmptyWindow after reset, got %v", err)
	}
}

func TestSlidingWindowConfig(t *testing.T) {
	cfg := WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorAvg,
		Size:           10,
		Slide:          5,
	}
	w, err := NewSlidingWindow("test", cfg)
	if err != nil {
		t.Fatalf("NewSlidingWindow failed: %v", err)
	}

	gotCfg := w.Config()
	if gotCfg.WindowType != cfg.WindowType {
		t.Errorf("expected WindowType %v, got %v", cfg.WindowType, gotCfg.WindowType)
	}
	if gotCfg.AggregatorType != cfg.AggregatorType {
		t.Errorf("expected AggregatorType %v, got %v", cfg.AggregatorType, gotCfg.AggregatorType)
	}
	if gotCfg.Size != cfg.Size {
		t.Errorf("expected Size %d, got %d", cfg.Size, gotCfg.Size)
	}
	if gotCfg.Slide != cfg.Slide {
		t.Errorf("expected Slide %d, got %d", cfg.Slide, gotCfg.Slide)
	}
}

func TestWindowManager(t *testing.T) {
	mgr := NewWindowManager()

	if mgr.WindowCount() != 0 {
		t.Errorf("expected 0 windows, got %d", mgr.WindowCount())
	}

	err := mgr.AddWindow("sum-5", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorSum,
		Size:           5,
		Slide:          5,
	})
	if err != nil {
		t.Fatalf("AddWindow failed: %v", err)
	}

	if mgr.WindowCount() != 1 {
		t.Errorf("expected 1 window, got %d", mgr.WindowCount())
	}

	err = mgr.AddWindow("avg-3", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorAvg,
		Size:           3,
		Slide:          1,
	})
	if err != nil {
		t.Fatalf("AddWindow failed: %v", err)
	}

	if mgr.WindowCount() != 2 {
		t.Errorf("expected 2 windows, got %d", mgr.WindowCount())
	}
}

func TestWindowManagerAddDuplicate(t *testing.T) {
	mgr := NewWindowManager()

	err := mgr.AddWindow("test", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorSum,
		Size:           5,
		Slide:          5,
	})
	if err != nil {
		t.Fatalf("AddWindow failed: %v", err)
	}

	err = mgr.AddWindow("test", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorCount,
		Size:           3,
		Slide:          3,
	})
	if err != ErrWindowExists {
		t.Errorf("expected ErrWindowExists, got %v", err)
	}
}

func TestWindowManagerAddInvalidConfig(t *testing.T) {
	mgr := NewWindowManager()

	err := mgr.AddWindow("test", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorSum,
		Size:           0,
		Slide:          5,
	})
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestWindowManagerRemoveWindow(t *testing.T) {
	mgr := NewWindowManager()

	mgr.AddWindow("test", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorSum,
		Size:           5,
		Slide:          5,
	})

	err := mgr.RemoveWindow("test")
	if err != nil {
		t.Fatalf("RemoveWindow failed: %v", err)
	}

	if mgr.WindowCount() != 0 {
		t.Errorf("expected 0 windows after remove, got %d", mgr.WindowCount())
	}
}

func TestWindowManagerRemoveNotFound(t *testing.T) {
	mgr := NewWindowManager()

	err := mgr.RemoveWindow("nonexistent")
	if err != ErrWindowNotFound {
		t.Errorf("expected ErrWindowNotFound, got %v", err)
	}
}

func TestWindowManagerGetWindow(t *testing.T) {
	mgr := NewWindowManager()

	mgr.AddWindow("test", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorSum,
		Size:           5,
		Slide:          5,
	})

	w, err := mgr.GetWindow("test")
	if err != nil {
		t.Fatalf("GetWindow failed: %v", err)
	}
	if w == nil {
		t.Error("expected non-nil window")
	}
	if w.Name() != "test" {
		t.Errorf("expected name 'test', got %s", w.Name())
	}
}

func TestWindowManagerGetWindowNotFound(t *testing.T) {
	mgr := NewWindowManager()

	_, err := mgr.GetWindow("nonexistent")
	if err != ErrWindowNotFound {
		t.Errorf("expected ErrWindowNotFound, got %v", err)
	}
}

func TestWindowManagerPushAndResults(t *testing.T) {
	mgr := NewWindowManager()

	mgr.AddWindow("sum-3", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorSum,
		Size:           3,
		Slide:          3,
	})
	mgr.AddWindow("count-5", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorCount,
		Size:           5,
		Slide:          5,
	})
	mgr.AddWindow("avg-2", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorAvg,
		Size:           2,
		Slide:          1,
	})

	now := time.Now()
	mgr.Push(1.0, now)
	mgr.Push(2.0, now)
	mgr.Push(3.0, now)

	sumResult, err := mgr.GetResult("sum-3")
	if err != nil {
		t.Fatalf("GetResult sum-3 failed: %v", err)
	}
	if sumResult != 6.0 {
		t.Errorf("expected sum 6.0, got %f", sumResult)
	}

	countResult, err := mgr.GetResult("count-5")
	if err != nil {
		t.Fatalf("GetResult count-5 failed: %v", err)
	}
	if countResult != 3.0 {
		t.Errorf("expected count 3, got %f", countResult)
	}

	avgResult, err := mgr.GetResult("avg-2")
	if err != nil {
		t.Fatalf("GetResult avg-2 failed: %v", err)
	}
	if avgResult != 2.5 {
		t.Errorf("expected avg 2.5, got %f", avgResult)
	}

	allResults := mgr.GetAllResults()
	if len(allResults) != 3 {
		t.Errorf("expected 3 results, got %d", len(allResults))
	}
}

func TestWindowManagerGetResultNotFound(t *testing.T) {
	mgr := NewWindowManager()

	_, err := mgr.GetResult("nonexistent")
	if err != ErrWindowNotFound {
		t.Errorf("expected ErrWindowNotFound, got %v", err)
	}
}

func TestWindowManagerReset(t *testing.T) {
	mgr := NewWindowManager()

	mgr.AddWindow("test1", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorSum,
		Size:           5,
		Slide:          5,
	})
	mgr.AddWindow("test2", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorCount,
		Size:           3,
		Slide:          3,
	})

	now := time.Now()
	mgr.Push(1.0, now)
	mgr.Push(2.0, now)

	mgr.Reset()

	w1, _ := mgr.GetWindow("test1")
	if w1.Count() != 0 {
		t.Errorf("expected test1 count 0 after reset, got %d", w1.Count())
	}

	w2, _ := mgr.GetWindow("test2")
	if w2.Count() != 0 {
		t.Errorf("expected test2 count 0 after reset, got %d", w2.Count())
	}
}

func TestWindowManagerResetWindow(t *testing.T) {
	mgr := NewWindowManager()

	mgr.AddWindow("test1", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorSum,
		Size:           5,
		Slide:          5,
	})
	mgr.AddWindow("test2", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorCount,
		Size:           3,
		Slide:          3,
	})

	now := time.Now()
	mgr.Push(1.0, now)
	mgr.Push(2.0, now)

	err := mgr.ResetWindow("test1")
	if err != nil {
		t.Fatalf("ResetWindow failed: %v", err)
	}

	w1, _ := mgr.GetWindow("test1")
	if w1.Count() != 0 {
		t.Errorf("expected test1 count 0 after reset, got %d", w1.Count())
	}

	w2, _ := mgr.GetWindow("test2")
	if w2.Count() != 2 {
		t.Errorf("expected test2 count 2 (unchanged), got %d", w2.Count())
	}
}

func TestWindowManagerResetWindowNotFound(t *testing.T) {
	mgr := NewWindowManager()

	err := mgr.ResetWindow("nonexistent")
	if err != ErrWindowNotFound {
		t.Errorf("expected ErrWindowNotFound, got %v", err)
	}
}

func TestMaxAggregatorRemoveNonMax(t *testing.T) {
	agg := NewMaxAggregator()
	agg.Add(5.0)
	agg.Add(3.0)
	agg.Add(1.0)

	agg.Remove(3.0)
	result, err := agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 5.0 {
		t.Errorf("expected max 5.0 after removing non-max, got %f", result)
	}
}

func TestMinAggregatorRemoveNonMin(t *testing.T) {
	agg := NewMinAggregator()
	agg.Add(1.0)
	agg.Add(3.0)
	agg.Add(5.0)

	agg.Remove(3.0)
	result, err := agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 1.0 {
		t.Errorf("expected min 1.0 after removing non-min, got %f", result)
	}
}

func TestAvgAggregatorRemoveAll(t *testing.T) {
	agg := NewAvgAggregator()
	agg.Add(1.0)
	agg.Add(2.0)
	agg.Remove(1.0)
	agg.Remove(2.0)
	agg.Remove(3.0)

	_, err := agg.Result()
	if err != ErrEmptyWindow {
		t.Errorf("expected ErrEmptyWindow after removing all, got %v", err)
	}
}

func TestCountAggregatorUnderflow(t *testing.T) {
	agg := NewCountAggregator()
	agg.Add(1.0)
	agg.Remove(1.0)
	agg.Remove(1.0)

	result, err := agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 0 {
		t.Errorf("expected count 0 (no underflow), got %f", result)
	}
}

func TestSlidingWindowAddValueWithSeq(t *testing.T) {
	w, err := NewSlidingWindow("test", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorSum,
		Size:           3,
		Slide:          3,
	})
	if err != nil {
		t.Fatalf("NewSlidingWindow failed: %v", err)
	}

	now := time.Now()
	w.AddValueWithSeq(1.0, now, 10)
	w.AddValueWithSeq(2.0, now, 11)
	w.AddValueWithSeq(3.0, now, 12)

	result, err := w.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 6.0 {
		t.Errorf("expected sum 6.0, got %f", result)
	}
}

func TestWindowManagerPushWithSeq(t *testing.T) {
	mgr := NewWindowManager()

	mgr.AddWindow("sum-3", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorSum,
		Size:           3,
		Slide:          3,
	})

	now := time.Now()
	mgr.PushWithSeq(1.0, now, 1)
	mgr.PushWithSeq(2.0, now, 2)
	mgr.PushWithSeq(3.0, now, 3)

	result, err := mgr.GetResult("sum-3")
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}
	if result != 6.0 {
		t.Errorf("expected sum 6.0, got %f", result)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		cfg         WindowConfig
		expectError bool
	}{
		{"valid count", WindowConfig{WindowType: WindowTypeCount, Size: 10, Slide: 5}, false},
		{"valid time", WindowConfig{WindowType: WindowTypeTime, Size: 1000, Slide: 500}, false},
		{"invalid type", WindowConfig{WindowType: WindowType(99), Size: 10, Slide: 5}, true},
		{"size zero", WindowConfig{WindowType: WindowTypeCount, Size: 0, Slide: 5}, true},
		{"size negative", WindowConfig{WindowType: WindowTypeCount, Size: -1, Slide: 5}, true},
		{"slide zero", WindowConfig{WindowType: WindowTypeCount, Size: 10, Slide: 0}, true},
		{"slide negative", WindowConfig{WindowType: WindowTypeCount, Size: 10, Slide: -1}, true},
		{"slide > size", WindowConfig{WindowType: WindowTypeCount, Size: 5, Slide: 10}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.cfg)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestConcurrentWindowAccess(t *testing.T) {
	w, err := NewSlidingWindow("test", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorSum,
		Size:           100,
		Slide:          50,
	})
	if err != nil {
		t.Fatalf("NewSlidingWindow failed: %v", err)
	}

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n * 2)
	now := time.Now()

	for i := 0; i < n; i++ {
		go func(val float64) {
			defer wg.Done()
			w.AddValue(val, now)
		}(float64(i))
	}

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			w.Result()
			w.Count()
		}()
	}

	wg.Wait()
}

func TestConcurrentWindowManager(t *testing.T) {
	mgr := NewWindowManager()

	mgr.AddWindow("w1", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorSum,
		Size:           50,
		Slide:          25,
	})
	mgr.AddWindow("w2", WindowConfig{
		WindowType:     WindowTypeCount,
		AggregatorType: AggregatorCount,
		Size:           30,
		Slide:          15,
	})

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n * 2)
	now := time.Now()

	for i := 0; i < n; i++ {
		go func(val float64) {
			defer wg.Done()
			mgr.Push(val, now)
		}(float64(i))
	}

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			mgr.GetAllResults()
			mgr.WindowCount()
		}()
	}

	wg.Wait()

	if mgr.WindowCount() != 2 {
		t.Errorf("expected 2 windows, got %d", mgr.WindowCount())
	}
}

func TestEmptyWindowResult(t *testing.T) {
	tests := []struct {
		name       string
		aggType    AggregatorType
		canBeEmpty bool
	}{
		{"count", AggregatorCount, true},
		{"sum", AggregatorSum, true},
		{"avg", AggregatorAvg, false},
		{"max", AggregatorMax, false},
		{"min", AggregatorMin, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agg, _ := NewAggregator(tt.aggType)
			_, err := agg.Result()
			if tt.canBeEmpty {
				if err != nil {
					t.Errorf("expected no error for %s, got %v", tt.name, err)
				}
			} else {
				if err != ErrEmptyWindow {
					t.Errorf("expected ErrEmptyWindow for %s, got %v", tt.name, err)
				}
			}
		})
	}
}

func TestMaxAggregatorRemoveMiddle(t *testing.T) {
	agg := NewMaxAggregator()
	agg.Add(1.0)
	agg.Add(5.0)
	agg.Add(3.0)
	agg.Add(4.0)
	agg.Add(2.0)

	agg.Remove(5.0)
	result, err := agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 4.0 {
		t.Errorf("expected max 4.0, got %f", result)
	}
}

func TestMinAggregatorRemoveMiddle(t *testing.T) {
	agg := NewMinAggregator()
	agg.Add(5.0)
	agg.Add(1.0)
	agg.Add(3.0)
	agg.Add(2.0)
	agg.Add(4.0)

	agg.Remove(1.0)
	result, err := agg.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 2.0 {
		t.Errorf("expected min 2.0, got %f", result)
	}
}
