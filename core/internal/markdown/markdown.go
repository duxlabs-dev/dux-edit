// Package markdown renders a document to the HTML subset Qt rich text understands.
package markdown

import (
	"bytes"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"

	"github.com/duxlabs-dev/dux-edit/core/internal/highlight"
)

const MaxBytes = 1 << 20

type Options struct {
	// Palette maps highlight role names to "#rrggbb"; the UI sends it so Go never knows the theme.
	Palette map[string]string
	// CodeBackground and LinkColor are "#rrggbb", empty to leave the default.
	CodeBackground string
	LinkColor      string
}

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)

// color drops anything that is not a hex color, since it lands inside a style attribute.
func color(c string) string {
	if !colorPattern.MatchString(c) {
		return ""
	}
	return c
}

// Render returns HTML. Task items become <a href="task:LINE"> links, LINE being the 0-based source line.
func Render(source string, opt Options) (string, error) {
	src := []byte(source)
	r := &qtRenderer{opt: opt, lineStarts: lineStarts(src)}
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			renderer.WithNodeRenderers(util.Prioritized(r, 1)),
		),
	)
	var out bytes.Buffer
	if err := md.Convert(src, &out); err != nil {
		return "", err
	}
	return out.String(), nil
}

func lineStarts(src []byte) []int {
	starts := []int{0}
	for i, c := range src {
		if c == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// qtRenderer overrides the nodes whose default HTML Qt cannot show or must not load.
type qtRenderer struct {
	opt        Options
	lineStarts []int
}

func (r *qtRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.renderCode)
	reg.Register(ast.KindCodeBlock, r.renderCode)
	reg.Register(east.KindTaskCheckBox, r.renderTask)
	reg.Register(east.KindStrikethrough, r.renderStrike)
	reg.Register(east.KindTable, r.renderTable)
	reg.Register(ast.KindImage, r.renderImage)
	reg.Register(ast.KindLink, r.renderLink)
	reg.Register(ast.KindAutoLink, r.renderAutoLink)
}

func (r *qtRenderer) lineOf(offset int) int {
	return sort.Search(len(r.lineStarts), func(i int) bool { return r.lineStarts[i] > offset }) - 1
}

func (r *qtRenderer) renderCode(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	var code strings.Builder
	lines := node.Lines()
	for i := range lines.Len() {
		seg := lines.At(i)
		code.Write(seg.Value(source))
	}
	language := ""
	if fenced, ok := node.(*ast.FencedCodeBlock); ok {
		language = string(fenced.Language(source))
	}

	_, _ = w.WriteString(`<pre style="white-space:pre-wrap;`)
	if bg := color(r.opt.CodeBackground); bg != "" {
		_, _ = w.WriteString(" background-color:" + bg + ";")
	}
	_, _ = w.WriteString(`">`)
	r.writeHighlighted(w, language, code.String())
	_, _ = w.WriteString("</pre>\n")
	return ast.WalkSkipChildren, nil
}

func (r *qtRenderer) writeHighlighted(w util.BufWriter, language, code string) {
	var lexer chroma.Lexer
	if language != "" {
		lexer = highlight.Lexer(language)
	}
	if lexer == nil {
		_, _ = w.Write(util.EscapeHTML([]byte(code)))
		return
	}
	tokens, err := chroma.Coalesce(lexer).Tokenise(nil, code)
	if err != nil {
		_, _ = w.Write(util.EscapeHTML([]byte(code)))
		return
	}
	// chroma's EnsureLF adds a newline the source did not have.
	remaining := len(code)
	for token := tokens(); token != chroma.EOF; token = tokens() {
		value := token.Value
		if len(value) > remaining {
			value = value[:remaining]
		}
		remaining -= len(value)
		if value == "" {
			continue
		}
		c := color(r.opt.Palette[highlight.RoleNames[highlight.RoleOf(token.Type)]])
		if c == "" {
			_, _ = w.Write(util.EscapeHTML([]byte(value)))
			continue
		}
		_, _ = w.WriteString(`<span style="color:` + c + `">`)
		_, _ = w.Write(util.EscapeHTML([]byte(value)))
		_, _ = w.WriteString("</span>")
	}
}

func (r *qtRenderer) renderTask(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	line := 0
	if parent := node.Parent(); parent != nil && parent.Lines().Len() > 0 {
		line = r.lineOf(parent.Lines().At(0).Start)
	}
	box := "☐"
	if node.(*east.TaskCheckBox).IsChecked {
		box = "☑"
	}
	style := ""
	if c := color(r.opt.LinkColor); c != "" {
		style = ` style="color:` + c + `; text-decoration:none"`
	}
	_, _ = w.WriteString(`<a href="task:` + strconv.Itoa(line) + `"` + style + `>` + box + "</a> ")
	return ast.WalkContinue, nil
}

func (r *qtRenderer) renderStrike(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("<s>")
	} else {
		_, _ = w.WriteString("</s>")
	}
	return ast.WalkContinue, nil
}

