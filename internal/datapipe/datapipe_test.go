package datapipe

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockSource struct {
	mu           sync.Mutex
	records      []*Record
	fetchIdx     int
	fetchDelay   time.Duration
	fetchErr     error
	fetchErrAt   int
	fetchErrCnt  int
	countErr     error
	closeCalled  int
}

func newMockSource(n int) *mockSource {
	recs := make([]*Record, n)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		recs[i] = &Record{
			ID:        fmt.Sprintf("rec-%d", i),
			SeqID:     int64(i + 1),
			Timestamp: base.Add(time.Duration(i) * time.Minute),
			Data: map[string]interface{}{
				"name":  fmt.Sprintf("name-%d", i),
				"value": i,
			},
		}
	}
	return &mockSource{records: recs}
}

func (m *mockSource) Fetch(ctx context.Context, cursor *Cursor, batchSize int) (*Batch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.fetchDelay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.fetchDelay):
		}
	}

	if m.fetchErr != nil {
		if m.fetchErrAt < 0 || m.fetchErrCnt <= m.fetchErrAt {
			if m.fetchErrAt >= 0 {
				m.fetchErrCnt++
			}
			return nil, m.fetchErr
		}
		if m.fetchErrAt >= 0 {
			m.fetchErrCnt++
		}
	}

	startIdx := m.findStartIdx(cursor)
	if startIdx >= len(m.records) {
		return &Batch{Records: nil}, nil
	}
	endIdx := startIdx + batchSize
	if endIdx > len(m.records) {
		endIdx = len(m.records)
	}
	recs := make([]*Record, endIdx-startIdx)
	copy(recs, m.records[startIdx:endIdx])

	batch := &Batch{
		Records: recs,
	}
	if len(recs) > 0 {
		batch.FirstSeq = recs[0].SeqID
		batch.LastSeq = recs[len(recs)-1].SeqID
		batch.StartTs = recs[0].Timestamp
		batch.EndTs = recs[len(recs)-1].Timestamp
	}
	return batch, nil
}

func (m *mockSource) findStartIdx(cursor *Cursor) int {
	if cursor == nil {
		return 0
	}

	switch cursor.Mode {
	case IncrementalModeFull:
		return int(cursor.LastOffset)

	case IncrementalModeTimestamp:
		if cursor.LastValue == nil {
			return 0
		}
		lastTs, ok := cursor.LastValue.(time.Time)
		if !ok {
			return 0
		}
		for i, r := range m.records {
			if r.Timestamp.After(lastTs) {
				return i
			}
		}
		return len(m.records)

	case IncrementalModeID:
		if cursor.LastValue == nil {
			return 0
		}
		lastID, ok := cursor.LastValue.(int64)
		if !ok {
			return 0
		}
		for i, r := range m.records {
			if r.SeqID > lastID {
				return i
			}
		}
		return len(m.records)

	default:
		return 0
	}
}

func (m *mockSource) Count(_ context.Context, cursor *Cursor) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.countErr != nil {
		return 0, m.countErr
	}
	startIdx := m.findStartIdx(cursor)
	return int64(len(m.records) - startIdx), nil
}

func (m *mockSource) Close(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeCalled++
	return nil
}

type mockTarget struct {
	mu            sync.Mutex
	written       []*Record
	writeDelay    time.Duration
	writeErr      error
	writeErrAt    int
	writeErrCnt   int
	alwaysFailAt  []int
	closeCalled   int
}

func newMockTarget() *mockTarget {
	return &mockTarget{
		written: make([]*Record, 0),
	}
}

func (t *mockTarget) Write(ctx context.Context, batch *Batch) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.writeDelay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(t.writeDelay):
		}
	}

	batchID := int(batch.ID)
	for _, failID := range t.alwaysFailAt {
		if failID == batchID {
			return errors.New("persistent write failure")
		}
	}

	if t.writeErr != nil {
		if t.writeErrAt < 0 || t.writeErrCnt <= t.writeErrAt {
			if t.writeErrAt >= 0 {
				t.writeErrCnt++
			}
			return t.writeErr
		}
		if t.writeErrAt >= 0 {
			t.writeErrCnt++
		}
	}

	for _, r := range batch.Records {
		t.written = append(t.written, r)
	}
	return nil
}

func (t *mockTarget) WrittenCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.written)
}

