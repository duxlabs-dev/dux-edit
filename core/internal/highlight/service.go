// Package highlight lexes open documents with chroma on one worker goroutine and publishes role spans.
package highlight

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/alecthomas/chroma/v2"
	"github.com/duxlabs-dev/dux-edit/core/internal/buffer"
	"github.com/duxlabs-dev/dux-edit/core/internal/document"
)

const (
	TierExact = "exact"
	TierOff   = "off"

	MaxBytes    = 2 << 20
	MaxLines    = 50_000
	MaxLineLen  = 10_000
	TopicEvents = "highlight"
)

const (
	ReasonSize        = "size"
	ReasonLines       = "lines"
	ReasonLongLine    = "longLine"
	ReasonUnsupported = "unsupported"
)

const (
	// Lexers that cannot resume are re-lexed whole on every change, so the cap is what one parse may cost
	// per keystroke. Markdown re-parses only the blocks around an edit, but one that reaches past them
	// costs a whole parse.
	wholeMaxBytes    = 64 << 10
	markdownMaxBytes = 64 << 10
	// A save re-lexes the whole document to catch lookahead the checkpoints miss. Past this size that costs
	// seconds per autosave, so a larger file keeps the gap until reopened.
	saveRelexBytes = 256 << 10
	// restartBack lines of context before an edit cover lookahead that reaches into the next lines.
	restartBack = 8
	// windowMargin lines past the target are lexed but not kept; chroma sees what follows the target.
	windowMargin = 512
	firstChunk   = 512
	maxChunk     = 16384
	// A view more than specGap lines below the lexed prefix is first lexed alone, starting specBack lines
	// above it, and shown until the exact lex arrives.
	specGap  = 1024
	specBack = 128
	specMax  = 1024
)

var ErrUnknownDocument = errors.New("unknown document")

// Span is one token inside a line. Col and Len are UTF-16 units.
type Span struct {
	Col  int
	Len  int
	Role Role
	// Prose marks text a spellchecker should read: comments, docstrings, prose-lexer text, spaced string literals.
	Prose bool
}

// Line holds the spans of one line. Plain means no lexer applies and the whole line is prose.
type Line struct {
	Spans []Span
	Plain bool
}

type State struct {
	DocID       string `json:"docId"`
	Lexer       string `json:"lexer"`
	LexerSource string `json:"lexerSource"`
	Tier        string `json:"tier"`
	Reason      string `json:"reason"`
}

type LexerInfo struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}

// textDoc is what the service reads from a document.Doc; tests use a fake.
type textDoc interface {
	Read(fn func(b *buffer.Buffer, version int64))
	Path() string
}

type engine uint8

const (
	engPlain engine = iota
	engIncremental
	engWhole
	engMarkdown
)

// lineMeta feeds the tier caps. bytes < 0 is not measured yet.
type lineMeta struct {
	bytes int32
	long  bool
}

var unknownMeta = lineMeta{bytes: -1}

type lineState struct {
	spans []span
	ck    checkpoint
	sent  uint64
	// dirty lines were edited since they were lexed. A seam is the first line of an older lex that a run
	// stopped short of; a run may converge on it but not past it.
	dirty bool
	seam  bool
}

var editedLine = lineState{ck: noCheckpoint, dirty: true}

type specLines struct {
	first, last int
	spans       [][]span
}

func (sp specLines) covers(first, last int) bool { return sp.first <= first && last <= sp.last }

func (sp specLines) line(i int) ([]span, bool) {
	if i < sp.first || i-sp.first >= len(sp.spans) {
		return nil, false
	}
	return sp.spans[i-sp.first], true
}

type linesReply struct {
	lines   []Line
	version int64
	err     error
}

type waiter struct {
	first, last int
	reply       chan linesReply
}

