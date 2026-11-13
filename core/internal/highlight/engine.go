package highlight

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

const resumeState = "__resume"

var errUnfinishedToken = errors.New("token extends past lexer window")

// checkpoint is the lexer state at the first rule boundary of a line: where it is and which stack it holds.
type checkpoint struct {
	col   int32 // bytes into the line
	col16 int32
	stack int32 // interned stack, -1 when no rule boundary falls in the line
}

var noCheckpoint = checkpoint{stack: -1}

func (c checkpoint) ok() bool { return c.stack >= 0 }

// mark is a checkpoint as the capture mutator sees it, in runes from the line start.
type mark struct {
	col   int
	stack int32
}

// incLexer is a copy of a RegexLexer whose rules report where the lexer can be resumed. Only the worker uses it.
type incLexer struct {
	lexer *chroma.RegexLexer
	prose bool

	stacks map[string]int32
	byID   [][]string

	outer     *chroma.LexerState
	resume    []string
	resumeAt  int
	col16     int32
	lastReset int
	starts    []int
	line      int
	nextStart int
	marks     []mark
}

type capture struct {
	inner chroma.Mutator
	il    *incLexer
}

func (c capture) Mutate(s *chroma.LexerState) error {
	if c.inner != nil {
		if err := c.inner.Mutate(s); err != nil {
			return err
		}
	}
	c.il.record(s)
	return nil
}

func newIncLexer(base *chroma.RegexLexer) (*incLexer, error) {
	raw, err := base.Rules()
	if err != nil {
		return nil, err
	}
	il := &incLexer{
		prose:  proseLexers[lexerID(base)],
		stacks: map[string]int32{"root": 0},
		byID:   [][]string{{"root"}},
	}
	wrapped := chroma.Rules{}
	for state, rules := range raw {
		out := make([]chroma.Rule, len(rules))
		for i, r := range rules {
			if _, compileTime := r.Mutator.(chroma.LexerMutator); !compileTime {
				r.Mutator = capture{inner: r.Mutator, il: il}
			}
			out[i] = r
		}
		wrapped[state] = out
	}
	wrapped[resumeState] = []chroma.Rule{{
		Type:    chroma.EmitterFunc(il.emitResume),
		Mutator: chroma.MutatorFunc(il.resumeMutator),
	}}
	cfg := *base.Config()
	l, err := chroma.NewLexer(&cfg, func() chroma.Rules { return wrapped })
	if err != nil {
		return nil, err
	}
	l.SetRegistry(lexers.GlobalLexerRegistry)
	il.lexer = l
	return il, nil
}

func (il *incLexer) intern(stack []string) int32 {
	if len(stack) == 1 && stack[0] == "root" {
		return 0
	}
	key := strings.Join(stack, "\x00")
	if id, ok := il.stacks[key]; ok {
		return id
	}
	id := int32(len(il.byID))
	il.stacks[key] = id
	il.byID = append(il.byID, append([]string(nil), stack...))
	return id
}

func (il *incLexer) record(s *chroma.LexerState) {
	if s != il.outer || s.Pos < il.nextStart || len(s.MutatorContext) != 0 {
		return
	}
	for il.line+1 < len(il.starts) && il.starts[il.line+1] <= s.Pos {
		il.line++
	}
	il.marks[il.line] = mark{col: s.Pos - il.starts[il.line], stack: il.intern(s.Stack)}
	il.nextStart = int(^uint(0) >> 1)
	if il.line+1 < len(il.starts) {
		il.nextStart = il.starts[il.line+1]
	}
}

// resumeMutator installs the saved stack once. chroma sends the lexer back to its start state when a newline
// matches nothing; from then on this state stands in for root.
func (il *incLexer) resumeMutator(s *chroma.LexerState) error {
	switch {
	case il.outer == nil:
		il.outer = s
		s.Pos = il.resumeAt
		s.Stack = append([]string(nil), il.resume...)
	case s.Pos == il.lastReset:
		// root has no rule for this newline either: chroma emits it as an error and moves on.
		s.Groups = []string{string(s.Text[s.Pos])}
		s.Pos++
		s.Stack = []string{"root"}
	default:
		il.lastReset = s.Pos
		s.Stack = []string{"root"}
	}
	return nil
}

func (il *incLexer) emitResume(groups []string, _ *chroma.LexerState) chroma.Iterator {
	if len(groups) == 0 || groups[0] == "" {
		return chroma.Literator()
	}
	return chroma.Literator(chroma.Token{Type: chroma.Error, Value: groups[0]})
}

