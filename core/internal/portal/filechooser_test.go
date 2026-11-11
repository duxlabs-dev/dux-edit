package portal

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestPathFollowsTheSenderName(t *testing.T) {
	assert.Equal(t, dbus.ObjectPath("/org/freedesktop/portal/desktop/request/1_42/duxeditab"), requestPath(":1.42", "duxeditab"))
}

func TestResponseCodesAndURIs(t *testing.T) {
	results := map[string]dbus.Variant{"uris": dbus.MakeVariant([]string{
		"file:///home/me/My%20Notes/a%23b.md",
		"https://example.org/remote.txt",
	})}
	uris, err := response([]any{uint32(0), results})
	require.NoError(t, err)
	assert.Equal(t, []string{"/home/me/My Notes/a#b.md"}, localPaths(uris))

	_, err = response([]any{uint32(1), map[string]dbus.Variant{}})
	assert.ErrorIs(t, err, ErrCanceled)

	_, err = response([]any{uint32(2), map[string]dbus.Variant{}})
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrCanceled)
}

func TestNoSessionBusIsNoPortal(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "no-bus"))
	_, err := OpenFiles(context.Background(), OpenOptions{Title: "Open"})
	assert.ErrorIs(t, err, ErrNoPortal)
	_, err = SaveFile(context.Background(), SaveOptions{Name: "a.txt"})
	assert.ErrorIs(t, err, ErrNoPortal)
}
