package perfsampler

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAlwaysSample(t *testing.T) {
	s := NewAlwaysSample()

	if !s.ShouldSample("any-id") {
		t.Error("AlwaysSample should always return true")
	}

	if s.Rate() != 1.0 {
		t.Errorf("expected rate 1.0, got %v", s.Rate())
	}
}

func TestNeverSample(t *testing.T) {
	s := NewNeverSample()

	if s.ShouldSample("any-id") {
		t.Error("NeverSample should always return false")
	}

	if s.Rate() != 0.0 {
		t.Errorf("expected rate 0.0, got %v", s.Rate())
	}
}

func TestProbabilitySampler_ValidRate(t *testing.T) {
	testCases := []float64{0, 0.1, 0.5, 0.9, 1.0}

	for _, rate := range testCases {
		s, err := NewProbabilitySampler(rate)
		if err != nil {
			t.Errorf("NewProbabilitySampler(%v) returned error: %v", rate, err)
			continue
		}
		if s.Rate() != rate {
			t.Errorf("expected rate %v, got %v", rate, s.Rate())
		}
	}
}

func TestProbabilitySampler_InvalidRate(t *testing.T) {
	testCases := []float64{-0.1, 1.1, -100, 100}

	for _, rate := range testCases {
		_, err := NewProbabilitySampler(rate)
		if err == nil {
			t.Errorf("NewProbabilitySampler(%v) should return error", rate)
		}
		if !errors.Is(err, ErrInvalidSampleRate) {
			t.Errorf("expected ErrInvalidSampleRate, got %v", err)
		}
	}
}

func TestProbabilitySampler_ShouldSample(t *testing.T) {
	s, err := NewProbabilitySampler(0.5)
	if err != nil {
		t.Fatal(err)
	}

	sampledCount := 0
	total := 10000

	for i := 0; i < total; i++ {
		reqID, err := GenerateRequestID()
		if err != nil {
			t.Fatal(err)
		}
		if s.ShouldSample(reqID) {
			sampledCount++
		}
	}

	ratio := float64(sampledCount) / float64(total)
	if ratio < 0.4 || ratio > 0.6 {
		t.Errorf("expected sampling ratio around 0.5, got %v", ratio)
	}
}

func TestProbabilitySampler_BoundaryRates(t *testing.T) {
	s1, _ := NewProbabilitySampler(0.0)
	if s1.ShouldSample("any-id") {
		t.Error("rate 0.0 should never sample")
	}

	s2, _ := NewProbabilitySampler(1.0)
	if !s2.ShouldSample("any-id") {
		t.Error("rate 1.0 should always sample")
	}
}

func TestGenerateRequestID(t *testing.T) {
	id1, err := GenerateRequestID()
	if err != nil {
		t.Fatal(err)
	}
	if len(id1) != 32 {
		t.Errorf("expected 32-char hex string, got %d chars", len(id1))
	}

	id2, err := GenerateRequestID()
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Error("generated IDs should be unique")
	}
}

func TestNewRequestProfiler_Valid(t *testing.T) {
	sampler := NewAlwaysSample()
	p, err := NewRequestProfiler("test-req-1", sampler)
	if err != nil {
		t.Fatal(err)
	}
	if p.RequestID() != "test-req-1" {
		t.Errorf("expected request ID 'test-req-1', got %v", p.RequestID())
	}
	if !p.IsSampled() {
		t.Error("expected sampled=true with AlwaysSample")
	}
}

func TestNewRequestProfiler_EmptyID(t *testing.T) {
	_, err := NewRequestProfiler("", NewAlwaysSample())
	if err == nil {
		t.Error("expected error for empty request ID")
	}
	if !errors.Is(err, ErrEmptyRequestID) {
		t.Errorf("expected ErrEmptyRequestID, got %v", err)
	}
}