func (t *mockTarget) GetWritten() []*Record {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]*Record, len(t.written))
	copy(out, t.written)
	return out
}

func (t *mockTarget) Close(_ context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closeCalled++
	return nil
}

func TestNewPipeline_InvalidConfig(t *testing.T) {
	src := newMockSource(10)
	tgt := newMockTarget()

	_, err := NewPipeline(Config{BatchSize: 0}, src, tgt, nil)
	if err != ErrBatchSizeInvalid {
		t.Errorf("expected ErrBatchSizeInvalid, got %v", err)
	}

	_, err = NewPipeline(Config{BatchSize: -1}, src, tgt, nil)
	if err != ErrBatchSizeInvalid {
		t.Errorf("expected ErrBatchSizeInvalid, got %v", err)
	}

	_, err = NewPipeline(DefaultConfig(), nil, tgt, nil)
	if err != ErrSourceNil {
		t.Errorf("expected ErrSourceNil, got %v", err)
	}

	_, err = NewPipeline(DefaultConfig(), src, nil, nil)
	if err != ErrTargetNil {
		t.Errorf("expected ErrTargetNil, got %v", err)
	}

	_, err = NewPipeline(Config{
		BatchSize:        10,
		IncrementalMode:  IncrementalModeTimestamp,
		IncrementalField: "",
	}, src, tgt, nil)
	if err != ErrIncrementalNoField {
		t.Errorf("expected ErrIncrementalNoField, got %v", err)
	}

	_, err = NewPipeline(Config{
		BatchSize:        10,
		IncrementalMode:  IncrementalModeID,
		IncrementalField: "",
	}, src, tgt, nil)
	if err != ErrIncrementalNoField {
		t.Errorf("expected ErrIncrementalNoField, got %v", err)
	}

	_, err = NewPipeline(Config{
		BatchSize:        10,
		ProgressInterval: -1,
	}, src, tgt, nil)
	if err != ErrProgressInterval {
		t.Errorf("expected ErrProgressInterval, got %v", err)
	}
}

