package structlog

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestLogger(level Level) (*Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := New(&buf, level)
	return logger, &buf
}

func parseLogEntries(data []byte) []map[string]interface{} {
	lines := bytes.Split(data, []byte("\n"))
	var entries []map[string]interface{}
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{Level(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestNewNilOutput(t *testing.T) {
	logger := New(nil, LevelDebug)
	if logger == nil {
		t.Fatal("New(nil, ...) returned nil")
	}
	if logger.state.output == nil {
		t.Error("output should default to os.Stdout when nil is passed")
	}
}

func TestJSONOutputFormat(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	logger.Info("hello world")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	entry := entries[0]

	if _, ok := entry["ts"]; !ok {
		t.Error("missing 'ts' field")
	}
	if _, ok := entry["level"]; !ok {
		t.Error("missing 'level' field")
	}
	if _, ok := entry["msg"]; !ok {
		t.Error("missing 'msg' field")
	}
	if _, ok := entry["caller"]; !ok {
		t.Error("missing 'caller' field")
	}

	if entry["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", entry["level"])
	}
	if entry["msg"] != "hello world" {
		t.Errorf("msg = %v, want 'hello world'", entry["msg"])
	}
}

func TestTimestampISO8601(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	logger.Info("ts test")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	tsStr, ok := entries[0]["ts"].(string)
	if !ok {
		t.Fatal("ts is not a string")
	}

	_, err := time.Parse(time.RFC3339Nano, tsStr)
	if err != nil {
		t.Errorf("timestamp %q is not valid ISO 8601 / RFC3339Nano: %v", tsStr, err)
	}
}

func TestLevelFiltering(t *testing.T) {
	logger, buf := newTestLogger(LevelWarn)

	logger.Debug("debug msg")
	logger.Info("info msg")
	logger.Warn("warn msg")
	logger.Error("error msg")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (Warn+Error), got %d", len(entries))
	}

	for _, entry := range entries {
		lvl := entry["level"].(string)
		if lvl != "WARN" && lvl != "ERROR" {
			t.Errorf("unexpected level %q in output", lvl)
		}
	}
}

func TestLevelFilteringDebug(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)

	logger.Debug("d")
	logger.Info("i")
	logger.Warn("w")
	logger.Error("e")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
}

func TestLevelFilteringError(t *testing.T) {
	logger, buf := newTestLogger(LevelError)

	logger.Debug("d")
	logger.Info("i")
	logger.Warn("w")
	logger.Error("e")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (Error only), got %d", len(entries))
	}
	if entries[0]["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", entries[0]["level"])
	}
}

func TestSetLevel(t *testing.T) {
	logger, buf := newTestLogger(LevelError)

	logger.Info("before switch")
	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries before level switch, got %d", len(entries))
	}

	logger.SetLevel(LevelDebug)
	logger.Info("after switch")

	entries = parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after level switch, got %d", len(entries))
	}
	if entries[0]["msg"] != "after switch" {
		t.Errorf("msg = %v, want 'after switch'", entries[0]["msg"])
	}
}

func TestSetLevelImmediateEffect(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)

	logger.Debug("1")
	logger.SetLevel(LevelWarn)
	logger.Debug("2")
	logger.Info("3")
	logger.Warn("4")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (Debug#1 + Warn#4), got %d", len(entries))
	}
	if entries[0]["msg"] != "1" {
		t.Errorf("first entry msg = %v, want '1'", entries[0]["msg"])
	}
	if entries[1]["msg"] != "4" {
		t.Errorf("second entry msg = %v, want '4'", entries[1]["msg"])
	}
}

func TestGetLevel(t *testing.T) {
	logger, _ := newTestLogger(LevelInfo)
	if logger.GetLevel() != LevelInfo {
		t.Errorf("GetLevel() = %v, want Info", logger.GetLevel())
	}
	logger.SetLevel(LevelWarn)
	if logger.GetLevel() != LevelWarn {
		t.Errorf("GetLevel() = %v, want Warn", logger.GetLevel())
	}
}

func TestWithFields(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)

	parent := logger.WithFields(map[string]interface{}{"service": "api", "version": 1})
	parent.Info("with fields")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0]["service"] != "api" {
		t.Errorf("service = %v, want 'api'", entries[0]["service"])
	}
	if entries[0]["version"] != float64(1) {
		t.Errorf("version = %v, want 1", entries[0]["version"])
	}
}

