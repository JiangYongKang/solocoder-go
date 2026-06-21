package diffpatch

import (
	"errors"
	"strings"
	"testing"
)

func TestDiff_BothEmpty(t *testing.T) {
	_, err := Diff("", "")
	if !errors.Is(err, ErrEmptyInput) {
		t.Errorf("expected ErrEmptyInput for both empty, got %v", err)
	}
}

func TestDiff_OldEmpty(t *testing.T) {
	result, err := Diff("", "a\nb\nc\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hunks) == 0 {
		t.Fatal("expected hunks for old empty text")
	}
	for _, hunk := range result.Hunks {
		if hunk.OldCount != 0 {
			t.Errorf("expected OldCount=0 for old empty, got %d", hunk.OldCount)
		}
		for _, line := range hunk.Lines {
			if line.Type != LineInsert {
				t.Errorf("expected all lines to be insert, got %v", line.Type)
			}
		}
	}
}

func TestDiff_NewEmpty(t *testing.T) {
	result, err := Diff("a\nb\nc\n", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hunks) == 0 {
		t.Fatal("expected hunks for new empty text")
	}
	for _, hunk := range result.Hunks {
		if hunk.NewCount != 0 {
			t.Errorf("expected NewCount=0 for new empty, got %d", hunk.NewCount)
		}
		for _, line := range hunk.Lines {
			if line.Type != LineDelete {
				t.Errorf("expected all lines to be delete, got %v", line.Type)
			}
		}
	}
}

func TestDiff_IdenticalTexts(t *testing.T) {
	result, err := Diff("a\nb\nc\n", "a\nb\nc\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hunks) != 0 {
		t.Errorf("expected no hunks for identical texts, got %d", len(result.Hunks))
	}
}

func TestDiff_SimpleInsertion(t *testing.T) {
	old := "a\nb\nc\n"
	new_ := "a\nb\nx\nc\n"
	result, err := Diff(old, new_)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hunks) == 0 {
		t.Fatal("expected hunks for insertion")
	}

	found := false
	for _, hunk := range result.Hunks {
		for _, line := range hunk.Lines {
			if line.Type == LineInsert && line.Content == "x" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected insert line 'x' in diff result")
	}
}

func TestDiff_SimpleDeletion(t *testing.T) {
	old := "a\nb\nc\n"
	new_ := "a\nc\n"
	result, err := Diff(old, new_)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hunks) == 0 {
		t.Fatal("expected hunks for deletion")
	}

	found := false
	for _, hunk := range result.Hunks {
		for _, line := range hunk.Lines {
			if line.Type == LineDelete && line.Content == "b" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected delete line 'b' in diff result")
	}
}

func TestDiff_SimpleModification(t *testing.T) {
	old := "a\nb\nc\n"
	new_ := "a\nB\nc\n"
	result, err := Diff(old, new_)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hunks) == 0 {
		t.Fatal("expected hunks for modification")
	}

	hasDelete := false
	hasInsert := false
	for _, hunk := range result.Hunks {
		for _, line := range hunk.Lines {
			if line.Type == LineDelete && line.Content == "b" {
				hasDelete = true
			}
			if line.Type == LineInsert && line.Content == "B" {
				hasInsert = true
			}
		}
	}
	if !hasDelete || !hasInsert {
		t.Errorf("expected delete 'b' and insert 'B', got delete=%v insert=%v", hasDelete, hasInsert)
	}
}

func TestDiff_HunkLineNumbers(t *testing.T) {
	old := "line1\nline2\nline3\nline4\nline5\n"
	new_ := "line1\nline2\nmodified3\nline4\nline5\n"
	result, err := Diff(old, new_)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hunks) == 0 {
		t.Fatal("expected at least one hunk")
	}

	hunk := result.Hunks[0]
	if hunk.OldStart < 1 {
		t.Errorf("OldStart should be >= 1, got %d", hunk.OldStart)
	}
	if hunk.NewStart < 1 {
		t.Errorf("NewStart should be >= 1, got %d", hunk.NewStart)
	}
	if hunk.OldCount <= 0 {
		t.Errorf("OldCount should be > 0, got %d", hunk.OldCount)
	}
	if hunk.NewCount <= 0 {
		t.Errorf("NewCount should be > 0, got %d", hunk.NewCount)
	}
}