func TestNewRequestProfiler_NilSampler(t *testing.T) {
	_, err := NewRequestProfiler("test-id", nil)
	if err == nil {
		t.Error("expected error for nil sampler")
	}
	if !errors.Is(err, ErrNilSampler) {
		t.Errorf("expected ErrNilSampler, got %v", err)
	}
}

func TestNewRequestProfiler_NotSampled(t *testing.T) {
	p, err := NewRequestProfiler("test-req-2", NewNeverSample())
	if err != nil {
		t.Fatal(err)
	}
	if p.IsSampled() {
		t.Error("expected sampled=false with NeverSample")
	}
}

func TestRequestProfiler_StartStop(t *testing.T) {
	p, _ := NewRequestProfiler("test-start-stop", NewAlwaysSample())

	if err := p.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	if err := p.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	result, err := p.Export()
	if err != nil {
		t.Fatalf("Export() failed: %v", err)
	}

	if result.Duration < 10*time.Millisecond {
		t.Errorf("expected duration >= 10ms, got %v", result.Duration)
	}
	if result.StartTime.After(result.EndTime) {
		t.Error("start time should be before end time")
	}
}

func TestRequestProfiler_StartTwice(t *testing.T) {
	p, _ := NewRequestProfiler("test-start-twice", NewAlwaysSample())

	_ = p.Start()
	err := p.Start()
	if err == nil {
		t.Error("expected error for starting twice")
	}
	if !errors.Is(err, ErrProfilerAlreadyStarted) {
		t.Errorf("expected ErrProfilerAlreadyStarted, got %v", err)
	}
}

func TestRequestProfiler_StopWithoutStart(t *testing.T) {
	p, _ := NewRequestProfiler("test-stop-no-start", NewAlwaysSample())

	err := p.Stop()
	if err == nil {
		t.Error("expected error for stopping without start")
	}
	if !errors.Is(err, ErrProfilerNotStarted) {
		t.Errorf("expected ErrProfilerNotStarted, got %v", err)
	}
}

func TestRequestProfiler_ExportWithoutStop(t *testing.T) {
	p, _ := NewRequestProfiler("test-export-no-stop", NewAlwaysSample())
	_ = p.Start()

	_, err := p.Export()
	if err == nil {
		t.Error("expected error for exporting without stop")
	}
	if !errors.Is(err, ErrProfilerNotStarted) {
		t.Errorf("expected ErrProfilerNotStarted, got %v", err)
	}
}

func TestCPUProfiling_EnterExit(t *testing.T) {
	p, _ := NewRequestProfiler("test-cpu-enter-exit", NewAlwaysSample())
	_ = p.Start()

	err := p.EnterCPUFunction("main")
	if err != nil {
		t.Fatal(err)
	}

	err = p.EnterCPUFunction("foo")
	if err != nil {
		t.Fatal(err)
	}

	err = p.ExitCPUFunction()
	if err != nil {
		t.Fatal(err)
	}

	err = p.EnterCPUFunction("bar")
	if err != nil {
		t.Fatal(err)
	}

	err = p.ExitCPUFunction()
	if err != nil {
		t.Fatal(err)
	}

	err = p.ExitCPUFunction()
	if err != nil {
		t.Fatal(err)
	}

	_ = p.Stop()

	result, _ := p.Export()
	if result.CPUProfile == nil {
		t.Fatal("expected CPU profile")
	}

	if result.CPUProfile.FunctionName != "root" {
		t.Errorf("expected root node, got %v", result.CPUProfile.FunctionName)
	}

	if len(result.CPUProfile.Children) != 1 {
		t.Fatalf("expected 1 child of root, got %d", len(result.CPUProfile.Children))
	}

	mainNode := result.CPUProfile.Children[0]
	if mainNode.FunctionName != "main" {
		t.Errorf("expected 'main', got %v", mainNode.FunctionName)
	}
	if mainNode.SampleCount != 1 {
		t.Errorf("expected sample count 1, got %d", mainNode.SampleCount)
	}

	if len(mainNode.Children) != 2 {
		t.Fatalf("expected 2 children of main, got %d", len(mainNode.Children))
	}
}

