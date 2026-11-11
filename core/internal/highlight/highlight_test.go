package highlight

import (
	"context"
	"fmt"
	"math/rand/v2"
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
