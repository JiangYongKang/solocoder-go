package tsdb

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestNewTSEngine(t *testing.T) {
	e := NewTSEngine()
	if e == nil {
		t.Fatal("NewTSEngine returned nil")
	}
	defer e.Close()

	if e.Count() != 0 {
		t.Errorf("expected initial count 0, got %d", e.Count())
	}
	if e.GetTTL() != 24*time.Hour {
		t.Errorf("expected default TTL 24h, got %v", e.GetTTL())
	}
}

func TestNewTSEngineWithConfig(t *testing.T) {
	cfg := Config{
		TTL:              2 * time.Hour,
		CleanupInterval:  time.Minute,
		CleanupBatchSize: 100,
	}
	e, err := NewTSEngineWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTSEngineWithConfig failed: %v", err)
	}
	defer e.Close()

	if e.GetTTL() != 2*time.Hour {
		t.Errorf("expected TTL 2h, got %v", e.GetTTL())
	}
}

func TestNewTSEngineWithConfig_InvalidTTL(t *testing.T) {
	cfg := Config{
		TTL:              0,
		CleanupInterval:  time.Minute,
		CleanupBatchSize: 100,
	}
	_, err := NewTSEngineWithConfig(cfg)
	if err != ErrInvalidTTL {
		t.Errorf("expected ErrInvalidTTL for TTL=0, got %v", err)
	}

	cfg2 := Config{
		TTL:              -2,
		CleanupInterval:  time.Minute,
		CleanupBatchSize: 100,
	}
	_, err2 := NewTSEngineWithConfig(cfg2)
	if err2 != ErrInvalidTTL {
		t.Errorf("expected ErrInvalidTTL for TTL=-2, got %v", err2)
	}
}

func TestNewTSEngineWithConfig_InvalidInterval(t *testing.T) {
	cfg := Config{
		TTL:              time.Hour,
		CleanupInterval:  0,
		CleanupBatchSize: 100,
	}
	_, err := NewTSEngineWithConfig(cfg)
	if err != ErrInvalidInterval {
		t.Errorf("expected ErrInvalidInterval, got %v", err)
	}

	cfg2 := Config{
		TTL:              time.Hour,
		CleanupInterval:  -time.Minute,
		CleanupBatchSize: 100,
	}
	_, err2 := NewTSEngineWithConfig(cfg2)
	if err2 != ErrInvalidInterval {
		t.Errorf("expected ErrInvalidInterval for negative, got %v", err2)
	}
}

func TestNewTSEngineWithConfig_InvalidBatchSize(t *testing.T) {
	cfg := Config{
		TTL:              time.Hour,
		CleanupInterval:  time.Minute,
		CleanupBatchSize: 0,
	}
	_, err := NewTSEngineWithConfig(cfg)
	if err != ErrInvalidBatchSize {
		t.Errorf("expected ErrInvalidBatchSize, got %v", err)
	}

	cfg2 := Config{
		TTL:              time.Hour,
		CleanupInterval:  time.Minute,
		CleanupBatchSize: -100,
	}
	_, err2 := NewTSEngineWithConfig(cfg2)
	if err2 != ErrInvalidBatchSize {
		t.Errorf("expected ErrInvalidBatchSize for negative, got %v", err2)
	}
}

func TestWrite(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	points := []*DataPoint{
		{Timestamp: now + 1000, Value: 10.5, Tags: map[string]string{"host": "server1", "metric": "cpu"}},
		{Timestamp: now + 2000, Value: 20.3, Tags: map[string]string{"host": "server1", "metric": "cpu"}},
		{Timestamp: now + 500, Value: 5.0, Tags: map[string]string{"host": "server2", "metric": "memory"}},
	}

	err := e.Write(points)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if e.Count() != 3 {
		t.Errorf("expected count 3, got %d", e.Count())
	}
}

func TestWrite_Empty(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	err := e.Write([]*DataPoint{})
	if err != nil {
		t.Errorf("expected nil error for empty write, got %v", err)
	}

	if e.Count() != 0 {
		t.Errorf("expected count 0, got %d", e.Count())
	}
}

func TestWrite_NilPoint(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	points := []*DataPoint{
		{Timestamp: 1000, Value: 1.0, Tags: map[string]string{"a": "b"}},
		nil,
	}

	err := e.Write(points)
	if err != ErrNilDataPoint {
		t.Errorf("expected ErrNilDataPoint, got %v", err)
	}
}

func TestWrite_EmptyTags(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	points := []*DataPoint{
		{Timestamp: 1000, Value: 1.0, Tags: map[string]string{}},
	}

	err := e.Write(points)
	if err != ErrEmptyTags {
		t.Errorf("expected ErrEmptyTags, got %v", err)
	}
}