type docState struct {
	id  string
	doc textDoc
	// gen moves on every listener call; a job compares it at every line and drops stale work.
	gen atomic.Int64

	version int64
	// fresh asks settle to rebuild everything from the mirror.
	fresh     bool
	blocked   bool
	meta      []lineMeta
	unknownLo int
	unknownHi int
	bytes     int64
	longLines int
	override  string
	force     bool
	redetect  bool
	lexer     chroma.Lexer
	source    string
	kind      engine
	il        *incLexer
	md        *mdDoc
	spec      specLines
	failed    bool
	state     State
	lines     []lineState
	lexedTo   int
	pending   int
	full      bool
	fullFrom  int
	viewFirst int
	viewLast  int
	viewDirty bool
	waiters   []*waiter
}

type event struct {
	topic string
	data  any
}

type Service struct {
	publish document.Publisher
	// pubMu keeps events in order when the worker and a handler flush at once.
	pubMu sync.Mutex

	mu      sync.Mutex
	docs    map[string]*docState
	focus   string
	closed  bool
	signals atomic.Int64
	outbox  []event

	wake  chan struct{}
	stop  chan struct{}
	done  chan struct{}
	start sync.Once

	// Worker only.
	cache map[string]*incLexer
}

func New(_ *document.Store, publish document.Publisher) *Service {
	return &Service{
		publish: publish,
		docs:    make(map[string]*docState),
		wake:    make(chan struct{}, 1),
		stop:    make(chan struct{}),
	}
}

func (s *Service) DocOpened(d *document.Doc) { s.opened(d.ID, d) }

func (s *Service) DocChanged(d *document.Doc, change buffer.Change) { s.changed(d.ID, change) }

func (s *Service) DocReset(d *document.Doc) { s.reset(d.ID) }

func (s *Service) DocSaved(d *document.Doc) { s.saved(d.ID) }

func (s *Service) DocClosed(docID string) {
	s.mu.Lock()
	ds := s.docs[docID]
	delete(s.docs, docID)
	if s.focus == docID {
		s.focus = ""
	}
	if ds != nil {
		ds.gen.Add(1)
		ds.failWaiters(ErrUnknownDocument)
	}
	s.mu.Unlock()
	s.signal()
}

func (s *Service) opened(id string, doc textDoc) {
	ds := &docState{id: id, doc: doc, fresh: true, redetect: true, viewLast: -1}
	s.mu.Lock()
	if old := s.docs[id]; old != nil {
		old.gen.Add(1)
		old.failWaiters(ErrUnknownDocument)
	}
	s.docs[id] = ds
	s.mu.Unlock()
	s.signal()
}

func (s *Service) changed(id string, ch buffer.Change) {
	s.mu.Lock()
	ds := s.docs[id]
	if ds == nil {
		s.mu.Unlock()
		return
	}
	ds.gen.Add(1)
	ds.version++
	ds.blocked = false
	// A save backstop is moot once the text moves on, and retrying it per keystroke would keep a core busy.
	ds.full, ds.fullFrom = false, 0
	ds.spec = specLines{}
	ds.applyChange(ch)
	s.mu.Unlock()
	s.signal()
}

func (s *Service) reset(id string) {
	s.mu.Lock()
	if ds := s.docs[id]; ds != nil {
		ds.gen.Add(1)
		ds.fresh, ds.redetect, ds.blocked, ds.full = true, true, false, false
	}
	s.mu.Unlock()
	s.signal()
}

func (s *Service) saved(id string) {
	s.mu.Lock()
	if ds := s.docs[id]; ds != nil {
		ds.redetect = true
		ds.full = ds.full || ds.bytes+int64(len(ds.meta)) <= saveRelexBytes
	}
	s.mu.Unlock()
	s.signal()
}

// View sets the line range [first, last] the UI shows; spans for it are published as they are ready.
// Lines that enter the range are sent again: the UI keeps spans only around its viewport.
func (s *Service) View(docID string, first, last int) {
	s.mu.Lock()
	ds := s.docs[docID]
	if ds == nil {
		s.mu.Unlock()
		return
	}
	first = max(first, 0)
	for i := first; i <= last && i < len(ds.lines); i++ {
		if i < ds.viewFirst || i > ds.viewLast {
			ds.lines[i].sent = 0
		}
	}
	ds.viewFirst, ds.viewLast, ds.viewDirty = first, last, true
	s.focus = docID
	s.mu.Unlock()
	s.signal()
}

