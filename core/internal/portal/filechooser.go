// Package portal talks to xdg-desktop-portal: the file chooser dialogs.
package portal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	desktopDest  = "org.freedesktop.portal.Desktop"
	desktopPath  = "/org/freedesktop/portal/desktop"
	chooserIface = "org.freedesktop.portal.FileChooser"
	requestIface = "org.freedesktop.portal.Request"
)

var (
	ErrCanceled = errors.New("canceled")
	ErrNoPortal = errors.New("no file chooser portal")
)

type OpenOptions struct {
	Title    string
	Dir      string
	Multiple bool
}

type SaveOptions struct {
	Title string
	Dir   string
	Name  string
}

func OpenFiles(ctx context.Context, opt OpenOptions) ([]string, error) {
	options := map[string]dbus.Variant{"multiple": dbus.MakeVariant(opt.Multiple)}
	if opt.Dir != "" {
		options["current_folder"] = folder(opt.Dir)
	}
	uris, err := choose(ctx, "OpenFile", opt.Title, options)
	if err != nil {
		return nil, err
	}
	return localPaths(uris), nil
}

func SaveFile(ctx context.Context, opt SaveOptions) (string, error) {
	options := map[string]dbus.Variant{}
	if opt.Dir != "" {
		options["current_folder"] = folder(opt.Dir)
	}
	if opt.Name != "" {
		options["current_name"] = dbus.MakeVariant(opt.Name)
	}
	uris, err := choose(ctx, "SaveFile", opt.Title, options)
	if err != nil {
		return "", err
	}
	paths := localPaths(uris)
	if len(paths) == 0 {
		return "", errors.New("the file chooser returned no local file")
	}
	return paths[0], nil
}

// The portal wants a NUL-terminated byte string, not a D-Bus string.
func folder(dir string) dbus.Variant {
	return dbus.MakeVariant(append([]byte(dir), 0))
}

// A private connection per dialog: its signal match and the request die with it, nothing stays registered.
func choose(ctx context.Context, method, title string, options map[string]dbus.Variant) ([]string, error) {
	conn, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return nil, ErrNoPortal
	}
	defer conn.Close()

	token := newToken()
	options["handle_token"] = dbus.MakeVariant(token)
	request := requestPath(conn.Names()[0], token)

	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)
	// Subscribed before the call, so a dialog answered at once is not missed.
	if err := conn.AddMatchSignal(responseMatch(request)...); err != nil {
		return nil, ErrNoPortal
	}

	var handle dbus.ObjectPath
	call := conn.Object(desktopDest, desktopPath).CallWithContext(ctx, chooserIface+"."+method, 0, "", title, options)
	if err := call.Store(&handle); err != nil {
		return nil, callFailure(ctx, err)
	}
	// Portals older than 0.9 ignore handle_token.
	if handle != request {
		if err := conn.AddMatchSignal(responseMatch(handle)...); err != nil {
			return nil, err
		}
		request = handle
	}

	for {
		select {
		case sig := <-signals:
			if sig == nil || sig.Path != request || sig.Name != requestIface+".Response" || len(sig.Body) < 2 {
				continue
			}
			return response(sig.Body)
		case <-ctx.Done():
			closeRequest(conn, request)
			return nil, ctx.Err()
		}
	}
}

func responseMatch(path dbus.ObjectPath) []dbus.MatchOption {
	return []dbus.MatchOption{
		dbus.WithMatchInterface(requestIface),
		dbus.WithMatchMember("Response"),
		dbus.WithMatchObjectPath(path),
	}
}

func response(body []any) ([]string, error) {
	code, ok := body[0].(uint32)
	if !ok {
		return nil, errors.New("malformed file chooser response")
	}
	switch code {
	case 0:
	case 1:
		return nil, ErrCanceled
	default:
		return nil, fmt.Errorf("file chooser failed (code %d)", code)
	}
	results, _ := body[1].(map[string]dbus.Variant)
	uris, _ := results["uris"].Value().([]string)
	return uris, nil
}

func closeRequest(conn *dbus.Conn, request dbus.ObjectPath) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = conn.Object(desktopDest, request).CallWithContext(ctx, requestIface+".Close", 0).Err
}

func callFailure(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	switch errorName(err) {
	case "org.freedesktop.DBus.Error.ServiceUnknown",
		"org.freedesktop.DBus.Error.NameHasNoOwner",
		"org.freedesktop.DBus.Error.UnknownMethod",
		"org.freedesktop.DBus.Error.UnknownInterface",
		"org.freedesktop.DBus.Error.UnknownObject":
		return ErrNoPortal
	}
	return fmt.Errorf("file chooser: %w", err)
}

// godbus returns both dbus.Error and *dbus.Error.
func errorName(err error) string {
	var value dbus.Error
	if errors.As(err, &value) {
		return value.Name
	}
	var ptr *dbus.Error
	if errors.As(err, &ptr) {
		return ptr.Name
	}
	return ""
}

func newToken() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "duxedit" + hex.EncodeToString(b[:])
}

// The portal names the request after the caller's unique name: ":1.42" becomes "1_42".
func requestPath(sender, token string) dbus.ObjectPath {
	name := strings.ReplaceAll(strings.TrimPrefix(sender, ":"), ".", "_")
	return dbus.ObjectPath(desktopPath + "/request/" + name + "/" + token)
}

func localPaths(uris []string) []string {
	paths := make([]string, 0, len(uris))
	for _, raw := range uris {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "file" || u.Path == "" {
			continue
		}
		paths = append(paths, u.Path)
	}
	return paths
}