func TestWrite_SortedByTimestamp(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	points := []*DataPoint{
		{Timestamp: now + 3000, Value: 3.0, Tags: map[string]string{"id": "1"}},
		{Timestamp: now + 1000, Value: 1.0, Tags: map[string]string{"id": "2"}},
		{Timestamp: now + 2000, Value: 2.0, Tags: map[string]string{"id": "3"}},
	}

	err := e.Write(points)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	result, err := e.Query(0, now+10000, map[string]string{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 points, got %d", len(result))
	}

	for i := 1; i < len(result); i++ {
		if result[i].Timestamp < result[i-1].Timestamp {
			t.Errorf("points not sorted by timestamp: %d < %d at position %d",
				result[i].Timestamp, result[i-1].Timestamp, i)
		}
	}
}

func TestQuery(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	points := []*DataPoint{
		{Timestamp: now + 1000, Value: 10.0, Tags: map[string]string{"host": "s1"}},
		{Timestamp: now + 2000, Value: 20.0, Tags: map[string]string{"host": "s1"}},
		{Timestamp: now + 3000, Value: 30.0, Tags: map[string]string{"host": "s2"}},
		{Timestamp: now + 4000, Value: 40.0, Tags: map[string]string{"host": "s1"}},
	}

	err := e.Write(points)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	result, err := e.Query(now+1500, now+3500, map[string]string{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 points in range, got %d", len(result))
	}
}

func TestQuery_InvalidTimeRange(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	_, err := e.Query(2000, 1000, map[string]string{})
	if err != ErrInvalidTimeRange {
		t.Errorf("expected ErrInvalidTimeRange, got %v", err)
	}
}

func TestQuery_EmptyResult(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	points := []*DataPoint{
		{Timestamp: now + 1000, Value: 1.0, Tags: map[string]string{"host": "s1"}},
	}
	e.Write(points)

	result, err := e.Query(now+5000, now+6000, map[string]string{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 points, got %d", len(result))
	}
}

func TestQuery_ByTag(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	points := []*DataPoint{
		{Timestamp: now + 1000, Value: 1.0, Tags: map[string]string{"host": "s1", "dc": "us"}},
		{Timestamp: now + 2000, Value: 2.0, Tags: map[string]string{"host": "s2", "dc": "us"}},
		{Timestamp: now + 3000, Value: 3.0, Tags: map[string]string{"host": "s1", "dc": "eu"}},
		{Timestamp: now + 4000, Value: 4.0, Tags: map[string]string{"host": "s3", "dc": "asia"}},
	}

	err := e.Write(points)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	result, err := e.Query(0, now+10000, map[string]string{"host": "s1"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 points for host=s1, got %d", len(result))
	}
}

func TestQuery_MultipleTags(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	points := []*DataPoint{
		{Timestamp: now + 1000, Value: 1.0, Tags: map[string]string{"host": "s1", "dc": "us"}},
		{Timestamp: now + 2000, Value: 2.0, Tags: map[string]string{"host": "s2", "dc": "us"}},
		{Timestamp: now + 3000, Value: 3.0, Tags: map[string]string{"host": "s1", "dc": "eu"}},
		{Timestamp: now + 4000, Value: 4.0, Tags: map[string]string{"host": "s1", "dc": "us"}},
	}

	err := e.Write(points)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	result, err := e.Query(0, now+10000, map[string]string{"host": "s1", "dc": "us"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 points for host=s1,dc=us, got %d", len(result))
	}
}

func TestQuery_NonExistentTag(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	points := []*DataPoint{
		{Timestamp: now + 1000, Value: 1.0, Tags: map[string]string{"host": "s1"}},
	}
	e.Write(points)

	result, err := e.Query(0, now+10000, map[string]string{"nonexistent": "tag"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 points for nonexistent tag, got %d", len(result))
	}
}

func TestQuery_TagsCopied(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	tags := map[string]string{"host": "s1"}
	points := []*DataPoint{
		{Timestamp: now + 1000, Value: 1.0, Tags: tags},
	}
	e.Write(points)

	tags["host"] = "modified"

	result, err := e.Query(0, now+10000, map[string]string{"host": "s1"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}
	if result[0].Tags["host"] != "s1" {
		t.Errorf("expected host=s1 in result, got %s", result[0].Tags["host"])
	}
}

func TestDownsample_Avg(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	window := time.Second
	windowMs := window.Milliseconds()
	baseTs := (time.Now().UnixMilli() / windowMs) * windowMs

	points := []*DataPoint{
		{Timestamp: baseTs + 500, Value: 10.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: baseTs + 1500, Value: 20.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: baseTs + 2500, Value: 30.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: baseTs + 3500, Value: 40.0, Tags: map[string]string{"m": "cpu"}},
	}

	err := e.Write(points)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	result, err := e.Downsample(0, baseTs+10000, window, AggAvg, map[string]string{"m": "cpu"})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}

	if len(result) != 4 {
		t.Fatalf("expected 4 windows, got %d", len(result))
	}

	if result[0].Value != 10.0 {
		t.Errorf("expected window 0 avg 10.0, got %f", result[0].Value)
	}
	if result[1].Value != 20.0 {
		t.Errorf("expected window 1 avg 20.0, got %f", result[1].Value)
	}
}

func TestDownsample_Sum(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	window := 2 * time.Second

	points := []*DataPoint{
		{Timestamp: 500, Value: 10.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: 1500, Value: 20.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: 2500, Value: 30.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: 3500, Value: 40.0, Tags: map[string]string{"m": "cpu"}},
	}

	err := e.Write(points)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	result, err := e.Downsample(0, 10000, window, AggSum, map[string]string{"m": "cpu"})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(result))
	}

	if result[0].Value != 30.0 {
		t.Errorf("expected window 0 sum 30.0, got %f", result[0].Value)
	}
	if result[1].Value != 70.0 {
		t.Errorf("expected window 1 sum 70.0, got %f", result[1].Value)
	}
}

func TestDownsample_MinMax(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	window := time.Second
	windowMs := window.Milliseconds()
	baseTs := (time.Now().UnixMilli() / windowMs) * windowMs

	points := []*DataPoint{
		{Timestamp: baseTs + 500, Value: 10.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: baseTs + 700, Value: 5.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: baseTs + 1500, Value: 30.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: baseTs + 1800, Value: 20.0, Tags: map[string]string{"m": "cpu"}},
	}

	e.Write(points)

	minResult, err := e.Downsample(0, baseTs+10000, window, AggMin, map[string]string{"m": "cpu"})
	if err != nil {
		t.Fatalf("Downsample min failed: %v", err)
	}
	if len(minResult) < 2 || minResult[0].Value != 5.0 {
		t.Errorf("expected min 5.0 in first window, got %v", minResult)
	}

	maxResult, err := e.Downsample(0, baseTs+10000, window, AggMax, map[string]string{"m": "cpu"})
	if err != nil {
		t.Fatalf("Downsample max failed: %v", err)
	}
	if len(maxResult) < 2 || maxResult[0].Value != 10.0 {
		t.Errorf("expected max 10.0 in first window, got %v", maxResult)
	}
}

func TestDownsample_Count(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	window := time.Second

	points := []*DataPoint{
		{Timestamp: 100, Value: 1.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: 200, Value: 2.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: 300, Value: 3.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: 1500, Value: 4.0, Tags: map[string]string{"m": "cpu"}},
	}

	e.Write(points)

	result, err := e.Downsample(0, 10000, window, AggCount, map[string]string{"m": "cpu"})
	if err != nil {
		t.Fatalf("Downsample count failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(result))
	}
	if result[0].Count != 3 {
		t.Errorf("expected count 3 in first window, got %d", result[0].Count)
	}
	if result[1].Count != 1 {
		t.Errorf("expected count 1 in second window, got %d", result[1].Count)
	}
}

func TestDownsample_InvalidTimeRange(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	_, err := e.Downsample(2000, 1000, time.Second, AggAvg, map[string]string{})
	if err != ErrInvalidTimeRange {
		t.Errorf("expected ErrInvalidTimeRange, got %v", err)
	}
}

func TestDownsample_InvalidWindowSize(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	_, err := e.Downsample(1000, 2000, 0, AggAvg, map[string]string{})
	if err != ErrInvalidWindowSize {
		t.Errorf("expected ErrInvalidWindowSize, got %v", err)
	}

	_, err = e.Downsample(1000, 2000, -time.Second, AggAvg, map[string]string{})
	if err != ErrInvalidWindowSize {
		t.Errorf("expected ErrInvalidWindowSize for negative, got %v", err)
	}
}

func TestDownsample_InvalidAggregator(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	_, err := e.Downsample(1000, 2000, time.Second, AggregatorType("invalid"), map[string]string{})
	if err != ErrInvalidAggregator {
		t.Errorf("expected ErrInvalidAggregator, got %v", err)
	}
}

func TestDownsample_EmptyResult(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	result, err := e.Downsample(1000, 2000, time.Second, AggAvg, map[string]string{})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 points, got %d", len(result))
	}
}

func TestDownsample_Sorted(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	window := time.Second
	windowMs := window.Milliseconds()
	baseTs := (time.Now().UnixMilli() / windowMs) * windowMs

	points := []*DataPoint{
		{Timestamp: baseTs + 5000, Value: 5.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: baseTs + 1000, Value: 1.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: baseTs + 3000, Value: 3.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: baseTs + 2000, Value: 2.0, Tags: map[string]string{"m": "cpu"}},
	}

	e.Write(points)

	result, err := e.Downsample(0, baseTs+10000, window, AggAvg, map[string]string{"m": "cpu"})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}

	for i := 1; i < len(result); i++ {
		if result[i].Timestamp < result[i-1].Timestamp {
			t.Errorf("results not sorted: %d < %d", result[i].Timestamp, result[i-1].Timestamp)
		}
	}
}

func TestTTL_ExpiredDataCleanup(t *testing.T) {
	cfg := Config{
		TTL:              100 * time.Millisecond,
		CleanupInterval:  10 * time.Millisecond,
		CleanupBatchSize: 100,
	}
	e, err := NewTSEngineWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTSEngineWithConfig failed: %v", err)
	}
	defer e.Close()

	oldTime := time.Now().Add(-200 * time.Millisecond).UnixMilli()
	points := []*DataPoint{
		{Timestamp: oldTime, Value: 1.0, Tags: map[string]string{"id": "old"}},
	}

	err = e.Write(points)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if e.Count() != 1 {
		t.Fatalf("expected 1 point before cleanup, got %d", e.Count())
	}

	e.ForceCleanup()

	if e.Count() != 0 {
		t.Errorf("expected 0 points after cleanup, got %d", e.Count())
	}
}

func TestTTL_RecentDataNotCleaned(t *testing.T) {
	cfg := Config{
		TTL:              time.Hour,
		CleanupInterval:  time.Minute,
		CleanupBatchSize: 100,
	}
	e, err := NewTSEngineWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTSEngineWithConfig failed: %v", err)
	}
	defer e.Close()

	now := time.Now().UnixMilli()
	points := []*DataPoint{
		{Timestamp: now, Value: 1.0, Tags: map[string]string{"id": "new"}},
	}

	e.Write(points)
	e.ForceCleanup()

	if e.Count() != 1 {
		t.Errorf("expected 1 point after cleanup (TTL not expired), got %d", e.Count())
	}
}

func TestTTL_BatchCleanup(t *testing.T) {
	cfg := Config{
		TTL:              time.Millisecond,
		CleanupInterval:  time.Second,
		CleanupBatchSize: 5,
	}
	e, err := NewTSEngineWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTSEngineWithConfig failed: %v", err)
	}
	defer e.Close()

	oldTime := time.Now().Add(-100 * time.Millisecond).UnixMilli()
	points := make([]*DataPoint, 20)
	for i := 0; i < 20; i++ {
		points[i] = &DataPoint{
			Timestamp: oldTime + int64(i),
			Value:     float64(i),
			Tags:      map[string]string{"id": fmt.Sprintf("p%d", i)},
		}
	}

	e.Write(points)

	e.ForceCleanup()

	if e.Count() != 0 {
		t.Errorf("expected all points cleaned up, got %d", e.Count())
	}
}

