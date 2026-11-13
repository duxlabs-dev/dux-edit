pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.DCommon.Common
import qs.Services
import "../Common/LineIndex.js" as LineIndex
import "../Common/SyntaxPalette.js" as SyntaxPalette

// Span store for the lines around each document's viewport.
Singleton {
    id: root

    // [{name, aliases}] for the language picker, loaded on first use.
    property var lexers: []

    // One {color, bold, italic} per role id; follows the theme and the user's overrides, never the core.
    readonly property var palette: SyntaxPalette.build(Style, SettingsData.syntaxColors)
    // Role name to "#rrggbb" for markdown.render.
    readonly property var paletteNames: SyntaxPalette.names(palette)

    // docId -> LineMap of flat [col, len, role, ...] arrays. Plain JS so a merge never copies.
    readonly property var stores: LineIndex.createStores(3)

    // Emitted once per spans event that changed spans(docId).
    signal spansChanged(string docId)

    // A LineMap: get(line) returns [col, len, role, ...] or undefined. Columns are UTF-16 units in the line.
    function spans(docId) {
        return stores.get(docId);
    }

    // Tell the core which lines [first, last] are on screen, margin included. Spans outside are dropped.
    function view(doc, first, last) {
        stores.open(doc.docId).keep(first, last);
        Backend.notify("highlight.view", {
            "docId": doc.docId,
            "first": first,
            "last": last
        });
    }

    // name "" returns to automatic detection; force highlights past the size caps.
    function setLexer(doc, name, force) {
        Backend.call("highlight.setLexer", {
            "docId": doc.docId,
            "lexer": name,
            "force": force === true
        }, state => root.applyState(doc, state));
    }

    function loadLexers() {
        if (lexers.length > 0)
            return;
        Backend.call("highlight.lexers", null, result => root.lexers = result.lexers || []);
    }

    function applyState(doc, state) {
        if (!doc || state.error || state.ok === false)
            return;
        doc.lexer = state.lexer;
        doc.tier = state.tier;
        doc.tierReason = state.reason;
    }

    // Shifts stored spans for a pane edit; `change` is LineIndex.applyEdit's result.
    function linesEdited(docId, version, change) {
        stores.edited(docId, version, change);
    }

    function forget(docId) {
        stores.forget(docId);
    }

    Component.onCompleted: {
        Backend.subscribe("highlight", event => {
            switch (event.type) {
            case "state":
                root.applyState(Documents.byDocId(event.docId), event);
                return;
            case "spans":
                if (!event.lines || !Documents.byDocId(event.docId))
                    return;
                root.stores.open(event.docId).merge(event.version, event.first, event.lines);
                root.spansChanged(event.docId);
                return;
            }
        });
    }
}
