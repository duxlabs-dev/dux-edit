package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/duxlabs-dev/dux-edit/core/internal/backend"
)

func TestParseParamsBuildsLists(t *testing.T) {
	open, ok := backend.FindMethod("ui.open")
	require.True(t, ok)

	params, err := parseParams(open, []string{"paths=/a", "line=3", "simple=true"})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"paths": []any{"/a"}, "line": float64(3), "simple": true}, params, "a list param given once is still a list")

	params, err = parseParams(open, []string{"paths=/a", "paths=2024", "paths=[\"/c\"]"})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"paths": []any{"/a", "2024", "/c"}}, params, "list items stay strings")

	params, err = parseParams(backend.MethodSpec{}, []string{"x=1", "x=two", "name=[draft"})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"x": []any{float64(1), "two"}, "name": "[draft"}, params, "a repeated key becomes a list")

	_, err = parseParams(open, []string{"novalue"})
	assert.Error(t, err)
}

func TestOpenParamsReadPositions(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "plain.txt")
	literal := filepath.Join(dir, "odd:3")
	require.NoError(t, os.WriteFile(plain, nil, 0o644))
	require.NoError(t, os.WriteFile(literal, nil, 0o644))
	t.Cleanup(func() { openFlags.line, openFlags.column = 0, 0 })

	cases := []struct {
		name         string
		args         []string
		flagLine     int
		paths        []string
		line, column int
	}{
		{"suffix", []string{plain + ":12:5"}, 0, []string{plain}, 12, 5},
		{"compiler trailing colon", []string{plain + ":7:"}, 0, []string{plain}, 7, 0},
		{"literal file wins", []string{literal}, 0, []string{literal}, 0, 0},
		{"the reading that names a file wins", []string{literal + ":8"}, 0, []string{literal}, 8, 0},
		{"line zero is part of the name", []string{plain + ":0"}, 0, []string{plain + ":0"}, 0, 0},
		{"plus", []string{"+4:2", plain}, 0, []string{plain}, 4, 2},
		{"plus beats suffix", []string{plain + ":9", "+4"}, 0, []string{plain}, 4, 0},
		{"flag beats both", []string{"+4", plain + ":9"}, 2, []string{plain}, 2, 0},
		{"only the first file", []string{plain, literal + ":8"}, 0, []string{plain, literal}, 0, 0},
		{"plus alone opens nothing", []string{"+4"}, 0, []string{}, 4, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			openFlags.line, openFlags.column = tc.flagLine, 0
			params, err := openParams(tc.args)
			require.NoError(t, err)
			assert.Equal(t, tc.paths, params["paths"])
			assert.Equal(t, tc.line, params["line"])
			assert.Equal(t, tc.column, params["column"])
		})
	}
}

func TestPlusArgThatExistsIsAFile(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile("+5", nil, 0o644))
	params, err := openParams([]string{"+5"})
	require.NoError(t, err)
	abs, err := filepath.Abs("+5")
	require.NoError(t, err)
	assert.Equal(t, []string{abs}, params["paths"])
	assert.Equal(t, 0, params["line"])
}