// SetLexer overrides detection; an empty name returns to automatic. force lexes past the size caps.
func (s *Service) SetLexer(docID, lexer string, force bool) State {
	s.mu.Lock()
	defer s.signal()
	defer s.flush()
	defer s.mu.Unlock()
	ds := s.docs[docID]
	if ds == nil {
		return unknownState(docID)
	}
	if Lexer(lexer) == nil {
		lexer = ""
	}
	ds.override, ds.force, ds.redetect = lexer, force, true
	s.settle(ds)
	return ds.currentState()
}

func (s *Service) State(docID string) State {
	s.mu.Lock()
	defer s.flush()
	defer s.mu.Unlock()
	ds := s.docs[docID]
	if ds == nil {
		return unknownState(docID)
	}
	s.settle(ds)
	return ds.currentState()
}

func (s *Service) Lexers() []LexerInfo { return lexerInfos() }

// Lines returns exact spans for lines [first, last) at the document's current version, lexing them if needed.
// internal/spell calls this to find prose. A document that is off by the caps returns nil lines.
func (s *Service) Lines(ctx context.Context, docID string, first, last int) ([]Line, int64, error) {
	w := &waiter{first: max(first, 0), last: last, reply: make(chan linesReply, 1)}
	s.mu.Lock()
	ds := s.docs[docID]
	if ds == nil || s.closed {
		s.mu.Unlock()
		return nil, 0, ErrUnknownDocument
	}
	ds.waiters = append(ds.waiters, w)
	s.mu.Unlock()
	s.signal()

	select {
	case r := <-w.reply:
		return r.lines, r.version, r.err
	case <-ctx.Done():
		s.mu.Lock()
		ds.waiters = slices.DeleteFunc(ds.waiters, func(o *waiter) bool { return o == w })
		s.mu.Unlock()
		return nil, 0, ctx.Err()
	}
}

func (s *Service) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	for _, ds := range s.docs {
		ds.gen.Add(1)
		ds.failWaiters(ErrUnknownDocument)
	}
	s.mu.Unlock()
	s.signals.Add(1)
	s.start.Do(func() {})
	close(s.stop)
	if s.done != nil {
		<-s.done
	}
}

func (s *Service) signal() {
	s.signals.Add(1)
	s.start.Do(func() {
		s.done = make(chan struct{})
		go s.loop()
	})
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) loop() {
	defer close(s.done)
	for {
		select {
		case <-s.stop:
			return
		case <-s.wake:
		}
		for {
			select {
			case <-s.stop:
				return
			default:
			}
			ds := s.next()
			if ds == nil {
				break
			}
			s.work(ds)
		}
	}
}

func (s *Service) next() *docState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.docs) == 0 {
		s.cache = nil
		return nil
	}
	if ds := s.docs[s.focus]; ds != nil && s.needsWork(ds) {
		return ds
	}
	for _, ds := range s.docs {
		if s.needsWork(ds) {
			return ds
		}
	}
	return nil
}

func (s *Service) needsWork(ds *docState) bool {
	switch {
	case ds.blocked:
		return false
	case ds.redetect, ds.fresh, ds.unknownLo < ds.unknownHi, ds.viewDirty, len(ds.waiters) > 0:
		return true
	case ds.state.Tier != TierExact:
		return false
	case ds.kind == engIncremental:
		return ds.pending < ds.target() || ds.full
	}
	return ds.pending < len(ds.lines) && ds.target() > 0
}

// target is how many lines from the top the view and the waiting Lines calls need.
func (ds *docState) target() int {
	t := ds.viewLast + 1
	for _, w := range ds.waiters {
		t = max(t, w.last)
	}
	return min(t, len(ds.lines))
}

func (s *Service) work(ds *docState) {
	s.mu.Lock()
	if !s.workLocked(ds) {
		s.mu.Unlock()
		s.flush()
		return
	}
	gen := ds.gen.Load()
	s.mu.Unlock()
	s.flush()
	s.mu.Lock()
	defer s.mu.Unlock()
	if ds.gen.Load() == gen {
		s.answer(ds)
	}
}

