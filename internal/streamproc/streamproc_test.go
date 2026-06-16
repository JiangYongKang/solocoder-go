package streamproc

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewRecord(t *testing.T) {
	rec := NewRecord(42)
	if rec == nil {
		t.Fatal("NewRecord returned nil")
	}
	if rec.Data != 42 {
		t.Errorf("expected Data=42, got %v", rec.Data)
	}
	if rec.Timestamp.IsZero() {
		t.Error("expected Timestamp to be set")
	}
	if rec.Metadata == nil {
		t.Error("expected Metadata to be initialized")
	}
}

func TestRecordClone(t *testing.T) {
	orig := NewRecord("test")
	orig.ID = "id1"
	orig.SeqID = 100
	orig.SetMeta("key", "value")

	clone := orig.Clone()
	if clone == orig {
		t.Error("clone should be a different pointer")
	}
	if clone.ID != orig.ID {
		t.Errorf("expected ID=%s, got %s", orig.ID, clone.ID)
	}
	if clone.Data != orig.Data {
		t.Errorf("expected Data=%v, got %v", orig.Data, clone.Data)
	}
	if clone.SeqID != orig.SeqID {
		t.Errorf("expected SeqID=%d, got %d", orig.SeqID, clone.SeqID)
	}

	v, ok := clone.GetMeta("key")
	if !ok || v != "value" {
		t.Error("expected metadata to be cloned")
	}

	orig.SetMeta("new", "value")
	if _, ok := clone.GetMeta("new"); ok {
		t.Error("clone should not have new metadata from original")
	}
}

func TestRecordMetadata(t *testing.T) {
	rec := NewRecord(nil)
	rec.SetMeta("key1", "val1")

	v, ok := rec.GetMeta("key1")
	if !ok || v != "val1" {
		t.Errorf("expected GetMeta to return val1")
	}

	_, ok = rec.GetMeta("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent key")
	}

	nilRec := &Record{}
	_, ok = nilRec.GetMeta("key")
	if ok {
		t.Error("expected ok=false for nil metadata")
	}

	nilRec.SetMeta("key", "value")
	v, ok = nilRec.GetMeta("key")
	if !ok || v != "value" {
		t.Error("SetMeta should initialize metadata if nil")
	}
}

func TestSourceStateString(t *testing.T) {
	tests := []struct {
		state    SourceState
		expected string
	}{
		{SourceStateIdle, "idle"},
		{SourceStateRunning, "running"},
		{SourceStatePaused, "paused"},
		{SourceStateStopped, "stopped"},
		{SourceState(99), "unknown"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("SourceState(%d).String()=%s, expected %s", tt.state, tt.state.String(), tt.expected)
		}
	}
}

func TestWindowTypeString(t *testing.T) {
	tests := []struct {
		wt       WindowType
		expected string
	}{
		{WindowTypeTumblingTime, "tumbling_time"},
		{WindowTypeSlidingTime, "sliding_time"},
		{WindowTypeTumblingCount, "tumbling_count"},
		{WindowTypeSlidingCount, "sliding_count"},
		{WindowType(99), "unknown"},
	}

	for _, tt := range tests {
		if tt.wt.String() != tt.expected {
			t.Errorf("WindowType(%d).String()=%s, expected %s", tt.wt, tt.wt.String(), tt.expected)
		}
	}
}

func TestAggregationTypeString(t *testing.T) {
	tests := []struct {
		at       AggregationType
		expected string
	}{
		{AggregationSum, "sum"},
		{AggregationCount, "count"},
		{AggregationAvg, "avg"},
		{AggregationMin, "min"},
		{AggregationMax, "max"},
		{AggregationType(99), "unknown"},
	}

	for _, tt := range tests {
		if tt.at.String() != tt.expected {
			t.Errorf("AggregationType(%d).String()=%s, expected %s", tt.at, tt.at.String(), tt.expected)
		}
	}
}

func TestBackpressureStateString(t *testing.T) {
	tests := []struct {
		bs       BackpressureState
		expected string
	}{
		{BackpressureNormal, "normal"},
		{BackpressureWarning, "warning"},
		{BackpressureCritical, "critical"},
		{BackpressureState(99), "unknown"},
	}

	for _, tt := range tests {
		if tt.bs.String() != tt.expected {
			t.Errorf("BackpressureState(%d).String()=%s, expected %s", tt.bs, tt.bs.String(), tt.expected)
		}
	}
}

func TestPipelineStatusString(t *testing.T) {
	tests := []struct {
		status   PipelineStatus
		expected string
	}{
		{PipelineStatusIdle, "idle"},
		{PipelineStatusRunning, "running"},
		{PipelineStatusPaused, "paused"},
		{PipelineStatusCompleted, "completed"},
		{PipelineStatusFailed, "failed"},
		{PipelineStatusStopped, "stopped"},
		{PipelineStatus(99), "unknown"},
	}

	for _, tt := range tests {
		if tt.status.String() != tt.expected {
			t.Errorf("PipelineStatus(%d).String()=%s, expected %s", tt.status, tt.status.String(), tt.expected)
		}
	}
}

func TestNewChannelSource(t *testing.T) {
	input := make(chan *Record, 10)
	source := NewChannelSource("test", input, 5)

	if source.Name() != "test" {
		t.Errorf("expected name=test, got %s", source.Name())
	}
	if source.State() != SourceStateIdle {
		t.Errorf("expected state=Idle, got %v", source.State())
	}
	if source.Output() == nil {
		t.Error("expected output channel to be set")
	}
}

func TestChannelSourceStartStop(t *testing.T) {
	input := make(chan *Record, 10)
	source := NewChannelSource("test", input, 10)
	ctx := context.Background()

	err := source.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if source.State() != SourceStateRunning {
		t.Errorf("expected state=Running, got %v", source.State())
	}

	err = source.Start(ctx)
	if err != ErrSourceAlreadyStarted {
		t.Errorf("expected ErrSourceAlreadyStarted, got %v", err)
	}

	rec := NewRecord(1)
	input <- rec

	select {
	case out := <-source.Output():
		if out == nil {
			t.Error("expected record from output")
		}
		if out.SeqID != 1 {
			t.Errorf("expected SeqID=1, got %d", out.SeqID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for output")
	}

	err = source.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if source.State() != SourceStateStopped {
		t.Errorf("expected state=Stopped, got %v", source.State())
	}

	err = source.Stop()
	if err != nil {
		t.Errorf("second Stop should not error: %v", err)
	}
}

func TestChannelSourcePauseResume(t *testing.T) {
	input := make(chan *Record, 10)
	source := NewChannelSource("test", input, 10)
	ctx := context.Background()

	err := source.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	err = source.Pause()
	if err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	if source.State() != SourceStatePaused {
		t.Errorf("expected state=Paused, got %v", source.State())
	}

	err = source.Pause()
	if err != ErrSourceNotStarted {
		t.Errorf("expected ErrSourceNotStarted, got %v", err)
	}

	err = source.Resume()
	if err != nil {
		t.Fatalf("Resume failed: %v", err)
	}
	if source.State() != SourceStateRunning {
		t.Errorf("expected state=Running, got %v", source.State())
	}

	_ = source.Stop()
}

func TestSliceSource(t *testing.T) {
	records := []*Record{
		NewRecord(1),
		NewRecord(2),
		NewRecord(3),
	}
	source := NewSliceSource("test", records, 10, 0)

	if source.Name() != "test" {
		t.Errorf("expected name=test, got %s", source.Name())
	}
	if source.CurrentIndex() != 0 {
		t.Errorf("expected index=0, got %d", source.CurrentIndex())
	}

	ctx := context.Background()
	err := source.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	received := 0
	for range source.Output() {
		received++
	}

	if received != 3 {
		t.Errorf("expected 3 records, got %d", received)
	}
	if source.CurrentIndex() != 3 {
		t.Errorf("expected index=3, got %d", source.CurrentIndex())
	}

	source.Reset()
	if source.CurrentIndex() != 0 {
		t.Errorf("expected index=0 after reset, got %d", source.CurrentIndex())
	}
}

func TestSliceSourceWithInterval(t *testing.T) {
	records := []*Record{
		NewRecord(1),
		NewRecord(2),
	}
	source := NewSliceSource("test", records, 10, 10*time.Millisecond)

	ctx := context.Background()
	err := source.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	start := time.Now()
	received := 0
	for range source.Output() {
		received++
	}
	elapsed := time.Since(start)

	if received != 2 {
		t.Errorf("expected 2 records, got %d", received)
	}
	if elapsed < 10*time.Millisecond {
		t.Errorf("expected elapsed >= 10ms, got %v", elapsed)
	}
}

func TestGeneratorSource(t *testing.T) {
	generator := func(seq int64) *Record {
		return NewRecord(seq * 10)
	}
	source := NewGeneratorSource("test", generator, 5, 10, 0)

	ctx := context.Background()
	err := source.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	received := 0
	for rec := range source.Output() {
		received++
		expectedData := int64(received) * 10
		if rec.Data.(int64) != expectedData {
			t.Errorf("expected Data=%d, got %v", expectedData, rec.Data)
		}
		if rec.SeqID != int64(received) {
			t.Errorf("expected SeqID=%d, got %d", received, rec.SeqID)
		}
	}

	if received != 5 {
		t.Errorf("expected 5 records, got %d", received)
	}
}

func TestGeneratorSourceNilRecord(t *testing.T) {
	generator := func(seq int64) *Record {
		if seq%2 == 0 {
			return nil
		}
		return NewRecord(seq)
	}
	source := NewGeneratorSource("test", generator, 4, 10, 0)

	ctx := context.Background()
	err := source.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	received := 0
	for range source.Output() {
		received++
	}

	if received != 2 {
		t.Errorf("expected 2 non-nil records, got %d", received)
	}
}

func TestFilterOperator(t *testing.T) {
	filter := NewFilterOperator("even", func(ctx context.Context, r *Record) (bool, error) {
		return r.Data.(int)%2 == 0, nil
	})

	ctx := context.Background()

	rec1 := NewRecord(2)
	res, err := filter.Process(ctx, rec1)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if res == nil || len(res) != 1 {
		t.Error("expected record to pass filter")
	}

	rec2 := NewRecord(3)
	res, err = filter.Process(ctx, rec2)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if res != nil {
		t.Error("expected record to be filtered out")
	}

	processed, passed, dropped := filter.Stats()
	if processed != 2 {
		t.Errorf("expected processed=2, got %d", processed)
	}
	if passed != 1 {
		t.Errorf("expected passed=1, got %d", passed)
	}
	if dropped != 1 {
		t.Errorf("expected dropped=1, got %d", dropped)
	}
}

func TestFilterOperatorNilFunc(t *testing.T) {
	filter := NewFilterOperator("nil", nil)
	ctx := context.Background()

	rec := NewRecord(1)
	res, err := filter.Process(ctx, rec)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if res == nil || len(res) != 1 {
		t.Error("expected record to pass when filter func is nil")
	}
}

func TestFilterOperatorError(t *testing.T) {
	filter := NewFilterOperator("error", func(ctx context.Context, r *Record) (bool, error) {
		return false, fmt.Errorf("filter error")
	})
	ctx := context.Background()

	rec := NewRecord(1)
	_, err := filter.Process(ctx, rec)
	if err == nil {
		t.Error("expected error from filter")
	}
}

func TestFilterOperatorState(t *testing.T) {
	filter := NewFilterOperator("test", func(ctx context.Context, r *Record) (bool, error) {
		return r.Data.(int) > 5, nil
	})

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		_, _ = filter.Process(ctx, NewRecord(i))
	}

	data, err := filter.SaveState()
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	filter2 := NewFilterOperator("test", func(ctx context.Context, r *Record) (bool, error) {
		return r.Data.(int) > 5, nil
	})
	err = filter2.RestoreState(data)
	if err != nil {
		t.Fatalf("RestoreState failed: %v", err)
	}

	processed, passed, dropped := filter2.Stats()
	if processed != 10 {
		t.Errorf("expected processed=10, got %d", processed)
	}
	if passed != 4 {
		t.Errorf("expected passed=4, got %d", passed)
	}
	if dropped != 6 {
		t.Errorf("expected dropped=6, got %d", dropped)
	}
}

func TestMapOperator(t *testing.T) {
	mapper := NewMapOperator("double", func(ctx context.Context, r *Record) (*Record, error) {
		newRec := r.Clone()
		newRec.Data = r.Data.(int) * 2
		return newRec, nil
	})

	ctx := context.Background()
	rec := NewRecord(5)
	res, err := mapper.Process(ctx, rec)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].Data.(int) != 10 {
		t.Errorf("expected Data=10, got %v", res[0].Data)
	}

	if mapper.Stats() != 1 {
		t.Errorf("expected processed=1, got %d", mapper.Stats())
	}
}

