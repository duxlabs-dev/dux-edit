package backend

import (
	"context"
	"encoding/json"
	"os"
	"syscall"

	"github.com/duxlabs-dev/dux-go/ipc/params"
)

type OpenPosition struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

func (b *Backend) uiAction(action string) any {
	b.pending.Deliver(b.bus, map[string]any{"action": action})
	return ok()
}

func (b *Backend) uiShow(context.Context, map[string]any) any { return b.uiAction("show") }

func (b *Backend) uiHide(context.Context, map[string]any) any { return b.uiAction("hide") }

func (b *Backend) uiToggle(context.Context, map[string]any) any { return b.uiAction("toggle") }

func (b *Backend) uiOpen(_ context.Context, p map[string]any) any {
	paths := params.StringSlice(p, "paths")
	payload := map[string]any{
		"action": "open",
		"paths":  paths,
		"line":   params.IntOpt(p, "line", 0),
		"column": params.IntOpt(p, "column", 0),
		"simple": params.BoolOpt(p, "simple", false),
	}
	if raw := p["positions"]; raw != nil {
		encoded, err := json.Marshal(raw)
		if err != nil {
			return fail("INVALID", "positions must be an array of line/column objects")
		}
		var positions []*OpenPosition
		if json.Unmarshal(encoded, &positions) != nil || len(positions) != len(paths) {
			return fail("INVALID", "positions must contain one line/column object per path")
		}
		for _, position := range positions {
			if position == nil || position.Line < 0 || position.Column < 0 {
				return fail("INVALID", "positions must use nonnegative integer lines and columns")
			}
		}
		payload["positions"] = positions
	}
	b.pending.Deliver(b.bus, payload)
	return ok()
}

func (b *Backend) uiQuit(context.Context, map[string]any) any {
	b.docs.Flush()
	// The reply is written before the signal is handled, since handle() responds after we return.
	go func() { _ = syscall.Kill(os.Getpid(), syscall.SIGTERM) }()
	return ok()
}