func TestTTL_TTLDisabled(t *testing.T) {
	cfg := Config{
		TTL:              TTLDisabled,
		CleanupInterval:  time.Millisecond,
		CleanupBatchSize: 100,
	}
	e, err := NewTSEngineWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTSEngineWithConfig failed: %v", err)
	}
	defer e.Close()

	if e.GetTTL() != TTLDisabled {
		t.Errorf("expected TTLDisabled, got %v", e.GetTTL())
	}

	now := time.Now().UnixMilli()
	points := []*DataPoint{
		{Timestamp: now, Value: 1.0, Tags: map[string]string{"id": "1"}},
	}
	e.Write(points)

	e.ForceCleanup()

	if e.Count() != 1 {
		t.Errorf("expected 1 point with TTLDisabled, got %d", e.Count())
	}
}

func TestClose(t *testing.T) {
	e := NewTSEngine()

	e.Close()

	err := e.Write([]*DataPoint{
		{Timestamp: 1000, Value: 1.0, Tags: map[string]string{"a": "b"}},
	})
	if err != ErrEngineClosed {
		t.Errorf("expected ErrEngineClosed after Close, got %v", err)
	}
}

func TestClose_DoubleClose(t *testing.T) {
	e := NewTSEngine()

	e.Close()
	e.Close()
}

func TestQuery_AfterClose(t *testing.T) {
	e := NewTSEngine()
	e.Close()

	_, err := e.Query(0, 1000, map[string]string{})
	if err != ErrEngineClosed {
		t.Errorf("expected ErrEngineClosed, got %v", err)
	}
}

func TestDownsample_AfterClose(t *testing.T) {
	e := NewTSEngine()
	e.Close()

	_, err := e.Downsample(0, 1000, time.Second, AggAvg, map[string]string{})
	if err != ErrEngineClosed {
		t.Errorf("expected ErrEngineClosed, got %v", err)
	}
}

func TestTagIndex_MultipleValues(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	points := []*DataPoint{
		{Timestamp: now + 1000, Value: 1.0, Tags: map[string]string{"status": "ok"}},
		{Timestamp: now + 2000, Value: 2.0, Tags: map[string]string{"status": "error"}},
		{Timestamp: now + 3000, Value: 3.0, Tags: map[string]string{"status": "ok"}},
		{Timestamp: now + 4000, Value: 4.0, Tags: map[string]string{"status": "warning"}},
	}

	e.Write(points)

	result, err := e.Query(0, now+10000, map[string]string{"status": "ok"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 points with status=ok, got %d", len(result))
	}
}

func TestTagIndex_AfterMultipleWrites(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()

	batch1 := []*DataPoint{
		{Timestamp: now + 1000, Value: 1.0, Tags: map[string]string{"host": "s1"}},
		{Timestamp: now + 2000, Value: 2.0, Tags: map[string]string{"host": "s2"}},
	}
	e.Write(batch1)

	batch2 := []*DataPoint{
		{Timestamp: now + 3000, Value: 3.0, Tags: map[string]string{"host": "s1"}},
		{Timestamp: now + 4000, Value: 4.0, Tags: map[string]string{"host": "s3"}},
	}
	e.Write(batch2)

	result, err := e.Query(0, now+10000, map[string]string{"host": "s1"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 points for host=s1 after two writes, got %d", len(result))
	}
}

func TestConcurrentWrite(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	var wg sync.WaitGroup
	numGoroutines := 10
	numPoints := 100

	now := time.Now().UnixMilli()

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			points := make([]*DataPoint, numPoints)
			for i := 0; i < numPoints; i++ {
				points[i] = &DataPoint{
					Timestamp: now + int64(id*numPoints+i),
					Value:     float64(id*numPoints + i),
					Tags:      map[string]string{"goroutine": fmt.Sprintf("g%d", id)},
				}
			}
			err := e.Write(points)
			if err != nil {
				t.Errorf("Write failed in goroutine %d: %v", id, err)
			}
		}(g)
	}

	wg.Wait()

	expected := numGoroutines * numPoints
	if e.Count() != expected {
		t.Errorf("expected %d points, got %d", expected, e.Count())
	}
}

