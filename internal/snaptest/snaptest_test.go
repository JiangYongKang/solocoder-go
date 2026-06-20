package snaptest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testNestedStruct struct {
	Value int
	Name  string
	Tags  []string
}

type testComplexStruct struct {
	ID      int
	Name    string
	Nested  testNestedStruct
	Items   []int
	Mapping map[string]int
}

func TestSerialize_Nil(t *testing.T) {
	result, err := Serialize(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "null" {
		t.Errorf("expected 'null', got %q", result)
	}
}

func TestSerialize_PrimitiveTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"string", "hello", `"hello"`},
		{"int", 42, "42"},
		{"float", 3.14, "3.14"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Serialize(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSerialize_Struct(t *testing.T) {
	input := struct {
		Name string
		Age  int
	}{
		Name: "Alice",
		Age:  30,
	}

	result, err := Serialize(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "{\n  \"Name\": \"Alice\",\n  \"Age\": 30\n}"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestSerialize_ComplexStruct(t *testing.T) {
	input := testComplexStruct{
		ID:   1,
		Name: "test",
		Nested: testNestedStruct{
			Value: 100,
			Name:  "nested",
			Tags:  []string{"a", "b", "c"},
		},
		Items:   []int{1, 2, 3},
		Mapping: map[string]int{"x": 1, "y": 2},
	}

	result, err := Serialize(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "\"ID\": 1") {
		t.Error("serialized output missing ID field")
	}
	if !strings.Contains(result, "\"Name\": \"test\"") {
		t.Error("serialized output missing Name field")
	}
	if !strings.Contains(result, "\"Tags\": [") {
		t.Error("serialized output missing Tags array")
	}
}

func TestSerialize_Slice(t *testing.T) {
	input := []string{"a", "b", "c"}
	result, err := Serialize(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "[\n  \"a\",\n  \"b\",\n  \"c\"\n]"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestSerialize_Map(t *testing.T) {
	input := map[string]int{"a": 1, "b": 2}
	result, err := Serialize(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "\"a\": 1") {
		t.Error("serialized output missing key a")
	}
	if !strings.Contains(result, "\"b\": 2") {
		t.Error("serialized output missing key b")
	}
}

func TestDiff_Identical(t *testing.T) {
	text := "line1\nline2\nline3"
	result := Diff(text, text)
	if !result.Matches() {
		t.Error("expected identical texts to match")
	}
	if result.TotalSame != 3 {
		t.Errorf("expected 3 same lines, got %d", result.TotalSame)
	}
	if result.TotalAdded != 0 {
		t.Errorf("expected 0 added lines, got %d", result.TotalAdded)
	}
	if result.TotalRemoved != 0 {
		t.Errorf("expected 0 removed lines, got %d", result.TotalRemoved)
	}
	if len(result.Lines) != 3 {
		t.Errorf("expected 3 diff lines, got %d", len(result.Lines))
	}
	for _, l := range result.Lines {
		if l.Type != DiffSame {
			t.Error("all lines should be DiffSame")
		}
	}
}

func TestDiff_CompletelyDifferent(t *testing.T) {
	expected := "a\nb\nc"
	actual := "x\ny\nz"
	result := Diff(expected, actual)
	if result.Matches() {
		t.Error("expected different texts not to match")
	}
	if result.TotalSame != 0 {
		t.Errorf("expected 0 same lines, got %d", result.TotalSame)
	}
	if result.TotalRemoved != 3 {
		t.Errorf("expected 3 removed lines, got %d", result.TotalRemoved)
	}
	if result.TotalAdded != 3 {
		t.Errorf("expected 3 added lines, got %d", result.TotalAdded)
	}
}

func TestDiff_PartialChanges(t *testing.T) {
	expected := "line1\nline2\nline3\nline4\nline5"
	actual := "line1\nline2_modified\nline3\nline4_new\nline5"
	result := Diff(expected, actual)

	if result.TotalSame < 2 {
		t.Errorf("expected at least 2 same lines, got %d", result.TotalSame)
	}
	if result.TotalRemoved < 1 {
		t.Errorf("expected at least 1 removed line, got %d", result.TotalRemoved)
	}
	if result.TotalAdded < 1 {
		t.Errorf("expected at least 1 added line, got %d", result.TotalAdded)
	}
}

func TestDiff_AddLinesAtEnd(t *testing.T) {
	expected := "a\nb"
	actual := "a\nb\nc\nd"
	result := Diff(expected, actual)
	if result.TotalSame != 2 {
		t.Errorf("expected 2 same lines, got %d", result.TotalSame)
	}
	if result.TotalAdded != 2 {
		t.Errorf("expected 2 added lines, got %d", result.TotalAdded)
	}
	if result.TotalRemoved != 0 {
		t.Errorf("expected 0 removed lines, got %d", result.TotalRemoved)
	}
}

func TestDiff_RemoveLinesAtBeginning(t *testing.T) {
	expected := "a\nb\nc\nd"
	actual := "c\nd"
	result := Diff(expected, actual)
	if result.TotalSame != 2 {
		t.Errorf("expected 2 same lines, got %d", result.TotalSame)
	}
	if result.TotalRemoved != 2 {
		t.Errorf("expected 2 removed lines, got %d", result.TotalRemoved)
	}
	if result.TotalAdded != 0 {
		t.Errorf("expected 0 added lines, got %d", result.TotalAdded)
	}
}

func TestDiff_EmptyStrings(t *testing.T) {
	result := Diff("", "")
	if !result.Matches() {
		t.Error("empty strings should match")
	}
	if len(result.Lines) != 0 {
		t.Errorf("expected 0 lines, got %d", len(result.Lines))
	}
}

func TestDiff_EmptyVsNonEmpty(t *testing.T) {
	result := Diff("", "hello")
	if result.Matches() {
		t.Error("empty vs non-empty should not match")
	}
	if result.TotalAdded != 1 {
		t.Errorf("expected 1 added line, got %d", result.TotalAdded)
	}
}

func TestDiff_WindowsLineEndings(t *testing.T) {
	expected := "line1\r\nline2\r\nline3"
	actual := "line1\nline2\nline3"
	result := Diff(expected, actual)
	if !result.Matches() {
		t.Error("texts differing only in line endings should match")
	}
}

func TestDiff_SingleLine(t *testing.T) {
	result := Diff("hello", "hello")
	if !result.Matches() {
		t.Error("single identical lines should match")
	}

	result = Diff("hello", "world")
	if result.Matches() {
		t.Error("different single lines should not match")
	}
	if result.TotalRemoved != 1 {
		t.Errorf("expected 1 removed line, got %d", result.TotalRemoved)
	}
	if result.TotalAdded != 1 {
		t.Errorf("expected 1 added line, got %d", result.TotalAdded)
	}
}

func TestDiffResult_Format_NoDiff(t *testing.T) {
	text := "a\nb\nc"
	result := Diff(text, text)
	formatted := result.Format(3)
	if formatted != "" {
		t.Errorf("expected empty format for matching diff, got:\n%s", formatted)
	}
}

func TestDiffResult_Format_WithDiff(t *testing.T) {
	expected := "line1\nline2\nline3\nline4\nline5"
	actual := "line1\nline2_changed\nline3\nline4\nline5"
	result := Diff(expected, actual)
	formatted := result.Format(3)

	if formatted == "" {
		t.Fatal("expected non-empty format")
	}
	if !strings.Contains(formatted, "--- Expected") {
		t.Error("format should contain expected header")
	}
	if !strings.Contains(formatted, "+++ Actual") {
		t.Error("format should contain actual header")
	}
	if !strings.Contains(formatted, "-") {
		t.Error("format should contain removed line marker")
	}
	if !strings.Contains(formatted, "+") {
		t.Error("format should contain added line marker")
	}
	if !strings.Contains(formatted, "Summary:") {
		t.Error("format should contain summary")
	}
}

func TestDiffResult_Format_ContextLines(t *testing.T) {
	expected := "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10"
	actual := "l1\nl2\nl3\nMODIFIED\nl5\nl6\nl7\nl8\nl9\nl10"
	result := Diff(expected, actual)

	full := result.Format(100)
	with1 := result.Format(1)
	_ = with1

	if !strings.Contains(full, "l1") {
		t.Error("full context should contain l1")
	}
}

func TestDiffResult_Format_NegativeContext(t *testing.T) {
	expected := "a\nb\nc"
	actual := "a\nX\nc"
	result := Diff(expected, actual)
	formatted := result.Format(-1)
	if formatted == "" {
		t.Error("should still format with negative context (treated as 0)")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.SnapshotDir != defaultSnapshotDir {
		t.Errorf("expected SnapshotDir %q, got %q", defaultSnapshotDir, cfg.SnapshotDir)
	}
	if cfg.UpdateMode != false {
		t.Error("expected UpdateMode false by default")
	}
	if cfg.ContextLines != defaultContextLines {
		t.Errorf("expected ContextLines %d, got %d", defaultContextLines, cfg.ContextLines)
	}
}

func TestNewWithConfig_Normalization(t *testing.T) {
	cfg := Config{
		SnapshotDir:  "",
		ContextLines: -5,
	}
	m := NewWithConfig(cfg)
	got := m.Config()

	if got.SnapshotDir != defaultSnapshotDir {
		t.Errorf("expected SnapshotDir normalized to %q, got %q", defaultSnapshotDir, got.SnapshotDir)
	}
	if got.ContextLines != defaultContextLines {
		t.Errorf("expected ContextLines normalized to %d, got %d", defaultContextLines, got.ContextLines)
	}
}

func TestNew_UpdateModeFromEnv(t *testing.T) {
	orig, hadOrig := os.LookupEnv(updateEnvVar)
	defer func() {
		if hadOrig {
			os.Setenv(updateEnvVar, orig)
		} else {
			os.Unsetenv(updateEnvVar)
		}
	}()

	tests := []struct {
		envValue string
		expected bool
	}{
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"yes", true},
		{"YES", true},
		{"0", false},
		{"false", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.envValue, func(t *testing.T) {
			os.Setenv(updateEnvVar, tt.envValue)
			m := New()
			if m.Config().UpdateMode != tt.expected {
				t.Errorf("env %q: expected UpdateMode=%v, got %v", tt.envValue, tt.expected, m.Config().UpdateMode)
			}
		})
	}
}

func TestNewWithConfig_ExplicitUpdateModeOverridesEnv(t *testing.T) {
	orig, hadOrig := os.LookupEnv(updateEnvVar)
	defer func() {
		if hadOrig {
			os.Setenv(updateEnvVar, orig)
		} else {
			os.Unsetenv(updateEnvVar)
		}
	}()

	os.Setenv(updateEnvVar, "1")
	m := NewWithConfig(Config{UpdateMode: false})
	if m.Config().UpdateMode {
		t.Error("explicit UpdateMode=false should be kept even with env var set")
	}

	os.Unsetenv(updateEnvVar)
	m2 := NewWithConfig(Config{UpdateMode: true})
	if !m2.Config().UpdateMode {
		t.Error("explicit UpdateMode=true should be kept")
	}
}

func TestMatcher_SnapshotPath_Valid(t *testing.T) {
	m := New()
	path, err := m.snapshotPath("test_case")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(defaultSnapshotDir, "test_case.snap")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestMatcher_SnapshotPath_Subdirectory(t *testing.T) {
	m := New()
	path, err := m.snapshotPath("group/sub_test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(defaultSnapshotDir, "group", "sub_test.snap")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestMatcher_SnapshotPath_Empty(t *testing.T) {
	m := New()
	_, err := m.snapshotPath("")
	if !errors.Is(err, ErrInvalidName) {
		t.Errorf("expected ErrInvalidName, got %v", err)
	}
}

func TestMatcher_SnapshotPath_PathTraversal(t *testing.T) {
	m := New()
	tests := []string{
		"../etc/passwd",
		"foo/../../bar",
		"..",
		".",
	}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := m.snapshotPath(name)
			if !errors.Is(err, ErrInvalidName) {
				t.Errorf("name %q: expected ErrInvalidName, got %v", name, err)
			}
		})
	}
}

func createTempSnapshotDir(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "snaptest-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	cleanup := func() {
		os.RemoveAll(dir)
	}
	return dir, cleanup
}

func TestMatcher_WriteAndReadSnapshot(t *testing.T) {
	dir, cleanup := createTempSnapshotDir(t)
	defer cleanup()

	m := NewWithConfig(Config{SnapshotDir: dir})

	err := m.writeSnapshot("test1", "hello\nworld")
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	content, err := m.readSnapshot("test1")
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if content != "hello\nworld" {
		t.Errorf("expected %q, got %q", "hello\nworld", content)
	}
}

func TestMatcher_WriteSnapshot_Subdirectory(t *testing.T) {
	dir, cleanup := createTempSnapshotDir(t)
	defer cleanup()

	m := NewWithConfig(Config{SnapshotDir: dir})

	err := m.writeSnapshot("nested/test", "content")
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	snapPath, _ := m.snapshotPath("nested/test")
	if _, err := os.Stat(snapPath); os.IsNotExist(err) {
		t.Error("snapshot file in subdirectory should exist")
	}
}

func TestMatcher_ReadSnapshot_NotFound(t *testing.T) {
	dir, cleanup := createTempSnapshotDir(t)
	defer cleanup()

	m := NewWithConfig(Config{SnapshotDir: dir})
	_, err := m.readSnapshot("nonexistent")
	if !errors.Is(err, ErrSnapshotNotFound) {
		t.Errorf("expected ErrSnapshotNotFound, got %v", err)
	}
}

func TestMatcher_Match_NewSnapshot(t *testing.T) {
	dir, cleanup := createTempSnapshotDir(t)
	defer cleanup()

	m := NewWithConfig(Config{SnapshotDir: dir})

	data := map[string]string{"key": "value"}
	ok, info, err := m.Match("new_snap", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match to succeed for new snapshot")
	}
	if info == "" {
		t.Error("expected info about new snapshot creation")
	}

	snapPath, _ := m.snapshotPath("new_snap")
	if _, err := os.Stat(snapPath); os.IsNotExist(err) {
		t.Error("snapshot file should have been created")
	}
}

func TestMatcher_Match_ExistingMatch(t *testing.T) {
	dir, cleanup := createTempSnapshotDir(t)
	defer cleanup()

	m := NewWithConfig(Config{SnapshotDir: dir})

	data := map[string]int{"a": 1, "b": 2}
	_, _, err := m.Match("existing", data)
	if err != nil {
		t.Fatalf("unexpected error on first match: %v", err)
	}

	ok, report, err := m.Match("existing", data)
	if err != nil {
		t.Fatalf("unexpected error on second match: %v", err)
	}
	if !ok {
		t.Error("expected match to succeed for identical data")
	}
	if report != "" {
		t.Errorf("expected empty report, got:\n%s", report)
	}
}

func TestMatcher_Match_ExistingMismatch(t *testing.T) {
	dir, cleanup := createTempSnapshotDir(t)
	defer cleanup()

	m := NewWithConfig(Config{SnapshotDir: dir, ContextLines: 3})

	data1 := map[string]string{"status": "ok", "count": "1"}
	data2 := map[string]string{"status": "error", "count": "2"}

	_, _, err := m.Match("mismatch_test", data1)
	if err != nil {
		t.Fatalf("unexpected error creating snapshot: %v", err)
	}

	ok, report, err := m.Match("mismatch_test", data2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected match to fail for different data")
	}
	if report == "" {
		t.Error("expected non-empty diff report")
	}
	if !strings.Contains(report, "Summary:") {
		t.Error("report should contain summary")
	}
}

func TestMatcher_Match_UpdateMode(t *testing.T) {
	dir, cleanup := createTempSnapshotDir(t)
	defer cleanup()

	data1 := "original"
	data2 := "updated"

	m1 := NewWithConfig(Config{SnapshotDir: dir})
	_, _, err := m1.Match("update_test", data1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m2 := NewWithConfig(Config{SnapshotDir: dir, UpdateMode: true})
	ok, report, err := m2.Match("update_test", data2)
	if err != nil {
		t.Fatalf("unexpected error in update mode: %v", err)
	}
	if !ok {
		t.Error("update mode should always succeed")
	}
	if report != "" {
		t.Error("update mode should not produce diff report")
	}

	content, err := m2.readSnapshot("update_test")
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if !strings.Contains(content, "updated") {
		t.Errorf("snapshot should contain updated content, got: %s", content)
	}
}

func TestMatcher_Match_InvalidName(t *testing.T) {
	dir, cleanup := createTempSnapshotDir(t)
	defer cleanup()

	m := NewWithConfig(Config{SnapshotDir: dir})
	ok, _, err := m.Match("../bad", "data")
	if ok {
		t.Error("expected failure for invalid name")
	}
	if err == nil {
		t.Error("expected error for invalid name")
	}
	if !errors.Is(err, ErrInvalidName) {
		t.Errorf("expected ErrInvalidName, got %v", err)
	}
}

func TestMatcher_Update(t *testing.T) {
	dir, cleanup := createTempSnapshotDir(t)
	defer cleanup()

	m := NewWithConfig(Config{SnapshotDir: dir})

	err := m.Update("direct_update", "initial")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := m.readSnapshot("direct_update")
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if content != `"initial"` {
		t.Errorf("expected %q, got %q", `"initial"`, content)
	}

	err = m.Update("direct_update", "modified")
	if err != nil {
		t.Fatalf("unexpected error on second update: %v", err)
	}

	content, err = m.readSnapshot("direct_update")
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if content != `"modified"` {
		t.Errorf("expected %q, got %q", `"modified"`, content)
	}
}

func TestMatcher_Update_InvalidName(t *testing.T) {
	m := New()
	err := m.Update("", "data")
	if err == nil {
		t.Error("expected error for empty name")
	}
	if !errors.Is(err, ErrInvalidName) {
		t.Errorf("expected ErrInvalidName, got %v", err)
	}
}

func TestMatcher_Assert_Match(t *testing.T) {
	dir, cleanup := createTempSnapshotDir(t)
	defer cleanup()

	m := NewWithConfig(Config{SnapshotDir: dir})

	fakeT := &testing.T{}
	m.Assert(fakeT, "assert_match", map[string]int{"x": 1})

	fakeT2 := &testing.T{}
	m.Assert(fakeT2, "assert_match", map[string]int{"x": 1})

	if fakeT2.Failed() {
		t.Error("Assert should not fail for matching snapshot")
	}
}

func TestConvenienceFunctions_Match(t *testing.T) {
	dir, cleanup := createTempSnapshotDir(t)
	defer cleanup()

	m := NewWithConfig(Config{SnapshotDir: dir})
	ok, _, err := m.Match("convenience", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected match success")
	}
}

func TestConvenienceFunctions_Update(t *testing.T) {
	dir, cleanup := createTempSnapshotDir(t)
	defer cleanup()

	m := NewWithConfig(Config{SnapshotDir: dir})
	err := m.Update("conv_update", 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := m.readSnapshot("conv_update")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "123" {
		t.Errorf("expected '123', got %q", content)
	}
}

func TestSerialize_NoHTMLEscaping(t *testing.T) {
	input := map[string]string{"url": "https://example.com?a=1&b=2"}
	result, err := Serialize(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "\\u0026") {
		t.Errorf("HTML escaping should be disabled, got: %s", result)
	}
	if !strings.Contains(result, "&") {
		t.Errorf("expected raw & character, got: %s", result)
	}
}

func TestNormalizeSnapshotContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"windows newlines", "a\r\nb\r\nc", "a\nb\nc"},
		{"trailing newlines", "a\nb\n\n", "a\nb"},
		{"mixed", "a\r\nb\n\n", "a\nb"},
		{"no trailing", "a\nb", "a\nb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeSnapshotContent(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestDiff_LineNumbers(t *testing.T) {
	expected := "a\nb\nc"
	actual := "a\nX\nc"
	result := Diff(expected, actual)

	foundRemoved := false
	foundAdded := false
	for _, l := range result.Lines {
		if l.Type == DiffRemoved {
			if l.LeftNum != 2 {
				t.Errorf("expected removed line LeftNum=2, got %d", l.LeftNum)
			}
			foundRemoved = true
		}
		if l.Type == DiffAdded {
			if l.RightNum != 2 {
				t.Errorf("expected added line RightNum=2, got %d", l.RightNum)
			}
			foundAdded = true
		}
		if l.Type == DiffSame {
			if l.LeftNum == 0 || l.RightNum == 0 {
				t.Error("same lines should have both line numbers > 0")
			}
		}
	}
	if !foundRemoved {
		t.Error("should have found a removed line")
	}
	if !foundAdded {
		t.Error("should have found an added line")
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{}},
		{"a", []string{"a"}},
		{"a\nb", []string{"a", "b"}},
		{"a\nb\nc", []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		result := splitLines(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("input %q: expected %d lines, got %d", tt.input, len(tt.expected), len(result))
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("input %q line %d: expected %q, got %q", tt.input, i, tt.expected[i], result[i])
			}
		}
	}
}

func TestApplyContext_AllSame(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffSame, Content: "a", LeftNum: 1, RightNum: 1},
		{Type: DiffSame, Content: "b", LeftNum: 2, RightNum: 2},
	}
	result := applyContext(lines, 0)
	if len(result) != len(lines) {
		t.Errorf("expected all same lines to be preserved, got %d vs %d", len(result), len(lines))
	}
}

func TestApplyContext_Empty(t *testing.T) {
	result := applyContext(nil, 3)
	if len(result) != 0 {
		t.Errorf("expected empty result for empty input, got %d", len(result))
	}
}

func TestApplyContext_SingleChangeWithContext(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffSame, Content: "1", LeftNum: 1, RightNum: 1},
		{Type: DiffSame, Content: "2", LeftNum: 2, RightNum: 2},
		{Type: DiffSame, Content: "3", LeftNum: 3, RightNum: 3},
		{Type: DiffRemoved, Content: "old", LeftNum: 4, RightNum: 0},
		{Type: DiffAdded, Content: "new", LeftNum: 0, RightNum: 4},
		{Type: DiffSame, Content: "5", LeftNum: 5, RightNum: 5},
		{Type: DiffSame, Content: "6", LeftNum: 6, RightNum: 6},
		{Type: DiffSame, Content: "7", LeftNum: 7, RightNum: 7},
	}

	result := applyContext(lines, 1)

	keptChanges := 0
	for _, l := range result {
		if l.Type == DiffRemoved || l.Type == DiffAdded {
			keptChanges++
		}
	}
	if keptChanges != 2 {
		t.Errorf("expected 2 change lines kept, got %d", keptChanges)
	}
}

func TestMatcher_Config_ReturnsCopy(t *testing.T) {
	m := NewWithConfig(Config{SnapshotDir: "custom", ContextLines: 5})
	cfg := m.Config()
	cfg.SnapshotDir = "modified"
	cfg.ContextLines = 999

	got := m.Config()
	if got.SnapshotDir == "modified" {
		t.Error("Config() should return a copy, not reference")
	}
	if got.ContextLines == 999 {
		t.Error("Config() should return a copy, not reference")
	}
}
