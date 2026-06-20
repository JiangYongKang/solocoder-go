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
			},
			wantErr: ErrInvalidPeriodLength,
		},
		{
			name: "valid seasonal config",
			cfg: Config{
				WindowSize:     100,
				StdDevFactor:   3.0,
				MinSamples:     10,
				EnableSeasonal: true,
				PeriodLength:   24,
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
	cfg := Config{
		WindowSize:        100,
		StdDevFactor:      2.0,
		MinSamples:        3,
		EnableSeasonal:    true,
		PeriodLength:      4,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	pattern := []float64{10, 20, 30, 40}
	for cycle := 0; cycle < 3; cycle++ {
		for i, v := range pattern {
			p := &DataPoint{
				Timestamp: baseTime.Add(time.Duration(cycle*4+i) * time.Second),
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
		Timestamp: baseTime.Add(12 * time.Second),
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
	cfg := Config{
		WindowSize:        10,
		StdDevFactor:      2.0,
		MinSamples:        2,
		EnableSeasonal:    true,
		PeriodLength:      3,
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 100,
	}
	d, err := NewDetector(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Now()
	for cycle := 0; cycle < 2; cycle++ {
		for i := 0; i < 3; i++ {
			p := &DataPoint{
				Timestamp: baseTime.Add(time.Duration(cycle*3+i) * time.Second),
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
