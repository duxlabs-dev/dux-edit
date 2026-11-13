package backend

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/duxlabs-dev/dux-edit/core/internal/buffer"
	"github.com/duxlabs-dev/dux-edit/core/internal/document"
	"github.com/duxlabs-dev/dux-edit/core/internal/highlight"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

const bracketBudget = 20000

type bracketLine struct {
	text   string
	offset int
	width  int
}

type bracketWindow struct {
	lines   []bracketLine
	first   int
	column  int
	open    rune
	close   rune
	forward bool
}

func bracketPair(ch rune) (rune, rune, bool) {
	switch ch {
	case '(', '[', '{':
		return ch, rune(")]}"[strings.IndexRune("([{", ch)]), true
	case ')', ']', '}':
		return rune("([{"[strings.IndexRune(")]}", ch)]), ch, false
	}
	return 0, 0, false
}

// Keep only lines the scan can reach within its UTF-16 budget. Strings remain immutable snapshots.
func bracketSnapshot(buf *buffer.Buffer, offset, first, last int) bracketWindow {
	line, at, err := buf.Locate(offset)
	if err != nil || line < first || line > last {
		return bracketWindow{}
	}
	text := buf.Line(line)
	start := buf.Offset(line)
	ch, size := utf8.DecodeLastRuneInString(text[:at])
	open, close, forward := bracketPair(ch)
	column := offset - start - size
	if open == 0 {
		ch, _ = utf8.DecodeRuneInString(text[at:])
		open, close, forward = bracketPair(ch)
		column = offset - start
	}
	if open == 0 {
		return bracketWindow{}
	}
	w := bracketWindow{first: line, column: column, open: open, close: close, forward: forward}
	current := bracketLine{text: text, offset: start, width: buf.Width(line)}
	w.lines = append(w.lines, current)
	units, step := column+1, -1
	if forward {
		units, step = current.width-column, 1
	}
	for next := line + step; units < bracketBudget && next >= max(first, 0) && next <= min(last, buf.LineCount()-1); next += step {
		previous := w.lines[len(w.lines)-1]
		width := buf.Width(next)
		start := previous.offset - width - 1
		if forward {
			start = previous.offset + previous.width + 1
		}
		w.lines = append(w.lines, bracketLine{text: buf.Line(next), offset: start, width: width})
		units += width + 1
	}
	if !forward {
		w.first = line - len(w.lines) + 1
		slices.Reverse(w.lines)
	}
	return w
}

func bracketIgnored(spans []highlight.Span, column int) bool {
	i, _ := slices.BinarySearchFunc(spans, column, func(sp highlight.Span, col int) int {
		return sp.Col - col
	})
	if i >= len(spans) || spans[i].Col > column {
		i--
	}
	if i < 0 || column >= spans[i].Col+spans[i].Len {
		return false
	}
	switch spans[i].Role {
	case highlight.RoleString, highlight.RoleEscape, highlight.RoleRegex, highlight.RoleCode, highlight.RoleComment:
		return true
	}
	return false
}

func matchBracket(w bracketWindow, syntax []highlight.Line, budget int) []int {
	if len(w.lines) == 0 || len(syntax) != len(w.lines) {
		return nil
	}
	line, step := 0, 1
	if !w.forward {
		line, step = len(w.lines)-1, -1
	}
	origin := w.lines[line].offset + w.column
	if bracketIgnored(syntax[line].Spans, w.column) {
		return nil
	}
	depth := 0
	for i := line; i >= 0 && i < len(w.lines); i += step {
		text := w.lines[i].text
		column, at := 0, 0
		if !w.forward {
			column, at = w.lines[i].width, len(text)
		}
		if i == line {
			column = w.column
			if !w.forward {
				column++
			}
			at, _ = buffer.ByteIndex(text, column)
		}
		for w.forward && at < len(text) || !w.forward && at > 0 {
			var ch rune
			var size int
			if w.forward {
				ch, size = utf8.DecodeRuneInString(text[at:])
				at += size
			} else {
				ch, size = utf8.DecodeLastRuneInString(text[:at])
				at -= size
			}
			width := 1
			if ch >= 0x10000 {
				width = 2
			}
			if !w.forward {
				column -= width
			}
			budget -= width
			if budget < 0 {
				return nil
			}
			if !bracketIgnored(syntax[i].Spans, column) {
				switch {
				case ch == w.open && w.forward, ch == w.close && !w.forward:
					depth++
				case ch == w.close && w.forward, ch == w.open && !w.forward:
					depth--
					if depth == 0 {
						return []int{origin, w.lines[i].offset + column}
					}
				}
			}
			if w.forward {
				column += width
			}
		}
		budget--
		if budget < 0 {
			return nil
		}
	}
	return nil
}

func (b *Backend) docMatchBracket(ctx context.Context, p map[string]any) any {
	docID := params.StringOpt(p, "docId", "")
	d, found := b.docs.Get(docID)
	if !found {
		return fail(document.CodeUnknownDoc, "")
	}
	ctx, cancel := context.WithTimeout(ctx, awaitEdits)
	defer cancel()
	version := int64(params.IntOpt(p, "version", 0))
	if d.Await(ctx, version) != nil {
		return fail(document.CodeDesync, "")
	}
	var window bracketWindow
	d.Read(func(buf *buffer.Buffer, v int64) {
		if v == version {
			window = bracketSnapshot(buf, params.IntOpt(p, "offset", 0), params.IntOpt(p, "first", 0), params.IntOpt(p, "last", 0))
		}
	})
	var pair []int
	if len(window.lines) > 0 {
		lines, lexVersion, err := b.highlight.Lines(ctx, docID, window.first, window.first+len(window.lines))
		if err != nil {
			return fail(document.CodeDesync, "")
		}
		if lexVersion != version {
			return fail("SUPERSEDED", "")
		}
		pair = matchBracket(window, lines, bracketBudget)
	}
	if d.Version() != version {
		return fail("SUPERSEDED", "")
	}
	return map[string]any{"ok": true, "version": version, "pair": pair}
}
