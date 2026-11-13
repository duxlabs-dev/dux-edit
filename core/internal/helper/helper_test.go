package helper

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/duxlabs-dev/dux-edit/core/internal/atomicfile"
)

func runOp(t *testing.T, stdin []byte, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	_, err := Run(args, bytes.NewReader(stdin), &out)
	return out.String(), err
}

func frames(chunks ...string) []byte {
	var buf bytes.Buffer
	for _, chunk := range chunks {
		buf.Write(binary.BigEndian.AppendUint32(nil, uint32(len(chunk))))
		buf.WriteString(chunk)
	}
	return buf.Bytes()
}

func committed(chunks ...string) []byte {
	return append(frames(chunks...), 0, 0, 0, 0)
}

func identityOf(t *testing.T, path string) string {
	t.Helper()
	var st unix.Stat_t
	require.NoError(t, unix.Stat(path, &st))
	return atomicfile.StatIdentity(&st)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestRunRefusesBadArguments(t *testing.T) {
	root := t.TempDir()
	cases := map[string][]string{
		"no operation":      {},
		"unknown op":        {"chmod", root},
		"relative path":     {OpRead, "relative/a"},
		"unclean path":      {OpRead, root + "//a"},
		"dotdot path":       {OpRead, root + "/a/.."},
		"root path":         {OpWrite, "/"},
		"nul in path":       {OpRead, root + "/a\x00b"},
		"too many paths":    {OpRead, root, root},
		"write arity":       {OpWrite, root + "/f", "644"},
		"replace arity":     {OpReplace, root + "/f"},
		"identity garbage":  {OpReplace, root + "/f", "1:2:3"},
		"identity not nums": {OpReplace, root + "/f", "a:b:c:d"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := runOp(t, committed("x"), args...)
			require.Error(t, err)
			assert.Equal(t, CodeInvalid, CodeOf(err), err.Error())
		})
	}
}

func TestSymlinksAreRefused(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	require.NoError(t, os.Mkdir(real, 0o755))
	target := filepath.Join(real, "f")
	require.NoError(t, os.WriteFile(target, []byte("x"), 0o644))
	require.NoError(t, os.Symlink(real, filepath.Join(root, "dirlink")))
	require.NoError(t, os.Symlink(target, filepath.Join(root, "leaf")))
	identity := identityOf(t, target)

	for name, args := range map[string][]string{
		"read a symlink leaf":        {OpRead, filepath.Join(root, "leaf")},
		"replace a symlink leaf":     {OpReplace, filepath.Join(root, "leaf"), ""},
		"replace through a dir link": {OpReplace, filepath.Join(root, "dirlink", "f"), identity},
		"create through a dir link":  {OpWrite, filepath.Join(root, "dirlink", "new")},
		"create over a symlink":      {OpWrite, filepath.Join(root, "leaf")},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := runOp(t, committed("data"), args...)
			require.Error(t, err)
			assert.NotEqual(t, CodeIO, CodeOf(err), err.Error())
		})
	}
	assert.Equal(t, "x", readFile(t, target))
	assert.NoFileExists(t, filepath.Join(real, "new"))
}

func TestReadStreamsRegularFilesOnly(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "f")
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte("z"), 70000), 0o600))

	out, err := runOp(t, nil, OpRead, path)
	require.NoError(t, err)
	assert.Len(t, out, 70000)

	fifo := filepath.Join(root, "fifo")
	require.NoError(t, unix.Mkfifo(fifo, 0o600))
	_, err = runOp(t, nil, OpRead, fifo)
	assert.Equal(t, CodeInvalid, CodeOf(err))

	_, err = runOp(t, nil, OpRead, filepath.Join(root, "missing"))
	assert.Equal(t, CodeNotFound, CodeOf(err))
}

func TestWriteCreatesOnlyNewFilesAndDropsAbortedOnes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "out")

	_, err := runOp(t, committed("hello ", "world"), OpWrite, path)
	require.NoError(t, err)
	assert.Equal(t, "hello world", readFile(t, path))

	_, err = runOp(t, committed("again"), OpWrite, path)
	assert.Equal(t, CodeExists, CodeOf(err))
	assert.Equal(t, "hello world", readFile(t, path))

	aborted := filepath.Join(root, "aborted")
	_, err = runOp(t, frames("partial"), OpWrite, aborted)
	require.Error(t, err)
	assert.NoFileExists(t, aborted)
}