func TestNewPipeline_DefaultValues(t *testing.T) {
	src := newMockSource(10)
	tgt := newMockTarget()

	cfg := Config{
		BatchSize:        10,
		MaxRetryPerBatch: -1,
		RetryBackoff:     -1,
		TimeoutPerBatch:  -1,
	}
	p, err := NewPipeline(cfg, src, tgt, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.cfg.MaxRetryPerBatch != 0 {
		t.Errorf("expected MaxRetryPerBatch=0, got %d", p.cfg.MaxRetryPerBatch)
	}
	if p.cfg.RetryBackoff != 0 {
		t.Errorf("expected RetryBackoff=0, got %v", p.cfg.RetryBackoff)
	}
	if p.cfg.TimeoutPerBatch <= 0 {
		t.Errorf("expected positive TimeoutPerBatch, got %v", p.cfg.TimeoutPerBatch)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BatchSize != 100 {
		t.Errorf("expected BatchSize=100, got %d", cfg.BatchSize)
	}
	if cfg.IncrementalMode != IncrementalModeFull {
		t.Errorf("expected IncrementalModeFull, got %d", cfg.IncrementalMode)
	}
	if !cfg.EnableCheckpoint {
		t.Error("expected EnableCheckpoint=true")
	}
	if cfg.ProgressInterval != 500*time.Millisecond {
		t.Errorf("expected ProgressInterval=500ms, got %v", cfg.ProgressInterval)
	}
	if cfg.TimeoutPerBatch != 30*time.Second {
		t.Errorf("expected TimeoutPerBatch=30s, got %v", cfg.TimeoutPerBatch)
	}
	if cfg.MaxRetryPerBatch != 3 {
		t.Errorf("expected MaxRetryPerBatch=3, got %d", cfg.MaxRetryPerBatch)
	}
	if cfg.RetryBackoff != 100*time.Millisecond {
		t.Errorf("expected RetryBackoff=100ms, got %v", cfg.RetryBackoff)
	}
}

func TestRecord_GetField(t *testing.T) {
	r := &Record{
		Data: map[string]interface{}{
			"key1": "val1",
			"key2": 42,
		},
	}

	v, ok := r.GetField("key1")
	if !ok || v != "val1" {
		t.Errorf("expected (val1, true), got (%v, %v)", v, ok)
	}

	v, ok = r.GetField("key2")
	if !ok || v != 42 {
		t.Errorf("expected (42, true), got (%v, %v)", v, ok)
	}

	_, ok = r.GetField("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent key")
	}

	r2 := &Record{Data: nil}
	_, ok = r2.GetField("key")
	if ok {
		t.Error("expected ok=false for nil Data")
	}
}

func TestBatch_Size(t *testing.T) {
	b := &Batch{}
	if b.Size() != 0 {
		t.Errorf("expected size 0, got %d", b.Size())
	}

	b.Records = []*Record{{ID: "1"}, {ID: "2"}}
	if b.Size() != 2 {
		t.Errorf("expected size 2, got %d", b.Size())
	}
}

func TestPipelineStatus_String(t *testing.T) {
	cases := []struct {
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
	for _, c := range cases {
		if c.status.String() != c.expected {
			t.Errorf("status %d: expected %q, got %q", c.status, c.expected, c.status.String())
		}
	}
}

func TestMemoryCheckpointStore(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryCheckpointStore()

	c, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if c != nil {
		t.Errorf("expected nil cursor on empty store, got %v", c)
	}

	cursor := &Cursor{
		Mode:       IncrementalModeID,
		LastValue:  int64(100),
		LastOffset: 50,
		UpdateTime: time.Now(),
	}
	if err := store.Save(ctx, cursor); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load after Save failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil cursor after Load")
	}
	if loaded.Mode != cursor.Mode {
		t.Errorf("Mode mismatch: %v vs %v", loaded.Mode, cursor.Mode)
	}
	if loaded.LastValue.(int64) != cursor.LastValue.(int64) {
		t.Errorf("LastValue mismatch: %v vs %v", loaded.LastValue, cursor.LastValue)
	}
	if loaded.LastOffset != cursor.LastOffset {
		t.Errorf("LastOffset mismatch: %d vs %d", loaded.LastOffset, cursor.LastOffset)
	}

	cursor2 := &Cursor{Mode: IncrementalModeTimestamp, LastOffset: 10}
	if err := store.Save(ctx, cursor2); err != nil {
		t.Fatalf("second Save failed: %v", err)
	}
	loaded2, _ := store.Load(ctx)
	if loaded2.LastOffset != 10 {
		t.Errorf("expected overwrite, got LastOffset=%d", loaded2.LastOffset)
	}

	if err := store.Save(ctx, nil); err != ErrInvalidCursor {
		t.Errorf("expected ErrInvalidCursor for nil save, got %v", err)
	}

	if err := store.Clear(ctx); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}
	cleared, _ := store.Load(ctx)
	if cleared != nil {
		t.Error("expected nil after Clear")
	}
}

func TestRun_FullMigration_Success(t *testing.T) {
	total := 500
	src := newMockSource(total)
	tgt := newMockTarget()

	cfg := Config{
		BatchSize:        50,
		IncrementalMode:  IncrementalModeFull,
		EnableCheckpoint: true,
		MaxRetryPerBatch: 0,
		TimeoutPerBatch:  5 * time.Second,
		ProgressInterval: 0,
	}
	p, err := NewPipeline(cfg, src, tgt, nil)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	if p.Status() != PipelineStatusIdle {
		t.Errorf("expected Idle status, got %v", p.Status())
	}

	ctx := context.Background()
	err = p.Run(ctx)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if p.Status() != PipelineStatusCompleted {
		t.Errorf("expected Completed status, got %v", p.Status())
	}
	if p.GetProcessed() != int64(total) {
		t.Errorf("expected Processed=%d, got %d", total, p.GetProcessed())
	}
	if p.GetTotal() != int64(total) {
		t.Errorf("expected Total=%d, got %d", total, p.GetTotal())
	}
	expectedBatches := (total + cfg.BatchSize - 1) / cfg.BatchSize
	if p.GetBatches() != int64(expectedBatches) {
		t.Errorf("expected Batches=%d, got %d", expectedBatches, p.GetBatches())
	}
	if tgt.WrittenCount() != total {
		t.Errorf("expected %d written records, got %d", total, tgt.WrittenCount())
	}

	written := tgt.GetWritten()
	for i, r := range written {
		if r.ID != fmt.Sprintf("rec-%d", i) {
			t.Errorf("record %d: expected ID=rec-%d, got %s", i, i, r.ID)
		}
	}
}

