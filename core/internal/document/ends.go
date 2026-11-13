package document

import (
	"errors"
	"slices"
	"strings"

	"github.com/duxlabs-dev/dux-edit/core/internal/buffer"
	"github.com/duxlabs-dev/dux-edit/core/internal/textcodec"
)

// Qt can merge deltas into one undo step; only ending changes are kept here, not text history.
type endUndo struct {
	step   int
	start  int
	before []textcodec.EOL
	after  []textcodec.EOL
}

func (d *Doc) replace(next edit) (buffer.Change, error) {
	if d.ends == nil || next.undo == nil {
		d.clearEndUndo()
		return d.replaceText(next)
	}
	if next.undo.Steps < d.undoSteps || next.undo.Redo {
		cursor := d.endCursor
		switch {
		case next.undo.Redo:
			for cursor < len(d.endUndo) && d.endUndo[cursor].step <= next.undo.Steps {
				cursor++
			}
		default:
			for cursor > 0 && d.endUndo[cursor-1].step > next.undo.Steps {
				cursor--
			}
		}
		d.undoSteps = next.undo.Steps
		if cursor == d.endCursor {
			return d.replaceText(next)
		}
		change, err := d.buf.Replace(next.start, next.removed, next.text)
		if err != nil || d.buf.Len16() != next.length {
			return change, err
		}
		for d.endCursor != cursor {
			index := d.endCursor
			if !next.undo.Redo {
				index--
			}
			entry := d.endUndo[index]
			old, fresh := entry.after, entry.before
			if next.undo.Redo {
				old, fresh = entry.before, entry.after
			}
			end := entry.start + len(old)
			if end > len(d.ends) || !slices.Equal(d.ends[entry.start:end], old) {
				return change, errors.New("line ending undo history does not match mirror")
			}
			d.ends = slices.Replace(d.ends, entry.start, end, fresh...)
			d.endCursor = index
			if next.undo.Redo {
				d.endCursor++
			}
		}
		if len(d.ends) != d.buf.LineCount()-1 {
			return change, errors.New("line ending undo history has wrong line count")
		}
		return change, nil
	}

	first, _, err := d.buf.Locate(next.start)
	if err != nil {
		return buffer.Change{}, err
	}
	last, _, err := d.buf.Locate(next.start + next.removed)
	if err != nil {
		return buffer.Change{}, err
	}
	clear(d.endUndo[d.endCursor:])
	d.endUndo = d.endUndo[:d.endCursor]
	d.undoSteps = next.undo.Steps
	if first == last && !strings.Contains(next.text, "\n") {
		return d.replaceText(next)
	}
	before := slices.Clone(d.ends[first:min(last+1, len(d.ends))])
	change, err := d.replaceText(next)
	if err != nil || d.buf.Len16() != next.length {
		return change, err
	}
	after := d.ends[first:min(first+change.NewLines, len(d.ends))]
	if slices.Equal(before, after) {
		return change, nil
	}
	d.endUndo = append(d.endUndo, endUndo{
		step: next.undo.Steps, start: first, before: before, after: slices.Clone(after),
	})
	d.endCursor = len(d.endUndo)
	return change, nil
}

func (d *Doc) clearEndUndo() {
	d.endUndo = nil
	d.endCursor = 0
	d.undoSteps = 0
}

func (d *Doc) replaceText(next edit) (buffer.Change, error) {
	if d.ends == nil || next.removed == 0 && !strings.Contains(next.text, "\n") {
		return d.buf.Replace(next.start, next.removed, next.text)
	}
	first, _, err := d.buf.Locate(next.start)
	if err != nil {
		return buffer.Change{}, err
	}
	last, _, err := d.buf.Locate(next.start + next.removed)
	if err != nil {
		return buffer.Change{}, err
	}
	if first == last && !strings.Contains(next.text, "\n") {
		return d.buf.Replace(next.start, next.removed, next.text)
	}
	old := d.buf.Lines(first, last+1)
	change, err := d.buf.Replace(next.start, next.removed, next.text)
	if err != nil || d.buf.Len16() != next.length {
		return change, err
	}
	fresh := d.buf.Lines(change.StartLine, change.StartLine+change.NewLines)
	d.ends = spliceEnds(d.ends, change, old, fresh, d.info.EOL)
	return change, nil
}

