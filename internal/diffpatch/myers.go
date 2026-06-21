package diffpatch

type editOp int

const (
	opEqual  editOp = iota
	opDelete
	opInsert
)

type editScript struct {
	op   editOp
	line string
}

func myersDiff(a, b []string) []editScript {
	n := len(a)
	m := len(b)

	if n == 0 && m == 0 {
		return nil
	}
	if n == 0 {
		result := make([]editScript, m)
		for i := 0; i < m; i++ {
			result[i] = editScript{op: opInsert, line: b[i]}
		}
		return result
	}
	if m == 0 {
		result := make([]editScript, n)
		for i := 0; i < n; i++ {
			result[i] = editScript{op: opDelete, line: a[i]}
		}
		return result
	}

	max := n + m
	offset := max

	v := make([]int, 2*max+1)
	v[offset+1] = 0

	trace := make([][]int, 0, max+1)

	var d int
	for d = 0; d <= max; d++ {
		vCopy := make([]int, 2*max+1)
		copy(vCopy, v)
		trace = append(trace, vCopy)

		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}

			y := x - k

			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}

			v[offset+k] = x

			if x >= n && y >= m {
				return backtrack(trace, a, b, d, offset)
			}
		}
	}

	return nil
}

func backtrack(trace [][]int, a, b []string, d, offset int) []editScript {
	n := len(a)
	m := len(b)

	x := n
	y := m

	var scripts []editScript

	for dd := d; dd > 0; dd-- {
		v := trace[dd]
		k := x - y

		var prevK int
		if k == -dd || (k != dd && v[offset+k-1] < v[offset+k+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}

		prevX := v[offset+prevK]
		prevY := prevX - prevK

		for x > prevX && y > prevY {
			scripts = append(scripts, editScript{op: opEqual, line: a[x-1]})
			x--
			y--
		}

		if dd > 0 {
			if x > prevX {
				scripts = append(scripts, editScript{op: opDelete, line: a[x-1]})
				x--
			} else if y > prevY {
				scripts = append(scripts, editScript{op: opInsert, line: b[y-1]})
				y--
			}
		}
	}

	for x > 0 && y > 0 {
		scripts = append(scripts, editScript{op: opEqual, line: a[x-1]})
		x--
		y--
	}

	for i, j := 0, len(scripts)-1; i < j; i, j = i+1, j-1 {
		scripts[i], scripts[j] = scripts[j], scripts[i]
	}

	return scripts
}

func buildHunks(ses []editScript, oldLines, newLines []string) []Hunk {
	if len(ses) == 0 {
		return nil
	}

	contextSize := 3

	var hunks []Hunk
	var currentLines []Line
	var currentOldStart, currentNewStart int
	var currentOldCount, currentNewCount int
	inHunk := false

	oldIdx := 0
	newIdx := 0

	pendingEquals := 0

	for _, op := range ses {
		switch op.op {
		case opEqual:
			if inHunk {
				pendingEquals++
				if pendingEquals > contextSize*2 {
					currentLines = append(currentLines, Line{
						Content:   op.line,
						Type:      LineEqual,
						OldLineNo: oldIdx + 1,
						NewLineNo: newIdx + 1,
					})
					currentOldCount++
					currentNewCount++

					hunks = append(hunks, Hunk{
						OldStart: currentOldStart,
						OldCount: currentOldCount - contextSize,
						NewStart: currentNewStart,
						NewCount: currentNewCount - contextSize,
						Lines:    currentLines[:len(currentLines)-contextSize],
					})

					currentLines = nil
					inHunk = false
					pendingEquals = 0

					oldIdx++
					newIdx++
					continue
				}
				currentLines = append(currentLines, Line{
					Content:   op.line,
					Type:      LineEqual,
					OldLineNo: oldIdx + 1,
					NewLineNo: newIdx + 1,
				})
				currentOldCount++
				currentNewCount++
			}
			oldIdx++
			newIdx++
		case opDelete:
			if !inHunk {
				inHunk = true
				pendingEquals = 0
				startOld := oldIdx - contextSize
				if startOld < 0 {
					startOld = 0
				}
				currentOldStart = startOld + 1
				currentNewStart = newIdx - (oldIdx - startOld) + 1
				if currentNewStart < 1 {
					currentNewStart = 1
				}
				currentOldCount = 0
				currentNewCount = 0
				currentLines = nil

				for i := startOld; i < oldIdx; i++ {
					currentLines = append(currentLines, Line{
						Content:   oldLines[i],
						Type:      LineEqual,
						OldLineNo: i + 1,
						NewLineNo: i + 1,
					})
					currentOldCount++
					currentNewCount++
				}
			} else {
				pendingEquals = 0
			}
			currentLines = append(currentLines, Line{
				Content:   op.line,
				Type:      LineDelete,
				OldLineNo: oldIdx + 1,
				NewLineNo: 0,
			})
			currentOldCount++
			oldIdx++
		case opInsert:
			if !inHunk {
				inHunk = true
				pendingEquals = 0
				startOld := oldIdx - contextSize
				if startOld < 0 {
					startOld = 0
				}
				currentOldStart = startOld + 1
				currentNewStart = newIdx - (oldIdx - startOld) + 1
				if currentNewStart < 1 {
					currentNewStart = 1
				}
				currentOldCount = 0
				currentNewCount = 0
				currentLines = nil

				for i := startOld; i < oldIdx; i++ {
					currentLines = append(currentLines, Line{
						Content:   oldLines[i],
						Type:      LineEqual,
						OldLineNo: i + 1,
						NewLineNo: i + 1,
					})
					currentOldCount++
					currentNewCount++
				}
			} else {
				pendingEquals = 0
			}
			currentLines = append(currentLines, Line{
				Content:   op.line,
				Type:      LineInsert,
				OldLineNo: 0,
				NewLineNo: newIdx + 1,
			})
			currentNewCount++
			newIdx++
		}
	}

	if inHunk {
		trailingContext := pendingEquals
		if trailingContext > contextSize {
			excess := trailingContext - contextSize
			currentLines = currentLines[:len(currentLines)-excess]
			currentOldCount -= excess
			currentNewCount -= excess
		}
		hunks = append(hunks, Hunk{
			OldStart: currentOldStart,
			OldCount: currentOldCount,
			NewStart: currentNewStart,
			NewCount: currentNewCount,
			Lines:    currentLines,
		})
	}

	return hunks
}