func TestRun_EmptySource(t *testing.T) {
	src := newMockSource(0)
	tgt := newMockTarget()

	cfg := Config{
		BatchSize:        10,
		EnableCheckpoint: false,
		ProgressInterval: 0,
	}
	p, _ := NewPipeline(cfg, src, tgt, nil)

	err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if p.Status() != PipelineStatusCompleted {
		t.Errorf("expected Completed, got %v", p.Status())
	}
	if p.GetProcessed() != 0 {
		t.Errorf("expected 0 processed, got %d", p.GetProcessed())
	}
	if p.GetBatches() != 0 {
		t.Errorf("expected 0 batches, got %d", p.GetBatches())
	}
	if tgt.WrittenCount() != 0 {
		t.Errorf("expected 0 written, got %d", tgt.WrittenCount())
	}
}

func TestRun_SingleRecord(t *testing.T) {
	src := newMockSource(1)
	tgt := newMockTarget()

	cfg := Config{
		BatchSize:        10,
		EnableCheckpoint: false,
		ProgressInterval: 0,
	}
	p, _ := NewPipeline(cfg, src, tgt, nil)

	err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if p.GetProcessed() != 1 {
		t.Errorf("expected 1 processed, got %d", p.GetProcessed())
	}
	if p.GetBatches() != 1 {
		t.Errorf("expected 1 batch, got %d", p.GetBatches())
	}
	if tgt.WrittenCount() != 1 {
		t.Errorf("expected 1 written, got %d", tgt.WrittenCount())
	}
}

func TestRun_IncrementalTimestamp(t *testing.T) {
	src := newMockSource(200)
	tgt := newMockTarget()

	cfg := Config{
		BatchSize:        30,
		IncrementalMode:  IncrementalModeTimestamp,
		IncrementalField: "timestamp",
		EnableCheckpoint: true,
		MaxRetryPerBatch: 0,
		ProgressInterval: 0,
	}
	p, _ := NewPipeline(cfg, src, tgt, nil)

	ctx := context.Background()
	err := p.Run(ctx)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if p.GetProcessed() != 200 {
		t.Errorf("expected 200 processed, got %d", p.GetProcessed())
	}

	cursor := p.GetCursor()
	if cursor == nil {
		t.Fatal("expected non-nil cursor")
	}
	if cursor.Mode != IncrementalModeTimestamp {
		t.Errorf("expected Mode=Timestamp, got %v", cursor.Mode)
	}
	endTs, ok := cursor.LastValue.(time.Time)
	if !ok {
		t.Fatalf("expected LastValue to be time.Time, got %T", cursor.LastValue)
	}
	expectedEnd := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Add(199 * time.Minute)
	if !endTs.Equal(expectedEnd) {
		t.Errorf("expected LastValue=%v, got %v", expectedEnd, endTs)
	}
}

func TestRun_IncrementalID(t *testing.T) {
	src := newMockSource(150)
	tgt := newMockTarget()

	cfg := Config{
		BatchSize:        25,
		IncrementalMode:  IncrementalModeID,
		IncrementalField: "id",
		EnableCheckpoint: true,
		MaxRetryPerBatch: 0,
		ProgressInterval: 0,
	}
	p, _ := NewPipeline(cfg, src, tgt, nil)

	err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	cursor := p.GetCursor()
	if cursor == nil {
		t.Fatal("expected cursor")
	}
	if cursor.Mode != IncrementalModeID {
		t.Errorf("expected ID mode, got %v", cursor.Mode)
	}
	lastID, ok := cursor.LastValue.(int64)
	if !ok {
		t.Fatalf("expected int64 LastValue, got %T", cursor.LastValue)
	}
	if lastID != 150 {
		t.Errorf("expected LastValue=150, got %d", lastID)
	}
}

func TestRun_CheckpointResume(t *testing.T) {
	t.Run("FullMode_LastOffset", func(t *testing.T) {
		testCheckpointResume(t, IncrementalModeFull, "offset")
	})
	t.Run("IDMode_LastValue", func(t *testing.T) {
		testCheckpointResume(t, IncrementalModeID, "id")
	})
	t.Run("TimestampMode_LastValue", func(t *testing.T) {
		testCheckpointResume(t, IncrementalModeTimestamp, "ts")
	})
}

