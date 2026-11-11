package atomicfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveKeepsModeAndWritesThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.sh")
	link := filepath.Join(dir, "link.sh")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o640))
	require.NoError(t, os.Chmod(target, 0o751))
	require.NoError(t, os.Symlink("real.sh", link))

	id, err := Save(link, []byte("new"), Options{})
	require.NoError(t, err)

	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink, "the link must stay a link")
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))
	info, err = os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o751), info.Mode().Perm())

	current, err := Stat(link)
	require.NoError(t, err)
	assert.Equal(t, current, id)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 2, "no temp file left behind")
}

func TestSaveCreatesThroughDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink("made.txt", link))

	_, err := Save(link, []byte("x"), Options{Mode: 0o600})
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(dir, "made.txt"))
	require.NoError(t, err)
	assert.Equal(t, "x", string(data))
}

func TestSaveKeepsHardLinks(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	require.NoError(t, os.WriteFile(a, []byte("old and longer"), 0o644))
	require.NoError(t, os.Link(a, b))

	_, err := Save(a, []byte("new"), Options{})
	require.NoError(t, err)
	data, err := os.ReadFile(b)
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))
}
