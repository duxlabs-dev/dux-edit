package highlight

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/duxlabs-dev/dux-edit/core/internal/buffer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDoc struct {
	mu        sync.RWMutex
	buf       *buffer.Buffer
	version   int64
	path      string
	afterRead atomic.Pointer[func()]
}

func (f *fakeDoc) Read(fn func(b *buffer.Buffer, version int64)) {
	f.mu.RLock()
	fn(f.buf, f.version)
	f.mu.RUnlock()
	if hook := f.afterRead.Load(); hook != nil {
		(*hook)()
	}
}

func (f *fakeDoc) Path() string { return f.path }

func (f *fakeDoc) replace(t *testing.T, start, removed int, text string) buffer.Change {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch, err := f.buf.Replace(start, removed, text)
	require.NoError(t, err)
	f.version++
	return ch
}

func (f *fakeDoc) text() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.buf.Text()
}

type harness struct {
	t  *testing.T
	s  *Service
	mu sync.Mutex
	ev []map[string]any
}

func newHarness(t *testing.T) *harness {
	h := &harness{t: t}
	h.s = New(nil, h.record)
	t.Cleanup(h.s.Close)
	return h
}

func (h *harness) record(topic string, data any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch d := data.(type) {
	case stateEvent:
		h.ev = append(h.ev, map[string]any{"type": d.Type, "tier": d.Tier, "reason": d.Reason, "lexer": d.Lexer})
	case map[string]any:
		h.ev = append(h.ev, d)
	}
}

func (h *harness) events() []map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]map[string]any(nil), h.ev...)
}

func (h *harness) open(id, path, text string) *fakeDoc {
	d := &fakeDoc{buf: buffer.New(text), path: path}
	h.s.opened(id, d)
	return d
}

func (h *harness) edit(id string, d *fakeDoc, start, removed int, text string) {
	h.s.changed(id, d.replace(h.t, start, removed, text))
}

func (h *harness) lines(id string, first, last int) ([]Line, int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	lines, version, err := h.s.Lines(ctx, id, first, last)
	require.NoError(h.t, err)
	return lines, version
}

func spansOf(lines []Line) [][]Span {
	out := make([][]Span, len(lines))
	for i, l := range lines {
		if len(l.Spans) > 0 {
			out[i] = l.Spans
		}
	}
	return out
}

// reference lexes text from scratch with the unmodified lexer.
func reference(t *testing.T, lexer chroma.Lexer, text string) [][]Span {
	raw, err := lexWhole(lexer, strings.Split(text, "\n"))
	require.NoError(t, err)
	out := make([][]Span, len(raw))
	for i, spans := range raw {
		for _, s := range spans {
			out[i] = append(out[i], s.export())
		}
	}
	return out
}

const goSource = `package main

import (
	"fmt"
	"strings"
)

/* Block comment
   spanning lines */
type thing struct {
	name  string // a name
	count int
}

// Doc comment for run.
func run(t *thing, values ...int) (string, error) {
	raw := ` + "`raw\nstring {{ .Field }}`" + `
	s := "hello world\t\"quoted\""
	r := 'x'
	for i, v := range values {
		if v > 0x1f && i%2 == 0 {
			t.count += v * 3.5e2
		}
	}
	return fmt.Sprintf("%s %d", raw+s, t.count), nil
}
`

const jsSource = `import { readFile } from "node:fs";

/**
 * Docs for load.
 */
export async function load(path, opts = {}) {
    const re = /ab+c\/d/gi;
    const text = ` + "`value ${path} and ${opts.x ?? 1}\nsecond line`" + `;
    let n = 0x1F + 2.5e3;
    if (re.test(text) && n !== 3) {
        n++; // count it
    }
    return { text, n, "key": 'single quoted' };
}

class Widget extends Base {
    constructor() { super(); this.items = [1, 2, 3]; }
    get size() { return this.items.length; }
}
`

const qmlSource = `import QtQuick
import qs.Common

// A component.
Item {
    id: root

    property int count: 0
    property string label: "Hello world"
    readonly property bool wide: width > 400 /* inline */

    signal activated(string name)

    function bump(step) {
        count += step;
        return count > 10 ? "big" : 'small';
    }

    Rectangle {
        anchors.fill: parent
        color: root.wide ? Theme.primary : "transparent"
        Text { text: qsTr("Count %1").arg(root.count) }
    }
}
`

func repeatSource(src string, n int) string {
	var b strings.Builder
	for i := range n {
		b.WriteString(strings.ReplaceAll(src, "run", fmt.Sprintf("run%d", i)))
	}
	return b.String()
}

