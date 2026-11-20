package elevate

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/duxlabs-dev/dux-go/atomicfile"
	"github.com/duxlabs-dev/dux-go/elevate"
	dhelper "github.com/duxlabs-dev/dux-go/elevate/helper"
	"github.com/duxlabs-dev/dux-go/files"

	"github.com/duxlabs-dev/dux-edit/core/internal/helper"
)

const fakeEnv = "DUX_EDIT_ELEVATE_FAKE_HELPER"

// The test binary stands in for pkexec: "<binary> <exe> helper <op> ..." runs the operation unprivileged.
func TestMain(m *testing.M) {
	if os.Getenv(fakeEnv) == "" {
		os.Exit(m.Run())
	}
	args := os.Args[1:]
	if len(args) < 2 || args[1] != "helper" {
		fmt.Fprintln(os.Stderr, "bad fake invocation")
		os.Exit(127)
	}
	result, err := helper.Run(args[2:], os.Stdin, os.Stdout)
	os.Exit(dhelper.Finish(os.Stderr, result, err))
}

func fakeClient(t *testing.T) *Client {
	t.Helper()
	self, err := os.Executable()
	require.NoError(t, err)
	return &Client{helper: &elevate.Client{Pkexec: self, Exe: "dux-edit", Env: []string{fakeEnv + "=1"}}}
}

func identityOf(t *testing.T, path string) string {
	t.Helper()
	var st unix.Stat_t
	require.NoError(t, unix.Stat(path, &st))
	return atomicfile.StatIdentity(&st)
}

func TestClientRoundTripsThroughTheHelper(t *testing.T) {
	root := t.TempDir()
	client := fakeClient(t)
	real := filepath.Join(root, "real.conf")
	require.NoError(t, os.WriteFile(real, []byte("one"), 0o644))
	link := filepath.Join(root, "link.conf")
	require.NoError(t, os.Symlink("real.conf", link))

	data, loaded, err := client.Read(link)
	require.NoError(t, err)
	assert.Equal(t, "one", string(data))
	assert.Equal(t, identityOf(t, real), loaded)

	big := bytes.Repeat([]byte("abc"), 1<<20)
	written, err := client.Replace(link, big, loaded)
	require.NoError(t, err)
	assert.Equal(t, identityOf(t, real), written, "the caller learns the identity without a stat of its own")
	got, err := os.ReadFile(real)
	require.NoError(t, err)
	assert.Equal(t, big, got)
	target, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, "real.conf", target, "the link survives; its target is written")

	stale := identityOf(t, real)
	require.NoError(t, os.WriteFile(real, []byte("theirs"), 0o644))
	_, err = client.Replace(link, []byte("mine"), stale)
	assert.ErrorIs(t, err, files.ErrConflict)

	_, err = client.Create(real, []byte("x"))
	assert.ErrorIs(t, err, fs.ErrExist)
	fresh := filepath.Join(root, "fresh")
	created, err := client.Create(fresh, nil)
	require.NoError(t, err)
	assert.Equal(t, identityOf(t, fresh), created)

	_, _, err = client.Read(filepath.Join(root, "missing"))
	assert.ErrorIs(t, err, fs.ErrNotExist)
}

func TestReadStopsPastTheLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big")
	require.NoError(t, os.WriteFile(path, make([]byte, maxRead+1), 0o644))
	_, _, err := fakeClient(t).Read(path)
	assert.ErrorIs(t, err, ErrTooLarge)
}
