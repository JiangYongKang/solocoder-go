package auditlog

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type flakyWriter struct {
	mu         sync.Mutex
	failCount  int
	totalCalls int
	failFirstN int
	panicOn    int
	logs       []*AuditLog
}

func newFlakyWriter(failFirstN int) *flakyWriter {
	return &flakyWriter{failFirstN: failFirstN, logs: make([]*AuditLog, 0)}
}

func (fw *flakyWriter) Write(log *AuditLog) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.totalCalls++
	fw.failCount++
	if fw.panicOn > 0 && fw.totalCalls == fw.panicOn {
		panic("simulated writer panic")
	}
	if fw.failCount <= fw.failFirstN {
		return errors.New("simulated write failure")
	}
	cp := *log
	fw.logs = append(fw.logs, &cp)
	return nil
}

func (fw *flakyWriter) Count() int {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	return len(fw.logs)
}

func (fw *flakyWriter) TotalCalls() int {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	return fw.totalCalls
}

type alwaysFailWriter struct {
	mu         sync.Mutex
	totalCalls int
}

func (afw *alwaysFailWriter) Write(log *AuditLog) error {
	afw.mu.Lock()
	defer afw.mu.Unlock()
	afw.totalCalls++
	return errors.New("always fail")
}

func (afw *alwaysFailWriter) TotalCalls() int {
	afw.mu.Lock()
	defer afw.mu.Unlock()
	return afw.totalCalls
}

func waitForCondition(t *testing.T, condition func() bool, timeout time.Duration, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for: %s", msg)
}

func startLogger(t *testing.T, writer Writer) *Logger {
	t.Helper()
	logger, err := NewLogger(writer)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	if err := logger.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	return logger
}

func startLoggerWithConfig(t *testing.T, writer Writer, cfg Config) *Logger {
	t.Helper()
	logger, err := NewLoggerWithConfig(writer, cfg)
	if err != nil {
		t.Fatalf("NewLoggerWithConfig failed: %v", err)
	}
	if err := logger.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	return logger
}

func makeEntry(subject, resource string, op OperationType, result OperationResult) *Entry {
	return &Entry{
		SubjectID:    subject,
		Operation:    op,
		ResourceID:   resource,
		ResourceType: "test-resource",
		Result:       result,
		SourceIP:     "127.0.0.1",
		UserAgent:    "test-agent",
		Detail:       fmt.Sprintf("subject=%s resource=%s op=%s", subject, resource, op),
	}
}

func TestNewLogger_NilWriter(t *testing.T) {
	_, err := NewLogger(nil)
	if !errors.Is(err, ErrNilWriter) {
		t.Errorf("expected ErrNilWriter, got %v", err)
	}
}

func TestNewLoggerWithConfig_InvalidConfig(t *testing.T) {
	writer := NewMemoryWriter()
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "negative MaxRetries",
			cfg: Config{
				MaxRetries:      -1,
				RetryInterval:   10 * time.Millisecond,
				BufferSize:      10,
				WorkerCount:     1,
				EnableHashChain: true,
			},
			wantErr: true,
		},
		{
			name: "negative RetryInterval",
			cfg: Config{
				MaxRetries:      3,
				RetryInterval:   -1 * time.Millisecond,
				BufferSize:      10,
				WorkerCount:     1,
				EnableHashChain: true,
			},
			wantErr: true,
		},
		{
			name: "zero BufferSize",
			cfg: Config{
				MaxRetries:      3,
				RetryInterval:   10 * time.Millisecond,
				BufferSize:      0,
				WorkerCount:     1,
				EnableHashChain: true,
			},
			wantErr: true,
		},
		{
			name: "negative BufferSize",
			cfg: Config{
				MaxRetries:      3,
				RetryInterval:   10 * time.Millisecond,
				BufferSize:      -1,
				WorkerCount:     1,
				EnableHashChain: true,
			},
			wantErr: true,
		},
		{
			name: "zero WorkerCount",
			cfg: Config{
				MaxRetries:      3,
				RetryInterval:   10 * time.Millisecond,
				BufferSize:      10,
				WorkerCount:     0,
				EnableHashChain: true,
			},
			wantErr: true,
		},
		{
			name: "valid config",
			cfg: Config{
				MaxRetries:      3,
				RetryInterval:   10 * time.Millisecond,
				BufferSize:      10,
				WorkerCount:     1,
				EnableHashChain: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewLoggerWithConfig(writer, tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLoggerWithConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalidConfig) {
				t.Errorf("expected ErrInvalidConfig wrapped, got %v", err)
			}
		})
	}
}

func TestLogger_Start_AlreadyStarted(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	err := logger.Start()
	if !errors.Is(err, ErrLoggerAlreadyStarted) {
		t.Errorf("expected ErrLoggerAlreadyStarted, got %v", err)
	}
}

func TestLogger_Stop_MultipleTimes(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)

	logger.Stop()
	logger.Stop()
}

func TestLogger_Log_NilEntry(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	err := logger.Log(nil)
	if err == nil {
		t.Error("expected error for nil entry, got nil")
	}
}

func TestLogger_LogSync_NilEntry(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	err := logger.LogSync(nil)
	if err == nil {
		t.Error("expected error for nil entry, got nil")
	}
}

func TestLogger_Log_NotStarted(t *testing.T) {
	writer := NewMemoryWriter()
	logger, err := NewLogger(writer)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	err = logger.Log(makeEntry("user1", "res1", OpCreate, ResultSuccess))
	if !errors.Is(err, ErrLoggerStopped) {
		t.Errorf("expected ErrLoggerStopped, got %v", err)
	}
}

func TestLogger_LogSync_Stopped(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	logger.Stop()

	err := logger.LogSync(makeEntry("user1", "res1", OpCreate, ResultSuccess))
	if !errors.Is(err, ErrLoggerStopped) {
		t.Errorf("expected ErrLoggerStopped, got %v", err)
	}
}

func TestLogger_Log_BasicAsyncWrite(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	err := logger.Log(makeEntry("user1", "doc:123", OpCreate, ResultSuccess))
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	waitForCondition(t, func() bool { return writer.Count() >= 1 }, 2*time.Second, "async write to complete")

	if writer.Count() != 1 {
		t.Fatalf("expected 1 log written, got %d", writer.Count())
	}

	logs := writer.ReadAll()
	log := logs[0]
	if log.SubjectID != "user1" {
		t.Errorf("expected SubjectID 'user1', got '%s'", log.SubjectID)
	}
	if log.ResourceID != "doc:123" {
		t.Errorf("expected ResourceID 'doc:123', got '%s'", log.ResourceID)
	}
	if log.Operation != OpCreate {
		t.Errorf("expected OpCreate, got %v", log.Operation)
	}
	if log.Result != ResultSuccess {
		t.Errorf("expected ResultSuccess, got %v", log.Result)
	}
	if log.SourceIP != "127.0.0.1" {
		t.Errorf("expected SourceIP '127.0.0.1', got '%s'", log.SourceIP)
	}
	if log.EventID == "" {
		t.Error("expected non-empty EventID")
	}
	if log.Timestamp.IsZero() {
		t.Error("expected non-zero Timestamp")
	}
	if log.CurrentHash == "" {
		t.Error("expected non-empty CurrentHash")
	}
	if log.PreviousHash != "" {
		t.Errorf("expected empty PreviousHash for first log, got '%s'", log.PreviousHash)
	}
}