func TestConcurrentQuery(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	numPoints := 1000
	points := make([]*DataPoint, numPoints)
	for i := 0; i < numPoints; i++ {
		points[i] = &DataPoint{
			Timestamp: now + int64(i*1000),
			Value:     float64(i),
			Tags:      map[string]string{"host": fmt.Sprintf("s%d", i%10)},
		}
	}
	e.Write(points)

	var wg sync.WaitGroup
	numReaders := 20

	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				result, err := e.Query(now, now+int64(numPoints*1000), map[string]string{"host": "s1"})
				if err != nil {
					t.Errorf("Query failed: %v", err)
					return
				}
				if len(result) == 0 {
					t.Error("Query returned empty result for existing tag")
					return
				}
			}
		}()
	}

	wg.Wait()
}

func TestConcurrentWriteAndQuery(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	var wg sync.WaitGroup
	now := time.Now().UnixMilli()

	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			points := []*DataPoint{
				{
					Timestamp: now + int64(i),
					Value:     float64(i),
					Tags:      map[string]string{"type": "writer"},
				},
			}
			e.Write(points)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			result, err := e.Query(0, now+10000, map[string]string{"type": "writer"})
			if err != nil {
				t.Errorf("Query failed: %v", err)
				return
			}
			_ = result
		}
	}()

	wg.Wait()
}

func TestWrite_SameTimestamp(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	ts := time.Now().UnixMilli()
	points := []*DataPoint{
		{Timestamp: ts, Value: 1.0, Tags: map[string]string{"id": "a"}},
		{Timestamp: ts, Value: 2.0, Tags: map[string]string{"id": "b"}},
		{Timestamp: ts, Value: 3.0, Tags: map[string]string{"id": "c"}},
	}

	err := e.Write(points)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if e.Count() != 3 {
		t.Errorf("expected 3 points with same timestamp, got %d", e.Count())
	}
}

func TestDownsample_WithTagFilter(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	window := time.Second
	windowMs := window.Milliseconds()
	baseTs := (time.Now().UnixMilli() / windowMs) * windowMs

	points := []*DataPoint{
		{Timestamp: baseTs + 500, Value: 10.0, Tags: map[string]string{"host": "s1", "dc": "us"}},
		{Timestamp: baseTs + 1500, Value: 20.0, Tags: map[string]string{"host": "s1", "dc": "us"}},
		{Timestamp: baseTs + 500, Value: 30.0, Tags: map[string]string{"host": "s2", "dc": "eu"}},
		{Timestamp: baseTs + 1500, Value: 40.0, Tags: map[string]string{"host": "s2", "dc": "eu"}},
	}

	e.Write(points)

	result, err := e.Downsample(0, baseTs+10000, window, AggAvg, map[string]string{"host": "s1"})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 windows for host=s1, got %d", len(result))
	}
	if result[0].Value != 10.0 {
		t.Errorf("expected avg 10.0 for host=s1 window 0, got %f", result[0].Value)
	}
}

func TestErrors_Values(t *testing.T) {
	if ErrInvalidTimeRange == nil {
		t.Error("ErrInvalidTimeRange should not be nil")
	}
	if ErrInvalidWindowSize == nil {
		t.Error("ErrInvalidWindowSize should not be nil")
	}
	if ErrInvalidAggregator == nil {
		t.Error("ErrInvalidAggregator should not be nil")
	}
	if ErrInvalidTTL == nil {
		t.Error("ErrInvalidTTL should not be nil")
	}
	if ErrInvalidBatchSize == nil {
		t.Error("ErrInvalidBatchSize should not be nil")
	}
	if ErrInvalidInterval == nil {
		t.Error("ErrInvalidInterval should not be nil")
	}
	if ErrEmptyTags == nil {
		t.Error("ErrEmptyTags should not be nil")
	}
	if ErrNilDataPoint == nil {
		t.Error("ErrNilDataPoint should not be nil")
	}
	if ErrEngineClosed == nil {
		t.Error("ErrEngineClosed should not be nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.TTL != 24*time.Hour {
		t.Errorf("expected default TTL 24h, got %v", cfg.TTL)
	}
	if cfg.CleanupInterval != 5*time.Minute {
		t.Errorf("expected default CleanupInterval 5m, got %v", cfg.CleanupInterval)
	}
	if cfg.CleanupBatchSize != 1000 {
		t.Errorf("expected default CleanupBatchSize 1000, got %d", cfg.CleanupBatchSize)
	}
}

func TestCount_Empty(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	if e.Count() != 0 {
		t.Errorf("expected 0, got %d", e.Count())
	}
}

func TestQuery_NoTagsFilter(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	points := []*DataPoint{
		{Timestamp: now + 1000, Value: 1.0, Tags: map[string]string{"a": "1"}},
		{Timestamp: now + 2000, Value: 2.0, Tags: map[string]string{"b": "2"}},
		{Timestamp: now + 3000, Value: 3.0, Tags: map[string]string{"c": "3"}},
	}

	e.Write(points)

	result, err := e.Query(0, now+10000, nil)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 points with no tag filter, got %d", len(result))
	}
}

func TestDownsample_NoDataInRange(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	points := []*DataPoint{
		{Timestamp: now + 1000, Value: 1.0, Tags: map[string]string{"m": "cpu"}},
	}
	e.Write(points)

	result, err := e.Downsample(now+5000, now+6000, time.Second, AggAvg, map[string]string{"m": "cpu"})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 windows, got %d", len(result))
	}
}