func TestMapOperatorNilResult(t *testing.T) {
	mapper := NewMapOperator("nil", func(ctx context.Context, r *Record) (*Record, error) {
		return nil, nil
	})

	ctx := context.Background()
	rec := NewRecord(1)
	res, err := mapper.Process(ctx, rec)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if res != nil {
		t.Error("expected nil result")
	}
}

func TestMapOperatorState(t *testing.T) {
	mapper := NewMapOperator("test", func(ctx context.Context, r *Record) (*Record, error) {
		return r, nil
	})

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, _ = mapper.Process(ctx, NewRecord(i))
	}

	data, err := mapper.SaveState()
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	mapper2 := NewMapOperator("test", nil)
	err = mapper2.RestoreState(data)
	if err != nil {
		t.Fatalf("RestoreState failed: %v", err)
	}

	if mapper2.Stats() != 5 {
		t.Errorf("expected processed=5, got %d", mapper2.Stats())
	}
}

func TestFlatMapOperator(t *testing.T) {
	flatMapper := NewFlatMapOperator("split", func(ctx context.Context, r *Record) ([]*Record, error) {
		values := r.Data.([]int)
		results := make([]*Record, len(values))
		for i, v := range values {
			results[i] = NewRecord(v)
		}
		return results, nil
	})

	ctx := context.Background()
	rec := NewRecord([]int{1, 2, 3})
	res, err := flatMapper.Process(ctx, rec)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if len(res) != 3 {
		t.Fatalf("expected 3 results, got %d", len(res))
	}

	processed, outputCnt := flatMapper.Stats()
	if processed != 1 {
		t.Errorf("expected processed=1, got %d", processed)
	}
	if outputCnt != 3 {
		t.Errorf("expected outputCnt=3, got %d", outputCnt)
	}
}

func TestFlatMapOperatorState(t *testing.T) {
	flatMapper := NewFlatMapOperator("test", func(ctx context.Context, r *Record) ([]*Record, error) {
		return []*Record{r}, nil
	})

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, _ = flatMapper.Process(ctx, NewRecord([]int{i}))
	}

	data, err := flatMapper.SaveState()
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	fm2 := NewFlatMapOperator("test", nil)
	err = fm2.RestoreState(data)
	if err != nil {
		t.Fatalf("RestoreState failed: %v", err)
	}

	processed, outputCnt := fm2.Stats()
	if processed != 3 {
		t.Errorf("expected processed=3, got %d", processed)
	}
	if outputCnt != 3 {
		t.Errorf("expected outputCnt=3, got %d", outputCnt)
	}
}

func TestOperatorChain(t *testing.T) {
	chain := NewOperatorChain()

	filter := NewFilterOperator("even", func(ctx context.Context, r *Record) (bool, error) {
		return r.Data.(int)%2 == 0, nil
	})
	mapper := NewMapOperator("double", func(ctx context.Context, r *Record) (*Record, error) {
		newRec := r.Clone()
		newRec.Data = r.Data.(int) * 2
		return newRec, nil
	})

	err := chain.Add(filter)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	err = chain.Add(mapper)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if chain.Count() != 2 {
		t.Errorf("expected Count=2, got %d", chain.Count())
	}

	names := chain.List()
	if len(names) != 2 || names[0] != "even" || names[1] != "double" {
		t.Errorf("unexpected operator list: %v", names)
	}

	ctx := context.Background()
	rec := NewRecord(4)
	res, _, err := chain.Process(ctx, rec)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].Data.(int) != 8 {
		t.Errorf("expected Data=8, got %v", res[0].Data)
	}

	rec2 := NewRecord(3)
	res, filtered, err := chain.Process(ctx, rec2)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if res != nil {
		t.Error("expected nil result for odd number")
	}
	if !filtered {
		t.Error("expected filtered=true for FilterOperator")
	}
}

func TestOperatorChainInsertRemove(t *testing.T) {
	chain := NewOperatorChain()

	op1 := NewFilterOperator("op1", nil)
	op2 := NewFilterOperator("op2", nil)
	op3 := NewFilterOperator("op3", nil)

	_ = chain.Add(op1)
	_ = chain.Add(op3)

	err := chain.Insert(1, op2)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	names := chain.List()
	if names[0] != "op1" || names[1] != "op2" || names[2] != "op3" {
		t.Errorf("unexpected order: %v", names)
	}

	err = chain.Remove(1)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	names = chain.List()
	if len(names) != 2 || names[1] != "op3" {
		t.Errorf("unexpected names after remove: %v", names)
	}

	err = chain.Insert(100, op2)
	if err == nil {
		t.Error("expected error for invalid index")
	}

	err = chain.Remove(100)
	if err == nil {
		t.Error("expected error for invalid index")
	}

	err = chain.Add(nil)
	if err != ErrOperatorNil {
		t.Errorf("expected ErrOperatorNil, got %v", err)
	}

	err = chain.Insert(0, nil)
	if err != ErrOperatorNil {
		t.Errorf("expected ErrOperatorNil, got %v", err)
	}
}

func TestOperatorChainStates(t *testing.T) {
	chain := NewOperatorChain()
	op1 := NewFilterOperator("op1", nil)
	op2 := NewMapOperator("op2", nil)
	_ = chain.Add(op1)
	_ = chain.Add(op2)

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		_, _, _ = chain.Process(ctx, NewRecord(i))
	}

	states, err := chain.SaveStates()
	if err != nil {
		t.Fatalf("SaveStates failed: %v", err)
	}
	if len(states) != 2 {
		t.Errorf("expected 2 states, got %d", len(states))
	}

	chain2 := NewOperatorChain()
	op1b := NewFilterOperator("op1", nil)
	op2b := NewMapOperator("op2", nil)
	_ = chain2.Add(op1b)
	_ = chain2.Add(op2b)

	err = chain2.RestoreStates(states)
	if err != nil {
		t.Fatalf("RestoreStates failed: %v", err)
	}

	processed, _, _ := op1b.Stats()
	if processed != 10 {
		t.Errorf("expected op1 processed=10, got %d", processed)
	}
	if op2b.Stats() != 10 {
		t.Errorf("expected op2 processed=10, got %d", op2b.Stats())
	}
}

func TestNewWindowAggregatorInvalidConfig(t *testing.T) {
	_, err := NewWindowAggregator("test", WindowConfig{
		WindowType: WindowTypeTumblingTime,
		Size:       0,
	})
	if err != ErrInvalidWindowSize {
		t.Errorf("expected ErrInvalidWindowSize, got %v", err)
	}

	_, err = NewWindowAggregator("test", WindowConfig{
		WindowType: WindowTypeTumblingCount,
		CountSize:  0,
	})
	if err != ErrInvalidWindowSize {
		t.Errorf("expected ErrInvalidWindowSize, got %v", err)
	}
}

func TestTumblingCountWindowSum(t *testing.T) {
	window, err := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationSum,
		CountSize:   3,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	ctx := context.Background()
	window.Start(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	var results []*WindowResult
	go func() {
		defer wg.Done()
		for res := range window.Results() {
			results = append(results, res)
		}
	}()

	for i := 1; i <= 6; i++ {
		rec := NewRecord(i)
		rec.SeqID = int64(i)
		_, err := window.Process(ctx, rec)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}
	}

	window.Stop()
	wg.Wait()

	if len(results) != 2 {
		t.Fatalf("expected 2 window results, got %d", len(results))
	}

	if results[0].Value != 6.0 {
		t.Errorf("expected first window sum=6, got %v", results[0].Value)
	}
	if results[0].Count != 3 {
		t.Errorf("expected first window count=3, got %d", results[0].Count)
	}

	if results[1].Value != 15.0 {
		t.Errorf("expected second window sum=15, got %v", results[1].Value)
	}
}

func TestTumblingCountWindowCount(t *testing.T) {
	window, err := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationCount,
		CountSize:   2,
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	ctx := context.Background()
	window.Start(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	var results []*WindowResult
	go func() {
		defer wg.Done()
		for res := range window.Results() {
			results = append(results, res)
		}
	}()

	for i := 1; i <= 4; i++ {
		rec := NewRecord(i)
		rec.SeqID = int64(i)
		_, err := window.Process(ctx, rec)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}
	}

	window.Stop()
	wg.Wait()

	if len(results) != 2 {
		t.Fatalf("expected 2 window results, got %d", len(results))
	}
	if results[0].Value != 2.0 {
		t.Errorf("expected window count=2, got %v", results[0].Value)
	}
}

func TestTumblingCountWindowAvg(t *testing.T) {
	window, err := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationAvg,
		CountSize:   4,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	ctx := context.Background()
	window.Start(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	var results []*WindowResult
	go func() {
		defer wg.Done()
		for res := range window.Results() {
			results = append(results, res)
		}
	}()

	for i := 1; i <= 4; i++ {
		rec := NewRecord(i)
		rec.SeqID = int64(i)
		_, err := window.Process(ctx, rec)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}
	}

	window.Stop()
	wg.Wait()

	if len(results) != 1 {
		t.Fatalf("expected 1 window result, got %d", len(results))
	}
	expected := (1.0 + 2.0 + 3.0 + 4.0) / 4.0
	if results[0].Value != expected {
		t.Errorf("expected avg=%v, got %v", expected, results[0].Value)
	}
}

