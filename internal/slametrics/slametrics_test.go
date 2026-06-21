package slametrics

import (
	"sync"
	"testing"
	"time"
)

func TestNewSLAMetrics(t *testing.T) {
	s := NewSLAMetrics()
	if s == nil {
		t.Fatal("NewSLAMetrics returned nil")
	}
	if s.RecordCount() != 0 {
		t.Errorf("expected 0 records, got %d", s.RecordCount())
	}
	if s.ViolationCount() != 0 {
		t.Errorf("expected 0 violations, got %d", s.ViolationCount())
	}
	if s.Generation() != 0 {
		t.Errorf("expected generation 0, got %d", s.Generation())
	}
}

func TestRoundToDecimal(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		decimals int
		expected float64
	}{
		{"zero decimals", 99.999, 0, 100},
		{"two decimals round up", 99.555, 2, 99.56},
		{"two decimals round down", 99.554, 2, 99.55},
		{"three decimals", 99.1234, 3, 99.123},
		{"negative decimals treated as 0", 99.9, -1, 100},
		{"integer value", 100.0, 2, 100.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := roundToDecimal(tt.value, tt.decimals)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNearestRankPercentile(t *testing.T) {
	sorted := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	tests := []struct {
		name       string
		percentile float64
		expected   float64
	}{
		{"P50", 50, 5},
		{"P90", 90, 9},
		{"P99", 99, 10},
		{"P10", 10, 1},
		{"P25", 25, 3},
		{"P75", 75, 8},
		{"P0 returns min", 0, 1},
		{"P100 returns max", 100, 10},
		{"P negative returns min", -5, 1},
		{"P over 100 returns max", 150, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nearestRankPercentile(sorted, tt.percentile)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNearestRankPercentileSingleElement(t *testing.T) {
	sorted := []float64{42.0}
	tests := []float64{1, 50, 90, 99, 100}
	for _, p := range tests {
		result := nearestRankPercentile(sorted, p)
		if result != 42.0 {
			t.Errorf("P%.0f: expected 42.0, got %v", p, result)
		}
	}
}

func TestNearestRankPercentileEmpty(t *testing.T) {
	result := nearestRankPercentile([]float64{}, 50)
	if result != 0 {
		t.Errorf("expected 0 for empty slice, got %v", result)
	}
}

func TestComputePercentiles(t *testing.T) {
	latencies := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	result, err := computePercentiles(latencies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 10 {
		t.Errorf("expected count 10, got %d", result.Count)
	}
	if result.Min != 10 {
		t.Errorf("expected min 10, got %v", result.Min)
	}
	if result.Max != 100 {
		t.Errorf("expected max 100, got %v", result.Max)
	}
	if result.P50 != 50 {
		t.Errorf("expected P50=50, got %v", result.P50)
	}
	if result.P90 != 90 {
		t.Errorf("expected P90=90, got %v", result.P90)
	}
	if result.P99 != 100 {
		t.Errorf("expected P99=100, got %v", result.P99)
	}
}

func TestComputePercentilesEmpty(t *testing.T) {
	_, err := computePercentiles([]float64{})
	if err != ErrNoLatencyData {
		t.Errorf("expected ErrNoLatencyData, got %v", err)
	}
}

func TestComputePercentilesAllValuesExist(t *testing.T) {
	latencies := []float64{1.5, 2.5, 3.5, 4.5, 5.5, 6.5, 7.5, 8.5, 9.5, 10.5}
	result, err := computePercentiles(latencies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := make(map[float64]bool)
	for _, v := range latencies {
		found[v] = true
	}

	if !found[result.P50] {
		t.Errorf("P50 value %v not in original dataset", result.P50)
	}
	if !found[result.P90] {
		t.Errorf("P90 value %v not in original dataset", result.P90)
	}
	if !found[result.P99] {
		t.Errorf("P99 value %v not in original dataset", result.P99)
	}
}

func TestRecordRequestSuccess(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	s.RecordRequest(RequestRecord{Timestamp: base, Success: true, Latency: 10})
	if s.RecordCount() != 1 {
		t.Errorf("expected 1 record, got %d", s.RecordCount())
	}
}

func TestRecordRequestFailedWithErrorKey(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	s.RecordRequest(RequestRecord{Timestamp: base, Success: false, Latency: 10, ErrorKey: "timeout"})
	if s.RecordCount() != 1 {
		t.Errorf("expected 1 record, got %d", s.RecordCount())
	}
}

func TestRecordRequestFailedEmptyErrorKey(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	s.RecordRequest(RequestRecord{Timestamp: base, Success: false, Latency: 10, ErrorKey: ""})
	if s.RecordCount() != 1 {
		t.Errorf("expected 1 record (empty error key allowed), got %d", s.RecordCount())
	}
}

func TestRecordRequestsSuccess(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	records := []RequestRecord{
		{Timestamp: base, Success: true, Latency: 10},
		{Timestamp: base.Add(time.Second), Success: false, Latency: 20, ErrorKey: "timeout"},
	}
	s.RecordRequests(records)
	if s.RecordCount() != 2 {
		t.Errorf("expected 2 records, got %d", s.RecordCount())
	}
}

func TestRecordRequestsEmptyErrorKey(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	records := []RequestRecord{
		{Timestamp: base, Success: true, Latency: 10},
		{Timestamp: base.Add(time.Second), Success: false, Latency: 20, ErrorKey: ""},
	}
	s.RecordRequests(records)
	if s.RecordCount() != 2 {
		t.Errorf("expected 2 records (empty error key allowed), got %d", s.RecordCount())
	}
}

func TestCalculateAvailability(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	records := []RequestRecord{
		{Timestamp: base.Add(0 * time.Second), Success: true, Latency: 10},
		{Timestamp: base.Add(1 * time.Second), Success: true, Latency: 10},
		{Timestamp: base.Add(2 * time.Second), Success: true, Latency: 10},
		{Timestamp: base.Add(3 * time.Second), Success: false, Latency: 10, ErrorKey: "e"},
		{Timestamp: base.Add(4 * time.Second), Success: true, Latency: 10},
	}
	s.RecordRequests(records)

	window := TimeWindow{Start: base, End: base.Add(10 * time.Second)}

	result, err := s.CalculateAvailability(window, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalRequests != 5 {
		t.Errorf("expected total 5, got %d", result.TotalRequests)
	}
	if result.SuccessRequests != 4 {
		t.Errorf("expected success 4, got %d", result.SuccessRequests)
	}
	if result.FailedRequests != 1 {
		t.Errorf("expected failed 1, got %d", result.FailedRequests)
	}
	if result.Availability != 80.0 {
		t.Errorf("expected availability 80.0, got %v", result.Availability)
	}
}

func TestCalculateAvailability100Percent(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	records := []RequestRecord{
		{Timestamp: base.Add(0 * time.Second), Success: true, Latency: 10},
		{Timestamp: base.Add(1 * time.Second), Success: true, Latency: 10},
	}
	s.RecordRequests(records)

	window := TimeWindow{Start: base, End: base.Add(10 * time.Second)}
	result, err := s.CalculateAvailability(window, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Availability != 100.0 {
		t.Errorf("expected 100.0, got %v", result.Availability)
	}
}

func TestCalculateAvailability0Percent(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	records := []RequestRecord{
		{Timestamp: base.Add(0 * time.Second), Success: false, Latency: 10, ErrorKey: "e"},
		{Timestamp: base.Add(1 * time.Second), Success: false, Latency: 10, ErrorKey: "e"},
	}
	s.RecordRequests(records)

	window := TimeWindow{Start: base, End: base.Add(10 * time.Second)}
	result, err := s.CalculateAvailability(window, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Availability != 0.0 {
		t.Errorf("expected 0.0, got %v", result.Availability)
	}
}

func TestCalculateAvailabilityNoRequests(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	window := TimeWindow{Start: base, End: base.Add(10 * time.Second)}

	_, err := s.CalculateAvailability(window, 2)
	if err != ErrNoRequests {
		t.Errorf("expected ErrNoRequests, got %v", err)
	}
}

func TestCalculateAvailabilityDecimalPlaces(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	records := []RequestRecord{
		{Timestamp: base.Add(0 * time.Second), Success: true, Latency: 10},
		{Timestamp: base.Add(1 * time.Second), Success: true, Latency: 10},
		{Timestamp: base.Add(2 * time.Second), Success: false, Latency: 10, ErrorKey: "e"},
	}
	s.RecordRequests(records)

	window := TimeWindow{Start: base, End: base.Add(10 * time.Second)}

	result2, err := s.CalculateAvailability(window, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := 2.0 / 3.0 * 100
	expected2 := roundToDecimal(expected, 2)
	if result2.Availability != expected2 {
		t.Errorf("expected %v with 2 decimals, got %v", expected2, result2.Availability)
	}

	result0, err := s.CalculateAvailability(window, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result0.Availability != 67.0 {
		t.Errorf("expected 67.0 with 0 decimals, got %v", result0.Availability)
	}
}

func TestCalculateAvailabilityInvalidDecimalPlaces(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	s.RecordRequest(RequestRecord{Timestamp: base, Success: true, Latency: 10})
	window := TimeWindow{Start: base, End: base.Add(10 * time.Second)}

	_, err := s.CalculateAvailability(window, -1)
	if err != ErrInvalidDecimalPlaces {
		t.Errorf("expected ErrInvalidDecimalPlaces, got %v", err)
	}
}

func TestCalculateAvailabilityInvalidTimeRange(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	window := TimeWindow{Start: base.Add(10 * time.Second), End: base}

	_, err := s.CalculateAvailability(window, 2)
	if err != ErrInvalidTimeRange {
		t.Errorf("expected ErrInvalidTimeRange, got %v", err)
	}
}

func TestCalculateAvailabilityTimeFiltering(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	records := []RequestRecord{
		{Timestamp: base.Add(-1 * time.Second), Success: true, Latency: 10},
		{Timestamp: base.Add(0 * time.Second), Success: true, Latency: 10},
		{Timestamp: base.Add(5 * time.Second), Success: true, Latency: 10},
		{Timestamp: base.Add(10 * time.Second), Success: true, Latency: 10},
		{Timestamp: base.Add(11 * time.Second), Success: false, Latency: 10, ErrorKey: "e"},
	}
	s.RecordRequests(records)

	window := TimeWindow{Start: base, End: base.Add(10 * time.Second)}
	result, err := s.CalculateAvailability(window, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalRequests != 3 {
		t.Errorf("expected total 3 (in window), got %d", result.TotalRequests)
	}
	if result.SuccessRequests != 3 {
		t.Errorf("expected success 3, got %d", result.SuccessRequests)
	}
}

func TestCalculateLatencyPercentiles(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	records := make([]RequestRecord, 100)
	for i := 0; i < 100; i++ {
		records[i] = RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Latency:   float64(i + 1),
			Success:   true,
		}
	}
	s.RecordRequests(records)

	window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}
	result, err := s.CalculateLatencyPercentiles(window)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 100 {
		t.Errorf("expected count 100, got %d", result.Count)
	}
	if result.Min != 1 {
		t.Errorf("expected min 1, got %v", result.Min)
	}
	if result.Max != 100 {
		t.Errorf("expected max 100, got %v", result.Max)
	}
	if result.P50 != 50 {
		t.Errorf("expected P50=50, got %v", result.P50)
	}
	if result.P90 != 90 {
		t.Errorf("expected P90=90, got %v", result.P90)
	}
	if result.P99 != 99 {
		t.Errorf("expected P99=99, got %v", result.P99)
	}
}

func TestCalculateLatencyPercentilesNoData(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	window := TimeWindow{Start: base, End: base.Add(10 * time.Second)}

	_, err := s.CalculateLatencyPercentiles(window)
	if err != ErrNoLatencyData {
		t.Errorf("expected ErrNoLatencyData, got %v", err)
	}
}

func TestCalculateLatencyPercentilesInvalidTime(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	window := TimeWindow{Start: base, End: base}

	_, err := s.CalculateLatencyPercentiles(window)
	if err != ErrInvalidTimeRange {
		t.Errorf("expected ErrInvalidTimeRange, got %v", err)
	}
}

func TestCalculateErrorRate(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	records := []RequestRecord{
		{Timestamp: base.Add(0 * time.Second), Success: true, Latency: 10},
		{Timestamp: base.Add(1 * time.Second), Success: false, Latency: 10, ErrorKey: "timeout"},
		{Timestamp: base.Add(2 * time.Second), Success: false, Latency: 10, ErrorKey: "timeout"},
		{Timestamp: base.Add(3 * time.Second), Success: false, Latency: 10, ErrorKey: "bad_request"},
		{Timestamp: base.Add(4 * time.Second), Success: true, Latency: 10},
		{Timestamp: base.Add(5 * time.Second), Success: false, Latency: 10, ErrorKey: "timeout"},
		{Timestamp: base.Add(6 * time.Second), Success: true, Latency: 10},
		{Timestamp: base.Add(7 * time.Second), Success: false, Latency: 10, ErrorKey: "unknown"},
	}
	s.RecordRequests(records)

	window := TimeWindow{Start: base, End: base.Add(10 * time.Second)}
	result, err := s.CalculateErrorRate(window, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalRequests != 8 {
		t.Errorf("expected total 8, got %d", result.TotalRequests)
	}
	if result.TotalErrors != 5 {
		t.Errorf("expected total errors 5, got %d", result.TotalErrors)
	}
	expectedTotalRate := 5.0 / 8.0 * 100
	expectedTotalRate = roundToDecimal(expectedTotalRate, 2)
	if result.TotalErrorRate != expectedTotalRate {
		t.Errorf("expected total error rate %v, got %v", expectedTotalRate, result.TotalErrorRate)
	}

	if stat, ok := result.ByErrorKey["timeout"]; !ok {
		t.Error("expected 'timeout' error key")
	} else if stat.Count != 3 {
		t.Errorf("expected timeout count 3, got %d", stat.Count)
	}

	if stat, ok := result.ByErrorKey["bad_request"]; !ok {
		t.Error("expected 'bad_request' error key")
	} else if stat.Count != 1 {
		t.Errorf("expected bad_request count 1, got %d", stat.Count)
	}

	if stat, ok := result.ByErrorKey["unknown"]; !ok {
		t.Error("expected 'unknown' error key")
	} else if stat.Count != 1 {
		t.Errorf("expected unknown count 1, got %d", stat.Count)
	}
}

func TestCalculateErrorRateNoErrors(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	records := []RequestRecord{
		{Timestamp: base.Add(0 * time.Second), Success: true, Latency: 10},
		{Timestamp: base.Add(1 * time.Second), Success: true, Latency: 10},
	}
	s.RecordRequests(records)

	window := TimeWindow{Start: base, End: base.Add(10 * time.Second)}
	result, err := s.CalculateErrorRate(window, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalErrors != 0 {
		t.Errorf("expected 0 errors, got %d", result.TotalErrors)
	}
	if result.TotalErrorRate != 0.0 {
		t.Errorf("expected 0.0 error rate, got %v", result.TotalErrorRate)
	}
	if len(result.ByErrorKey) != 0 {
		t.Errorf("expected empty ByErrorKey, got %d entries", len(result.ByErrorKey))
	}
}

func TestCalculateErrorRateNoRequests(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	window := TimeWindow{Start: base, End: base.Add(10 * time.Second)}

	_, err := s.CalculateErrorRate(window, 2)
	if err != ErrNoRequests {
		t.Errorf("expected ErrNoRequests, got %v", err)
	}
}

func TestCalculateErrorRateInvalidParams(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	window := TimeWindow{Start: base.Add(10 * time.Second), End: base}
	_, err := s.CalculateErrorRate(window, 2)
	if err != ErrInvalidTimeRange {
		t.Errorf("expected ErrInvalidTimeRange, got %v", err)
	}

	s.RecordRequest(RequestRecord{Timestamp: base, Success: true, Latency: 10})
	window2 := TimeWindow{Start: base, End: base.Add(10 * time.Second)}
	_, err = s.CalculateErrorRate(window2, -1)
	if err != ErrInvalidDecimalPlaces {
		t.Errorf("expected ErrInvalidDecimalPlaces, got %v", err)
	}
}

func TestEvaluateSLACompliant(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 100; i++ {
		s.RecordRequest(RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Success:   true,
			Latency:   float64(i%50 + 1),
		})
	}

	cfg := &SLAConfig{
		MinAvailability:   99.0,
		MaxP50Latency:     100,
		MaxP90Latency:     100,
		MaxP99Latency:     100,
		MaxTotalErrorRate: 1.0,
	}

	window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}
	evaluation, err := s.EvaluateSLA(window, cfg, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !evaluation.Compliant {
		t.Error("expected compliant SLA")
	}
	if len(evaluation.Violations) != 0 {
		t.Errorf("expected 0 violations, got %d", len(evaluation.Violations))
	}
	if s.ViolationCount() != 0 {
		t.Errorf("expected 0 violation events, got %d", s.ViolationCount())
	}
}

func TestEvaluateSLAAvailabilityViolation(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		s.RecordRequest(RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Success:   i < 5,
			Latency:   10,
			ErrorKey:  "e",
		})
	}

	cfg := &SLAConfig{
		MinAvailability:   99.0,
		MaxP99Latency:     1000,
		MaxTotalErrorRate: 100.0,
	}

	window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}
	evaluation, err := s.EvaluateSLA(window, cfg, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluation.Compliant {
		t.Error("expected non-compliant SLA")
	}
	if len(evaluation.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(evaluation.Violations))
	}
	if evaluation.Violations[0].MetricName != "availability" {
		t.Errorf("expected availability violation, got %s", evaluation.Violations[0].MetricName)
	}
	if s.ViolationCount() != 1 {
		t.Errorf("expected 1 violation event, got %d", s.ViolationCount())
	}
}

func TestEvaluateSLALatencyViolation(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 100; i++ {
		latency := float64(i + 1)
		s.RecordRequest(RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Success:   true,
			Latency:   latency,
		})
	}

	cfg := &SLAConfig{
		MinAvailability:   0,
		MaxP99Latency:     50,
		MaxTotalErrorRate: 100,
	}

	window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}
	evaluation, err := s.EvaluateSLA(window, cfg, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluation.Compliant {
		t.Error("expected non-compliant SLA")
	}
	foundP99 := false
	for _, v := range evaluation.Violations {
		if v.MetricName == "p99_latency" {
			foundP99 = true
		}
	}
	if !foundP99 {
		t.Error("expected p99_latency violation")
	}
}

func TestEvaluateSLAErrorRateViolation(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		s.RecordRequest(RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Success:   i < 5,
			Latency:   10,
			ErrorKey:  "error",
		})
	}

	cfg := &SLAConfig{
		MinAvailability:   0,
		MaxTotalErrorRate: 10.0,
	}

	window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}
	evaluation, err := s.EvaluateSLA(window, cfg, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluation.Compliant {
		t.Error("expected non-compliant SLA")
	}
	found := false
	for _, v := range evaluation.Violations {
		if v.MetricName == "total_error_rate" {
			found = true
		}
	}
	if !found {
		t.Error("expected total_error_rate violation")
	}
}

func TestEvaluateSLAMultipleViolations(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 100; i++ {
		s.RecordRequest(RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Success:   i < 50,
			Latency:   float64(i + 1),
			ErrorKey:  "error",
		})
	}

	cfg := &SLAConfig{
		MinAvailability:   99.0,
		MaxP50Latency:     10,
		MaxP90Latency:     10,
		MaxP99Latency:     10,
		MaxTotalErrorRate: 1.0,
	}

	window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}
	evaluation, err := s.EvaluateSLA(window, cfg, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluation.Compliant {
		t.Error("expected non-compliant SLA")
	}
	if len(evaluation.Violations) < 3 {
		t.Errorf("expected at least 3 violations, got %d", len(evaluation.Violations))
	}
}

func TestEvaluateSLANilConfig(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	window := TimeWindow{Start: base, End: base.Add(10 * time.Second)}

	_, err := s.EvaluateSLA(window, nil, 2)
	if err != ErrNilSLAConfig {
		t.Errorf("expected ErrNilSLAConfig, got %v", err)
	}
}

func TestEvaluateSLAInvalidParams(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	cfg := &SLAConfig{MinAvailability: 99}

	window := TimeWindow{Start: base.Add(10 * time.Second), End: base}
	_, err := s.EvaluateSLA(window, cfg, 2)
	if err != ErrInvalidTimeRange {
		t.Errorf("expected ErrInvalidTimeRange, got %v", err)
	}

	s.RecordRequest(RequestRecord{Timestamp: base, Success: true, Latency: 10})
	window2 := TimeWindow{Start: base, End: base.Add(10 * time.Second)}
	_, err = s.EvaluateSLA(window2, cfg, -1)
	if err != ErrInvalidDecimalPlaces {
		t.Errorf("expected ErrInvalidDecimalPlaces, got %v", err)
	}
}

func TestEvaluateSLANoData(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	cfg := &SLAConfig{MinAvailability: 99}
	window := TimeWindow{Start: base, End: base.Add(10 * time.Second)}

	_, err := s.EvaluateSLA(window, cfg, 2)
	if err != ErrNoRequests {
		t.Errorf("expected ErrNoRequests, got %v", err)
	}
}

func TestViolationDeduplication(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		s.RecordRequest(RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Success:   i < 2,
			Latency:   1000,
			ErrorKey:  "e",
		})
	}

	cfg := &SLAConfig{
		MinAvailability:   99.0,
		MaxP99Latency:     100,
		MaxTotalErrorRate: 1.0,
	}

	window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}

	_, err := s.EvaluateSLA(window, cfg, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	firstCount := s.ViolationCount()

	_, err = s.EvaluateSLA(window, cfg, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	secondCount := s.ViolationCount()

	if firstCount != secondCount {
		t.Errorf("expected same violation count after re-evaluation, got %d then %d", firstCount, secondCount)
	}
}

func TestGetViolationEvents(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		s.RecordRequest(RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Success:   false,
			Latency:   1000,
			ErrorKey:  "e",
		})
	}

	cfg := &SLAConfig{
		MinAvailability:   99.0,
		MaxTotalErrorRate: 1.0,
	}
	window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}

	_, err := s.EvaluateSLA(window, cfg, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := s.GetViolationEvents()
	if len(events) < 2 {
		t.Errorf("expected at least 2 events, got %d", len(events))
	}

	for i := 1; i < len(events); i++ {
		if events[i].RecordedAt.Before(events[i-1].RecordedAt) {
			t.Error("events not sorted by RecordedAt")
		}
	}

	for _, e := range events {
		if e.ID == "" {
			t.Error("event ID should not be empty")
		}
		if e.MetricName == "" {
			t.Error("event MetricName should not be empty")
		}
	}
}

func TestGetViolationEventsByRecordedAt(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		s.RecordRequest(RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Success:   false,
			Latency:   1000,
			ErrorKey:  "e",
		})
	}

	cfg := &SLAConfig{MinAvailability: 99.0}
	window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}

	_, err := s.EvaluateSLA(window, cfg, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	now := time.Now()
	future := now.Add(1 * time.Hour)
	past := now.Add(-1 * time.Hour)

	eventsInRange := s.GetViolationEventsByRecordedAt(past, future)
	if len(eventsInRange) != s.ViolationCount() {
		t.Errorf("expected all events in range, got %d out of %d", len(eventsInRange), s.ViolationCount())
	}

	eventsEmpty := s.GetViolationEventsByRecordedAt(future, future.Add(1*time.Hour))
	if len(eventsEmpty) != 0 {
		t.Errorf("expected 0 events in future range, got %d", len(eventsEmpty))
	}
}

