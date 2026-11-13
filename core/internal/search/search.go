// Package search runs find and replace over a document mirror. Offsets are UTF-16 units.
package search

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/duxlabs-dev/dux-edit/core/internal/buffer"
)

const DefaultLimit = 10_000

const firstBatch = 1024

var (
	ErrBadRegex = errors.New("invalid regular expression")
	// ErrNoMatch means the text changed and the query no longer matches the range being replaced.
	ErrNoMatch = errors.New("no match at that range")
)

type Query struct {
	Text          string
	Regex         bool
	CaseSensitive bool
	WholeWord     bool
	Limit         int
}

type Matches struct {
	OK bool `json:"ok"`
	// Ranges is flat: start, length, start, length...
	Ranges []int `json:"ranges"`
	Total  int   `json:"total"`
	Capped bool  `json:"capped"`
}

// Replacement is one edit covering every match: replace [Start, End) with Text.
type Replacement struct {
	OK    bool   `json:"ok"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Text  string `json:"text"`
	Count int    `json:"count"`
}

// matcher is a compiled query. A case-insensitive ASCII literal skips the regex engine, whose case folding
// is about 30 times slower than a byte search over a lowered copy.
type matcher struct {
	re     *regexp.Regexp
	folded []byte
}

func compile(q Query) (matcher, error) {
	if !q.Regex && !q.CaseSensitive && q.Text != "" && isASCII(q.Text) {
		return matcher{folded: lowerASCII([]byte(q.Text))}, nil
	}
	pattern := q.Text
	if !q.Regex {
		pattern = regexp.QuoteMeta(pattern)
	}
	flags := "(?m)"
	if !q.CaseSensitive {
		flags = "(?mi)"
	}
	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return matcher{}, fmt.Errorf("%w: %s", ErrBadRegex, strings.TrimPrefix(err.Error(), "error parsing regexp: "))
	}
	return matcher{re: re}, nil
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// lowerASCII lowers b in place. UTF-8 never uses ASCII bytes inside a multibyte rune, so offsets hold.
func lowerASCII(b []byte) []byte {
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return b
}

// matches returns the submatch indexes of the non-empty matches. Whole word is checked here rather than
// with \b, which is ASCII-only in RE2 and would never match a word ending in é.
//
// The regex engine is asked for growing batches until enough (kept) or the text runs out.
// A nil enough takes every match.
func (mt matcher) matches(text string, wholeWord bool, first int, enough func(kept [][]int) bool) [][]int {
	if mt.re == nil {
		return mt.foldedMatches(text, wholeWord, enough)
	}
	batch := first
	if enough == nil {
		batch = -1
	}
	for {
		all := mt.re.FindAllStringSubmatchIndex(text, batch)
		exhausted := batch < 0 || len(all) < batch
		kept := all[:0]
		for _, m := range all {
			if m[0] == m[1] {
				continue
			}
			if wholeWord && !isWordBoundary(text, m[0], m[1]) {
				continue
			}
			kept = append(kept, m)
		}
		if exhausted || enough(kept) {
			return kept
		}
		batch *= 4
	}
}

func (mt matcher) foldedMatches(text string, wholeWord bool, enough func(kept [][]int) bool) [][]int {
	lowered := lowerASCII([]byte(text))
	var kept [][]int
	for at := 0; ; {
		i := bytes.Index(lowered[at:], mt.folded)
		if i < 0 {
			return kept
		}
		start, end := at+i, at+i+len(mt.folded)
		at = end
		if wholeWord && !isWordBoundary(text, start, end) {
			continue
		}
		kept = append(kept, []int{start, end})
		if enough != nil && enough(kept) {
			return kept
		}
	}
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r)
}

func isWordBoundary(text string, start, end int) bool {
	if start > 0 {
		r, _ := utf8.DecodeLastRuneInString(text[:start])
		if isWordRune(r) {
			return false
		}
	}
	if end < len(text) {
		r, _ := utf8.DecodeRuneInString(text[end:])
		if isWordRune(r) {
			return false
		}
	}
	return true
}

// utf16Cursor turns increasing byte offsets into UTF-16 offsets in a single pass over the text.
type utf16Cursor struct {
	text  string
	byteN int
	unitN int
}

func (c *utf16Cursor) at(byteIdx int) int {
	for c.byteN < byteIdx {
		r, size := utf8.DecodeRuneInString(c.text[c.byteN:])
		c.byteN += size
		c.unitN++
		if r >= 0x10000 {
			c.unitN++
		}
	}
	return c.unitN
}

// Find stops at the limit: past it Total is the limit and Capped is set.
func Find(text string, q Query) (Matches, error) {
	mt, err := compile(q)
	if err != nil {
		return Matches{}, err
	}
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	first := firstBatch
	if !q.WholeWord {
		first = limit + 1
	}
	found := mt.matches(text, q.WholeWord, first, func(kept [][]int) bool { return len(kept) > limit })
	shown := min(len(found), limit)
	result := Matches{OK: true, Ranges: make([]int, 0, 2*shown), Total: shown, Capped: len(found) > limit}
	cursor := utf16Cursor{text: text}
	for _, m := range found[:shown] {
		start := cursor.at(m[0])
		result.Ranges = append(result.Ranges, start, cursor.at(m[1])-start)
	}
	return result, nil
}

// ReplaceAll expands $1 and ${name} in replacement when q.Regex is set.
func ReplaceAll(text string, q Query, replacement string) (Replacement, error) {
	mt, err := compile(q)
	if err != nil {
		return Replacement{}, err
	}
	found := mt.matches(text, q.WholeWord, -1, nil)
	if len(found) == 0 {
		return Replacement{OK: true}, nil
	}
	first, last := found[0][0], found[len(found)-1][1]
	var out strings.Builder
	prev := first
	for _, m := range found {
		out.WriteString(text[prev:m[0]])
		out.WriteString(expand(mt, q, replacement, text, m))
		prev = m[1]
	}
	cursor := utf16Cursor{text: text}
	return Replacement{OK: true, Start: cursor.at(first), End: cursor.at(last), Text: out.String(), Count: len(found)}, nil
}

func expand(mt matcher, q Query, replacement, text string, m []int) string {
	if !q.Regex {
		return replacement
	}
	return string(mt.re.ExpandString(nil, replacement, text, m))
}

// Expand returns the text that replaces the single match at [start, start+length).
func Expand(text string, q Query, replacement string, start, length int) (string, error) {
	mt, err := compile(q)
	if err != nil {
		return "", err
	}
	from, okFrom := buffer.ByteIndex(text, start)
	to, okTo := buffer.ByteIndex(text, start+length)
	if !okFrom || !okTo || length <= 0 {
		return "", ErrNoMatch
	}
	reached := func(kept [][]int) bool { return len(kept) > 0 && kept[len(kept)-1][0] >= from }
	for _, m := range mt.matches(text, q.WholeWord, firstBatch, reached) {
		if m[0] > from {
			break
		}
		if m[0] == from && m[1] == to {
			return expand(mt, q, replacement, text, m), nil
		}
	}
	return "", ErrNoMatch
}
