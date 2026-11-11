package swap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteIsPrivateAndLockIsExclusive(t *testing.T) {
	dir := Dir(t.TempDir())
	id := NewID()
	require.NoError(t, dir.Write(id, "secret", Sidecar{Title: "t"}))
	for _, ext := range []string{".txt", ".json"} {
		info, err := os.Stat(filepath.Join(string(dir), id+ext))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}

	lock, err := dir.Lock(id)
	require.NoError(t, err)
	_, err = dir.Lock(id)
	assert.ErrorIs(t, err, ErrLocked)
	lock.Release()
	again, err := dir.Lock(id)
	require.NoError(t, err)
	again.Release()

	_, err = dir.Lock("../escape")
	assert.ErrorIs(t, err, ErrBadID)
}