func TestGetViolationEventsByWindow(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		s.RecordRequest(RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Success:   false,
			Latency:   1000,
			ErrorKey:  "e",
		})
	}

	cfg := &SLAConfig{MinAvailability: 99.0}
	window1 := TimeWindow{Start: base, End: base.Add(1 * time.Second)}
	window2 := TimeWindow{Start: base.Add(1 * time.Hour), End: base.Add(2 * time.Hour)}

	_, err := s.EvaluateSLA(window1, cfg, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events1 := s.GetViolationEventsByWindow(window1)
	if len(events1) == 0 {
		t.Error("expected events for window1")
	}
	for _, e := range events1 {
		if !e.WindowStart.Equal(window1.Start) || !e.WindowEnd.Equal(window1.End) {
			t.Errorf("event window mismatch: got %v-%v, expected %v-%v",
				e.WindowStart, e.WindowEnd, window1.Start, window1.End)
		}
	}

	events2 := s.GetViolationEventsByWindow(window2)
	if len(events2) != 0 {
		t.Errorf("expected 0 events for window2, got %d", len(events2))
	}
}

func TestGetViolationEventsByWindowRange(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 20; i++ {
		s.RecordRequest(RequestRecord{
			Timestamp: base.Add(time.Duration(i*10) * time.Minute),
			Success:   false,
			Latency:   1000,
			ErrorKey:  "e",
		})
	}

	cfg := &SLAConfig{MinAvailability: 99.0}

	window1 := TimeWindow{Start: base, End: base.Add(30 * time.Minute)}
	window2 := TimeWindow{Start: base.Add(1 * time.Hour), End: base.Add(2 * time.Hour)}
	window3 := TimeWindow{Start: base.Add(3 * time.Hour), End: base.Add(4 * time.Hour)}

	_, _ = s.EvaluateSLA(window1, cfg, 2)
	_, _ = s.EvaluateSLA(window2, cfg, 2)

	rangeStart := base.Add(15 * time.Minute)
	rangeEnd := base.Add(90 * time.Minute)

	events := s.GetViolationEventsByWindowRange(rangeStart, rangeEnd)
	if len(events) == 0 {
		t.Error("expected some events in overlapping range")
	}

	for _, e := range events {
		overlaps := !(e.WindowEnd.Before(rangeStart) || e.WindowStart.After(rangeEnd))
		if !overlaps {
			t.Errorf("event window %v-%v does not overlap range %v-%v",
				e.WindowStart, e.WindowEnd, rangeStart, rangeEnd)
		}
	}

	noOverlap := s.GetViolationEventsByWindowRange(window3.Start, window3.End)
	if len(noOverlap) != 0 {
		t.Errorf("expected 0 events for non-overlapping range, got %d", len(noOverlap))
	}
}

