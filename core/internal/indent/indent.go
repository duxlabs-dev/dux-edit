// Package indent reads the indentation a file asks for from .editorconfig and from Vim or Emacs modelines.
// Only the style and width are read: the editor never rewrites text on its own.
package indent

import (
	"cmp"
	"strconv"
	"strings"
)

const (
	Space = "space"
	Tab   = "tab"
)

const maxWidth = 32

type Hints struct {
	Style string
	Width int
}

type settings struct {
	style  string
	indent int
	tab    int
}

func (s settings) over(base settings) settings {
	return settings{
		style:  cmp.Or(s.style, base.style),
		indent: cmp.Or(s.indent, base.indent),
		tab:    cmp.Or(s.tab, base.tab),
	}
}

func (s settings) hints() Hints {
	if s.style == Tab {
		return Hints{Style: s.style, Width: cmp.Or(s.tab, s.indent)}
	}
	return Hints{Style: s.style, Width: cmp.Or(s.indent, s.tab)}
}

// Detect resolves the .editorconfig sections that match path, then lets a modeline in text override them.
func Detect(path, text string) Hints {
	return modeline(text).over(editorConfig(path)).hints()
}

func width(value string) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 1 || n > maxWidth {
		return 0
	}
	return n
}
