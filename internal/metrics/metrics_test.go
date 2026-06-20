package metrics

import (
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestCounter_Basic(t *testing.T) {
	r := NewRegistry()
	c := r.RegisterCounter("test_counter", nil)

	if c.Value() != 0 {
		t.Errorf("expected initial value 0, got %v", c.Value())
	}

	c.Inc()
	if c.Value() != 1 {
		t.Errorf("expected value 1 after Inc(), got %v", c.Value())
	}

	c.Add(5)
	if c.Value() != 6 {
		t.Errorf("expected value 6 after Add(5), got %v", c.Value())
	}

	c.Reset()
	if c.Value() != 0 {
		t.Errorf("expected value 0 after Reset(), got %v", c.Value())
	}
}

func TestCounter_NegativeAdd(t *testing.T) {
	r := NewRegistry()
	c := r.RegisterCounter("test_counter_neg", nil)

	c.Add(10)
	c.Add(-5)
	if c.Value() != 10 {
		t.Errorf("counter should ignore negative adds, expected 10, got %v", c.Value())
	}
}

func TestCounter_Labels(t *testing.T) {
	r := NewRegistry()
	labels1 := Labels{{Name: "method", Value: "GET"}}
	labels2 := Labels{{Name: "method", Value: "POST"}}

	c1 := r.RegisterCounter("http_requests_total", labels1)
	c2 := r.RegisterCounter("http_requests_total", labels2)

	c1.Inc()
	c1.Inc()
	c2.Inc()

	if c1.Value() != 2 {
		t.Errorf("expected c1 value 2, got %v", c1.Value())
	}
	if c2.Value() != 1 {
		t.Errorf("expected c2 value 1, got %v", c2.Value())
	}

	if got, ok := r.GetCounter("http_requests_total", labels1); ok {
		if got.Value() != 2 {
			t.Errorf("GetCounter returned wrong value, expected 2, got %v", got.Value())
		}
	} else {
		t.Error("GetCounter failed to find counter with labels1")
	}
}

func TestGauge_Basic(t *testing.T) {
	r := NewRegistry()
	g := r.RegisterGauge("test_gauge", nil)

	if g.Value() != 0 {
		t.Errorf("expected initial value 0, got %v", g.Value())
	}

	g.Set(42)
	if g.Value() != 42 {
		t.Errorf("expected value 42 after Set(42), got %v", g.Value())
	}

	g.Inc()
	if g.Value() != 43 {
		t.Errorf("expected value 43 after Inc(), got %v", g.Value())
	}

	g.Dec()
	if g.Value() != 42 {
		t.Errorf("expected value 42 after Dec(), got %v", g.Value())
	}

	g.Add(8)
	if g.Value() != 50 {
		t.Errorf("expected value 50 after Add(8), got %v", g.Value())
	}

	g.Sub(20)
	if g.Value() != 30 {
		t.Errorf("expected value 30 after Sub(20), got %v", g.Value())
	}
}

func TestGauge_NegativeValue(t *testing.T) {
	r := NewRegistry()
	g := r.RegisterGauge("test_gauge_neg", nil)

	g.Set(-10)
	if g.Value() != -10 {
		t.Errorf("expected value -10, got %v", g.Value())
	}

	g.Sub(5)
	if g.Value() != -15 {
		t.Errorf("expected value -15 after Sub(5), got %v", g.Value())
	}
}

func TestHistogram_Basic(t *testing.T) {
	r := NewRegistry()
	buckets := []float64{1, 5, 10}
	h := r.RegisterHistogram("test_histogram", nil, buckets)

	h.Observe(0.5)
	h.Observe(3)
	h.Observe(7)
	h.Observe(15)

	if h.Count() != 4 {
		t.Errorf("expected count 4, got %d", h.Count())
	}

	expectedSum := 0.5 + 3 + 7 + 15
	if math.Abs(h.Sum()-expectedSum) > 0.0001 {
		t.Errorf("expected sum %v, got %v", expectedSum, h.Sum())
	}

	bucketValues := h.Buckets()
	expectedBuckets := []struct {
		upper float64
		count uint64
	}{
		{1, 1},
		{5, 2},
		{10, 3},
		{math.Inf(1), 4},
	}

	if len(bucketValues) != len(expectedBuckets) {
		t.Fatalf("expected %d buckets, got %d", len(expectedBuckets), len(bucketValues))
	}

	for i, expected := range expectedBuckets {
		if bucketValues[i].Count != expected.count {
			t.Errorf("bucket %d: expected count %d, got %d", i, expected.count, bucketValues[i].Count)
		}
		if math.Abs(bucketValues[i].UpperBound-expected.upper) > 0.0001 {
			t.Errorf("bucket %d: expected upper bound %v, got %v", i, expected.upper, bucketValues[i].UpperBound)
		}
	}
}

func TestHistogram_Empty(t *testing.T) {
	r := NewRegistry()
	buckets := []float64{1, 2, 3}
	h := r.RegisterHistogram("test_histogram_empty", nil, buckets)

	if h.Count() != 0 {
		t.Errorf("expected count 0, got %d", h.Count())
	}
	if h.Sum() != 0 {
		t.Errorf("expected sum 0, got %v", h.Sum())
	}

	bucketValues := h.Buckets()
	for _, b := range bucketValues {
		if b.Count != 0 {
			t.Errorf("expected all buckets 0, got %d for bound %v", b.Count, b.UpperBound)
		}
	}
}

func TestHistogram_DefaultBuckets(t *testing.T) {
	buckets := DefaultBuckets()
	if len(buckets) != 11 {
		t.Errorf("expected 11 default buckets, got %d", len(buckets))
	}
}

func TestHistogram_ExponentialBuckets(t *testing.T) {
	buckets := ExponentialBuckets(1, 2, 4)
	expected := []float64{1, 2, 4, 8}
	if len(buckets) != len(expected) {
		t.Fatalf("expected %d buckets, got %d", len(expected), len(buckets))
	}
	for i, v := range expected {
		if buckets[i] != v {
			t.Errorf("bucket %d: expected %v, got %v", i, v, buckets[i])
		}
	}
}

func TestHistogram_LinearBuckets(t *testing.T) {
	buckets := LinearBuckets(0, 5, 4)
	expected := []float64{0, 5, 10, 15}
	if len(buckets) != len(expected) {
		t.Fatalf("expected %d buckets, got %d", len(expected), len(buckets))
	}
	for i, v := range expected {
		if buckets[i] != v {
			t.Errorf("bucket %d: expected %v, got %v", i, v, buckets[i])
		}
	}
}

func TestSummary_Basic(t *testing.T) {
	r := NewRegistry()
	quantiles := []float64{0.5, 0.9, 0.99}
	s := r.RegisterSummary("test_summary", nil, quantiles)

	for i := 1; i <= 100; i++ {
		s.Observe(float64(i))
	}

	if s.Count() != 100 {
		t.Errorf("expected count 100, got %d", s.Count())
	}

	expectedSum := 5050.0
	if math.Abs(s.Sum()-expectedSum) > 0.0001 {
		t.Errorf("expected sum %v, got %v", expectedSum, s.Sum())
	}

	qValues := s.Quantiles()
	if len(qValues) != 3 {
		t.Fatalf("expected 3 quantiles, got %d", len(qValues))
	}

	if qValues[0].Quantile != 0.5 {
		t.Errorf("expected quantile 0.5, got %v", qValues[0].Quantile)
	}
	if qValues[0].Value < 45 || qValues[0].Value > 55 {
		t.Errorf("expected P50 around 50, got %v", qValues[0].Value)
	}
}

func TestSummary_Empty(t *testing.T) {
	r := NewRegistry()
	quantiles := []float64{0.5, 0.9}
	s := r.RegisterSummary("test_summary_empty", nil, quantiles)

	if s.Count() != 0 {
		t.Errorf("expected count 0, got %d", s.Count())
	}
	if s.Sum() != 0 {
		t.Errorf("expected sum 0, got %v", s.Sum())
	}

	qValues := s.Quantiles()
	for _, q := range qValues {
		if q.Value != 0 {
			t.Errorf("expected quantile value 0 for empty summary, got %v", q.Value)
		}
	}
}

func TestSummary_DefaultQuantiles(t *testing.T) {
	q := DefaultQuantiles()
	expected := []float64{0.5, 0.9, 0.99}
	if len(q) != len(expected) {
		t.Fatalf("expected %d default quantiles, got %d", len(expected), len(q))
	}
	for i, v := range expected {
		if q[i] != v {
			t.Errorf("quantile %d: expected %v, got %v", i, v, q[i])
		}
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	r.RegisterCounter("test_dup", nil)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for duplicate counter registration")
		}
	}()

	r.RegisterCounter("test_dup", nil)
}