func TestLogger_LogSync_BasicWrite(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	err := logger.LogSync(makeEntry("user1", "doc:123", OpUpdate, ResultFailure))
	if err != nil {
		t.Fatalf("LogSync failed: %v", err)
	}

	if writer.Count() != 1 {
		t.Fatalf("expected 1 log written, got %d", writer.Count())
	}

	logs := writer.ReadAll()
	log := logs[0]
	if log.Operation != OpUpdate {
		t.Errorf("expected OpUpdate, got %v", log.Operation)
	}
	if log.Result != ResultFailure {
		t.Errorf("expected ResultFailure, got %v", log.Result)
	}
}

func TestLogger_RetryOnWriteFailure(t *testing.T) {
	cfg := Config{
		MaxRetries:      3,
		RetryInterval:   10 * time.Millisecond,
		BufferSize:      10,
		WorkerCount:     1,
		EnableHashChain: true,
	}
	writer := newFlakyWriter(2)
	logger := startLoggerWithConfig(t, writer, cfg)
	defer logger.Stop()

	err := logger.LogSync(makeEntry("user1", "res1", OpCreate, ResultSuccess))
	if err != nil {
		t.Fatalf("LogSync failed after retries: %v", err)
	}

	if writer.Count() != 1 {
		t.Errorf("expected 1 log written after retries, got %d", writer.Count())
	}
	if writer.TotalCalls() != 3 {
		t.Errorf("expected 3 total write calls (1 initial + 2 retries), got %d", writer.TotalCalls())
	}
}

func TestLogger_RetryExhausted_ReturnsError(t *testing.T) {
	cfg := Config{
		MaxRetries:      2,
		RetryInterval:   5 * time.Millisecond,
		BufferSize:      10,
		WorkerCount:     1,
		EnableHashChain: true,
	}
	writer := newFlakyWriter(100)
	logger := startLoggerWithConfig(t, writer, cfg)
	defer logger.Stop()

	err := logger.LogSync(makeEntry("user1", "res1", OpCreate, ResultSuccess))
	if err == nil {
		t.Fatal("expected error after retry exhaustion, got nil")
	}
	if !errors.Is(err, ErrWriteFailed) {
		t.Errorf("expected ErrWriteFailed wrapped, got %v", err)
	}
	if writer.TotalCalls() != 3 {
		t.Errorf("expected 3 total calls (1 initial + 2 retries), got %d", writer.TotalCalls())
	}
}

func TestLogger_AsyncWrite_DegradeHandler(t *testing.T) {
	cfg := Config{
		MaxRetries:      1,
		RetryInterval:   5 * time.Millisecond,
		BufferSize:      10,
		WorkerCount:     1,
		EnableHashChain: true,
	}
	writer := &alwaysFailWriter{}
	logger := startLoggerWithConfig(t, writer, cfg)

	var degradedMu sync.Mutex
	var degradedEntry *Entry
	var degradedErr error
	var degradedCalled bool

	logger.SetDegradeHandler(func(entry *Entry, err error) {
		degradedMu.Lock()
		defer degradedMu.Unlock()
		degradedCalled = true
		degradedEntry = entry
		degradedErr = err
	})

	entry := makeEntry("user1", "res1", OpDelete, ResultFailure)
	err := logger.Log(entry)
	if err != nil {
		t.Fatalf("Log returned error: %v", err)
	}

	waitForCondition(t, func() bool {
		degradedMu.Lock()
		defer degradedMu.Unlock()
		return degradedCalled
	}, 2*time.Second, "degrade handler to be called")

	logger.Stop()

	degradedMu.Lock()
	defer degradedMu.Unlock()
	if !degradedCalled {
		t.Fatal("degrade handler was not called")
	}
	if degradedEntry.SubjectID != "user1" {
		t.Errorf("degraded entry SubjectID mismatch: expected 'user1', got '%s'", degradedEntry.SubjectID)
	}
	if !errors.Is(degradedErr, ErrWriteFailed) {
		t.Errorf("expected degraded error to wrap ErrWriteFailed, got %v", degradedErr)
	}
}

func TestLogger_AsyncWrite_PanicRecovery(t *testing.T) {
	cfg := Config{
		MaxRetries:      0,
		RetryInterval:   5 * time.Millisecond,
		BufferSize:      10,
		WorkerCount:     1,
		EnableHashChain: true,
	}
	writer := newFlakyWriter(100)
	writer.panicOn = 1
	logger := startLoggerWithConfig(t, writer, cfg)

	var degradedMu sync.Mutex
	var degradedCalled bool
	logger.SetDegradeHandler(func(entry *Entry, err error) {
		degradedMu.Lock()
		defer degradedMu.Unlock()
		degradedCalled = true
	})

	err := logger.Log(makeEntry("user1", "res1", OpCreate, ResultSuccess))
	if err != nil {
		t.Fatalf("Log returned error: %v", err)
	}

	waitForCondition(t, func() bool {
		degradedMu.Lock()
		defer degradedMu.Unlock()
		return degradedCalled
	}, 2*time.Second, "degrade handler called after panic")

	logger.Stop()
}

