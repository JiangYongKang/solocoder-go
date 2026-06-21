package diffpatch

import (
	"fmt"
	"strings"
)

func GeneratePatch(oldFile, newFile, oldText, newText string) (string, error) {
	diffResult, err := Diff(oldText, newText)
	if err != nil {
		return "", err
	}

	patch := &Patch{
		Header: PatchHeader{
			OldFile: oldFile,
			NewFile: newFile,
		},
		Hunks: diffResult.Hunks,
	}

	return PatchToUnified(patch), nil
}

func PatchToUnified(p *Patch) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("--- %s\n", p.Header.OldFile))
	sb.WriteString(fmt.Sprintf("+++ %s\n", p.Header.NewFile))

	for _, hunk := range p.Hunks {
		sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n",
			hunk.OldStart, hunk.OldCount,
			hunk.NewStart, hunk.NewCount))

		for _, line := range hunk.Lines {
			switch line.Type {
			case LineEqual:
				sb.WriteString(" ")
				sb.WriteString(line.Content)
				sb.WriteString("\n")
			case LineDelete:
				sb.WriteString("-")
				sb.WriteString(line.Content)
				sb.WriteString("\n")
			case LineInsert:
				sb.WriteString("+")
				sb.WriteString(line.Content)
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

func ParsePatch(patchText string) (*Patch, error) {
	lines := splitText(patchText)
	if len(lines) < 2 {
		return nil, ErrInvalidPatch
	}

	var header PatchHeader
	var hunks []Hunk
	var currentHunk *Hunk
	i := 0

	for i < len(lines) {
		line := lines[i]

		if strings.HasPrefix(line, "--- ") {
			header.OldFile = strings.TrimPrefix(line, "--- ")
			i++
			continue
		}

		if strings.HasPrefix(line, "+++ ") {
			header.NewFile = strings.TrimPrefix(line, "+++ ")
			i++
			continue
		}

		if strings.HasPrefix(line, "@@ ") {
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
			}

			hunk, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			currentHunk = &hunk
			i++
			continue
		}

		if currentHunk != nil {
			if strings.HasPrefix(line, "-") {
				currentHunk.Lines = append(currentHunk.Lines, Line{
					Content: line[1:],
					Type:    LineDelete,
				})
			} else if strings.HasPrefix(line, "+") {
				currentHunk.Lines = append(currentHunk.Lines, Line{
					Content: line[1:],
					Type:    LineInsert,
				})
			} else if strings.HasPrefix(line, " ") {
				currentHunk.Lines = append(currentHunk.Lines, Line{
					Content: line[1:],
					Type:    LineEqual,
				})
			}
		}

		i++
	}

	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}

	if header.OldFile == "" && header.NewFile == "" && len(hunks) == 0 {
		return nil, ErrInvalidPatch
	}

	return &Patch{
		Header: header,
		Hunks:  hunks,
	}, nil
}

func parseHunkHeader(line string) (Hunk, error) {
	line = strings.TrimPrefix(line, "@@ ")
	idx := strings.Index(line, " @@")
	if idx < 0 {
		return Hunk{}, ErrInvalidPatch
	}
	rangePart := line[:idx]

	parts := strings.SplitN(rangePart, " ", 2)
	if len(parts) != 2 {
		return Hunk{}, ErrInvalidPatch
	}

	oldStart, oldCount, err := parseRange(parts[0])
	if err != nil {
		return Hunk{}, err
	}

	newStart, newCount, err := parseRange(parts[1])
	if err != nil {
		return Hunk{}, err
	}

	return Hunk{
		OldStart: oldStart,
		OldCount: oldCount,
		NewStart: newStart,
		NewCount: newCount,
	}, nil
}

func parseRange(s string) (int, int, error) {
	s = strings.TrimPrefix(s, "-")
	s = strings.TrimPrefix(s, "+")

	parts := strings.SplitN(s, ",", 2)
	start := 0
	count := 0

	if _, err := fmt.Sscanf(parts[0], "%d", &start); err != nil {
		return 0, 0, ErrInvalidPatch
	}

	if len(parts) > 1 {
		if _, err := fmt.Sscanf(parts[1], "%d", &count); err != nil {
			return 0, 0, ErrInvalidPatch
		}
	} else {
		count = 1
	}

	return start, count, nil
}