func (s *Service) workLocked(ds *docState) bool {
	if !s.settle(ds) {
		return false
	}
	if ds.state.Tier == TierExact {
		ok := true
		switch ds.kind {
		case engIncremental:
			ok = s.workIncremental(ds)
		case engWhole, engMarkdown:
			ok = s.workWhole(ds)
		}
		if !ok {
			return false
		}
	}
	s.publishView(ds)
	return true
}

func (s *Service) emit(topic string, data any) { s.outbox = append(s.outbox, event{topic, data}) }

// flush publishes the queued events without s.mu held, so a slow socket write never stalls edits or requests.
func (s *Service) flush() {
	s.pubMu.Lock()
	defer s.pubMu.Unlock()
	s.mu.Lock()
	events := s.outbox
	s.outbox = nil
	s.mu.Unlock()
	for _, e := range events {
		s.publish(e.topic, e.data)
	}
}

// settle brings detection, line metrics and the tier up to date and publishes a state change. It returns
// false while the mirror is ahead of the listener calls that describe it; the next call wakes the worker.
func (s *Service) settle(ds *docState) bool {
	if ds.redetect {
		s.detectLexer(ds)
	}
	lag := false
	ds.doc.Read(func(b *buffer.Buffer, version int64) {
		switch {
		case version > ds.version && !ds.fresh:
			lag = true
			return
		case ds.fresh, version != ds.version, b.LineCount() != len(ds.meta):
			ds.version, ds.fresh = version, false
			ds.meta = slices.Repeat([]lineMeta{unknownMeta}, b.LineCount())
			ds.unknownLo, ds.unknownHi, ds.bytes, ds.longLines = 0, b.LineCount(), 0, 0
			ds.resetLines()
		}
		ds.measure(b)
	})
	if lag {
		ds.blocked = true
		return false
	}
	s.retier(ds)
	return true
}

func (s *Service) detectLexer(ds *docState) {
	ds.redetect = false
	var head strings.Builder
	ds.doc.Read(func(b *buffer.Buffer, _ int64) {
		for i := 0; i < b.LineCount() && head.Len() < headBytes; i++ {
			if i > 0 {
				head.WriteByte('\n')
			}
			line := b.Line(i)
			head.WriteString(line[:min(len(line), headBytes-head.Len())])
		}
	})
	lexer, source := Lexer(ds.override), SourceOverride
	if lexer == nil {
		lexer, source = detect(ds.doc.Path(), head.String())
	}
	ds.source = source
	if ds.lexer != nil && lexerID(ds.lexer) == lexerID(lexer) {
		return
	}
	ds.full = ds.lexer != nil
	ds.lexer, ds.kind, ds.il, ds.failed = lexer, kindOf(lexer), nil, false
	ds.resetLines()
}

func kindOf(l chroma.Lexer) engine {
	switch lexerID(l) {
	case "plaintext", "fallback":
		return engPlain
	case "markdown":
		return engMarkdown
	}
	if _, ok := l.(*chroma.RegexLexer); ok {
		return engIncremental
	}
	return engWhole
}

// resetLines drops lexed state; the bump stops a job that is lexing it.
func (ds *docState) resetLines() {
	ds.gen.Add(1)
	ds.lines, ds.lexedTo, ds.pending, ds.fullFrom = nil, 0, 0, 0
	ds.md, ds.spec = nil, specLines{}
}

// measure fills the metrics of lines changed since the last call.
func (ds *docState) measure(b *buffer.Buffer) {
	for i := ds.unknownLo; i < min(ds.unknownHi, len(ds.meta)); i++ {
		ds.forget(ds.meta[i])
		m := lineMeta{bytes: int32(len(b.Line(i))), long: b.Width(i) > MaxLineLen}
		ds.meta[i] = m
		ds.bytes += int64(m.bytes)
		if m.long {
			ds.longLines++
		}
	}
	ds.unknownLo, ds.unknownHi = 0, 0
}