func TestReplaceChecksTheIdentity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "f")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o640))
	stale := identityOf(t, path)
	require.NoError(t, os.WriteFile(path, []byte("changed elsewhere"), 0o640))

	_, err := runOp(t, committed("mine"), OpReplace, path, stale)
	require.ErrorIs(t, err, ErrConflict)
	assert.Equal(t, CodeConflict, CodeOf(err))
	assert.Equal(t, "changed elsewhere", readFile(t, path))

	_, err = runOp(t, committed("mine"), OpReplace, filepath.Join(root, "gone"), stale)
	assert.Equal(t, CodeConflict, CodeOf(err), "a deleted file is a conflict too")

	_, err = runOp(t, committed("forced"), OpReplace, path, "")
	require.NoError(t, err, "an empty identity overwrites whatever is there")
	assert.Equal(t, "forced", readFile(t, path))
}

func TestReplaceKeepsModeAndLeavesNoTempBehind(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "f")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o600))
	require.NoError(t, os.Chmod(path, 0o640))
	before := identityOf(t, path)

	_, err := runOp(t, frames("half"), OpReplace, path, before)
	require.Error(t, err)
	assert.Equal(t, "old", readFile(t, path), "an uncommitted replace changes nothing")

	_, err = runOp(t, committed("new ", "text"), OpReplace, path, before)
	require.NoError(t, err)
	assert.Equal(t, "new text", readFile(t, path))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())

	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "the temp file is renamed or removed")
}

func TestReplaceWritesHardLinkedFilesInPlace(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "f")
	link := filepath.Join(root, "g")
	require.NoError(t, os.WriteFile(path, []byte("shared"), 0o644))
	require.NoError(t, os.Link(path, link))

	_, err := runOp(t, frames("half"), OpReplace, path, identityOf(t, path))
	require.Error(t, err)
	assert.Equal(t, "shared", readFile(t, link), "an aborted in-place save never truncates")

	_, err = runOp(t, committed("both"), OpReplace, path, identityOf(t, path))
	require.NoError(t, err)
	assert.Equal(t, "both", readFile(t, link))
}

func TestMainRefusesOutsidePkexec(t *testing.T) {
	t.Setenv("PKEXEC_UID", "")
	var stderr bytes.Buffer
	assert.Equal(t, ExitRefused, Main([]string{OpRead, "/etc/hostname"}, nil, nil, &stderr))
	assert.Contains(t, stderr.String(), "pkexec")

	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	t.Setenv("PKEXEC_UID", "1000")
	stderr.Reset()
	assert.Equal(t, ExitRefused, Main([]string{OpRead, "/etc/hostname"}, nil, nil, &stderr))
	assert.Contains(t, stderr.String(), "root")
}

func TestWriteAndReplaceAnnounceReadyOnlyOnceTheFileChecksOut(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "f")
	out, err := runOp(t, committed("x"), OpWrite, path)
	require.NoError(t, err)
	assert.Equal(t, Ready+"\n", out)

	out, err = runOp(t, committed("x"), OpWrite, path)
	require.Error(t, err)
	assert.Empty(t, out, "nothing is announced when the create fails")

	stale := identityOf(t, path)
	require.NoError(t, os.WriteFile(path, []byte("changed"), 0o644))
	out, err = runOp(t, committed("y"), OpReplace, path, stale)
	require.ErrorIs(t, err, ErrConflict)
	assert.Empty(t, out)

	out, err = runOp(t, committed("y"), OpReplace, path, identityOf(t, path))
	require.NoError(t, err)
	assert.Equal(t, Ready+"\n", out)
}

// RLIMIT_FSIZE stands in for a full disk: the reservation fails before a byte of the file changes.
func TestInPlaceReplaceKeepsTheFileWhenSpaceRunsOut(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "f")
	original := strings.Repeat("o", 64)
	require.NoError(t, os.WriteFile(path, []byte(original), 0o644))
	require.NoError(t, os.Link(path, filepath.Join(root, "g")))
	identity := identityOf(t, path)

	var limit unix.Rlimit
	require.NoError(t, unix.Getrlimit(unix.RLIMIT_FSIZE, &limit))
	lowered := limit
	lowered.Cur = 128
	require.NoError(t, unix.Setrlimit(unix.RLIMIT_FSIZE, &lowered))
	probe, err := os.Create(filepath.Join(t.TempDir(), "probe"))
	require.NoError(t, err)
	reserveErr := unix.Fallocate(int(probe.Fd()), unix.FALLOC_FL_KEEP_SIZE, 0, 256)
	probe.Close()
	_, err = runOp(t, committed(strings.Repeat("n", 256)), OpReplace, path, identity)
	require.NoError(t, unix.Setrlimit(unix.RLIMIT_FSIZE, &limit))
	if !errors.Is(reserveErr, unix.EFBIG) {
		t.Skip("this filesystem does not apply the file size limit to fallocate")
	}

	require.ErrorIs(t, err, unix.EFBIG)
	assert.Equal(t, original, readFile(t, path), "the file is never cut before the new bytes fit")
}