func TestCPUProfiling_RecordSample(t *testing.T) {
	p, _ := NewRequestProfiler("test-cpu-record", NewAlwaysSample())
	_ = p.Start()

	stack1 := []string{"main", "foo", "bar"}
	stack2 := []string{"main", "foo", "baz"}
	stack3 := []string{"main", "qux"}

	for i := 0; i < 5; i++ {
		_ = p.RecordCPUSample(stack1)
	}
	for i := 0; i < 3; i++ {
		_ = p.RecordCPUSample(stack2)
	}
	for i := 0; i < 2; i++ {
		_ = p.RecordCPUSample(stack3)
	}

	_ = p.Stop()

	result, _ := p.Export()
	cpu := result.CPUProfile

	if cpu.Children[0].FunctionName != "main" {
		t.Fatalf("expected 'main', got %v", cpu.Children[0].FunctionName)
	}

	main := cpu.Children[0]
	if main.SampleCount != 10 {
		t.Errorf("expected main sample count 10, got %d", main.SampleCount)
	}

	foo := main.Children[0]
	if foo.FunctionName != "foo" {
		t.Fatalf("expected 'foo', got %v", foo.FunctionName)
	}
	if foo.SampleCount != 8 {
		t.Errorf("expected foo sample count 8, got %d", foo.SampleCount)
	}

	bar := foo.Children[0]
	if bar.SampleCount != 5 {
		t.Errorf("expected bar sample count 5, got %d", bar.SampleCount)
	}

	baz := foo.Children[1]
	if baz.SampleCount != 3 {
		t.Errorf("expected baz sample count 3, got %d", baz.SampleCount)
	}
}

func TestCPUProfiling_ExitEmptyStack(t *testing.T) {
	p, _ := NewRequestProfiler("test-cpu-exit-empty", NewAlwaysSample())
	_ = p.Start()

	err := p.ExitCPUFunction()
	if err == nil {
		t.Error("expected error when exiting empty stack")
	}
}

func TestMemoryProfiling_AllocFree(t *testing.T) {
	p, _ := NewRequestProfiler("test-mem-alloc", NewAlwaysSample())
	_ = p.Start()

	_ = p.RecordAlloc("parseJSON", 1024)
	_ = p.RecordAlloc("parseJSON", 2048)
	_ = p.RecordAlloc("processData", 512)
	_ = p.RecordFree("parseJSON", 1024)

	_ = p.Stop()

	result, _ := p.Export()

	if len(result.MemoryStats) != 2 {
		t.Fatalf("expected 2 memory stats, got %d", len(result.MemoryStats))
	}

	var parseJSON *MemoryFuncStat
	var processData *MemoryFuncStat

	for _, stat := range result.MemoryStats {
		if stat.FunctionName == "parseJSON" {
			parseJSON = stat
		} else if stat.FunctionName == "processData" {
			processData = stat
		}
	}

	if parseJSON == nil {
		t.Fatal("parseJSON stat not found")
	}
	if parseJSON.AllocCount != 2 {
		t.Errorf("expected AllocCount 2, got %d", parseJSON.AllocCount)
	}
	if parseJSON.AllocBytes != 3072 {
		t.Errorf("expected AllocBytes 3072, got %d", parseJSON.AllocBytes)
	}
	if parseJSON.FreeCount != 1 {
		t.Errorf("expected FreeCount 1, got %d", parseJSON.FreeCount)
	}
	if parseJSON.InUseBytes != 2048 {
		t.Errorf("expected InUseBytes 2048, got %d", parseJSON.InUseBytes)
	}

	if processData.AllocBytes != 512 {
		t.Errorf("expected processData AllocBytes 512, got %d", processData.AllocBytes)
	}
}

