package highlight

import (
	"bytes"
	"sort"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

var markdownParser = sync.OnceValue(func() parser.Parser {
	return goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser()
})

const proseBit = 0x80

// mdPainter colors source bytes from goldmark node positions; inner nodes paint over outer ones.
type mdPainter struct {
	src    []byte
	paint  []uint8 // role+1, proseBit for prose; 0 is unpainted
	starts []int
}

func markdownSpans(lines []string) [][]span {
	p := &mdPainter{src: []byte(strings.Join(lines, "\n"))}
	p.paint = make([]uint8, len(p.src))
	p.starts = make([]int, len(lines))
	off := 0
	for i, l := range lines {
		p.starts[i] = off
		off += len(l) + 1
	}
	p.block(markdownParser().Parse(text.NewReader(p.src)))
	return p.spans(lines)
}

func (p *mdPainter) set(a, b int, role Role, prose bool) {
	v := uint8(role) + 1
	if prose {
		v |= proseBit
	}
	for i := max(a, 0); i < min(b, len(p.src)); i++ {
		if p.src[i] != '\n' {
			p.paint[i] = v
		}
	}
}

// line returns the line index holding pos and its byte bounds without the newline.
func (p *mdPainter) line(pos int) (int, int, int) {
	i := sort.Search(len(p.starts), func(i int) bool { return p.starts[i] > pos }) - 1
	return p.lineBounds(max(i, 0))
}

func (p *mdPainter) lineBounds(i int) (int, int, int) {
	end := len(p.src)
	if i+1 < len(p.starts) {
		end = p.starts[i+1] - 1
	}
	return i, p.starts[i], end
}

func (p *mdPainter) block(n ast.Node) {
	switch n := n.(type) {
	case *ast.Heading:
		p.heading(n)
		p.inline(n, RoleHeading, true)
		return
	case *ast.FencedCodeBlock:
		p.fenced(n)
		return
	case *ast.CodeBlock:
		p.segments(n.Lines(), RoleCode)
		return
	case *ast.HTMLBlock:
		p.segments(n.Lines(), RoleTag)
		if n.HasClosure() {
			p.set(n.ClosureLine.Start, n.ClosureLine.Stop, RoleTag, false)
		}
		return
	case *ast.ThematicBreak:
		if n.Pos() >= 0 {
			_, a, b := p.line(n.Pos())
			p.set(a, b, RolePunctuation, false)
		}
		return
	case *ast.Blockquote:
		p.quoteMarkers(n)
	case *ast.ListItem:
		p.listMarker(n)
	}
	if c := n.FirstChild(); c != nil && c.Type() == ast.TypeInline {
		p.inline(n, RoleText, true)
		return
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		p.block(c)
	}
}

func (p *mdPainter) segments(lines *text.Segments, role Role) {
	for i := range lines.Len() {
		seg := lines.At(i)
		p.set(seg.Start, seg.Stop, role, false)
	}
}

func (p *mdPainter) heading(n *ast.Heading) {
	if n.Lines().Len() == 0 {
		if n.Pos() >= 0 {
			_, _, end := p.line(n.Pos())
			p.set(n.Pos(), end, RoleHeading, true)
		}
		return
	}
	first := n.Lines().At(0).Start
	start := first
	for start > 0 && (p.src[start-1] == '#' || p.src[start-1] == ' ' || p.src[start-1] == '\t') {
		start--
	}
	if p.src[start] == '#' {
		_, _, end := p.line(first)
		p.set(start, end, RoleHeading, true)
		return
	}
	last := 0
	for i := range n.Lines().Len() {
		seg := n.Lines().At(i)
		l, _, end := p.line(seg.Start)
		p.set(seg.Start, end, RoleHeading, true)
		last = l
	}
	if last+1 < len(p.starts) {
		_, a, b := p.lineBounds(last + 1)
		p.set(a, b, RoleHeading, false)
	}
}

func (p *mdPainter) fenced(n *ast.FencedCodeBlock) {
	if n.Pos() < 0 {
		return
	}
	open, a, b := p.line(n.Pos())
	p.set(a, b, RoleCode, false)
	closing := open + 1
	if lines := n.Lines(); lines.Len() > 0 {
		l, _, _ := p.line(lines.At(lines.Len() - 1).Start)
		closing = l + 1
	}
	if closing < len(p.starts) {
		_, a, b := p.lineBounds(closing)
		fence := p.src[n.Pos()]
		if t := bytes.TrimLeft(p.src[a:b], " \t"); len(t) > 0 && t[0] == fence {
			p.set(a, b, RoleCode, false)
		}
	}
	var lexer chroma.Lexer
	if lang := n.Language(p.src); len(lang) > 0 {
		lexer = Lexer(string(lang))
	}
	if lexer == nil || !p.lexCode(lexer, n.Lines()) {
		p.segments(n.Lines(), RoleCode)
	}
}

// lexCode paints a fenced block's lines with chroma; code holds the segments back to back.
func (p *mdPainter) lexCode(lexer chroma.Lexer, lines *text.Segments) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	var code []byte
	srcAt := make([]int, 0, 64)
	for i := range lines.Len() {
		seg := lines.At(i)
		for j := seg.Start; j < seg.Stop; j++ {
			srcAt = append(srcAt, j)
		}
		code = append(code, p.src[seg.Start:seg.Stop]...)
	}
	it, err := lexer.Tokenise(nil, string(code))
	if err != nil {
		return false
	}
	prose := proseLexers[lexerID(lexer)]
	pos := 0
	for t := it(); t != chroma.EOF; t = it() {
		role, isProse := RoleOf(t.Type), proseOf(t.Type, t.Value, prose)
		for k := 0; k < len(t.Value) && pos < len(srcAt); k++ {
			if role != RoleText || isProse {
				p.set(srcAt[pos], srcAt[pos]+1, role, isProse)
			}
			pos++
		}
	}
	return true
}

