package backend

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUIOpenKeepsPerFilePositions(t *testing.T) {
	b := &Backend{bus: ipc.NewEventBus()}
	positions := []any{map[string]any{"line": float64(3)}, map[string]any{"line": float64(9), "column": float64(2)}}
	result := b.uiOpen(context.Background(), map[string]any{
		"paths": []any{"/a", "/b"}, "line": 3, "positions": positions,
	}).(map[string]any)
	require.Equal(t, true, result["ok"])
	require.Len(t, b.pending.payloads, 1)
	payload, err := json.Marshal(b.pending.payloads[0]["positions"])
	require.NoError(t, err)
	assert.JSONEq(t, `[{"line":3,"column":0},{"line":9,"column":2}]`, string(payload))
	assert.Equal(t, 3, b.pending.payloads[0]["line"])
}

func TestUIOpenRejectsInvalidPerFilePositions(t *testing.T) {
	cases := []any{
		"3", []any{}, []any{nil}, []any{map[string]any{"line": -1}},
		[]any{map[string]any{"line": 1.5}}, []any{map[string]any{"line": "3"}},
		[]any{map[string]any{"column": -1}},
	}

	for _, positions := range cases {
		b := &Backend{bus: ipc.NewEventBus()}
		result := b.uiOpen(context.Background(), map[string]any{
			"paths": []any{"/a"}, "positions": positions,
		}).(map[string]any)
		assert.Equal(t, false, result["ok"], "%#v", positions)
		assert.Equal(t, "INVALID", result["code"])
		assert.Empty(t, b.pending.payloads)
	}
}

func TestUIOpenEnvironmentUsesTheSamePositionValidation(t *testing.T) {
	cases := []struct {
		name, payload string
		valid         bool
	}{
		{"per-file positions", `{"paths":["/a","/b"],"line":3,"positions":[{"line":3},{"line":9,"column":2}]}`, true},
		{"legacy first-file position", `{"paths":["/a"],"line":3,"column":2}`, true},
		{"invalid position", `{"paths":["/a"],"positions":[{"line":-1}]}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(OpenEnv, tc.payload)
			b := &Backend{bus: ipc.NewEventBus()}
			b.takeOpenEnv()
			assert.Empty(t, os.Getenv(OpenEnv))
			if !tc.valid {
				assert.Empty(t, b.pending.payloads)
				return
			}
			require.Len(t, b.pending.payloads, 1)
			assert.Equal(t, "open", b.pending.payloads[0]["action"])
			assert.Equal(t, 3, b.pending.payloads[0]["line"])
		})
	}
}
