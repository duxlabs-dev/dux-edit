package indent

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModeline(t *testing.T) {
	filler := strings.Repeat("x\n", 20)
	cases := []struct {
		name string
		text string
		want Hints
	}{
		{"vim first form", "# vim: ts=4 sw=2 et\nbody\n", Hints{Space, 2}},
		{"vim colon separated", "// vim:noet:ts=8\n", Hints{Tab, 8}},
		{"vim set form", "/* vim: set expandtab shiftwidth=3: */\n", Hints{Space, 3}},
		{"vi and versioned vim", "x vi: sw=6\ny vim600: et\n", Hints{Space, 6}},
		{"set form without a closing colon is not a modeline", "/* vim: set sw=3 */\n", Hints{}},
		{"marker glued to text is not a modeline", "foovim: sw=3\n", Hints{}},
		{"ex at the start of a line is ignored", "ex: sw=3\n", Hints{}},
		{"ex after a blank", "# ex: sw=3\n", Hints{"", 3}},
		{"tab style takes the tab width", "# vim: noet ts=8 sw=4\n", Hints{Tab, 8}},
		{"width out of range", "# vim: sw=0 ts=99\n", Hints{}},
		{"emacs first line", "# -*- mode: python; indent-tabs-mode: nil; tab-width: 4 -*-\n", Hints{Space, 4}},
		{"emacs t", ";; -*- indent-tabs-mode: t -*-\n", Hints{Tab, 0}},
		{"emacs local variables", filler + "# Local Variables:\n# tab-width: 3\n# indent-tabs-mode: t\n# End:\n", Hints{Tab, 3}},
		{"emacs local variables with suffix", filler + "/* Local Variables: */\n/* indent-tabs-mode: nil */\n/* End: */\n", Hints{Space, 0}},
		{"middle lines are not read", "a\n" + "# vim: sw=3\n" + filler + filler, Hints{"", 3}},
		{"only the edges", "1\n2\n3\n4\n5\n6\n# vim: sw=3\n" + filler, Hints{}},
		{"the last modeline wins", "# vim: sw=2 et\n" + filler + "# vim: sw=8 noet ts=4\n", Hints{Tab, 4}},
		{"long text keeps the edges", "# vim: sw=2\n" + strings.Repeat("y", 3*modelineScan) + "\n# vim: et\n", Hints{Space, 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, modeline(tc.text).hints())
		})
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestEditorConfig(t *testing.T) {
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, ".editorconfig"), "[*]\nindent_style = tab\nindent_size = 7\n")
	root := filepath.Join(outside, "project")
	writeFile(t, filepath.Join(root, ".editorconfig"), `root = true

[*]
indent_style = space
indent_size = 4
trim_trailing_whitespace = true

[*.{js,ts}]
indent_size = 2

[Makefile]
indent_style = tab
tab_width = 8

[*.go]
indent_style = tab
indent_size = tab
tab_width = 4

[*.md]
indent_style = unset
indent_size = unset
`)
	writeFile(t, filepath.Join(root, "sub", ".editorconfig"), "[*.py]\nindent_size = 3\n")

	cases := map[string]Hints{
		"a.txt":        {Space, 4},
		"web/app.ts":   {Space, 2},
		"Makefile":     {Tab, 8},
		"cmd/main.go":  {Tab, 4},
		"sub/tool.py":  {Space, 3},
		"sub/notes.md": {},
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, want, Detect(filepath.Join(root, name), ""))
		})
	}
}

func TestModelineOverridesEditorConfig(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".editorconfig"), "root = true\n[*]\nindent_style = space\nindent_size = 4\n")
	path := filepath.Join(root, "a.c")
	assert.Equal(t, Hints{Tab, 4}, Detect(path, "/* vim: set noet: */\n"))
	assert.Equal(t, Hints{Space, 2}, Detect(path, "# vim: sw=2\n"))
}

func TestEditorConfigSkipsWhatItCannotReadQuickly(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".editorconfig"), "root = true\n[*]\nindent_size = 4\n")
	fifoDir := filepath.Join(root, "fifo")
	require.NoError(t, os.Mkdir(fifoDir, 0o755))
	require.NoError(t, syscall.Mkfifo(filepath.Join(fifoDir, ".editorconfig"), 0o644))
	writeFile(t, filepath.Join(root, "big", ".editorconfig"), "[*]\nindent_size = 2\n"+strings.Repeat("#\n", maxConfigBytes))

	done := make(chan [2]Hints, 1)
	go func() {
		done <- [2]Hints{Detect(filepath.Join(fifoDir, "a.txt"), ""), Detect(filepath.Join(root, "big", "a.txt"), "")}
	}()
	select {
	case got := <-done:
		assert.Equal(t, Hints{"", 4}, got[0], "a FIFO is skipped, not opened")
		assert.Equal(t, Hints{"", 4}, got[1], "an oversized file is skipped")
	case <-time.After(5 * time.Second):
		t.Fatal("Detect blocked on a FIFO")
	}
}