func TestRegistry_GetNonExistent(t *testing.T) {
	r := NewRegistry()

	if _, ok := r.GetCounter("nonexistent", nil); ok {
		t.Error("expected GetCounter to return false for non-existent metric")
	}
	if _, ok := r.GetGauge("nonexistent", nil); ok {
		t.Error("expected GetGauge to return false for non-existent metric")
	}
	if _, ok := r.GetHistogram("nonexistent", nil); ok {
		t.Error("expected GetHistogram to return false for non-existent metric")
	}
	if _, ok := r.GetSummary("nonexistent", nil); ok {
		t.Error("expected GetSummary to return false for non-existent metric")
	}
}

func TestRegistry_InvalidMetricName(t *testing.T) {
	r := NewRegistry()

	testCases := []string{
		"",
		"123invalid",
		"invalid-name",
	}

	for _, name := range testCases {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("expected panic for invalid metric name: %s", name)
				}
			}()
			r.RegisterCounter(name, nil)
		}()
	}
}

func TestRegistry_InvalidLabelName(t *testing.T) {
	r := NewRegistry()

	testCases := []Labels{
		{{Name: "__reserved", Value: "val"}},
		{{Name: "123invalid", Value: "val"}},
		{{Name: "invalid-name", Value: "val"}},
	}

	for _, labels := range testCases {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("expected panic for invalid label name: %v", labels)
				}
			}()
			r.RegisterCounter("test_label", labels)
		}()
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := NewRegistry()
	labels := Labels{{Name: "key", Value: "val"}}

	r.RegisterCounter("test_unreg", labels)
	if !r.Unregister("test_unreg", labels) {
		t.Error("expected Unregister to return true")
	}
	if _, ok := r.GetCounter("test_unreg", labels); ok {
		t.Error("expected counter to be unregistered")
	}

	if r.Unregister("nonexistent", nil) {
		t.Error("expected Unregister to return false for non-existent")
	}
}