func TestLogger_DegradeHandlerPanic_Recovery(t *testing.T) {
	cfg := Config{
		MaxRetries:      0,
		RetryInterval:   0,
		BufferSize:      10,
		WorkerCount:     1,
		EnableHashChain: true,
	}
	writer := &alwaysFailWriter{}
	logger := startLoggerWithConfig(t, writer, cfg)

	logger.SetDegradeHandler(func(entry *Entry, err error) {
		panic("degrade handler panic")
	})

	err := logger.Log(makeEntry("user1", "res1", OpCreate, ResultSuccess))
	if err != nil {
		t.Fatalf("Log returned error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	logger.Stop()
}

func TestLogger_Query_BySubject(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	logger.LogSync(makeEntry("alice", "doc:1", OpCreate, ResultSuccess))
	logger.LogSync(makeEntry("bob", "doc:1", OpRead, ResultSuccess))
	logger.LogSync(makeEntry("alice", "doc:2", OpUpdate, ResultSuccess))
	logger.LogSync(makeEntry("charlie", "doc:3", OpDelete, ResultFailure))

	results := logger.Query(Query{SubjectID: "alice"})
	if len(results) != 2 {
		t.Fatalf("expected 2 results for alice, got %d", len(results))
	}
	for _, r := range results {
		if r.SubjectID != "alice" {
			t.Errorf("expected SubjectID 'alice', got '%s'", r.SubjectID)
		}
	}
	if results[0].Timestamp.Before(results[1].Timestamp) {
		t.Error("expected results ordered by timestamp descending")
	}
}

func TestLogger_Query_ByResource(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	logger.LogSync(makeEntry("alice", "doc:1", OpCreate, ResultSuccess))
	logger.LogSync(makeEntry("bob", "doc:1", OpRead, ResultSuccess))
	logger.LogSync(makeEntry("alice", "doc:2", OpUpdate, ResultSuccess))

	results := logger.Query(Query{ResourceID: "doc:1"})
	if len(results) != 2 {
		t.Fatalf("expected 2 results for doc:1, got %d", len(results))
	}
	for _, r := range results {
		if r.ResourceID != "doc:1" {
			t.Errorf("expected ResourceID 'doc:1', got '%s'", r.ResourceID)
		}
	}
}

func TestLogger_Query_BySubjectAndResource(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	logger.LogSync(makeEntry("alice", "doc:1", OpCreate, ResultSuccess))
	logger.LogSync(makeEntry("bob", "doc:1", OpRead, ResultSuccess))
	logger.LogSync(makeEntry("alice", "doc:1", OpUpdate, ResultSuccess))
	logger.LogSync(makeEntry("alice", "doc:2", OpUpdate, ResultSuccess))

	results := logger.Query(Query{SubjectID: "alice", ResourceID: "doc:1"})
	if len(results) != 2 {
		t.Fatalf("expected 2 results for alice+doc:1, got %d", len(results))
	}
	for _, r := range results {
		if r.SubjectID != "alice" || r.ResourceID != "doc:1" {
			t.Errorf("expected alice/doc:1, got %s/%s", r.SubjectID, r.ResourceID)
		}
	}
}

func TestLogger_Query_ByTimeRange(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	t1 := time.Now().Add(-2 * time.Hour)
	t2 := time.Now().Add(-1 * time.Hour)
	t3 := time.Now()

	logger.LogSync(makeEntry("user1", "res1", OpCreate, ResultSuccess))
	logger.LogSync(makeEntry("user1", "res2", OpRead, ResultSuccess))
	logger.LogSync(makeEntry("user1", "res3", OpUpdate, ResultSuccess))

	allLogs := logger.Query(Query{})
	if len(allLogs) != 3 {
		t.Fatalf("expected 3 total logs, got %d", len(allLogs))
	}

	after := t2
	before := t3
	results := logger.Query(Query{StartTime: &after, EndTime: &before})
	_ = t1
	for _, r := range results {
		if r.Timestamp.Before(after) || r.Timestamp.After(before) {
			t.Errorf("log timestamp %v outside range [%v, %v]", r.Timestamp, after, before)
		}
	}
}

func TestLogger_Query_EmptyResult(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	logger.LogSync(makeEntry("alice", "doc:1", OpCreate, ResultSuccess))

	results := logger.Query(Query{SubjectID: "nonexistent"})
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestLogger_Query_AllLogs(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	for i := 0; i < 5; i++ {
		logger.LogSync(makeEntry(fmt.Sprintf("user%d", i), fmt.Sprintf("res%d", i), OpCreate, ResultSuccess))
	}

	results := logger.Query(Query{})
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	for i := 1; i < len(results); i++ {
		if results[i-1].Timestamp.Before(results[i].Timestamp) {
			t.Error("results should be in descending time order")
		}
	}
}

func TestLogger_Query_StartTimeOnly(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	logger.LogSync(makeEntry("u1", "r1", OpCreate, ResultSuccess))
	logger.LogSync(makeEntry("u2", "r2", OpCreate, ResultSuccess))

	start := time.Now().Add(-10 * time.Millisecond)
	results := logger.Query(Query{StartTime: &start})
	if len(results) != 2 {
		t.Errorf("expected 2 results from start time, got %d", len(results))
	}
}

func TestLogger_Query_EndTimeOnly(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	logger.LogSync(makeEntry("u1", "r1", OpCreate, ResultSuccess))

	future := time.Now().Add(1 * time.Hour)
	results := logger.Query(Query{EndTime: &future})
	if len(results) != 1 {
		t.Errorf("expected 1 result before end time, got %d", len(results))
	}
}

func TestLogger_GetByEventID(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	err := logger.LogSync(makeEntry("user1", "res1", OpCreate, ResultSuccess))
	if err != nil {
		t.Fatalf("LogSync failed: %v", err)
	}

	all := logger.Query(Query{})
	if len(all) == 0 {
		t.Fatal("no logs found")
	}

	id := all[0].EventID
	log, err := logger.GetByEventID(id)
	if err != nil {
		t.Fatalf("GetByEventID failed: %v", err)
	}
	if log.EventID != id {
		t.Errorf("expected EventID '%s', got '%s'", id, log.EventID)
	}

	_, err = logger.GetByEventID("nonexistent")
	if !errors.Is(err, ErrLogNotFound) {
		t.Errorf("expected ErrLogNotFound, got %v", err)
	}
}

func TestLogger_Count(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	if logger.Count() != 0 {
		t.Errorf("expected initial count 0, got %d", logger.Count())
	}

	for i := 0; i < 5; i++ {
		logger.LogSync(makeEntry("u", "r", OpCreate, ResultSuccess))
	}

	if logger.Count() != 5 {
		t.Errorf("expected count 5, got %d", logger.Count())
	}
}

func TestLogger_HashChain_Integrity_Valid(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	for i := 0; i < 10; i++ {
		logger.LogSync(makeEntry(fmt.Sprintf("user%d", i), fmt.Sprintf("res%d", i), OpCreate, ResultSuccess))
	}

	result := logger.VerifyIntegrity()
	if !result.Valid {
		t.Errorf("expected valid hash chain, got invalid: %s", result.Message)
	}
	if result.TamperedIndex != -1 {
		t.Errorf("expected TamperedIndex -1, got %d", result.TamperedIndex)
	}
}

func TestLogger_HashChain_Integrity_Empty(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	result := logger.VerifyIntegrity()
	if !result.Valid {
		t.Errorf("expected valid for empty logs, got: %s", result.Message)
	}
}

func TestLogger_HashChain_LinksCorrect(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	for i := 0; i < 5; i++ {
		logger.LogSync(makeEntry(fmt.Sprintf("user%d", i), "res", OpCreate, ResultSuccess))
		time.Sleep(1 * time.Millisecond)
	}

	result := logger.VerifyIntegrity()
	if !result.Valid {
		t.Fatalf("VerifyIntegrity should pass, got: %s", result.Message)
	}

	logs := logger.Query(Query{})
	n := len(logs)
	if n != 5 {
		t.Fatalf("expected 5 logs, got %d", n)
	}

	for i := 1; i < n; i++ {
		if logs[i-1].Timestamp.Before(logs[i].Timestamp) {
			t.Errorf("query order wrong: log %d time %v should not be earlier than log %d time %v",
				i-1, logs[i-1].Timestamp, i, logs[i].Timestamp)
		}
	}

	for i := 0; i < n; i++ {
		log := logs[i]
		expected := computeHash(log)
		if log.CurrentHash != expected {
			t.Errorf("log %d CurrentHash mismatch", i)
		}
	}

	writerLogs := writer.ReadAll()
	if len(writerLogs) != 5 {
		t.Fatalf("expected 5 writer logs, got %d", len(writerLogs))
	}

	if writerLogs[0].PreviousHash != "" {
		t.Errorf("first writer log PreviousHash should be empty, got '%s'", writerLogs[0].PreviousHash)
	}

	for i := 1; i < len(writerLogs); i++ {
		current := writerLogs[i]
		prev := writerLogs[i-1]
		if current.PreviousHash != prev.CurrentHash {
			t.Errorf("writer log %d: PreviousHash should equal writer log %d CurrentHash", i, i-1)
		}
		expectedCurrent := computeHash(current)
		if current.CurrentHash != expectedCurrent {
			t.Errorf("writer log %d: CurrentHash mismatch", i)
		}
	}
}

func TestLogger_HashChain_DetectTamperedDetail(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	for i := 0; i < 5; i++ {
		logger.LogSync(makeEntry(fmt.Sprintf("user%d", i), fmt.Sprintf("res%d", i), OpCreate, ResultSuccess))
	}

	tamperIdx := 2
	logger.tamperLog(tamperIdx, "TAMPERED CONTENT")

	result := logger.VerifyIntegrity()
	if result.Valid {
		t.Fatal("expected invalid result after tampering")
	}
	if result.TamperedIndex != tamperIdx {
		t.Errorf("expected TamperedIndex %d, got %d (msg: %s)", tamperIdx, result.TamperedIndex, result.Message)
	}
}

func TestLogger_HashChain_DetectBrokenLink(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	for i := 0; i < 5; i++ {
		logger.LogSync(makeEntry(fmt.Sprintf("user%d", i), fmt.Sprintf("res%d", i), OpCreate, ResultSuccess))
	}

	breakIdx := 3
	logger.breakChain(breakIdx)

	result := logger.VerifyIntegrity()
	if result.Valid {
		t.Fatal("expected invalid result after breaking chain")
	}
	if result.TamperedIndex != breakIdx {
		t.Errorf("expected TamperedIndex %d, got %d (msg: %s)", breakIdx, result.TamperedIndex, result.Message)
	}
}

func TestLogger_HashChain_DetectFirstTamper(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	for i := 0; i < 10; i++ {
		logger.LogSync(makeEntry(fmt.Sprintf("user%d", i), fmt.Sprintf("res%d", i), OpCreate, ResultSuccess))
	}

	logger.tamperLog(7, "tampered7")
	logger.tamperLog(2, "tampered2")
	logger.tamperLog(5, "tampered5")

	result := logger.VerifyIntegrity()
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	if result.TamperedIndex != 2 {
		t.Errorf("expected first tamper at index 2, got %d (msg: %s)", result.TamperedIndex, result.Message)
	}
}

func TestLogger_HashChain_Disabled(t *testing.T) {
	cfg := Config{
		MaxRetries:      0,
		RetryInterval:   10 * time.Millisecond,
		BufferSize:      10,
		WorkerCount:     1,
		EnableHashChain: false,
	}
	writer := NewMemoryWriter()
	logger := startLoggerWithConfig(t, writer, cfg)
	defer logger.Stop()

	logger.LogSync(makeEntry("user1", "res1", OpCreate, ResultSuccess))
	logger.LogSync(makeEntry("user2", "res2", OpCreate, ResultSuccess))

	logs := writer.ReadAll()
	for _, log := range logs {
		if log.CurrentHash != "" {
			t.Errorf("expected empty CurrentHash when chain disabled, got '%s'", log.CurrentHash)
		}
	}
}

func TestLogger_OperationTypeStrings(t *testing.T) {
	tests := []struct {
		op       OperationType
		expected string
	}{
		{OpCreate, "CREATE"},
		{OpRead, "READ"},
		{OpUpdate, "UPDATE"},
		{OpDelete, "DELETE"},
		{OpLogin, "LOGIN"},
		{OpLogout, "LOGOUT"},
		{OpCustom, "CUSTOM"},
		{OperationType(999), "CUSTOM"},
	}

	for _, tt := range tests {
		if got := tt.op.String(); got != tt.expected {
			t.Errorf("OperationType(%d).String() = %s, want %s", tt.op, got, tt.expected)
		}
	}
}

func TestLogger_OperationResultStrings(t *testing.T) {
	if ResultSuccess.String() != "SUCCESS" {
		t.Errorf("ResultSuccess.String() = %s, want SUCCESS", ResultSuccess.String())
	}
	if ResultFailure.String() != "FAILURE" {
		t.Errorf("ResultFailure.String() = %s, want FAILURE", ResultFailure.String())
	}
}

func TestLogger_ConcurrentWrites(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	var wg sync.WaitGroup
	numWorkers := 10
	perWorker := 100

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				err := logger.Log(&Entry{
					SubjectID:    fmt.Sprintf("worker-%d", workerID),
					Operation:    OpCreate,
					ResourceID:   fmt.Sprintf("res-%d-%d", workerID, i),
					ResourceType: "concurrent",
					Result:       ResultSuccess,
					SourceIP:     fmt.Sprintf("10.0.0.%d", workerID),
					Detail:       fmt.Sprintf("worker %d iteration %d", workerID, i),
				})
				if err != nil {
					t.Errorf("worker %d Log failed at %d: %v", workerID, i, err)
				}
			}
		}(w)
	}

	wg.Wait()

	waitForCondition(t, func() bool { return writer.Count() == numWorkers*perWorker },
		5*time.Second, "all concurrent writes to complete")

	if logger.Count() != numWorkers*perWorker {
		t.Errorf("expected %d total logs, got %d", numWorkers*perWorker, logger.Count())
	}
	if writer.Count() != numWorkers*perWorker {
		t.Errorf("expected %d written logs, got %d", numWorkers*perWorker, writer.Count())
	}

	result := logger.VerifyIntegrity()
	if !result.Valid {
		t.Errorf("hash chain invalid after concurrent writes: %s", result.Message)
	}
}

