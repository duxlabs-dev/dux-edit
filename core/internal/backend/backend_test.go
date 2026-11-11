package backend

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startBackend(t *testing.T) *Backend {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(dir, "no-bus"))
	b, err := Start(context.Background(), "dux-edit", "test")
	require.NoError(t, err)
	t.Cleanup(b.Close)
	return b
}

func request(t *testing.T, b *Backend, method string, params map[string]any) map[string]any {
	t.Helper()
	client, err := ipc.Dial(b.SocketPath())
	require.NoError(t, err)
	defer client.Close()

	resp, err := client.Call(ipc.Request{ID: 1, Method: method, Params: params})
	require.NoError(t, err)
	require.Empty(t, resp.Error)
	result, ok := (*resp.Result).(map[string]any)
	require.True(t, ok)
	return result
}

func TestEditsReachTheMirrorInVersionOrder(t *testing.T) {
	b := startBackend(t)
	path := filepath.Join(t.TempDir(), "note.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o644))

	opened := request(t, b, "doc.open", map[string]any{"path": path})
	require.Equal(t, "hello", opened["text"])
	docID := opened["docId"]

	// Version 2 arrives first and must wait for version 1.
	request(t, b, "doc.edit", map[string]any{"docId": docID, "version": 2, "start": 6, "removed": 0, "text": "?", "length": 7})
	request(t, b, "doc.edit", map[string]any{"docId": docID, "version": 1, "start": 5, "removed": 0, "text": "!", "length": 6})

	text := request(t, b, "doc.text", map[string]any{"docId": docID, "version": 2})
	assert.Equal(t, "hello!?", text["text"])
}

func TestEveryRouteIsDescribed(t *testing.T) {
	b := startBackend(t)
	for _, spec := range Methods {
		// These leave the sandbox: a signal to the test process, a portal dialog, the user's DMS files.
		if slices.Contains([]string{"subscribe", "ui.quit", "dialog.openFiles", "dialog.saveFile", "import.dms"}, spec.Name) {
			continue
		}
		client, err := ipc.Dial(b.SocketPath())
		require.NoError(t, err)
		resp, err := client.Call(ipc.Request{ID: 1, Method: spec.Name})
		client.Close()
		require.NoError(t, err)
		assert.NotContains(t, resp.Error, "unknown method", spec.Name)
	}
}