func TestRegistry_EmptyBuckets(t *testing.T) {
	r := NewRegistry()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for empty buckets")
		}
	}()

	r.RegisterHistogram("test_empty_buckets", nil, []float64{})
}

func TestRegistry_InvalidQuantile(t *testing.T) {
	r := NewRegistry()

	testCases := [][]float64{
		{-0.1},
		{1.1},
	}

	for _, qs := range testCases {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("expected panic for invalid quantiles: %v", qs)
				}
			}()
			r.RegisterSummary("test_inv_q", nil, qs)
		}()
	}
}

func TestSnapshot_Basic(t *testing.T) {
	r := NewRegistry()

	c := r.RegisterCounter("snap_counter", Labels{{Name: "env", Value: "prod"}})
	c.Inc()
	c.Inc()

	g := r.RegisterGauge("snap_gauge", nil)
	g.Set(100)

	snapshot := r.Snapshot()

	if len(snapshot) != 2 {
		t.Fatalf("expected 2 metrics in snapshot, got %d", len(snapshot))
	}

	foundCounter := false
	foundGauge := false

	for _, mv := range snapshot {
		switch mv.Name {
		case "snap_counter":
			foundCounter = true
			if mv.Type != CounterType {
				t.Errorf("expected counter type, got %v", mv.Type)
			}
			if mv.Value != 2 {
				t.Errorf("expected counter value 2, got %v", mv.Value)
			}
			if len(mv.Labels) != 1 {
				t.Errorf("expected 1 label, got %d", len(mv.Labels))
			}
		case "snap_gauge":
			foundGauge = true
			if mv.Type != GaugeType {
				t.Errorf("expected gauge type, got %v", mv.Type)
			}
			if mv.Value != 100 {
				t.Errorf("expected gauge value 100, got %v", mv.Value)
			}
		}
	}

	if !foundCounter {
		t.Error("counter not found in snapshot")
	}
	if !foundGauge {
		t.Error("gauge not found in snapshot")
	}
}