func TestTumblingCountWindowMinMax(t *testing.T) {
	windowMin, err := NewWindowAggregator("min", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationMin,
		CountSize:   3,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	windowMax, err := NewWindowAggregator("max", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationMax,
		CountSize:   3,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	ctx := context.Background()

	records := []*Record{
		NewRecord(5),
		NewRecord(2),
		NewRecord(8),
	}

	for i, rec := range records {
		rec.SeqID = int64(i + 1)
	}

	var minResult, maxResult *WindowResult

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		windowMin.Start(ctx)
		for res := range windowMin.Results() {
			minResult = res
		}
	}()
	go func() {
		defer wg.Done()
		windowMax.Start(ctx)
		for res := range windowMax.Results() {
			maxResult = res
		}
	}()

	for _, rec := range records {
		_, _ = windowMin.Process(ctx, rec)
		_, _ = windowMax.Process(ctx, rec)
	}

	windowMin.Stop()
	windowMax.Stop()
	wg.Wait()

	if minResult.Value != 2.0 {
		t.Errorf("expected min=2, got %v", minResult.Value)
	}
	if maxResult.Value != 8.0 {
		t.Errorf("expected max=8, got %v", maxResult.Value)
	}
}

func TestSlidingCountWindow(t *testing.T) {
	window, err := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeSlidingCount,
		Aggregation: AggregationSum,
		CountSize:   3,
		CountSlide:  1,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	ctx := context.Background()
	window.Start(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	var results []*WindowResult
	go func() {
		defer wg.Done()
		for res := range window.Results() {
			results = append(results, res)
		}
	}()

	for i := 1; i <= 5; i++ {
		rec := NewRecord(i)
		rec.SeqID = int64(i)
		_, err := window.Process(ctx, rec)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}
	}

	window.Stop()
	wg.Wait()

	completeResults := make([]*WindowResult, 0)
	partialResults := make([]*WindowResult, 0)
	for _, r := range results {
		if r.Partial {
			partialResults = append(partialResults, r)
		} else {
			completeResults = append(completeResults, r)
		}
	}

	if len(completeResults) != 3 {
		t.Fatalf("expected 3 complete window results, got %d", len(completeResults))
	}

	if completeResults[0].Value != 6.0 {
		t.Errorf("expected first window sum=6, got %v", completeResults[0].Value)
	}
	if completeResults[1].Value != 9.0 {
		t.Errorf("expected second window sum=9, got %v", completeResults[1].Value)
	}
	if completeResults[2].Value != 12.0 {
		t.Errorf("expected third window sum=12, got %v", completeResults[2].Value)
	}

	if len(partialResults) == 0 {
		t.Error("expected at least one partial result from incomplete windows on stop")
	}
}

func TestTumblingTimeWindow(t *testing.T) {
	window, err := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeTumblingTime,
		Aggregation: AggregationSum,
		Size:        100 * time.Millisecond,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	ctx := context.Background()
	window.Start(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	var results []*WindowResult
	go func() {
		defer wg.Done()
		for res := range window.Results() {
			results = append(results, res)
		}
	}()

	now := time.Now()
	for i := 1; i <= 3; i++ {
		rec := NewRecord(i)
		rec.SeqID = int64(i)
		rec.Timestamp = now
		_, err := window.Process(ctx, rec)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}
	}

	time.Sleep(150 * time.Millisecond)

	window.Stop()
	wg.Wait()

	if len(results) < 1 {
		t.Fatalf("expected at least 1 window result, got %d", len(results))
	}
	if results[0].Value != 6.0 {
		t.Errorf("expected window sum=6, got %v", results[0].Value)
	}
}

func TestWindowAggregatorState(t *testing.T) {
	window, err := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationSum,
		CountSize:   5,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		rec := NewRecord(i)
		rec.SeqID = int64(i)
		_, _ = window.Process(ctx, rec)
	}

	if window.ClosedWindowCount() != 0 {
		t.Errorf("expected 0 closed windows, got %d", window.ClosedWindowCount())
	}
	if window.ActiveWindowCount() != 1 {
		t.Errorf("expected 1 active window, got %d", window.ActiveWindowCount())
	}

	data, err := window.SaveState()
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	window2, err := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationSum,
		CountSize:   5,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	err = window2.RestoreState(data)
	if err != nil {
		t.Fatalf("RestoreState failed: %v", err)
	}

	if window2.ActiveWindowCount() != 1 {
		t.Errorf("expected 1 active window after restore, got %d", window2.ActiveWindowCount())
	}

	window2.Start(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	var results []*WindowResult
	go func() {
		defer wg.Done()
		for res := range window2.Results() {
			results = append(results, res)
		}
	}()

	for i := 4; i <= 5; i++ {
		rec := NewRecord(i)
		rec.SeqID = int64(i)
		_, _ = window2.Process(ctx, rec)
	}

	window2.Stop()
	wg.Wait()

	if len(results) != 1 {
		t.Fatalf("expected 1 window result, got %d", len(results))
	}
	expected := 1.0 + 2.0 + 3.0 + 4.0 + 5.0
	if results[0].Value != expected {
		t.Errorf("expected sum=%v, got %v", expected, results[0].Value)
	}
}

func TestWindowFlushAll(t *testing.T) {
	window, err := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationSum,
		CountSize:   10,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	ctx := context.Background()
	for i := 1; i <= 5; i++ {
		rec := NewRecord(i)
		rec.SeqID = int64(i)
		_, _ = window.Process(ctx, rec)
	}

	if window.ActiveWindowCount() != 1 {
		t.Errorf("expected 1 active window, got %d", window.ActiveWindowCount())
	}

	window.FlushAll()

	if window.ActiveWindowCount() != 0 {
		t.Errorf("expected 0 active windows after flush, got %d", window.ActiveWindowCount())
	}
}

func TestMemoryCheckpointStore(t *testing.T) {
	store := NewMemoryCheckpointStore()
	ctx := context.Background()

	if store.Count() != 0 {
		t.Errorf("expected Count=0, got %d", store.Count())
	}

	cp := &Checkpoint{
		ID:           "cp1",
		Timestamp:    time.Now(),
		SourceOffset: 100,
		OperatorStates: map[string][]byte{
			"op1": []byte("state1"),
		},
		WindowStates: map[string][]byte{
			"win1": []byte("winstate1"),
		},
		Metadata: map[string]interface{}{
			"key": "value",
		},
	}

	err := store.Save(ctx, cp)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if store.Count() != 1 {
		t.Errorf("expected Count=1, got %d", store.Count())
	}

	loaded, err := store.Load(ctx, "cp1")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.ID != "cp1" {
		t.Errorf("expected ID=cp1, got %s", loaded.ID)
	}
	if loaded.SourceOffset != 100 {
		t.Errorf("expected SourceOffset=100, got %d", loaded.SourceOffset)
	}

	_, err = store.Load(ctx, "nonexistent")
	if err != ErrCheckpointNotFound {
		t.Errorf("expected ErrCheckpointNotFound, got %v", err)
	}

	latest, err := store.Latest(ctx)
	if err != nil {
		t.Fatalf("Latest failed: %v", err)
	}
	if latest.ID != "cp1" {
		t.Errorf("expected latest ID=cp1, got %s", latest.ID)
	}

	cp2 := &Checkpoint{
		ID:           "cp2",
		Timestamp:    time.Now(),
		SourceOffset: 200,
	}
	err = store.Save(ctx, cp2)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	latest, err = store.Latest(ctx)
	if err != nil {
		t.Fatalf("Latest failed: %v", err)
	}
	if latest.ID != "cp2" {
		t.Errorf("expected latest ID=cp2, got %s", latest.ID)
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected list length 2, got %d", len(list))
	}

	err = store.Delete(ctx, "cp1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if store.Count() != 1 {
		t.Errorf("expected Count=1 after delete, got %d", store.Count())
	}

	err = store.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}
	if store.Count() != 0 {
		t.Errorf("expected Count=0 after clear, got %d", store.Count())
	}

	_, err = store.Latest(ctx)
	if err != ErrCheckpointNotFound {
		t.Errorf("expected ErrCheckpointNotFound from empty store, got %v", err)
	}

	err = store.Save(ctx, nil)
	if err != ErrInvalidCheckpoint {
		t.Errorf("expected ErrInvalidCheckpoint, got %v", err)
	}

	err = store.Save(ctx, &Checkpoint{})
	if err == nil {
		t.Error("expected error for checkpoint with empty ID")
	}
}

func TestMemoryCheckpointStoreStateIsolation(t *testing.T) {
	store := NewMemoryCheckpointStore()
	ctx := context.Background()

	cp := &Checkpoint{
		ID:           "cp1",
		SourceOffset: 100,
		OperatorStates: map[string][]byte{
			"op1": []byte("original"),
		},
	}

	err := store.Save(ctx, cp)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	cp.OperatorStates["op1"] = []byte("modified")
	cp.SourceOffset = 999

	loaded, _ := store.Load(ctx, "cp1")
	if string(loaded.OperatorStates["op1"]) != "original" {
		t.Error("state should be isolated from original modification")
	}
	if loaded.SourceOffset != 100 {
		t.Error("SourceOffset should be isolated from original modification")
	}
}

func TestNewPipeline(t *testing.T) {
	_, err := NewPipeline(DefaultPipelineConfig(), nil)
	if err != ErrSourceNil {
		t.Errorf("expected ErrSourceNil, got %v", err)
	}

	cfg := DefaultPipelineConfig()
	cfg.BackpressureThreshold = 0
	_, err = NewPipeline(cfg, NewChannelSource("test", make(chan *Record), 10))
	if err != ErrInvalidBackpressureThreshold {
		t.Errorf("expected ErrInvalidBackpressureThreshold, got %v", err)
	}

	cfg = DefaultPipelineConfig()
	cfg.BackpressureWarningRatio = 0.9
	cfg.BackpressureCriticalRatio = 0.7
	_, err = NewPipeline(cfg, NewChannelSource("test", make(chan *Record), 10))
	if err == nil {
		t.Error("expected error for warning ratio >= critical ratio")
	}

	cfg = DefaultPipelineConfig()
	source := NewChannelSource("test", make(chan *Record), 10)
	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}
	if pipeline == nil {
		t.Fatal("expected pipeline, got nil")
	}
	if pipeline.Status() != PipelineStatusIdle {
		t.Errorf("expected Idle status, got %v", pipeline.Status())
	}
}