func TestDiff_MultipleChanges(t *testing.T) {
	old := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"
	new_ := "a\nB\nc\nd\ne\nF\ng\nh\ni\nJ\n"
	result, err := Diff(old, new_)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hunks) == 0 {
		t.Fatal("expected hunks for multiple changes")
	}

	insertCount := 0
	deleteCount := 0
	for _, hunk := range result.Hunks {
		for _, line := range hunk.Lines {
			if line.Type == LineInsert {
				insertCount++
			}
			if line.Type == LineDelete {
				deleteCount++
			}
		}
	}
	if insertCount != 3 {
		t.Errorf("expected 3 insert lines, got %d", insertCount)
	}
	if deleteCount != 3 {
		t.Errorf("expected 3 delete lines, got %d", deleteCount)
	}
}

func TestDiff_SingleLine(t *testing.T) {
	result, err := Diff("hello\n", "world\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hunks) == 0 {
		t.Fatal("expected hunks for single line diff")
	}

	hunk := result.Hunks[0]
	foundDelete := false
	foundInsert := false
	for _, line := range hunk.Lines {
		if line.Type == LineDelete && line.Content == "hello" {
			foundDelete = true
		}
		if line.Type == LineInsert && line.Content == "world" {
			foundInsert = true
		}
	}
	if !foundDelete || !foundInsert {
		t.Error("expected delete 'hello' and insert 'world'")
	}
}

func TestGeneratePatch_Basic(t *testing.T) {
	old := "a\nb\nc\n"
	new_ := "a\nB\nc\n"
	patch, err := GeneratePatch("old.txt", "new.txt", old, new_)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(patch, "--- old.txt") {
		t.Error("patch should contain --- old.txt")
	}
	if !strings.Contains(patch, "+++ new.txt") {
		t.Error("patch should contain +++ new.txt")
	}
	if !strings.Contains(patch, "@@") {
		t.Error("patch should contain hunk header @@")
	}
	if !strings.Contains(patch, "-b") {
		t.Error("patch should contain -b")
	}
	if !strings.Contains(patch, "+B") {
		t.Error("patch should contain +B")
	}
}

func TestGeneratePatch_IdenticalFiles(t *testing.T) {
	patch, err := GeneratePatch("a.txt", "a.txt", "same\n", "same\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := splitText(patch)
	hunkLines := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			hunkLines++
		}
	}
	if hunkLines > 0 {
		t.Error("expected no hunks for identical files")
	}
}

func TestParsePatch_Valid(t *testing.T) {
	patchText := "--- a.txt\n+++ b.txt\n@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n"
	patch, err := ParsePatch(patchText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if patch.Header.OldFile != "a.txt" {
		t.Errorf("expected OldFile=a.txt, got %s", patch.Header.OldFile)
	}
	if patch.Header.NewFile != "b.txt" {
		t.Errorf("expected NewFile=b.txt, got %s", patch.Header.NewFile)
	}
	if len(patch.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(patch.Hunks))
	}
	hunk := patch.Hunks[0]
	if hunk.OldStart != 1 {
		t.Errorf("expected OldStart=1, got %d", hunk.OldStart)
	}
	if hunk.OldCount != 3 {
		t.Errorf("expected OldCount=3, got %d", hunk.OldCount)
	}
	if hunk.NewStart != 1 {
		t.Errorf("expected NewStart=1, got %d", hunk.NewStart)
	}
	if hunk.NewCount != 3 {
		t.Errorf("expected NewCount=3, got %d", hunk.NewCount)
	}
}

func TestParsePatch_Invalid(t *testing.T) {
	_, err := ParsePatch("")
	if !errors.Is(err, ErrInvalidPatch) {
		t.Errorf("expected ErrInvalidPatch, got %v", err)
	}
}

func TestParsePatch_InvalidHunkHeader(t *testing.T) {
	patchText := "--- a.txt\n+++ b.txt\n@@ invalid @@\n"
	_, err := ParsePatch(patchText)
	if !errors.Is(err, ErrInvalidPatch) {
		t.Errorf("expected ErrInvalidPatch, got %v", err)
	}
}

