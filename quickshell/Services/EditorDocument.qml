pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services

// One open tab. Everything the shell chrome, the editor pane and plugins share about a document lives here.
// Created by Documents; the editor pane writes the cursor and count properties and sets `editor`.
QtObject {
    id: root

    // Stable across restarts; docId is only valid while the core has the document open.
    property string tabId: ""
    property string docId: ""
    property string swapId: ""
    property string path: ""
    property string title: ""

    property bool loading: false
    property bool loaded: false
    // Unloaded with the window but still open in core: its unsaved edits are in no swap.
    property bool held: false

    property int version: 0
    property int savedVersion: 0
    property bool recoveredDirty: false
    readonly property bool dirty: version !== savedVersion || recoveredDirty
    readonly property bool isDraft: path === ""

    property string encoding: "utf-8"
    property bool bom: false
    // "lf" | "crlf" | "cr"
    property string eol: "lf"
    property bool mixedEol: false
    property bool readOnly: false
    property bool writable: true
    property bool elevated: false
    // False for a document read with administrator rights: the core writes no swap for it.
    property bool recoverable: true

    // Written by Highlight from highlight state events.
    property string lexer: "plaintext"
    property string tier: "off"
    property string tierReason: ""
    readonly property bool isMarkdown: lexer === "markdown"
    // The language the user picked for this path, "" for detection. Kept in the session and reapplied on load.
    property string lexerOverride: ""

    // "" | "modified" | "deleted": the file changed on disk under unsaved edits.
    property string conflict: ""
    property bool saving: false
    // The last successful save was an autosave.
    property bool autoSaved: false
    // A line ending picked for the next save, "" for what the file has.
    property string pendingEol: ""

    // "off" | "split" | "full"
    property string previewMode: "off"
    // Set once the session or the user decided the preview mode; until then a Markdown tab takes the default.
    property bool previewChosen: false

    onIsMarkdownChanged: {
        if (!isMarkdown || previewChosen)
            return;
        previewMode = SettingsData.markdownPreviewMode;
        previewChosen = true;
    }

    // Written by the editor pane. Line and column are 1-based.
    property int cursorPosition: 0
    property int cursorLine: 1
    property int cursorColumn: 1
    property int selectionLength: 0
    property int lineCount: 1
    property int charCount: 0
    property real scrollY: 0

    // Applied by the editor pane once the text is in, then reset to 0.
    property int pendingLine: 0
    property int pendingColumn: 0
    property int pendingCursor: -1

    // The EditorPane showing this document, null while none is loaded.
    property var editor: null

    // The pane replaces its whole text with this: emitted after open and reload.
    signal textLoaded(string text)
    signal saved

    function applyOpenResult(result) {
        docId = result.docId;
        swapId = result.swapId;
        path = result.path;
        if (result.path !== "" || title === "")
            title = result.title;
        encoding = result.encoding;
        bom = result.bom;
        eol = result.eol;
        mixedEol = result.mixedEol;
        readOnly = result.readOnly;
        writable = result.writable;
        elevated = result.elevated;
        recoverable = result.recoverable !== false;
        version = result.version;
        savedVersion = result.version;
        recoveredDirty = result.dirty;
        conflict = result.conflict || "";
        loaded = true;
        textLoaded(result.text);
    }

    // callback gets the raw result so the caller can ask about large files, elevation or encodings.
    function load(options, callback) {
        if (loading)
            return;
        loading = true;
        if (held) {
            reattach(options, callback);
            return;
        }
        const params = Object.assign({
            "path": path,
            "swapId": swapId
        }, options || {});
        Backend.call("doc.open", params, result => {
            root.loading = false;
            if (result.ok)
                root.applyOpenResult(result);
            callback?.(result);
        });
    }

    function hold() {
        held = true;
        loaded = false;
    }

    // The core kept the text and its version; only the pane needs it back.
    function reattach(options, callback) {
        Backend.call("doc.text", {
            "docId": docId,
            "version": version
        }, result => {
            root.loading = false;
            root.held = false;
            if (!result.ok) {
                // The core no longer has it, so what is on disk is all there is.
                root.docId = "";
                root.load(options, callback);
                return;
            }
            root.loaded = true;
            root.textLoaded(result.text);
            callback?.(result);
        });
    }

    // Called by the editor pane for every change it makes to its text.
    function edited(start, removed, text, length) {
        version++;
        Backend.notify("doc.edit", {
            "docId": docId,
            "version": version,
            "start": start,
            "removed": removed,
            "text": text,
            "length": length
        });
    }

    // After a "resync" event or a DESYNC save: the pane's text is the truth.
    function resync(callback) {
        if (!editor)
            return;
        Backend.call("doc.reset", {
            "docId": docId,
            "version": version,
            "text": editor.getText()
        }, result => callback?.(result));
    }

    // options: {path, encoding, bom, eol, elevated, force, overwrite}
    function save(options, callback) {
        if (!loaded || saving)
            return;
        saving = true;
        const params = Object.assign({
            "docId": docId,
            "version": version
        }, options || {});
        if (editor)
            params.md5 = Qt.md5(editor.getText());
        Backend.call("doc.save", params, result => {
            root.saving = false;
            if (result.ok) {
                root.path = result.path;
                root.title = result.title;
                root.encoding = result.encoding;
                root.bom = result.bom;
                root.eol = result.eol;
                root.elevated = result.elevated;
                root.recoverable = result.recoverable !== false;
                root.savedVersion = result.version;
                root.recoveredDirty = false;
                root.conflict = "";
                root.saved();
            }
            callback?.(result);
        });
    }

    function reload(options, callback) {
        const sent = version;
        Backend.call("doc.reload", Object.assign({
            "docId": docId
        }, options || {}), result => {
            if (result.ok && result.docId !== root.docId)
                return;
            if (result.ok && root.version !== sent) {
                // Typed while the reload was in flight: the pane's text wins and the core takes it back.
                root.conflict = "modified";
                root.resync(null);
            } else if (result.ok) {
                root.applyOpenResult(result);
            }
            callback?.(result);
        });
    }

    // Names a draft; the core keeps it in the swap sidecar so recovery can show it.
    function setTitle(name) {
        title = name;
        if (docId === "")
            return;
        Backend.call("doc.setTitle", {
            "docId": docId,
            "title": name
        }, null);
    }

    function close(discard) {
        if (docId === "")
            return;
        Backend.call("doc.close", {
            "docId": docId,
            "discard": discard
        }, null);
        Highlight.forget(docId);
        Spell.forget(docId);
        docId = "";
        loaded = false;
        held = false;
    }
}