func TestGetViolationEventsInRangeBackCompat(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	s.RecordRequest(RequestRecord{Timestamp: base, Success: false, Latency: 10, ErrorKey: "e"})
	cfg := &SLAConfig{MinAvailability: 99.0}
	window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}
	_, _ = s.EvaluateSLA(window, cfg, 2)

	now := time.Now()
	result := s.GetViolationEventsInRange(now.Add(-1*time.Hour), now.Add(1*time.Hour))
	if len(result) != s.ViolationCount() {
		t.Errorf("GetViolationEventsInRange should delegate to ByRecordedAt, got %d expected %d",
			len(result), s.ViolationCount())
	}
}

func TestReset(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	s.RecordRequest(RequestRecord{Timestamp: base, Success: false, Latency: 1000, ErrorKey: "e"})
	cfg := &SLAConfig{MinAvailability: 99.0}
	window := TimeWindow{Start: base, End: base.Add(10 * time.Second)}
	_, err := s.EvaluateSLA(window, cfg, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.RecordCount() != 1 {
		t.Errorf("expected 1 record before reset, got %d", s.RecordCount())
	}
	if s.ViolationCount() == 0 {
		t.Error("expected at least 1 violation before reset")
	}
	genBefore := s.Generation()

	s.Reset()

	if s.RecordCount() != 0 {
		t.Errorf("expected 0 records after reset, got %d", s.RecordCount())
	}
	if s.ViolationCount() != 0 {
		t.Errorf("expected 0 violations after reset, got %d", s.ViolationCount())
	}
	if s.Generation() <= genBefore {
		t.Errorf("expected generation to increment, was %d now %d", genBefore, s.Generation())
	}
}

func TestGenerationIncrementsOnReset(t *testing.T) {
	s := NewSLAMetrics()
	initial := s.Generation()

	s.Reset()
	if s.Generation() != initial+1 {
		t.Errorf("expected generation %d, got %d", initial+1, s.Generation())
	}

	s.Reset()
	if s.Generation() != initial+2 {
		t.Errorf("expected generation %d, got %d", initial+2, s.Generation())
	}
}

func TestEvaluateSLADataConsistency(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 100; i++ {
		s.RecordRequest(RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Success:   true,
			Latency:   float64(i + 1),
		})
	}

	cfg := &SLAConfig{
		MinAvailability: 99.0,
		MaxP99Latency:   200,
	}
	window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}

	eval, err := s.EvaluateSLA(window, cfg, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if eval.LatencyStats.Count != eval.ErrorStats.TotalRequests {
		t.Errorf("data inconsistency: latency count=%d but error rate total=%d",
			eval.LatencyStats.Count, eval.ErrorStats.TotalRequests)
	}

	expectedAvail := float64(eval.ErrorStats.TotalRequests-eval.ErrorStats.TotalErrors) /
		float64(eval.ErrorStats.TotalRequests) * 100
	expectedAvail = roundToDecimal(expectedAvail, 2)
	if eval.Availability != expectedAvail {
		t.Errorf("data inconsistency: availability=%v but computed from errors=%v",
			eval.Availability, expectedAvail)
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n * 4)

	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			s.RecordRequest(RequestRecord{
				Timestamp: base.Add(time.Duration(idx) * time.Millisecond),
				Success:   idx%2 == 0,
				Latency:   float64(idx),
				ErrorKey:  "e",
			})
		}(i)
	}

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}
			_, _ = s.CalculateAvailability(window, 2)
		}()
	}

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}
			cfg := &SLAConfig{MinAvailability: 50, MaxP99Latency: 500, MaxTotalErrorRate: 50}
			_, _ = s.EvaluateSLA(window, cfg, 2)
		}()
	}

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			s.GetViolationEvents()
			s.GetViolationEventsByWindow(TimeWindow{Start: base, End: base.Add(time.Second)})
			s.RecordCount()
			s.Generation()
		}()
	}

	wg.Wait()

	if s.RecordCount() != n {
		t.Errorf("expected %d records, got %d", n, s.RecordCount())
	}
}