// lexRun resumes at checkpoint from in lines[0], or at the document start when from is not ok, and hands
// fn each finished line with its first rule boundary; fn returns false to stop.
// window reports an unfinished closing-delimiter rule before its fallback spans can be stored.
func (il *incLexer) lexRun(lines []string, from checkpoint, window bool, fn func(i int, spans []span, m mark) bool) (err error) {
	il.resume, il.resumeAt, il.col16 = []string{"root"}, 0, 0
	if from.ok() {
		il.resume = il.byID[from.stack]
		il.resumeAt = utf8.RuneCountInString(lines[0][:from.col])
		il.col16 = from.col16
	}
	il.outer, il.lastReset, il.line, il.nextStart = nil, -1, 0, 0
	il.starts = il.starts[:0]
	il.marks = il.marks[:0]
	runes := 0
	for _, line := range lines {
		il.starts = append(il.starts, runes)
		il.marks = append(il.marks, mark{stack: -1})
		runes += utf8.RuneCountInString(line) + 1
	}
	// A boundary at the very end belongs to the line after the window, not to the last line.
	il.starts = append(il.starts, runes)
	il.marks = append(il.marks, mark{stack: -1})
	text := strings.Join(lines, "\n") + "\n"

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s: %v", il.lexer.Config().Name, r)
		}
		il.outer = nil
		// A full re-lex grows these to the document; do not keep that between runs.
		if cap(il.starts) > 4096 {
			il.starts, il.marks = nil, nil
		}
	}()
	it, err := il.lexer.Tokenise(&chroma.TokeniseOptions{State: resumeState}, text)
	if err != nil {
		return err
	}
	i := 0
	sp := splitter{prose: il.prose, col: il.col16, line: func(spans []span) bool {
		ok := fn(i, spans, il.marks[i])
		i++
		return ok && i < len(lines)
	}}
	pos := il.resumeAt
	for t := it(); t != chroma.EOF; t = it() {
		if window {
			if il.unfinished(t, pos) {
				return errUnfinishedToken
			}
			pos += utf8.RuneCountInString(t.Value)
		}
		if !sp.push(t) {
			return nil
		}
	}
	return nil
}

func (il *incLexer) unfinished(t chroma.Token, pos int) bool {
	switch RoleOf(t.Type) {
	case RoleString, RoleEscape, RoleRegex, RoleComment:
		return false
	}
	s := il.outer
	if s == nil || pos >= len(s.Text) {
		return false
	}
	suffix := tokenCloser(s.Text[pos:])
	if len(suffix) == 0 {
		return false
	}
	probe := make([]rune, len(s.Text), len(s.Text)+len(suffix))
	copy(probe, s.Text)
	probe = append(probe, suffix...)
	// A closing-delimiter rule can fail at the opener, not at the window's last token.
	for _, rule := range s.Rules[s.State] {
		m, err := rule.Regexp.FindRunesMatchStartingAt(probe, pos)
		if err != nil || m == nil || m.RuneIndex != pos {
			continue
		}
		return pos+m.RuneLength > len(s.Text)
	}
	return false
}

func tokenCloser(text []rune) []rune {
	switch text[0] {
	case '`', '"', '\'':
		return text[:1]
	case '/':
		if len(text) > 1 && text[1] == '*' {
			return []rune("*/")
		}
	case '<':
		if len(text) >= 4 && string(text[:4]) == "<!--" {
			return []rune("-->")
		}
	case 'r':
		i := 1
		for i < len(text) && text[i] == '#' {
			i++
		}
		if i < len(text) && text[i] == '"' {
			return append([]rune{'"'}, text[1:i]...)
		}
	case 'R':
		if len(text) < 2 || text[1] != '"' {
			return nil
		}
		// C++ raw-string delimiters are at most 16 characters.
		for i := 2; i < min(len(text), 19); i++ {
			switch text[i] {
			case '(':
				return append(append([]rune{')'}, text[2:i]...), '"')
			case ' ', '\\', ')', '\n', '\t':
				return nil
			}
		}
	}
	return nil
}

func toCheckpoint(line string, m mark) checkpoint {
	if m.stack < 0 {
		return noCheckpoint
	}
	c := checkpoint{stack: m.stack}
	runes := 0
	for b, r := range line {
		if runes == m.col {
			c.col = int32(b)
			return c
		}
		runes++
		c.col16++
		if r >= 0x10000 {
			c.col16++
		}
	}
	c.col = int32(len(line))
	return c
}
