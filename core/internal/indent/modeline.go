package indent

import "strings"

const (
	modelineLines = 5
	modelineScan  = 4 << 10
)

const localVariables = "Local Variables:"

// modeline reads Vim modelines and Emacs file variables from the first and last lines of text.
// Later lines override earlier ones, as in Vim.
func modeline(text string) settings {
	var s settings
	var block localBlock
	for _, line := range edgeLines(text) {
		if block.open {
			s = block.line(line, s)
			continue
		}
		if i := strings.Index(line, localVariables); i >= 0 {
			block = localBlock{open: true, prefix: line[:i], suffix: strings.TrimSpace(line[i+len(localVariables):])}
			continue
		}
		s = vimLine(line, emacsLine(line, s))
	}
	return s
}

func edgeLines(text string) []string {
	if len(text) <= modelineScan {
		lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
		if len(lines) <= 2*modelineLines {
			return lines
		}
		return append(lines[:modelineLines:modelineLines], lines[len(lines)-modelineLines:]...)
	}
	head := strings.Split(text[:modelineScan], "\n")
	head = head[:min(len(head), modelineLines)]
	tail := text[len(text)-modelineScan:]
	if i := strings.IndexByte(tail, '\n'); i >= 0 {
		tail = tail[i+1:]
	}
	lines := strings.Split(strings.TrimSuffix(tail, "\n"), "\n")
	return append(head[:len(head):len(head)], lines[max(0, len(lines)-modelineLines):]...)
}

// vimLine reads "[text]{white}vim: opts" and "[text]{white}vim: set opts:[text]", also spelled vi: and ex:.
func vimLine(line string, s settings) settings {
	for _, opt := range vimOptions(line) {
		name, value, _ := strings.Cut(opt, "=")
		switch name {
		case "et", "expandtab":
			s.style = Space
		case "noet", "noexpandtab":
			s.style = Tab
		case "ts", "tabstop":
			s.tab = cmpWidth(value, s.tab)
		case "sw", "shiftwidth":
			s.indent = cmpWidth(value, s.indent)
		}
	}
	return s
}

func cmpWidth(value string, current int) int {
	if n := width(value); n > 0 {
		return n
	}
	return current
}

func vimOptions(line string) []string {
	for i := range len(line) {
		if i > 0 && line[i-1] != ' ' && line[i-1] != '\t' {
			continue
		}
		n := vimMarker(line[i:], i == 0)
		if n == 0 {
			continue
		}
		return splitVimOptions(line[i+n:])
	}
	return nil
}

// vimMarker answers the length of vi:, ex: or vim[<=>NNN]: at the start of s. Vim skips ex: at the start of a line.
func vimMarker(s string, lineStart bool) int {
	switch {
	case strings.HasPrefix(s, "vi:"):
		return 3
	case strings.HasPrefix(s, "ex:") && !lineStart:
		return 3
	case strings.HasPrefix(s, "vim"), strings.HasPrefix(s, "Vim"):
		j := 3
		if j < len(s) && strings.IndexByte("<=>", s[j]) >= 0 {
			j++
		}
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j < len(s) && s[j] == ':' {
			return j + 1
		}
	}
	return 0
}

func splitVimOptions(rest string) []string {
	rest = strings.TrimLeft(rest, " \t")
	body, isSet := strings.CutPrefix(rest, "set ")
	if !isSet {
		body, isSet = strings.CutPrefix(rest, "se ")
	}
	if !isSet {
		return strings.FieldsFunc(rest, func(r rune) bool { return r == ' ' || r == '\t' || r == ':' })
	}
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '\\':
			i++
		case ':':
			return strings.Fields(strings.ReplaceAll(body[:i], `\:`, ":"))
		}
	}
	return nil
}

// emacsLine reads "-*- mode: c; tab-width: 4; indent-tabs-mode: nil -*-".
func emacsLine(line string, s settings) settings {
	_, rest, ok := strings.Cut(line, "-*-")
	if !ok {
		return s
	}
	vars, _, ok := strings.Cut(rest, "-*-")
	if !ok {
		return s
	}
	for pair := range strings.SplitSeq(vars, ";") {
		s = emacsVar(pair, s)
	}
	return s
}

func emacsVar(pair string, s settings) settings {
	name, value, ok := strings.Cut(pair, ":")
	if !ok {
		return s
	}
	value = strings.TrimSpace(value)
	switch strings.TrimSpace(name) {
	case "indent-tabs-mode":
		s.style = Tab
		if value == "nil" {
			s.style = Space
		}
	case "tab-width":
		s.tab = cmpWidth(value, s.tab)
	}
	return s
}

type localBlock struct {
	open   bool
	prefix string
	suffix string
}

func (b *localBlock) line(line string, s settings) settings {
	body, ok := strings.CutPrefix(line, b.prefix)
	if !ok {
		b.open = false
		return s
	}
	body = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(body), b.suffix))
	if body == "End:" {
		b.open = false
		return s
	}
	return emacsVar(body, s)
}
