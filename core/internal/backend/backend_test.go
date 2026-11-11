package backend

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startBackend(t *testing.T) *Backend {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	b, err := Start(context.Background(), "dux-edit", "test")
	require.NoError(t, err)
	t.Cleanup(b.Close)
	return b
}

func call(t *testing.T, b *Backend, method string, params map[string]any) map[string]any {
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

func TestCounterAddAndReset(t *testing.T) {
	b := startBackend(t)

	assert.Equal(t, float64(1), call(t, b, "counter.add", nil)["value"])
	assert.Equal(t, float64(6), call(t, b, "counter.add", map[string]any{"amount": 5})["value"])
	assert.Equal(t, float64(6), call(t, b, "counter.get", nil)["value"])
	assert.Equal(t, float64(0), call(t, b, "counter.reset", nil)["value"])
}

type counterEvent struct {
	Event string `json:"event"`
	Data  struct {
		Value int `json:"value"`
	} `json:"data"`
}

func nextCounterEvent(t *testing.T, lines *bufio.Scanner) int {
	t.Helper()
	require.True(t, lines.Scan(), "counter event")
	var event counterEvent
	require.NoError(t, json.Unmarshal(lines.Bytes(), &event))
	require.Equal(t, "counter", event.Event)
	return event.Data.Value
}

func TestSubscriberGetsCurrentValueThenChanges(t *testing.T) {
	b := startBackend(t)
	call(t, b, "counter.add", map[string]any{"amount": 2})

	conn, err := net.Dial("unix", b.SocketPath())
	require.NoError(t, err)
	defer conn.Close()
	lines := bufio.NewScanner(conn)
	require.True(t, lines.Scan(), "capabilities line")

	_, err = conn.Write([]byte(`{"id":1,"method":"subscribe","params":{"topics":["counter"]}}` + "\n"))
	require.NoError(t, err)
	require.True(t, lines.Scan(), "subscribe response")
	assert.Equal(t, 2, nextCounterEvent(t, lines))

	call(t, b, "counter.add", map[string]any{"amount": 3})
	assert.Equal(t, 5, nextCounterEvent(t, lines))
}

func TestUnknownMethodIsAnError(t *testing.T) {
	b := startBackend(t)

	client, err := ipc.Dial(b.SocketPath())
	require.NoError(t, err)
	defer client.Close()

	resp, err := client.Call(ipc.Request{ID: 1, Method: "nope"})
	require.NoError(t, err)
	assert.Contains(t, resp.Error, "unknown method")
}
