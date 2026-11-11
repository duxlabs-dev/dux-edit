package dmsimport

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixture(t *testing.T) (state, config string) {
	t.Helper()
	root := t.TempDir()
	state, config = filepath.Join(root, "state", "dms"), filepath.Join(root, "config", "dms")
	require.NoError(t, os.MkdirAll(filepath.Join(state, "notepad-files"), 0o755))
	require.NoError(t, os.MkdirAll(config, 0o755))
	write := func(path, body string) {
		require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	}
	write(filepath.Join(state, "notepad-files", "untitled-1.txt"), "first\r\nnote\n")
	write(filepath.Join(state, "notepad-files", "untitled-2.txt"), " \n\t")
	write(filepath.Join(root, "outside.txt"), "not a note")
	write(filepath.Join(state, "notepad-session.json"), `{
		"version": 1,
		"currentTabIndex": 3,
		"tabs": [
			{"id": 1, "title": "Groceries", "filePath": "notepad-files/untitled-1.txt", "isTemporary": true},
			{"id": 2, "title": "Untitled", "filePath": "notepad-files/untitled-2.txt", "isTemporary": true},
			{"id": 3, "title": "hosts", "filePath": "/etc/hosts", "isTemporary": false},
			{"id": 4, "title": "Escape", "filePath": "../../outside.txt", "isTemporary": true},
			{"id": 5, "title": "Gone", "filePath": "notepad-files/missing.txt", "isTemporary": true}
		]
	}`)
	write(filepath.Join(config, "settings.json"), `{"notepadFontSize": 15, "notepadShowLineNumbers": true, "notepadTransparencyOverride": 0.5}`)
	return state, config
}

func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	require.NoError(t, filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		require.NoError(t, err)
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		files[path] = string(data)
		return nil
	}))
	return files
}

func TestImportCopiesNotesIntoDrafts(t *testing.T) {
	state, config := fixture(t)
	before := snapshot(t, filepath.Dir(filepath.Dir(state)))
	swapDir := t.TempDir()

	result, err := Run(state, config, swapDir, false)
	require.NoError(t, err)
	assert.True(t, result.Found)
	require.Len(t, result.Tabs, 2, "empty, missing and escaping notes are skipped")
	assert.Equal(t, "Groceries", result.Tabs[0].Title)
	assert.Equal(t, Tab{Title: "hosts", Path: "/etc/hosts"}, result.Tabs[1])
	assert.Equal(t, 1, result.Current, "the skipped current tab falls back to the one before it")
	assert.Equal(t, map[string]any{"fontSize": float64(15), "showLineNumbers": true}, result.Settings)

	id := result.Tabs[0].SwapID
	require.NotEmpty(t, id)
	text, err := os.ReadFile(filepath.Join(swapDir, id+".txt"))
	require.NoError(t, err)
	assert.Equal(t, "first\nnote\n", string(text))
	raw, err := os.ReadFile(filepath.Join(swapDir, id+".json"))
	require.NoError(t, err)
	var side map[string]any
	require.NoError(t, json.Unmarshal(raw, &side))
	assert.Equal(t, "", side["path"])
	assert.Equal(t, "Groceries", side["title"])
	assert.Equal(t, "utf-8", side["encoding"])
	assert.Equal(t, "lf", side["eol"])
	for _, ext := range []string{".txt", ".json"} {
		info, err := os.Stat(filepath.Join(swapDir, id+ext))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}

	assert.Equal(t, before, snapshot(t, filepath.Dir(filepath.Dir(state))), "nothing under the DMS dirs changes")
}

func TestDryRunWritesNothing(t *testing.T) {
	state, config := fixture(t)
	swapDir := t.TempDir()

	result, err := Run(state, config, swapDir, true)
	require.NoError(t, err)
	require.Len(t, result.Tabs, 2)
	assert.Empty(t, result.Tabs[0].SwapID)
	entries, err := os.ReadDir(swapDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestNoSessionIsNotFound(t *testing.T) {
	result, err := Run(t.TempDir(), t.TempDir(), t.TempDir(), false)
	require.NoError(t, err)
	assert.False(t, result.Found)
	assert.Empty(t, result.Tabs)
}
