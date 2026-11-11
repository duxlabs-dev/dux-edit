package spell

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/duxlabs-dev/dux-edit/core/internal/buffer"
	"github.com/duxlabs-dev/dux-edit/core/internal/document"
	"github.com/duxlabs-dev/dux-edit/core/internal/highlight"
)

type fakeDoc struct {
	lines []string
	prose []highlight.Line
}

type fakeDocs struct {
	mu   sync.Mutex
	docs map[string]*fakeDoc
}

func (f *fakeDocs) doc(id string) *fakeDoc {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.docs[id]
}

func (f *fakeDocs) read(docID string, first, last int) ([]string, int64, int, bool) {
	d := f.doc(docID)
	if d == nil {
		return nil, 0, 0, false
	}
	last = min(last, len(d.lines))
	return d.lines[first:last], 1, 10, true
}

func (f *fakeDocs) Lines(_ context.Context, docID string, first, last int) ([]highlight.Line, int64, error) {
	d := f.doc(docID)
	if d.prose == nil {
		out := make([]highlight.Line, min(last, len(d.lines))-first)
		for i := range out {
			out[i].Plain = true
		}
		return out, 1, nil
	}
	return d.prose[first:min(last, len(d.prose))], 1, nil
}

type fakeRunner struct {
	mu    sync.Mutex
	bad   map[string]bool
	calls [][]string
}

func (f *fakeRunner) Check(_ context.Context, _, _ string, words []string) (map[string]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, words)
	out := map[string]bool{}
	for _, w := range words {
		if f.bad[w] {
			out[w] = true
		}
	}
	return out, nil
}

func (f *fakeRunner) Suggest(context.Context, string, string, string) ([]string, error) {
	return nil, nil
}

type harness struct {
	s      *Service
	docs   *fakeDocs
	runner *fakeRunner
	events []map[string]any
	dict   string
}

func newHarness(t *testing.T, bad ...string) *harness {
	t.Helper()
	dicts := t.TempDir()
	for _, f := range []string{"en_US.aff", "en_US.dic"} {
		require.NoError(t, os.WriteFile(filepath.Join(dicts, f), nil, 0o644))
	}
	h := &harness{docs: &fakeDocs{docs: map[string]*fakeDoc{}}, runner: &fakeRunner{bad: map[string]bool{}}}
	for _, w := range bad {
		h.runner.bad[w] = true
	}
	home := t.TempDir()
	h.dict = filepath.Join(home, "config", "custom_dictionary.txt")
	env := map[string]string{"DICPATH": dicts, "LANG": "en_US.UTF-8", "XDG_CONFIG_HOME": filepath.Join(home, "config")}
	h.s = newService(h.docs, h.docs, func(topic string, data any) {
		h.events = append(h.events, data.(map[string]any))
	}, h.dict, h.runner, func(k string) string { return env[k] }, home, func(string) (string, error) { return "/bin/hunspell", nil })
	status := h.s.Configure(Config{Enabled: true, Language: "auto"})
	require.True(t, status.Available)
	t.Cleanup(h.s.Close)
	return h
}

func (h *harness) open(id string, d *fakeDoc) {
	h.docs.mu.Lock()
	h.docs.docs[id] = d
	h.docs.mu.Unlock()
}

// view sets the range and waits for the worker so the test reads settled events.
func (h *harness) view(id string, first, last, cursorLine, cursorCol int) {
	h.s.View(id, first, last, cursorLine, cursorCol)
	h.s.worker.Wait()
}

func (h *harness) lastLines(t *testing.T) any {
	t.Helper()
	require.NotEmpty(t, h.events)
	return h.events[len(h.events)-1]["lines"]
}

func TestProseFilterChecksCommentsOnly(t *testing.T) {
	h := newHarness(t, "qick")
	line := "foo := bar // the qick fox"
	h.open("a", &fakeDoc{lines: []string{line}, prose: []highlight.Line{{Spans: []highlight.Span{
		{Col: 0, Len: 3, Role: highlight.RoleVariable},
		{Col: 4, Len: 2, Role: highlight.RoleOperator},
		{Col: 7, Len: 3, Role: highlight.RoleVariable},
		{Col: 11, Len: 15, Role: highlight.RoleComment, Prose: true},
	}}}})
	h.view("a", 0, 0, -1, -1)

	assert.Equal(t, [][]string{{"the", "qick", "fox"}}, h.runner.calls)
	assert.Equal(t, [][]int{{18, 4}}, h.lastLines(t))
	assert.Equal(t, map[string]any{"type": "ranges", "docId": "a", "version": int64(1), "first": 0, "lines": [][]int{{18, 4}}}, h.events[0])
}

func TestStringsOffDropsStringLiterals(t *testing.T) {
	h := newHarness(t)
	h.open("a", &fakeDoc{lines: []string{`x("helo world") // nite`}, prose: []highlight.Line{{Spans: []highlight.Span{
		{Col: 2, Len: 12, Role: highlight.RoleString, Prose: true},
		{Col: 16, Len: 7, Role: highlight.RoleComment, Prose: true},
	}}}})
	h.view("a", 0, 0, -1, -1)
	assert.Equal(t, [][]string{{"nite"}}, h.runner.calls)
}

func TestMasksAndSkipRules(t *testing.T) {
	h := newHarness(t)
	line := "see https://exmple.com/x and `cde` --flg $HOMEE ~/dirr ./locl /usr/libb a HTTP fooBar FooBar " +
		"snake_case abc123 #fff #a0b1c2d3 0xdeadbeef mail@exmple.com wrdd don’t"
	h.open("a", &fakeDoc{lines: []string{line}})
	h.view("a", 0, 0, -1, -1)
	assert.Equal(t, [][]string{{"see", "and", "wrdd", "don't"}}, h.runner.calls)
}

