package backend

import (
	"context"
	"time"

	"github.com/duxlabs-dev/dux-edit/core/internal/buffer"
	"github.com/duxlabs-dev/dux-edit/core/internal/document"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

// How long a request tagged with a version waits for the edits before it to arrive.
const awaitEdits = 2 * time.Second

// withText hands fn the text once the mirror has reached the version the request names. The copy is taken
// under the read lock and fn runs without it, so a long scan or render never holds up edits or the lexer.
func (b *Backend) withText(ctx context.Context, p map[string]any, fn func(text string, version int64) any) any {
	d, found := b.docs.Get(params.StringOpt(p, "docId", ""))
	if !found {
		return fail(document.CodeUnknownDoc, "")
	}
	ctx, cancel := context.WithTimeout(ctx, awaitEdits)
	defer cancel()
	if d.Await(ctx, int64(params.IntOpt(p, "version", 0))) != nil {
		return fail(document.CodeDesync, "")
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
	b.docs.Edit(
		params.StringOpt(p, "docId", ""),
		int64(params.IntOpt(p, "version", 0)),
		params.IntOpt(p, "start", 0),
		params.IntOpt(p, "removed", 0),
		params.StringOpt(p, "text", ""),
		params.IntOpt(p, "length", 0),
	)
	return ok()
}

func (b *Backend) docReset(_ context.Context, p map[string]any) any {
	return b.docs.Reset(params.StringOpt(p, "docId", ""), int64(params.IntOpt(p, "version", 0)), params.StringOpt(p, "text", ""))
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

func (b *Backend) docClose(_ context.Context, p map[string]any) any {
	return b.docs.Close(params.StringOpt(p, "docId", ""), params.BoolOpt(p, "discard", false))
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
	b.docs.Flush()
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