func TestMemoryProfiling_EmptyFunction(t *testing.T) {
	p, _ := NewRequestProfiler("test-mem-empty", NewAlwaysSample())
	_ = p.Start()

	err := p.RecordAlloc("", 100)
	if err == nil {
		t.Error("expected error for empty function name")
	}

	err = p.RecordFree("", 100)
	if err == nil {
		t.Error("expected error for empty function name")
	}
}

func TestTimingProfiling_Basic(t *testing.T) {
	p, _ := NewRequestProfiler("test-timing-basic", NewAlwaysSample())
	_ = p.Start()

	_ = p.StartSegment("parseRequest")
	time.Sleep(5 * time.Millisecond)
	seg1, err := p.EndSegment()
	if err != nil {
		t.Fatal(err)
	}
	if seg1.Duration < 5*time.Millisecond {
		t.Errorf("expected duration >= 5ms, got %v", seg1.Duration)
	}

	_ = p.StartSegment("processLogic")
	time.Sleep(10 * time.Millisecond)
	seg2, err := p.EndSegment()
	if err != nil {
		t.Fatal(err)
	}
	if seg2.Duration < 10*time.Millisecond {
		t.Errorf("expected duration >= 10ms, got %v", seg2.Duration)
	}

	_ = p.Stop()

	result, _ := p.Export()
	if len(result.Timing) != 2 {
		t.Fatalf("expected 2 timing segments, got %d", len(result.Timing))
	}
}

func TestTimingProfiling_Nested(t *testing.T) {
	p, _ := NewRequestProfiler("test-timing-nested", NewAlwaysSample())
	_ = p.Start()

	_ = p.StartSegment("outer")
	time.Sleep(2 * time.Millisecond)

	_ = p.StartSegment("inner")
	time.Sleep(3 * time.Millisecond)
	_, _ = p.EndSegment()

	time.Sleep(1 * time.Millisecond)
	_, _ = p.EndSegment()

	_ = p.Stop()

	result, _ := p.Export()
	if len(result.Timing) != 2 {
		t.Fatalf("expected 2 timing segments, got %d", len(result.Timing))
	}

	if result.Timing[0].Label != "inner" {
		t.Errorf("expected first segment 'inner', got %v", result.Timing[0].Label)
	}
	if result.Timing[1].Label != "outer" {
		t.Errorf("expected second segment 'outer', got %v", result.Timing[1].Label)
	}
}

func TestTimingProfiling_WithMetadata(t *testing.T) {
	p, _ := NewRequestProfiler("test-timing-meta", NewAlwaysSample())
	_ = p.Start()

	md := map[string]string{"db": "mysql", "table": "users"}
	_ = p.StartSegment("dbQuery", md)

	_ = p.SetSegmentMetadata("dbQuery", "rows", "100")

	_, _ = p.EndSegment()
	_ = p.Stop()

	result, _ := p.Export()
	seg := result.Timing[0]

	if seg.Metadata["db"] != "mysql" {
		t.Errorf("expected metadata db=mysql, got %v", seg.Metadata["db"])
	}
	if seg.Metadata["rows"] != "100" {
		t.Errorf("expected metadata rows=100, got %v", seg.Metadata["rows"])
	}
}

func TestTimingProfiling_EndWithoutStart(t *testing.T) {
	p, _ := NewRequestProfiler("test-timing-end-no-start", NewAlwaysSample())
	_ = p.Start()

	_, err := p.EndSegment()
	if err == nil {
		t.Error("expected error for ending without start")
	}
	if !errors.Is(err, ErrSegmentNotStarted) {
		t.Errorf("expected ErrSegmentNotStarted, got %v", err)
	}
}

func TestTimingProfiling_EmptyLabel(t *testing.T) {
	p, _ := NewRequestProfiler("test-timing-empty-label", NewAlwaysSample())
	_ = p.Start()

	err := p.StartSegment("")
	if err == nil {
		t.Error("expected error for empty label")
	}
}