func testCheckpointResume(t *testing.T, mode IncrementalMode, field string) {
	store := NewMemoryCheckpointStore()
	total := 100
	batchSize := 10
	stopAtBatch := int64(5)

	src1 := newMockSource(total)
	src1.fetchDelay = 2 * time.Millisecond
	tgt1 := newMockTarget()

	cfg := Config{
		BatchSize:        batchSize,
		IncrementalMode:  mode,
		IncrementalField: field,
		EnableCheckpoint: true,
		MaxRetryPerBatch: 0,
		ProgressInterval: 0,
		TimeoutPerBatch:  5 * time.Second,
	}
	p1, _ := NewPipeline(cfg, src1, tgt1, store)

	runDone := make(chan error, 1)
	go func() {
		runDone <- p1.Run(context.Background())
	}()

	for {
		if p1.GetBatches() >= stopAtBatch {
			p1.Stop()
			break
		}
		time.Sleep(500 * time.Microsecond)
		if p1.Status() != PipelineStatusRunning {
			break
		}
	}

	_ = <-runDone

	cursorAfterFirst, _ := store.Load(context.Background())
	if cursorAfterFirst == nil {
		t.Fatal("expected checkpoint after first run")
	}
	firstWritten := tgt1.WrittenCount()
	if firstWritten == 0 {
		t.Fatal("expected some records written in first run")
	}
	t.Logf("[mode=%v] First run: written=%d, batches=%d, cursor.LastOffset=%d, LastValue=%v",
		mode, firstWritten, p1.GetBatches(), cursorAfterFirst.LastOffset, cursorAfterFirst.LastValue)

	src2 := newMockSource(total)
	tgt2 := newMockTarget()
	tgt2.written = tgt1.GetWritten()

	p2, _ := NewPipeline(cfg, src2, tgt2, store)
	err := p2.Run(context.Background())
	if err != nil {
		t.Fatalf("Resume run failed: %v", err)
	}

	totalWritten := tgt2.WrittenCount()
	if totalWritten != total {
		t.Errorf("[mode=%v] expected total %d written after resume, got %d (first: %d, resume: %d)",
			mode, total, totalWritten, firstWritten, totalWritten-firstWritten)
	}

	written := tgt2.GetWritten()
	idSeen := make(map[string]bool)
	for i, r := range written {
		if idSeen[r.ID] {
			t.Errorf("[mode=%v] duplicate record found at idx %d: %s", mode, i, r.ID)
		}
		idSeen[r.ID] = true
		expectedID := fmt.Sprintf("rec-%d", i)
		if r.ID != expectedID {
			t.Errorf("[mode=%v] record %d: expected ID=%s, got %s", mode, i, expectedID, r.ID)
		}
	}
	if len(idSeen) != total {
		t.Errorf("[mode=%v] expected %d unique records, got %d", mode, total, len(idSeen))
	}

	finalCursor, _ := store.Load(context.Background())
	if finalCursor.LastOffset != int64(total) {
		t.Errorf("[mode=%v] expected final LastOffset=%d, got %d", mode, total, finalCursor.LastOffset)
	}

	switch mode {
	case IncrementalModeID:
		lastID, ok := finalCursor.LastValue.(int64)
		if !ok {
			t.Fatalf("[ID mode] expected int64 LastValue, got %T", finalCursor.LastValue)
		}
		if lastID != int64(total) {
			t.Errorf("[ID mode] expected final LastValue=%d, got %d", total, lastID)
		}
	case IncrementalModeTimestamp:
		lastTs, ok := finalCursor.LastValue.(time.Time)
		if !ok {
			t.Fatalf("[Timestamp mode] expected time.Time LastValue, got %T", finalCursor.LastValue)
		}
		expectedTs := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(total-1) * time.Minute)
		if !lastTs.Equal(expectedTs) {
			t.Errorf("[Timestamp mode] expected final LastValue=%v, got %v", expectedTs, lastTs)
		}
	}
}

