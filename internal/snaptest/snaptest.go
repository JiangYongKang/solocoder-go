package snaptest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	ErrSnapshotNotFound = errors.New("snaptest: snapshot not found")
	ErrSnapshotMismatch = errors.New("snaptest: snapshot mismatch")
	ErrInvalidName      = errors.New("snaptest: invalid snapshot name")
	ErrSerialization    = errors.New("snaptest: serialization error")
	ErrWriteSnapshot    = errors.New("snaptest: cannot write snapshot file")
	ErrReadSnapshot     = errors.New("snaptest: cannot read snapshot file")
)

const (
	defaultSnapshotDir = "__snapshots__"
	defaultContextLines = 3
	updateEnvVar       = "SNAPTEST_UPDATE"
)

type DiffType int

const (
	DiffSame DiffType = iota
	DiffRemoved
	DiffAdded
)

type DiffLine struct {
	Type     DiffType
	LeftNum  int
	RightNum int
	Content  string
}

type DiffResult struct {
	Lines      []DiffLine
	TotalSame  int
	TotalAdded int
	TotalRemoved int
}

type Config struct {
	SnapshotDir  string
	UpdateMode   bool
	ContextLines int
}

type Matcher struct {
	cfg Config
}

func DefaultConfig() Config {
	return Config{
		SnapshotDir:  defaultSnapshotDir,
		UpdateMode:   false,
		ContextLines: defaultContextLines,
	}
}

func New() *Matcher {
	cfg := DefaultConfig()
	if v, ok := os.LookupEnv(updateEnvVar); ok {
		cfg.UpdateMode = v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	}
	return NewWithConfig(cfg)
}

func NewWithConfig(cfg Config) *Matcher {
	if cfg.SnapshotDir == "" {
		cfg.SnapshotDir = defaultSnapshotDir
	}
	if cfg.ContextLines <= 0 {
		cfg.ContextLines = defaultContextLines
	}
	return &Matcher{cfg: cfg}
}

func (m *Matcher) Config() Config {
	return m.cfg
}

func Serialize(v interface{}) (string, error) {
	if v == nil {
		return "null", nil
	}

	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return "", fmt.Errorf("%w: %v", ErrSerialization, err)
	}

	result := buf.String()
	result = strings.TrimSuffix(result, "\n")
	return result, nil
}

func normalizeSnapshotContent(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	return s
}

func (m *Matcher) snapshotPath(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("%w: empty name", ErrInvalidName)
	}
	cleaned := filepath.Clean(name)
	if cleaned == "." || cleaned == ".." || strings.Contains(cleaned, "..") {
		return "", fmt.Errorf("%w: invalid path traversal in name %q", ErrInvalidName, name)
	}
	return filepath.Join(m.cfg.SnapshotDir, cleaned+".snap"), nil
}

func (m *Matcher) readSnapshot(name string) (string, error) {
	path, err := m.snapshotPath(name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %q", ErrSnapshotNotFound, name)
		}
		return "", fmt.Errorf("%w: %v", ErrReadSnapshot, err)
	}
	return normalizeSnapshotContent(string(data)), nil
}

func (m *Matcher) writeSnapshot(name string, content string) error {
	path, err := m.snapshotPath(name)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%w: cannot create dir %q: %v", ErrWriteSnapshot, dir, err)
	}
	normalized := normalizeSnapshotContent(content) + "\n"
	if err := os.WriteFile(path, []byte(normalized), 0o644); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteSnapshot, err)
	}
	return nil
}

func Diff(expected, actual string) DiffResult {
	expLines := splitLines(normalizeSnapshotContent(expected))
	actLines := splitLines(normalizeSnapshotContent(actual))

	lcs := computeLCS(expLines, actLines)

	var result DiffResult
	i, j := 0, 0

	for _, pair := range lcs {
		for i < pair.Left {
			result.Lines = append(result.Lines, DiffLine{
				Type:     DiffRemoved,
				LeftNum:  i + 1,
				RightNum: 0,
				Content:  expLines[i],
			})
			result.TotalRemoved++
			i++
		}
		for j < pair.Right {
			result.Lines = append(result.Lines, DiffLine{
				Type:     DiffAdded,
				LeftNum:  0,
				RightNum: j + 1,
				Content:  actLines[j],
			})
			result.TotalAdded++
			j++
		}
		result.Lines = append(result.Lines, DiffLine{
			Type:     DiffSame,
			LeftNum:  i + 1,
			RightNum: j + 1,
			Content:  expLines[i],
		})
		result.TotalSame++
		i++
		j++
	}

	for i < len(expLines) {
		result.Lines = append(result.Lines, DiffLine{
			Type:     DiffRemoved,
			LeftNum:  i + 1,
			RightNum: 0,
			Content:  expLines[i],
		})
		result.TotalRemoved++
		i++
	}
	for j < len(actLines) {
		result.Lines = append(result.Lines, DiffLine{
			Type:     DiffAdded,
			LeftNum:  0,
			RightNum: j + 1,
			Content:  actLines[j],
		})
		result.TotalAdded++
		j++
	}

	return result
}

type lcsPair struct {
	Left  int
	Right int
}

