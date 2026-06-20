package slametrics

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

type SLAMetrics struct {
	mu             sync.RWMutex
	records        []RequestRecord
	violationEvents []ViolationEvent
	violationSet   map[string]struct{}
}

func NewSLAMetrics() *SLAMetrics {
	return &SLAMetrics{
		records:        make([]RequestRecord, 0),
		violationEvents: make([]ViolationEvent, 0),
		violationSet:   make(map[string]struct{}),
	}
}

func (s *SLAMetrics) RecordRequest(r RequestRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, r)
}

func (s *SLAMetrics) RecordRequests(records []RequestRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, records...)
}

func (s *SLAMetrics) filterRecordsLocked(start, end time.Time) []RequestRecord {
	result := make([]RequestRecord, 0)
	for _, r := range s.records {
		if (r.Timestamp.Equal(start) || r.Timestamp.After(start)) && (r.Timestamp.Before(end) || r.Timestamp.Equal(end)) {
			result = append(result, r)
		}
	}
	return result
}

func (s *SLAMetrics) CalculateAvailability(window TimeWindow, decimalPlaces int) (AvailabilityResult, error) {
	if !window.Start.Before(window.End) {
		return AvailabilityResult{}, ErrInvalidTimeRange
	}
	if decimalPlaces < 0 {
		return AvailabilityResult{}, ErrInvalidDecimalPlaces
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	records := s.filterRecordsLocked(window.Start, window.End)
	total := len(records)

	if total == 0 {
		return AvailabilityResult{}, ErrNoRequests
	}

	successCount := 0
	for _, r := range records {
		if r.Success {
			successCount++
		}
	}

	availability := float64(successCount) / float64(total) * 100
	availability = roundToDecimal(availability, decimalPlaces)

	return AvailabilityResult{
		TotalRequests:   total,
		SuccessRequests: successCount,
		FailedRequests:  total - successCount,
		Availability:    availability,
	}, nil
}

func (s *SLAMetrics) CalculateLatencyPercentiles(window TimeWindow) (LatencyPercentiles, error) {
	if !window.Start.Before(window.End) {
		return LatencyPercentiles{}, ErrInvalidTimeRange
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	records := s.filterRecordsLocked(window.Start, window.End)
	latencies := make([]float64, 0, len(records))

	for _, r := range records {
		latencies = append(latencies, r.Latency)
	}

	return computePercentiles(latencies)
}

func computePercentiles(latencies []float64) (LatencyPercentiles, error) {
	if len(latencies) == 0 {
		return LatencyPercentiles{}, ErrNoLatencyData
	}

	sorted := make([]float64, len(latencies))
	copy(sorted, latencies)
	sort.Float64s(sorted)

	n := len(sorted)

	p50 := nearestRankPercentile(sorted, 50)
	p90 := nearestRankPercentile(sorted, 90)
	p99 := nearestRankPercentile(sorted, 99)

	return LatencyPercentiles{
		P50:   p50,
		P90:   p90,
		P99:   p99,
		Count: n,
		Min:   sorted[0],
		Max:   sorted[n-1],
	}, nil
}

func nearestRankPercentile(sorted []float64, percentile float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if percentile <= 0 {
		return sorted[0]
	}
	if percentile >= 100 {
		return sorted[len(sorted)-1]
	}

	rank := int(math.Ceil(percentile / 100.0 * float64(len(sorted))))
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}

func (s *SLAMetrics) CalculateErrorRate(window TimeWindow, decimalPlaces int) (ErrorRateResult, error) {
	if !window.Start.Before(window.End) {
		return ErrorRateResult{}, ErrInvalidTimeRange
	}
	if decimalPlaces < 0 {
		return ErrorRateResult{}, ErrInvalidDecimalPlaces
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	records := s.filterRecordsLocked(window.Start, window.End)
	total := len(records)

	if total == 0 {
		return ErrorRateResult{}, ErrNoRequests
	}

	errorCounts := make(map[string]int)
	totalErrors := 0

	for _, r := range records {
		if !r.Success {
			totalErrors++
			key := r.ErrorKey
			if key == "" {
				key = "unknown"
			}
			errorCounts[key]++
		}
	}

	totalErrorRate := roundToDecimal(float64(totalErrors)/float64(total)*100, decimalPlaces)

	byErrorKey := make(map[string]ErrorStat, len(errorCounts))
	for key, count := range errorCounts {
		rate := roundToDecimal(float64(count)/float64(total)*100, decimalPlaces)
		byErrorKey[key] = ErrorStat{
			Count:    count,
			ErrorRate: rate,
		}
	}

	return ErrorRateResult{
		TotalRequests:  total,
		TotalErrors:    totalErrors,
		TotalErrorRate: totalErrorRate,
		ByErrorKey:     byErrorKey,
	}, nil
}

func (s *SLAMetrics) EvaluateSLA(window TimeWindow, cfg *SLAConfig, decimalPlaces int) (SLAEvaluation, error) {
	if cfg == nil {
		return SLAEvaluation{}, ErrNilSLAConfig
	}
	if !window.Start.Before(window.End) {
		return SLAEvaluation{}, ErrInvalidTimeRange
	}
	if decimalPlaces < 0 {
		return SLAEvaluation{}, ErrInvalidDecimalPlaces
	}

	availability, err := s.CalculateAvailability(window, decimalPlaces)
	if err != nil {
		return SLAEvaluation{}, err
	}

	latencyStats, err := s.CalculateLatencyPercentiles(window)
	if err != nil {
		return SLAEvaluation{}, err
	}

	errorStats, err := s.CalculateErrorRate(window, decimalPlaces)
	if err != nil {
		return SLAEvaluation{}, err
	}

	evaluation := SLAEvaluation{
		WindowStart:  window.Start,
		WindowEnd:    window.End,
		Compliant:    true,
		Violations:   make([]ViolationDetail, 0),
		Availability: availability.Availability,
		LatencyStats: latencyStats,
		ErrorStats:   errorStats,
	}

	if availability.Availability < cfg.MinAvailability {
		evaluation.Compliant = false
		evaluation.Violations = append(evaluation.Violations, ViolationDetail{
			MetricName: "availability",
			Actual:     availability.Availability,
			Target:     cfg.MinAvailability,
		})
	}

	if cfg.MaxP50Latency > 0 && latencyStats.P50 > cfg.MaxP50Latency {
		evaluation.Compliant = false
		evaluation.Violations = append(evaluation.Violations, ViolationDetail{
			MetricName: "p50_latency",
			Actual:     latencyStats.P50,
			Target:     cfg.MaxP50Latency,
		})
	}

	if cfg.MaxP90Latency > 0 && latencyStats.P90 > cfg.MaxP90Latency {
		evaluation.Compliant = false
		evaluation.Violations = append(evaluation.Violations, ViolationDetail{
			MetricName: "p90_latency",
			Actual:     latencyStats.P90,
			Target:     cfg.MaxP90Latency,
		})
	}

	if cfg.MaxP99Latency > 0 && latencyStats.P99 > cfg.MaxP99Latency {
		evaluation.Compliant = false
		evaluation.Violations = append(evaluation.Violations, ViolationDetail{
			MetricName: "p99_latency",
			Actual:     latencyStats.P99,
			Target:     cfg.MaxP99Latency,
		})
	}

	if cfg.MaxTotalErrorRate >= 0 && errorStats.TotalErrorRate > cfg.MaxTotalErrorRate {
		evaluation.Compliant = false
		evaluation.Violations = append(evaluation.Violations, ViolationDetail{
			MetricName: "total_error_rate",
			Actual:     errorStats.TotalErrorRate,
			Target:     cfg.MaxTotalErrorRate,
		})
	}

	if !evaluation.Compliant {
		s.recordViolations(window, evaluation.Violations)
	}

	return evaluation, nil
}

func (s *SLAMetrics) recordViolations(window TimeWindow, violations []ViolationDetail) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, v := range violations {
		key := fmt.Sprintf("%d_%d_%s", window.Start.UnixNano(), window.End.UnixNano(), v.MetricName)
		if _, exists := s.violationSet[key]; exists {
			continue
		}

		event := ViolationEvent{
			ID:          key,
			WindowStart: window.Start,
			WindowEnd:   window.End,
			MetricName:  v.MetricName,
			Actual:      v.Actual,
			Target:      v.Target,
			RecordedAt:  time.Now(),
		}
		s.violationEvents = append(s.violationEvents, event)
		s.violationSet[key] = struct{}{}
	}

	s.sortViolationsLocked()
}

func (s *SLAMetrics) sortViolationsLocked() {
	sort.Slice(s.violationEvents, func(i, j int) bool {
		return s.violationEvents[i].RecordedAt.Before(s.violationEvents[j].RecordedAt)
	})
}

func (s *SLAMetrics) GetViolationEvents() []ViolationEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ViolationEvent, len(s.violationEvents))
	copy(result, s.violationEvents)
	return result
}

func (s *SLAMetrics) GetViolationEventsInRange(start, end time.Time) []ViolationEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ViolationEvent, 0)
	for _, e := range s.violationEvents {
		if (e.RecordedAt.Equal(start) || e.RecordedAt.After(start)) && (e.RecordedAt.Before(end) || e.RecordedAt.Equal(end)) {
			result = append(result, e)
		}
	}
	return result
}

func (s *SLAMetrics) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = s.records[:0]
	s.violationEvents = s.violationEvents[:0]
	s.violationSet = make(map[string]struct{})
}

func (s *SLAMetrics) RecordCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

func (s *SLAMetrics) ViolationCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.violationEvents)
}

func roundToDecimal(value float64, decimals int) float64 {
	if decimals < 0 {
		decimals = 0
	}
	shift := math.Pow(10, float64(decimals))
	return math.Round(value*shift) / shift
}
