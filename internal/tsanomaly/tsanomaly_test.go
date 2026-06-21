package tsanomaly

import (
	"errors"
	"math"
	"sync"
	"testing"
	"time"
)

const epsilon = 1e-9

func floatApproxEqual(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr error
	}{
		{
			name: "valid default config",
			cfg:  DefaultConfig(),
		},
		{
			name: "invalid window size zero",
			cfg: Config{
				WindowSize:   0,
				StdDevFactor: 3.0,
				MinSamples:   2,
			},
			wantErr: ErrInvalidWindowSize,
		},
		{
			name: "invalid window size negative",
			cfg: Config{
				WindowSize:   -5,
				StdDevFactor: 3.0,
				MinSamples:   2,
			},
			wantErr: ErrInvalidWindowSize,
		},
		{
			name: "invalid stddev factor negative",
			cfg: Config{
				WindowSize:   10,
				StdDevFactor: -1.0,
				MinSamples:   2,
			},
			wantErr: ErrInvalidStdDevFactor,
		},
		{
			name: "invalid min samples zero",
			cfg: Config{
				WindowSize:   10,
				StdDevFactor: 3.0,
				MinSamples:   0,
			},
			wantErr: ErrInvalidMinSamples,
		},
		{
			name: "invalid min samples greater than window",
			cfg: Config{
				WindowSize:   10,
				StdDevFactor: 3.0,
				MinSamples:   11,
			},
			wantErr: ErrInvalidMinSamples,
		},
		{
			name: "seasonal mode without period length",
			cfg: Config{
				WindowSize:     10,
				StdDevFactor:   3.0,
				MinSamples:     2,
				EnableSeasonal: true,
				PeriodLength:   0,
				PeriodSlot:     time.Hour,
			},
			wantErr: ErrInvalidPeriodLength,
		},
		{
			name: "seasonal mode with negative period",
			cfg: Config{
				WindowSize:     10,
				StdDevFactor:   3.0,
				MinSamples:     2,
				EnableSeasonal: true,
				PeriodLength:   -1,
				PeriodSlot:     time.Hour,
			},
			wantErr: ErrInvalidPeriodLength,
		},
		{
			name: "seasonal mode without period slot",
			cfg: Config{
				WindowSize:     10,
				StdDevFactor:   3.0,
				MinSamples:     2,
				EnableSeasonal: true,
				PeriodLength:   24,
				PeriodSlot:     0,
			},
			wantErr: ErrInvalidPeriodSlot,
		},
		{
			name: "seasonal mode with negative period slot",
			cfg: Config{
				WindowSize:     10,
				StdDevFactor:   3.0,
				MinSamples:     2,
				EnableSeasonal: true,
				PeriodLength:   24,
				PeriodSlot:     -time.Hour,
			},
			wantErr: ErrInvalidPeriodSlot,
		},
		{
			name: "valid seasonal config",
			cfg: Config{
				WindowSize:     100,
				StdDevFactor:   3.0,
				MinSamples:     10,
				EnableSeasonal: true,
				PeriodLength:   24,
				PeriodSlot:     time.Hour,
				SeasonalEpoch:  time.Unix(0, 0).UTC(),
				Direction:      DirectionBoth,
			},
		},
		{
			name: "zero stddev factor is valid",
			cfg: Config{
				WindowSize:   10,
				StdDevFactor: 0,
				MinSamples:   2,
			},
		},
		{
			name: "invalid direction",
			cfg: Config{
				WindowSize:   10,
				StdDevFactor: 3.0,
				MinSamples:   2,
				Direction:    DeviationDirection(99),
			},
			wantErr: ErrInvalidDirection,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.cfg)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidateConfig() error = %v, want %v", err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateConfig() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestNewDetector(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := DefaultConfig()
		d, err := NewDetector(cfg)
		if err != nil {
			t.Fatalf("NewDetector() error = %v", err)
		}
		if d == nil {
			t.Fatal("NewDetector() returned nil")
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		cfg := Config{WindowSize: 0}
		d, err := NewDetector(cfg)
		if err == nil {
			t.Error("NewDetector() expected error for invalid config")
		}
		if d != nil {
			t.Error("NewDetector() expected nil detector for invalid config")
		}
	})

	t.Run("with default", func(t *testing.T) {
		d := NewDetectorWithDefault()
		if d == nil {
			t.Fatal("NewDetectorWithDefault() returned nil")
		}
	})
}

func TestDetector_Add_NilPoint(t *testing.T) {
	d := NewDetectorWithDefault()
	event, err := d.Add(nil)
	if !errors.Is(err, ErrNilDataPoint) {
		t.Errorf("Add(nil) error = %v, want %v", err, ErrNilDataPoint)
	}
	if event != nil {
		t.Error("Add(nil) expected nil event")
	}
}

func TestDetector_Add_ClosedDetector(t *testing.T) {
	d := NewDetectorWithDefault()
	d.Close()
	point := &DataPoint{Timestamp: time.Now(), Value: 1.0}
	event, err := d.Add(point)
	if !errors.Is(err, ErrDetectorClosed) {
		t.Errorf("Add() on closed detector error = %v, want %v", err, ErrDetectorClosed)
	}
	if event != nil {
		t.Error("Add() on closed detector expected nil event")
	}
}

func TestDetector_Add_InsufficientSamples(t *testing.T) {
	cfg := Config{
		WindowSize:        10,
		StdDevFactor:      2.0,
		MinSamples:        5,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	for i := 0; i < 4; i++ {
		point := &DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Value:     10.0,
		}
		event, err := d.Add(point)
		if err != nil {
			t.Fatalf("Add() error at i=%d: %v", i, err)
		}
		if event != nil {
			t.Errorf("Add() at i=%d expected nil event during warmup, got %+v", i, event)
		}
	}
}