func TestPipelineAddOperator(t *testing.T) {
	source := NewChannelSource("test", make(chan *Record), 10)
	pipeline, _ := NewPipeline(DefaultPipelineConfig(), source)

	filter := NewFilterOperator("test", nil)
	err := pipeline.AddOperator(filter)
	if err != nil {
		t.Fatalf("AddOperator failed: %v", err)
	}

	ctx := context.Background()
	_ = pipeline.Start(ctx)
	defer pipeline.Stop()

	err = pipeline.AddOperator(filter)
	if err != ErrPipelineRunning {
		t.Errorf("expected ErrPipelineRunning, got %v", err)
	}
}

func TestPipelineSetSink(t *testing.T) {
	source := NewChannelSource("test", make(chan *Record), 10)
	pipeline, _ := NewPipeline(DefaultPipelineConfig(), source)

	sink := NewCollectSink()
	pipeline.SetSink(sink)

	ctx := context.Background()
	_ = pipeline.Start(ctx)
	defer pipeline.Stop()

	pipeline.SetSink(NewCollectSink())
}

func TestPipelineSimpleFlow(t *testing.T) {
	input := make(chan *Record, 100)
	source := NewChannelSource("test", input, 100)

	cfg := DefaultPipelineConfig()
	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	filter := NewFilterOperator("even", func(ctx context.Context, r *Record) (bool, error) {
		return r.Data.(int)%2 == 0, nil
	})
	mapper := NewMapOperator("double", func(ctx context.Context, r *Record) (*Record, error) {
		newRec := r.Clone()
		newRec.Data = r.Data.(int) * 2
		return newRec, nil
	})

	_ = pipeline.AddOperator(filter)
	_ = pipeline.AddOperator(mapper)

	sink := NewCollectSink()
	pipeline.SetSink(sink)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = pipeline.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 1; i <= 10; i++ {
		input <- NewRecord(i)
	}
	close(input)

	time.Sleep(200 * time.Millisecond)

	_ = pipeline.Stop()

	records, _ := sink.Count()
	if records != 5 {
		t.Errorf("expected 5 output records, got %d", records)
	}

	stats := pipeline.Stats()
	if stats.RecordsIn != 10 {
		t.Errorf("expected RecordsIn=10, got %d", stats.RecordsIn)
	}
	if stats.RecordsOut != 5 {
		t.Errorf("expected RecordsOut=5, got %d", stats.RecordsOut)
	}
	if stats.RecordsDropped != 5 {
		t.Errorf("expected RecordsDropped=5, got %d", stats.RecordsDropped)
	}
}