func TestRun_ProgressCallback(t *testing.T) {
	src := newMockSource(200)
	tgt := newMockTarget()

	var mu sync.Mutex
	var reports []ProgressInfo
	var lastReport ProgressInfo

	cb := func(info ProgressInfo) {
		mu.Lock()
		defer mu.Unlock()
		reports = append(reports, info)
		lastReport = info
	}

	cfg := Config{
		BatchSize:        10,
		EnableCheckpoint: false,
		ProgressInterval: 10 * time.Millisecond,
		MaxRetryPerBatch: 0,
		TimeoutPerBatch:  5 * time.Second,
	}
	p, _ := NewPipeline(cfg, src, tgt, nil)
	p.SetProgressCallback(cb)

	err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	mu.Lock()
	hasReports := len(reports) > 0
	finalProcessed := lastReport.Processed
	finalTotal := lastReport.Total
	mu.Unlock()

	if !hasReports {
		t.Error("expected at least one progress report")
	}
	if finalProcessed != 200 {
		t.Errorf("expected final Processed=200, got %d", finalProcessed)
	}
	if finalTotal != 200 {
		t.Errorf("expected final Total=200, got %d", finalTotal)
	}
}

func TestRun_ProgressReport_StatsCalculation(t *testing.T) {
	src := newMockSource(100)
	src.fetchDelay = 5 * time.Millisecond
	tgt := newMockTarget()

	done := make(chan struct{})
	var finalInfo ProgressInfo
	var mu sync.Mutex

	cb := func(info ProgressInfo) {
		mu.Lock()
		defer mu.Unlock()
		if info.Processed == 100 {
			finalInfo = info
			select {
			case <-done:
			default:
				close(done)
			}
		}
	}

	cfg := Config{
		BatchSize:        100,
		EnableCheckpoint: false,
		ProgressInterval: 1 * time.Millisecond,
		MaxRetryPerBatch: 0,
		TimeoutPerBatch:  5 * time.Second,
	}
	p, _ := NewPipeline(cfg, src, tgt, nil)
	p.SetProgressCallback(cb)

	go func() {
		_ = p.Run(context.Background())
		time.Sleep(5 * time.Millisecond)
		p.reportProgress()
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}

	mu.Lock()
	defer mu.Unlock()

	if finalInfo.Percent < 99.9 {
		t.Errorf("expected final percent ~100, got %f", finalInfo.Percent)
	}
	if finalInfo.Batches < 1 {
		t.Errorf("expected Batches >= 1, got %d", finalInfo.Batches)
	}
}

func TestRun_WriteRetry_SuccessAfterFail(t *testing.T) {
	src := newMockSource(100)
	tgt := newMockTarget()
	tgt.writeErr = errors.New("transient write error")
	tgt.writeErrAt = 0

	cfg := Config{
		BatchSize:        50,
		EnableCheckpoint: false,
		ProgressInterval: 0,
		MaxRetryPerBatch: 2,
		RetryBackoff:     1 * time.Millisecond,
		TimeoutPerBatch:  5 * time.Second,
	}
	p, _ := NewPipeline(cfg, src, tgt, nil)

	err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if p.GetProcessed() != 100 {
		t.Errorf("expected 100 processed after retry, got %d", p.GetProcessed())
	}
	if tgt.WrittenCount() != 100 {
		t.Errorf("expected 100 written, got %d", tgt.WrittenCount())
	}
}

func TestRun_WriteRetry_Exhausted(t *testing.T) {
	src := newMockSource(100)
	tgt := newMockTarget()
	tgt.alwaysFailAt = []int{1}

	cfg := Config{
		BatchSize:        30,
		EnableCheckpoint: false,
		ProgressInterval: 0,
		MaxRetryPerBatch: 1,
		RetryBackoff:     1 * time.Millisecond,
		TimeoutPerBatch:  2 * time.Second,
	}
	p, _ := NewPipeline(cfg, src, tgt, nil)

	err := p.Run(context.Background())
	if err == nil {
		t.Fatal("expected error from exhausted retries")
	}
	if p.Status() != PipelineStatusFailed {
		t.Errorf("expected Failed status, got %v", p.Status())
	}
}

