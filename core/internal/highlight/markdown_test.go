package highlight

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var mdBlocks = []string{
	"# Title with *emphasis*\n",
	"Setext heading\n==============\n",
	"A paragraph with **strong**, `code`, ~~gone~~ and a [link](http://x.y/z).\nSecond line with [ref] and [text][ref] and ![img](a.png).\n",
	"Trailing [ref](unbalanced and \\\n",
	"[ref]: http://example.com/ref \"Title\"\n",
	"[other\nlabel]: /multi\n",
	"```go\nfunc main() {}\n\n// after a blank line\n```\n",
	"~~~\nplain fence\n\n# not a heading\n~~~\n",
	"````md\n```\ninner\n```\n````\n",
	"- item one\n- item two\n\n  continued paragraph\n- [ ] task\n",
	"- step:\n  ```sh\n  echo hi\n\n  echo more\n  ```\n- next\n",
	"- loose\n\n- list\n\n- items\n",
	"1. first\n2. second\n   ```\n   nested, unclosed\n",
	"> quote line\n> second with `code`\nlazy continuation\n",
	"> ```\n> quoted fence\n",
	"<!-- comment\n\nstill comment\n-->\n",
	"<!-- one line -->\n",
	"<div class=\"x\">\n*not emphasis*\n</div>\n",
	"<pre>\nkept\n\n# as is\n</pre>\n",
	"<script>\nlet a = 1;\n",
	"<?php\n\necho 1;\n?>\n",
	"| a | b |\n|---|---|\n| 1 | `2` |\n",
	"    indented code\n\n    more of it\n",
	"---\n",
	"* * *\n",
	"Text with <span>raw</span> html and https://auto.link/x and <http://a.b>.\n",
	"`unclosed code span\n",
	"Emoji 😀 and *wide 字* text.\n",
	"\t- tab indented\n",
	"   ```\n   indented fence\n   ```\n",
	"```\nunclosed fence\n",
}

var mdInserts = []string{
	"x", " ", "\n", "\n\n", "```", "~~~", "`", "*", "**", "#", "# ", "> ", "- ", "1. ", "    ", "\t", "[", "]", "]:", "(", ")",
	"<!--", "-->", "<pre>", "</pre>", "<div>", "[ref]", "[ref]: /u", "---", "===", "|", "\\", "<", ">", "\n```\n", "\n\n```\n\n",
}

func randomMarkdown(rng *rand.Rand, blocks int) string {
	var b strings.Builder
	for range blocks {
		b.WriteString(mdBlocks[rng.IntN(len(mdBlocks))])
		if rng.IntN(5) > 0 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func randomMarkdownEdit(rng *rand.Rand, text string) string {
	at := rng.IntN(len(text) + 1)
	for at < len(text) && text[at]&0xC0 == 0x80 {
		at++
	}
	switch rng.IntN(6) {
	case 0, 1:
		end := min(len(text), at+1+rng.IntN(12))
		for end < len(text) && text[end]&0xC0 == 0x80 {
			end++
		}
		return text[:at] + text[end:]
	case 2:
		return text[:at] + mdBlocks[rng.IntN(len(mdBlocks))] + text[at:]
	}
	return text[:at] + mdInserts[rng.IntN(len(mdInserts))] + text[at:]
}

func TestMarkdownBlockReparseMatchesFullParse(t *testing.T) {
	rounds, edits := 8, 100
	partial, total := 0, 0
	for seed := range rounds {
		rng := rand.New(rand.NewPCG(11, uint64(seed)))
		text := randomMarkdown(rng, 20)
		var m mdDoc
		m.update(strings.Split(text, "\n"))
		for k := range edits {
			text = randomMarkdownEdit(rng, text)
			lines := strings.Split(text, "\n")
			got := m.update(lines)
			want := parseMarkdown(lines, nil)
			require.Len(t, got, len(lines))
			for i := range lines {
				require.Equal(t, want.spans[i], got[i], "seed %d edit %d line %d %q\n%s", seed, k, i, lines[i], text)
			}
			require.Equal(t, want.inner, m.inner, "seed %d edit %d\n%s", seed, k, text)
			total++
		}
		partial += m.partial
	}
	require.Greater(t, partial, total/4)
}

// Edits whose block range sits next to something that used to paint across a blank line.
func TestMarkdownBlockReparseStaysInsideItsRange(t *testing.T) {
	for name, tc := range map[string][2]string{
		"fence closed by its list item":   {"- a\n  ```\n  code\n\n`a` | b\n--- | ---\n", "- a\n  ```\n  code\n\n`a` | b!\n--- | ---\n"},
		"reference link before a bracket": {"[ref]: /u\n\n[ref](open\n\n| a | b ) |\n|---|---|\n", "[ref]: /u\n\n[ref](open\n\n| a | b ) c |\n|---|---|\n"},
		"edit below a comment block":      {"<!--\n\n# hidden\n\n-->\n\n# shown\n", "<!--\n\n# hidden\n\n-->\n\n# shown *now*\n"},
		"edit inside a fence":             {"intro\n\n```\na\n\nb\n```\n\noutro\n", "intro\n\n```\na\n\n# b\n```\n\noutro\n"},
		"link to a definition elsewhere":  {"[ref]: /u\n\nsee ref\n\nend\n", "[ref]: /u\n\nsee [ref]\n\nend\n"},
	} {
		t.Run(name, func(t *testing.T) {
			var m mdDoc
			m.update(strings.Split(tc[0], "\n"))
			lines := strings.Split(tc[1], "\n")
			got := m.update(lines)
			require.Equal(t, 1, m.partial, "the edit parsed the whole document")
			require.Equal(t, parseMarkdown(lines, nil).spans, got)
		})
	}
}