func (p *mdPainter) quoteMarkers(n *ast.Blockquote) {
	first, last := -1, -1
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || c.Type() != ast.TypeBlock {
			return ast.WalkContinue, nil
		}
		for i := range c.Lines().Len() {
			l, _, _ := p.line(c.Lines().At(i).Start)
			if first < 0 || l < first {
				first = l
			}
			last = max(last, l)
		}
		return ast.WalkContinue, nil
	})
	if n.Pos() >= 0 {
		l, _, _ := p.line(n.Pos())
		if first < 0 || l < first {
			first = l
		}
		last = max(last, l)
	}
	for l := max(first, 0); l <= last; l++ {
		_, a, b := p.lineBounds(l)
		for i := a; i < b && (p.src[i] == ' ' || p.src[i] == '\t' || p.src[i] == '>'); i++ {
			if p.src[i] == '>' {
				p.set(i, i+1, RolePunctuation, false)
			}
		}
	}
}

func (p *mdPainter) listMarker(n *ast.ListItem) {
	i := n.Pos()
	if i < 0 || i >= len(p.src) {
		return
	}
	j := i
	for j < len(p.src) && p.src[j] >= '0' && p.src[j] <= '9' {
		j++
	}
	if j < len(p.src) && strings.IndexByte("-+*.)", p.src[j]) >= 0 {
		j++
	}
	p.set(i, j, RolePunctuation, false)
}

func (p *mdPainter) inline(n ast.Node, role Role, prose bool) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Pos() < 0 {
			if _, ok := c.(*ast.Text); !ok {
				p.inline(c, role, prose)
				continue
			}
		}
		switch c := c.(type) {
		case *ast.Text:
			p.set(c.Segment.Start, c.Segment.Stop, role, prose)
		case *ast.CodeSpan:
			p.set(c.Pos(), p.codeSpanEnd(c), RoleCode, false)
		case *ast.Emphasis:
			r := RoleEmphasis
			if c.Level >= 2 {
				r = RoleStrong
			}
			if _, end, ok := p.extent(c); ok {
				p.set(c.Pos(), end+c.Level, r, true)
			}
			p.inline(c, r, true)
		case *east.Strikethrough:
			if _, end, ok := p.extent(c); ok {
				p.set(c.Pos(), end+p.run(c.Pos(), '~'), RoleDeleted, true)
			}
			p.inline(c, RoleDeleted, true)
		case *ast.Link, *ast.Image:
			p.set(c.Pos(), p.linkEnd(c), RoleLink, false)
			p.inline(c, RoleLink, true)
		case *ast.AutoLink:
			p.autoLink(c)
		case *ast.RawHTML:
			p.segments(c.Segments, RoleTag)
		default:
			p.inline(c, role, prose)
		}
	}
}

