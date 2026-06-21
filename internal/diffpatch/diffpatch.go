package diffpatch

import "errors"

var (
	ErrEmptyInput      = errors.New("diffpatch: empty input text")
	ErrInvalidPatch    = errors.New("diffpatch: invalid patch format")
	ErrPatchConflict   = errors.New("diffpatch: patch context does not match original text")
	ErrMergeConflict   = errors.New("diffpatch: merge conflict detected")
)

type LineType int

const (
	LineEqual  LineType = iota
	LineDelete
	LineInsert
)

type Line struct {
	Content   string
	Type      LineType
	OldLineNo int
	NewLineNo int
}

type Hunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []Line
}

type DiffResult struct {
	Hunks []Hunk
}

type PatchHeader struct {
	OldFile string
	NewFile string
}

type Patch struct {
	Header PatchHeader
	Hunks  []Hunk
}

type ConflictRange struct {
	StartLine int
	EndLine   int
	Ours      []string
	Theirs    []string
	Base      []string
}

type ApplyResult struct {
	Text      string
	Rejected  bool
	Conflicts []ConflictRange
}

type MergeResult struct {
	Text        string
	HasConflicts bool
	Conflicts   []ConflictRange
}

func Diff(oldText, newText string) (*DiffResult, error) {
	if oldText == "" && newText == "" {
		return nil, ErrEmptyInput
	}

	oldLines := splitLines(oldText)
	newLines := splitLines(newText)

	ses := myersDiff(oldLines, newLines)
	hunks := buildHunks(ses, oldLines, newLines)

	return &DiffResult{Hunks: hunks}, nil
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return splitText(text)
}

func splitText(text string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			lines = append(lines, text[start:i])
			start = i + 1
		}
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}