func TestTimingProfiling_AutoStop(t *testing.T) {
	p, _ := NewRequestProfiler("test-timing-autostop", NewAlwaysSample())
	_ = p.Start()

	_ = p.StartSegment("unclosed1")
	_ = p.StartSegment("unclosed2")

	_ = p.Stop()

	result, _ := p.Export()
	if len(result.Timing) != 2 {
		t.Fatalf("expected 2 auto-closed segments, got %d", len(result.Timing))
	}
}

func TestExport_NotSampled(t *testing.T) {
	p, _ := NewRequestProfiler("test-export-not-sampled", NewNeverSample())
	_ = p.Start()

	_ = p.EnterCPUFunction("foo")
	_ = p.RecordAlloc("bar", 100)
	_ = p.StartSegment("baz")

	_ = p.Stop()

	result, err := p.Export()
	if err != nil {
		t.Fatal(err)
	}

	if result.Sampled {
		t.Error("expected sampled=false")
	}
	if result.CPUProfile != nil {
		t.Error("expected nil CPU profile when not sampled")
	}
	if len(result.MemoryStats) != 0 {
		t.Error("expected empty memory stats when not sampled")
	}
	if len(result.Timing) != 0 {
		t.Error("expected empty timing when not sampled")
	}
}

func TestExport_JSON(t *testing.T) {
	p, _ := NewRequestProfiler("test-export-json", NewAlwaysSample())
	_ = p.Start()

	_ = p.EnterCPUFunction("handler")
	_ = p.EnterCPUFunction("dbQuery")
	_ = p.ExitCPUFunction()
	_ = p.ExitCPUFunction()

	_ = p.RecordAlloc("dbQuery", 2048)

	_ = p.StartSegment("dbQuery")
	_, _ = p.EndSegment()

	_ = p.Stop()

	result, err := p.Export()
	if err != nil {
		t.Fatal(err)
	}

	jsonData, err := result.JSON()
	if err != nil {
		t.Fatalf("JSON() failed: %v", err)
	}

	var decoded ProfileResult
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if decoded.RequestID != "test-export-json" {
		t.Errorf("expected request ID 'test-export-json', got %v", decoded.RequestID)
	}
	if !decoded.Sampled {
		t.Error("expected sampled=true")
	}
	if decoded.CPUProfile == nil {
		t.Error("expected non-nil CPU profile")
	}
}

func TestExport_PrettyJSON(t *testing.T) {
	p, _ := NewRequestProfiler("test-pretty-json", NewAlwaysSample())
	_ = p.Start()
	_ = p.Stop()

	result, _ := p.Export()
	jsonData, err := result.PrettyJSON()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(jsonData), "\n") {
		t.Error("expected pretty JSON to contain newlines")
	}
}

func TestFlameGraph(t *testing.T) {
	p, _ := NewRequestProfiler("test-flamegraph", NewAlwaysSample())
	_ = p.Start()

	_ = p.RecordCPUSample([]string{"main", "foo", "bar"})
	_ = p.RecordCPUSample([]string{"main", "foo", "bar"})
	_ = p.RecordCPUSample([]string{"main", "foo", "baz"})
	_ = p.RecordCPUSample([]string{"main", "qux"})

	_ = p.Stop()

	entries, err := p.ToFlameGraph()
	if err != nil {
		t.Fatalf("ToFlameGraph() failed: %v", err)
	}

	foundMain := false
	foundFoo := false
	foundBar := false
	foundBaz := false
	foundQux := false

	for _, entry := range entries {
		if len(entry.Stack) == 0 {
			continue
		}
		last := entry.Stack[len(entry.Stack)-1]
		switch last {
		case "main":
			foundMain = true
			if entry.Value != 4 {
				t.Errorf("expected main value 4, got %d", entry.Value)
			}
		case "foo":
			foundFoo = true
			if entry.Value != 3 {
				t.Errorf("expected foo value 3, got %d", entry.Value)
			}
		case "bar":
			foundBar = true
			if entry.Value != 2 {
				t.Errorf("expected bar value 2, got %d", entry.Value)
			}
		case "baz":
			foundBaz = true
			if entry.Value != 1 {
				t.Errorf("expected baz value 1, got %d", entry.Value)
			}
		case "qux":
			foundQux = true
			if entry.Value != 1 {
				t.Errorf("expected qux value 1, got %d", entry.Value)
			}
		}
	}

	if !foundMain || !foundFoo || !foundBar || !foundBaz || !foundQux {
		t.Error("not all flame graph entries found")
	}
}