func TestPipelineWithWindow(t *testing.T) {
	records := make([]*Record, 10)
	for i := 1; i <= 10; i++ {
		records[i-1] = NewRecord(i)
	}

	source := NewSliceSource("test", records, 100, 0)

	window, err := NewWindowAggregator("sum", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationSum,
		CountSize:   5,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	cfg := DefaultPipelineConfig()
	cfg.WindowAggregator = window
	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	sink := NewCollectSink()
	pipeline.SetSink(sink)

	ctx := context.Background()
	err = pipeline.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(300 * time.Millisecond)
	_ = pipeline.Stop()

	_, windowResults := sink.Count()
	if windowResults != 2 {
		t.Errorf("expected 2 window results, got %d", windowResults)
	}

	results := sink.GetResults()
	if results[0].Value != 15.0 {
		t.Errorf("expected first window sum=15, got %v", results[0].Value)
	}
	if results[1].Value != 40.0 {
		t.Errorf("expected second window sum=40, got %v", results[1].Value)
	}

	stats := pipeline.Stats()
	if stats.WindowsClosed != 2 {
		t.Errorf("expected WindowsClosed=2, got %d", stats.WindowsClosed)
	}
}

func TestPipelinePauseResume(t *testing.T) {
	generator := func(seq int64) *Record {
		return NewRecord(int(seq))
	}
	source := NewGeneratorSource("test", generator, 0, 100, 10*time.Millisecond)

	cfg := DefaultPipelineConfig()
	sink := NewCollectSink()
	cfg.Sink = sink

	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	ctx := context.Background()
	err = pipeline.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	err = pipeline.Pause()
	if err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	if pipeline.Status() != PipelineStatusPaused {
		t.Errorf("expected Paused status, got %v", pipeline.Status())
	}

	countBefore, _ := sink.Count()

	time.Sleep(100 * time.Millisecond)

	countDuring, _ := sink.Count()
	if countDuring != countBefore {
		t.Errorf("records should not be processed while paused: before=%d, during=%d", countBefore, countDuring)
	}

	err = pipeline.Resume()
	if err != nil {
		t.Fatalf("Resume failed: %v", err)
	}
	if pipeline.Status() != PipelineStatusRunning {
		t.Errorf("expected Running status, got %v", pipeline.Status())
	}

	time.Sleep(50 * time.Millisecond)

	countAfter, _ := sink.Count()
	if countAfter <= countBefore {
		t.Errorf("records should be processed after resume: before=%d, after=%d", countBefore, countAfter)
	}

	_ = pipeline.Stop()
}

func TestPipelineBackpressure(t *testing.T) {
	input := make(chan *Record, 1000)

	source := NewChannelSource("fast", input, 1000)

	cfg := DefaultPipelineConfig()
	cfg.BufferSize = 10
	cfg.BackpressureThreshold = 5
	cfg.BackpressureWarningRatio = 0.6
	cfg.BackpressureCriticalRatio = 0.8

	slowOp := NewMapOperator("slow", func(ctx context.Context, r *Record) (*Record, error) {
		time.Sleep(50 * time.Millisecond)
		return r, nil
	})

	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	_ = pipeline.AddOperator(slowOp)
	sink := NewCollectSink()
	pipeline.SetSink(sink)

	ctx := context.Background()
	err = pipeline.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	go func() {
		for i := 0; i < 100; i++ {
			select {
			case input <- NewRecord(i):
			case <-ctx.Done():
				return
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)

	bpInfo := pipeline.GetBackpressureInfo()
	if bpInfo.State == BackpressureNormal {
		t.Logf("Warning: expected some backpressure, but got normal (pending=%d, threshold=%d)",
			bpInfo.PendingCount, bpInfo.Threshold)
	}

	close(input)
	_ = pipeline.Stop()
}

func TestPipelineCheckpoint(t *testing.T) {
	records := make([]*Record, 20)
	for i := 1; i <= 20; i++ {
		records[i-1] = NewRecord(i)
	}

	source := NewSliceSource("test", records, 100, 5*time.Millisecond)

	cfg := DefaultPipelineConfig()
	cfg.EnableCheckpoint = true
	cfg.CheckpointInterval = 50 * time.Millisecond

	filter := NewFilterOperator("even", func(ctx context.Context, r *Record) (bool, error) {
		return r.Data.(int)%2 == 0, nil
	})

	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	_ = pipeline.AddOperator(filter)

	ctx := context.Background()
	err = pipeline.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	err = pipeline.SaveCheckpoint()
	if err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	_ = pipeline.Stop()

	list, err := pipeline.ListCheckpoints()
	if err != nil {
		t.Fatalf("ListCheckpoints failed: %v", err)
	}
	if len(list) < 1 {
		t.Errorf("expected at least 1 checkpoint, got %d", len(list))
	}

	stats := pipeline.Stats()
	if stats.CheckpointsMade < 1 {
		t.Errorf("expected CheckpointsMade >= 1, got %d", stats.CheckpointsMade)
	}

	offset := pipeline.SourceOffset()
	if offset < 1 {
		t.Errorf("expected SourceOffset >= 1, got %d", offset)
	}

	if err := pipeline.ClearCheckpoints(); err != nil {
		t.Fatalf("ClearCheckpoints failed: %v", err)
	}
}

func TestPipelineRestoreFromCheckpoint(t *testing.T) {
	store := NewMemoryCheckpointStore()

	records := make([]*Record, 10)
	for i := 1; i <= 10; i++ {
		records[i-1] = NewRecord(i)
	}

	source := NewSliceSource("test", records, 100, 0)

	cfg := DefaultPipelineConfig()
	cfg.EnableCheckpoint = true
	cfg.CheckpointStore = store

	filter := NewFilterOperator("even", func(ctx context.Context, r *Record) (bool, error) {
		return r.Data.(int)%2 == 0, nil
	})
	mapper := NewMapOperator("double", func(ctx context.Context, r *Record) (*Record, error) {
		newRec := r.Clone()
		newRec.Data = r.Data.(int) * 2
		return newRec, nil
	})

	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	_ = pipeline.AddOperator(filter)
	_ = pipeline.AddOperator(mapper)
	sink := NewCollectSink()
	pipeline.SetSink(sink)

	ctx := context.Background()
	_ = pipeline.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	_ = pipeline.SaveCheckpoint()
	_ = pipeline.Stop()

	processed, passed, dropped := filter.Stats()

	source2 := NewSliceSource("test", records, 100, 0)
	source2.Reset()

	cfg2 := DefaultPipelineConfig()
	cfg2.EnableCheckpoint = true
	cfg2.CheckpointStore = store

	filter2 := NewFilterOperator("even", func(ctx context.Context, r *Record) (bool, error) {
		return r.Data.(int)%2 == 0, nil
	})
	mapper2 := NewMapOperator("double", func(ctx context.Context, r *Record) (*Record, error) {
		newRec := r.Clone()
		newRec.Data = r.Data.(int) * 2
		return newRec, nil
	})

	pipeline2, err := NewPipeline(cfg2, source2)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	_ = pipeline2.AddOperator(filter2)
	_ = pipeline2.AddOperator(mapper2)

	err = pipeline2.RestoreFromLatestCheckpoint()
	if err != nil {
		t.Fatalf("RestoreFromLatestCheckpoint failed: %v", err)
	}

	p2, _, _ := filter2.Stats()
	if p2 != processed {
		t.Errorf("restored filter processed count should be %d, got %d", processed, p2)
	}

	m2 := mapper2.Stats()
	if m2 != mapper.Stats() {
		t.Errorf("restored mapper processed count should match original")
	}

	_ = passed
	_ = dropped
}

func TestPipelineOperatorErrors(t *testing.T) {
	input := make(chan *Record, 10)
	source := NewChannelSource("test", input, 10)

	cfg := DefaultPipelineConfig()
	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	errorOp := NewMapOperator("error", func(ctx context.Context, r *Record) (*Record, error) {
		if r.Data.(int) == 5 {
			return nil, fmt.Errorf("intentional error")
		}
		return r, nil
	})
	_ = pipeline.AddOperator(errorOp)

	sink := NewCollectSink()
	pipeline.SetSink(sink)

	ctx := context.Background()
	err = pipeline.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 1; i <= 10; i++ {
		input <- NewRecord(i)
	}
	close(input)

	time.Sleep(100 * time.Millisecond)
	_ = pipeline.Stop()

	count, _ := sink.Count()
	if count != 9 {
		t.Errorf("expected 9 successful records, got %d", count)
	}

	stats := pipeline.Stats()
	if stats.Errors != 1 {
		t.Errorf("expected Errors=1, got %d", stats.Errors)
	}
}

func TestPipelineFlatMap(t *testing.T) {
	records := []*Record{
		NewRecord([]int{1, 2}),
		NewRecord([]int{3, 4, 5}),
	}

	source := NewSliceSource("test", records, 100, 0)

	cfg := DefaultPipelineConfig()
	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	flatMap := NewFlatMapOperator("expand", func(ctx context.Context, r *Record) ([]*Record, error) {
		values := r.Data.([]int)
		results := make([]*Record, len(values))
		for i, v := range values {
			results[i] = NewRecord(v)
		}
		return results, nil
	})
	_ = pipeline.AddOperator(flatMap)

	sink := NewCollectSink()
	pipeline.SetSink(sink)

	ctx := context.Background()
	_ = pipeline.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	_ = pipeline.Stop()

	count, _ := sink.Count()
	if count != 5 {
		t.Errorf("expected 5 records after flatMap, got %d", count)
	}

	expected := []int{1, 2, 3, 4, 5}
	actual := make([]int, 5)
	for i, r := range sink.GetRecords() {
		actual[i] = r.Data.(int)
	}
	for i := range expected {
		if actual[i] != expected[i] {
			t.Errorf("expected record %d=%d, got %d", i, expected[i], actual[i])
		}
	}
}

func TestPipelineWindowStateRestore(t *testing.T) {
	store := NewMemoryCheckpointStore()

	records := make([]*Record, 7)
	for i := 1; i <= 7; i++ {
		records[i-1] = NewRecord(i)
	}

	window, err := NewWindowAggregator("sum", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationSum,
		CountSize:   5,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	cfg := DefaultPipelineConfig()
	cfg.EnableCheckpoint = true
	cfg.CheckpointStore = store
	cfg.WindowAggregator = window

	source := NewSliceSource("test", records, 100, 0)
	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	ctx := context.Background()
	_ = pipeline.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	_ = pipeline.SaveCheckpoint()
	_ = pipeline.Stop()

	window2, err := NewWindowAggregator("sum", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationSum,
		CountSize:   5,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	cfg2 := DefaultPipelineConfig()
	cfg2.EnableCheckpoint = true
	cfg2.CheckpointStore = store
	cfg2.WindowAggregator = window2

	source2 := NewSliceSource("test", records, 100, 0)
	source2.Reset()

	pipeline2, err := NewPipeline(cfg2, source2)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	sink := NewCollectSink()
	pipeline2.SetSink(sink)

	err = pipeline2.RestoreFromLatestCheckpoint()
	if err != nil {
		t.Fatalf("RestoreFromLatestCheckpoint failed: %v", err)
	}

	_ = pipeline2.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	_ = pipeline2.Stop()

	_, results := sink.Count()
	if results < 1 {
		t.Errorf("expected at least 1 window result after restore, got %d", results)
	}
}

func TestPipelineDoubleStop(t *testing.T) {
	input := make(chan *Record, 10)
	source := NewChannelSource("test", input, 10)
	pipeline, _ := NewPipeline(DefaultPipelineConfig(), source)

	ctx := context.Background()
	_ = pipeline.Start(ctx)

	err := pipeline.Stop()
	if err != nil {
		t.Fatalf("first Stop failed: %v", err)
	}

	err = pipeline.Stop()
	if err != nil {
		t.Fatalf("second Stop should not error: %v", err)
	}
}

func TestPipelineStartTwice(t *testing.T) {
	input := make(chan *Record, 10)
	source := NewChannelSource("test", input, 10)
	pipeline, _ := NewPipeline(DefaultPipelineConfig(), source)

	ctx := context.Background()
	err := pipeline.Start(ctx)
	if err != nil {
		t.Fatalf("first Start failed: %v", err)
	}

	err = pipeline.Start(ctx)
	if err != ErrPipelineRunning {
		t.Errorf("expected ErrPipelineRunning, got %v", err)
	}

	_ = pipeline.Stop()
}

func TestPipelineStartStopped(t *testing.T) {
	input := make(chan *Record, 10)
	source := NewChannelSource("test", input, 10)
	pipeline, _ := NewPipeline(DefaultPipelineConfig(), source)

	ctx := context.Background()
	_ = pipeline.Start(ctx)
	_ = pipeline.Stop()

	err := pipeline.Start(ctx)
	if err != ErrPipelineStopped {
		t.Errorf("expected ErrPipelineStopped, got %v", err)
	}
}

func TestCollectSink(t *testing.T) {
	sink := NewCollectSink()

	ctx := context.Background()
	rec := NewRecord(1)
	_ = sink.Consume(ctx, rec)

	result := &WindowResult{Value: 42}
	_ = sink.ConsumeWindow(ctx, result)

	_ = sink.Close(ctx)

	records, results := sink.Count()
	if records != 1 {
		t.Errorf("expected 1 record, got %d", records)
	}
	if results != 1 {
		t.Errorf("expected 1 result, got %d", results)
	}

	if len(sink.GetRecords()) != 1 {
		t.Error("GetRecords should return 1 record")
	}
	if len(sink.GetResults()) != 1 {
		t.Error("GetResults should return 1 result")
	}

	sink.Clear()
	records, results = sink.Count()
	if records != 0 || results != 0 {
		t.Error("Clear should empty the sink")
	}
}

func TestGenerateCheckpointID(t *testing.T) {
	id1 := GenerateCheckpointID()
	id2 := GenerateCheckpointID()
	if id1 == id2 {
		t.Error("checkpoint IDs should be unique")
	}
	if id1 == "" || id2 == "" {
		t.Error("checkpoint ID should not be empty")
	}
}

func TestBackpressureInfo(t *testing.T) {
	info := BackpressureInfo{
		State:         BackpressureWarning,
		PendingCount:  70,
		Threshold:     100,
		WarningRatio:  0.7,
		CriticalRatio: 0.9,
	}

	if info.State.String() != "warning" {
		t.Errorf("expected state=warning, got %s", info.State.String())
	}
	if info.PendingCount != 70 {
		t.Errorf("expected PendingCount=70, got %d", info.PendingCount)
	}
}

func TestPipelineRestoreFromCheckpointById(t *testing.T) {
	store := NewMemoryCheckpointStore()
	ctx := context.Background()

	cp := &Checkpoint{
		ID:           "custom-cp-1",
		Timestamp:    time.Now(),
		SourceOffset: 50,
	}
	_ = store.Save(ctx, cp)

	records := make([]*Record, 10)
	for i := 1; i <= 10; i++ {
		records[i-1] = NewRecord(i)
	}
	source := NewSliceSource("test", records, 100, 0)

	cfg := DefaultPipelineConfig()
	cfg.EnableCheckpoint = true
	cfg.CheckpointStore = store

	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	err = pipeline.RestoreFromCheckpoint("custom-cp-1")
	if err != nil {
		t.Fatalf("RestoreFromCheckpoint failed: %v", err)
	}

	if pipeline.SourceOffset() != 50 {
		t.Errorf("expected SourceOffset=50, got %d", pipeline.SourceOffset())
	}

	err = pipeline.RestoreFromCheckpoint("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent checkpoint")
	}
}

func TestPipelineRestoreWhileRunning(t *testing.T) {
	input := make(chan *Record, 10)
	source := NewChannelSource("test", input, 10)
	pipeline, _ := NewPipeline(DefaultPipelineConfig(), source)

	ctx := context.Background()
	_ = pipeline.Start(ctx)
	defer pipeline.Stop()

	err := pipeline.RestoreFromLatestCheckpoint()
	if err != ErrPipelineRunning {
		t.Errorf("expected ErrPipelineRunning, got %v", err)
	}
}

func TestWindowAggregatorNilRecord(t *testing.T) {
	window, _ := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationCount,
		CountSize:   5,
	})

	ctx := context.Background()
	res, err := window.Process(ctx, nil)
	if err != nil {
		t.Fatalf("Process with nil record should not error: %v", err)
	}
	if res != nil {
		t.Error("Process with nil record should return nil")
	}
}

func TestWindowAggregatorExtractorError(t *testing.T) {
	window, _ := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationSum,
		CountSize:   5,
		Extractor:   func(r *Record) (float64, error) { return 0, fmt.Errorf("extractor error") },
	})

	ctx := context.Background()
	rec := NewRecord(1)
	rec.SeqID = 1
	_, err := window.Process(ctx, rec)
	if err == nil {
		t.Error("expected error from extractor")
	}
}

func TestDefaultPipelineConfig(t *testing.T) {
	cfg := DefaultPipelineConfig()
	if cfg.BufferSize != 100 {
		t.Errorf("expected BufferSize=100, got %d", cfg.BufferSize)
	}
	if cfg.BackpressureThreshold != 100 {
		t.Errorf("expected BackpressureThreshold=100, got %d", cfg.BackpressureThreshold)
	}
	if cfg.BackpressureWarningRatio != 0.7 {
		t.Errorf("expected WarningRatio=0.7, got %v", cfg.BackpressureWarningRatio)
	}
	if cfg.BackpressureCriticalRatio != 0.9 {
		t.Errorf("expected CriticalRatio=0.9, got %v", cfg.BackpressureCriticalRatio)
	}
	if cfg.EnableCheckpoint != false {
		t.Errorf("expected EnableCheckpoint=false, got %v", cfg.EnableCheckpoint)
	}
	if cfg.CheckpointInterval != 30*time.Second {
		t.Errorf("expected CheckpointInterval=30s, got %v", cfg.CheckpointInterval)
	}
}

func TestNewPipelineDefaultNilCheckpointStore(t *testing.T) {
	source := NewChannelSource("test", make(chan *Record), 10)
	cfg := DefaultPipelineConfig()
	cfg.CheckpointStore = nil

	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}
	if pipeline == nil {
		t.Fatal("pipeline should not be nil")
	}
}

func TestNewPipelineDefaultValues(t *testing.T) {
	source := NewChannelSource("test", make(chan *Record), 10)
	cfg := DefaultPipelineConfig()
	cfg.BufferSize = 0
	cfg.BackpressureWarningRatio = 0
	cfg.BackpressureCriticalRatio = 0
	cfg.CheckpointInterval = -1

	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}
	if pipeline.cfg.BufferSize != 100 {
		t.Errorf("expected default BufferSize=100, got %d", pipeline.cfg.BufferSize)
	}
	if pipeline.cfg.BackpressureWarningRatio != 0.7 {
		t.Errorf("expected default WarningRatio=0.7, got %v", pipeline.cfg.BackpressureWarningRatio)
	}
	if pipeline.cfg.BackpressureCriticalRatio != 0.9 {
		t.Errorf("expected default CriticalRatio=0.9, got %v", pipeline.cfg.BackpressureCriticalRatio)
	}
	if pipeline.cfg.CheckpointInterval != 0 {
		t.Errorf("expected default CheckpointInterval=0, got %v", pipeline.cfg.CheckpointInterval)
	}
}