func TestWithFieldsInheritance(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)

	parent := logger.WithFields(map[string]interface{}{"env": "prod", "region": "us"})
	child := parent.WithFields(map[string]interface{}{"region": "eu", "app": "web"})

	child.Info("child log")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0]["env"] != "prod" {
		t.Errorf("env = %v, want 'prod' (inherited from parent)", entries[0]["env"])
	}
	if entries[0]["region"] != "eu" {
		t.Errorf("region = %v, want 'eu' (child overrides parent)", entries[0]["region"])
	}
	if entries[0]["app"] != "web" {
		t.Errorf("app = %v, want 'web' (child's own field)", entries[0]["app"])
	}
}

func TestWithFieldsNoModifyParent(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)

	parent := logger.WithFields(map[string]interface{}{"env": "prod"})
	child := parent.WithFields(map[string]interface{}{"app": "web"})

	parent.Info("parent msg")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if _, ok := entries[0]["app"]; ok {
		t.Error("parent entry should not have 'app' field from child")
	}
	if entries[0]["env"] != "prod" {
		t.Errorf("env = %v, want 'prod'", entries[0]["env"])
	}

	buf.Reset()
	child.Info("child msg")
	entries = parseLogEntries(buf.Bytes())
	if entries[0]["app"] != "web" {
		t.Errorf("child entry app = %v, want 'web'", entries[0]["app"])
	}
	if entries[0]["env"] != "prod" {
		t.Errorf("child entry env = %v, want 'prod'", entries[0]["env"])
	}
}

func TestWithFieldsEmptyMap(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)

	child := logger.WithFields(map[string]interface{}{})
	child.Info("empty fields")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestWithFieldsNilMap(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)

	child := logger.WithFields(nil)
	child.Info("nil fields")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestSamplingRate(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	logger.SetSamplingRate(LevelDebug, 3)

	for i := 0; i < 9; i++ {
		logger.Debug("msg")
	}

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 3 {
		t.Errorf("expected 3 entries (1 out of 3), got %d", len(entries))
	}
}

func TestSamplingRateOne(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	logger.SetSamplingRate(LevelDebug, 1)

	for i := 0; i < 5; i++ {
		logger.Debug("msg")
	}

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 5 {
		t.Errorf("expected 5 entries (1 out of 1 = all), got %d", len(entries))
	}
}

func TestSamplingRateZero(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	logger.SetSamplingRate(LevelDebug, 5)

	for i := 0; i < 3; i++ {
		logger.Debug("sampled")
	}
	sampled := len(parseLogEntries(buf.Bytes()))

	buf.Reset()
	logger.SetSamplingRate(LevelDebug, 0)

	for i := 0; i < 3; i++ {
		logger.Debug("all")
	}
	all := len(parseLogEntries(buf.Bytes()))

	if sampled >= 3 {
		t.Errorf("with rate=5, expected <3 entries, got %d", sampled)
	}
	if all != 3 {
		t.Errorf("with rate=0, expected 3 entries, got %d", all)
	}
}

func TestSamplingRatePerLevel(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	logger.SetSamplingRate(LevelDebug, 5)
	logger.SetSamplingRate(LevelInfo, 2)

	for i := 0; i < 10; i++ {
		logger.Debug("d")
	}
	for i := 0; i < 10; i++ {
		logger.Info("i")
	}

	entries := parseLogEntries(buf.Bytes())
	debugCount := 0
	infoCount := 0
	for _, e := range entries {
		switch e["level"] {
		case "DEBUG":
			debugCount++
		case "INFO":
			infoCount++
		}
	}

	if debugCount != 2 {
		t.Errorf("debug entries = %d, want 2 (1 out of 5 from 10)", debugCount)
	}
	if infoCount != 5 {
		t.Errorf("info entries = %d, want 5 (1 out of 2 from 10)", infoCount)
	}
}

func TestSamplingRateIndependentCounters(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	logger.SetSamplingRate(LevelDebug, 10)
	logger.SetSamplingRate(LevelInfo, 10)

	logger.Debug("d1")
	logger.Info("i1")
	logger.Debug("d2")
	logger.Info("i2")

	entries := parseLogEntries(buf.Bytes())
	debugCount := 0
	infoCount := 0
	for _, e := range entries {
		switch e["level"] {
		case "DEBUG":
			debugCount++
		case "INFO":
			infoCount++
		}
	}

	if debugCount != 1 {
		t.Errorf("debug count = %d, want 1 (first of each level passes)", debugCount)
	}
	if infoCount != 1 {
		t.Errorf("info count = %d, want 1 (first of each level passes)", infoCount)
	}
}