func TestSnapshot_Histogram(t *testing.T) {
	r := NewRegistry()
	h := r.RegisterHistogram("snap_hist", nil, []float64{1, 5})
	h.Observe(2)
	h.Observe(3)
	h.Observe(10)

	snapshot := r.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(snapshot))
	}

	mv := snapshot[0]
	if len(mv.Buckets) != 3 {
		t.Errorf("expected 3 buckets (including +Inf), got %d", len(mv.Buckets))
	}
	if mv.Count != 3 {
		t.Errorf("expected count 3, got %d", mv.Count)
	}
	if mv.Sum != 15 {
		t.Errorf("expected sum 15, got %v", mv.Sum)
	}
}

func TestSnapshot_Summary(t *testing.T) {
	r := NewRegistry()
	s := r.RegisterSummary("snap_summary", nil, []float64{0.5, 0.9})
	s.Observe(1)
	s.Observe(2)
	s.Observe(3)

	snapshot := r.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(snapshot))
	}

	mv := snapshot[0]
	if len(mv.Quantiles) != 2 {
		t.Errorf("expected 2 quantiles, got %d", len(mv.Quantiles))
	}
	if mv.Count != 3 {
		t.Errorf("expected count 3, got %d", mv.Count)
	}
	if mv.Sum != 6 {
		t.Errorf("expected sum 6, got %v", mv.Sum)
	}
}

func TestPrometheusFormat_Counter(t *testing.T) {
	r := NewRegistry()
	labels := Labels{{Name: "method", Value: "GET"}}
	c := r.RegisterCounter("http_requests", labels)
	c.Add(42)

	output := string(r.PrometheusFormat())

	if !strings.Contains(output, "http_requests_total") {
		t.Error("expected _total suffix for counter")
	}
	if !strings.Contains(output, "method=\"GET\"") {
		t.Error("expected label in output")
	}
	if !strings.Contains(output, " 42") {
		t.Errorf("expected value 42 in output, got: %s", output)
	}
	if !strings.Contains(output, "# TYPE http_requests_total counter") {
		t.Error("expected TYPE line for counter")
	}
}

func TestPrometheusFormat_Gauge(t *testing.T) {
	r := NewRegistry()
	g := r.RegisterGauge("memory_usage", nil)
	g.Set(1024.5)

	output := string(r.PrometheusFormat())

	if !strings.Contains(output, "memory_usage ") {
		t.Error("expected gauge metric name")
	}
	if !strings.Contains(output, "# TYPE memory_usage gauge") {
		t.Error("expected TYPE line for gauge")
	}
}

func TestPrometheusFormat_Histogram(t *testing.T) {
	r := NewRegistry()
	h := r.RegisterHistogram("request_duration", nil, []float64{0.1, 0.5, 1})
	h.Observe(0.05)
	h.Observe(0.3)
	h.Observe(2)

	output := string(r.PrometheusFormat())

	if !strings.Contains(output, "request_duration_bucket") {
		t.Error("expected _bucket suffix for histogram buckets")
	}
	if !strings.Contains(output, "request_duration_sum") {
		t.Error("expected _sum suffix for histogram")
	}
	if !strings.Contains(output, "request_duration_count") {
		t.Error("expected _count suffix for histogram")
	}
	if !strings.Contains(output, "le=\"+Inf\"") {
		t.Error("expected +Inf bucket")
	}
	if !strings.Contains(output, "# TYPE request_duration histogram") {
		t.Error("expected TYPE line for histogram")
	}
}

