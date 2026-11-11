package markdown

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskLinksCarrySourceLines(t *testing.T) {
	html, err := Render("# Todo\n\n- [ ] first\n- [x] second\n  continued\n- [ ] third\n", Options{})
	require.NoError(t, err)
	assert.Contains(t, html, `<a href="task:2">☐</a> first`)
	assert.Contains(t, html, `<a href="task:3">☑</a> second`)
	assert.Contains(t, html, `<a href="task:5">☐</a> third`)
	assert.NotContains(t, html, "<input")
}

func TestRawHTMLIsDropped(t *testing.T) {
	html, err := Render("<script>alert(1)</script>\n\ntext <b onclick=\"x\">bold</b>\n", Options{})
	require.NoError(t, err)
	assert.NotContains(t, html, "<script")
	assert.NotContains(t, html, "<b ")
}

func TestFencedCodeUsesPalette(t *testing.T) {
	opt := Options{Palette: map[string]string{"keyword": "#ff0000"}, CodeBackground: "#101010"}
	html, err := Render("```go\nfunc main() {}\n```\n", opt)
	require.NoError(t, err)
	assert.Contains(t, html, `<pre style="white-space:pre-wrap; background-color:#101010;">`)
	assert.Contains(t, html, `<span style="color:#ff0000">func</span>`)
	assert.NotContains(t, html, "class=")
}

func TestPaletteRejectsNonColors(t *testing.T) {
	html, err := Render("```go\nfunc main() {}\n```\n", Options{Palette: map[string]string{"keyword": `red" onload="x`}})
	require.NoError(t, err)
	assert.NotContains(t, html, "onload")
}

func TestImages(t *testing.T) {
	html, err := Render("![a cat](https://example.com/cat.png) ![local](img/cat.png)\n", Options{})
	require.NoError(t, err)
	assert.Contains(t, html, `<a href="https://example.com/cat.png">a cat</a>`)
	assert.NotContains(t, html, `<img src="https://`)
	assert.Contains(t, html, `<img src="img/cat.png" alt="local">`)
}

func TestStrikethroughAndLinks(t *testing.T) {
	html, err := Render("~~gone~~ [site](https://example.com)\n", Options{LinkColor: "#00ff00"})
	require.NoError(t, err)
	assert.Contains(t, html, "<s>gone</s>")
	assert.Contains(t, html, `<a href="https://example.com" style="color:#00ff00">site</a>`)
}
