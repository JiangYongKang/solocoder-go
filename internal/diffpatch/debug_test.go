package diffpatch

import (
	"fmt"
	"testing"
)

func TestDebug_BuildHunksLineNumbers(t *testing.T) {
	oldText := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"
	newText := "line1\nMOD2\nline3\nline4\nline5\nMOD6\nline7\nline8\nline9\nline10\n"

	result, err := Diff(oldText, newText)
	if err != nil {
		t.Fatalf("diff error: %v", err)
	}

	t.Logf("Number of hunks: %d", len(result.Hunks))
	for i, hunk := range result.Hunks {
		t.Logf("Hunk %d: OldStart=%d, OldCount=%d, NewStart=%d, NewCount=%d",
			i, hunk.OldStart, hunk.OldCount, hunk.NewStart, hunk.NewCount)
		for j, line := range hunk.Lines {
			var prefix string
			switch line.Type {
			case LineEqual:
				prefix = " "
			case LineDelete:
				prefix = "-"
			case LineInsert:
				prefix = "+"
			}
			t.Logf("  Line %d: %s%s (OldLineNo=%d, NewLineNo=%d)",
				j, prefix, line.Content, line.OldLineNo, line.NewLineNo)
		}
	}

	changes := diffToChanges(result)
	t.Logf("Number of changes: %d", len(changes))
	for i, ch := range changes {
		t.Logf("Change %d: oldStart=%d, oldEnd=%d, newLines=%v", i, ch.oldStart, ch.oldEnd, ch.newLines)
	}

	fmt.Println("--- Detailed analysis ---")
	for i, hunk := range result.Hunks {
		fmt.Printf("Hunk %d:\n", i)
		fmt.Printf("  OldStart (1-based): %d\n", hunk.OldStart)
		fmt.Printf("  changeStart initial (0-based): %d\n", hunk.OldStart-1)
		
		for _, line := range hunk.Lines {
			if line.Type == LineEqual {
				fmt.Printf("  Equal line '%s': OldLineNo=%d (1-based) => changeStart becomes %d (= next line 0-based idx = %d\n",
					line.Content, line.OldLineNo, line.OldLineNo, line.OldLineNo)
			}
		}
	}
}