// Qt draws tables without borders unless asked.
func (r *qtRenderer) renderTable(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(`<table border="1" cellspacing="0" cellpadding="4">` + "\n")
	} else {
		_, _ = w.WriteString("</table>\n")
	}
	return ast.WalkContinue, nil
}

func (r *qtRenderer) openLink(w util.BufWriter, dest, title []byte) {
	_, _ = w.WriteString(`<a href="`)
	dest = util.URLEscape(dest, true)
	if !html.IsDangerousURL(dest) {
		_, _ = w.Write(util.EscapeHTML(dest))
	}
	_ = w.WriteByte('"')
	if len(title) > 0 {
		_, _ = w.WriteString(` title="`)
		_, _ = w.Write(util.EscapeHTML(title))
		_ = w.WriteByte('"')
	}
	if c := color(r.opt.LinkColor); c != "" {
		_, _ = w.WriteString(` style="color:` + c + `"`)
	}
	_ = w.WriteByte('>')
}

func (r *qtRenderer) renderLink(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</a>")
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Link)
	r.openLink(w, n.Destination, n.Title)
	return ast.WalkContinue, nil
}

func (r *qtRenderer) renderAutoLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.AutoLink)
	url := n.URL(source)
	label := n.Label(source)
	if n.AutoLinkType == ast.AutoLinkEmail && !bytes.HasPrefix(bytes.ToLower(url), []byte("mailto:")) {
		url = append([]byte("mailto:"), url...)
	}
	r.openLink(w, url, nil)
	_, _ = w.Write(util.EscapeHTML(label))
	_, _ = w.WriteString("</a>")
	return ast.WalkContinue, nil
}

// Remote images become links so the preview never fetches anything.
func (r *qtRenderer) renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Image)
	alt := altText(source, n)
	dest := string(n.Destination)
	lower := strings.ToLower(dest)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		if len(alt) == 0 {
			alt = n.Destination
		}
		r.openLink(w, n.Destination, n.Title)
		_, _ = w.Write(util.EscapeHTML(alt))
		_, _ = w.WriteString("</a>")
		return ast.WalkSkipChildren, nil
	}
	_, _ = w.WriteString(`<img src="`)
	escaped := util.URLEscape(n.Destination, true)
	if !html.IsDangerousURL(escaped) {
		_, _ = w.Write(util.EscapeHTML(escaped))
	}
	_, _ = w.WriteString(`" alt="`)
	_, _ = w.Write(util.EscapeHTML(alt))
	_, _ = w.WriteString(`">`)
	return ast.WalkSkipChildren, nil
}

func altText(source []byte, n ast.Node) []byte {
	var out []byte
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch t := c.(type) {
		case *ast.Text:
			out = append(out, t.Segment.Value(source)...)
		case *ast.String:
			out = append(out, t.Value...)
		default:
			out = append(out, altText(source, c)...)
		}
	}
	return out
}