func computeLCS(a, b []string) []lcsPair {
	la, lb := len(a), len(b)
	if la == 0 || lb == 0 {
		return nil
	}

	dp := make([][]int, la+1)
	for i := range dp {
		dp[i] = make([]int, lb+1)
	}

	for i := la - 1; i >= 0; i-- {
		for j := lb - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else {
				if dp[i+1][j] > dp[i][j+1] {
					dp[i][j] = dp[i+1][j]
				} else {
					dp[i][j] = dp[i][j+1]
				}
			}
		}
	}

	result := make([]lcsPair, 0, dp[0][0])
	i, j := 0, 0
	for i < la && j < lb {
		if a[i] == b[j] {
			result = append(result, lcsPair{Left: i, Right: j})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			i++
		} else {
			j++
		}
	}

	return result
}

func splitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, "\n")
}

func (d DiffResult) Matches() bool {
	return d.TotalAdded == 0 && d.TotalRemoved == 0
}

func (d DiffResult) Format(contextLines int) string {
	if d.Matches() {
		return ""
	}
	if contextLines < 0 {
		contextLines = 0
	}

	filtered := applyContext(d.Lines, contextLines)

	var buf bytes.Buffer
	buf.WriteString("--- Expected (snapshot)\n")
	buf.WriteString("+++ Actual (current output)\n")
	buf.WriteString(fmt.Sprintf("@@ -%d +%d @@\n", firstLeftNum(filtered), firstRightNum(filtered)))

	for _, line := range filtered {
		switch line.Type {
		case DiffSame:
			fmt.Fprintf(&buf, "  %4d %4d | %s\n", line.LeftNum, line.RightNum, line.Content)
		case DiffRemoved:
			fmt.Fprintf(&buf, "- %4d      | %s\n", line.LeftNum, line.Content)
		case DiffAdded:
			fmt.Fprintf(&buf, "+      %4d | %s\n", line.RightNum, line.Content)
		}
	}

	fmt.Fprintf(&buf, "\nSummary: %d same, %d removed, %d added\n",
		d.TotalSame, d.TotalRemoved, d.TotalAdded)

	return buf.String()
}

func firstLeftNum(lines []DiffLine) int {
	for _, l := range lines {
		if l.LeftNum > 0 {
			return l.LeftNum
		}
	}
	return 1
}

func firstRightNum(lines []DiffLine) int {
	for _, l := range lines {
		if l.RightNum > 0 {
			return l.RightNum
		}
	}
	return 1
}

func applyContext(lines []DiffLine, contextLines int) []DiffLine {
	if len(lines) == 0 {
		return lines
	}

	changed := make([]bool, len(lines))
	hasChange := false
	for i, l := range lines {
		if l.Type != DiffSame {
			changed[i] = true
			hasChange = true
		}
	}

	if !hasChange {
		return lines
	}

	kept := make([]bool, len(lines))
	for i := range lines {
		if changed[i] {
			start := i - contextLines
			if start < 0 {
				start = 0
			}
			end := i + contextLines
			if end >= len(lines) {
				end = len(lines) - 1
			}
			for j := start; j <= end; j++ {
				kept[j] = true
			}
		}
	}

	var result []DiffLine
	prevKept := false
	for i, l := range lines {
		if kept[i] {
			if i > 0 && !prevKept {
				result = append(result, DiffLine{
					Type:    DiffSame,
					Content: "...",
				})
			}
			result = append(result, l)
			prevKept = true
		} else {
			prevKept = false
		}
	}

	if len(lines) > 0 && !kept[len(lines)-1] {
		result = append(result, DiffLine{
			Type:    DiffSame,
			Content: "...",
		})
	}

	return result
}

func (m *Matcher) Match(name string, v interface{}) (bool, string, error) {
	actual, err := Serialize(v)
	if err != nil {
		return false, "", err
	}

	if m.cfg.UpdateMode {
		if err := m.writeSnapshot(name, actual); err != nil {
			return false, "", err
		}
		return true, "", nil
	}

	expected, err := m.readSnapshot(name)
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			if err := m.writeSnapshot(name, actual); err != nil {
				return false, "", err
			}
			return true, "created new snapshot", nil
		}
		return false, "", err
	}

	diff := Diff(expected, actual)
	if diff.Matches() {
		return true, "", nil
	}

	report := diff.Format(m.cfg.ContextLines)
	return false, report, nil
}

func (m *Matcher) Update(name string, v interface{}) error {
	actual, err := Serialize(v)
	if err != nil {
		return err
	}
	return m.writeSnapshot(name, actual)
}

func (m *Matcher) Assert(t *testing.T, name string, v interface{}) {
	t.Helper()
	ok, report, err := m.Match(name, v)
	if err != nil {
		t.Fatalf("snaptest: error matching snapshot %q: %v", name, err)
	}
	if !ok {
		if report != "" {
			t.Errorf("snaptest: snapshot %q does not match:\n%s", name, report)
		} else {
			t.Errorf("snaptest: snapshot %q does not match", name)
		}
	}
}

func Assert(t *testing.T, name string, v interface{}) {
	t.Helper()
	New().Assert(t, name, v)
}

func Update(name string, v interface{}) error {
	return New().Update(name, v)
}

func Match(name string, v interface{}) (bool, string, error) {
	return New().Match(name, v)
}