func (ds *docState) forget(m lineMeta) {
	if m.bytes < 0 {
		return
	}
	ds.bytes -= int64(m.bytes)
	if m.long {
		ds.longLines--
	}
}

func (ds *docState) tier() (string, string) {
	n := len(ds.meta)
	size := ds.bytes + int64(max(n-1, 0))
	switch {
	case ds.kind == engPlain:
		return TierOff, ""
	case ds.failed:
		return TierOff, ReasonUnsupported
	case ds.force:
		return TierExact, ""
	case size > MaxBytes:
		return TierOff, ReasonSize
	case n > MaxLines:
		return TierOff, ReasonLines
	case ds.longLines > 0:
		return TierOff, ReasonLongLine
	case ds.kind == engWhole && size > wholeMaxBytes:
		return TierOff, ReasonUnsupported
	case ds.kind == engMarkdown && size > markdownMaxBytes:
		return TierOff, ReasonSize
	}
	return TierExact, ""
}

// withinCaps says whether a forced document is small enough for the save backstop.
func (ds *docState) withinCaps() bool {
	return ds.bytes+int64(len(ds.meta)) <= MaxBytes && len(ds.meta) <= MaxLines && ds.longLines == 0
}

func (s *Service) retier(ds *docState) {
	st := ds.currentState()
	st.Tier, st.Reason = ds.tier()
	if st.Tier != TierExact {
		ds.resetLines()
	} else if ds.lines == nil {
		ds.lines = slices.Repeat([]lineState{editedLine}, len(ds.meta))
	}
	if st == ds.state {
		return
	}
	ds.state = st
	s.emit(TopicEvents, stateEvent{Type: "state", State: st})
}

type stateEvent struct {
	Type string `json:"type"`
	State
}

func (ds *docState) currentState() State {
	st := ds.state
	st.DocID = ds.id
	if ds.lexer != nil {
		st.Lexer, st.LexerSource = lexerID(ds.lexer), ds.source
	}
	if st.Tier == "" {
		st.Tier = TierOff
	}
	return st
}

func unknownState(docID string) State {
	return State{DocID: docID, Lexer: "plaintext", LexerSource: SourceNone, Tier: TierOff}
}

// applyChange shifts per-line state the way buffer.Replace moved the lines.
func (ds *docState) applyChange(ch buffer.Change) {
	if ds.fresh {
		return
	}
	start, old, n := ch.StartLine, ch.OldLines, ch.NewLines
	if start < 0 || old < 1 || n < 1 || start+old > len(ds.meta) {
		ds.fresh = true
		return
	}
	for _, m := range ds.meta[start : start+old] {
		ds.forget(m)
	}
	lines := len(ds.meta)
	ds.meta = splice(ds.meta, start, old, n, unknownMeta)
	ds.unknownLo, ds.unknownHi = shiftRange(ds.unknownLo, ds.unknownHi, ch)

	if len(ds.lines) != lines {
		ds.resetLines()
		return
	}
	ds.lines = splice(ds.lines, start, old, n, editedLine)
	switch {
	case start >= ds.lexedTo:
		// Lines past the lexed prefix can still change the last lexed ones through lookahead.
		ds.pending = min(ds.pending, max(ds.lexedTo-1, 0))
	case start+old <= ds.lexedTo:
		ds.lexedTo += n - old
		ds.pending = min(ds.pending, start)
	default:
		ds.lexedTo = start
		ds.pending = min(ds.pending, start)
	}
}

// shiftRange moves the half-open line range [lo, hi) through a change and adds the new lines to it.
func shiftRange(lo, hi int, ch buffer.Change) (int, int) {
	start, end, delta := ch.StartLine, ch.StartLine+ch.NewLines, ch.NewLines-ch.OldLines
	if lo >= hi {
		return start, end
	}
	move := func(p int) int {
		switch {
		case p <= start:
			return p
		case p >= ch.StartLine+ch.OldLines:
			return p + delta
		}
		return end
	}
	return min(move(lo), start), max(move(hi), end)
}