func TestLogger_ConcurrentWrites_Sync(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	var wg sync.WaitGroup
	numWorkers := 10
	perWorker := 50
	var errorCount int32

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				err := logger.LogSync(&Entry{
					SubjectID:    fmt.Sprintf("sworker-%d", workerID),
					Operation:    OpUpdate,
					ResourceID:   fmt.Sprintf("sres-%d-%d", workerID, i),
					ResourceType: "sync",
					Result:       ResultSuccess,
					SourceIP:     "127.0.0.1",
				})
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
				}
			}
		}(w)
	}

	wg.Wait()

	if errorCount != 0 {
		t.Errorf("got %d errors during concurrent sync writes", errorCount)
	}
	if logger.Count() != numWorkers*perWorker {
		t.Errorf("expected %d total logs, got %d", numWorkers*perWorker, logger.Count())
	}

	result := logger.VerifyIntegrity()
	if !result.Valid {
		t.Errorf("hash chain invalid after concurrent sync writes: %s", result.Message)
	}
}

func TestLogger_StopFlushesBuffer(t *testing.T) {
	cfg := Config{
		MaxRetries:      0,
		RetryInterval:   0,
		BufferSize:      100,
		WorkerCount:     1,
		EnableHashChain: true,
	}
	writer := NewMemoryWriter()
	logger := startLoggerWithConfig(t, writer, cfg)

	for i := 0; i < 50; i++ {
		err := logger.Log(makeEntry("u", "r", OpCreate, ResultSuccess))
		if err != nil {
			t.Fatalf("Log failed: %v", err)
		}
	}

	logger.Stop()

	if writer.Count() != 50 {
		t.Errorf("expected 50 logs flushed after Stop, got %d", writer.Count())
	}
}

