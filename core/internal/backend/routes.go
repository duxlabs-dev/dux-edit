package backend

import (
	"context"

	"github.com/duxlabs-dev/dux-go/ipc"
)

type call func(ctx context.Context, p map[string]any) any

// A request sent without an id is fire-and-forget and gets no reply line.
func handle(fn call) ipc.Handler {
	return func(ctx context.Context, w *ipc.ConnWriter, req ipc.Request, _ *ipc.Subscriber) {
		result := fn(ctx, req.Params)
		if req.ID == 0 {
			return
		}
		w.Respond(req.ID, result)
	}
}

func ok() any { return map[string]any{"ok": true} }

func fail(code, message string) any {
	return map[string]any{"ok": false, "code": code, "message": message}
}

// Every method is registered here and described in registry.go. Add both in the same change.
func (b *Backend) routes() *ipc.Mux {
	mux := ipc.NewMux()
	mux.Handle("version", handle(b.versionInfo))
	mux.Handle("describe", handle(b.describe))

	mux.Handle("ui.show", handle(b.uiShow))
	mux.Handle("ui.hide", handle(b.uiHide))
	mux.Handle("ui.toggle", handle(b.uiToggle))
	mux.Handle("ui.open", handle(b.uiOpen))
	mux.Handle("ui.quit", handle(b.uiQuit))

	mux.Handle("doc.open", handle(b.docOpen))
	mux.Handle("doc.edit", handle(b.docEdit))
	mux.Handle("doc.reset", handle(b.docReset))
	mux.Handle("doc.save", handle(b.docSave))
	mux.Handle("doc.reload", handle(b.docReload))
	mux.Handle("doc.close", handle(b.docClose))
	mux.Handle("doc.setTitle", handle(b.docSetTitle))
	mux.Handle("doc.text", handle(b.docText))
	mux.Handle("doc.flush", handle(b.docFlush))
	mux.Handle("recovery.list", handle(b.recoveryList))
	mux.Handle("recovery.discard", handle(b.recoveryDiscard))

	mux.Handle("highlight.view", handle(b.highlightView))
	mux.Handle("highlight.setLexer", handle(b.highlightSetLexer))
	mux.Handle("highlight.state", handle(b.highlightState))
	mux.Handle("highlight.lexers", handle(b.highlightLexers))

	mux.Handle("search.find", handle(b.searchFind))
	mux.Handle("search.replaceAll", handle(b.searchReplaceAll))
	mux.Handle("search.expand", handle(b.searchExpand))

	mux.Handle("spell.configure", handle(b.spellConfigure))
	mux.Handle("spell.languages", handle(b.spellLanguages))
	mux.Handle("spell.view", handle(b.spellView))
	mux.Handle("spell.suggest", handle(b.spellSuggest))
	mux.Handle("spell.add", handle(b.spellAdd))
	mux.Handle("spell.ignore", handle(b.spellIgnore))

	mux.Handle("markdown.render", handle(b.markdownRender))

	mux.Handle("plugins.list", handle(b.pluginsList))

	mux.Handle("dialog.openFiles", handle(b.dialogOpenFiles))
	mux.Handle("dialog.saveFile", handle(b.dialogSaveFile))
	mux.Handle("system.colorScheme.get", handle(b.systemColorScheme))
	mux.Handle("system.elevation", handle(b.systemElevation))
	mux.Handle("import.dms", handle(b.importDMS))
	return mux
}

func (b *Backend) versionInfo(context.Context, map[string]any) any {
	return map[string]any{"version": b.version, "apiVersion": APIVersion}
}

func (b *Backend) describe(context.Context, map[string]any) any {
	return map[string]any{"methods": Methods}
}