func TestCursorWordIsSkipped(t *testing.T) {
	h := newHarness(t, "wrdd")
	h.open("a", &fakeDoc{lines: []string{"one wrdd", "wrdd"}})
	h.view("a", 0, 1, 0, 8)
	assert.Equal(t, [][]string{{"one", "wrdd"}}, h.runner.calls)
	assert.Equal(t, [][]int{{}, {0, 4}}, h.lastLines(t))
}

func TestCacheAvoidsSecondRun(t *testing.T) {
	h := newHarness(t, "wrdd")
	h.open("a", &fakeDoc{lines: []string{"one wrdd"}})
	h.view("a", 0, 0, -1, -1)
	h.s.DocChanged(&document.Doc{ID: "a"}, buffer.Change{})
	h.s.worker.Wait()
	h.view("a", 0, 0, 0, 0)
	assert.Len(t, h.runner.calls, 1)

	h.s.DocClosed("a")
	h.view("a", 0, 0, -1, -1)
	assert.Len(t, h.runner.calls, 2, "closing the last document drops the cache")
}

func TestAddPersistsAndClearsRange(t *testing.T) {
	h := newHarness(t, "wrdd")
	h.open("a", &fakeDoc{lines: []string{"one wrdd"}})
	h.view("a", 0, 0, -1, -1)
	require.Equal(t, [][]int{{4, 4}}, h.lastLines(t))

	require.NoError(t, h.s.Add("wrdd"))
	h.s.worker.Wait()
	assert.Equal(t, [][]int{{}}, h.lastLines(t))
	data, err := os.ReadFile(h.dict)
	require.NoError(t, err)
	assert.Equal(t, "wrdd\n", string(data))
	info, err := os.Stat(h.dict)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())

	require.NoError(t, h.s.Add("wrdd"))
	data, _ = os.ReadFile(h.dict)
	assert.Equal(t, "wrdd\n", string(data))
}

func TestIgnore(t *testing.T) {
	h := newHarness(t, "wrdd")
	h.open("a", &fakeDoc{lines: []string{"one wrdd"}})
	h.view("a", 0, 0, -1, -1)
	h.s.Ignore("wrdd")
	h.s.worker.Wait()
	assert.Equal(t, [][]int{{}}, h.lastLines(t))
}

func TestPickLanguage(t *testing.T) {
	langs := []Language{{Code: "de_CH"}, {Code: "de_DE"}, {Code: "en_AU"}, {Code: "en_US"}, {Code: "fr_FR"}}
	cases := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"LANG": "fr_FR.UTF-8@euro"}, "fr_FR"},
		{map[string]string{"LANG": "en_GB.UTF-8"}, "en_US"},
		{map[string]string{"LANG": "C"}, "en_US"},
		{map[string]string{}, "en_US"},
		{map[string]string{"LANGUAGE": "de_AT:fr", "LANG": "fr_FR.UTF-8"}, "fr_FR"},
		{map[string]string{"LANGUAGE": "de_AT", "LANG": "it_IT.UTF-8"}, "de_DE"},
		{map[string]string{"LC_MESSAGES": "en_AU.UTF-8", "LANG": "de_DE.UTF-8"}, "en_AU"},
		{map[string]string{"LANG": "it_IT.UTF-8"}, ""},
	}
	for _, tc := range cases {
		got, _ := pickLanguage(localeCandidates(func(k string) string { return tc.env[k] }), langs)
		assert.Equal(t, tc.want, got.Code, "%v", tc.env)
	}
}

func TestListDictionaries(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	touch := func(dir string, names ...string) {
		for _, n := range names {
			require.NoError(t, os.WriteFile(filepath.Join(dir, n), nil, 0o644))
		}
	}
	touch(first, "en_US.aff", "en_US.dic", "de_DE.aff", "hyph_en_US.aff", "hyph_en_US.dic", "th_en_US.aff", "th_en_US.dic")
	touch(second, "en_US.aff", "en_US.dic", "fr_FR.aff", "fr_FR.dic")

	assert.Equal(t, []Language{
		{Code: "en_US", Path: filepath.Join(first, "en_US")},
		{Code: "fr_FR", Path: filepath.Join(second, "fr_FR")},
	}, listDictionaries([]string{first, filepath.Join(first, "missing"), second}))
}

func TestConfigureReasons(t *testing.T) {
	s := newService(nil, nil, nil, "", &fakeRunner{}, func(string) string { return "" }, t.TempDir(),
		func(string) (string, error) { return "", os.ErrNotExist })
	assert.Equal(t, ReasonNoHunspell, s.Configure(Config{Enabled: true}).Reason)

	s = newService(nil, nil, nil, "", &fakeRunner{}, func(k string) string {
		return map[string]string{"DICPATH": t.TempDir(), "LANG": "xx_XX"}[k]
	}, t.TempDir(), func(string) (string, error) { return "/bin/hunspell", nil })
	assert.Equal(t, ReasonNoDict, s.Configure(Config{Enabled: true, Language: "xx_XX"}).Reason)
}

func TestParseSuggestions(t *testing.T) {
	out := "@(#) International Ispell Version 3.2.06 (but really Hunspell 1.7.6)\n& wrold 2 1: world, wold\n\n"
	assert.Equal(t, []string{"world", "wold"}, parseSuggestions(out))
	assert.Nil(t, parseSuggestions("@(#) banner\n*\n\n"))
}
