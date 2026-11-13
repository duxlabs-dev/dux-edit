package elevate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/duxlabs-dev/dux-edit/core/internal/atomicfile"
	"github.com/duxlabs-dev/dux-edit/core/internal/helper"
)

const (
	fakeEnv     = "DUX_EDIT_ELEVATE_FAKE_HELPER"
	fakeExitEnv = "DUX_EDIT_ELEVATE_FAKE_EXIT"
	// The fake helper sleeps this long before it starts, like a password prompt, or after it is done.
	fakePromptEnv = "DUX_EDIT_ELEVATE_FAKE_PROMPT"
	fakeStuckEnv  = "DUX_EDIT_ELEVATE_FAKE_STUCK"
)

// The test binary stands in for pkexec: "<binary> <exe> helper <op> ..." runs the operation unprivileged.
func TestMain(m *testing.M) {
	if code := os.Getenv(fakeExitEnv); code != "" {
		n, _ := strconv.Atoi(code)
		os.Exit(n)
	}
	if os.Getenv(fakeEnv) == "" {
		os.Exit(m.Run())
	}
	args := os.Args[1:]
	if len(args) < 2 || args[1] != "helper" {
		fmt.Fprintln(os.Stderr, "bad fake invocation")
		os.Exit(127)
	}
	if pause, err := time.ParseDuration(os.Getenv(fakePromptEnv)); err == nil {
		time.Sleep(pause)
	}
	identity, err := helper.Run(args[2:], os.Stdin, os.Stdout)
	if pause, err := time.ParseDuration(os.Getenv(fakeStuckEnv)); err == nil {
		time.Sleep(pause)
	}
	if err != nil {
		line, _ := json.Marshal(helper.Failure{Code: string(helper.CodeOf(err)), Error: err.Error()})
		fmt.Fprintln(os.Stderr, string(line))
		os.Exit(helper.ExitFailed)
	}
	line, _ := json.Marshal(helper.Success{Identity: identity})
	fmt.Fprintln(os.Stderr, string(line))
	os.Exit(0)
}

func fakeClient(t *testing.T) *Client {
	t.Helper()
	self, err := os.Executable()
	require.NoError(t, err)
	return &Client{pkexec: self, exe: "dux-edit", env: []string{fakeEnv + "=1"}}
}

func identityOf(t *testing.T, path string) string {
	t.Helper()
	var st unix.Stat_t
	require.NoError(t, unix.Stat(path, &st))
	return atomicfile.StatIdentity(&st)
}

func TestPolicyCoversOnlyThisBinary(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "bin", "dux-edit")
	require.NoError(t, os.MkdirAll(filepath.Dir(exe), 0o755))
	require.NoError(t, os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755))
	link := filepath.Join(dir, "dedit")
	require.NoError(t, os.Symlink(exe, link))

	actions := filepath.Join(dir, "actions")
	require.NoError(t, os.Mkdir(actions, 0o755))
	policy := `<policyconfig><action id="` + ActionID + `"><annotate key="org.freedesktop.policykit.exec.path">` + link + `</annotate></action></policyconfig>`
	require.NoError(t, os.WriteFile(filepath.Join(actions, PolicyName), []byte(policy), 0o644))

	assert.True(t, policyCovers(exe, []string{"/nonexistent", actions}), "the annotation is resolved through symlinks")
	assert.False(t, policyCovers(filepath.Join(dir, "other"), []string{actions}))
	assert.False(t, policyCovers(exe, []string{"/nonexistent"}))
	assert.Equal(t, "", annotatedPath("<policyconfig/>"))
}

func TestFailureMapsExitCodesAndHelperReports(t *testing.T) {
	self, err := os.Executable()
	require.NoError(t, err)
	exitWith := func(code int) error {
		cmd := exec.Command(self)
		cmd.Env = append(os.Environ(), fakeExitEnv+"="+strconv.Itoa(code))
		return cmd.Run()
	}

	err = failure(exitWith(126), nil)
	assert.ErrorIs(t, err, fs.ErrPermission)
	assert.Contains(t, err.Error(), "not granted")

	err = failure(exitWith(127), []byte("Error executing command as another user: Not authorized\n"))
	assert.ErrorIs(t, err, fs.ErrPermission)

	err = failure(exitWith(127), []byte("Error executing command as another user: No authentication agent\n"))
	assert.Equal(t, helper.CodeIO, helper.CodeOf(err))
	assert.Contains(t, err.Error(), "No authentication agent")

	err = failure(exitWith(1), []byte("noise\n{\"code\":\"CONFLICT\",\"error\":\"the file changed\"}\n"))
	assert.ErrorIs(t, err, ErrConflict)
	assert.Equal(t, "the file changed", err.Error())

	err = failure(errors.New("fork failed"), nil)
	assert.Equal(t, helper.CodeIO, helper.CodeOf(err))
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

	big := bytes.Repeat([]byte("abc"), frameSize)
	written, err := client.Replace(link, big, loaded)
	require.NoError(t, err)
	assert.Equal(t, identityOf(t, real), written, "the caller learns the identity without a stat of its own")
	got, err := os.ReadFile(real)
	require.NoError(t, err)
	assert.Equal(t, big, got, "several frames arrive in order")
	target, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, "real.conf", target, "the link survives; its target is written")

	stale := identityOf(t, real)
	require.NoError(t, os.WriteFile(real, []byte("theirs"), 0o644))
	_, err = client.Replace(link, []byte("mine"), stale)
	assert.ErrorIs(t, err, ErrConflict)

	_, err = client.Create(real, []byte("x"))
	assert.ErrorIs(t, err, fs.ErrExist)
	fresh := filepath.Join(root, "fresh")
	created, err := client.Create(fresh, nil)
	require.NoError(t, err)
	assert.Equal(t, identityOf(t, fresh), created)

	_, _, err = client.Read(filepath.Join(root, "missing"))
	assert.ErrorIs(t, err, fs.ErrNotExist)
}

func TestStuckHelperIsNotWaitedOnForever(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o644))

	waitAfterEOF = time.Second
	t.Cleanup(func() { waitAfterEOF = 30 * time.Second })

	client := fakeClient(t)
	client.env = append(client.env, fakePromptEnv+"=1500ms")
	_, err := client.Replace(path, []byte("typed slowly"), "")
	require.NoError(t, err, "the time spent before the helper is ready is not bounded")

	client = fakeClient(t)
	client.env = append(client.env, fakeStuckEnv+"=4s")
	done := make(chan error, 1)
	go func() {
		_, err := client.Replace(path, []byte("new"), "")
		done <- err
	}()
	select {
	case err := <-done:
		require.Error(t, err)
		assert.Equal(t, helper.CodeIO, helper.CodeOf(err))
	case <-time.After(3 * time.Second):
		t.Fatal("waited on a helper that never exits")
	}
}
