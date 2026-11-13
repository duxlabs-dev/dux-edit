package spell

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/clipperhouse/uax29/v2/words"

	"github.com/duxlabs-dev/dux-edit/core/internal/buffer"
	"github.com/duxlabs-dev/dux-edit/core/internal/highlight"
)

// word is a candidate for checking. Col and Len are UTF-16 units in its line; key is what the cache stores.
type word struct {
	col, len int
	key      string
}

// masks cover text that is not prose even inside a comment. A pattern with a group masks only the group.
var masks = []struct {
	re       *regexp.Regexp
	triggers []string
}{
	{regexp.MustCompile("`[^`]*`"), []string{"`"}},
	{regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\s<>"'` + "`" + `]+`), []string{"://"}},
	{regexp.MustCompile(`\bwww\.[^\s<>"'` + "`" + `]+`), []string{"www."}},
	{regexp.MustCompile(`[\w.%+-]+@[\w-]+(?:\.[\w-]+)+`), []string{"@"}},
	{regexp.MustCompile(`(?:^|[^\w.~/])((?:~|\.\.?)/\S*|/[\w.-]+/\S*)`), []string{"/"}},
	{regexp.MustCompile(`(?:^|\s)(--[A-Za-z0-9][\w-]*)`), []string{"--"}},
	{regexp.MustCompile(`\$\{?[A-Za-z_]\w*\}?`), []string{"$"}},
	{regexp.MustCompile(`\b0[xX][0-9a-fA-F]+\b`), []string{"0x", "0X"}},
	{regexp.MustCompile(`#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{3,4})\b`), []string{"#"}},
}

// mask blanks masked text byte for byte, so offsets into the result are offsets into s.
func mask(s string) string {
	var out []byte
	for _, mk := range masks {
		if !slices.ContainsFunc(mk.triggers, func(t string) bool { return strings.Contains(s, t) }) {
			continue
		}
		for _, m := range mk.re.FindAllStringSubmatchIndex(s, -1) {
			start, end := m[0], m[1]
			if len(m) > 2 && m[2] >= 0 {
				start, end = m[2], m[3]
			}
			if out == nil {
				out = []byte(s)
			}
			for i := start; i < end; i++ {
				out[i] = ' '
			}
		}
	}
	if out == nil {
		return s
	}
	return string(out)
}

// checkable drops what is not a dictionary word: single letters, digits, snake_case, ALLCAPS and camelCase.
func checkable(w string) bool {
	if utf8.RuneCountInString(w) < 2 {
		return false
	}
	letter := false
	for i, r := range w {
		switch {
		case unicode.IsDigit(r) || r == '_':
			return false
		case unicode.IsUpper(r) && i > 0:
			return false
		case unicode.IsLetter(r):
			letter = true
		}
	}
	return letter
}

// proseRanges returns the UTF-16 [start, end) pairs of a line the spellchecker reads.
func proseRanges(text string, line highlight.Line, checkStrings bool) [][2]int {
	if line.Plain {
		return [][2]int{{0, buffer.Len16(text)}}
	}
	var out [][2]int
	for _, span := range line.Spans {
		if !span.Prose || (!checkStrings && span.Role == highlight.RoleString) {
			continue
		}
		if n := len(out); n > 0 && out[n-1][1] == span.Col {
			out[n-1][1] += span.Len
			continue
		}
		out = append(out, [2]int{span.Col, span.Col + span.Len})
	}
	return out
}

// lineWords lists the checkable words of one line, skipping the one that touches cursorCol (-1 for none).
func lineWords(text string, line highlight.Line, checkStrings bool, cursorCol int) []word {
	var out []word
	for _, r := range proseRanges(text, line, checkStrings) {
		from, okFrom := buffer.ByteIndex(text, r[0])
		to, okTo := buffer.ByteIndex(text, r[1])
		if !okFrom || !okTo || from >= to {
			continue
		}
		segment := text[from:to]
		col, at := r[0], 0
		iter := words.FromString(mask(segment))
		for iter.Next() {
			start := iter.Start()
			col += buffer.Len16(segment[at:start])
			at = start
			// The masked value: a masked span reads as spaces, never as the text under it.
			w := iter.Value()
			length := buffer.Len16(w)
			if checkable(w) && (cursorCol < col || cursorCol > col+length) {
				out = append(out, word{col: col, len: length, key: strings.ReplaceAll(w, "’", "'")})
			}
		}
	}
	return out
}