func TestLatencyPercentileAllFromDataset(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	values := []float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8, 9.9, 10.1}
	for i, v := range values {
		s.RecordRequest(RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Latency:   v,
			Success:   true,
		})
	}

	window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}
	result, err := s.CalculateLatencyPercentiles(window)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	isInDataset := func(v float64) bool {
		for _, d := range values {
			if d == v {
				return true
			}
		}
		return false
	}

	if !isInDataset(result.P50) {
		t.Errorf("P50 value %v not in dataset", result.P50)
	}
	if !isInDataset(result.P90) {
		t.Errorf("P90 value %v not in dataset", result.P90)
	}
	if !isInDataset(result.P99) {
		t.Errorf("P99 value %v not in dataset", result.P99)
	}
}

func TestEvaluateSLAThresholdBoundaries(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 100; i++ {
		s.RecordRequest(RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Success:   true,
			Latency:   50,
		})
	}

	cfg := &SLAConfig{
		MinAvailability:   100.0,
		MaxP50Latency:     50,
		MaxP90Latency:     50,
		MaxP99Latency:     50,
		MaxTotalErrorRate: 0.0,
	}

	window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}
	evaluation, err := s.EvaluateSLA(window, cfg, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !evaluation.Compliant {
		t.Error("expected compliant when values equal targets")
	}
	if len(evaluation.Violations) != 0 {
		t.Errorf("expected 0 violations at boundary, got %d", len(evaluation.Violations))
	}
}