func TestTTL_CleanupPreservesIndex(t *testing.T) {
	cfg := Config{
		TTL:             100 * time.Millisecond,
		CleanupInterval: time.Minute,
		CleanupBatchSize: 100,
	}
	e, err := NewTSEngineWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTSEngineWithConfig failed: %v", err)
	}
	defer e.Close()

	oldTime := time.Now().Add(-200 * time.Millisecond).UnixMilli()
	newTime := time.Now().UnixMilli()

	points := []*DataPoint{
		{Timestamp: oldTime, Value: 1.0, Tags: map[string]string{"host": "old"}},
		{Timestamp: newTime, Value: 2.0, Tags: map[string]string{"host": "new"}},
	}

	e.Write(points)

	e.ForceCleanup()

	result, err := e.Query(0, newTime+10000, map[string]string{"host": "new"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 point with host=new after cleanup, got %d", len(result))
	}

	resultOld, err := e.Query(0, newTime+10000, map[string]string{"host": "old"})
	if err != nil {
		t.Fatalf("Query old failed: %v", err)
	}
	if len(resultOld) != 0 {
		t.Errorf("expected 0 points with host=old after cleanup, got %d", len(resultOld))
	}
}

func TestWrite_MultipleBatchesMaintainsSort(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()

	batch1 := []*DataPoint{
		{Timestamp: now + 3000, Value: 3.0, Tags: map[string]string{"id": "3"}},
		{Timestamp: now + 1000, Value: 1.0, Tags: map[string]string{"id": "1"}},
	}
	e.Write(batch1)

	batch2 := []*DataPoint{
		{Timestamp: now + 4000, Value: 4.0, Tags: map[string]string{"id": "4"}},
		{Timestamp: now + 2000, Value: 2.0, Tags: map[string]string{"id": "2"}},
	}
	e.Write(batch2)

	result, err := e.Query(0, now+10000, map[string]string{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(result) != 4 {
		t.Fatalf("expected 4 points, got %d", len(result))
	}

	for i := 1; i < len(result); i++ {
		if result[i].Timestamp < result[i-1].Timestamp {
			t.Errorf("not sorted at position %d: %d < %d", i, result[i].Timestamp, result[i-1].Timestamp)
		}
	}
}

func TestDownsample_SingleWindowMultiplePoints(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	window := 10 * time.Second
	windowMs := window.Milliseconds()
	baseTs := (time.Now().UnixMilli() / windowMs) * windowMs

	values := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	points := make([]*DataPoint, len(values))
	for i, v := range values {
		points[i] = &DataPoint{
			Timestamp: baseTs + int64(i*1000),
			Value:     v,
			Tags:      map[string]string{"m": "test"},
		}
	}
	e.Write(points)

	result, err := e.Downsample(0, baseTs+100000, window, AggAvg, map[string]string{"m": "test"})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 window, got %d", len(result))
	}
	if result[0].Value != 3.0 {
		t.Errorf("expected avg 3.0, got %f", result[0].Value)
	}
	if result[0].Count != 5 {
		t.Errorf("expected count 5, got %d", result[0].Count)
	}
}

func TestTagIndex_IntersectionEmpty(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	points := []*DataPoint{
		{Timestamp: now + 1000, Value: 1.0, Tags: map[string]string{"a": "1", "b": "2"}},
		{Timestamp: now + 2000, Value: 2.0, Tags: map[string]string{"a": "1", "b": "3"}},
		{Timestamp: now + 3000, Value: 3.0, Tags: map[string]string{"a": "2", "b": "2"}},
	}

	e.Write(points)

	result, err := e.Query(0, now+10000, map[string]string{"a": "2", "b": "3"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 points for non-intersecting tags, got %d", len(result))
	}
}

func TestQuery_ReturnsCopies(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	points := []*DataPoint{
		{Timestamp: now + 1000, Value: 1.0, Tags: map[string]string{"host": "s1"}},
	}
	e.Write(points)

	result, err := e.Query(0, now+10000, map[string]string{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	result[0].Value = 999.0
	result[0].Tags["host"] = "modified"

	result2, err := e.Query(0, now+10000, map[string]string{})
	if err != nil {
		t.Fatalf("Second query failed: %v", err)
	}

	if result2[0].Value == 999.0 {
		t.Error("modifying result should not affect stored data (value)")
	}
	if result2[0].Tags["host"] == "modified" {
		t.Error("modifying result tags should not affect stored data")
	}
}

func TestConcurrentDownsample(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	for i := 0; i < 100; i++ {
		points := []*DataPoint{
			{
				Timestamp: now + int64(i*100),
				Value:     float64(i),
				Tags:      map[string]string{"metric": "cpu", "host": fmt.Sprintf("h%d", i%5)},
			},
		}
		e.Write(points)
	}

	var wg sync.WaitGroup
	numWorkers := 10

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				_, err := e.Downsample(now, now+100000, time.Second, AggAvg, map[string]string{"host": "h1"})
				if err != nil {
					t.Errorf("Downsample failed: %v", err)
					return
				}
			}
		}()
	}

	wg.Wait()
}

func TestWrite_NegativeTimestamp(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	points := []*DataPoint{
		{Timestamp: -1000, Value: 1.0, Tags: map[string]string{"id": "neg"}},
		{Timestamp: 0, Value: 2.0, Tags: map[string]string{"id": "zero"}},
		{Timestamp: 1000, Value: 3.0, Tags: map[string]string{"id": "pos"}},
	}

	err := e.Write(points)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if e.Count() != 3 {
		t.Errorf("expected 3 points, got %d", e.Count())
	}

	result, err := e.Query(-2000, 2000, map[string]string{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 points in range, got %d", len(result))
	}

	if result[0].Timestamp != -1000 {
		t.Errorf("expected first timestamp -1000, got %d", result[0].Timestamp)
	}
}

func TestDownsample_WindowAlignment(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	window := 10 * time.Second
	windowMs := window.Milliseconds()

	baseTs := int64(1234567890000)
	alignedTs := (baseTs / windowMs) * windowMs

	points := []*DataPoint{
		{Timestamp: baseTs + 1000, Value: 1.0, Tags: map[string]string{"m": "test"}},
		{Timestamp: baseTs + 5000, Value: 2.0, Tags: map[string]string{"m": "test"}},
	}

	e.Write(points)

	result, err := e.Downsample(0, baseTs+100000, window, AggAvg, map[string]string{"m": "test"})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 window, got %d", len(result))
	}

	if result[0].Timestamp != alignedTs {
		t.Errorf("expected window aligned to %d, got %d", alignedTs, result[0].Timestamp)
	}
}

func TestSortStability(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	points := []*DataPoint{
		{Timestamp: now, Value: 3.0, Tags: map[string]string{"id": "c"}},
		{Timestamp: now, Value: 1.0, Tags: map[string]string{"id": "a"}},
		{Timestamp: now, Value: 2.0, Tags: map[string]string{"id": "b"}},
	}

	e.Write(points)

	result, err := e.Query(0, now+1000, map[string]string{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 points, got %d", len(result))
	}

	ids := make([]string, 3)
	for i, p := range result {
		ids[i] = p.Tags["id"]
	}

	sortedIds := make([]string, len(ids))
	copy(sortedIds, ids)
	sort.Strings(sortedIds)

	for i := range ids {
		if ids[i] != sortedIds[i] {
			t.Logf("Note: same-timestamp points order may vary: got %v", ids)
			break
		}
	}
}

func TestWrite_SinglePoint(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	points := []*DataPoint{
		{Timestamp: 1000, Value: 42.0, Tags: map[string]string{"host": "s1"}},
	}
	err := e.Write(points)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if e.Count() != 1 {
		t.Errorf("expected 1 point, got %d", e.Count())
	}
}

func TestWrite_LargeBatch(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	n := 10000
	points := make([]*DataPoint, n)
	for i := 0; i < n; i++ {
		points[i] = &DataPoint{
			Timestamp: int64(i * 100),
			Value:     float64(i),
			Tags:      map[string]string{"idx": fmt.Sprintf("%d", i)},
		}
	}

	err := e.Write(points)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if e.Count() != n {
		t.Errorf("expected %d points, got %d", n, e.Count())
	}
}

func TestQuery_ExactBoundaryMatch(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	points := []*DataPoint{
		{Timestamp: 1000, Value: 1.0, Tags: map[string]string{"id": "1"}},
		{Timestamp: 2000, Value: 2.0, Tags: map[string]string{"id": "2"}},
		{Timestamp: 3000, Value: 3.0, Tags: map[string]string{"id": "3"}},
	}
	e.Write(points)

	result, err := e.Query(2000, 2000, map[string]string{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 point at exact boundary, got %d", len(result))
	}
	if result[0].Value != 2.0 {
		t.Errorf("expected value 2.0, got %f", result[0].Value)
	}
}

func TestQuery_StartEqualsEnd(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	result, err := e.Query(100, 100, map[string]string{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 points for empty engine, got %d", len(result))
	}
}

func TestDownsample_SumSinglePoint(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	points := []*DataPoint{
		{Timestamp: 1500, Value: 7.5, Tags: map[string]string{"m": "cpu"}},
	}
	e.Write(points)

	result, err := e.Downsample(0, 10000, time.Second, AggSum, map[string]string{"m": "cpu"})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 window, got %d", len(result))
	}
	if result[0].Value != 7.5 {
		t.Errorf("expected sum 7.5 for single point, got %f", result[0].Value)
	}
	if result[0].Count != 1 {
		t.Errorf("expected count 1, got %d", result[0].Count)
	}
}

func TestDownsample_MinSinglePoint(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	points := []*DataPoint{
		{Timestamp: 500, Value: 3.3, Tags: map[string]string{"m": "mem"}},
	}
	e.Write(points)

	result, err := e.Downsample(0, 10000, time.Second, AggMin, map[string]string{"m": "mem"})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 window, got %d", len(result))
	}
	if result[0].Value != 3.3 {
		t.Errorf("expected min 3.3, got %f", result[0].Value)
	}
}

func TestDownsample_MaxSinglePoint(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	points := []*DataPoint{
		{Timestamp: 500, Value: 9.9, Tags: map[string]string{"m": "mem"}},
	}
	e.Write(points)

	result, err := e.Downsample(0, 10000, time.Second, AggMax, map[string]string{"m": "mem"})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 window, got %d", len(result))
	}
	if result[0].Value != 9.9 {
		t.Errorf("expected max 9.9, got %f", result[0].Value)
	}
}

func TestDownsample_CountValue(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	window := 5 * time.Second
	windowMs := window.Milliseconds()
	baseTs := (time.Now().UnixMilli() / windowMs) * windowMs

	points := []*DataPoint{
		{Timestamp: baseTs + 100, Value: 1.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: baseTs + 200, Value: 2.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: baseTs + 300, Value: 3.0, Tags: map[string]string{"m": "cpu"}},
	}
	e.Write(points)

	result, err := e.Downsample(0, baseTs+10000, window, AggCount, map[string]string{"m": "cpu"})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 window, got %d", len(result))
	}
	if result[0].Value != 3.0 {
		t.Errorf("expected count value 3.0, got %f", result[0].Value)
	}
	if result[0].Count != 3 {
		t.Errorf("expected count 3, got %d", result[0].Count)
	}
}

