package colorscheme

import (
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
)

func setting(namespace, key string, value any) *dbus.Signal {
	return &dbus.Signal{
		Name: settingsIface + ".SettingChanged",
		Body: []any{namespace, key, dbus.MakeVariant(value)},
	}
}

func TestPublishesOnlyColorSchemeChanges(t *testing.T) {
	var published []any
	w := &Watcher{scheme: "none", publish: func(topic string, data any) {
		assert.Equal(t, Topic, topic)
		published = append(published, data)
	}}

	w.handle(setting(appearanceNS, colorSchemeKey, uint32(1)))
	w.handle(setting(appearanceNS, colorSchemeKey, uint32(1)))
	w.handle(setting(appearanceNS, "accent-color", uint32(2)))
	w.handle(setting("org.gnome.desktop.interface", colorSchemeKey, uint32(2)))
	assert.Equal(t, "dark", w.Scheme())

	// Older portals wrap the value in a second variant.
	w.handle(setting(appearanceNS, colorSchemeKey, dbus.MakeVariant(uint32(2))))
	assert.Equal(t, "light", w.Scheme())

	assert.Equal(t, []any{map[string]any{"scheme": "dark"}, map[string]any{"scheme": "light"}}, published)
}