func TestLogger_MultipleWorkers(t *testing.T) {
	cfg := Config{
		MaxRetries:      0,
		RetryInterval:   0,
		BufferSize:      100,
		WorkerCount:     4,
		EnableHashChain: true,
	}
	writer := NewMemoryWriter()
	logger := startLoggerWithConfig(t, writer, cfg)
	defer logger.Stop()

	for i := 0; i < 100; i++ {
		logger.Log(makeEntry(fmt.Sprintf("u%d", i), fmt.Sprintf("r%d", i), OpCreate, ResultSuccess))
	}

	waitForCondition(t, func() bool { return writer.Count() == 100 },
		3*time.Second, "all writes to complete with multiple workers")

	result := logger.VerifyIntegrity()
	if !result.Valid {
		t.Errorf("hash chain invalid with multiple workers: %s", result.Message)
	}
}

func TestLogger_BufferOverflow_DelegatesToGoroutine(t *testing.T) {
	cfg := Config{
		MaxRetries:      0,
		RetryInterval:   0,
		BufferSize:      1,
		WorkerCount:     1,
		EnableHashChain: true,
	}

	slow := &slowWriter{delay: 50 * time.Millisecond}
	logger := startLoggerWithConfig(t, slow, cfg)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			err := logger.Log(makeEntry("u", fmt.Sprintf("r%d", i), OpCreate, ResultSuccess))
			if err != nil {
				t.Errorf("Log failed at %d: %v", i, err)
			}
		}
	}()

	wg.Wait()
	logger.Stop()

	if slow.Count() != 10 {
		t.Errorf("expected 10 writes with buffer overflow fallback, got %d", slow.Count())
	}
}

type slowWriter struct {
	mu    sync.Mutex
	logs  []*AuditLog
	delay time.Duration
}

func (sw *slowWriter) Write(log *AuditLog) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	time.Sleep(sw.delay)
	cp := *log
	sw.logs = append(sw.logs, &cp)
	return nil
}

func (sw *slowWriter) Count() int {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return len(sw.logs)
}

func (sw *slowWriter) ReadAll() []*AuditLog {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	result := make([]*AuditLog, len(sw.logs))
	for i, log := range sw.logs {
		cp := *log
		result[i] = &cp
	}
	return result
}

func TestLogger_QueryBySubjectWithTime(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	start := time.Now()

	logger.LogSync(makeEntry("alice", "doc:1", OpCreate, ResultSuccess))
	logger.LogSync(makeEntry("bob", "doc:2", OpRead, ResultSuccess))
	logger.LogSync(makeEntry("alice", "doc:3", OpUpdate, ResultSuccess))

	time.Sleep(10 * time.Millisecond)
	afterMid := time.Now()
	logger.LogSync(makeEntry("alice", "doc:4", OpDelete, ResultSuccess))

	results := logger.Query(Query{SubjectID: "alice", StartTime: &afterMid})
	if len(results) != 1 {
		t.Fatalf("expected 1 result for alice after mid, got %d", len(results))
	}
	if results[0].ResourceID != "doc:4" {
		t.Errorf("expected doc:4, got %s", results[0].ResourceID)
	}
	_ = start
}

func TestLogger_OperationLoginLogout(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	logger.LogSync(&Entry{
		SubjectID:    "user1",
		Operation:    OpLogin,
		ResourceType: "session",
		Result:       ResultSuccess,
		SourceIP:     "192.168.1.1",
		UserAgent:    "Mozilla/5.0",
		Detail:       "login successful",
	})

	logger.LogSync(&Entry{
		SubjectID:    "user1",
		Operation:    OpLogout,
		ResourceType: "session",
		Result:       ResultSuccess,
		SourceIP:     "192.168.1.1",
		Detail:       "logout",
	})

	logs := writer.ReadAll()
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
	if logs[0].Operation != OpLogin {
		t.Errorf("expected OpLogin, got %v", logs[0].Operation)
	}
	if logs[1].Operation != OpLogout {
		t.Errorf("expected OpLogout, got %v", logs[1].Operation)
	}
}

func TestMemoryWriter_NewAndBasics(t *testing.T) {
	mw := NewMemoryWriter()
	if mw == nil {
		t.Fatal("NewMemoryWriter returned nil")
	}
	if mw.Count() != 0 {
		t.Errorf("expected initial count 0, got %d", mw.Count())
	}

	log := &AuditLog{EventID: "test-1", SubjectID: "u"}
	if err := mw.Write(log); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if mw.Count() != 1 {
		t.Errorf("expected count 1, got %d", mw.Count())
	}

	logs := mw.ReadAll()
	if len(logs) != 1 {
		t.Fatalf("expected ReadAll len 1, got %d", len(logs))
	}
	if logs[0].EventID != "test-1" {
		t.Errorf("expected EventID 'test-1', got '%s'", logs[0].EventID)
	}

	log.SubjectID = "changed"
	logs2 := mw.ReadAll()
	if logs2[0].SubjectID == "changed" {
		t.Error("MemoryWriter should return copies, not references to internal state")
	}
}

func TestLogger_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("DefaultConfig MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.RetryInterval != 50*time.Millisecond {
		t.Errorf("DefaultConfig RetryInterval = %v, want 50ms", cfg.RetryInterval)
	}
	if cfg.BufferSize != 1024 {
		t.Errorf("DefaultConfig BufferSize = %d, want 1024", cfg.BufferSize)
	}
	if cfg.WorkerCount != 1 {
		t.Errorf("DefaultConfig WorkerCount = %d, want 1", cfg.WorkerCount)
	}
	if !cfg.EnableHashChain {
		t.Error("DefaultConfig EnableHashChain = false, want true")
	}
}

func TestLogger_HashChain_EmptyPreviousHash(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	logger.LogSync(makeEntry("u1", "r1", OpCreate, ResultSuccess))

	all := logger.Query(Query{})
	first := all[len(all)-1]
	if first.PreviousHash != "" {
		t.Errorf("first log PreviousHash should be empty, got '%s'", first.PreviousHash)
	}
	if first.CurrentHash == "" {
		t.Error("first log CurrentHash should not be empty")
	}
}

func TestLogger_AllOperationTypesLogged(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	ops := []OperationType{OpCreate, OpRead, OpUpdate, OpDelete, OpLogin, OpLogout, OpCustom}
	for _, op := range ops {
		logger.LogSync(makeEntry("u", "r", op, ResultSuccess))
	}

	logs := writer.ReadAll()
	if len(logs) != len(ops) {
		t.Fatalf("expected %d logs, got %d", len(ops), len(logs))
	}
	for i, op := range ops {
		if logs[i].Operation != op {
			t.Errorf("log %d: expected op %v, got %v", i, op, logs[i].Operation)
		}
		if logs[i].OperationDesc != op.String() {
			t.Errorf("log %d: expected OperationDesc '%s', got '%s'", i, op.String(), logs[i].OperationDesc)
		}
	}
}

func TestLogger_SyncWrite_TamperAfterWrite(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	logger.LogSync(makeEntry("u1", "r1", OpCreate, ResultSuccess))
	logger.LogSync(makeEntry("u1", "r2", OpRead, ResultSuccess))
	logger.LogSync(makeEntry("u1", "r3", OpUpdate, ResultSuccess))

	result := logger.VerifyIntegrity()
	if !result.Valid {
		t.Fatalf("initial integrity check failed: %s", result.Message)
	}

	logger.tamperLog(1, "CHANGED")

	result2 := logger.VerifyIntegrity()
	if result2.Valid {
		t.Fatal("integrity check should fail after tamper")
	}
	if result2.TamperedIndex != 1 {
		t.Errorf("expected TamperedIndex 1, got %d", result2.TamperedIndex)
	}
}