// extent is the source range covered by the text inside an inline node.
func (p *mdPainter) extent(n ast.Node) (int, int, bool) {
	start, end, ok := 0, 0, false
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		t, isText := c.(*ast.Text)
		if !entering || !isText {
			return ast.WalkContinue, nil
		}
		if !ok || t.Segment.Start < start {
			start = t.Segment.Start
		}
		end = max(end, t.Segment.Stop)
		ok = true
		return ast.WalkContinue, nil
	})
	return start, end, ok
}

func (p *mdPainter) run(i int, c byte) int {
	n := 0
	for i+n < len(p.src) && p.src[i+n] == c {
		n++
	}
	return n
}

func (p *mdPainter) codeSpanEnd(n *ast.CodeSpan) int {
	ticks := p.run(n.Pos(), '`')
	_, end, ok := p.extent(n)
	if !ok {
		end = n.Pos() + ticks
	}
	for end < len(p.src) && p.src[end] != '`' && end-n.Pos() < 1<<16 {
		end++
	}
	return end + p.run(end, '`')
}

func (p *mdPainter) linkEnd(n ast.Node) int {
	end := n.Pos() + 1
	if _, e, ok := p.extent(n); ok {
		end = e
	}
	for end < len(p.src) && p.src[end] != ']' {
		end++
	}
	end++
	if end >= len(p.src) {
		return end
	}
	closer := map[byte]byte{'(': ')', '[': ']'}[p.src[end]]
	if closer == 0 {
		return end
	}
	depth := 0
	for i := end; i < len(p.src) && i-end < 4096; i++ {
		switch p.src[i] {
		case '\\':
			i++
		case p.src[end]:
			depth++
		case closer:
			if depth--; depth == 0 {
				return i + 1
			}
		}
	}
	return end
}

func (p *mdPainter) autoLink(n *ast.AutoLink) {
	label := n.Label(p.src)
	if len(label) == 0 {
		return
	}
	start := bytes.Index(p.src[n.Pos():], label)
	if start < 0 {
		return
	}
	start += n.Pos()
	end := start + len(label)
	if start > 0 && p.src[start-1] == '<' && end < len(p.src) && p.src[end] == '>' {
		start, end = start-1, end+1
	}
	p.set(start, end, RoleLink, false)
}

func (p *mdPainter) spans(lines []string) [][]span {
	out := make([][]span, len(lines))
	for i, line := range lines {
		base := p.starts[i]
		var cur []span
		col := int32(0)
		for b, r := range line {
			v := p.paint[base+b]
			n := int32(1)
			if r >= 0x10000 {
				n = 2
			}
			if v != 0 {
				s := span{col: col, n: n, role: Role(v&^proseBit) - 1, prose: v&proseBit != 0}
				if k := len(cur) - 1; k >= 0 && cur[k].role == s.role && cur[k].prose == s.prose && cur[k].col+cur[k].n == col {
					cur[k].n += n
				} else {
					cur = append(cur, s)
				}
			}
			col += n
		}
		out[i] = dropBlankText(line, cur)
	}
	return out
}

func dropBlankText(line string, spans []span) []span {
	out := spans[:0]
	for _, s := range spans {
		if s.role == RoleText && strings.TrimSpace(slice16(line, s.col, s.n)) == "" {
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// slice16 cuts line by UTF-16 column and length.
func slice16(line string, col, n int32) string {
	start, end, c := -1, len(line), int32(0)
	for b, r := range line {
		if c == col {
			start = b
		}
		if c == col+n {
			end = b
			break
		}
		c++
		if r >= 0x10000 {
			c++
		}
	}
	if start < 0 {
		return ""
	}
	return line[start:end]
}