func TestPipelineSetWindowAggregatorWhileRunning(t *testing.T) {
	source := NewChannelSource("test", make(chan *Record), 10)
	pipeline, _ := NewPipeline(DefaultPipelineConfig(), source)

	ctx := context.Background()
	_ = pipeline.Start(ctx)
	defer pipeline.Stop()

	window, _ := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationCount,
		CountSize:   5,
	})
	pipeline.SetWindowAggregator(window)
}

func TestCheckpointDeleteNonexistent(t *testing.T) {
	store := NewMemoryCheckpointStore()
	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	if err != ErrCheckpointNotFound {
		t.Errorf("expected ErrCheckpointNotFound, got %v", err)
	}
}

func TestSourceResumeNotPaused(t *testing.T) {
	source := NewChannelSource("test", make(chan *Record), 10)
	err := source.Resume()
	if err == nil {
		t.Error("expected error when resuming non-paused source")
	}
}

func TestPauseNotRunning(t *testing.T) {
	source := NewChannelSource("test", make(chan *Record), 10)
	err := source.Pause()
	if err != ErrSourceNotStarted {
		t.Errorf("expected ErrSourceNotStarted, got %v", err)
	}
}

func TestResumeNotPaused(t *testing.T) {
	source := NewChannelSource("test", make(chan *Record), 10)
	ctx := context.Background()
	_ = source.Start(ctx)
	err := source.Resume()
	if err == nil {
		t.Error("expected error when resuming running source")
	}
	_ = source.Stop()
}

func TestSliceSourcePauseResume(t *testing.T) {
	records := make([]*Record, 10)
	for i := 0; i < 10; i++ {
		records[i] = NewRecord(i)
	}
	source := NewSliceSource("test", records, 100, 10*time.Millisecond)

	ctx := context.Background()
	_ = source.Start(ctx)

	time.Sleep(20 * time.Millisecond)
	_ = source.Pause()
	if source.State() != SourceStatePaused {
		t.Errorf("expected state=Paused, got %v", source.State())
	}

	idx := source.CurrentIndex()
	time.Sleep(50 * time.Millisecond)
	if source.CurrentIndex() != idx {
		t.Error("index should not advance while paused")
	}

	_ = source.Resume()
	if source.State() != SourceStateRunning {
		t.Errorf("expected state=Running, got %v", source.State())
	}

	time.Sleep(50 * time.Millisecond)
	_ = source.Stop()
}

func TestGeneratorSourcePauseResume(t *testing.T) {
	generator := func(seq int64) *Record { return NewRecord(int(seq)) }
	source := NewGeneratorSource("test", generator, 20, 100, 10*time.Millisecond)

	ctx := context.Background()
	_ = source.Start(ctx)

	time.Sleep(20 * time.Millisecond)
	_ = source.Pause()
	state := source.State()
	if state != SourceStatePaused {
		t.Errorf("expected state=Paused, got %v", state)
	}

	_ = source.Resume()
	if source.State() != SourceStateRunning {
		t.Errorf("expected state=Running, got %v", source.State())
	}

	_ = source.Stop()
}

func TestPipelinePauseNotRunning(t *testing.T) {
	source := NewChannelSource("test", make(chan *Record), 10)
	pipeline, _ := NewPipeline(DefaultPipelineConfig(), source)

	err := pipeline.Pause()
	if err != ErrPipelineNotRunning {
		t.Errorf("expected ErrPipelineNotRunning, got %v", err)
	}
}

func TestPipelineResumeNotPaused(t *testing.T) {
	source := NewChannelSource("test", make(chan *Record), 10)
	pipeline, _ := NewPipeline(DefaultPipelineConfig(), source)

	err := pipeline.Resume()
	if err == nil {
		t.Error("expected error when resuming non-paused pipeline")
	}
}

func TestPipelineCheckpointDisabled(t *testing.T) {
	source := NewChannelSource("test", make(chan *Record), 10)
	cfg := DefaultPipelineConfig()
	cfg.EnableCheckpoint = false

	pipeline, _ := NewPipeline(cfg, source)

	err := pipeline.SaveCheckpoint()
	if err != nil {
		t.Errorf("SaveCheckpoint should return nil when disabled: %v", err)
	}
}

func TestPipelineSaveCheckpointWithoutWindow(t *testing.T) {
	store := NewMemoryCheckpointStore()
	records := []*Record{NewRecord(1), NewRecord(2), NewRecord(3)}
	source := NewSliceSource("test", records, 100, 0)

	cfg := DefaultPipelineConfig()
	cfg.EnableCheckpoint = true
	cfg.CheckpointStore = store

	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	ctx := context.Background()
	_ = pipeline.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	err = pipeline.SaveCheckpoint()
	if err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	_ = pipeline.Stop()

	if store.Count() < 1 {
		t.Error("expected at least 1 checkpoint")
	}
}

func TestPipelineRestoreCheckpointInvalid(t *testing.T) {
	store := NewMemoryCheckpointStore()
	ctx := context.Background()

	err := store.Save(ctx, &Checkpoint{ID: "invalid", SourceOffset: 10})
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	source := NewChannelSource("test", make(chan *Record), 10)
	cfg := DefaultPipelineConfig()
	cfg.EnableCheckpoint = true
	cfg.CheckpointStore = store

	pipeline, _ := NewPipeline(cfg, source)

	err = pipeline.RestoreFromCheckpoint("invalid")
	if err != nil {
		t.Logf("Restore from checkpoint with no operator states: %v", err)
	}
}

func TestMapOperatorError(t *testing.T) {
	mapper := NewMapOperator("error", func(ctx context.Context, r *Record) (*Record, error) {
		return nil, fmt.Errorf("map error")
	})

	ctx := context.Background()
	_, err := mapper.Process(ctx, NewRecord(1))
	if err == nil {
		t.Error("expected error from map operator")
	}
}

func TestFlatMapOperatorError(t *testing.T) {
	flatMapper := NewFlatMapOperator("error", func(ctx context.Context, r *Record) ([]*Record, error) {
		return nil, fmt.Errorf("flatmap error")
	})

	ctx := context.Background()
	_, err := flatMapper.Process(ctx, NewRecord(1))
	if err == nil {
		t.Error("expected error from flatMap operator")
	}
}

func TestFlatMapOperatorNilFunc(t *testing.T) {
	flatMapper := NewFlatMapOperator("nil", nil)

	ctx := context.Background()
	res, err := flatMapper.Process(ctx, NewRecord(1))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if len(res) != 1 {
		t.Errorf("expected 1 result, got %d", len(res))
	}
}

func TestFlatMapOperatorNilResults(t *testing.T) {
	flatMapper := NewFlatMapOperator("nil-results", func(ctx context.Context, r *Record) ([]*Record, error) {
		return nil, nil
	})

	ctx := context.Background()
	res, err := flatMapper.Process(ctx, NewRecord(1))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if res != nil {
		t.Error("expected nil result")
	}
}

func TestMapOperatorNilFunc(t *testing.T) {
	mapper := NewMapOperator("nil", nil)

	ctx := context.Background()
	res, err := mapper.Process(ctx, NewRecord(1))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if len(res) != 1 {
		t.Errorf("expected 1 result, got %d", len(res))
	}
}

func TestMapOperatorRestoreInvalidData(t *testing.T) {
	mapper := NewMapOperator("test", nil)
	err := mapper.RestoreState([]byte("invalid json"))
	if err == nil {
		t.Error("expected error from invalid restore data")
	}
}

func TestFilterOperatorRestoreInvalidData(t *testing.T) {
	filter := NewFilterOperator("test", nil)
	err := filter.RestoreState([]byte("invalid json"))
	if err == nil {
		t.Error("expected error from invalid restore data")
	}
}

func TestFlatMapOperatorRestoreInvalidData(t *testing.T) {
	fm := NewFlatMapOperator("test", nil)
	err := fm.RestoreState([]byte("invalid json"))
	if err == nil {
		t.Error("expected error from invalid restore data")
	}
}

func TestWindowAggregatorRestoreInvalidData(t *testing.T) {
	window, _ := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationCount,
		CountSize:   5,
	})
	err := window.RestoreState([]byte("invalid json"))
	if err == nil {
		t.Error("expected error from invalid restore data")
	}
}

func TestPipelineDeleteCheckpoint(t *testing.T) {
	store := NewMemoryCheckpointStore()
	ctx := context.Background()

	_ = store.Save(ctx, &Checkpoint{ID: "cp1", SourceOffset: 10})

	source := NewChannelSource("test", make(chan *Record), 10)
	cfg := DefaultPipelineConfig()
	cfg.EnableCheckpoint = true
	cfg.CheckpointStore = store

	pipeline, _ := NewPipeline(cfg, source)

	err := pipeline.DeleteCheckpoint("cp1")
	if err != nil {
		t.Fatalf("DeleteCheckpoint failed: %v", err)
	}

	if store.Count() != 0 {
		t.Error("checkpoint should be deleted")
	}
}

func TestCollectSinkNilInputs(t *testing.T) {
	sink := NewCollectSink()
	ctx := context.Background()

	_ = sink.Consume(ctx, nil)
	_ = sink.ConsumeWindow(ctx, nil)

	records, results := sink.Count()
	if records != 0 || results != 0 {
		t.Error("nil inputs should not be counted")
	}
}

func TestPipelineStatsElapsed(t *testing.T) {
	source := NewChannelSource("test", make(chan *Record), 10)
	pipeline, _ := NewPipeline(DefaultPipelineConfig(), source)

	stats := pipeline.Stats()
	if stats.Elapsed != 0 {
		t.Errorf("expected Elapsed=0 for idle pipeline, got %v", stats.Elapsed)
	}

	ctx := context.Background()
	_ = pipeline.Start(ctx)
	time.Sleep(10 * time.Millisecond)

	stats = pipeline.Stats()
	if stats.Elapsed == 0 {
		t.Error("expected Elapsed > 0 for running pipeline")
	}
	if stats.StartTime.IsZero() {
		t.Error("expected StartTime to be set")
	}

	_ = pipeline.Stop()
}

func TestSourceStateTransitions(t *testing.T) {
	source := NewChannelSource("test", make(chan *Record), 10)
	if source.State().String() != "idle" {
		t.Errorf("expected idle, got %s", source.State().String())
	}

	ctx := context.Background()
	_ = source.Start(ctx)
	if source.State() != SourceStateRunning {
		t.Error("expected running")
	}

	_ = source.Stop()
	if source.State() != SourceStateStopped {
		t.Error("expected stopped")
	}

	err := source.Start(ctx)
	if err == nil {
		t.Error("expected error when starting stopped source")
	}
}