func TestSamplingRateResetOnSet(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	logger.SetSamplingRate(LevelDebug, 10)

	for i := 0; i < 9; i++ {
		logger.Debug("before reset")
	}
	beforeReset := len(parseLogEntries(buf.Bytes()))

	buf.Reset()
	logger.SetSamplingRate(LevelDebug, 10)

	logger.Debug("after reset")
	afterEntries := parseLogEntries(buf.Bytes())

	if beforeReset != 1 {
		t.Errorf("before reset: expected 1, got %d", beforeReset)
	}
	if len(afterEntries) != 1 {
		t.Errorf("after reset: expected 1, got %d", len(afterEntries))
	}
}

func TestSamplingRateInvalidLevel(t *testing.T) {
	logger, _ := newTestLogger(LevelDebug)
	logger.SetSamplingRate(Level(-1), 10)
	logger.SetSamplingRate(Level(4), 10)
}

func TestErrorStack(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	logger.Error("something failed")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	stack, ok := entries[0]["stack"]
	if !ok {
		t.Fatal("Error level log should have 'stack' field")
	}

	stackArr, ok := stack.([]interface{})
	if !ok {
		t.Fatalf("stack field should be an array, got %T", stack)
	}
	if len(stackArr) == 0 {
		t.Error("stack should not be empty")
	}

	for _, frame := range stackArr {
		frameStr, ok := frame.(string)
		if !ok {
			t.Errorf("stack frame should be string, got %T", frame)
			continue
		}
		if !strings.Contains(frameStr, ":") {
			t.Errorf("stack frame %q should contain ':' separator", frameStr)
		}
	}
}

func TestErrorHasCaller(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	logger.Error("with caller")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if _, ok := entries[0]["caller"]; !ok {
		t.Error("Error level log should also have 'caller' field")
	}
}

func TestNonErrorNoStack(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)

	logger.Debug("d")
	logger.Info("i")
	logger.Warn("w")

	entries := parseLogEntries(buf.Bytes())
	for i, entry := range entries {
		if _, ok := entry["stack"]; ok {
			t.Errorf("entry %d (level=%v) should not have 'stack' field", i, entry["level"])
		}
	}
}

func TestNonErrorHasCaller(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)

	logger.Info("with caller")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	caller, ok := entries[0]["caller"].(string)
	if !ok {
		t.Fatal("caller field should be a string")
	}
	if !strings.Contains(caller, ".go:") {
		t.Errorf("caller %q should contain '.go:' pattern", caller)
	}
}

func TestOutputToBytesBuffer(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, LevelInfo)
	logger.Info("buffer test")

	if buf.Len() == 0 {
		t.Error("expected output in buffer")
	}

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0]["msg"] != "buffer test" {
		t.Errorf("msg = %v, want 'buffer test'", entries[0]["msg"])
	}
}

func TestEmptyMessage(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	logger.Info("")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0]["msg"] != "" {
		t.Errorf("msg = %v, want empty string", entries[0]["msg"])
	}
}

func TestFieldWithNilValue(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	child := logger.WithFields(map[string]interface{}{"key": nil})
	child.Info("nil value")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if v, ok := entries[0]["key"]; !ok {
		t.Error("missing 'key' field")
	} else if v != nil {
		t.Errorf("key = %v, want nil", v)
	}
}

func TestConcurrentLogging(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)

	var wg sync.WaitGroup
	goroutines := 4
	msgsPerGoroutine := 100

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < msgsPerGoroutine; i++ {
				logger.Info("concurrent msg")
			}
		}(g)
	}

	wg.Wait()

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != goroutines*msgsPerGoroutine {
		t.Errorf("expected %d entries, got %d", goroutines*msgsPerGoroutine, len(entries))
	}
}

func TestConcurrentSetLevel(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			logger.Info("msg")
		}()
		go func(val int) {
			defer wg.Done()
			if val%2 == 0 {
				logger.SetLevel(LevelDebug)
			} else {
				logger.SetLevel(LevelWarn)
			}
		}(i)
	}
	wg.Wait()

	entries := parseLogEntries(buf.Bytes())
	for _, e := range entries {
		if e["level"] != "INFO" && e["level"] != "WARN" && e["level"] != "ERROR" {
			t.Errorf("unexpected level %v", e["level"])
		}
	}
}

func TestConcurrentWithFields(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			child := logger.WithFields(map[string]interface{}{"goroutine": id})
			child.Info("from goroutine")
		}(i)
	}
	wg.Wait()

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 50 {
		t.Errorf("expected 50 entries, got %d", len(entries))
	}
}

func TestMultipleLogEntries(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)

	logger.Debug("first")
	logger.Info("second")
	logger.Warn("third")
	logger.Error("fourth")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	expected := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	for i, entry := range entries {
		if entry["level"] != expected[i] {
			t.Errorf("entry %d: level = %v, want %v", i, entry["level"], expected[i])
		}
	}
}