func TestPatchToUnified_RoundTrip(t *testing.T) {
	old := "line1\nline2\nline3\nline4\nline5\n"
	new_ := "line1\nmodified2\nline3\nline4\nadded5\n"
	patch, err := GeneratePatch("a.txt", "b.txt", old, new_)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parsed, err := ParsePatch(patch)
	if err != nil {
		t.Fatalf("unexpected error parsing patch: %v", err)
	}

	if parsed.Header.OldFile != "a.txt" {
		t.Errorf("expected OldFile=a.txt, got %s", parsed.Header.OldFile)
	}
	if parsed.Header.NewFile != "b.txt" {
		t.Errorf("expected NewFile=b.txt, got %s", parsed.Header.NewFile)
	}
	if len(parsed.Hunks) == 0 {
		t.Fatal("expected at least one hunk in roundtrip")
	}
}

func TestApplyPatch_Basic(t *testing.T) {
	old := "a\nb\nc\n"
	new_ := "a\nB\nc\n"
	patch, err := GeneratePatch("a.txt", "b.txt", old, new_)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := ApplyPatch(old, patch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rejected {
		t.Errorf("patch should not be rejected")
	}
	if result.Text != new_ {
		t.Errorf("expected result=%q, got %q", new_, result.Text)
	}
}

func TestApplyPatch_Insertion(t *testing.T) {
	old := "a\nb\nc\n"
	new_ := "a\nb\nx\nc\n"
	patch, err := GeneratePatch("a.txt", "b.txt", old, new_)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := ApplyPatch(old, patch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rejected {
		t.Errorf("patch should not be rejected")
	}
	if result.Text != new_ {
		t.Errorf("expected result=%q, got %q", new_, result.Text)
	}
}

func TestApplyPatch_Deletion(t *testing.T) {
	old := "a\nb\nc\n"
	new_ := "a\nc\n"
	patch, err := GeneratePatch("a.txt", "b.txt", old, new_)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := ApplyPatch(old, patch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rejected {
		t.Errorf("patch should not be rejected")
	}
	if result.Text != new_ {
		t.Errorf("expected result=%q, got %q", new_, result.Text)
	}
}

func TestApplyPatch_NoChanges(t *testing.T) {
	old := "a\nb\nc\n"
	new_ := "a\nb\nc\n"
	patch, err := GeneratePatch("a.txt", "b.txt", old, new_)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := ApplyPatch(old, patch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rejected {
		t.Errorf("patch should not be rejected")
	}
	if result.Text != old {
		t.Errorf("expected result=%q, got %q", old, result.Text)
	}
}

func TestApplyPatch_InvalidPatch(t *testing.T) {
	_, err := ApplyPatch("a\n", "invalid patch text")
	if !errors.Is(err, ErrInvalidPatch) {
		t.Errorf("expected ErrInvalidPatch, got %v", err)
	}
}

func TestApplyPatch_ConflictingContext(t *testing.T) {
	old := "a\nb\nc\n"
	patchText := "--- a.txt\n+++ b.txt\n@@ -1,3 +1,3 @@\n a\n-X\n+Y\n c\n"

	result, err := ApplyPatch(old, patchText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Rejected {
		t.Error("expected patch to be rejected due to context mismatch")
	}
	if len(result.Conflicts) == 0 {
		t.Error("expected conflict ranges")
	}
}

func TestApplyPatch_DirectValid(t *testing.T) {
	old := "line1\nline2\nline3\n"
	patch := &Patch{
		Header: PatchHeader{OldFile: "a", NewFile: "b"},
		Hunks: []Hunk{
			{
				OldStart: 1, OldCount: 3,
				NewStart: 1, NewCount: 3,
				Lines: []Line{
					{Content: "line1", Type: LineEqual},
					{Content: "line2", Type: LineDelete},
					{Content: "LINE2", Type: LineInsert},
					{Content: "line3", Type: LineEqual},
				},
			},
		},
	}

	result, err := ApplyPatchDirect(old, patch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rejected {
		t.Errorf("patch should not be rejected")
	}
	expected := "line1\nLINE2\nline3\n"
	if result.Text != expected {
		t.Errorf("expected %q, got %q", expected, result.Text)
	}
}

func TestThreeWayMerge_OursNoChange(t *testing.T) {
	base := "a\nb\nc\n"
	ours := "a\nb\nc\n"
	theirs := "a\nB\nc\n"

	result, err := ThreeWayMerge(base, ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasConflicts {
		t.Error("should not have conflicts when ours has no changes")
	}
	if result.Text != theirs {
		t.Errorf("expected %q, got %q", theirs, result.Text)
	}
}

func TestThreeWayMerge_TheirsNoChange(t *testing.T) {
	base := "a\nb\nc\n"
	ours := "a\nB\nc\n"
	theirs := "a\nb\nc\n"

	result, err := ThreeWayMerge(base, ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasConflicts {
		t.Error("should not have conflicts when theirs has no changes")
	}
	if result.Text != ours {
		t.Errorf("expected %q, got %q", ours, result.Text)
	}
}

func TestThreeWayMerge_BothSameChange(t *testing.T) {
	base := "a\nb\nc\n"
	ours := "a\nB\nc\n"
	theirs := "a\nB\nc\n"

	result, err := ThreeWayMerge(base, ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasConflicts {
		t.Error("should not have conflicts when both make same change")
	}
	if result.Text != ours {
		t.Errorf("expected %q, got %q", ours, result.Text)
	}
}

func TestThreeWayMerge_NonOverlappingChanges(t *testing.T) {
	base := "a\nb\nc\nd\ne\n"
	ours := "A\nb\nc\nd\ne\n"
	theirs := "a\nb\nc\nd\nE\n"

	result, err := ThreeWayMerge(base, ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasConflicts {
		t.Error("should not have conflicts for non-overlapping changes")
	}

	if !strings.Contains(result.Text, "A") {
		t.Error("merged text should contain our change 'A'")
	}
	if !strings.Contains(result.Text, "E") {
		t.Error("merged text should contain their change 'E'")
	}
}

func TestThreeWayMerge_ConflictingChanges(t *testing.T) {
	base := "a\nb\nc\n"
	ours := "a\nOURS\nc\n"
	theirs := "a\nTHEIRS\nc\n"

	result, err := ThreeWayMerge(base, ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasConflicts {
		t.Error("expected conflicts when both sides modify same region differently")
	}
	if len(result.Conflicts) == 0 {
		t.Error("expected conflict ranges")
	}
	if !strings.Contains(result.Text, "<<<<<<< ours") {
		t.Error("merged text should contain conflict marker")
	}
	if !strings.Contains(result.Text, "=======") {
		t.Error("merged text should contain separator marker")
	}
	if !strings.Contains(result.Text, ">>>>>>> theirs") {
		t.Error("merged text should contain end marker")
	}
	if !strings.Contains(result.Text, "OURS") {
		t.Error("merged text should contain ours content")
	}
	if !strings.Contains(result.Text, "THEIRS") {
		t.Error("merged text should contain theirs content")
	}
}

func TestThreeWayMerge_AllIdentical(t *testing.T) {
	text := "a\nb\nc\n"
	result, err := ThreeWayMerge(text, text, text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasConflicts {
		t.Error("should not have conflicts when all texts are identical")
	}
	if result.Text != text {
		t.Errorf("expected %q, got %q", text, result.Text)
	}
}

func TestFormatConflict(t *testing.T) {
	conflict := ConflictRange{
		StartLine: 2,
		EndLine:   2,
		Ours:      []string{"our_line"},
		Theirs:    []string{"their_line"},
	}
	formatted := FormatConflict(conflict)
	if !strings.Contains(formatted, "<<<<<<< ours") {
		t.Error("formatted conflict should contain ours marker")
	}
	if !strings.Contains(formatted, "our_line") {
		t.Error("formatted conflict should contain ours content")
	}
	if !strings.Contains(formatted, "=======") {
		t.Error("formatted conflict should contain separator")
	}
	if !strings.Contains(formatted, "their_line") {
		t.Error("formatted conflict should contain theirs content")
	}
	if !strings.Contains(formatted, ">>>>>>> theirs") {
		t.Error("formatted conflict should contain theirs marker")
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty", "", nil},
		{"single line", "hello\n", []string{"hello"}},
		{"multi line", "a\nb\nc\n", []string{"a", "b", "c"}},
		{"no trailing newline", "a\nb", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("expected %d lines, got %d", len(tt.want), len(got))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d: expected %q, got %q", i, tt.want[i], got[i])
				}
			}
		})
	}
}

func TestDiff_LargeFile(t *testing.T) {
	var oldLines, newLines []string
	for i := 0; i < 100; i++ {
		oldLines = append(oldLines, "line"+itoa(i))
	}
	newLines = make([]string, len(oldLines))
	copy(newLines, oldLines)
	newLines[50] = "modified50"
	newLines[51] = "modified51"
	newLines = append(newLines[:75], append([]string{"inserted75"}, newLines[75:]...)...)

	oldText := strings.Join(oldLines, "\n") + "\n"
	newText := strings.Join(newLines, "\n") + "\n"

	result, err := Diff(oldText, newText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hunks) == 0 {
		t.Fatal("expected hunks for large file diff")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

func TestGeneratePatch_AllDeleted(t *testing.T) {
	old := "line1\nline2\nline3\n"
	patch, err := GeneratePatch("a.txt", "b.txt", old, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(patch, "-line1") {
		t.Error("patch should contain -line1")
	}
	if !strings.Contains(patch, "-line2") {
		t.Error("patch should contain -line2")
	}
	if !strings.Contains(patch, "-line3") {
		t.Error("patch should contain -line3")
	}
}

func TestGeneratePatch_AllInserted(t *testing.T) {
	new_ := "line1\nline2\nline3\n"
	patch, err := GeneratePatch("a.txt", "b.txt", "", new_)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(patch, "+line1") {
		t.Error("patch should contain +line1")
	}
	if !strings.Contains(patch, "+line2") {
		t.Error("patch should contain +line2")
	}
	if !strings.Contains(patch, "+line3") {
		t.Error("patch should contain +line3")
	}
}

func TestApplyPatch_EmptyOriginal(t *testing.T) {
	new_ := "line1\nline2\n"
	patch, err := GeneratePatch("a.txt", "b.txt", "", new_)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := ApplyPatch("", patch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rejected {
		t.Error("patch should not be rejected")
	}
	if result.Text != new_ {
		t.Errorf("expected %q, got %q", new_, result.Text)
	}
}

func TestApplyPatch_EmptyNew(t *testing.T) {
	old := "line1\nline2\n"
	patch, err := GeneratePatch("a.txt", "b.txt", old, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := ApplyPatch(old, patch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rejected {
		t.Error("patch should not be rejected")
	}
	if result.Text != "" {
		t.Errorf("expected empty result, got %q", result.Text)
	}
}

func TestDiff_ReplaceEntireContent(t *testing.T) {
	result, err := Diff("a\nb\n", "x\ny\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hunks) == 0 {
		t.Fatal("expected hunks")
	}

	deleted := 0
	inserted := 0
	for _, hunk := range result.Hunks {
		for _, line := range hunk.Lines {
			if line.Type == LineDelete {
				deleted++
			}
			if line.Type == LineInsert {
				inserted++
			}
		}
	}
	if deleted != 2 {
		t.Errorf("expected 2 deletions, got %d", deleted)
	}
	if inserted != 2 {
		t.Errorf("expected 2 insertions, got %d", inserted)
	}
}

func TestParsePatch_MultipleHunks(t *testing.T) {
	patchText := "--- a.txt\n+++ b.txt\n@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n@@ -5,3 +5,3 @@\n e\n-f\n+F\n g\n"
	patch, err := ParsePatch(patchText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(patch.Hunks) != 2 {
		t.Errorf("expected 2 hunks, got %d", len(patch.Hunks))
	}
}

func TestApplyPatch_MultipleHunks(t *testing.T) {
	old := "a\nb\nc\nd\ne\nf\ng\n"
	new_ := "a\nB\nc\nd\ne\nF\ng\n"
	patch, err := GeneratePatch("a.txt", "b.txt", old, new_)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := ApplyPatch(old, patch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rejected {
		t.Errorf("patch should not be rejected")
	}
	if result.Text != new_ {
		t.Errorf("expected %q, got %q", new_, result.Text)
	}
}

func TestDiff_OneLineNoNewline(t *testing.T) {
	result, err := Diff("hello", "world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hunks) == 0 {
		t.Fatal("expected hunks for single line no newline diff")
	}
}

func TestThreeWayMerge_InsertNonOverlapping(t *testing.T) {
	base := "a\nb\nc\n"
	ours := "a\nX\nb\nc\n"
	theirs := "a\nb\nc\nY\n"

	result, err := ThreeWayMerge(base, ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasConflicts {
		t.Error("should not have conflicts for non-overlapping insertions")
	}
	if !strings.Contains(result.Text, "X") {
		t.Error("merged should contain our insertion X")
	}
	if !strings.Contains(result.Text, "Y") {
		t.Error("merged should contain their insertion Y")
	}
}

func TestThreeWayMerge_DeleteVsModify(t *testing.T) {
	base := "a\nb\nc\n"
	ours := "a\nc\n"
	theirs := "a\nB\nc\n"

	result, err := ThreeWayMerge(base, ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasConflicts {
		t.Error("expected conflict when one side deletes and other modifies same line")
	}
}

func TestApplyPatch_ContextVerificationSuccess(t *testing.T) {
	old := "line1\nline2\nline3\nline4\nline5\n"
	patch := &Patch{
		Header: PatchHeader{OldFile: "a", NewFile: "b"},
		Hunks: []Hunk{
			{
				OldStart: 2, OldCount: 3,
				NewStart: 2, NewCount: 3,
				Lines: []Line{
					{Content: "line2", Type: LineEqual},
					{Content: "line3", Type: LineDelete},
					{Content: "LINE3", Type: LineInsert},
					{Content: "line4", Type: LineEqual},
				},
			},
		},
	}

	result, err := ApplyPatchDirect(old, patch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rejected {
		t.Error("patch should not be rejected when context matches")
	}
	expected := "line1\nline2\nLINE3\nline4\nline5\n"
	if result.Text != expected {
		t.Errorf("expected %q, got %q", expected, result.Text)
	}
}

func TestApplyPatch_ContextVerificationFailure(t *testing.T) {
	old := "line1\nDIFFERENT\nline3\nline4\nline5\n"
	patch := &Patch{
		Header: PatchHeader{OldFile: "a", NewFile: "b"},
		Hunks: []Hunk{
			{
				OldStart: 2, OldCount: 3,
				NewStart: 2, NewCount: 3,
				Lines: []Line{
					{Content: "line2", Type: LineEqual},
					{Content: "line3", Type: LineDelete},
					{Content: "LINE3", Type: LineInsert},
					{Content: "line4", Type: LineEqual},
				},
			},
		},
	}

	result, err := ApplyPatchDirect(old, patch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Rejected {
		t.Error("patch should be rejected when context doesn't match")
	}
	if len(result.Conflicts) == 0 {
		t.Error("expected conflict ranges")
	}
}

func TestDiff_LongSequence(t *testing.T) {
	var oldBuilder, newBuilder strings.Builder
	for i := 0; i < 50; i++ {
		oldBuilder.WriteString("line")
		oldBuilder.WriteString(itoa(i))
		oldBuilder.WriteString("\n")
	}
	for i := 0; i < 50; i++ {
		if i == 25 {
			newBuilder.WriteString("CHANGED25\n")
		} else {
			newBuilder.WriteString("line")
			newBuilder.WriteString(itoa(i))
			newBuilder.WriteString("\n")
		}
	}

	result, err := Diff(oldBuilder.String(), newBuilder.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hunks) == 0 {
		t.Fatal("expected hunks for long sequence")
	}

	foundChange := false
	for _, hunk := range result.Hunks {
		for _, line := range hunk.Lines {
			if line.Type == LineInsert && line.Content == "CHANGED25" {
				foundChange = true
			}
		}
	}
	if !foundChange {
		t.Error("expected to find CHANGED25 in diff result")
	}
}

func TestEndToEnd_DiffGenerateApply(t *testing.T) {
	old := "aaa\nbbb\nccc\nddd\neee\n"
	new_ := "aaa\nBBB\nccc\nddd\nEEE\n"

	patchText, err := GeneratePatch("original.txt", "modified.txt", old, new_)
	if err != nil {
		t.Fatalf("GeneratePatch error: %v", err)
	}

	applyResult, err := ApplyPatch(old, patchText)
	if err != nil {
		t.Fatalf("ApplyPatch error: %v", err)
	}
	if applyResult.Rejected {
		t.Error("patch should apply cleanly")
	}
	if applyResult.Text != new_ {
		t.Errorf("expected %q, got %q", new_, applyResult.Text)
	}
}

func TestEndToEnd_DiffMerge(t *testing.T) {
	base := "line1\nline2\nline3\nline4\nline5\n"
	ours := "line1\nline2\nMODIFIED3\nline4\nline5\n"
	theirs := "line1\nline2\nline3\nline4\nMODIFIED5\n"

	result, err := ThreeWayMerge(base, ours, theirs)
	if err != nil {
		t.Fatalf("ThreeWayMerge error: %v", err)
	}
	if result.HasConflicts {
		t.Error("should merge cleanly for non-overlapping changes")
	}
	if !strings.Contains(result.Text, "MODIFIED3") {
		t.Error("merged should contain our change")
	}
	if !strings.Contains(result.Text, "MODIFIED5") {
		t.Error("merged should contain their change")
	}
}

func TestDiff_ConsecutiveModifications(t *testing.T) {
	old := "a\nb\nc\n"
	new_ := "A\nB\nC\n"
	result, err := Diff(old, new_)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hunks) == 0 {
		t.Fatal("expected hunks")
	}

	totalDeletes := 0
	totalInserts := 0
	for _, hunk := range result.Hunks {
		for _, line := range hunk.Lines {
			if line.Type == LineDelete {
				totalDeletes++
			}
			if line.Type == LineInsert {
				totalInserts++
			}
		}
	}
	if totalDeletes != 3 {
		t.Errorf("expected 3 deletes, got %d", totalDeletes)
	}
	if totalInserts != 3 {
		t.Errorf("expected 3 inserts, got %d", totalInserts)
	}
}

func TestThreeWayMerge_EmptyBase(t *testing.T) {
	ours := "a\nb\n"
	theirs := "c\nd\n"

	result, err := ThreeWayMerge("", ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasConflicts {
		t.Error("expected conflict when both sides add different content to empty base")
	}
}

func TestParsePatch_HunkWithOnlyInserts(t *testing.T) {
	patchText := "--- a.txt\n+++ b.txt\n@@ -0,0 +1,2 @@\n+x\n+y\n"
	patch, err := ParsePatch(patchText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(patch.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(patch.Hunks))
	}
	insertCount := 0
	for _, line := range patch.Hunks[0].Lines {
		if line.Type == LineInsert {
			insertCount++
		}
	}
	if insertCount != 2 {
		t.Errorf("expected 2 insert lines, got %d", insertCount)
	}
}

func TestApplyPatch_PatchWithOnlyInserts(t *testing.T) {
	patch := &Patch{
		Header: PatchHeader{OldFile: "a", NewFile: "b"},
		Hunks: []Hunk{
			{
				OldStart: 0, OldCount: 0,
				NewStart: 1, NewCount: 2,
				Lines: []Line{
					{Content: "x", Type: LineInsert},
					{Content: "y", Type: LineInsert},
				},
			},
		},
	}

	result, err := ApplyPatchDirect("", patch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rejected {
		t.Error("patch should not be rejected")
	}
	expected := "x\ny\n"
	if result.Text != expected {
		t.Errorf("expected %q, got %q", expected, result.Text)
	}
}

func TestDiff_TabsAndSpaces(t *testing.T) {
	old := "\tindented\n  spaced\n"
	new_ := "\tindented\n\tspaced\n"
	result, err := Diff(old, new_)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hunks) == 0 {
		t.Fatal("expected hunks for whitespace changes")
	}
}

func TestRangesOverlap(t *testing.T) {
	tests := []struct {
		name                  string
		aStart, aEnd, bStart, bEnd int
		want                  bool
	}{
		{"no overlap", 0, 2, 3, 5, false},
		{"overlap", 0, 3, 2, 5, true},
		{"contained", 1, 3, 0, 5, true},
		{"adjacent", 0, 2, 2, 4, false},
		{"same range", 0, 3, 0, 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rangesOverlap(tt.aStart, tt.aEnd, tt.bStart, tt.bEnd)
			if got != tt.want {
				t.Errorf("rangesOverlap(%d,%d,%d,%d) = %v, want %v",
					tt.aStart, tt.aEnd, tt.bStart, tt.bEnd, got, tt.want)
			}
		})
	}
}

func TestSameContent(t *testing.T) {
	if !sameContent([]string{"a", "b"}, []string{"a", "b"}) {
		t.Error("same content should be equal")
	}
	if sameContent([]string{"a", "b"}, []string{"a", "c"}) {
		t.Error("different content should not be equal")
	}
	if sameContent([]string{"a"}, []string{"a", "b"}) {
		t.Error("different lengths should not be equal")
	}
	if !sameContent(nil, nil) {
		t.Error("both nil should be equal")
	}
}

func TestDiffToChanges_SingleHunkMultipleChanges(t *testing.T) {
	diffResult := &DiffResult{
		Hunks: []Hunk{
			{
				OldStart: 1, OldCount: 10,
				NewStart: 1, NewCount: 10,
				Lines: []Line{
					{Content: "line1", Type: LineEqual, OldLineNo: 1, NewLineNo: 1},
					{Content: "line2", Type: LineDelete, OldLineNo: 2, NewLineNo: 0},
					{Content: "OURS2", Type: LineInsert, OldLineNo: 0, NewLineNo: 2},
					{Content: "line3", Type: LineEqual, OldLineNo: 3, NewLineNo: 3},
					{Content: "line4", Type: LineEqual, OldLineNo: 4, NewLineNo: 4},
					{Content: "line5", Type: LineEqual, OldLineNo: 5, NewLineNo: 5},
					{Content: "line6", Type: LineDelete, OldLineNo: 6, NewLineNo: 0},
					{Content: "OURS6", Type: LineInsert, OldLineNo: 0, NewLineNo: 6},
					{Content: "line7", Type: LineEqual, OldLineNo: 7, NewLineNo: 7},
					{Content: "line8", Type: LineEqual, OldLineNo: 8, NewLineNo: 8},
					{Content: "line9", Type: LineDelete, OldLineNo: 9, NewLineNo: 0},
					{Content: "OURS9", Type: LineInsert, OldLineNo: 0, NewLineNo: 9},
					{Content: "line10", Type: LineEqual, OldLineNo: 10, NewLineNo: 10},
				},
			},
		},
	}

	changes := diffToChanges(diffResult)

	if len(changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(changes))
	}

	if changes[0].oldStart != 1 {
		t.Errorf("change 0 oldStart: expected 1, got %d", changes[0].oldStart)
	}
	if changes[0].oldEnd != 2 {
		t.Errorf("change 0 oldEnd: expected 2, got %d", changes[0].oldEnd)
	}
	if len(changes[0].newLines) != 1 || changes[0].newLines[0] != "OURS2" {
		t.Errorf("change 0 newLines: expected [OURS2], got %v", changes[0].newLines)
	}

	if changes[1].oldStart != 5 {
		t.Errorf("change 1 oldStart: expected 5, got %d", changes[1].oldStart)
	}
	if changes[1].oldEnd != 6 {
		t.Errorf("change 1 oldEnd: expected 6, got %d", changes[1].oldEnd)
	}
	if len(changes[1].newLines) != 1 || changes[1].newLines[0] != "OURS6" {
		t.Errorf("change 1 newLines: expected [OURS6], got %v", changes[1].newLines)
	}

	if changes[2].oldStart != 8 {
		t.Errorf("change 2 oldStart: expected 8, got %d", changes[2].oldStart)
	}
	if changes[2].oldEnd != 9 {
		t.Errorf("change 2 oldEnd: expected 9, got %d", changes[2].oldEnd)
	}
	if len(changes[2].newLines) != 1 || changes[2].newLines[0] != "OURS9" {
		t.Errorf("change 2 newLines: expected [OURS9], got %v", changes[2].newLines)
	}
}

func TestThreeWayMerge_SingleHunkMultipleChanges(t *testing.T) {
	base := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"
	ours := "line1\nOURS2\nline3\nline4\nline5\nOURS6\nline7\nline8\nOURS9\nline10\n"
	theirs := "line1\nline2\nline3\nTHEIRS4\nline5\nline6\nline7\nTHEIRS8\nline9\nline10\n"

	result, err := ThreeWayMerge(base, ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasConflicts {
		t.Error("should not have conflicts for non-overlapping changes")
	}

	expectedChanges := []string{"OURS2", "THEIRS4", "OURS6", "THEIRS8", "OURS9"}
	for _, expected := range expectedChanges {
		if !strings.Contains(result.Text, expected) {
			t.Errorf("merged text should contain %q", expected)
		}
	}

	unwantedChanges := []string{"line2", "line4", "line6", "line8", "line9"}
	for _, unwanted := range unwantedChanges {
		if strings.Contains(result.Text, unwanted+"\n") {
			t.Errorf("merged text should not contain original %q", unwanted)
		}
	}

	expectedLineCount := 10
	actualLines := len(splitLines(result.Text))
	if actualLines != expectedLineCount {
		t.Errorf("expected %d lines, got %d lines\nresult:\n%s", expectedLineCount, actualLines, result.Text)
	}
}