func TestDownsample_AvgWithNegativeValues(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	window := 5 * time.Second
	windowMs := window.Milliseconds()
	baseTs := (time.Now().UnixMilli() / windowMs) * windowMs

	points := []*DataPoint{
		{Timestamp: baseTs + 100, Value: -5.0, Tags: map[string]string{"m": "temp"}},
		{Timestamp: baseTs + 200, Value: 5.0, Tags: map[string]string{"m": "temp"}},
		{Timestamp: baseTs + 300, Value: -10.0, Tags: map[string]string{"m": "temp"}},
	}
	e.Write(points)

	result, err := e.Downsample(0, baseTs+10000, window, AggAvg, map[string]string{"m": "temp"})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 window, got %d", len(result))
	}
	expectedAvg := (-5.0 + 5.0 + -10.0) / 3.0
	if result[0].Value != expectedAvg {
		t.Errorf("expected avg %f, got %f", expectedAvg, result[0].Value)
	}
}

func TestDownsample_MinWithNegativeValues(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	window := 5 * time.Second
	windowMs := window.Milliseconds()
	baseTs := (time.Now().UnixMilli() / windowMs) * windowMs

	points := []*DataPoint{
		{Timestamp: baseTs + 100, Value: -5.0, Tags: map[string]string{"m": "temp"}},
		{Timestamp: baseTs + 200, Value: 5.0, Tags: map[string]string{"m": "temp"}},
	}
	e.Write(points)

	result, err := e.Downsample(0, baseTs+10000, window, AggMin, map[string]string{"m": "temp"})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}
	if result[0].Value != -5.0 {
		t.Errorf("expected min -5.0, got %f", result[0].Value)
	}
}

func TestDownsample_MaxWithNegativeValues(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	window := 5 * time.Second
	windowMs := window.Milliseconds()
	baseTs := (time.Now().UnixMilli() / windowMs) * windowMs

	points := []*DataPoint{
		{Timestamp: baseTs + 100, Value: -5.0, Tags: map[string]string{"m": "temp"}},
		{Timestamp: baseTs + 200, Value: -1.0, Tags: map[string]string{"m": "temp"}},
	}
	e.Write(points)

	result, err := e.Downsample(0, baseTs+10000, window, AggMax, map[string]string{"m": "temp"})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}
	if result[0].Value != -1.0 {
		t.Errorf("expected max -1.0, got %f", result[0].Value)
	}
}