func TestPrometheusFormat_Summary(t *testing.T) {
	r := NewRegistry()
	s := r.RegisterSummary("latency", nil, []float64{0.5, 0.99})
	s.Observe(10)
	s.Observe(20)
	s.Observe(30)

	output := string(r.PrometheusFormat())

	if !strings.Contains(output, "latency_sum") {
		t.Error("expected _sum suffix for summary")
	}
	if !strings.Contains(output, "latency_count") {
		t.Error("expected _count suffix for summary")
	}
	if !strings.Contains(output, "quantile=") {
		t.Error("expected quantile label")
	}
	if !strings.Contains(output, "# TYPE latency summary") {
		t.Error("expected TYPE line for summary")
	}
}

func TestLabels_Hash(t *testing.T) {
	l1 := Labels{{Name: "a", Value: "1"}, {Name: "b", Value: "2"}}
	l2 := Labels{{Name: "a", Value: "1"}, {Name: "b", Value: "2"}}
	l3 := Labels{{Name: "b", Value: "2"}, {Name: "a", Value: "1"}}

	if l1.Hash() != l2.Hash() {
		t.Error("same labels should have same hash")
	}

	if l1.Hash() == l3.Hash() {
		t.Log("Note: label order affects hash (consistent with Prometheus)")
	}
}

func TestLabels_EmptyHash(t *testing.T) {
	var l Labels
	if l.Hash() != "" {
		t.Errorf("expected empty hash for empty labels, got %q", l.Hash())
	}
}

func TestCounter_Concurrent(t *testing.T) {
	r := NewRegistry()
	c := r.RegisterCounter("concurrent_counter", nil)

	var wg sync.WaitGroup
	numGoroutines := 10
	iterations := 1000

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				c.Inc()
			}
		}()
	}

	wg.Wait()

	expected := uint64(numGoroutines * iterations)
	if uint64(c.Value()) != expected {
		t.Errorf("expected %d, got %v", expected, c.Value())
	}
}

func TestGauge_Concurrent(t *testing.T) {
	r := NewRegistry()
	g := r.RegisterGauge("concurrent_gauge", nil)

	var wg sync.WaitGroup
	numGoroutines := 10
	iterations := 1000

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				g.Inc()
			}
		}()
	}

	wg.Wait()

	expected := float64(numGoroutines * iterations)
	if g.Value() != expected {
		t.Errorf("expected %v, got %v", expected, g.Value())
	}
}

func TestHistogram_Concurrent(t *testing.T) {
	r := NewRegistry()
	h := r.RegisterHistogram("concurrent_hist", nil, []float64{1, 10, 100})

	var wg sync.WaitGroup
	numGoroutines := 10
	iterations := 100

	var totalSum float64
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			localSum := 0.0
			for j := 0; j < iterations; j++ {
				val := float64(j%50 + 1)
				h.Observe(val)
				localSum += val
			}
			mu.Lock()
			totalSum += localSum
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	expectedCount := uint64(numGoroutines * iterations)
	if h.Count() != expectedCount {
		t.Errorf("expected count %d, got %d", expectedCount, h.Count())
	}

	if math.Abs(h.Sum()-totalSum) > 0.0001 {
		t.Errorf("expected sum %v, got %v", totalSum, h.Sum())
	}
}