func TestIncrementalMatchesFullLex(t *testing.T) {
	inserts := []string{"x", " ", "\n", "\"", "'", "`", "/*", "*/", "//", "{", "}", "(", ")", "\n\t", "${", "/", "\\"}
	for _, tc := range []struct {
		name, path, src string
	}{
		{"go", "a.go", repeatSource(goSource, 8)},
		{"javascript", "a.js", repeatSource(jsSource, 12)},
		{"qml", "A.qml", repeatSource(qmlSource, 10)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			d := h.open("d", tc.path, tc.src)
			require.Equal(t, tc.name, h.s.State("d").Lexer)
			lexer := Match(tc.path)
			rng := rand.New(rand.NewPCG(7, uint64(len(tc.name))))
			for k := range 160 {
				text := d.text()
				size := buffer.Len16(text)
				start := rng.IntN(size + 1)
				removed := 0
				insert := inserts[rng.IntN(len(inserts))]
				if rng.IntN(3) == 0 {
					removed, insert = min(rng.IntN(20)+1, size-start), ""
				}
				if _, ok := buffer.ByteIndex(text, start); !ok {
					continue
				}
				h.edit("d", d, start, removed, insert)

				n := strings.Count(d.text(), "\n") + 1
				switch {
				case k%10 == 9:
					got, _ := h.lines("d", 0, n)
					requireSameSpans(t, d.text(), reference(t, lexer, d.text()), spansOf(got), k)
				case k%3 == 0:
					first := rng.IntN(n)
					h.s.View("d", first, first+40)
					h.lines("d", first, min(n, first+40))
				}
			}
		})
	}
}

func requireSameSpans(t *testing.T, text string, want, got [][]Span, edit int) {
	t.Helper()
	require.Len(t, got, len(want), "after edit %d", edit)
	lines := strings.Split(text, "\n")
	for i := range want {
		require.Equal(t, want[i], got[i], "after edit %d, line %d %q", edit, i, lines[i])
	}
}

func TestColumnsCountUTF16Units(t *testing.T) {
	h := newHarness(t)
	h.open("d", "a.go", "x := \"😀\" + 42")
	got, _ := h.lines("d", 0, 1)
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Spans, Span{Col: 5, Len: 4, Role: RoleString})
	assert.Contains(t, got[0].Spans, Span{Col: 12, Len: 2, Role: RoleNumber})
}

func TestDetectionOrder(t *testing.T) {
	h := newHarness(t)
	h.open("override", "main.go", "package main\n")
	h.s.SetLexer("override", "rust", false)
	h.open("filename", "main.go", "#!/usr/bin/env python3\nprint(1)\n")
	h.open("shebang", "/tmp/tool", "#!/usr/bin/env -S python3 -u\nprint(1)\n")
	h.open("mime", "", "<?xml version=\"1.0\"?>\n<a/>\n")
	h.open("content", "", "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println() }\n")
	h.open("plain", "", "Remember the package for mom\n")

	want := map[string][2]string{
		"override": {"rust", SourceOverride},
		"filename": {"go", SourceFilename},
		"shebang":  {"python", SourceShebang},
		"mime":     {"xml", SourceMIME},
		"content":  {"go", SourceContent},
		"plain":    {"plaintext", SourceNone},
	}
	for id, w := range want {
		st := h.s.State(id)
		assert.Equal(t, w, [2]string{st.Lexer, st.LexerSource}, id)
	}
	plain, _ := h.lines("plain", 0, 2)
	assert.Equal(t, []Line{{Plain: true}, {Plain: true}}, plain)
}

func TestTierThresholds(t *testing.T) {
	h := newHarness(t)
	line := strings.Repeat("x", 99) + "\n"
	bytesAt := strings.Repeat(line, MaxBytes/100) + strings.Repeat("y", MaxBytes%100)
	h.open("bytes", "a.go", bytesAt)
	h.open("bytes+1", "a.go", bytesAt+"y")
	h.open("lines", "a.go", strings.Repeat("\n", MaxLines-1))
	h.open("lines+1", "a.go", strings.Repeat("\n", MaxLines))
	// Emoji count two UTF-16 units each.
	h.open("long", "a.go", strings.Repeat("😀", MaxLineLen/2))
	h.open("long+1", "a.go", strings.Repeat("😀", MaxLineLen/2)+"x")

	for id, want := range map[string][2]string{
		"bytes": {TierExact, ""}, "bytes+1": {TierOff, ReasonSize},
		"lines": {TierExact, ""}, "lines+1": {TierOff, ReasonLines},
		"long": {TierExact, ""}, "long+1": {TierOff, ReasonLongLine},
	} {
		st := h.s.State(id)
		assert.Equal(t, want, [2]string{st.Tier, st.Reason}, id)
	}

	lines, version := h.lines("long+1", 0, 1)
	assert.Nil(t, lines)
	assert.Zero(t, version)
	assert.Equal(t, TierExact, h.s.SetLexer("long+1", "", true).Tier)
}