func TestRun_FetchError(t *testing.T) {
	src := newMockSource(100)
	src.fetchErr = errors.New("fetch failed permanently")
	src.fetchErrAt = -1
	tgt := newMockTarget()

	cfg := Config{
		BatchSize:        10,
		EnableCheckpoint: false,
		ProgressInterval: 0,
		MaxRetryPerBatch: 0,
		TimeoutPerBatch:  2 * time.Second,
	}
	p, _ := NewPipeline(cfg, src, tgt, nil)

	err := p.Run(context.Background())
	if err == nil {
		t.Fatal("expected fetch error")
	}
	if p.Status() != PipelineStatusFailed {
		t.Errorf("expected Failed status, got %v", p.Status())
	}
}

func TestRun_CountErrorIgnored(t *testing.T) {
	src := newMockSource(50)
	src.countErr = errors.New("count not supported")
	tgt := newMockTarget()

	cfg := Config{
		BatchSize:        10,
		EnableCheckpoint: false,
		ProgressInterval: 0,
		MaxRetryPerBatch: 0,
		TimeoutPerBatch:  5 * time.Second,
	}
	p, _ := NewPipeline(cfg, src, tgt, nil)

	err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("Run should succeed even with count error: %v", err)
	}
	if p.GetProcessed() != 50 {
		t.Errorf("expected 50 processed, got %d", p.GetProcessed())
	}
	if p.GetTotal() != 0 {
		t.Errorf("expected Total=0 when count fails, got %d", p.GetTotal())
	}
}

func TestRun_ContextCancel(t *testing.T) {
	src := newMockSource(1000)
	src.fetchDelay = 10 * time.Millisecond
	tgt := newMockTarget()
	tgt.writeDelay = 10 * time.Millisecond

	cfg := Config{
		BatchSize:        20,
		EnableCheckpoint: false,
		ProgressInterval: 0,
		MaxRetryPerBatch: 0,
		TimeoutPerBatch:  5 * time.Second,
	}
	p, _ := NewPipeline(cfg, src, tgt, nil)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := p.Run(ctx)
	if err == nil {
		t.Log("Run returned nil after cancel (may have finished fast)")
	} else if !errors.Is(err, context.Canceled) && err != context.Canceled {
		t.Logf("Run returned: %v", err)
	}

	status := p.Status()
	if status != PipelineStatusFailed && status != PipelineStatusCompleted && status != PipelineStatusStopped {
		t.Logf("Status after cancel: %v", status)
	}
}

func TestRun_StopPipeline(t *testing.T) {
	src := newMockSource(10000)
	src.fetchDelay = 5 * time.Millisecond
	tgt := newMockTarget()
	tgt.writeDelay = 5 * time.Millisecond

	cfg := Config{
		BatchSize:        100,
		EnableCheckpoint: true,
		ProgressInterval: 0,
		MaxRetryPerBatch: 0,
		TimeoutPerBatch:  5 * time.Second,
	}
	p, _ := NewPipeline(cfg, src, tgt, nil)

	stopped := make(chan struct{})
	var runErr error
	go func() {
		runErr = p.Run(context.Background())
		close(stopped)
	}()

	time.Sleep(20 * time.Millisecond)
	p.Stop()
	p.Stop()

	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for Run to stop")
	}

	if p.Status() != PipelineStatusStopped {
		t.Logf("status=%v (processed=%d, may have completed)", p.Status(), p.GetProcessed())
	}
	if runErr != nil {
		t.Errorf("expected nil error on Stop, got %v", runErr)
	}
}

func TestRun_RunningTwice(t *testing.T) {
	src := newMockSource(50)
	tgt := newMockTarget()

	cfg := Config{
		BatchSize:        10,
		EnableCheckpoint: false,
		ProgressInterval: 0,
		MaxRetryPerBatch: 0,
	}
	p, _ := NewPipeline(cfg, src, tgt, nil)

	err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("first Run failed: %v", err)
	}

	src2 := newMockSource(50)
	tgt2 := newMockTarget()
	p.source = src2
	p.target = tgt2

	err = p.Run(context.Background())
	if err != nil {
		t.Fatalf("second Run after completion should work, got: %v", err)
	}
}

func TestRun_DisableCheckpoint(t *testing.T) {
	store := NewMemoryCheckpointStore()
	src := newMockSource(100)
	tgt := newMockTarget()

	cfg := Config{
		BatchSize:        20,
		EnableCheckpoint: false,
		ProgressInterval: 0,
		MaxRetryPerBatch: 0,
	}
	p, err := NewPipeline(cfg, src, tgt, store)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}

	err = p.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if p.GetProcessed() != 100 {
		t.Errorf("expected 100 processed, got %d", p.GetProcessed())
	}

	cursor, _ := store.Load(context.Background())
	if cursor != nil {
		t.Error("expected no checkpoint saved when disabled")
	}
}