func splice[T any](s []T, start, old, n int, fill T) []T {
	switch delta := n - old; {
	case delta > 0:
		s = slices.Insert(s, start+old, make([]T, delta)...)
	case delta < 0:
		s = slices.Delete(s, start+n, start+old)
	}
	for i := start; i < start+n; i++ {
		s[i] = fill
	}
	return s
}

// snapshot copies lines [from, to) of the mirror if it is still at the version the state describes.
func (s *Service) snapshot(ds *docState, from, to int) ([]string, bool) {
	var lines []string
	ok := false
	ds.doc.Read(func(b *buffer.Buffer, version int64) {
		if version != ds.version || b.LineCount() != len(ds.lines) {
			return
		}
		lines, ok = b.Lines(from, to), true
	})
	if !ok {
		ds.blocked = true
	}
	return lines, ok
}

func (s *Service) incLexer(ds *docState) *incLexer {
	if ds.il != nil {
		return ds.il
	}
	id := lexerID(ds.lexer)
	if il := s.cache[id]; il != nil {
		ds.il = il
		return il
	}
	il, err := newIncLexer(ds.lexer.(*chroma.RegexLexer))
	if err != nil {
		return nil
	}
	if s.cache == nil {
		s.cache = make(map[string]*incLexer)
	}
	s.cache[id], ds.il = il, il
	return il
}

// workIncremental lexes up to the target, then runs a pending full re-lex. It returns false when the job was
// dropped; the state change that dropped it has already woken the worker.
func (s *Service) workIncremental(ds *docState) bool {
	if s.incLexer(ds) == nil {
		ds.failed = true
		s.retier(ds)
		return true
	}
	first, last, far := ds.farView()
	switch {
	case !far:
		ds.spec = specLines{}
	case !ds.spec.covers(first, last):
		if !s.lexSpec(ds, first, last) {
			return false
		}
		s.publishView(ds)
	}
	for chunk := firstChunk; ds.pending < ds.target(); chunk = min(chunk*2, maxChunk) {
		signals := s.signals.Load()
		if !s.lexRange(ds, min(ds.target(), ds.pending+chunk)) {
			return false
		}
		if s.signals.Load() != signals {
			return true
		}
	}
	if !ds.full {
		return true
	}
	ds.full = false
	if ds.force && !ds.withinCaps() {
		ds.fullFrom = 0
		return true
	}
	return s.lexFull(ds)
}

func (ds *docState) farView() (int, int, bool) {
	first, last := ds.viewFirst, min(ds.viewLast+1, len(ds.lines), ds.viewFirst+specMax)
	return first, last, first < last && first-ds.pending > specGap
}

// lexSpec lexes lines [first, last) as if the lexer were in its root state. Called with s.mu held; unlocks while lexing.
func (s *Service) lexSpec(ds *docState, first, last int) bool {
	from := first - specBack
	text, ok := s.snapshot(ds, from, min(len(ds.lines), last+windowMargin))
	if !ok {
		return false
	}
	gen, il := ds.gen.Load(), ds.il
	s.mu.Unlock()
	s.flush()

	out := make([][]span, 0, last-first)
	_ = il.lexRun(text, noCheckpoint, func(i int, spans []span, _ mark) bool {
		if ds.gen.Load() != gen {
			return false
		}
		if from+i >= first {
			out = append(out, spans)
		}
		return from+i+1 < last
	})

	s.mu.Lock()
	if ds.gen.Load() != gen || s.docs[ds.id] != ds {
		return false
	}
	ds.spec = specLines{first: first, last: last, spans: out}
	return true
}

type lexed struct {
	spans []span
	ck    checkpoint
}

