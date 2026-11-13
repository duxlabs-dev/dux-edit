package spell

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendWordKeepsASymlinkedDictionary(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "dotfiles", "words.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(real), 0o755))
	require.NoError(t, os.WriteFile(real, []byte("first\n"), 0o644))
	link := filepath.Join(dir, "custom_dictionary.txt")
	require.NoError(t, os.Symlink(real, link))

	require.NoError(t, appendWord(link, "second"))

	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink, "the dictionary must stay a link")
	data, err := os.ReadFile(real)
	require.NoError(t, err)
	assert.Equal(t, "first\nsecond\n", string(data))
}

func TestConcurrentAppendsKeepEveryWord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom_dictionary.txt")
	const n = 64
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { assert.NoError(t, appendWord(path, "word"+strconv.Itoa(i))) })
	}
	wg.Wait()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Len(t, strings.Fields(string(data)), n)
}