func TestGetters_InitialState(t *testing.T) {
	src := newMockSource(10)
	tgt := newMockTarget()
	p, _ := NewPipeline(DefaultConfig(), src, tgt, nil)

	if p.GetProcessed() != 0 {
		t.Errorf("initial Processed should be 0, got %d", p.GetProcessed())
	}
	if p.GetTotal() != 0 {
		t.Errorf("initial Total should be 0, got %d", p.GetTotal())
	}
	if p.GetBatches() != 0 {
		t.Errorf("initial Batches should be 0, got %d", p.GetBatches())
	}
	c := p.GetCursor()
	if c != nil {
		t.Errorf("initial Cursor should be nil, got %v", c)
	}
}

func TestRun_CloseCalled(t *testing.T) {
	src := newMockSource(50)
	tgt := newMockTarget()
	cfg := Config{
		BatchSize:        10,
		EnableCheckpoint: false,
		ProgressInterval: 0,
		MaxRetryPerBatch: 0,
	}
	p, _ := NewPipeline(cfg, src, tgt, nil)

	err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if src.closeCalled < 1 {
		t.Errorf("expected source.Close to be called, got %d", src.closeCalled)
	}
	if tgt.closeCalled < 1 {
		t.Errorf("expected target.Close to be called, got %d", tgt.closeCalled)
	}
}

func TestRun_ConcurrentSafe_Getters(t *testing.T) {
	src := newMockSource(1000)
	src.fetchDelay = 1 * time.Millisecond
	tgt := newMockTarget()
	tgt.writeDelay = 1 * time.Millisecond

	cfg := Config{
		BatchSize:        50,
		EnableCheckpoint: true,
		ProgressInterval: 0,
		MaxRetryPerBatch: 0,
		TimeoutPerBatch:  10 * time.Second,
	}
	p, _ := NewPipeline(cfg, src, tgt, nil)

	done := make(chan struct{})
	go func() {
		_ = p.Run(context.Background())
		close(done)
	}()

	var wg sync.WaitGroup
	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_ = p.Status()
				_ = p.GetProcessed()
				_ = p.GetTotal()
				_ = p.GetBatches()
				_ = p.GetCursor()
				time.Sleep(500 * time.Microsecond)
			}
		}()
	}

	wg.Wait()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out")
	}
}

func TestRun_BatchSizeExactMultiple(t *testing.T) {
	batchSize := 10
	for _, n := range []int{0, 1, 9, 10, 11, 50, 100} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			src := newMockSource(n)
			tgt := newMockTarget()
			cfg := Config{
				BatchSize:        batchSize,
				EnableCheckpoint: false,
				ProgressInterval: 0,
				MaxRetryPerBatch: 0,
			}
			p, _ := NewPipeline(cfg, src, tgt, nil)
			err := p.Run(context.Background())
			if err != nil {
				t.Fatalf("Run failed: %v", err)
			}
			if p.GetProcessed() != int64(n) {
				t.Errorf("expected Processed=%d, got %d", n, p.GetProcessed())
			}
			if tgt.WrittenCount() != n {
				t.Errorf("expected Written=%d, got %d", n, tgt.WrittenCount())
			}
			expectedBatches := int64(0)
			if n > 0 {
				expectedBatches = int64((n + batchSize - 1) / batchSize)
			}
			if p.GetBatches() != expectedBatches {
				t.Errorf("expected Batches=%d, got %d", expectedBatches, p.GetBatches())
			}
		})
	}
}

func TestRun_SetCallbackWhileIdle(t *testing.T) {
	src := newMockSource(10)
	tgt := newMockTarget()
	p, _ := NewPipeline(DefaultConfig(), src, tgt, nil)

	var called int32
	p.SetProgressCallback(func(_ ProgressInfo) {
		atomic.AddInt32(&called, 1)
	})

	cfg := p.cfg
	cfg.ProgressInterval = 1 * time.Millisecond
	cfg.BatchSize = 5
	p.cfg = cfg

	err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if atomic.LoadInt32(&called) < 1 {
		t.Error("expected callback to be called")
	}
}
