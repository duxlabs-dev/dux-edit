// Package colorscheme follows the desktop portal's light/dark preference.
package colorscheme

import (
	"sync"

	"github.com/duxlabs-dev/dux-go/log"
	"github.com/godbus/dbus/v5"
)

const (
	Topic = "colorScheme"

	portalDest     = "org.freedesktop.portal.Desktop"
	portalPath     = "/org/freedesktop/portal/desktop"
	settingsIface  = "org.freedesktop.portal.Settings"
	appearanceNS   = "org.freedesktop.appearance"
	colorSchemeKey = "color-scheme"
)

type Watcher struct {
	publish func(topic string, data any)

	mu     sync.Mutex
	conn   *dbus.Conn
	closed bool
	scheme string
}

// New subscribes in the background, so a slow or missing portal never holds up startup.
func New(publish func(topic string, data any)) *Watcher {
	w := &Watcher{publish: publish, scheme: "none"}
	go w.run()
	return w
}

// Scheme is "dark", "light" or "none".
func (w *Watcher) Scheme() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.scheme
}

func (w *Watcher) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	if w.conn != nil {
		w.conn.Close()
		w.conn = nil
	}
}

func (w *Watcher) run() {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Debugf("color scheme: no session bus: %v", err)
		return
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		conn.Close()
		return
	}
	w.conn = conn
	w.mu.Unlock()

	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)
	// arg0 narrows the match to the appearance namespace, so other portal settings never wake us.
	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(portalPath),
		dbus.WithMatchInterface(settingsIface),
		dbus.WithMatchMember("SettingChanged"),
		dbus.WithMatchArg(0, appearanceNS),
	); err != nil {
		w.Close()
		return
	}

	var value dbus.Variant
	if err := conn.Object(portalDest, portalPath).Call(settingsIface+".ReadOne", 0, appearanceNS, colorSchemeKey).Store(&value); err != nil {
		log.Debugf("color scheme: portal unavailable: %v", err)
		w.Close()
		return
	}
	w.update(value)

	for sig := range signals {
		w.handle(sig)
	}
}

func (w *Watcher) handle(sig *dbus.Signal) {
	if sig.Name != settingsIface+".SettingChanged" || len(sig.Body) < 3 {
		return
	}
	namespace, _ := sig.Body[0].(string)
	key, _ := sig.Body[1].(string)
	value, ok := sig.Body[2].(dbus.Variant)
	if namespace != appearanceNS || key != colorSchemeKey || !ok {
		return
	}
	w.update(value)
}

func (w *Watcher) update(value dbus.Variant) {
	scheme := schemeName(value)
	w.mu.Lock()
	changed := scheme != w.scheme
	w.scheme = scheme
	w.mu.Unlock()
	if changed && w.publish != nil {
		w.publish(Topic, map[string]any{"scheme": scheme})
	}
}

func schemeName(value dbus.Variant) string {
	switch v := value.Value().(type) {
	case dbus.Variant:
		return schemeName(v)
	case uint32:
		switch v {
		case 1:
			return "dark"
		case 2:
			return "light"
		}
	}
	return "none"
}
