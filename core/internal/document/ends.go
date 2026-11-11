package document

import (
	"slices"

	"github.com/duxlabs-dev/dux-edit/core/internal/buffer"
	"github.com/duxlabs-dev/dux-edit/core/internal/textcodec"
)

// spliceEnds follows a change in a file with mixed line endings. ends[i] is the ending after line i; the
// breaks inside the replaced lines are new and get the dominant style, the rest keep their bytes.
func spliceEnds(ends []textcodec.EOL, c buffer.Change, fill textcodec.EOL) []textcodec.EOL {
	from := min(c.StartLine, len(ends))
	to := min(c.StartLine+c.OldLines-1, len(ends))
	return slices.Replace(ends, from, to, slices.Repeat([]textcodec.EOL{fill}, max(c.NewLines-1, 0))...)
}

// carryEnds keeps the endings of the lines a whole-text replacement left alone at either end.
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
	return spliceEnds(slices.Clone(ends), change, fill)
}
