package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeManifest(t *testing.T, root, dir, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, dir, "plugin.json"), []byte(body), 0o644))
}

func TestScan(t *testing.T) {
	user, system := t.TempDir(), t.TempDir()
	writeManifest(t, user, "a", `{"id":"wordCount","name":"Word count","components":{"editor":"./Main.qml"},"settings":"./Settings.qml"}`)
	writeManifest(t, user, "b", `{"id":"typed","type":"editor","component":"./Typed.qml"}`)
	writeManifest(t, user, "c", `{"id":"barOnly","type":"widget","component":"./Widget.qml"}`)
	writeManifest(t, user, "d", `{not json`)
	writeManifest(t, user, "e", `{"id":"escape","components":{"editor":"../../evil.qml"}}`)
	writeManifest(t, user, "f", `{"id":"dankNotepadModule","type":"daemon","component":"./X.qml"}`)
	writeManifest(t, user, "g", `{"id":"../x","components":{"editor":"./Main.qml"}}`)
	writeManifest(t, system, "a", `{"id":"wordCount","components":{"editor":"./Other.qml"}}`)
	writeManifest(t, system, "h", `{"id":"absolute","components":{"editor":"/etc/x.qml"}}`)

	byID := map[string]Manifest{}
	for _, m := range Scan([]string{user, system, filepath.Join(user, "missing")}) {
		assert.NotContains(t, byID, m.ID, "duplicate id %s", m.ID)
		byID[m.ID] = m
	}

	assert.Equal(t, Manifest{
		ID: "wordCount", Name: "Word count", Dir: filepath.Join(user, "a"), Editor: "Main.qml", Settings: "Settings.qml",
		Permissions: []string{},
	}, byID["wordCount"])
	assert.Equal(t, "Typed.qml", byID["typed"].Editor)
	assert.Empty(t, byID["typed"].Error)
	assert.Equal(t, "no editor surface", byID["barOnly"].Error)
	assert.Contains(t, byID["d"].Error, "invalid plugin.json")
	assert.Equal(t, errEscapes.Error(), byID["escape"].Error)
	assert.Equal(t, errEscapes.Error(), byID["absolute"].Error)
	assert.Equal(t, "built in", byID["dankNotepadModule"].Error)
	assert.Equal(t, "invalid id", byID["g"].Error)
}