func TestSummary_Concurrent(t *testing.T) {
	r := NewRegistry()
	s := r.RegisterSummary("concurrent_summary", nil, []float64{0.5})

	var wg sync.WaitGroup
	numGoroutines := 10
	iterations := 100

	var totalCount int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				s.Observe(float64(j))
				atomic.AddInt64(&totalCount, 1)
			}
		}()
	}

	wg.Wait()

	if s.Count() != uint64(totalCount) {
		t.Errorf("expected count %d, got %d", totalCount, s.Count())
	}
}

func TestMetricValue_Types(t *testing.T) {
	var snapshotMu sync.RWMutex
	guard := newSnapshotGuard(&snapshotMu)

	c := newCounter("test", nil, guard)
	if c.Type() != CounterType {
		t.Errorf("expected counter type, got %v", c.Type())
	}

	g := newGauge("test", nil, guard)
	if g.Type() != GaugeType {
		t.Errorf("expected gauge type, got %v", g.Type())
	}

	h := newHistogram("test", nil, []float64{1}, guard)
	if h.Type() != HistogramType {
		t.Errorf("expected histogram type, got %v", h.Type())
	}

	s := newSummary("test", nil, []float64{0.5}, guard)
	if s.Type() != SummaryType {
		t.Errorf("expected summary type, got %v", s.Type())
	}
}

func TestDefaultRegistry(t *testing.T) {
	c := RegisterCounter("default_reg_test", nil)
	c.Inc()

	if c.Value() != 1 {
		t.Errorf("expected value 1 on default registry, got %v", c.Value())
	}

	if _, ok := GetCounter("default_reg_test", nil); !ok {
		t.Error("expected to find counter in default registry")
	}

	if !Unregister("default_reg_test", nil) {
		t.Error("expected Unregister to succeed")
	}
}

func TestSnapshot_Atomic(t *testing.T) {
	r := NewRegistry()

	c1 := r.RegisterCounter("atomic_c1", nil)
	c2 := r.RegisterCounter("atomic_c2", nil)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				c1.Inc()
				c2.Inc()
			}
		}
	}()

	maxDiff := 0.0
	for i := 0; i < 100; i++ {
		snapshot := r.Snapshot()
		var v1, v2 float64
		for _, mv := range snapshot {
			if mv.Name == "atomic_c1" {
				v1 = mv.Value
			}
			if mv.Name == "atomic_c2" {
				v2 = mv.Value
			}
		}
		diff := math.Abs(v1 - v2)
		if diff > maxDiff {
			maxDiff = diff
		}
		if diff > 1 {
			t.Errorf("snapshot values differ by %v, expected at most 1 (atomic snapshot)", diff)
		}
	}

	close(stop)
	wg.Wait()

	if maxDiff > 1 {
		t.Logf("max difference observed: %v", maxDiff)
	}
}

func TestHistogram_BucketBoundary(t *testing.T) {
	r := NewRegistry()
	h := r.RegisterHistogram("boundary_test", nil, []float64{10, 20})

	h.Observe(10)
	buckets := h.Buckets()

	if buckets[0].Count != 1 {
		t.Errorf("value equal to bucket bound should be in that bucket, expected 1, got %d", buckets[0].Count)
	}

	h.Observe(20)
	buckets = h.Buckets()
	if buckets[1].Count != 2 {
		t.Errorf("expected second bucket count 2, got %d", buckets[1].Count)
	}
}

func TestEscapeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{`hello "world"`, `hello \"world\"`},
		{"hello\\world", "hello\\\\world"},
		{"line1\nline2", "line1\\nline2"},
	}

	for _, tt := range tests {
		result := escapeString(tt.input)
		if result != tt.expected {
			t.Errorf("escapeString(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatLabels(t *testing.T) {
	labels := Labels{
		{Name: "method", Value: "GET"},
		{Name: "path", Value: "/api/users"},
	}

	result := formatLabels(labels)
	if !strings.HasPrefix(result, "{") || !strings.HasSuffix(result, "}") {
		t.Errorf("expected labels to be wrapped in {}, got %s", result)
	}
	if !strings.Contains(result, `method="GET"`) {
		t.Errorf("expected method label, got %s", result)
	}
}

func TestQuantile(t *testing.T) {
	sorted := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	tests := []struct {
		q     float64
		want  float64
	}{
		{0, 1},
		{1, 10},
		{0.5, 5.5},
		{0.9, 9.1},
	}

	for _, tt := range tests {
		result := quantile(sorted, tt.q)
		if math.Abs(result-tt.want) > 0.0001 {
			t.Errorf("quantile(%v) = %v, want %v", tt.q, result, tt.want)
		}
	}
}

func TestQuantile_EmptySlice(t *testing.T) {
	result := quantile([]float64{}, 0.5)
	if result != 0 {
		t.Errorf("expected 0 for empty slice, got %v", result)
	}
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if len(r.Snapshot()) != 0 {
		t.Error("expected empty snapshot on new registry")
	}
}

func TestPrometheusFormat_Empty(t *testing.T) {
	r := NewRegistry()
	output := r.PrometheusFormat()
	if len(output) != 0 {
		t.Errorf("expected empty output for empty registry, got %d bytes", len(output))
	}
}

func TestSummary_ReservoirSampling_SmallSample(t *testing.T) {
	r := NewRegistry()
	s := r.RegisterSummary("test_small_sample", nil, []float64{0.5})

	for i := 1; i <= 100; i++ {
		s.Observe(float64(i))
	}

	if s.Count() != 100 {
		t.Errorf("expected count 100, got %d", s.Count())
	}

	qs := s.Quantiles()
	if len(qs) != 1 {
		t.Fatalf("expected 1 quantile, got %d", len(qs))
	}

	if qs[0].Value < 40 || qs[0].Value > 60 {
		t.Errorf("expected P50 around 50 (with small sample all data retained), got %v", qs[0].Value)
	}
}

func TestSummary_ReservoirSampling_LargeSample(t *testing.T) {
	r := NewRegistry()
	s := r.RegisterSummary("test_large_sample", nil, []float64{0.5, 0.9, 0.99})

	for i := 1; i <= 10000; i++ {
		s.Observe(float64(i))
	}

	if s.Count() != 10000 {
		t.Errorf("expected count 10000, got %d", s.Count())
	}

	expectedSum := float64(10000 * 10001 / 2)
	if math.Abs(s.Sum()-expectedSum) > 0.0001 {
		t.Errorf("expected sum %v, got %v", expectedSum, s.Sum())
	}

	qs := s.Quantiles()

	if qs[0].Value < 4000 || qs[0].Value > 6000 {
		t.Errorf("P50 out of expected range [4000, 6000], got %v", qs[0].Value)
	}

	if qs[1].Value < 8000 || qs[1].Value > 10000 {
		t.Errorf("P90 out of expected range [8000, 10000], got %v", qs[1].Value)
	}

	if qs[2].Value < 9000 || qs[2].Value > 10000 {
		t.Errorf("P99 out of expected range [9000, 10000], got %v", qs[2].Value)
	}
}

func TestSummary_ReservoirSampling_UniformDistribution(t *testing.T) {
	r := NewRegistry()
	s := r.RegisterSummary("test_uniform", nil, []float64{0.25, 0.5, 0.75})

	n := 50000
	for i := 0; i < n; i++ {
		s.Observe(float64(i) / float64(n))
	}

	if s.Count() != uint64(n) {
		t.Errorf("expected count %d, got %d", n, s.Count())
	}

	qs := s.Quantiles()

	expected := []float64{0.25, 0.5, 0.75}
	tolerance := 0.05

	for i, q := range qs {
		if math.Abs(q.Value-expected[i]) > tolerance {
			t.Errorf("P%.0f: expected ~%v, got %v (tolerance %v)",
				q.Quantile*100, expected[i], q.Value, tolerance)
		}
	}
}

func TestSummary_ConcurrentReservoirSampling(t *testing.T) {
	r := NewRegistry()
	s := r.RegisterSummary("concurrent_reservoir", nil, []float64{0.5, 0.9, 0.99})

	var wg sync.WaitGroup
	numGoroutines := 10
	iterations := 1000

	var totalCount int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				s.Observe(float64(id*iterations + j))
				atomic.AddInt64(&totalCount, 1)
			}
		}(i)
	}

	wg.Wait()

	if s.Count() != uint64(totalCount) {
		t.Errorf("expected count %d, got %d", totalCount, s.Count())
	}

	qs := s.Quantiles()
	if len(qs) != 3 {
		t.Fatalf("expected 3 quantiles, got %d", len(qs))
	}

	expectedSum := float64(numGoroutines*iterations) * float64(numGoroutines*iterations-1) / 2
	if math.Abs(s.Sum()-expectedSum) > 0.0001 {
		t.Errorf("expected sum %v, got %v", expectedSum, s.Sum())
	}

	if qs[0].Quantile != 0.5 {
		t.Errorf("expected quantile 0.5, got %v", qs[0].Quantile)
	}
	if qs[0].Value < 4000 || qs[0].Value > 6000 {
		t.Errorf("P50 out of expected range [4000, 6000], got %v", qs[0].Value)
	}

	if qs[1].Quantile != 0.9 {
		t.Errorf("expected quantile 0.9, got %v", qs[1].Quantile)
	}
	if qs[1].Value < 8000 || qs[1].Value > 10000 {
		t.Errorf("P90 out of expected range [8000, 10000], got %v", qs[1].Value)
	}

	if qs[2].Quantile != 0.99 {
		t.Errorf("expected quantile 0.99, got %v", qs[2].Quantile)
	}
	if qs[2].Value < 9000 || qs[2].Value > 10000 {
		t.Errorf("P99 out of expected range [9000, 10000], got %v", qs[2].Value)
	}
}