func TestFlameGraph_NotSampled(t *testing.T) {
	p, _ := NewRequestProfiler("test-flame-not-sampled", NewNeverSample())
	_ = p.Start()
	_ = p.Stop()

	_, err := p.ToFlameGraph()
	if err == nil {
		t.Error("expected error for not sampled")
	}
	if !errors.Is(err, ErrNotSampled) {
		t.Errorf("expected ErrNotSampled, got %v", err)
	}
}

func TestFlameGraph_JSONSerialization(t *testing.T) {
	p, _ := NewRequestProfiler("test-flame-json", NewAlwaysSample())
	_ = p.Start()
	_ = p.RecordCPUSample([]string{"a", "b", "c"})
	_ = p.Stop()

	entries, _ := p.ToFlameGraph()
	jsonData, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("flame graph JSON marshal failed: %v", err)
	}

	var decoded []*FlameGraphEntry
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("flame graph JSON unmarshal failed: %v", err)
	}

	if len(decoded) != 3 {
		t.Errorf("expected 3 entries, got %d", len(decoded))
	}
}

func TestConcurrentAccess(t *testing.T) {
	p, _ := NewRequestProfiler("test-concurrent", NewAlwaysSample())
	_ = p.Start()

	var wg sync.WaitGroup
	numGoroutines := 10
	iterations := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				funcName := "worker"
				_ = p.EnterCPUFunction(funcName)
				_ = p.RecordAlloc(funcName, 64)
				_ = p.ExitCPUFunction()

				segName := "segment"
				_ = p.StartSegment(segName)
				_, _ = p.EndSegment()
			}
		}(i)
	}

	wg.Wait()
	_ = p.Stop()

	result, _ := p.Export()

	totalAllocs := int64(0)
	for _, stat := range result.MemoryStats {
		totalAllocs += stat.AllocCount
	}

	expectedAllocs := int64(numGoroutines * iterations)
	if totalAllocs != expectedAllocs {
		t.Errorf("expected %d allocs, got %d", expectedAllocs, totalAllocs)
	}

	expectedSegments := numGoroutines * iterations
	if len(result.Timing) != expectedSegments {
		t.Errorf("expected %d timing segments, got %d", expectedSegments, len(result.Timing))
	}
}

func TestOperationsWithoutStart(t *testing.T) {
	p, _ := NewRequestProfiler("test-op-no-start", NewAlwaysSample())

	err := p.EnterCPUFunction("foo")
	if !errors.Is(err, ErrProfilerNotStarted) {
		t.Errorf("expected ErrProfilerNotStarted, got %v", err)
	}

	err = p.RecordAlloc("foo", 100)
	if !errors.Is(err, ErrProfilerNotStarted) {
		t.Errorf("expected ErrProfilerNotStarted, got %v", err)
	}

	err = p.StartSegment("foo")
	if !errors.Is(err, ErrProfilerNotStarted) {
		t.Errorf("expected ErrProfilerNotStarted, got %v", err)
	}
}

func TestOperationsAfterStop(t *testing.T) {
	p, _ := NewRequestProfiler("test-op-after-stop", NewAlwaysSample())
	_ = p.Start()
	_ = p.Stop()

	err := p.EnterCPUFunction("foo")
	if !errors.Is(err, ErrProfilerNotStarted) {
		t.Errorf("expected ErrProfilerNotStarted, got %v", err)
	}
}

