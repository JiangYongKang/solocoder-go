package diffpatch

import (
	"fmt"
	"strings"
)

func ThreeWayMerge(baseText, oursText, theirsText string) (*MergeResult, error) {
	if baseText == oursText {
		return &MergeResult{
			Text:        theirsText,
			HasConflicts: false,
		}, nil
	}
	if baseText == theirsText {
		return &MergeResult{
			Text:        oursText,
			HasConflicts: false,
		}, nil
	}
	if oursText == theirsText {
		return &MergeResult{
			Text:        oursText,
			HasConflicts: false,
		}, nil
	}

	oursDiff, err := Diff(baseText, oursText)
	if err != nil {
		return nil, err
	}
	theirsDiff, err := Diff(baseText, theirsText)
	if err != nil {
		return nil, err
	}

	oursChanges := diffToChanges(oursDiff)
	theirsChanges := diffToChanges(theirsDiff)

	conflicts := detectConflicts(oursChanges, theirsChanges)
	if len(conflicts) > 0 {
		baseLines := splitLines(baseText)
		oursLines := splitLines(oursText)
		theirsLines := splitLines(theirsText)

		merged := mergeWithConflictMarkers(baseLines, oursLines, theirsLines, oursChanges, theirsChanges, conflicts)
		return &MergeResult{
			Text:        merged,
			HasConflicts: true,
			Conflicts:   conflicts,
		}, nil
	}

	baseLines := splitLines(baseText)
	merged := applyBothChanges(baseLines, oursChanges, theirsChanges)

	return &MergeResult{
		Text:        merged,
		HasConflicts: false,
	}, nil
}

type change struct {
	oldStart int
	oldEnd   int
	newLines []string
	source   string
}

func diffToChanges(diffResult *DiffResult) []change {
	var changes []change

	for _, hunk := range diffResult.Hunks {
		var deleted int
		var inserted []string
		changeStart := hunk.OldStart - 1

		for _, line := range hunk.Lines {
			switch line.Type {
			case LineDelete:
				deleted++
			case LineInsert:
				inserted = append(inserted, line.Content)
			case LineEqual:
				if deleted > 0 || len(inserted) > 0 {
					changes = append(changes, change{
						oldStart: changeStart,
						oldEnd:   changeStart + deleted,
						newLines: inserted,
					})
					deleted = 0
					inserted = nil
				}
				changeStart = line.OldLineNo
			}
		}

		if deleted > 0 || len(inserted) > 0 {
			changes = append(changes, change{
				oldStart: changeStart,
				oldEnd:   changeStart + deleted,
				newLines: inserted,
			})
		}
	}

	return changes
}

func detectConflicts(oursChanges, theirsChanges []change) []ConflictRange {
	var conflicts []ConflictRange

	for _, oc := range oursChanges {
		for _, tc := range theirsChanges {
			if rangesOverlap(oc.oldStart, oc.oldEnd, tc.oldStart, tc.oldEnd) {
				if !sameContent(oc.newLines, tc.newLines) {
					conflicts = append(conflicts, ConflictRange{
						StartLine: oc.oldStart + 1,
						EndLine:   oc.oldEnd,
						Ours:      oc.newLines,
						Theirs:    tc.newLines,
					})
				}
			}
		}
	}

	return conflicts
}

func rangesOverlap(aStart, aEnd, bStart, bEnd int) bool {
	if aStart == bStart {
		return true
	}
	if aStart < bEnd && aEnd > bStart {
		return true
	}
	return false
}

func sameContent(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mergeWithConflictMarkers(baseLines, oursLines, theirsLines []string, oursChanges, theirsChanges []change, conflicts []ConflictRange) string {
	allChanges := mergeChanges(oursChanges, theirsChanges)

	var result []string
	baseIdx := 0

	for _, ch := range allChanges {
		for baseIdx < ch.oldStart && baseIdx < len(baseLines) {
			result = append(result, baseLines[baseIdx])
			baseIdx++
		}

		isConflict := false
		for _, c := range conflicts {
			if ch.oldStart >= c.StartLine-1 && ch.oldStart < c.EndLine {
				isConflict = true
				break
			}
		}

		if isConflict {
			var oursContent []string
			var theirsContent []string

			for _, oc := range oursChanges {
				if oc.oldStart == ch.oldStart {
					oursContent = oc.newLines
					break
				}
			}
			for _, tc := range theirsChanges {
				if tc.oldStart == ch.oldStart {
					theirsContent = tc.newLines
					break
				}
			}

			result = append(result, "<<<<<<< ours")
			result = append(result, oursContent...)
			result = append(result, "=======")
			result = append(result, theirsContent...)
			result = append(result, ">>>>>>> theirs")
		} else {
			result = append(result, ch.newLines...)
		}

		baseIdx = ch.oldEnd
	}

	for baseIdx < len(baseLines) {
		result = append(result, baseLines[baseIdx])
		baseIdx++
	}

	return strings.Join(result, "\n")
}

func mergeChanges(oursChanges, theirsChanges []change) []change {
	all := make([]change, 0, len(oursChanges)+len(theirsChanges))
	all = append(all, oursChanges...)
	all = append(all, theirsChanges...)

	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[i].oldStart > all[j].oldStart {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	return deduplicateChanges(all)
}

func deduplicateChanges(changes []change) []change {
	seen := make(map[int]bool)
	var result []change
	for _, ch := range changes {
		if !seen[ch.oldStart] {
			seen[ch.oldStart] = true
			result = append(result, ch)
		}
	}
	return result
}

func applyBothChanges(baseLines []string, oursChanges, theirsChanges []change) string {
	allChanges := mergeChanges(oursChanges, theirsChanges)

	var result []string
	baseIdx := 0

	for _, ch := range allChanges {
		for baseIdx < ch.oldStart && baseIdx < len(baseLines) {
			result = append(result, baseLines[baseIdx])
			baseIdx++
		}

		if baseIdx < ch.oldEnd {
			baseIdx = ch.oldEnd
		}

		result = append(result, ch.newLines...)
	}

	for baseIdx < len(baseLines) {
		result = append(result, baseLines[baseIdx])
		baseIdx++
	}

	return strings.Join(result, "\n")
}

func FormatConflict(conflict ConflictRange) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<<<<<<< ours\n"))
	for _, line := range conflict.Ours {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	sb.WriteString("=======\n")
	for _, line := range conflict.Theirs {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	sb.WriteString(">>>>>>> theirs\n")
	return sb.String()
}
