package backend

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/duxlabs-dev/dux-edit/core/internal/buffer"
	"github.com/duxlabs-dev/dux-edit/core/internal/document"
	"github.com/duxlabs-dev/dux-edit/core/internal/highlight"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bracketBackend(t *testing.T, text, lexer string) (*Backend, string) {
	t.Helper()
	publish := func(string, any) {}
	b := &Backend{docs: document.NewStore(publish)}
	b.docs.SetSwapDir(t.TempDir())
	b.highlight = highlight.New(b.docs, publish)
	b.docs.AddListener(b.highlight)
	t.Cleanup(func() {
		b.highlight.Close()
		b.docs.Shutdown()
	})
	opened := b.docs.Open(document.OpenRequest{}).(document.OpenResult)
	require.True(t, opened.OK)
	docID := opened.DocID
	require.Equal(t, true, b.docs.Reset(docID, 0, text, false).(map[string]any)["ok"])
	b.highlight.SetLexer(docID, lexer, false)
	return b, docID
}

func bracketRequest(b *Backend, docID string, version int, offset, first, last int) map[string]any {
	return b.docMatchBracket(context.Background(), map[string]any{
		"docId": docID, "version": version, "offset": offset, "first": first, "last": last,
	}).(map[string]any)
}

func TestDocMatchBracketIgnoresStringsAndComments(t *testing.T) {
	for _, tc := range []struct {
		name  string
		text  string
		lexer string
	}{
		{"quoted and block comment", `f(")", /* ) */ (a))`, "javascript"},
		{"astral and nesting", "😀f(')', /* ) */ [a, (😀)])", "javascript"},
		{"line comment", "f(a, // ) (\n[b])", "javascript"},
		{"escaped quote", `f("\" )", a)`, "go"},
		{"raw string", "f(`) (\n😀)`, a)", "go"},
		{"empty and astral lines", "f(\n\n😀\n)", "go"},
		{"regex", `f(/\)/, a)`, "javascript"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, docID := bracketBackend(t, tc.text, tc.lexer)
			open := buffer.Len16(tc.text[:strings.Index(tc.text, "(")])
			close := buffer.Len16(tc.text[:strings.LastIndex(tc.text, ")")])
			last := strings.Count(tc.text, "\n")
			for _, offset := range []int{open, open + 1} {
				result := bracketRequest(b, docID, 0, offset, 0, last)
				require.Equal(t, true, result["ok"])
				assert.Equal(t, []int{open, close}, result["pair"])
			}
			result := bracketRequest(b, docID, 0, close+1, 0, last)
			require.Equal(t, true, result["ok"])
			assert.Equal(t, []int{close, open}, result["pair"])
		})
	}
}

func TestDocMatchBracketDoesNotStartInsideALiteral(t *testing.T) {
	text := "f(\"(😀)\", /* (a) */ `(b)`)"
	b, docID := bracketBackend(t, text, "javascript")
	for _, inside := range []string{"(😀)", "(a)", "(b)"} {
		offset := buffer.Len16(text[:strings.Index(text, inside)])
		for _, cursor := range []int{offset, offset + 1} {
			result := bracketRequest(b, docID, 0, cursor, 0, 0)
			require.Equal(t, true, result["ok"])
			assert.Nil(t, result["pair"])
		}
	}
}

func TestDocMatchBracketBoundsAndPlainText(t *testing.T) {
	b, docID := bracketBackend(t, "(😀\n[{a}]\n)", "plaintext")
	for _, tc := range []struct {
		offset, first, last int
		pair                []int
	}{
		{0, 0, 2, []int{0, 10}},
		{11, 0, 2, []int{10, 0}},
		{10, 0, 2, []int{10, 0}},
		{4, 0, 2, []int{4, 8}},
		{9, 0, 2, []int{8, 4}},
		{5, 0, 2, []int{4, 8}},
		{6, 0, 2, []int{5, 7}},
		{8, 0, 2, []int{7, 5}},
		{0, 0, 1, nil},
		{11, 1, 2, nil},
		{0, 1, 2, nil},
		{2, 0, 2, nil},
	} {
		result := bracketRequest(b, docID, 0, tc.offset, tc.first, tc.last)
		require.Equal(t, true, result["ok"])
		assert.Equal(t, tc.pair, result["pair"])
	}
	wide := "(" + strings.Repeat(" ", bracketBudget) + ")"
	b, docID = bracketBackend(t, wide, "plaintext")
	assert.Nil(t, bracketRequest(b, docID, 0, 1, 0, 0)["pair"])
	assert.Nil(t, bracketRequest(b, docID, 0, len(wide), 0, 0)["pair"])
}

func TestDocMatchBracketSkipsTierOff(t *testing.T) {
	b, docID := bracketBackend(t, "f("+strings.Repeat("a", 10001)+")", "go")
	require.Equal(t, highlight.TierOff, b.highlight.State(docID).Tier)
	result := bracketRequest(b, docID, 0, 2, 0, 0)
	require.Equal(t, true, result["ok"])
	assert.Nil(t, result["pair"])
}

func TestDocMatchBracketWaitsForEditsAndRejectsOldVersions(t *testing.T) {
	b, docID := bracketBackend(t, "()", "go")
	reply := make(chan map[string]any, 1)
	go func() { reply <- bracketRequest(b, docID, 1, 1, 0, 0) }()
	b.docs.Edit(docID, 1, 1, 0, `")"`, 5, nil)
	select {
	case result := <-reply:
		require.Equal(t, true, result["ok"])
		assert.Equal(t, int64(1), result["version"])
		assert.Equal(t, []int{0, 4}, result["pair"])
	case <-time.After(3 * time.Second):
		t.Fatal("bracket request did not observe the edit")
	}
	assert.Equal(t, "SUPERSEDED", bracketRequest(b, docID, 0, 1, 0, 0)["code"])
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := b.docMatchBracket(ctx, map[string]any{"docId": docID, "version": 2}).(map[string]any)
	assert.Equal(t, document.CodeDesync, result["code"])
}