// lexRange re-lexes from a verified checkpoint before ds.pending until the lexer state matches what it was
// before the edit, until line target-1 is done, or until another request arrives. Called with s.mu held.
func (s *Service) lexRange(ds *docState, target int) bool {
	c, from := 0, noCheckpoint
	for i := min(ds.pending-restartBack, ds.pending-1); i > 0; i-- {
		if ds.lines[i].ck.ok() {
			c, from = i, ds.lines[i].ck
			break
		}
	}
	end := min(len(ds.lines), target+windowMargin)
	text, ok := s.snapshot(ds, c, end)
	if !ok {
		return false
	}
	old := make([]lineState, target-c)
	copy(old, ds.lines[c:target])
	pending, lexedTo, gen, signals, il := ds.pending, ds.lexedTo, ds.gen.Load(), s.signals.Load(), ds.il
	s.mu.Unlock()
	s.flush()

	var out []lexed
	converged := -1
	err := il.lexRun(text, from, func(i int, spans []span, m mark) bool {
		if ds.gen.Load() != gen {
			return false
		}
		line := c + i
		ck := toCheckpoint(text[i], m)
		if i == 0 && from.ok() {
			spans, ck = merge(append(clip(old[0].spans, from.col16), spans...)), from
		}
		out = append(out, lexed{spans: spans, ck: ck})
		if line >= pending && line < lexedTo && !old[i].dirty && old[i].ck.ok() && old[i].ck == ck {
			converged = line
			return false
		}
		if line >= pending && s.signals.Load() != signals {
			return false
		}
		return line+1 < target
	})

	s.mu.Lock()
	if ds.gen.Load() != gen || s.docs[ds.id] != ds {
		return false
	}
	if err != nil {
		ds.failed = true
		s.retier(ds)
		return false
	}
	ds.store(c, out)
	last := c + len(out)
	if converged < 0 && last <= pending {
		// chroma ended the stream early; never loop on a document that makes no progress.
		ds.failed = true
		s.retier(ds)
		return false
	}
	if converged < 0 {
		if last < ds.lexedTo {
			ds.lines[last].seam = true
		}
		ds.pending, ds.lexedTo = last, max(ds.lexedTo, last)
		return true
	}
	ds.pending = ds.lexedTo
	for i := converged + 1; i < ds.lexedTo; i++ {
		if ds.lines[i].dirty || ds.lines[i].seam {
			ds.pending = i
			break
		}
	}
	return true
}

// lexFull re-lexes the whole document from the top: the backstop for lookahead the checkpoints cannot see.
// It yields to any other request so a large file never holds up the viewport.
func (s *Service) lexFull(ds *docState) bool {
	c, from := 0, noCheckpoint
	for i := ds.fullFrom - 1; i > 0; i-- {
		if ds.lines[i].ck.ok() {
			c, from = i, ds.lines[i].ck
			break
		}
	}
	text, ok := s.snapshot(ds, c, len(ds.lines))
	if !ok {
		return false
	}
	head, resumeAt := ds.lines[c].spans, ds.fullFrom
	gen, signals, il := ds.gen.Load(), s.signals.Load(), ds.il
	s.mu.Unlock()
	s.flush()

	out := make([]lexed, 0, len(text))
	yielded := false
	err := il.lexRun(text, from, func(i int, spans []span, m mark) bool {
		if ds.gen.Load() != gen {
			return false
		}
		ck := toCheckpoint(text[i], m)
		if i == 0 && from.ok() {
			spans, ck = merge(append(clip(head, from.col16), spans...)), from
		}
		out = append(out, lexed{spans: spans, ck: ck})
		if c+i >= resumeAt && s.signals.Load() != signals {
			yielded = true
			return false
		}
		return true
	})

	s.mu.Lock()
	if ds.gen.Load() != gen || s.docs[ds.id] != ds {
		return false
	}
	if err != nil {
		ds.failed = true
		s.retier(ds)
		return false
	}
	end := c + len(out)
	switch {
	case end == len(ds.lines):
		ds.store(c, out)
		ds.pending, ds.lexedTo, ds.fullFrom = end, end, 0
	case yielded:
		ds.store(c, out)
		if end < ds.lexedTo {
			ds.lines[end].seam = true
		}
		ds.pending, ds.lexedTo = max(ds.pending, end), max(ds.lexedTo, end)
		ds.full, ds.fullFrom = true, end
	default:
		ds.fullFrom = 0
	}
	return true
}