// Pure permutations match duplicate occurrences in order; transforms first anchor unchanged positions.
func spliceEnds(ends []textcodec.EOL, c buffer.Change, old, fresh []string, fill textcodec.EOL) []textcodec.EOL {
	if len(old) == 1 && len(fresh) == 1 {
		return ends
	}
	from := min(c.StartLine, len(ends))
	to := min(c.StartLine+c.OldLines, len(ends))
	count := c.NewLines
	hasBoundary := c.StartLine+c.OldLines <= len(ends)
	if !hasBoundary {
		count--
	}
	replacement := slices.Repeat([]textcodec.EOL{fill}, count)
	if hasBoundary {
		replacement[len(replacement)-1] = ends[to-1]
	}

	origins, used := lineOrigins(old, fresh, false)
	switch {
	case len(old) == len(fresh):
		if slices.Contains(origins, -1) {
			if transformedInPlace(old, fresh) {
				return ends
			}
			origins, used = lineOrigins(old, fresh, true)
		}
		origin := 0
		for i := range origins {
			if origins[i] >= 0 {
				continue
			}
			for used[origin] {
				origin++
			}
			origins[i], used[origin] = origin, true
		}
	case len(old) > len(fresh):
		if line, origin, ok := joinedOrigin(old, fresh, origins, used); ok {
			origins[line] = origin
		}
	}
	for i := range replacement {
		origin := c.StartLine + origins[i]
		if origins[i] >= 0 && origin < len(ends) {
			replacement[i] = ends[origin]
		}
	}
	return slices.Replace(ends, from, to, replacement...)
}

func lineOrigins(old, fresh []string, unchanged bool) ([]int, []bool) {
	origins := slices.Repeat([]int{-1}, len(fresh))
	used := make([]bool, len(old))
	matches := make(map[string][]int, len(old))
	for i, line := range old {
		if unchanged && line == fresh[i] {
			origins[i], used[i] = i, true
			continue
		}
		matches[line] = append(matches[line], i)
	}
	for i, line := range fresh {
		if origins[i] >= 0 {
			continue
		}
		candidates := matches[line]
		if len(candidates) == 0 {
			continue
		}
		origin := candidates[0]
		matches[line] = candidates[1:]
		origins[i], used[origin] = origin, true
	}
	return origins, used
}

// Indentation or case conversion can produce another source line's text without moving it.
func transformedInPlace(old, fresh []string) bool {
	upper, lower, indent := true, true, true
	direction := 0
	for i, line := range old {
		upper = upper && strings.ToUpper(line) == fresh[i]
		lower = lower && strings.ToLower(line) == fresh[i]
		if !indent {
			continue
		}
		if strings.TrimLeft(line, " \t") != strings.TrimLeft(fresh[i], " \t") {
			indent = false
			continue
		}
		delta := len(fresh[i]) - len(line)
		if delta == 0 {
			continue
		}
		sign := 1
		if delta < 0 {
			sign = -1
		}
		if direction != 0 && direction != sign {
			indent = false
			continue
		}
		direction = sign
	}
	return upper || lower || indent
}

// A whitespace join keeps the last source line's terminator, not the first one's.
func joinedOrigin(old, fresh []string, origins []int, used []bool) (int, int, bool) {
	line := -1
	for i, origin := range origins {
		if origin >= 0 {
			continue
		}
		if line >= 0 {
			return 0, 0, false
		}
		line = i
	}
	if line < 0 {
		return 0, 0, false
	}
	first, last := -1, -1
	for i, matched := range used {
		if matched {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
	}
	if first < 0 || first == last {
		return 0, 0, false
	}
	var joined strings.Builder
	for i := first; i <= last; i++ {
		if used[i] {
			return 0, 0, false
		}
		if i > first {
			joined.WriteByte(' ')
		}
		joined.WriteString(strings.TrimSpace(old[i]))
	}
	if joined.String() != strings.TrimSpace(fresh[line]) {
		return 0, 0, false
	}
	return line, last, true
}

// Whole-text resets only match inside the changed region, leaving duplicate lines outside it alone.
func carryEnds(old, fresh *buffer.Buffer, ends []textcodec.EOL, fill textcodec.EOL) []textcodec.EOL {
	oldN, freshN := old.LineCount(), fresh.LineCount()
	limit := min(oldN, freshN) - 1
	head := 0
	for head < limit && old.Line(head) == fresh.Line(head) {
		head++
	}
	tail := 0
	for tail < limit-head && old.Line(oldN-1-tail) == fresh.Line(freshN-1-tail) {
		tail++
	}
	change := buffer.Change{StartLine: head, OldLines: oldN - head - tail, NewLines: freshN - head - tail}
	return spliceEnds(slices.Clone(ends), change, old.Lines(head, oldN-tail), fresh.Lines(head, freshN-tail), fill)
}