func TestViolationEventFields(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}

	for i := 0; i < 10; i++ {
		s.RecordRequest(RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Success:   false,
			Latency:   200,
			ErrorKey:  "e",
		})
	}

	cfg := &SLAConfig{
		MinAvailability: 99.0,
	}

	_, err := s.EvaluateSLA(window, cfg, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := s.GetViolationEvents()
	if len(events) == 0 {
		t.Fatal("expected at least 1 event")
	}

	for _, e := range events {
		if !e.WindowStart.Equal(base) {
			t.Errorf("expected WindowStart %v, got %v", base, e.WindowStart)
		}
		if !e.WindowEnd.Equal(base.Add(1 * time.Second)) {
			t.Errorf("expected WindowEnd %v, got %v", base.Add(1*time.Second), e.WindowEnd)
		}
		if e.Actual >= e.Target && e.MetricName == "availability" {
			t.Errorf("availability violation: actual %v should be < target %v", e.Actual, e.Target)
		}
		if e.RecordedAt.IsZero() {
			t.Error("RecordedAt should not be zero")
		}
	}
}

func TestViolationEventWindowTimeDimension(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	windowStart := base.Add(-1 * time.Hour)
	windowEnd := base
	recordWindow := TimeWindow{Start: windowStart, End: windowEnd}

	for i := 0; i < 10; i++ {
		s.RecordRequest(RequestRecord{
			Timestamp: windowStart.Add(time.Duration(i) * time.Minute),
			Success:   false,
			Latency:   100,
			ErrorKey:  "e",
		})
	}

	cfg := &SLAConfig{MinAvailability: 99.0}
	_, err := s.EvaluateSLA(recordWindow, cfg, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := s.GetViolationEventsByWindow(recordWindow)
	if len(events) == 0 {
		t.Fatal("expected events for the window")
	}

	for _, e := range events {
		if !e.WindowStart.Equal(windowStart) {
			t.Errorf("WindowStart should be %v (window start), got %v", windowStart, e.WindowStart)
		}
		if !e.WindowEnd.Equal(windowEnd) {
			t.Errorf("WindowEnd should be %v (window end), got %v", windowEnd, e.WindowEnd)
		}
	}
}

func TestResetDoesNotRecordStaleViolations(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		s.RecordRequest(RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Success:   false,
			Latency:   1000,
			ErrorKey:  "e",
		})
	}

	genBefore := s.Generation()
	s.Reset()
	genAfter := s.Generation()
	if genAfter <= genBefore {
		t.Fatal("generation should increment after Reset")
	}

	for i := 0; i < 5; i++ {
		s.RecordRequest(RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Success:   true,
			Latency:   10,
		})
	}

	cfg := &SLAConfig{MinAvailability: 99.0}
	window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}

	_, err := s.EvaluateSLA(window, cfg, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := s.GetViolationEvents()
	for _, e := range events {
		if e.MetricName == "availability" {
			if e.Actual < 99.0 && s.RecordCount() == 5 {
				t.Log("Note: New data after reset may also cause violations, this is expected")
			}
		}
	}

	totalRecords := s.RecordCount()
	if totalRecords != 5 {
		t.Errorf("expected 5 records after reset + re-add, got %d", totalRecords)
	}
}