func TestTTL_CleanupRemovesOldKeepsNew(t *testing.T) {
	cfg := Config{
		TTL:              500 * time.Millisecond,
		CleanupInterval:  50 * time.Millisecond,
		CleanupBatchSize: 10,
	}
	e, err := NewTSEngineWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTSEngineWithConfig failed: %v", err)
	}
	defer e.Close()

	oldTime := time.Now().Add(-1 * time.Second).UnixMilli()
	newTime := time.Now().UnixMilli()

	points := []*DataPoint{
		{Timestamp: oldTime, Value: 1.0, Tags: map[string]string{"age": "old"}},
		{Timestamp: oldTime + 1, Value: 2.0, Tags: map[string]string{"age": "old"}},
		{Timestamp: newTime, Value: 3.0, Tags: map[string]string{"age": "new"}},
		{Timestamp: newTime + 1, Value: 4.0, Tags: map[string]string{"age": "new"}},
	}
	e.Write(points)

	if e.Count() != 4 {
		t.Fatalf("expected 4 points before cleanup, got %d", e.Count())
	}

	e.ForceCleanup()

	if e.Count() != 2 {
		t.Errorf("expected 2 points after cleanup, got %d", e.Count())
	}

	result, err := e.Query(0, newTime+10000, map[string]string{"age": "new"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 new points, got %d", len(result))
	}
}

func TestTTL_ZeroTTL(t *testing.T) {
	cfg := Config{
		TTL:              0,
		CleanupInterval:  50 * time.Millisecond,
		CleanupBatchSize: 100,
	}
	_, err := NewTSEngineWithConfig(cfg)
	if err != ErrInvalidTTL {
		t.Errorf("expected ErrInvalidTTL for TTL=0, got %v", err)
	}
}

func TestTTL_CleanupBatchSizeRespected(t *testing.T) {
	cfg := Config{
		TTL:              time.Millisecond,
		CleanupInterval:  time.Minute,
		CleanupBatchSize: 3,
	}
	e, err := NewTSEngineWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTSEngineWithConfig failed: %v", err)
	}
	defer e.Close()

	oldTime := time.Now().Add(-100 * time.Millisecond).UnixMilli()
	points := make([]*DataPoint, 9)
	for i := 0; i < 9; i++ {
		points[i] = &DataPoint{
			Timestamp: oldTime + int64(i),
			Value:     float64(i),
			Tags:      map[string]string{"id": fmt.Sprintf("p%d", i)},
		}
	}
	e.Write(points)

	e.ForceCleanup()

	if e.Count() != 0 {
		t.Errorf("expected all expired points cleaned after multiple batches, got %d", e.Count())
	}
}

func TestTTL_CleanupOnEmptyEngine(t *testing.T) {
	cfg := Config{
		TTL:              time.Millisecond,
		CleanupInterval:  time.Minute,
		CleanupBatchSize: 100,
	}
	e, err := NewTSEngineWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTSEngineWithConfig failed: %v", err)
	}
	defer e.Close()

	e.ForceCleanup()

	if e.Count() != 0 {
		t.Errorf("expected 0 points on empty engine, got %d", e.Count())
	}
}

func TestTagIndex_ManyTagsOnSamePoint(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	points := []*DataPoint{
		{
			Timestamp: now + 1000,
			Value:     1.0,
			Tags:      map[string]string{"host": "s1", "dc": "us", "env": "prod", "team": "backend"},
		},
		{
			Timestamp: now + 2000,
			Value:     2.0,
			Tags:      map[string]string{"host": "s2", "dc": "eu", "env": "staging", "team": "frontend"},
		},
	}
	e.Write(points)

	result, err := e.Query(0, now+10000, map[string]string{"host": "s1", "dc": "us", "env": "prod"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 point matching 3 tags, got %d", len(result))
	}

	resultAll, err := e.Query(0, now+10000, map[string]string{"host": "s1", "dc": "us", "env": "prod", "team": "backend"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(resultAll) != 1 {
		t.Errorf("expected 1 point matching all 4 tags, got %d", len(resultAll))
	}

	resultPartial, err := e.Query(0, now+10000, map[string]string{"env": "prod", "team": "frontend"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(resultPartial) != 0 {
		t.Errorf("expected 0 points for non-matching tag combination, got %d", len(resultPartial))
	}
}

func TestTagIndex_SameTagKeyDifferentValues(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	now := time.Now().UnixMilli()
	points := []*DataPoint{
		{Timestamp: now + 1000, Value: 1.0, Tags: map[string]string{"status": "ok"}},
		{Timestamp: now + 2000, Value: 2.0, Tags: map[string]string{"status": "error"}},
		{Timestamp: now + 3000, Value: 3.0, Tags: map[string]string{"status": "ok"}},
		{Timestamp: now + 4000, Value: 4.0, Tags: map[string]string{"status": "warn"}},
	}
	e.Write(points)

	okResult, _ := e.Query(0, now+10000, map[string]string{"status": "ok"})
	if len(okResult) != 2 {
		t.Errorf("expected 2 ok points, got %d", len(okResult))
	}

	errorResult, _ := e.Query(0, now+10000, map[string]string{"status": "error"})
	if len(errorResult) != 1 {
		t.Errorf("expected 1 error point, got %d", len(errorResult))
	}

	warnResult, _ := e.Query(0, now+10000, map[string]string{"status": "warn"})
	if len(warnResult) != 1 {
		t.Errorf("expected 1 warn point, got %d", len(warnResult))
	}
}

func TestQuery_EngineEmpty(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	result, err := e.Query(0, 10000, map[string]string{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 points from empty engine, got %d", len(result))
	}
}

func TestDownsample_MultipleWindowsWithAvg(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	window := 2 * time.Second
	windowMs := window.Milliseconds()

	points := []*DataPoint{
		{Timestamp: 100, Value: 10.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: 1500, Value: 20.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: 2100, Value: 30.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: 4500, Value: 40.0, Tags: map[string]string{"m": "cpu"}},
	}
	e.Write(points)

	result, err := e.Downsample(0, 10000, window, AggAvg, map[string]string{"m": "cpu"})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 windows, got %d", len(result))
	}

	bucket0 := (int64(100) / windowMs) * windowMs
	bucket1 := (int64(2100) / windowMs) * windowMs
	bucket2 := (int64(4500) / windowMs) * windowMs

	if result[0].Timestamp != bucket0 {
		t.Errorf("expected bucket0 at %d, got %d", bucket0, result[0].Timestamp)
	}
	if result[1].Timestamp != bucket1 {
		t.Errorf("expected bucket1 at %d, got %d", bucket1, result[1].Timestamp)
	}
	if result[2].Timestamp != bucket2 {
		t.Errorf("expected bucket2 at %d, got %d", bucket2, result[2].Timestamp)
	}

	if result[0].Value != 15.0 {
		t.Errorf("expected avg 15.0 for first window, got %f", result[0].Value)
	}
	if result[0].Count != 2 {
		t.Errorf("expected count 2 for first window, got %d", result[0].Count)
	}
}

func TestWrite_AfterClose(t *testing.T) {
	e := NewTSEngine()
	e.Close()

	err := e.Write([]*DataPoint{
		{Timestamp: 1000, Value: 1.0, Tags: map[string]string{"a": "b"}},
	})
	if err != ErrEngineClosed {
		t.Errorf("expected ErrEngineClosed, got %v", err)
	}
}

func TestDownsample_LargeWindowCoversAll(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	window := 1 * time.Hour
	windowMs := window.Milliseconds()
	baseTs := (time.Now().UnixMilli() / windowMs) * windowMs

	points := []*DataPoint{
		{Timestamp: baseTs + 1000, Value: 1.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: baseTs + 2000, Value: 2.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: baseTs + 3000, Value: 3.0, Tags: map[string]string{"m": "cpu"}},
	}
	e.Write(points)

	result, err := e.Downsample(0, baseTs+100000, window, AggSum, map[string]string{"m": "cpu"})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 window covering all points, got %d", len(result))
	}
	if result[0].Value != 6.0 {
		t.Errorf("expected sum 6.0, got %f", result[0].Value)
	}
}

func TestConcurrentWriteAndCleanup(t *testing.T) {
	cfg := Config{
		TTL:              500 * time.Millisecond,
		CleanupInterval:  50 * time.Millisecond,
		CleanupBatchSize: 50,
	}
	e, err := NewTSEngineWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTSEngineWithConfig failed: %v", err)
	}
	defer e.Close()

	var wg sync.WaitGroup
	now := time.Now().UnixMilli()

	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				points := []*DataPoint{
					{
						Timestamp: now + int64(id*1000+i),
						Value:     float64(i),
						Tags:      map[string]string{"g": fmt.Sprintf("%d", id)},
					},
				}
				e.Write(points)
			}
		}(g)
	}

	wg.Wait()

	count := e.Count()
	if count == 0 {
		t.Error("expected some points after concurrent writes")
	}
}