func (ds *docState) store(first int, out []lexed) {
	for i, r := range out {
		l := &ds.lines[first+i]
		l.spans, l.ck, l.dirty, l.seam = r.spans, r.ck, false, false
	}
}

func (s *Service) workWhole(ds *docState) bool {
	if ds.pending >= len(ds.lines) || ds.target() == 0 {
		return true
	}
	text, ok := s.snapshot(ds, 0, len(ds.lines))
	if !ok {
		return false
	}
	if ds.kind == engMarkdown && ds.md == nil {
		ds.md = &mdDoc{}
	}
	gen, kind, lexer, md := ds.gen.Load(), ds.kind, ds.lexer, ds.md
	s.mu.Unlock()
	s.flush()

	var out [][]span
	var err error
	if kind == engMarkdown {
		out = md.update(text)
	} else {
		out, err = lexWhole(lexer, text)
	}

	s.mu.Lock()
	if ds.gen.Load() != gen || s.docs[ds.id] != ds {
		return false
	}
	if err != nil || len(out) != len(ds.lines) {
		ds.failed = true
		s.retier(ds)
		return false
	}
	for i, spans := range out {
		l := &ds.lines[i]
		l.spans, l.ck, l.dirty = spans, noCheckpoint, false
	}
	ds.pending, ds.lexedTo = len(ds.lines), len(ds.lines)
	return true
}

func lexWhole(lexer chroma.Lexer, lines []string) (out [][]span, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("lexer failed")
		}
	}()
	it, err := lexer.Tokenise(nil, strings.Join(lines, "\n")+"\n")
	if err != nil {
		return nil, err
	}
	out = make([][]span, 0, len(lines))
	sp := splitter{prose: proseLexers[lexerID(lexer)], line: func(spans []span) bool {
		out = append(out, spans)
		return len(out) < len(lines)
	}}
	for t := it(); t != chroma.EOF; t = it() {
		if !sp.push(t) {
			break
		}
	}
	return out, nil
}

// publishView sends the view lines whose spans changed since they were last sent.
func (s *Service) publishView(ds *docState) {
	ds.viewDirty = false
	if ds.state.Tier != TierExact {
		return
	}
	if ds.pending >= ds.spec.last {
		ds.spec = specLines{}
	}
	first := -1
	var batch [][]int
	flush := func() {
		if len(batch) > 0 {
			s.emit(TopicEvents, map[string]any{"type": "spans", "docId": ds.id, "version": ds.version, "first": first, "lines": batch})
		}
		batch = nil
	}
	for i := ds.viewFirst; i <= min(ds.viewLast, len(ds.lines)-1); i++ {
		spans, ok := ds.lines[i].spans, i < ds.pending
		if !ok {
			spans, ok = ds.spec.line(i)
		}
		if !ok {
			flush()
			continue
		}
		enc := encode(spans)
		h := sentHash(enc)
		if h == ds.lines[i].sent {
			flush()
			continue
		}
		ds.lines[i].sent = h
		if batch == nil {
			first = i
		}
		batch = append(batch, enc)
	}
	flush()
}

// answer replies to the Lines calls the current state can serve.
func (s *Service) answer(ds *docState) {
	ds.waiters = slices.DeleteFunc(ds.waiters, func(w *waiter) bool {
		n := len(ds.meta)
		last := min(w.last, n)
		r := linesReply{version: ds.version}
		switch {
		case ds.kind == engPlain:
			for i := w.first; i < last; i++ {
				r.lines = append(r.lines, Line{Plain: true})
			}
		case ds.state.Tier != TierExact:
		case last > ds.pending:
			return false
		default:
			for i := w.first; i < last; i++ {
				line := Line{Spans: make([]Span, len(ds.lines[i].spans))}
				for k, sp := range ds.lines[i].spans {
					line.Spans[k] = sp.export()
				}
				r.lines = append(r.lines, line)
			}
		}
		w.reply <- r
		return true
	})
}

func (ds *docState) failWaiters(err error) {
	for _, w := range ds.waiters {
		w.reply <- linesReply{err: err}
	}
	ds.waiters = nil
}
