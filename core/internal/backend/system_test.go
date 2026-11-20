package backend

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDialogsWithoutASessionBusAnswerNoPortal(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "no-bus"))
	b := &Backend{}
	for _, result := range []any{
		b.dialogOpenFiles(context.Background(), map[string]any{"title": "Open"}),
		b.dialogSaveFile(context.Background(), map[string]any{"name": "a.txt"}),
	} {
		assert.Equal(t, false, result.(map[string]any)["ok"])
		assert.Equal(t, "NOPORTAL", result.(map[string]any)["code"])
	}
}
