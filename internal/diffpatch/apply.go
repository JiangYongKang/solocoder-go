package diffpatch

import (
	"strings"
)

func ApplyPatch(originalText, patchText string) (*ApplyResult, error) {
	patch, err := ParsePatch(patchText)
	if err != nil {
		return nil, err
	}

	return ApplyPatchDirect(originalText, patch)
}

func ApplyPatchDirect(originalText string, patch *Patch) (*ApplyResult, error) {
	originalLines := splitLines(originalText)
	trailingNewline := len(originalText) > 0 && originalText[len(originalText)-1] == '\n'
	if len(originalText) == 0 && len(patch.Hunks) > 0 {
		trailingNewline = true
	}

	var result []string
	var conflicts []ConflictRange

	origIdx := 0

	for _, hunk := range patch.Hunks {
		for origIdx < hunk.OldStart-1 {
			if origIdx < len(originalLines) {
				result = append(result, originalLines[origIdx])
			}
			origIdx++
		}

		contextLines := collectContextLines(hunk)
		expectedStart := hunk.OldStart - 1
		if expectedStart < 0 {
			expectedStart = 0
		}

		if !verifyContext(originalLines, expectedStart, contextLines) {
			conflict := ConflictRange{
				StartLine: hunk.OldStart,
				EndLine:   hunk.OldStart + hunk.OldCount - 1,
				Theirs:    extractNewLines(hunk),
				Base:      extractBaseLines(originalLines, expectedStart, hunk.OldCount),
				Ours:      extractBaseLines(originalLines, expectedStart, hunk.OldCount),
			}
			conflicts = append(conflicts, conflict)

			for i := 0; i < hunk.OldCount && origIdx < len(originalLines); i++ {
				result = append(result, originalLines[origIdx])
				origIdx++
			}
			continue
		}

		for _, line := range hunk.Lines {
			switch line.Type {
			case LineEqual:
				if origIdx < len(originalLines) {
					result = append(result, originalLines[origIdx])
					origIdx++
				}
			case LineDelete:
				origIdx++
			case LineInsert:
				result = append(result, line.Content)
			}
		}
	}

	for origIdx < len(originalLines) {
		result = append(result, originalLines[origIdx])
		origIdx++
	}

	text := strings.Join(result, "\n")
	if trailingNewline && len(result) > 0 {
		text += "\n"
	}

	return &ApplyResult{
		Text:      text,
		Rejected:  len(conflicts) > 0,
		Conflicts: conflicts,
	}, nil
}

func collectContextLines(hunk Hunk) []Line {
	return hunk.Lines
}

func verifyContext(originalLines []string, startIdx int, contextLines []Line) bool {
	origPos := startIdx

	for _, line := range contextLines {
		switch line.Type {
		case LineEqual:
			if origPos >= len(originalLines) {
				return false
			}
			if originalLines[origPos] != line.Content {
				return false
			}
			origPos++
		case LineDelete:
			if origPos >= len(originalLines) {
				return false
			}
			if originalLines[origPos] != line.Content {
				return false
			}
			origPos++
		case LineInsert:
		}
	}

	return true
}

func extractNewLines(hunk Hunk) []string {
	var lines []string
	for _, line := range hunk.Lines {
		if line.Type == LineInsert {
			lines = append(lines, line.Content)
		}
	}
	return lines
}

func extractBaseLines(originalLines []string, startIdx, count int) []string {
	end := startIdx + count
	if end > len(originalLines) {
		end = len(originalLines)
	}
	if startIdx >= len(originalLines) {
		return nil
	}
	result := make([]string, end-startIdx)
	copy(result, originalLines[startIdx:end])
	return result
}