func TestSetSegmentMetadata_NotFound(t *testing.T) {
	p, _ := NewRequestProfiler("test-meta-notfound", NewAlwaysSample())
	_ = p.Start()

	err := p.SetSegmentMetadata("nonexistent", "key", "value")
	if err == nil {
		t.Error("expected error for nonexistent segment")
	}
	if !errors.Is(err, ErrSegmentNotStarted) {
		t.Errorf("expected ErrSegmentNotStarted, got %v", err)
	}
}

func TestCPUProfile_TotalTimeCalculation(t *testing.T) {
	p, _ := NewRequestProfiler("test-cpu-totaltime", NewAlwaysSample())
	_ = p.Start()

	_ = p.RecordCPUSample([]string{"a", "b", "c"})
	_ = p.RecordCPUSample([]string{"a", "b", "d"})
	_ = p.RecordCPUSample([]string{"a", "e"})

	_ = p.Stop()

	result, _ := p.Export()
	cpu := result.CPUProfile

	a := cpu.Children[0]
	if a.TotalTimeNs != 3000 {
		t.Errorf("expected a.TotalTimeNs 3000, got %d", a.TotalTimeNs)
	}

	b := a.Children[0]
	if b.TotalTimeNs != 2000 {
		t.Errorf("expected b.TotalTimeNs 2000, got %d", b.TotalTimeNs)
	}

	c := b.Children[0]
	if c.SelfTimeNs == 0 {
		t.Error("leaf node should have SelfTimeNs > 0")
	}
}

func TestProbabilitySampler_ShortID(t *testing.T) {
	s, _ := NewProbabilitySampler(0.5)

	if !s.ShouldSample("short") {
		t.Error("short ID with rate>0 should sample")
	}

	s0, _ := NewProbabilitySampler(0.0)
	if s0.ShouldSample("short") {
		t.Error("short ID with rate=0 should not sample")
	}
}

func TestProbabilitySampler_InvalidHex(t *testing.T) {
	s, _ := NewProbabilitySampler(0.5)
	if !s.ShouldSample("zzzzzzzzzzzzzzzz") {
		t.Error("invalid hex with rate>0 should sample")
	}
}

func TestMemoryProfiling_FreeMoreThanAlloc(t *testing.T) {
	p, _ := NewRequestProfiler("test-mem-overfree", NewAlwaysSample())
	_ = p.Start()

	_ = p.RecordAlloc("foo", 100)
	_ = p.RecordFree("foo", 200)

	_ = p.Stop()

	result, _ := p.Export()
	for _, stat := range result.MemoryStats {
		if stat.FunctionName == "foo" {
			if stat.InUseBytes != 0 {
				t.Errorf("expected InUseBytes 0, got %d", stat.InUseBytes)
			}
		}
	}
}

func TestTimingProfiling_EmptyMetadata(t *testing.T) {
	p, _ := NewRequestProfiler("test-timing-empty-meta", NewAlwaysSample())
	_ = p.Start()

	_ = p.StartSegment("test")
	_ = p.SetSegmentMetadata("test", "key", "value")
	seg, _ := p.EndSegment()

	if seg.Metadata["key"] != "value" {
		t.Errorf("expected metadata key=value, got %v", seg.Metadata["key"])
	}
}

func TestProfileResult_JSONOmitempty(t *testing.T) {
	result := &ProfileResult{
		RequestID: "test",
		Sampled:   false,
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}

	jsonData, err := result.JSON()
	if err != nil {
		t.Fatal(err)
	}

	jsonStr := string(jsonData)
	if strings.Contains(jsonStr, "cpu_profile") {
		t.Error("nil CPU profile should be omitted from JSON")
	}
	if strings.Contains(jsonStr, "memory_stats") {
		t.Error("nil memory stats should be omitted from JSON")
	}
	if strings.Contains(jsonStr, "timing_segments") {
		t.Error("nil timing should be omitted from JSON")
	}
}