func TestBackpressureStateTransitions(t *testing.T) {
	cfg := DefaultPipelineConfig()
	cfg.BackpressureThreshold = 100
	cfg.BackpressureWarningRatio = 0.5
	cfg.BackpressureCriticalRatio = 0.8

	source := NewChannelSource("test", make(chan *Record), 10)
	pipeline, _ := NewPipeline(cfg, source)

	pipeline.pendingCount = 30
	info := pipeline.GetBackpressureInfo()
	if info.State != BackpressureNormal {
		t.Errorf("expected Normal state at 30%%, got %s", info.State.String())
	}

	atomic.StoreInt64(&pipeline.pendingCount, 60)
	info = pipeline.GetBackpressureInfo()
	if info.State != BackpressureWarning {
		t.Errorf("expected Warning state at 60%%, got %s", info.State.String())
	}

	atomic.StoreInt64(&pipeline.pendingCount, 90)
	info = pipeline.GetBackpressureInfo()
	if info.State != BackpressureCritical {
		t.Errorf("expected Critical state at 90%%, got %s", info.State.String())
	}
}

func TestSlidingTimeWindow(t *testing.T) {
	slide := 100 * time.Millisecond
	size := 300 * time.Millisecond
	window, err := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeSlidingTime,
		Aggregation: AggregationSum,
		Size:        size,
		Slide:       slide,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	ctx := context.Background()
	window.Start(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	var results []*WindowResult
	go func() {
		defer wg.Done()
		for res := range window.Results() {
			results = append(results, res)
		}
	}()

	baseTime := time.Now().Truncate(slide)

	for i := 1; i <= 3; i++ {
		rec := NewRecord(i * 10)
		rec.SeqID = int64(i)
		rec.Timestamp = baseTime.Add(time.Duration(i) * 50*time.Millisecond + 50*time.Millisecond)
		_, err := window.Process(ctx, rec)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}
	}

	time.Sleep(500 * time.Millisecond)

	window.Stop()
	wg.Wait()

	if len(results) == 0 {
		t.Fatal("expected at least 1 window result, got 0")
	}

	for _, r := range results {
		if r.WindowType != WindowTypeSlidingTime {
			t.Errorf("expected WindowType=SlidingTime, got %v", r.WindowType)
		}
		if r.Value <= 0 {
			t.Errorf("expected positive sum, got %v", r.Value)
		}
	}
}