func TestEachEntryIsSeparateJSONLine(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)

	logger.Info("line1")
	logger.Info("line2")

	lines := bytes.Split(buf.Bytes(), []byte("\n"))
	nonEmpty := 0
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) > 0 {
			nonEmpty++
			var entry map[string]interface{}
			if err := json.Unmarshal(line, &entry); err != nil {
				t.Errorf("line %q is not valid JSON: %v", string(line), err)
			}
		}
	}
	if nonEmpty != 2 {
		t.Errorf("expected 2 non-empty lines, got %d", nonEmpty)
	}
}

func TestTimestampIsUTC(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	logger.Info("utc check")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	tsStr := entries[0]["ts"].(string)
	parsed, err := time.Parse(time.RFC3339Nano, tsStr)
	if err != nil {
		t.Fatalf("failed to parse timestamp: %v", err)
	}

	now := time.Now().UTC()
	diff := now.Sub(parsed)
	if diff < 0 {
		diff = -diff
	}
	if diff > 5*time.Second {
		t.Errorf("timestamp diff = %v, expected near current time", diff)
	}
}

func TestStackExcludesInternalFrames(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	logger.Error("stack test")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	stackArr := entries[0]["stack"].([]interface{})
	for _, frame := range stackArr {
		frameStr := frame.(string)
		if strings.Contains(frameStr, "structlog") {
			t.Errorf("stack should not contain structlog internal frames, got %q", frameStr)
		}
	}
}

func TestSamplingDoesNotAffectOtherLevels(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	logger.SetSamplingRate(LevelInfo, 100)

	logger.Info("sampled info")
	logger.Warn("unsampled warn")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0]["level"] != "INFO" {
		t.Errorf("first entry level = %v, want INFO", entries[0]["level"])
	}
	if entries[1]["level"] != "WARN" {
		t.Errorf("second entry level = %v, want WARN", entries[1]["level"])
	}
}

func TestLargeSamplingRate(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	logger.SetSamplingRate(LevelDebug, 1000000)

	logger.Debug("first")
	logger.Debug("second")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Errorf("expected 1 entry (1 out of 1000000), got %d", len(entries))
	}
	if entries[0]["msg"] != "first" {
		t.Errorf("expected first entry to pass, got msg=%v", entries[0]["msg"])
	}
}

func TestNestedWithFields(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)

	l1 := logger.WithFields(map[string]interface{}{"a": 1})
	l2 := l1.WithFields(map[string]interface{}{"b": 2})
	l3 := l2.WithFields(map[string]interface{}{"c": 3})

	l3.Info("nested")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0]["a"] != float64(1) {
		t.Errorf("a = %v, want 1", entries[0]["a"])
	}
	if entries[0]["b"] != float64(2) {
		t.Errorf("b = %v, want 2", entries[0]["b"])
	}
	if entries[0]["c"] != float64(3) {
		t.Errorf("c = %v, want 3", entries[0]["c"])
	}
}

func TestSamplingWithLevelChange(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	logger.SetSamplingRate(LevelDebug, 5)

	for i := 0; i < 5; i++ {
		logger.Debug("before")
	}

	logger.SetLevel(LevelInfo)

	buf.Reset()
	logger.SetLevel(LevelDebug)

	for i := 0; i < 5; i++ {
		logger.Debug("after")
	}

	entries := parseLogEntries(buf.Bytes())
	if len(entries) < 1 {
		t.Error("expected at least 1 entry after level change")
	}
}

func TestWithFieldsDoesNotAffectSampling(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	logger.SetSamplingRate(LevelInfo, 2)

	child := logger.WithFields(map[string]interface{}{"key": "val"})

	for i := 0; i < 4; i++ {
		child.Info("sampled")
	}

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 2 {
		t.Errorf("expected 2 entries (1 out of 2), got %d", len(entries))
	}

	for _, e := range entries {
		if e["key"] != "val" {
			t.Errorf("entry missing context field 'key'")
		}
	}
}

func TestLogEntryFieldTypes(t *testing.T) {
	logger, buf := newTestLogger(LevelDebug)
	child := logger.WithFields(map[string]interface{}{
		"str_field":  "hello",
		"int_field":  42,
		"bool_field": true,
	})
	child.Info("type test")

	entries := parseLogEntries(buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry["str_field"] != "hello" {
		t.Errorf("str_field = %v, want 'hello'", entry["str_field"])
	}
	if entry["int_field"] != float64(42) {
		t.Errorf("int_field = %v, want 42", entry["int_field"])
	}
	if entry["bool_field"] != true {
		t.Errorf("bool_field = %v, want true", entry["bool_field"])
	}
}
