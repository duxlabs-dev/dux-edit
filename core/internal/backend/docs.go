package backend

import (
	"context"
	"encoding/json"
	"time"

	"github.com/duxlabs-dev/dux-edit/core/internal/buffer"
	"github.com/duxlabs-dev/dux-edit/core/internal/document"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

// How long a request tagged with a version waits for the edits before it to arrive.
const awaitEdits = 2 * time.Second

// Copy under the lock; run fn after unlocking so slow readers do not block edits.
func (b *Backend) withText(ctx context.Context, p map[string]any, fn func(text string, version int64) any) any {
	return b.withTextUnless(ctx, p, nil, fn)
}

// withTextUnless answers SUPERSEDED instead of copying the text when superseded reports a newer request.
func (b *Backend) withTextUnless(ctx context.Context, p map[string]any, superseded func() bool, fn func(text string, version int64) any) any {
	d, found := b.docs.Get(params.StringOpt(p, "docId", ""))
	if !found {
		return fail(document.CodeUnknownDoc, "")
	}
	ctx, cancel := context.WithTimeout(ctx, awaitEdits)
	defer cancel()
	if d.Await(ctx, int64(params.IntOpt(p, "version", 0))) != nil {
		return fail(document.CodeDesync, "")
	}
	if superseded != nil && superseded() {
		return fail("SUPERSEDED", "")
	}
	var text string
	var version int64
	d.Read(func(buf *buffer.Buffer, v int64) { text, version = buf.Text(), v })
	return fn(text, version)
}

func (b *Backend) docOpen(_ context.Context, p map[string]any) any {
	return b.docs.Open(document.OpenRequest{
		Path:     params.StringOpt(p, "path", ""),
		SwapID:   params.StringOpt(p, "swapId", ""),
		Encoding: params.StringOpt(p, "encoding", ""),
		Elevated: params.BoolOpt(p, "elevated", false),
		Force:    params.BoolOpt(p, "force", false),
	})
}

func (b *Backend) docEdit(_ context.Context, p map[string]any) any {
	var undo *document.UndoState
	if value, exists := p["undoSteps"]; exists {
		var steps int
		encoded, err := json.Marshal(value)
		if value == nil || err != nil || json.Unmarshal(encoded, &steps) != nil || steps < 0 {
			return fail("INVALID", "undoSteps must be a nonnegative integer")
		}
		undo = &document.UndoState{Steps: steps}
	}
	if value, exists := p["redo"]; exists {
		redo, valid := value.(bool)
		if !valid || undo == nil {
			return fail("INVALID", "redo must be a boolean with undoSteps")
		}
		undo.Redo = redo
	}
	b.docs.Edit(
		params.StringOpt(p, "docId", ""),
		int64(params.IntOpt(p, "version", 0)),
		params.IntOpt(p, "start", 0),
		params.IntOpt(p, "removed", 0),
		params.StringOpt(p, "text", ""),
		params.IntOpt(p, "length", 0),
		undo,
	)
	return ok()
}

func (b *Backend) docReset(_ context.Context, p map[string]any) any {
	return b.docs.Reset(params.StringOpt(p, "docId", ""), int64(params.IntOpt(p, "version", 0)), params.StringOpt(p, "text", ""), params.BoolOpt(p, "conflict", false))
}

func (b *Backend) docSave(ctx context.Context, p map[string]any) any {
	ctx, cancel := context.WithTimeout(ctx, awaitEdits)
	defer cancel()
	req := document.SaveRequest{
		DocID:     params.StringOpt(p, "docId", ""),
		Version:   int64(params.IntOpt(p, "version", 0)),
		MD5:       params.StringOpt(p, "md5", ""),
		Path:      params.StringOpt(p, "path", ""),
		Encoding:  params.StringOpt(p, "encoding", ""),
		EOL:       params.StringOpt(p, "eol", ""),
		Elevated:  params.BoolOpt(p, "elevated", false),
		Force:     params.BoolOpt(p, "force", false),
		Overwrite: params.BoolOpt(p, "overwrite", false),
	}
	if bom, err := params.Bool(p, "bom"); err == nil {
		req.BOM = &bom
	}
	return b.docs.Save(ctx, req)
}

func (b *Backend) docReload(_ context.Context, p map[string]any) any {
	return b.docs.Reload(params.StringOpt(p, "docId", ""), params.StringOpt(p, "encoding", ""))
}

func (b *Backend) docClose(ctx context.Context, p map[string]any) any {
	ctx, cancel := context.WithTimeout(ctx, awaitEdits)
	defer cancel()
	return b.docs.Close(ctx, document.CloseRequest{
		DocID:   params.StringOpt(p, "docId", ""),
		Discard: params.BoolOpt(p, "discard", false),
		Version: int64(params.IntOpt(p, "version", 0)),
		MD5:     params.StringOpt(p, "md5", ""),
	})
}

func (b *Backend) docSetTitle(_ context.Context, p map[string]any) any {
	return b.docs.SetTitle(params.StringOpt(p, "docId", ""), params.StringOpt(p, "title", ""))
}

func (b *Backend) docText(ctx context.Context, p map[string]any) any {
	return b.withText(ctx, p, func(text string, version int64) any {
		return map[string]any{"ok": true, "text": text, "version": version}
	})
}

func (b *Backend) docFlush(context.Context, map[string]any) any {
	if failed := b.docs.Flush(); len(failed) > 0 {
		return map[string]any{"ok": false, "code": document.CodeIO, "docIds": failed}
	}
	return ok()
}

func (b *Backend) recoveryList(context.Context, map[string]any) any {
	items := b.docs.Recovery()
	if items == nil {
		items = []document.RecoveryItem{}
	}
	return map[string]any{"ok": true, "items": items}
}

func (b *Backend) recoveryDiscard(_ context.Context, p map[string]any) any {
	return b.docs.DiscardRecovery(params.StringOpt(p, "swapId", ""))
}