func TestFullWorkflow(t *testing.T) {
	reqID, _ := GenerateRequestID()
	sampler, _ := NewProbabilitySampler(1.0)

	p, err := NewRequestProfiler(reqID, sampler)
	if err != nil {
		t.Fatal(err)
	}

	if err := p.Start(); err != nil {
		t.Fatal(err)
	}

	_ = p.StartSegment("total")

	_ = p.EnterCPUFunction("handleRequest")

	_ = p.StartSegment("parseInput")
	_ = p.RecordAlloc("parseJSON", 1024)
	_ = p.EnterCPUFunction("parseJSON")
	time.Sleep(1 * time.Millisecond)
	_ = p.ExitCPUFunction()
	_, _ = p.EndSegment()

	_ = p.StartSegment("businessLogic")
	_ = p.RecordAlloc("processData", 2048)
	_ = p.EnterCPUFunction("processData")
	_ = p.EnterCPUFunction("validate")
	time.Sleep(2 * time.Millisecond)
	_ = p.ExitCPUFunction()
	_ = p.ExitCPUFunction()
	_, _ = p.EndSegment()

	_ = p.StartSegment("sendResponse")
	_ = p.EnterCPUFunction("sendResponse")
	time.Sleep(1 * time.Millisecond)
	_ = p.ExitCPUFunction()
	_, _ = p.EndSegment()

	_ = p.ExitCPUFunction()

	_, _ = p.EndSegment()

	if err := p.Stop(); err != nil {
		t.Fatal(err)
	}

	result, err := p.Export()
	if err != nil {
		t.Fatal(err)
	}

	if result.RequestID != reqID {
		t.Errorf("request ID mismatch")
	}
	if !result.Sampled {
		t.Error("should be sampled")
	}
	if result.CPUProfile == nil {
		t.Error("CPU profile should not be nil")
	}
	if len(result.MemoryStats) != 2 {
		t.Errorf("expected 2 memory stats, got %d", len(result.MemoryStats))
	}
	if len(result.Timing) != 4 {
		t.Errorf("expected 4 timing segments, got %d", len(result.Timing))
	}

	flame, err := p.ToFlameGraph()
	if err != nil {
		t.Fatal(err)
	}
	if len(flame) == 0 {
		t.Error("flame graph should not be empty")
	}

	jsonData, err := result.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if len(jsonData) == 0 {
		t.Error("JSON should not be empty")
	}
}

func TestDoubleStop(t *testing.T) {
	p, _ := NewRequestProfiler("test-double-stop", NewAlwaysSample())
	_ = p.Start()

	if err := p.Stop(); err != nil {
		t.Fatalf("first Stop() failed: %v", err)
	}

	if err := p.Stop(); err != nil {
		t.Errorf("second Stop() should not return error, got: %v", err)
	}
}

func TestExportWithoutStart(t *testing.T) {
	p, _ := NewRequestProfiler("test-export-no-start", NewAlwaysSample())

	_, err := p.Export()
	if err == nil {
		t.Error("expected error for export without start")
	}
	if !errors.Is(err, ErrProfilerNotStarted) {
		t.Errorf("expected ErrProfilerNotStarted, got %v", err)
	}
}

func TestCPUProfiling_RecordEmptyStack(t *testing.T) {
	p, _ := NewRequestProfiler("test-empty-stack", NewAlwaysSample())
	_ = p.Start()

	err := p.RecordCPUSample([]string{})
	if err == nil {
		t.Error("expected error for empty stack")
	}
}

func TestCPUProfiling_EmptyFunctionName(t *testing.T) {
	p, _ := NewRequestProfiler("test-empty-func", NewAlwaysSample())
	_ = p.Start()

	err := p.EnterCPUFunction("")
	if err == nil {
		t.Error("expected error for empty function name")
	}
}