func TestRegistry_SameNameDifferentTypes(t *testing.T) {
	r := NewRegistry()

	c := r.RegisterCounter("same_name", nil)
	g := r.RegisterGauge("same_name", nil)
	h := r.RegisterHistogram("same_name", nil, []float64{1, 2, 3})
	s := r.RegisterSummary("same_name", nil, []float64{0.5})

	c.Add(42)
	g.Set(100)
	h.Observe(1.5)
	s.Observe(50)

	snap := r.Snapshot()

	if len(snap) != 4 {
		t.Fatalf("expected 4 metrics in snapshot, got %d", len(snap))
	}

	typeCount := map[MetricType]int{}
	for _, m := range snap {
		if m.Name != "same_name" {
			t.Errorf("unexpected metric name: %v", m.Name)
		}
		typeCount[m.Type]++
	}

	if typeCount[CounterType] != 1 {
		t.Errorf("expected 1 counter, got %d", typeCount[CounterType])
	}
	if typeCount[GaugeType] != 1 {
		t.Errorf("expected 1 gauge, got %d", typeCount[GaugeType])
	}
	if typeCount[HistogramType] != 1 {
		t.Errorf("expected 1 histogram, got %d", typeCount[HistogramType])
	}
	if typeCount[SummaryType] != 1 {
		t.Errorf("expected 1 summary, got %d", typeCount[SummaryType])
	}

	ok := r.Unregister("same_name", nil)
	if !ok {
		t.Error("unregister should succeed")
	}

	snap2 := r.Snapshot()
	if len(snap2) != 3 {
		t.Fatalf("expected 3 metrics after unregistering counter, got %d", len(snap2))
	}

	_, counterExists := r.GetCounter("same_name", nil)
	if counterExists {
		t.Error("counter should not exist after unregister")
	}

	_, gaugeExists := r.GetGauge("same_name", nil)
	if !gaugeExists {
		t.Error("gauge should still exist after unregistering counter")
	}

	snap3 := r.Snapshot()
	hasGauge := false
	for _, m := range snap3 {
		if m.Type == GaugeType {
			hasGauge = true
			break
		}
	}
	if !hasGauge {
		t.Error("gauge should still be visible in snapshot after unregistering counter")
	}
}