func TestEditAcrossCapPublishesState(t *testing.T) {
	h := newHarness(t)
	d := h.open("d", "a.go", "x := 1\n"+strings.Repeat("a", MaxLineLen))
	require.Equal(t, TierExact, h.s.State("d").Tier)

	h.edit("d", d, 7, 0, "b")
	h.lines("d", 0, 1)
	h.edit("d", d, 7, 1, "")
	h.lines("d", 0, 1)

	var tiers [][2]string
	for _, e := range h.events() {
		if e["type"] == "state" {
			tiers = append(tiers, [2]string{e["tier"].(string), e["reason"].(string)})
		}
	}
	assert.Equal(t, [][2]string{{TierExact, ""}, {TierOff, ReasonLongLine}, {TierExact, ""}}, tiers)
}

func TestEditDuringJobDropsOlderVersion(t *testing.T) {
	h := newHarness(t)
	src := repeatSource(goSource, 150)
	d := h.open("d", "a.go", src)
	n := strings.Count(src, "\n") + 1
	require.Equal(t, TierExact, h.s.State("d").Tier)

	// The job's snapshot (the second read after settle) edits the document while the worker lexes it.
	var reads atomic.Int32
	edited := make(chan struct{})
	hook := func() {
		if reads.Add(1) != 2 {
			return
		}
		ch := d.replace(t, 0, 0, "/*")
		go func() {
			h.s.changed("d", ch)
			close(edited)
		}()
	}
	d.afterRead.Store(&hook)
	h.s.View("d", 0, n)
	<-edited
	d.afterRead.Store(nil)
	mark := len(h.events())

	got, version := h.lines("d", 0, n)
	assert.Equal(t, int64(1), version)
	assert.Equal(t, reference(t, Match("a.go"), d.text()), spansOf(got))
	sawNew := false
	for _, e := range h.events()[mark:] {
		if e["type"] == "spans" {
			assert.Equal(t, int64(1), e["version"])
			sawNew = true
		}
	}
	assert.True(t, sawNew)
}

func TestMarkdownRoles(t *testing.T) {
	h := newHarness(t)
	h.open("d", "notes.md", "# Big *title*\n\nSome **bold** and `code` here.\n\n```go\nfunc main() {} // hi there\n```\n")
	got, _ := h.lines("d", 0, 8)
	require.Len(t, got, 8)

	assert.Equal(t, []Span{
		{Col: 0, Len: 6, Role: RoleHeading, Prose: true},
		{Col: 6, Len: 7, Role: RoleEmphasis, Prose: true},
	}, got[0].Spans)
	assert.Equal(t, []Span{
		{Col: 0, Len: 5, Role: RoleText, Prose: true},
		{Col: 5, Len: 8, Role: RoleStrong, Prose: true},
		{Col: 13, Len: 5, Role: RoleText, Prose: true},
		{Col: 18, Len: 6, Role: RoleCode},
		{Col: 24, Len: 6, Role: RoleText, Prose: true},
	}, got[2].Spans)
	assert.Equal(t, []Span{{Col: 0, Len: 5, Role: RoleCode}}, got[4].Spans)
	assert.Contains(t, got[5].Spans, Span{Col: 0, Len: 4, Role: RoleKeyword})
	assert.Contains(t, got[5].Spans, Span{Col: 15, Len: 11, Role: RoleComment, Prose: true})
	assert.Equal(t, []Span{{Col: 0, Len: 3, Role: RoleCode}}, got[6].Spans)
}

func TestFullRelexResumesUnderConstantRequests(t *testing.T) {
	h := newHarness(t)
	src := repeatSource(goSource, 30)
	h.open("d", "a.go", src)
	n := strings.Count(src, "\n") + 1
	h.lines("d", 0, n)
	want := reference(t, Match("a.go"), src)

	h.s.mu.Lock()
	for i := range h.s.docs["d"].lines {
		h.s.docs["d"].lines[i].spans = nil
	}
	h.s.mu.Unlock()
	relexed := func() bool {
		h.s.mu.Lock()
		defer h.s.mu.Unlock()
		for i, l := range h.s.docs["d"].lines {
			var got []Span
			for _, sp := range l.spans {
				got = append(got, sp.export())
			}
			if !slices.Equal(want[i], got) {
				return false
			}
		}
		return true
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				h.s.View("d", 0, 40)
			}
		}
	})
	h.s.saved("d")
	ok := assert.Eventually(t, relexed, 10*time.Second, 5*time.Millisecond, "every request restarted the full re-lex")
	close(stop)
	wg.Wait()
	require.True(t, ok)
	got, _ := h.lines("d", 0, n)
	assert.Equal(t, want, spansOf(got))
}