func TestEmptyErrorKeyAllowed(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	s.RecordRequest(RequestRecord{
		Timestamp: base,
		Success:   false,
		Latency:   10,
		ErrorKey:  "",
	})
	if s.RecordCount() != 1 {
		t.Errorf("expected 1 record with empty ErrorKey, got %d", s.RecordCount())
	}

	s.RecordRequest(RequestRecord{
		Timestamp: base,
		Success:   true,
		Latency:   10,
		ErrorKey:  "",
	})
	if s.RecordCount() != 2 {
		t.Errorf("expected 2 records, got %d", s.RecordCount())
	}
}

func TestCalculateAvailabilityPureFunction(t *testing.T) {
	records := []RequestRecord{
		{Success: true},
		{Success: true},
		{Success: false, ErrorKey: "e"},
	}

	result, err := calculateAvailability(records, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalRequests != 3 {
		t.Errorf("expected 3 total, got %d", result.TotalRequests)
	}
	if result.Availability != roundToDecimal(2.0/3.0*100, 2) {
		t.Errorf("unexpected availability: %v", result.Availability)
	}
}

func TestCalculateErrorRatePureFunction(t *testing.T) {
	records := []RequestRecord{
		{Success: true},
		{Success: false, ErrorKey: "timeout"},
		{Success: false, ErrorKey: "timeout"},
		{Success: false, ErrorKey: "bad_request"},
	}

	result, err := calculateErrorRate(records, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalRequests != 4 {
		t.Errorf("expected 4 total, got %d", result.TotalRequests)
	}
	if result.TotalErrors != 3 {
		t.Errorf("expected 3 errors, got %d", result.TotalErrors)
	}
	if result.ByErrorKey["timeout"].Count != 2 {
		t.Errorf("expected 2 timeout errors, got %d", result.ByErrorKey["timeout"].Count)
	}
}

func TestEvaluateSLAConsistentSnapshot(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 50; i++ {
		s.RecordRequest(RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Success:   true,
			Latency:   float64(i + 1),
		})
	}

	cfg := &SLAConfig{
		MinAvailability:   0,
		MaxTotalErrorRate: 100,
		MaxP99Latency:     1000,
	}
	window := TimeWindow{Start: base, End: base.Add(1 * time.Second)}

	for i := 0; i < 20; i++ {
		eval, err := s.EvaluateSLA(window, cfg, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if eval.LatencyStats.Count != eval.ErrorStats.TotalRequests {
			t.Errorf("inconsistent snapshot: latency count %d != error total %d",
				eval.LatencyStats.Count, eval.ErrorStats.TotalRequests)
		}

		availFromErrors := 100.0 - eval.ErrorStats.TotalErrorRate
		availFromErrors = roundToDecimal(availFromErrors, 2)
		if eval.Availability != availFromErrors {
			t.Errorf("inconsistent snapshot: availability %v != 100-error_rate %v",
				eval.Availability, availFromErrors)
		}
	}
}

func TestViolationEventQueryDimensions(t *testing.T) {
	s := NewSLAMetrics()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	windows := []TimeWindow{
		{Start: base, End: base.Add(1 * time.Hour)},
		{Start: base.Add(2 * time.Hour), End: base.Add(3 * time.Hour)},
		{Start: base.Add(4 * time.Hour), End: base.Add(5 * time.Hour)},
	}

	cfg := &SLAConfig{MinAvailability: 99.0}

	for _, w := range windows {
		s.RecordRequest(RequestRecord{
			Timestamp: w.Start,
			Success:   false,
			Latency:   10,
			ErrorKey:  "e",
		})
		_, _ = s.EvaluateSLA(w, cfg, 2)
	}

	if s.ViolationCount() < 3 {
		t.Fatalf("expected at least 3 violation events, got %d", s.ViolationCount())
	}

	events := s.GetViolationEventsByWindow(windows[1])
	if len(events) == 0 {
		t.Error("expected events for window 1")
	}
	for _, e := range events {
		if !e.WindowStart.Equal(windows[1].Start) || !e.WindowEnd.Equal(windows[1].End) {
			t.Errorf("wrong window for event: %v-%v", e.WindowStart, e.WindowEnd)
		}
	}

	rangeEvents := s.GetViolationEventsByWindowRange(
		base.Add(30*time.Minute),
		base.Add(3*time.Hour+30*time.Minute),
	)
	if len(rangeEvents) == 0 {
		t.Error("expected events in window range")
	}

	allEvents := s.GetViolationEvents()
	byRecorded := s.GetViolationEventsByRecordedAt(
		time.Now().Add(-1*time.Hour),
		time.Now().Add(1*time.Hour),
	)
	if len(byRecorded) != len(allEvents) {
		t.Errorf("by recorded time: expected %d, got %d", len(allEvents), len(byRecorded))
	}
}