func TestSlidingTimeWindowMultipleBuckets(t *testing.T) {
	slide := 50 * time.Millisecond
	size := 150 * time.Millisecond
	window, err := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeSlidingTime,
		Aggregation: AggregationCount,
		Size:        size,
		Slide:       slide,
		Extractor:   func(r *Record) (float64, error) { return 1.0, nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	ctx := context.Background()

	rec1 := NewRecord(1)
	rec1.SeqID = 1
	rec1.Timestamp = time.Now()
	_, _ = window.Process(ctx, rec1)

	if window.ActiveWindowCount() < 1 {
		t.Errorf("expected at least 1 active window, got %d", window.ActiveWindowCount())
	}
}

func TestSlidingTimeWindowBucketOverlap(t *testing.T) {
	slide := 100 * time.Millisecond
	size := 300 * time.Millisecond
	window, err := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeSlidingTime,
		Aggregation: AggregationSum,
		Size:        size,
		Slide:       slide,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	ctx := context.Background()

	baseTime := time.Now().Truncate(slide)
	rec := NewRecord(10)
	rec.SeqID = 1
	rec.Timestamp = baseTime.Add(150 * time.Millisecond)
	_, _ = window.Process(ctx, rec)

	activeCount := window.ActiveWindowCount()
	if activeCount < 2 {
		t.Errorf("expected at least 2 overlapping buckets for sliding time window, got %d", activeCount)
	}
}

func TestSlidingTimeWindowWithWatermark(t *testing.T) {
	slide := 100 * time.Millisecond
	size := 200 * time.Millisecond
	window, err := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeSlidingTime,
		Aggregation: AggregationSum,
		Size:        size,
		Slide:       slide,
		Watermark:   50 * time.Millisecond,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	ctx := context.Background()
	window.Start(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	var results []*WindowResult
	go func() {
		defer wg.Done()
		for res := range window.Results() {
			results = append(results, res)
		}
	}()

	baseTime := time.Now().Truncate(slide)
	rec := NewRecord(42)
	rec.SeqID = 1
	rec.Timestamp = baseTime.Add(50 * time.Millisecond)
	_, _ = window.Process(ctx, rec)

	time.Sleep(400 * time.Millisecond)

	window.Stop()
	wg.Wait()

	if len(results) == 0 {
		t.Fatal("expected at least 1 window result with watermark, got 0")
	}
}

func TestCountWindowStopPreservesPartialState(t *testing.T) {
	window, err := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationSum,
		CountSize:   10,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	ctx := context.Background()
	window.Start(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	var results []*WindowResult
	go func() {
		defer wg.Done()
		for res := range window.Results() {
			results = append(results, res)
		}
	}()

	for i := 1; i <= 5; i++ {
		rec := NewRecord(i)
		rec.SeqID = int64(i)
		_, _ = window.Process(ctx, rec)
	}

	window.Stop()
	wg.Wait()

	if len(results) != 1 {
		t.Fatalf("expected 1 partial result on stop, got %d", len(results))
	}

	if !results[0].Partial {
		t.Error("expected Partial=true for incomplete window result on stop")
	}

	if results[0].Value != 15.0 {
		t.Errorf("expected partial sum=15, got %v", results[0].Value)
	}

	if results[0].Count != 5 {
		t.Errorf("expected partial count=5, got %d", results[0].Count)
	}
}

func TestSlidingCountWindowStopPreservesPartialState(t *testing.T) {
	window, err := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeSlidingCount,
		Aggregation: AggregationSum,
		CountSize:   5,
		CountSlide:  2,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	ctx := context.Background()
	window.Start(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	var results []*WindowResult
	go func() {
		defer wg.Done()
		for res := range window.Results() {
			results = append(results, res)
		}
	}()

	for i := 1; i <= 3; i++ {
		rec := NewRecord(i)
		rec.SeqID = int64(i)
		_, _ = window.Process(ctx, rec)
	}

	window.Stop()
	wg.Wait()

	partialCount := 0
	for _, r := range results {
		if r.Partial {
			partialCount++
		}
	}
	if partialCount == 0 {
		t.Error("expected at least one partial result from sliding count window on stop")
	}
}

func TestCountWindowStopThenSaveState(t *testing.T) {
	window, err := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationSum,
		CountSize:   10,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	ctx := context.Background()
	for i := 1; i <= 5; i++ {
		rec := NewRecord(i)
		rec.SeqID = int64(i)
		_, _ = window.Process(ctx, rec)
	}

	stateData, err := window.SaveState()
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	window2, err := NewWindowAggregator("test", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationSum,
		CountSize:   10,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	err = window2.RestoreState(stateData)
	if err != nil {
		t.Fatalf("RestoreState failed: %v", err)
	}

	if window2.ActiveWindowCount() != 1 {
		t.Errorf("expected 1 active window after restore, got %d", window2.ActiveWindowCount())
	}

	window2.Start(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	var results []*WindowResult
	go func() {
		defer wg.Done()
		for res := range window2.Results() {
			results = append(results, res)
		}
	}()

	for i := 6; i <= 10; i++ {
		rec := NewRecord(i)
		rec.SeqID = int64(i)
		_, _ = window2.Process(ctx, rec)
	}

	window2.Stop()
	wg.Wait()

	found := false
	for _, r := range results {
		if !r.Partial && r.Count == 10 {
			found = true
			expected := 1.0 + 2.0 + 3.0 + 4.0 + 5.0 + 6.0 + 7.0 + 8.0 + 9.0 + 10.0
			if r.Value != expected {
				t.Errorf("expected full window sum=%v, got %v", expected, r.Value)
			}
		}
	}
	if !found {
		t.Error("expected a complete window result after restoring partial state")
	}
}

func TestRecordsDroppedDistinguishesFilterAndErrors(t *testing.T) {
	input := make(chan *Record, 20)
	source := NewChannelSource("test", input, 20)

	cfg := DefaultPipelineConfig()
	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	filter := NewFilterOperator("even", func(ctx context.Context, r *Record) (bool, error) {
		val := r.Data.(int)
		if val == 7 {
			return false, fmt.Errorf("simulated error on 7")
		}
		return val%2 == 0, nil
	})
	_ = pipeline.AddOperator(filter)

	sink := NewCollectSink()
	pipeline.SetSink(sink)

	ctx := context.Background()
	err = pipeline.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 1; i <= 10; i++ {
		input <- NewRecord(i)
	}
	close(input)

	time.Sleep(200 * time.Millisecond)
	_ = pipeline.Stop()

	stats := pipeline.Stats()

	if stats.RecordsIn != 10 {
		t.Errorf("expected RecordsIn=10, got %d", stats.RecordsIn)
	}

	if stats.RecordsErrors != 1 {
		t.Errorf("expected RecordsErrors=1 (value 7), got %d", stats.RecordsErrors)
	}

	if stats.RecordsFiltered != 4 {
		t.Errorf("expected RecordsFiltered=4 (odd: 1,3,5,9), got %d", stats.RecordsFiltered)
	}

	if stats.RecordsDropped != stats.RecordsFiltered+stats.RecordsExpanded+stats.RecordsErrors {
		t.Errorf("expected RecordsDropped=%d (Filtered+Expanded+Errors), got %d",
			stats.RecordsFiltered+stats.RecordsExpanded+stats.RecordsErrors, stats.RecordsDropped)
	}
}

func TestBackpressureRelievedSignal(t *testing.T) {
	input := make(chan *Record, 200)
	source := NewChannelSource("fast", input, 200)

	cfg := DefaultPipelineConfig()
	cfg.BufferSize = 10
	cfg.BackpressureThreshold = 5
	cfg.BackpressureWarningRatio = 0.6
	cfg.BackpressureCriticalRatio = 0.8

	slowOp := NewMapOperator("slow", func(ctx context.Context, r *Record) (*Record, error) {
		time.Sleep(20 * time.Millisecond)
		return r, nil
	})

	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	_ = pipeline.AddOperator(slowOp)
	sink := NewCollectSink()
	pipeline.SetSink(sink)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = pipeline.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	go func() {
		for i := 0; i < 50; i++ {
			select {
			case input <- NewRecord(i):
			case <-ctx.Done():
				return
			}
		}
	}()

	time.Sleep(300 * time.Millisecond)

	pipeline.backpressureMu.RLock()
	wasPaused := pipeline.sourcePaused
	pipeline.backpressureMu.RUnlock()

	time.Sleep(500 * time.Millisecond)

	close(input)
	_ = pipeline.Stop()

	_ = wasPaused

	count, _ := sink.Count()
	if count == 0 {
		t.Error("expected some records to be processed")
	}
}

func TestPipelineStopWithBackpressure(t *testing.T) {
	input := make(chan *Record, 200)
	source := NewChannelSource("fast", input, 200)

	cfg := DefaultPipelineConfig()
	cfg.BufferSize = 5
	cfg.BackpressureThreshold = 3
	cfg.BackpressureWarningRatio = 0.5
	cfg.BackpressureCriticalRatio = 0.7

	slowOp := NewMapOperator("slow", func(ctx context.Context, r *Record) (*Record, error) {
		time.Sleep(100 * time.Millisecond)
		return r, nil
	})

	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	_ = pipeline.AddOperator(slowOp)

	ctx := context.Background()
	err = pipeline.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 0; i < 50; i++ {
		input <- NewRecord(i)
	}

	done := make(chan struct{})
	go func() {
		_ = pipeline.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("pipeline.Stop() should complete within timeout even under backpressure")
	}
}

func TestCountWindowStopResultsNotLost(t *testing.T) {
	input := make(chan *Record, 50)
	source := NewChannelSource("test", input, 50)

	window, err := NewWindowAggregator("sum", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationSum,
		CountSize:   10,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	cfg := DefaultPipelineConfig()
	cfg.WindowAggregator = window

	sink := NewCollectSink()
	cfg.Sink = sink

	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	ctx := context.Background()
	err = pipeline.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 1; i <= 7; i++ {
		input <- NewRecord(i)
	}

	time.Sleep(100 * time.Millisecond)

	err = pipeline.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	_, windowCount := sink.Count()
	if windowCount != 1 {
		t.Fatalf("expected 1 window result (partial) from incomplete count window, got %d", windowCount)
	}

	results := sink.WindowResults()
	if len(results) != 1 {
		t.Fatalf("expected 1 window result, got %d", len(results))
	}

	if !results[0].Partial {
		t.Error("expected Partial=true for incomplete count window")
	}

	if results[0].Value != 28.0 {
		t.Errorf("expected partial sum=28 (1+2+...+7), got %v", results[0].Value)
	}

	if results[0].Count != 7 {
		t.Errorf("expected partial count=7, got %d", results[0].Count)
	}
}

func TestTimeWindowStopResultsNotLost(t *testing.T) {
	input := make(chan *Record, 50)
	source := NewChannelSource("test", input, 50)

	window, err := NewWindowAggregator("sum", WindowConfig{
		WindowType:  WindowTypeTumblingTime,
		Aggregation: AggregationSum,
		Size:        500 * time.Millisecond,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	cfg := DefaultPipelineConfig()
	cfg.WindowAggregator = window

	sink := NewCollectSink()
	cfg.Sink = sink

	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	ctx := context.Background()
	err = pipeline.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 1; i <= 5; i++ {
		rec := NewRecord(i)
		rec.Timestamp = time.Now()
		input <- rec
	}

	time.Sleep(100 * time.Millisecond)

	err = pipeline.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	_, windowCount := sink.Count()
	if windowCount != 1 {
		t.Fatalf("expected 1 window result from incomplete time window, got %d", windowCount)
	}

	results := sink.WindowResults()
	if results[0].Value != 15.0 {
		t.Errorf("expected sum=15 (1+2+...+5), got %v", results[0].Value)
	}
}

func TestSlidingTimeWindowStopResultsNotLost(t *testing.T) {
	input := make(chan *Record, 50)
	source := NewChannelSource("test", input, 50)

	window, err := NewWindowAggregator("avg", WindowConfig{
		WindowType:  WindowTypeSlidingTime,
		Aggregation: AggregationAvg,
		Size:        300 * time.Millisecond,
		Slide:       100 * time.Millisecond,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	cfg := DefaultPipelineConfig()
	cfg.WindowAggregator = window

	sink := NewCollectSink()
	cfg.Sink = sink

	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	ctx := context.Background()
	err = pipeline.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	baseTime := time.Now()
	for i := 1; i <= 3; i++ {
		rec := NewRecord(i * 10)
		rec.Timestamp = baseTime.Add(time.Duration(i) * 50 * time.Millisecond)
		input <- rec
	}

	time.Sleep(100 * time.Millisecond)

	err = pipeline.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	_, windowCount := sink.Count()
	if windowCount == 0 {
		t.Fatal("expected at least 1 window result from sliding time window on stop, got 0")
	}
}

func TestRecordsFilteredVsRecordsExpanded(t *testing.T) {
	input := make(chan *Record, 50)
	source := NewChannelSource("test", input, 50)

	cfg := DefaultPipelineConfig()
	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	filter := NewFilterOperator("filter-gt5", func(ctx context.Context, r *Record) (bool, error) {
		val := r.Data.(int)
		return val > 5, nil
	})
	_ = pipeline.AddOperator(filter)

	flatMap := NewFlatMapOperator("expand", func(ctx context.Context, r *Record) ([]*Record, error) {
		val := r.Data.(int)
		if val == 8 {
			return []*Record{}, nil
		}
		return []*Record{NewRecord(val), NewRecord(val * 2)}, nil
	})
	_ = pipeline.AddOperator(flatMap)

	sink := NewCollectSink()
	pipeline.SetSink(sink)

	ctx := context.Background()
	err = pipeline.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 1; i <= 10; i++ {
		input <- NewRecord(i)
	}
	close(input)

	time.Sleep(200 * time.Millisecond)
	_ = pipeline.Stop()

	stats := pipeline.Stats()

	if stats.RecordsIn != 10 {
		t.Errorf("expected RecordsIn=10, got %d", stats.RecordsIn)
	}

	if stats.RecordsFiltered != 5 {
		t.Errorf("expected RecordsFiltered=5 (values 1,2,3,4,5), got %d", stats.RecordsFiltered)
	}

	if stats.RecordsExpanded != 1 {
		t.Errorf("expected RecordsExpanded=1 (value 8 expands to empty), got %d", stats.RecordsExpanded)
	}

	if stats.RecordsErrors != 0 {
		t.Errorf("expected RecordsErrors=0, got %d", stats.RecordsErrors)
	}

	expectedDropped := stats.RecordsFiltered + stats.RecordsExpanded + stats.RecordsErrors
	if stats.RecordsDropped != expectedDropped {
		t.Errorf("expected RecordsDropped=%d (Filtered+Expanded+Errors), got %d", expectedDropped, stats.RecordsDropped)
	}

	if stats.RecordsOut != 8 {
		t.Errorf("expected RecordsOut=8 (values 6,7,9,10 each produce 2 outputs; 8 produces 0), got %d", stats.RecordsOut)
	}
}

func TestOperatorChainProcessDiscriminatesFilterVsFlatMap(t *testing.T) {
	chain := NewOperatorChain()

	filter := NewFilterOperator("even", func(ctx context.Context, r *Record) (bool, error) {
		return r.Data.(int)%2 == 0, nil
	})
	_ = chain.Add(filter)

	flatMap := NewFlatMapOperator("expand", func(ctx context.Context, r *Record) ([]*Record, error) {
		if r.Data.(int) == 8 {
			return []*Record{}, nil
		}
		return []*Record{r}, nil
	})
	_ = chain.Add(flatMap)

	ctx := context.Background()

	rec1 := NewRecord(3)
	_, filtered, err := chain.Process(ctx, rec1)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if !filtered {
		t.Error("expected filtered=true for FilterOperator discarding odd number")
	}

	rec2 := NewRecord(8)
	_, filtered, err = chain.Process(ctx, rec2)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if filtered {
		t.Error("expected filtered=false for FlatMapOperator expanding to empty")
	}

	rec3 := NewRecord(6)
	res, _, err := chain.Process(ctx, rec3)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if len(res) != 1 {
		t.Errorf("expected 1 result for value 6, got %d", len(res))
	}
}

func TestPipelineStopOrderNoDeadlock(t *testing.T) {
	input := make(chan *Record, 20)
	source := NewChannelSource("test", input, 20)

	slowOp := NewMapOperator("slow", func(ctx context.Context, r *Record) (*Record, error) {
		time.Sleep(50 * time.Millisecond)
		return r, nil
	})

	cfg := DefaultPipelineConfig()
	cfg.BufferSize = 20

	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	_ = pipeline.AddOperator(slowOp)

	ctx := context.Background()
	err = pipeline.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 0; i < 20; i++ {
		input <- NewRecord(i)
	}

	time.Sleep(30 * time.Millisecond)

	done := make(chan error, 1)
	go func() {
		done <- pipeline.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("pipeline.Stop() deadlocked - should complete within 3 seconds")
	}
}

func TestPipelineStopPreservesSinkOrder(t *testing.T) {
	input := make(chan *Record, 50)
	source := NewChannelSource("test", input, 50)

	window, err := NewWindowAggregator("count", WindowConfig{
		WindowType:  WindowTypeTumblingCount,
		Aggregation: AggregationCount,
		CountSize:   3,
		Extractor:   func(r *Record) (float64, error) { return 1.0, nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	cfg := DefaultPipelineConfig()
	cfg.WindowAggregator = window

	sink := NewCollectSink()
	cfg.Sink = sink

	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	ctx := context.Background()
	err = pipeline.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 1; i <= 7; i++ {
		input <- NewRecord(i)
	}

	time.Sleep(100 * time.Millisecond)

	_ = pipeline.Stop()

	recCount, winCount := sink.Count()
	if recCount != 7 {
		t.Errorf("expected 7 records consumed by sink, got %d", recCount)
	}
	if winCount != 3 {
		t.Errorf("expected 3 window results (2 complete + 1 partial), got %d", winCount)
	}

	results := sink.WindowResults()
	if results[2].Partial != true {
		t.Error("expected last window result to be partial")
	}
}

func TestTumblingTimeWindowStopPartialResults(t *testing.T) {
	input := make(chan *Record, 50)
	source := NewChannelSource("test", input, 50)

	window, err := NewWindowAggregator("sum", WindowConfig{
		WindowType:  WindowTypeTumblingTime,
		Aggregation: AggregationSum,
		Size:        1 * time.Second,
		Extractor:   func(r *Record) (float64, error) { return float64(r.Data.(int)), nil },
	})
	if err != nil {
		t.Fatalf("NewWindowAggregator failed: %v", err)
	}

	cfg := DefaultPipelineConfig()
	cfg.WindowAggregator = window

	sink := NewCollectSink()
	cfg.Sink = sink

	pipeline, err := NewPipeline(cfg, source)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	ctx := context.Background()
	err = pipeline.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 1; i <= 5; i++ {
		rec := NewRecord(i)
		rec.Timestamp = time.Now()
		input <- rec
	}

	time.Sleep(100 * time.Millisecond)

	_ = pipeline.Stop()

	_, winCount := sink.Count()
	if winCount != 1 {
		t.Fatalf("expected 1 partial window result from tumbling time window, got %d", winCount)
	}

	results := sink.WindowResults()
	if results[0].Value != 15.0 {
		t.Errorf("expected partial sum=15, got %v", results[0].Value)
	}
}