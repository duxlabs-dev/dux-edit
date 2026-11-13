package search

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFind(t *testing.T) {
	cases := []struct {
		name   string
		text   string
		q      Query
		ranges []int
	}{
		{"utf16 after emoji", "😀 cat\ncat", Query{Text: "cat"}, []int{3, 3, 7, 3}},
		{"case insensitive", "Cat cat", Query{Text: "cat"}, []int{0, 3, 4, 3}},
		{"case sensitive", "Cat cat", Query{Text: "cat", CaseSensitive: true}, []int{4, 3}},
		{"literal is quoted", "a.b axb", Query{Text: "a.b"}, []int{0, 3}},
		{"whole word", "cat concat cat_x cat", Query{Text: "cat", WholeWord: true}, []int{0, 3, 17, 3}},
		{"whole word non-ascii", "café cafés", Query{Text: "café", WholeWord: true}, []int{0, 4}},
		{"regex per line anchors", "ab\nab", Query{Text: "^a", Regex: true}, []int{0, 1, 3, 1}},
		{"zero-length skipped", "ab", Query{Text: "x*", Regex: true}, []int{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Find(tc.text, tc.q)
			require.NoError(t, err)
			assert.Equal(t, tc.ranges, m.Ranges)
			assert.Equal(t, len(tc.ranges)/2, m.Total)
		})
	}
}

func TestFindCap(t *testing.T) {
	m, err := Find("a a a a", Query{Text: "a", Limit: 2})
	require.NoError(t, err)
	assert.Equal(t, []int{0, 1, 2, 1}, m.Ranges)
	assert.Equal(t, 2, m.Total)
	assert.True(t, m.Capped)
}

func TestFindLooksPastTheFirstBatchOnlyWhenFiltered(t *testing.T) {
	text := strings.Repeat("xa ", 3*firstBatch) + "a"
	m, err := Find(text, Query{Text: "a", CaseSensitive: true, WholeWord: true, Limit: 5})
	require.NoError(t, err)
	assert.Equal(t, []int{9 * firstBatch, 1}, m.Ranges, "the one whole word sits behind three batches of rejected hits")
	assert.False(t, m.Capped)
}

func TestFoldedLiteralMatchesRegex(t *testing.T) {
	text := "Straße HELLO héllo hello😀Hello\nxHeLLo_ hello\tHELLOHELLO 日本hello語 aaaaa ÉHELLO café"
	for _, query := range []string{"hello", "HeLLo", "aa", "o h", "l", "caf", "zzz"} {
		folded, err := compile(Query{Text: query})
		require.NoError(t, err)
		require.Nil(t, folded.re, "%q takes the folded path", query)
		re := matcher{re: regexp.MustCompile("(?mi)" + regexp.QuoteMeta(query))}
		for _, wholeWord := range []bool{false, true} {
			want, got := re.matches(text, wholeWord, -1, nil), folded.matches(text, wholeWord, -1, nil)
			assert.Equal(t, len(want), len(got), "%q whole word %v", query, wholeWord)
			for i := range min(len(want), len(got)) {
				assert.Equal(t, want[i], got[i], "%q whole word %v", query, wholeWord)
			}
		}
	}
}

func TestBadRegex(t *testing.T) {
	_, err := Find("x", Query{Text: "(", Regex: true})
	assert.ErrorIs(t, err, ErrBadRegex)
}

func TestReplaceAll(t *testing.T) {
	b := "😀 x=1, y=2 end"
	r, err := ReplaceAll(b, Query{Text: `(?P<k>\w)=(\d)`, Regex: true}, "$2:${k}")
	require.NoError(t, err)
	assert.Equal(t, Replacement{OK: true, Start: 3, End: 11, Text: "1:x, 2:y", Count: 2}, r)

	r, err = ReplaceAll(b, Query{Text: "x=1"}, "$1")
	require.NoError(t, err)
	assert.Equal(t, "$1", r.Text)
}

func TestExpand(t *testing.T) {
	b := "😀 x=1"
	text, err := Expand(b, Query{Text: `(\w)=(\d)`, Regex: true}, "$2$1", 3, 3)
	require.NoError(t, err)
	assert.Equal(t, "1x", text)

	_, err = Expand(b, Query{Text: `(\w)=(\d)`, Regex: true}, "$2$1", 4, 2)
	assert.ErrorIs(t, err, ErrNoMatch)
}