func TestFix_Issue1_LogSync_NotStarted(t *testing.T) {
	writer := NewMemoryWriter()
	logger, err := NewLogger(writer)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	err = logger.LogSync(makeEntry("user1", "res1", OpCreate, ResultSuccess))
	if !errors.Is(err, ErrLoggerStopped) {
		t.Errorf("LogSync before Start should return ErrLoggerStopped, got %v", err)
	}

	err = logger.Log(makeEntry("user1", "res1", OpCreate, ResultSuccess))
	if !errors.Is(err, ErrLoggerStopped) {
		t.Errorf("Log before Start should return ErrLoggerStopped, got %v", err)
	}

	if logger.Count() != 0 {
		t.Errorf("expected 0 logs, got %d", logger.Count())
	}
}

func TestFix_Issue1_LogSync_AfterStop(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	logger.Stop()

	err := logger.LogSync(makeEntry("user1", "res1", OpCreate, ResultSuccess))
	if !errors.Is(err, ErrLoggerStopped) {
		t.Errorf("LogSync after Stop should return ErrLoggerStopped, got %v", err)
	}

	err = logger.Log(makeEntry("user1", "res1", OpCreate, ResultSuccess))
	if !errors.Is(err, ErrLoggerStopped) {
		t.Errorf("Log after Stop should return ErrLoggerStopped, got %v", err)
	}
}

func TestFix_Issue2_ConcurrentLogAndStop_NoPanic(t *testing.T) {
	for round := 0; round < 50; round++ {
		cfg := Config{
			MaxRetries:      0,
			RetryInterval:   0,
			BufferSize:      16,
			WorkerCount:     2,
			EnableHashChain: true,
		}
		writer := NewMemoryWriter()
		logger, err := NewLoggerWithConfig(writer, cfg)
		if err != nil {
			t.Fatalf("NewLoggerWithConfig failed: %v", err)
		}
		if err := logger.Start(); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		go func() {
			for i := 0; i < 100; i++ {
				_ = logger.Log(&Entry{
					SubjectID:    "u",
					Operation:    OpCreate,
					ResourceID:   fmt.Sprintf("r%d", i),
					ResourceType: "concurrent",
					Result:       ResultSuccess,
					SourceIP:     "127.0.0.1",
				})
			}
		}()

		time.Sleep(1 * time.Millisecond)
		logger.Stop()
	}
}

func TestFix_Issue2_StopFlushesAllPendingLogs(t *testing.T) {
	cfg := Config{
		MaxRetries:      0,
		RetryInterval:   0,
		BufferSize:      1000,
		WorkerCount:     1,
		EnableHashChain: true,
	}
	slow := &slowWriter{delay: 5 * time.Millisecond}
	logger := startLoggerWithConfig(t, slow, cfg)

	total := 50
	for i := 0; i < total; i++ {
		err := logger.Log(makeEntry("u", fmt.Sprintf("r%d", i), OpCreate, ResultSuccess))
		if err != nil {
			t.Fatalf("Log failed at %d: %v", i, err)
		}
	}

	logger.Stop()

	if slow.Count() != total {
		t.Errorf("expected all %d logs flushed after Stop, got %d", total, slow.Count())
	}
}

func TestFix_Issue2_SendToClosedChannel_NoPanic(t *testing.T) {
	cfg := Config{
		MaxRetries:      0,
		RetryInterval:   0,
		BufferSize:      1,
		WorkerCount:     1,
		EnableHashChain: true,
	}

	blocking := &blockingWriter{}
	logger, err := NewLoggerWithConfig(blocking, cfg)
	if err != nil {
		t.Fatalf("NewLoggerWithConfig failed: %v", err)
	}
	if err := logger.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	blocking.blockWrites()

	_ = logger.Log(makeEntry("u", "r1", OpCreate, ResultSuccess))
	_ = logger.Log(makeEntry("u", "r2", OpCreate, ResultSuccess))

	go func() {
		time.Sleep(10 * time.Millisecond)
		blocking.unblockWrites()
	}()

	logger.Stop()
}

type blockingWriter struct {
	mu       sync.Mutex
	blockCh  chan struct{}
	logs     []*AuditLog
}

func (bw *blockingWriter) Write(log *AuditLog) error {
	if bw.blockCh != nil {
		<-bw.blockCh
	}
	bw.mu.Lock()
	defer bw.mu.Unlock()
	cp := *log
	bw.logs = append(bw.logs, &cp)
	return nil
}

func (bw *blockingWriter) blockWrites() {
	bw.blockCh = make(chan struct{})
}

func (bw *blockingWriter) unblockWrites() {
	if bw.blockCh != nil {
		close(bw.blockCh)
		bw.blockCh = nil
	}
}

func (bw *blockingWriter) Count() int {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	return len(bw.logs)
}

func TestFix_Issue3_StopWaitsForAsyncDegradeGoroutines(t *testing.T) {
	cfg := Config{
		MaxRetries:      0,
		RetryInterval:   0,
		BufferSize:      1,
		WorkerCount:     1,
		EnableHashChain: true,
	}

	slowFail := &slowFailingWriter{delay: 50 * time.Millisecond}
	logger := startLoggerWithConfig(t, slowFail, cfg)

	var degradeCalled int32
	var degradeCompleted int32
	logger.SetDegradeHandler(func(entry *Entry, err error) {
		atomic.AddInt32(&degradeCalled, 1)
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&degradeCompleted, 1)
	})

	slowFail.blockWrites()

	for i := 0; i < 5; i++ {
		_ = logger.Log(makeEntry("u", fmt.Sprintf("r%d", i), OpCreate, ResultSuccess))
	}

	slowFail.unblockWrites()

	logger.Stop()

	called := atomic.LoadInt32(&degradeCalled)
	completed := atomic.LoadInt32(&degradeCompleted)
	if called == 0 {
		t.Fatal("expected degrade handler to be called")
	}
	if called != completed {
		t.Errorf("degrade handler not all completed before Stop returned: called=%d, completed=%d", called, completed)
	}
}

type slowFailingWriter struct {
	mu       sync.Mutex
	blockCh  chan struct{}
	delay    time.Duration
	callCnt  int32
}

func (sfw *slowFailingWriter) Write(log *AuditLog) error {
	atomic.AddInt32(&sfw.callCnt, 1)
	if sfw.blockCh != nil {
		<-sfw.blockCh
	}
	time.Sleep(sfw.delay)
	return errors.New("simulated failure for degrade test")
}

func (sfw *slowFailingWriter) blockWrites() {
	sfw.blockCh = make(chan struct{})
}

func (sfw *slowFailingWriter) unblockWrites() {
	if sfw.blockCh != nil {
		close(sfw.blockCh)
		sfw.blockCh = nil
	}
}