// spanTrace records, for every spans event, how far the exact lex had got when it was published.
type spanTrace struct {
	s       *Service
	mu      sync.Mutex
	pending []int
	first   map[int][]int
	wire    map[int][]int
}

func newSpanTrace(t *testing.T) *spanTrace {
	tr := &spanTrace{first: map[int][]int{}, wire: map[int][]int{}}
	tr.s = New(nil, tr.publish)
	t.Cleanup(tr.s.Close)
	return tr
}

func (tr *spanTrace) publish(_ string, data any) {
	e, ok := data.(map[string]any)
	if !ok || e["type"] != "spans" {
		return
	}
	tr.s.mu.Lock()
	pending := tr.s.docs[e["docId"].(string)].pending
	tr.s.mu.Unlock()
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.pending = append(tr.pending, pending)
	for i, enc := range e["lines"].([][]int) {
		line := e["first"].(int) + i
		if _, seen := tr.first[line]; !seen {
			tr.first[line] = enc
		}
		tr.wire[line] = enc
	}
}

func TestFarViewIsProvisionalUntilTheExactLexArrives(t *testing.T) {
	// The view sits inside a template literal opened far above it, so a lex from the root state gets it wrong.
	src := "const s = `\n" + strings.Repeat("function f() { return 1; } // $\n", 3000) + "`;\n" + jsSource
	want, err := lexWhole(Match("a.js"), strings.Split(src, "\n"))
	require.NoError(t, err)
	const first, last = 2500, 2540

	tr := newSpanTrace(t)
	tr.s.opened("d", &fakeDoc{buf: buffer.New(src), path: "a.js"})
	tr.s.View("d", first, last)
	exact := func() bool {
		tr.mu.Lock()
		defer tr.mu.Unlock()
		for i := first; i <= last; i++ {
			if !slices.Equal(encode(want[i]), tr.wire[i]) {
				return false
			}
		}
		return true
	}
	require.Eventually(t, exact, 20*time.Second, 5*time.Millisecond, "the view kept its provisional spans")

	tr.mu.Lock()
	defer tr.mu.Unlock()
	assert.Zero(t, tr.pending[0], "the first spans waited for the lex from the top")
	for i := first; i <= last; i++ {
		assert.NotEqual(t, encode(want[i]), tr.first[i], "line %d was not lexed from the root state", i)
	}
}

func TestNearViewWaitsForTheExactLex(t *testing.T) {
	src := repeatSource(goSource, 40)
	tr := newSpanTrace(t)
	tr.s.opened("d", &fakeDoc{buf: buffer.New(src), path: "a.go"})
	tr.s.View("d", specGap-40, specGap)
	sent := func() bool {
		tr.mu.Lock()
		defer tr.mu.Unlock()
		return len(tr.pending) > 0
	}
	require.Eventually(t, sent, 20*time.Second, 5*time.Millisecond)

	tr.mu.Lock()
	defer tr.mu.Unlock()
	assert.Greater(t, tr.pending[0], specGap-40)
}

func TestMarkdownEditsReparseBlocks(t *testing.T) {
	h := newHarness(t)
	rng := rand.New(rand.NewPCG(3, 5))
	d := h.open("d", "notes.md", randomMarkdown(rng, 20))
	for k := range 60 {
		text := d.text()
		next := randomMarkdownEdit(rng, text)
		p := 0
		for p < len(text) && p < len(next) && text[p] == next[p] {
			p++
		}
		for p > 0 && p < len(text) && text[p]&0xC0 == 0x80 {
			p--
		}
		s := 0
		for s < len(text)-p && s < len(next)-p && text[len(text)-1-s] == next[len(next)-1-s] {
			s++
		}
		for s > 0 && next[len(next)-s]&0xC0 == 0x80 {
			s--
		}
		h.edit("d", d, buffer.Len16(text[:p]), buffer.Len16(text[p:len(text)-s]), next[p:len(next)-s])
		require.Equal(t, next, d.text())

		lines := strings.Split(next, "\n")
		got, _ := h.lines("d", 0, len(lines))
		var want [][]Span
		for _, spans := range markdownSpans(lines) {
			var line []Span
			for _, sp := range spans {
				line = append(line, sp.export())
			}
			want = append(want, line)
		}
		requireSameSpans(t, next, want, spansOf(got), k)
	}
	h.s.mu.Lock()
	defer h.s.mu.Unlock()
	assert.Greater(t, h.s.docs["d"].md.partial, 10)
}
