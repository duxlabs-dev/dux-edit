package backend

import (
	"context"

	"github.com/duxlabs-dev/dux-edit/core/internal/highlight"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

func (b *Backend) highlightView(_ context.Context, p map[string]any) any {
	b.highlight.View(params.StringOpt(p, "docId", ""), params.IntOpt(p, "first", 0), params.IntOpt(p, "last", 0))
	return ok()
}

func (b *Backend) highlightSetLexer(_ context.Context, p map[string]any) any {
	return b.highlight.SetLexer(params.StringOpt(p, "docId", ""), params.StringOpt(p, "lexer", ""), params.BoolOpt(p, "force", false))
}

func (b *Backend) highlightState(_ context.Context, p map[string]any) any {
	return b.highlight.State(params.StringOpt(p, "docId", ""))
}

func (b *Backend) highlightLexers(context.Context, map[string]any) any {
	lexers := b.highlight.Lexers()
	if lexers == nil {
		lexers = []highlight.LexerInfo{}
	}
	return map[string]any{"lexers": lexers, "roles": highlight.RoleNames}
}
