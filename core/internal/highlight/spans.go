package highlight

import (
	"slices"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/duxlabs-dev/dux-edit/core/internal/buffer"
)

// span is the stored form of Span; 12 bytes instead of 24 for every token of a resident document.
type span struct {
	col, n int32
	role   Role
	prose  bool
}

func (s span) export() Span {
	return Span{Col: int(s.col), Len: int(s.n), Role: s.role, Prose: s.prose}
}

var proseLexers = map[string]bool{
	"plaintext": true, "markdown": true, "restructuredtext": true, "gemtext": true, "org mode": true, "html": true, "tex": true,
}

func proseOf(t chroma.TokenType, value string, proseLexer bool) bool {
	switch t {
	case chroma.CommentPreproc, chroma.CommentPreprocFile, chroma.CommentHashbang:
		return false
	case chroma.LiteralStringDoc, chroma.LiteralStringHeredoc:
		return true
	case chroma.LiteralString, chroma.LiteralStringDouble, chroma.LiteralStringSingle:
		return strings.ContainsAny(value, " \t\n")
	case chroma.Text, chroma.GenericHeading, chroma.GenericSubheading, chroma.GenericEmph, chroma.GenericStrong, chroma.GenericDeleted:
		return proseLexer
	}
	return t.InCategory(chroma.Comment)
}

// splitter cuts a token stream into per-line spans. Columns are UTF-16 units.
type splitter struct {
	prose bool
	col   int32
	cur   []span
	// line receives each finished line; returning false stops the stream.
	line func(spans []span) bool
}

func (sp *splitter) push(t chroma.Token) bool {
	role := RoleOf(t.Type)
	prose := proseOf(t.Type, t.Value, sp.prose)
	keep := role != RoleText || prose
	v := t.Value
	for {
		part, rest, newline := strings.Cut(v, "\n")
		n := int32(buffer.Len16(part))
		if n > 0 && keep && (role != RoleText || strings.TrimSpace(part) != "") {
			sp.cur = append(sp.cur, span{col: sp.col, n: n, role: role, prose: prose})
		}
		sp.col += n
		if !newline {
			return true
		}
		if !sp.line(merge(sp.cur)) {
			return false
		}
		sp.cur, sp.col, v = nil, 0, rest
	}
}

// merge joins touching spans that look the same. Restarts split tokens at rule boundaries, so merging keeps
// incremental and full results identical.
func merge(spans []span) []span {
	if len(spans) == 0 {
		return nil
	}
	out := spans[:1]
	for _, s := range spans[1:] {
		last := &out[len(out)-1]
		if last.role == s.role && last.prose == s.prose && last.col+last.n == s.col {
			last.n += s.n
			continue
		}
		out = append(out, s)
	}
	return slices.Clip(out)
}

// clip keeps the parts of spans left of col.
func clip(spans []span, col int32) []span {
	out := make([]span, 0, len(spans))
	for _, s := range spans {
		if s.col >= col {
			break
		}
		s.n = min(s.n, col-s.col)
		out = append(out, s)
	}
	return out
}

// encode is the wire form of a line: [col, len, role, ...] without text spans.
func encode(spans []span) []int {
	out := make([]int, 0, len(spans)*3)
	for _, s := range spans {
		if s.role == RoleText {
			continue
		}
		if n := len(out); n >= 3 && out[n-1] == int(s.role) && out[n-3]+out[n-2] == int(s.col) {
			out[n-2] += int(s.n)
			continue
		}
		out = append(out, int(s.col), int(s.n), int(s.role))
	}
	return out
}

// sentHash fingerprints an encoded line; 0 means nothing was sent.
func sentHash(encoded []int) uint64 {
	h := uint64(14695981039346656037)
	for _, v := range encoded {
		h = (h ^ uint64(v)) * 1099511628211
	}
	return max(h, 1)
}