func TestFix_Issue3_AsyncGoroutineWithoutDegrade(t *testing.T) {
	cfg := Config{
		MaxRetries:      0,
		RetryInterval:   0,
		BufferSize:      1,
		WorkerCount:     1,
		EnableHashChain: true,
	}

	slow := &slowWriter{delay: 20 * time.Millisecond}
	logger := startLoggerWithConfig(t, slow, cfg)

	for i := 0; i < 10; i++ {
		_ = logger.Log(makeEntry("u", fmt.Sprintf("r%d", i), OpCreate, ResultSuccess))
	}

	logger.Stop()

	if slow.Count() != 10 {
		t.Errorf("expected all 10 async writes to complete before Stop returns, got %d", slow.Count())
	}
}

func TestFix_Issue4_VerifyIntegrity_DetectsWriterTamper(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	for i := 0; i < 5; i++ {
		logger.LogSync(makeEntry(fmt.Sprintf("u%d", i), fmt.Sprintf("r%d", i), OpCreate, ResultSuccess))
	}

	waitForCondition(t, func() bool { return writer.Count() == 5 }, 2*time.Second, "all logs written")

	result := logger.VerifyIntegrity()
	if !result.Valid {
		t.Fatalf("initial integrity check failed: %s", result.Message)
	}

	writer.TamperLog(2, "TAMPERED in writer")

	result2 := logger.VerifyIntegrity()
	if result2.Valid {
		t.Fatal("integrity check should fail after writer tamper")
	}
	if result2.TamperedIndex != 2 {
		t.Errorf("expected TamperedIndex 2 for writer tamper, got %d (msg: %s)", result2.TamperedIndex, result2.Message)
	}
}

func TestFix_Issue4_VerifyIntegrity_DetectsWriterExtraLog(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	for i := 0; i < 3; i++ {
		logger.LogSync(makeEntry(fmt.Sprintf("u%d", i), fmt.Sprintf("r%d", i), OpCreate, ResultSuccess))
	}

	waitForCondition(t, func() bool { return writer.Count() == 3 }, 2*time.Second, "all logs written")

	spoof := &AuditLog{
		EventID:       "spoofed-event-id",
		SubjectID:     "hacker",
		Operation:     OpDelete,
		OperationDesc: "DELETE",
		Result:        ResultSuccess,
		CurrentHash:   "fake-hash",
	}
	writer.AppendSpoofedLog(spoof)

	result := logger.VerifyIntegrity()
	if result.Valid {
		t.Fatal("integrity check should fail with extra writer log")
	}
	if result.TamperedIndex != 3 {
		t.Errorf("expected TamperedIndex 3 for extra log, got %d (msg: %s)", result.TamperedIndex, result.Message)
	}
}

func TestFix_Issue4_VerifyIntegrity_DetectsWriterHashMismatch(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	for i := 0; i < 3; i++ {
		logger.LogSync(makeEntry(fmt.Sprintf("u%d", i), fmt.Sprintf("r%d", i), OpCreate, ResultSuccess))
	}

	waitForCondition(t, func() bool { return writer.Count() == 3 }, 2*time.Second, "all logs written")

	logs := writer.ReadAll()
	targetIdx := len(logs) - 1
	logs[targetIdx].CurrentHash = "manipulated-hash"
	writer.mu.Lock()
	cp := *logs[targetIdx]
	writer.logs[targetIdx] = &cp
	writer.mu.Unlock()

	result := logger.VerifyIntegrity()
	if result.Valid {
		t.Fatal("integrity check should fail for hash mismatch")
	}
	if result.TamperedIndex != targetIdx {
		t.Errorf("expected TamperedIndex %d for hash mismatch on last log, got %d (msg: %s)", targetIdx, result.TamperedIndex, result.Message)
	}
	if !containsSubstring(result.Message, "hash") {
		t.Errorf("expected Message to contain 'hash' to indicate hash-related failure, got: %s", result.Message)
	}
	if result.FullySynchronized {
		t.Error("expected FullySynchronized to be false when hash mismatch detected")
	}
}

func TestFix_Issue4_VerifyIntegrity_DetectsWriterBrokenChain(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	for i := 0; i < 4; i++ {
		logger.LogSync(makeEntry(fmt.Sprintf("u%d", i), fmt.Sprintf("r%d", i), OpCreate, ResultSuccess))
	}

	waitForCondition(t, func() bool { return writer.Count() == 4 }, 2*time.Second, "all logs written")

	writer.BreakChain(2)

	result := logger.VerifyIntegrity()
	if result.Valid {
		t.Fatal("integrity check should fail for broken chain in writer")
	}
	if result.TamperedIndex != 2 {
		t.Errorf("expected TamperedIndex 2 for broken chain at index 2, got %d", result.TamperedIndex)
	}
	if result.Message == "" {
		t.Fatal("expected non-empty Message")
	}
	if !containsSubstring(result.Message, "broken") {
		t.Errorf("expected Message to contain 'broken' to indicate chain break, got: %s", result.Message)
	}
	if !containsSubstring(result.Message, "chain") {
		t.Errorf("expected Message to contain 'chain' to indicate hash chain issue, got: %s", result.Message)
	}
	if result.FullySynchronized {
		t.Error("expected FullySynchronized to be false when chain is broken")
	}
}

func TestFix_Issue4_VerifyIntegrity_DetectsWriterDataLoss_AfterStop(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)

	total := 6
	for i := 0; i < total; i++ {
		logger.LogSync(makeEntry(fmt.Sprintf("u%d", i), fmt.Sprintf("r%d", i), OpCreate, ResultSuccess))
	}

	logger.Stop()

	result := logger.VerifyIntegrityStrict()
	if !result.Valid {
		t.Fatalf("integrity check should pass before data loss: %s", result.Message)
	}
	if !result.FullySynchronized {
		t.Error("expected FullySynchronized to be true after Stop")
	}
	if writer.Count() != total {
		t.Fatalf("expected writer to have %d logs, got %d", total, writer.Count())
	}

	removed := writer.Truncate(2)
	if removed != 2 {
		t.Fatalf("expected to truncate 2 logs, removed %d", removed)
	}

	result2 := logger.VerifyIntegrityStrict()
	if result2.Valid {
		t.Fatal("integrity check should fail after writer data loss")
	}
	if result2.TamperedIndex != total-2 {
		t.Errorf("expected TamperedIndex %d (after 4 remaining), got %d", total-2, result2.TamperedIndex)
	}
	if !containsSubstring(result2.Message, "data loss") {
		t.Errorf("expected Message to contain 'data loss', got: %s", result2.Message)
	}
	if result2.FullySynchronized {
		t.Error("expected FullySynchronized to be false after data loss")
	}
}

func TestFix_Issue4_VerifyIntegrity_PendingFlush_BeforeStop(t *testing.T) {
	cfg := Config{
		MaxRetries:      0,
		RetryInterval:   0,
		BufferSize:      100,
		WorkerCount:     1,
		EnableHashChain: true,
	}
	slow := &slowWriter{delay: 50 * time.Millisecond}
	logger, err := NewLoggerWithConfig(slow, cfg)
	if err != nil {
		t.Fatalf("NewLoggerWithConfig failed: %v", err)
	}
	if err := logger.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		_ = logger.Log(makeEntry(fmt.Sprintf("u%d", i), fmt.Sprintf("r%d", i), OpCreate, ResultSuccess))
	}

	time.Sleep(10 * time.Millisecond)

	result := logger.VerifyIntegrity()
	if !result.Valid {
		t.Fatalf("integrity check should pass with pending flush (before Stop): %s", result.Message)
	}
	if !containsSubstring(result.Message, "pending flush") {
		t.Errorf("expected Message to contain 'pending flush' while running, got: %s", result.Message)
	}

	logger.Stop()

	result2 := logger.VerifyIntegrity()
	if !result2.Valid {
		t.Fatalf("integrity check should pass after Stop (fully synced): %s", result2.Message)
	}
	if !containsSubstring(result2.Message, "fully synchronized") {
		t.Errorf("expected Message to contain 'fully synchronized' after Stop, got: %s", result2.Message)
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || containsSubstrHelper(s, sub))
}

func containsSubstrHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestFix_Issue4_VerifyIntegrity_WriterDeleteSpecificLog(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)

	for i := 0; i < 5; i++ {
		logger.LogSync(makeEntry(fmt.Sprintf("u%d", i), fmt.Sprintf("r%d", i), OpCreate, ResultSuccess))
	}

	logger.Stop()

	result := logger.VerifyIntegrityStrict()
	if !result.Valid {
		t.Fatalf("initial integrity check failed: %s", result.Message)
	}

	ok := writer.DeleteLog(2)
	if !ok {
		t.Fatal("DeleteLog returned false")
	}

	result2 := logger.VerifyIntegrityStrict()
	if result2.Valid {
		t.Fatal("integrity check should fail after deleting a log from writer")
	}
	if !containsSubstring(result2.Message, "data loss") {
		t.Errorf("expected 'data loss' in message, got: %s", result2.Message)
	}
}

func TestFix_Issue4_VerifyIntegrity_WriterLogNotInMemory(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	for i := 0; i < 2; i++ {
		logger.LogSync(makeEntry(fmt.Sprintf("u%d", i), fmt.Sprintf("r%d", i), OpCreate, ResultSuccess))
	}

	waitForCondition(t, func() bool { return writer.Count() == 2 }, 2*time.Second, "all logs written")

	spoof := &AuditLog{
		EventID:       "audit-9999999999999999999-999",
		SubjectID:     "ghost",
		Operation:     OpCreate,
		OperationDesc: "CREATE",
		Result:        ResultSuccess,
		CurrentHash:   computeHash(&AuditLog{
			EventID:       "audit-9999999999999999999-999",
			SubjectID:     "ghost",
			Operation:     OpCreate,
			OperationDesc: "CREATE",
			Result:        ResultSuccess,
			CurrentHash:   "",
		}),
	}
	spoof.CurrentHash = computeHash(spoof)
	writer.AppendSpoofedLog(spoof)

	result := logger.VerifyIntegrity()
	if result.Valid {
		t.Fatal("integrity check should fail for writer log not in memory")
	}
}

func TestFix_Issue4_MultipleWorkers_VerifyIntegrity(t *testing.T) {
	cfg := Config{
		MaxRetries:      0,
		RetryInterval:   0,
		BufferSize:      100,
		WorkerCount:     4,
		EnableHashChain: true,
	}
	writer := NewMemoryWriter()
	logger := startLoggerWithConfig(t, writer, cfg)

	for i := 0; i < 50; i++ {
		logger.Log(makeEntry(fmt.Sprintf("u%d", i), fmt.Sprintf("r%d", i), OpCreate, ResultSuccess))
	}

	waitForCondition(t, func() bool { return writer.Count() == 50 }, 3*time.Second, "all writes complete")

	result := logger.VerifyIntegrity()
	if !result.Valid {
		t.Errorf("integrity check failed with multiple workers: %s", result.Message)
	}

	logger.Stop()

	result2 := logger.VerifyIntegrity()
	if !result2.Valid {
		t.Errorf("integrity check after Stop failed: %s", result2.Message)
	}
	if result2.TamperedIndex != -1 {
		t.Errorf("expected TamperedIndex -1, got %d", result2.TamperedIndex)
	}
}

func TestFix_Issue4_WriterWithDuplicateLogs(t *testing.T) {
	writer := NewMemoryWriter()
	logger := startLogger(t, writer)
	defer logger.Stop()

	logger.LogSync(makeEntry("u1", "r1", OpCreate, ResultSuccess))
	logger.LogSync(makeEntry("u2", "r2", OpCreate, ResultSuccess))

	waitForCondition(t, func() bool { return writer.Count() == 2 }, 2*time.Second, "all logs written")

	logs := writer.ReadAll()
	writer.AppendSpoofedLog(logs[0])

	result := logger.VerifyIntegrity()
	if result.Valid {
		t.Fatal("integrity check should fail for duplicate writer log")
	}
	if result.TamperedIndex != 2 {
		t.Errorf("expected TamperedIndex 2 for duplicate, got %d (msg: %s)", result.TamperedIndex, result.Message)
	}
}

type nonReadableWriter struct {
	mu   sync.Mutex
	logs []*AuditLog
}

func (nw *nonReadableWriter) Write(log *AuditLog) error {
	nw.mu.Lock()
	defer nw.mu.Unlock()
	cp := *log
	nw.logs = append(nw.logs, &cp)
	return nil
}

func TestFix_Issue4_NonReadableWriter_OnlyMemoryCheck(t *testing.T) {
	sw := &nonReadableWriter{}

	logger := startLogger(t, sw)
	defer logger.Stop()

	for i := 0; i < 3; i++ {
		logger.LogSync(makeEntry(fmt.Sprintf("u%d", i), fmt.Sprintf("r%d", i), OpCreate, ResultSuccess))
	}

	result := logger.VerifyIntegrity()
	if !result.Valid {
		t.Errorf("integrity check should pass with non-ReadableWriter: %s", result.Message)
	}
}

func TestLogger_LogSync_StartedStoppedConsistency(t *testing.T) {
	writer := NewMemoryWriter()
	logger, err := NewLogger(writer)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	logErr := logger.Log(makeEntry("u", "r", OpCreate, ResultSuccess))
	syncErr := logger.LogSync(makeEntry("u", "r", OpCreate, ResultSuccess))

	if !errors.Is(logErr, ErrLoggerStopped) {
		t.Errorf("Log before Start: expected ErrLoggerStopped, got %v", logErr)
	}
	if !errors.Is(syncErr, ErrLoggerStopped) {
		t.Errorf("LogSync before Start: expected ErrLoggerStopped, got %v", syncErr)
	}

	if err := logger.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	logErr = logger.Log(makeEntry("u", "r", OpCreate, ResultSuccess))
	if logErr != nil {
		t.Errorf("Log after Start: expected nil, got %v", logErr)
	}
	syncErr = logger.LogSync(makeEntry("u", "r", OpCreate, ResultSuccess))
	if syncErr != nil {
		t.Errorf("LogSync after Start: expected nil, got %v", syncErr)
	}

	logger.Stop()

	logErr = logger.Log(makeEntry("u", "r", OpCreate, ResultSuccess))
	syncErr = logger.LogSync(makeEntry("u", "r", OpCreate, ResultSuccess))

	if !errors.Is(logErr, ErrLoggerStopped) {
		t.Errorf("Log after Stop: expected ErrLoggerStopped, got %v", logErr)
	}
	if !errors.Is(syncErr, ErrLoggerStopped) {
		t.Errorf("LogSync after Stop: expected ErrLoggerStopped, got %v", syncErr)
	}
}

