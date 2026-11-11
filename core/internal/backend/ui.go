package backend

import (
	"context"
	"os"
	"syscall"

	"github.com/duxlabs-dev/dux-go/ipc/params"
)

func (b *Backend) uiAction(action string) any {
	b.pending.Deliver(b.bus, map[string]any{"action": action})
	return ok()
}

func (b *Backend) uiShow(context.Context, map[string]any) any { return b.uiAction("show") }

func (b *Backend) uiHide(context.Context, map[string]any) any { return b.uiAction("hide") }

func (b *Backend) uiToggle(context.Context, map[string]any) any { return b.uiAction("toggle") }

func (b *Backend) uiOpen(_ context.Context, p map[string]any) any {
	b.pending.Deliver(b.bus, map[string]any{
		"action": "open",
		"paths":  params.StringSlice(p, "paths"),
		"line":   params.IntOpt(p, "line", 0),
		"column": params.IntOpt(p, "column", 0),
		"simple": params.BoolOpt(p, "simple", false),
	})
	return ok()
}

func (b *Backend) uiQuit(context.Context, map[string]any) any {
	b.docs.Flush()
	// The reply is written before the signal is handled, since handle() responds after we return.
	go func() { _ = syscall.Kill(os.Getpid(), syscall.SIGTERM) }()
	return ok()
}
