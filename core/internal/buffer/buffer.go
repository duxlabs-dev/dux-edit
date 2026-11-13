// Package buffer mirrors the text the UI holds in its TextEdit.
// Offsets and columns are UTF-16 code units, as QString counts them. Lines never contain "\n".
package buffer

import (
	"errors"
	"slices"
	"strings"
	"unicode/utf8"
)

var ErrRange = errors.New("edit outside the buffer or inside a surrogate pair")

// Change says lines [StartLine, StartLine+OldLines) were replaced by NewLines lines.
type Change struct {
	StartLine int
	OldLines  int
	NewLines  int
}

type Buffer struct {
	lines []string
	// widths caches Len16 of every line, so locating an offset adds integers instead of decoding text.
	widths []int
	len16  int
}

func New(text string) *Buffer {
	b := &Buffer{lines: strings.Split(text, "\n")}
	b.widths = make([]int, len(b.lines))
	for i, line := range b.lines {
		b.widths[i] = Len16(line)
	}
	b.len16 = Len16(text)
	return b
}

func Len16(s string) int {
	n := 0
	for _, r := range s {
		n++
		if r >= 0x10000 {
			n++
		}
	}
	return n
}

// ByteIndex converts a UTF-16 column to a byte index in s.
func ByteIndex(s string, col16 int) (int, bool) {
	if col16 == 0 {
		return 0, true
	}
	n := 0
	for i, r := range s {
		if n == col16 {
			return i, true
		}
		n++
		if r >= 0x10000 {
			n++
		}
		if n > col16 {
			return 0, false
		}
	}
	return len(s), n == col16
}

func (b *Buffer) Len16() int { return b.len16 }

func (b *Buffer) LineCount() int { return len(b.lines) }

func (b *Buffer) Line(i int) string { return b.lines[i] }

func (b *Buffer) Width(i int) int { return b.widths[i] }

// Lines returns lines [first, last) clamped to the buffer. The slice is a copy.
func (b *Buffer) Lines(first, last int) []string {
	first = max(first, 0)
	last = min(last, len(b.lines))
	if first >= last {
		return nil
	}
	return append([]string(nil), b.lines[first:last]...)
}

func (b *Buffer) Text() string { return strings.Join(b.lines, "\n") }

// Offset is the UTF-16 offset of the start of a line.
func (b *Buffer) Offset(line int) int {
	offset := 0
	for i := 0; i < line && i < len(b.widths); i++ {
		offset += b.widths[i] + 1
	}
	return offset
}

// Locate converts a UTF-16 offset to a line and a byte index inside it.
func (b *Buffer) Locate(offset int) (line, byteIdx int, err error) {
	if offset < 0 {
		return 0, 0, ErrRange
	}
	for i, n := range b.widths {
		if offset <= n {
			idx, ok := ByteIndex(b.lines[i], offset)
			if !ok {
				return 0, 0, ErrRange
			}
			return i, idx, nil
		}
		offset -= n + 1
	}
	return 0, 0, ErrRange
}

// Replace swaps `removed` UTF-16 units at `start` for text, which may hold "\n".
func (b *Buffer) Replace(start, removed int, text string) (Change, error) {
	if removed < 0 || !utf8.ValidString(text) {
		return Change{}, ErrRange
	}
	firstLine, firstIdx, err := b.Locate(start)
	if err != nil {
		return Change{}, err
	}
	lastLine, lastIdx := firstLine, firstIdx
	if removed > 0 {
		if lastLine, lastIdx, err = b.Locate(start + removed); err != nil {
			return Change{}, err
		}
	}
	merged := b.lines[firstLine][:firstIdx] + text + b.lines[lastLine][lastIdx:]
	fresh := strings.Split(merged, "\n")
	change := Change{StartLine: firstLine, OldLines: lastLine - firstLine + 1, NewLines: len(fresh)}
	added := Len16(text)

	widths := make([]int, len(fresh))
	switch {
	case firstLine == lastLine && len(fresh) == 1:
		// Recounting the merged line would decode all of it on every keystroke in a long line.
		widths[0] = b.widths[firstLine] - removed + added
	default:
		for i, line := range fresh {
			widths[i] = Len16(line)
		}
	}
	b.lines = slices.Replace(b.lines, firstLine, lastLine+1, fresh...)
	b.widths = slices.Replace(b.widths, firstLine, lastLine+1, widths...)
	b.len16 += added - removed
	return change, nil
}