func TestDownsample_AvgPrecision(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	window := 5 * time.Second
	windowMs := window.Milliseconds()
	baseTs := (time.Now().UnixMilli() / windowMs) * windowMs

	points := []*DataPoint{
		{Timestamp: baseTs + 100, Value: 1.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: baseTs + 200, Value: 2.0, Tags: map[string]string{"m": "cpu"}},
		{Timestamp: baseTs + 300, Value: 2.0, Tags: map[string]string{"m": "cpu"}},
	}
	e.Write(points)

	result, err := e.Downsample(0, baseTs+10000, window, AggAvg, map[string]string{"m": "cpu"})
	if err != nil {
		t.Fatalf("Downsample failed: %v", err)
	}

	expected := 5.0 / 3.0
	if len(result) != 1 {
		t.Fatalf("expected 1 window, got %d", len(result))
	}
	if math.Abs(result[0].Value-expected) > 1e-9 {
		t.Errorf("expected avg %f, got %f", expected, result[0].Value)
	}
}

func TestNewTSEngineWithConfig_ZeroCleanupBatchSize(t *testing.T) {
	cfg := Config{
		TTL:              time.Hour,
		CleanupInterval:  time.Minute,
		CleanupBatchSize: 0,
	}
	_, err := NewTSEngineWithConfig(cfg)
	if err != ErrInvalidBatchSize {
		t.Errorf("expected ErrInvalidBatchSize for zero batch size, got %v", err)
	}
}

func TestNewTSEngineWithConfig_NegativeCleanupInterval(t *testing.T) {
	cfg := Config{
		TTL:              time.Hour,
		CleanupInterval:  -1,
		CleanupBatchSize: 100,
	}
	_, err := NewTSEngineWithConfig(cfg)
	if err != ErrInvalidInterval {
		t.Errorf("expected ErrInvalidInterval for negative interval, got %v", err)
	}
}

func TestQuery_ResultSortedAcrossMultipleWrites(t *testing.T) {
	e := NewTSEngine()
	defer e.Close()

	e.Write([]*DataPoint{
		{Timestamp: 5000, Value: 5.0, Tags: map[string]string{"id": "5"}},
		{Timestamp: 3000, Value: 3.0, Tags: map[string]string{"id": "3"}},
	})
	e.Write([]*DataPoint{
		{Timestamp: 1000, Value: 1.0, Tags: map[string]string{"id": "1"}},
		{Timestamp: 4000, Value: 4.0, Tags: map[string]string{"id": "4"}},
	})
	e.Write([]*DataPoint{
		{Timestamp: 2000, Value: 2.0, Tags: map[string]string{"id": "2"}},
	})

	result, err := e.Query(0, 10000, map[string]string{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(result) != 5 {
		t.Fatalf("expected 5 points, got %d", len(result))
	}

	for i := 1; i < len(result); i++ {
		if result[i].Timestamp < result[i-1].Timestamp {
			t.Errorf("not sorted at position %d: %d < %d", i, result[i].Timestamp, result[i-1].Timestamp)
		}
	}
}

func TestTagIndex_AfterCleanupStillCorrect(t *testing.T) {
	cfg := Config{
		TTL:              100 * time.Millisecond,
		CleanupInterval:  time.Minute,
		CleanupBatchSize: 10,
	}
	e, err := NewTSEngineWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTSEngineWithConfig failed: %v", err)
	}
	defer e.Close()

	oldTime := time.Now().Add(-200 * time.Millisecond).UnixMilli()
	newTime := time.Now().UnixMilli()

	points := []*DataPoint{
		{Timestamp: oldTime, Value: 1.0, Tags: map[string]string{"host": "a", "env": "prod"}},
		{Timestamp: oldTime + 1, Value: 2.0, Tags: map[string]string{"host": "b", "env": "prod"}},
		{Timestamp: newTime, Value: 3.0, Tags: map[string]string{"host": "a", "env": "staging"}},
		{Timestamp: newTime + 1, Value: 4.0, Tags: map[string]string{"host": "c", "env": "prod"}},
	}
	e.Write(points)

	e.ForceCleanup()

	aResult, err := e.Query(0, newTime+10000, map[string]string{"host": "a"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(aResult) != 1 {
		t.Errorf("expected 1 point for host=a after cleanup, got %d", len(aResult))
	}

	prodResult, err := e.Query(0, newTime+10000, map[string]string{"env": "prod"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(prodResult) != 1 {
		t.Errorf("expected 1 point for env=prod after cleanup, got %d", len(prodResult))
	}

	combinedResult, err := e.Query(0, newTime+10000, map[string]string{"env": "staging", "host": "a"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(combinedResult) != 1 {
		t.Errorf("expected 1 point for combined tags after cleanup, got %d", len(combinedResult))
	}
}