func generateNormalData(mean, stddev float64, n int, baseTime time.Time) []*DataPoint {
	points := make([]*DataPoint, n)
	values := []float64{}
	for i := 0; i < n; i++ {
		v := mean + stddev*math.Sin(float64(i)*0.1)
		values = append(values, v)
	}
	actualMean := 0.0
	for _, v := range values {
		actualMean += v
	}
	actualMean /= float64(len(values))
	variance := 0.0
	for _, v := range values {
		variance += (v - actualMean) * (v - actualMean)
	}
	variance /= float64(len(values))
	actualStd := math.Sqrt(variance)

	scale := stddev / actualStd
	for i := 0; i < n; i++ {
		points[i] = &DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Value:     mean + (values[i]-actualMean)*scale,
		}
	}
	return points
}

func TestDetector_Add_UpAnomaly(t *testing.T) {
	cfg := Config{
		WindowSize:        50,
		StdDevFactor:      2.0,
		MinSamples:        20,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	points := generateNormalData(100.0, 5.0, 40, baseTime)

	for i, p := range points {
		event, err := d.Add(p)
		if err != nil {
			t.Fatalf("Add() error at i=%d: %v", i, err)
		}
		if i < 19 && event != nil {
			t.Errorf("i=%d: should not detect anomaly during warmup", i)
		}
	}

	anomalyPoint := &DataPoint{
		Timestamp: baseTime.Add(41 * time.Second),
		Value:     150.0,
	}
	event, err := d.Add(anomalyPoint)
	if err != nil {
		t.Fatalf("Add() anomaly error: %v", err)
	}
	if event == nil {
		t.Fatal("Expected up anomaly event, got nil")
	}
	if event.Direction != DirectionUp {
		t.Errorf("Expected DirectionUp, got %v", event.Direction)
	}
	if event.ActualValue != 150.0 {
		t.Errorf("Expected ActualValue=150, got %v", event.ActualValue)
	}
	if event.Deviation <= 0 {
		t.Errorf("Expected positive Deviation for up anomaly, got %v", event.Deviation)
	}
}

func TestDetector_Add_DownAnomaly(t *testing.T) {
	cfg := Config{
		WindowSize:        50,
		StdDevFactor:      2.0,
		MinSamples:        20,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	points := generateNormalData(100.0, 5.0, 40, baseTime)

	for _, p := range points {
		_, err := d.Add(p)
		if err != nil {
			t.Fatalf("Add() error: %v", err)
		}
	}

	anomalyPoint := &DataPoint{
		Timestamp: baseTime.Add(41 * time.Second),
		Value:     50.0,
	}
	event, err := d.Add(anomalyPoint)
	if err != nil {
		t.Fatalf("Add() anomaly error: %v", err)
	}
	if event == nil {
		t.Fatal("Expected down anomaly event, got nil")
	}
	if event.Direction != DirectionDown {
		t.Errorf("Expected DirectionDown, got %v", event.Direction)
	}
	if event.Deviation >= 0 {
		t.Errorf("Expected negative Deviation for down anomaly, got %v", event.Deviation)
	}
}

func TestDetector_DirectionUpOnly(t *testing.T) {
	cfg := Config{
		WindowSize:        50,
		StdDevFactor:      2.0,
		MinSamples:        20,
		Direction:         DirectionUp,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	points := generateNormalData(100.0, 5.0, 40, baseTime)
	for _, p := range points {
		_, _ = d.Add(p)
	}

	downPoint := &DataPoint{
		Timestamp: baseTime.Add(41 * time.Second),
		Value:     50.0,
	}
	event, _ := d.Add(downPoint)
	if event != nil {
		t.Error("Down anomaly should not be detected in DirectionUp mode")
	}

	upPoint := &DataPoint{
		Timestamp: baseTime.Add(42 * time.Second),
		Value:     150.0,
	}
	event, _ = d.Add(upPoint)
	if event == nil {
		t.Error("Up anomaly should be detected in DirectionUp mode")
	}
}

func TestDetector_DirectionDownOnly(t *testing.T) {
	cfg := Config{
		WindowSize:        50,
		StdDevFactor:      2.0,
		MinSamples:        20,
		Direction:         DirectionDown,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	points := generateNormalData(100.0, 5.0, 40, baseTime)
	for _, p := range points {
		_, _ = d.Add(p)
	}

	upPoint := &DataPoint{
		Timestamp: baseTime.Add(41 * time.Second),
		Value:     150.0,
	}
	event, _ := d.Add(upPoint)
	if event != nil {
		t.Error("Up anomaly should not be detected in DirectionDown mode")
	}

	downPoint := &DataPoint{
		Timestamp: baseTime.Add(42 * time.Second),
		Value:     50.0,
	}
	event, _ = d.Add(downPoint)
	if event == nil {
		t.Error("Down anomaly should be detected in DirectionDown mode")
	}
}

func TestDetector_BaselineIncrementalUpdate(t *testing.T) {
	cfg := Config{
		WindowSize:        5,
		StdDevFactor:      3.0,
		MinSamples:        3,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	values := []float64{10, 20, 30, 40, 50, 60}
	for i, v := range values {
		p := &DataPoint{Timestamp: baseTime.Add(time.Duration(i) * time.Second), Value: v}
		_, _ = d.Add(p)
	}

	mean, stdDev, count := d.GetBaseline()
	if count != 5 {
		t.Errorf("Expected count=5, got %d", count)
	}

	expectedMean := (20.0 + 30 + 40 + 50 + 60) / 5.0
	if !floatApproxEqual(mean, expectedMean) {
		t.Errorf("Expected mean=%v, got %v", expectedMean, mean)
	}

	expectedVariance := 0.0
	windowVals := []float64{20, 30, 40, 50, 60}
	for _, v := range windowVals {
		expectedVariance += (v - expectedMean) * (v - expectedMean)
	}
	expectedVariance /= 4.0
	expectedStd := math.Sqrt(expectedVariance)
	if math.Abs(stdDev-expectedStd) > 1e-6 {
		t.Errorf("Expected stdDev=%v, got %v", expectedStd, stdDev)
	}
}

func TestDetector_WindowEviction(t *testing.T) {
	cfg := Config{
		WindowSize:        3,
		StdDevFactor:      3.0,
		MinSamples:        2,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	values := []float64{1, 2, 3, 4, 5, 6}
	for i, v := range values {
		p := &DataPoint{Timestamp: baseTime.Add(time.Duration(i) * time.Second), Value: v}
		_, _ = d.Add(p)
	}

	_, _, count := d.GetBaseline()
	if count != 3 {
		t.Errorf("Expected count=3 after eviction, got %d", count)
	}

	mean, _, _ := d.GetBaseline()
	expectedMean := (4.0 + 5 + 6) / 3.0
	if !floatApproxEqual(mean, expectedMean) {
		t.Errorf("Expected mean=%v after eviction, got %v", expectedMean, mean)
	}
}

func TestDetector_SeasonalMode(t *testing.T) {
	epoch := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := Config{
		WindowSize:        100,
		StdDevFactor:      2.0,
		MinSamples:        3,
		EnableSeasonal:    true,
		PeriodLength:      4,
		PeriodSlot:        time.Second,
		SeasonalEpoch:     epoch,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	pattern := []float64{10, 20, 30, 40}
	for cycle := 0; cycle < 3; cycle++ {
		for i, v := range pattern {
			offsetSec := cycle*4 + i
			p := &DataPoint{
				Timestamp: epoch.Add(time.Duration(offsetSec) * time.Second),
				Value:     v,
			}
			_, err := d.Add(p)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	mean0, _, count0, err := d.GetSeasonalBaseline(0)
	if err != nil {
		t.Fatal(err)
	}
	if count0 != 3 {
		t.Errorf("Seasonal index 0: expected count=3, got %d", count0)
	}
	if !floatApproxEqual(mean0, 10.0) {
		t.Errorf("Seasonal index 0: expected mean=10, got %v", mean0)
	}

	mean1, _, count1, _ := d.GetSeasonalBaseline(1)
	if count1 != 3 {
		t.Errorf("Seasonal index 1: expected count=3, got %d", count1)
	}
	if !floatApproxEqual(mean1, 20.0) {
		t.Errorf("Seasonal index 1: expected mean=20, got %v", mean1)
	}

	anomalyPoint := &DataPoint{
		Timestamp: epoch.Add(12 * time.Second),
		Value:     100.0,
	}
	event, err := d.Add(anomalyPoint)
	if err != nil {
		t.Fatal(err)
	}
	if event == nil {
		t.Fatal("Expected seasonal anomaly at index 0")
	}
	if event.SeasonalIndex != 0 {
		t.Errorf("Expected SeasonalIndex=0, got %d", event.SeasonalIndex)
	}
	if event.BaselineValue != 10.0 {
		t.Errorf("Expected BaselineValue=10 (seasonal), got %v", event.BaselineValue)
	}
}

func TestDetector_Seasonal_UnevenTimestamps(t *testing.T) {
	epoch := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := Config{
		WindowSize:        100,
		StdDevFactor:      2.0,
		MinSamples:        3,
		EnableSeasonal:    true,
		PeriodLength:      4,
		PeriodSlot:        time.Second,
		SeasonalEpoch:     epoch,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	pattern := []float64{10, 20, 30, 40}
	for cycle := 0; cycle < 3; cycle++ {
		for i := range pattern {
			baseOffset := cycle * 4
			var irregularOffsets = []int{0, 3, 7, 12, 15, 22, 28, 31, 33, 37, 42, 48}
			idx := baseOffset + i
			offsetSec := irregularOffsets[idx]
			expectedSeasonalIdx := offsetSec % 4

			p := &DataPoint{
				Timestamp: epoch.Add(time.Duration(offsetSec) * time.Second),
				Value:     pattern[expectedSeasonalIdx],
			}
			_, err := d.Add(p)
			if err != nil {
				t.Fatalf("cycle=%d i=%d offset=%d err=%v", cycle, i, offsetSec, err)
			}
		}
	}

	mean0, _, count0, err := d.GetSeasonalBaseline(0)
	if err != nil {
		t.Fatal(err)
	}
	if count0 < 1 {
		t.Errorf("Seasonal index 0: expected at least 1 sample, got %d", count0)
	}
	if count0 >= 1 && !floatApproxEqual(mean0, 10.0) {
		t.Errorf("Seasonal index 0: expected mean=10, got %v (count=%d)", mean0, count0)
	}

	mean1, _, count1, _ := d.GetSeasonalBaseline(1)
	if count1 >= 1 && !floatApproxEqual(mean1, 20.0) {
		t.Errorf("Seasonal index 1: expected mean=20, got %v (count=%d)", mean1, count1)
	}

	anomalyTs := epoch.Add(100 * time.Second)
	expectedIdx := 100 % 4
	anomalyVal := pattern[expectedIdx] * 10
	anomalyPoint := &DataPoint{
		Timestamp: anomalyTs,
		Value:     anomalyVal,
	}
	event, err := d.Add(anomalyPoint)
	if err != nil {
		t.Fatal(err)
	}
	if event == nil {
		t.Fatalf("Expected seasonal anomaly at index %d, got nil (expectedIdx baseline may not have enough samples)", expectedIdx)
	}
	if event != nil && event.SeasonalIndex != expectedIdx {
		t.Errorf("Expected SeasonalIndex=%d (from timestamp 100%%4), got %d", expectedIdx, event.SeasonalIndex)
	}
}

func TestDetector_Seasonal_DailyHourCycle(t *testing.T) {
	epoch := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := Config{
		WindowSize:        50,
		StdDevFactor:      2.0,
		MinSamples:        3,
		EnableSeasonal:    true,
		PeriodLength:      24,
		PeriodSlot:        time.Hour,
		SeasonalEpoch:     epoch,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	for day := 0; day < 4; day++ {
		for hour := 0; hour < 24; hour++ {
			var val float64
			switch {
			case hour >= 8 && hour <= 10:
				val = 5000
			case hour >= 18 && hour <= 20:
				val = 6000
			case hour >= 0 && hour <= 5:
				val = 500
			default:
				val = 2000
			}
			p := &DataPoint{
				Timestamp: epoch.Add(time.Duration(day*24+hour) * time.Hour),
				Value:     val,
			}
			_, err := d.Add(p)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	mean8, _, count8, _ := d.GetSeasonalBaseline(8)
	if count8 != 4 {
		t.Errorf("8 hour slot: expected count=4, got %d", count8)
	}
	if !floatApproxEqual(mean8, 5000) {
		t.Errorf("8 hour slot: expected mean=5000, got %v", mean8)
	}

	mean3, _, count3, _ := d.GetSeasonalBaseline(3)
	if count3 != 4 {
		t.Errorf("3 hour slot: expected count=4, got %d", count3)
	}
	if !floatApproxEqual(mean3, 500) {
		t.Errorf("3 hour slot: expected mean=500, got %v", mean3)
	}

	crashPoint := &DataPoint{
		Timestamp: epoch.Add(4*24*time.Hour + 9*time.Hour),
		Value:     100.0,
	}
	event, _ := d.Add(crashPoint)
	if event == nil {
		t.Fatal("Expected anomaly during morning peak crash")
	}
	if event.SeasonalIndex != 9 {
		t.Errorf("Expected SeasonalIndex=9 (9th hour), got %d", event.SeasonalIndex)
	}
	if event.BaselineValue != 5000 {
		t.Errorf("Expected BaselineValue=5000 (peak hour), got %v", event.BaselineValue)
	}
}

func TestDetector_SeasonalBaselineErrors(t *testing.T) {
	cfg := Config{
		WindowSize:        10,
		StdDevFactor:      2.0,
		MinSamples:        2,
		EnableSeasonal:    false,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = d.GetSeasonalBaseline(0)
	if err == nil {
		t.Error("Expected error when seasonal mode not enabled")
	}

	cfg2 := Config{
		WindowSize:        10,
		StdDevFactor:      2.0,
		MinSamples:        2,
		EnableSeasonal:    true,
		PeriodLength:      4,
		PeriodSlot:        time.Second,
		SeasonalEpoch:     time.Unix(0, 0).UTC(),
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d2, _ := NewDetector(cfg2)
	_, _, _, err = d2.GetSeasonalBaseline(-1)
	if err == nil {
		t.Error("Expected error for negative seasonal index")
	}
	_, _, _, err = d2.GetSeasonalBaseline(10)
	if err == nil {
		t.Error("Expected error for out-of-range seasonal index")
	}
}

func TestDetector_AnomalyQuery(t *testing.T) {
	cfg := Config{
		WindowSize:        50,
		StdDevFactor:      1.0,
		MinSamples:        10,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	normalVals := generateNormalData(100.0, 2.0, 15, baseTime)
	for _, p := range normalVals {
		_, _ = d.Add(p)
	}

	anomalyTimes := []time.Time{
		baseTime.Add(20 * time.Second),
		baseTime.Add(30 * time.Second),
		baseTime.Add(40 * time.Second),
		baseTime.Add(50 * time.Second),
	}
	anomalyVals := []float64{200.0, 150.0, 50.0, 180.0}
	for i := range anomalyTimes {
		p := &DataPoint{Timestamp: anomalyTimes[i], Value: anomalyVals[i]}
		_, _ = d.Add(p)
	}

	all := d.GetAnomalies(nil)
	if len(all) < 2 {
		t.Errorf("Expected at least 2 anomalies, got %d", len(all))
	}

	queryStartTime := anomalyTimes[1]
	q1 := &AnomalyQuery{StartTime: &queryStartTime}
	r1 := d.GetAnomalies(q1)
	for _, a := range r1 {
		if a.Timestamp.Before(queryStartTime) {
			t.Error("Filtered anomaly before StartTime")
		}
	}

	queryEndTime := anomalyTimes[2]
	q2 := &AnomalyQuery{EndTime: &queryEndTime}
	r2 := d.GetAnomalies(q2)
	for _, a := range r2 {
		if a.Timestamp.After(queryEndTime) {
			t.Error("Filtered anomaly after EndTime")
		}
	}

	dirUp := DirectionUp
	q3 := &AnomalyQuery{Direction: &dirUp}
	r3 := d.GetAnomalies(q3)
	for _, a := range r3 {
		if a.Direction != DirectionUp {
			t.Errorf("Expected DirectionUp, got %v", a.Direction)
		}
	}

	sevWarn := SeverityWarning
	q4 := &AnomalyQuery{Severity: &sevWarn}
	r4 := d.GetAnomalies(q4)
	for _, a := range r4 {
		if a.Severity != SeverityWarning {
			t.Errorf("Expected SeverityWarning, got %v", a.Severity)
		}
	}

	q5 := &AnomalyQuery{Limit: 2}
	r5 := d.GetAnomalies(q5)
	if len(r5) > 2 {
		t.Errorf("Expected at most 2 anomalies with limit, got %d", len(r5))
	}
}

func TestDetector_AnomalySortedOrder(t *testing.T) {
	cfg := Config{
		WindowSize:        50,
		StdDevFactor:      1.0,
		MinSamples:        10,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	normalVals := generateNormalData(100.0, 2.0, 15, baseTime)
	for _, p := range normalVals {
		_, _ = d.Add(p)
	}

	anomalies := []struct {
		ts    time.Time
		value float64
	}{
		{baseTime.Add(25 * time.Second), 200},
		{baseTime.Add(15 * time.Second), 180},
		{baseTime.Add(35 * time.Second), 220},
	}
	for _, a := range anomalies {
		p := &DataPoint{Timestamp: a.ts, Value: a.value}
		_, _ = d.Add(p)
	}

	results := d.GetAnomalies(nil)
	for i := 1; i < len(results); i++ {
		if results[i].Timestamp.Before(results[i-1].Timestamp) {
			t.Error("Anomalies not sorted by time")
		}
	}
}

func TestDetector_AnomalyHistoryLimit(t *testing.T) {
	cfg := Config{
		WindowSize:        50,
		StdDevFactor:      1.0,
		MinSamples:        5,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 3,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	normalVals := generateNormalData(100.0, 1.0, 10, baseTime)
	for _, p := range normalVals {
		_, _ = d.Add(p)
	}

	for i := 0; i < 10; i++ {
		p := &DataPoint{
			Timestamp: baseTime.Add(time.Duration(100+i) * time.Second),
			Value:     1000.0,
		}
		_, _ = d.Add(p)
	}

	count := d.AnomalyCount()
	if count > 3 {
		t.Errorf("Expected at most 3 anomalies (history limit), got %d", count)
	}
}

func TestDetector_BatchAdd(t *testing.T) {
	cfg := Config{
		WindowSize:        50,
		StdDevFactor:      2.0,
		MinSamples:        20,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	points := generateNormalData(100.0, 5.0, 40, baseTime)
	points = append(points, &DataPoint{Timestamp: baseTime.Add(50 * time.Second), Value: 200.0})
	points = append(points, &DataPoint{Timestamp: baseTime.Add(51 * time.Second), Value: 250.0})

	events, err := d.BatchAdd(points)
	if err != nil {
		t.Fatalf("BatchAdd() error: %v", err)
	}
	if len(events) < 2 {
		t.Errorf("Expected at least 2 anomaly events, got %d", len(events))
	}

	empty, err := d.BatchAdd([]*DataPoint{})
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Error("Expected 0 events for empty batch")
	}

	_, err = d.BatchAdd([]*DataPoint{nil})
	if !errors.Is(err, ErrNilDataPoint) {
		t.Errorf("Expected ErrNilDataPoint, got %v", err)
	}

	d.Close()
	_, err = d.BatchAdd(points)
	if !errors.Is(err, ErrDetectorClosed) {
		t.Errorf("Expected ErrDetectorClosed, got %v", err)
	}
}

func TestDetector_BatchAdd_NoRaceWithClose(t *testing.T) {
	cfg := Config{
		WindowSize:        50,
		StdDevFactor:      2.0,
		MinSamples:        5,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	batch := make([]*DataPoint, 100)
	for i := 0; i < 100; i++ {
		batch[i] = &DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Millisecond),
			Value:     float64(i%10) + 1.0,
		}
	}

	var wg sync.WaitGroup
	var batchErr error
	var closeErr error
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, batchErr = d.BatchAdd(batch)
	}()

	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Microsecond)
		d.Close()
	}()

	wg.Wait()

	if batchErr != nil && !errors.Is(batchErr, ErrDetectorClosed) {
		t.Errorf("BatchAdd expected success or ErrDetectorClosed, got %v", batchErr)
	}
	_ = closeErr
}

func TestDetector_Reset(t *testing.T) {
	cfg := Config{
		WindowSize:        50,
		StdDevFactor:      2.0,
		MinSamples:        10,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	points := generateNormalData(100.0, 5.0, 30, baseTime)
	for _, p := range points {
		_, _ = d.Add(p)
	}

	if d.PointCount() != 30 {
		t.Errorf("Expected PointCount=30 before reset, got %d", d.PointCount())
	}
	_, _, countBefore := d.GetBaseline()
	if countBefore == 0 {
		t.Error("Expected non-zero baseline count before reset")
	}

	d.Reset()

	if d.PointCount() != 0 {
		t.Errorf("Expected PointCount=0 after reset, got %d", d.PointCount())
	}
	_, _, countAfter := d.GetBaseline()
	if countAfter != 0 {
		t.Errorf("Expected baseline count=0 after reset, got %d", countAfter)
	}
	if d.AnomalyCount() != 0 {
		t.Errorf("Expected AnomalyCount=0 after reset, got %d", d.AnomalyCount())
	}
}

func TestDetector_UpdateConfig(t *testing.T) {
	d := NewDetectorWithDefault()

	newCfg := Config{
		WindowSize:        200,
		StdDevFactor:      4.0,
		MinSamples:        50,
		EnableSeasonal:    true,
		PeriodLength:      7,
		PeriodSlot:        time.Hour * 24,
		SeasonalEpoch:     time.Unix(0, 0).UTC(),
		Direction:         DirectionUp,
		MaxAnomalyHistory: 500,
	}
	err := d.UpdateConfig(newCfg)
	if err != nil {
		t.Fatalf("UpdateConfig() error: %v", err)
	}

	gotCfg := d.Config()
	if gotCfg.WindowSize != 200 {
		t.Errorf("WindowSize not updated: got %d", gotCfg.WindowSize)
	}
	if gotCfg.Direction != DirectionUp {
		t.Errorf("Direction not updated: got %v", gotCfg.Direction)
	}

	invalidCfg := Config{WindowSize: -1}
	err = d.UpdateConfig(invalidCfg)
	if err == nil {
		t.Error("UpdateConfig() expected error for invalid config")
	}
}

func TestDetector_UpdateConfig_PreservesGlobalBaseline(t *testing.T) {
	cfg := Config{
		WindowSize:        20,
		StdDevFactor:      3.0,
		MinSamples:        5,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	for i := 0; i < 15; i++ {
		p := &DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Value:     100.0,
		}
		_, _ = d.Add(p)
	}

	meanBefore, stdBefore, countBefore := d.GetBaseline()
	if countBefore != 15 {
		t.Fatalf("Setup failed: baseline count=%d", countBefore)
	}

	newCfg := Config{
		WindowSize:        30,
		StdDevFactor:      2.5,
		MinSamples:        8,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 200,
	}
	err = d.UpdateConfig(newCfg)
	if err != nil {
		t.Fatal(err)
	}

	meanAfter, stdAfter, countAfter := d.GetBaseline()
	if countAfter != countBefore {
		t.Errorf("Baseline count changed after non-seasonal UpdateConfig: got %d, want %d", countAfter, countBefore)
	}
	if !floatApproxEqual(meanAfter, meanBefore) {
		t.Errorf("Baseline mean changed after non-seasonal UpdateConfig: got %v, want %v", meanAfter, meanBefore)
	}
	if !floatApproxEqual(stdAfter, stdBefore) {
		t.Errorf("Baseline std changed after non-seasonal UpdateConfig: got %v, want %v", stdAfter, stdBefore)
	}
}

func TestDetector_UpdateConfig_NonSeasonalToSeasonal(t *testing.T) {
	cfg := Config{
		WindowSize:        20,
		StdDevFactor:      3.0,
		MinSamples:        2,
		EnableSeasonal:    false,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	for i := 0; i < 12; i++ {
		p := &DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Value:     float64(10 + i),
		}
		_, _ = d.Add(p)
	}

	_, _, globalCountBefore := d.GetBaseline()
	if globalCountBefore != 12 {
		t.Fatalf("Setup failed: global count=%d", globalCountBefore)
	}

	epoch := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	seasonalCfg := Config{
		WindowSize:        20,
		StdDevFactor:      3.0,
		MinSamples:        2,
		EnableSeasonal:    true,
		PeriodLength:      4,
		PeriodSlot:        time.Second,
		SeasonalEpoch:     epoch,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	err = d.UpdateConfig(seasonalCfg)
	if err != nil {
		t.Fatal(err)
	}

	totalSeasonalCount := 0
	for i := 0; i < 4; i++ {
		_, _, cnt, _ := d.GetSeasonalBaseline(i)
		totalSeasonalCount += cnt
	}
	if totalSeasonalCount != globalCountBefore {
		t.Errorf("Total seasonal samples=%d, want %d (migrated from global)", totalSeasonalCount, globalCountBefore)
	}

	meanAfter, _, globalCountAfter := d.GetBaseline()
	if globalCountAfter != globalCountBefore {
		t.Errorf("Global count changed: got %d, want %d", globalCountAfter, globalCountBefore)
	}
	_ = meanAfter
}

func TestDetector_UpdateConfig_SeasonalToNonSeasonal(t *testing.T) {
	epoch := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := Config{
		WindowSize:        50,
		StdDevFactor:      3.0,
		MinSamples:        2,
		EnableSeasonal:    true,
		PeriodLength:      4,
		PeriodSlot:        time.Second,
		SeasonalEpoch:     epoch,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	pattern := []float64{10, 20, 30, 40}
	totalSamplesSeasonal := 0
	for cycle := 0; cycle < 3; cycle++ {
		for i, v := range pattern {
			offsetSec := cycle*4 + i
			p := &DataPoint{
				Timestamp: epoch.Add(time.Duration(offsetSec) * time.Second),
				Value:     v,
			}
			_, _ = d.Add(p)
			totalSamplesSeasonal++
		}
	}

	for i := 0; i < 4; i++ {
		_, _, cnt, _ := d.GetSeasonalBaseline(i)
		if cnt != 3 {
			t.Fatalf("Setup failed: seasonal[%d] count=%d, want 3", i, cnt)
		}
	}

	nonSeasonalCfg := Config{
		WindowSize:        50,
		StdDevFactor:      3.0,
		MinSamples:        5,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	err = d.UpdateConfig(nonSeasonalCfg)
	if err != nil {
		t.Fatal(err)
	}

	_, _, globalCount := d.GetBaseline()
	if globalCount != totalSamplesSeasonal {
		t.Errorf("Global count=%d after migration, want exactly %d (no duplicate data)", globalCount, totalSamplesSeasonal)
	}

	gotCfg := d.Config()
	if gotCfg.EnableSeasonal {
		t.Error("Seasonal mode should be disabled after UpdateConfig")
	}
}

func TestDetector_UpdateConfig_SamePeriodLength_PreservesData(t *testing.T) {
	epoch := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := Config{
		WindowSize:        50,
		StdDevFactor:      2.0,
		MinSamples:        2,
		EnableSeasonal:    true,
		PeriodLength:      4,
		PeriodSlot:        time.Second,
		SeasonalEpoch:     epoch,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	pattern := []float64{10, 20, 30, 40}
	for cycle := 0; cycle < 3; cycle++ {
		for i, v := range pattern {
			offsetSec := cycle*4 + i
			p := &DataPoint{
				Timestamp: epoch.Add(time.Duration(offsetSec) * time.Second),
				Value:     v,
			}
			_, _ = d.Add(p)
		}
	}

	beforeMeans := make([]float64, 4)
	beforeCounts := make([]int, 4)
	for i := 0; i < 4; i++ {
		m, _, c, _ := d.GetSeasonalBaseline(i)
		beforeMeans[i] = m
		beforeCounts[i] = c
	}

	newCfg := Config{
		WindowSize:        60,
		StdDevFactor:      3.0,
		MinSamples:        3,
		EnableSeasonal:    true,
		PeriodLength:      4,
		PeriodSlot:        time.Second,
		SeasonalEpoch:     epoch,
		Direction:         DirectionUp,
		MaxAnomalyHistory: 200,
	}
	err = d.UpdateConfig(newCfg)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 4; i++ {
		afterMean, _, afterCount, _ := d.GetSeasonalBaseline(i)
		if afterCount != beforeCounts[i] {
			t.Errorf("Seasonal[%d] count changed: got %d, want %d", i, afterCount, beforeCounts[i])
		}
		if !floatApproxEqual(afterMean, beforeMeans[i]) {
			t.Errorf("Seasonal[%d] mean changed: got %v, want %v", i, afterMean, beforeMeans[i])
		}
	}
}

func TestDetector_UpdateConfig_ChangePeriodLength(t *testing.T) {
	epoch := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := Config{
		WindowSize:        100,
		StdDevFactor:      2.0,
		MinSamples:        1,
		EnableSeasonal:    true,
		PeriodLength:      4,
		PeriodSlot:        time.Second,
		SeasonalEpoch:     epoch,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	for cycle := 0; cycle < 3; cycle++ {
		for i := 0; i < 4; i++ {
			offsetSec := cycle*4 + i
			p := &DataPoint{
				Timestamp: epoch.Add(time.Duration(offsetSec) * time.Second),
				Value:     float64(10 + i*10),
			}
			_, _ = d.Add(p)
		}
	}

	totalBefore := 0
	for i := 0; i < 4; i++ {
		_, _, cnt, _ := d.GetSeasonalBaseline(i)
		totalBefore += cnt
	}
	if totalBefore != 12 {
		t.Fatalf("Setup: total seasonal samples=%d, want 12", totalBefore)
	}

	newCfg := Config{
		WindowSize:        100,
		StdDevFactor:      2.0,
		MinSamples:        1,
		EnableSeasonal:    true,
		PeriodLength:      6,
		PeriodSlot:        time.Second,
		SeasonalEpoch:     epoch,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	err = d.UpdateConfig(newCfg)
	if err != nil {
		t.Fatal(err)
	}

	totalAfter := 0
	slotCounts := make([]int, 6)
	for i := 0; i < 6; i++ {
		_, _, cnt, _ := d.GetSeasonalBaseline(i)
		slotCounts[i] = cnt
		totalAfter += cnt
	}

	if totalAfter != totalBefore {
		t.Errorf("Total seasonal samples after migration=%d, want exactly %d (no data duplication across slots)", totalAfter, totalBefore)
	}

	if slotCounts[0] != 3 || slotCounts[1] != 3 || slotCounts[2] != 3 || slotCounts[3] != 3 {
		t.Errorf("First 4 slots should have 3 samples each (one-to-one mapping), got counts=%v", slotCounts)
	}
	if slotCounts[4] != 0 || slotCounts[5] != 0 {
		t.Errorf("Slots 4 and 5 should be empty (no data sharing across different phases), got counts=%v", slotCounts)
	}

	mean0, _, _, _ := d.GetSeasonalBaseline(0)
	if !floatApproxEqual(mean0, 10.0) {
		t.Errorf("Slot 0 mean should be 10.0 (mapped from old slot 0), got %v", mean0)
	}
	mean1, _, _, _ := d.GetSeasonalBaseline(1)
	if !floatApproxEqual(mean1, 20.0) {
		t.Errorf("Slot 1 mean should be 20.0 (mapped from old slot 1), got %v", mean1)
	}
	mean2, _, _, _ := d.GetSeasonalBaseline(2)
	if !floatApproxEqual(mean2, 30.0) {
		t.Errorf("Slot 2 mean should be 30.0 (mapped from old slot 2), got %v", mean2)
	}
	mean3, _, _, _ := d.GetSeasonalBaseline(3)
	if !floatApproxEqual(mean3, 40.0) {
		t.Errorf("Slot 3 mean should be 40.0 (mapped from old slot 3), got %v", mean3)
	}
}

func TestDetector_Close(t *testing.T) {
	d := NewDetectorWithDefault()
	if d.IsClosed() {
		t.Error("Should not be closed initially")
	}
	d.Close()
	if !d.IsClosed() {
		t.Error("Should be closed after Close()")
	}
}

func TestDetector_ConcurrentAccess(t *testing.T) {
	cfg := Config{
		WindowSize:        100,
		StdDevFactor:      2.0,
		MinSamples:        10,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 1000,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	numGoroutines := 10
	pointsPerGoroutine := 100

	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			baseTime := time.Now()
			for i := 0; i < pointsPerGoroutine; i++ {
				ts := baseTime.Add(time.Duration(goroutineID*1000+i) * time.Millisecond)
				val := 50.0 + 10.0*math.Sin(float64(goroutineID*pointsPerGoroutine+i)*0.05)
				if i%23 == 0 {
					val += 500
				}
				p := &DataPoint{Timestamp: ts, Value: val}
				_, _ = d.Add(p)

				if i%10 == 0 {
					_, _, _ = d.GetBaseline()
					_ = d.AnomalyCount()
					_ = d.GetAnomalies(nil)
				}
			}
		}(g)
	}
	wg.Wait()

	pc := d.PointCount()
	expectedPc := int64(numGoroutines * pointsPerGoroutine)
	if pc != expectedPc {
		t.Errorf("PointCount mismatch: got %d, want %d", pc, expectedPc)
	}
}

func TestDetector_SeverityLevels(t *testing.T) {
	cfg := Config{
		WindowSize:        50,
		StdDevFactor:      2.0,
		MinSamples:        20,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	points := generateNormalData(100.0, 5.0, 40, baseTime)
	for _, p := range points {
		_, _ = d.Add(p)
	}

	warningPoint := &DataPoint{
		Timestamp: baseTime.Add(41 * time.Second),
		Value:     115.0,
	}
	evt1, _ := d.Add(warningPoint)
	if evt1 != nil && evt1.Severity != SeverityWarning {
		t.Errorf("Expected warning severity, got %v, ratio=%v", evt1.Severity, evt1.DeviationRatio)
	}

	criticalPoint := &DataPoint{
		Timestamp: baseTime.Add(42 * time.Second),
		Value:     300.0,
	}
	evt2, _ := d.Add(criticalPoint)
	if evt2 != nil && evt2.Severity != SeverityCritical {
		t.Errorf("Expected critical severity, got %v, ratio=%v", evt2.Severity, evt2.DeviationRatio)
	}
}

func TestDetector_ZeroStdDev(t *testing.T) {
	cfg := Config{
		WindowSize:        10,
		StdDevFactor:      2.0,
		MinSamples:        5,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	for i := 0; i < 10; i++ {
		p := &DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Value:     100.0,
		}
		_, _ = d.Add(p)
	}

	_, stdDev, _ := d.GetBaseline()
	if stdDev != 0 {
		t.Errorf("Expected zero stddev for constant values, got %v", stdDev)
	}

	differentPoint := &DataPoint{
		Timestamp: baseTime.Add(11 * time.Second),
		Value:     200.0,
	}
	event, err := d.Add(differentPoint)
	if err != nil {
		t.Fatal(err)
	}
	if event == nil {
		t.Fatal("Expected anomaly with zero stddev and different value")
	}
}

func TestDeviationDirection_String(t *testing.T) {
	tests := []struct {
		dir  DeviationDirection
		want string
	}{
		{DirectionBoth, "both"},
		{DirectionUp, "up"},
		{DirectionDown, "down"},
		{DeviationDirection(999), "unknown"},
	}
	for _, tt := range tests {
		got := tt.dir.String()
		if got != tt.want {
			t.Errorf("DeviationDirection(%d).String() = %q, want %q", tt.dir, got, tt.want)
		}
	}
}

func TestDetector_AnomalyEventFields(t *testing.T) {
	cfg := Config{
		WindowSize:        50,
		StdDevFactor:      2.0,
		MinSamples:        20,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	points := generateNormalData(100.0, 5.0, 40, baseTime)
	for _, p := range points {
		_, _ = d.Add(p)
	}

	anomalyTS := baseTime.Add(41 * time.Second)
	anomalyValue := 150.0
	event, err := d.Add(&DataPoint{Timestamp: anomalyTS, Value: anomalyValue})
	if err != nil {
		t.Fatal(err)
	}
	if event == nil {
		t.Fatal("Expected event")
	}

	if !event.Timestamp.Equal(anomalyTS) {
		t.Errorf("Timestamp mismatch: got %v, want %v", event.Timestamp, anomalyTS)
	}
	if event.ActualValue != anomalyValue {
		t.Errorf("ActualValue mismatch: got %v, want %v", event.ActualValue, anomalyValue)
	}
	if !floatApproxEqual(event.Deviation, event.ActualValue-event.BaselineValue) {
		t.Errorf("Deviation calculation incorrect: %v != %v - %v",
			event.Deviation, event.ActualValue, event.BaselineValue)
	}
	if !floatApproxEqual(event.Threshold, cfg.StdDevFactor*event.StdDev) {
		t.Errorf("Threshold calculation incorrect: %v != %v * %v",
			event.Threshold, cfg.StdDevFactor, event.StdDev)
	}
}

func TestDetector_PointCount(t *testing.T) {
	d := NewDetectorWithDefault()
	if d.PointCount() != 0 {
		t.Errorf("Initial PointCount should be 0, got %d", d.PointCount())
	}

	baseTime := time.Now()
	for i := 0; i < 42; i++ {
		p := &DataPoint{Timestamp: baseTime.Add(time.Duration(i) * time.Second), Value: float64(i)}
		_, _ = d.Add(p)
	}

	if d.PointCount() != 42 {
		t.Errorf("PointCount should be 42, got %d", d.PointCount())
	}
}

func TestDetector_SeasonalReset(t *testing.T) {
	epoch := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := Config{
		WindowSize:        10,
		StdDevFactor:      2.0,
		MinSamples:        2,
		EnableSeasonal:    true,
		PeriodLength:      3,
		PeriodSlot:        time.Second,
		SeasonalEpoch:     epoch,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	for cycle := 0; cycle < 2; cycle++ {
		for i := 0; i < 3; i++ {
			offsetSec := cycle*3 + i
			p := &DataPoint{
				Timestamp: epoch.Add(time.Duration(offsetSec) * time.Second),
				Value:     float64(i*10 + 10),
			}
			_, _ = d.Add(p)
		}
	}

	_, _, count, _ := d.GetSeasonalBaseline(0)
	if count != 2 {
		t.Errorf("Seasonal baseline count before reset: got %d, want 2", count)
	}

	d.Reset()

	_, _, countAfter, _ := d.GetSeasonalBaseline(0)
	if countAfter != 0 {
		t.Errorf("Seasonal baseline count after reset: got %d, want 0", countAfter)
	}
}
