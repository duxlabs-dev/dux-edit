package backend

import (
	"context"
	"errors"

	"github.com/godbus/dbus/v5"

	"github.com/duxlabs-dev/dux-edit/core/internal/dmsimport"
	"github.com/duxlabs-dev/dux-edit/core/internal/paths"
	"github.com/duxlabs-dev/dux-go/ipc/params"
	"github.com/duxlabs-dev/dux-go/portal"
	"github.com/duxlabs-dev/dux-go/wayland/blur"
)

func dialogFailure(err error) any {
	switch {
	case errors.Is(err, portal.ErrCancelled):
		return fail("CANCELED", "")
	case errors.Is(err, portal.ErrNoPortal):
		return fail("NOPORTAL", "")
	}
	return fail("EIO", err.Error())
}

// One private connection per dialog, so nothing stays registered.
func dialogConn(ctx context.Context) (*dbus.Conn, error) {
	conn, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return nil, portal.ErrNoPortal
	}
	return conn, nil
}

func (b *Backend) dialogOpenFiles(ctx context.Context, p map[string]any) any {
	conn, err := dialogConn(ctx)
	if err != nil {
		return dialogFailure(err)
	}
	defer conn.Close()
	found, err := portal.OpenFile(ctx, conn, portal.OpenFileOptions{
		Title:    params.StringOpt(p, "title", ""),
		Dir:      params.StringOpt(p, "dir", ""),
		Multiple: params.BoolOpt(p, "multiple", true),
	})
	if err != nil {
		return dialogFailure(err)
	}
	return map[string]any{"ok": true, "paths": found}
}

func (b *Backend) dialogSaveFile(ctx context.Context, p map[string]any) any {
	conn, err := dialogConn(ctx)
	if err != nil {
		return dialogFailure(err)
	}
	defer conn.Close()
	path, err := portal.SaveFile(ctx, conn, portal.SaveFileOptions{
		Title: params.StringOpt(p, "title", ""),
		Dir:   params.StringOpt(p, "dir", ""),
		Name:  params.StringOpt(p, "name", ""),
	})
	if err != nil {
		return dialogFailure(err)
	}
	return map[string]any{"ok": true, "path": path}
}

func (b *Backend) systemColorScheme(context.Context, map[string]any) any {
	return schemePayload(b.scheme.Scheme())
}

// The UI reads the scheme name, not colorscheme.State.
func schemePayload(name string) map[string]any { return map[string]any{"scheme": name} }

func (b *Backend) systemElevation(context.Context, map[string]any) any {
	state := b.docs.Elevation()
	if state == "available" {
		return map[string]any{"available": true}
	}
	return map[string]any{"available": false, "reason": state}
}

func (b *Backend) systemBlur(context.Context, map[string]any) any {
	supported, err := blur.Supported()
	if err != nil {
		return fail("EIO", err.Error())
	}
	return map[string]any{"ok": true, "supported": supported}
}

// With apply the result also reaches the UI as a "ui" import action, which is how the CLI command lands.
func (b *Backend) importDMS(_ context.Context, p map[string]any) any {
	dryRun := params.BoolOpt(p, "dryRun", false)
	swapDir := ""
	if !dryRun {
		dir, err := paths.SwapDir()
		if err != nil {
			return fail("EIO", err.Error())
		}
		swapDir = dir
	}
	state, config := dmsimport.Dirs()
	result, err := dmsimport.Run(state, config, swapDir, dryRun)
	if err != nil {
		return fail("EIO", err.Error())
	}
	if !dryRun && result.Found && params.BoolOpt(p, "apply", false) {
		b.pending.Deliver(b.bus, map[string]any{
			"action":   "import",
			"tabs":     result.Tabs,
			"current":  result.Current,
			"settings": result.Settings,
		})
	}
	return result
}
