pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services

// Documents owns the tab; EditorPane supplies cursor/counts and the loaded editor.
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
    // A close that keeps the swap is in flight; a load waits for it in queuedLoad.
    property bool closing: false
    property var queuedLoad: null
    // The core could not write this document's swap, so its unsaved edits live only in the core's memory.
    property bool swapFailed: false
    property bool existed: false
    // Its tab is gone; destroyed once the request in flight has answered.
    property bool orphaned: false

    property int version: 0
    property int savedVersion: 0
    property bool recoveredDirty: false
    // Initial swap metadata is no longer authoritative after a doc.open reply.
    property bool recoveryPending: false
    // A past file or Markdown link: a missing file closes the tab instead of starting a new file.
    property bool mustExist: false
    readonly property bool dirty: version !== savedVersion || recoveredDirty
    readonly property bool isDraft: path === ""

    property string encoding: "utf-8"
    property bool bom: false
    // "lf" | "crlf" | "cr"
    property string eol: "lf"
    property bool mixedEol: false
    // The file was not valid UTF-8 and was read as ISO-8859-1, which may not be what it is.
    property bool encodingGuessed: false
    // Its encoding cannot write the file's bytes back; a save has to convert it.
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
    // "" follows the wrap setting, "on" or "off" is this tab's own choice. Kept in the session.
    property string wrapOverride: ""
    // From .editorconfig or the file itself: "tab" | "space" | "" for the settings, and 0 for unset.
    property string indentStyle: ""
    property int indentWidth: 0

    property int lastUsed: 0
    property bool readOnlyNoticeDismissed: false

    // "" | "modified" | "deleted": the file changed on disk under unsaved edits.
    property string conflict: ""
    property bool saving: false
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
        recoveryPending = false;
        docId = result.docId;
        swapId = result.swapId;
        path = result.path;
        if (result.path !== "" || title === "")
            title = result.title;
        encoding = result.encoding;
        bom = result.bom;
        eol = result.eol;
        mixedEol = result.mixedEol;
        readOnly = result.readOnly === true;
        indentStyle = result.indentStyle || "";
        indentWidth = result.indentWidth || 0;
        encodingGuessed = result.encodingGuessed === true;
        writable = result.writable;
        elevated = result.elevated;
        recoverable = result.recoverable !== false;
        version = result.version;
        savedVersion = result.version;
        recoveredDirty = result.dirty;
        conflict = result.conflict || (result.missing && existed ? "deleted" : "");
        existed = existed || (result.path !== "" && !result.missing);
        swapFailed = false;
        loaded = true;
        textLoaded(result.text);
    }

    // callback gets the raw result so the caller can ask about large files, elevation or encodings.
    function load(options, callback) {
        if (loading)
            return;
        loading = true;
        if (closing) {
            queuedLoad = {
                "options": options,
                "callback": callback
            };
            return;
        }
        if (held) {
            reattach(options, callback, false);
            return;
        }
        const params = Object.assign({
            "path": path,
            "swapId": swapId
        }, options || {});
        Backend.call("doc.open", params, result => {
            root.loading = false;
            if (result.ok && !root.orphaned)
                root.applyOpenResult(result);
            callback?.(result);
            root.settle();
        });
    }

    // Removing a tab must not destroy a document a reply is still going to write to.
    function release() {
        if (loading || closing) {
            orphaned = true;
            return;
        }
        destroy();
    }

    function settle() {
        if (orphaned && !loading && !closing)
            destroy();
    }

    // The document stays open in core, which gets the pane's text first.
    function hold() {
        if (editor && dirty)
            resync(null, false);
        held = true;
        loaded = false;
    }

    // Stash the pane's current text; done reports whether core closed it safely.
    function stash(saveFirst, done) {
        const id = docId;
        const params = {
            "docId": id,
            "discard": false,
            "version": version
        };
        const text = dirty && editor ? editor.getText() : null;
        if (text !== null)
            params.md5 = Qt.md5(text);
        const finish = result => {
            const closed = result.ok === true || result.code === "NODOC";
            root.closing = false;
            root.swapFailed = !closed;
            if (closed) {
                root.docId = "";
                root.held = false;
            }
            done?.(closed);
            const queued = root.queuedLoad;
            root.queuedLoad = null;
            if (queued && !root.orphaned) {
                root.loading = false;
                root.load(queued.options, queued.callback);
            }
            root.settle();
        };
        const send = () => Backend.call("doc.close", params, result => {
                if (result.code !== "DESYNC" || text === null) {
                    finish(result);
                    return;
                }
                Backend.call("doc.reset", {
                    "docId": id,
                    "version": params.version,
                    "text": text
                }, () => Backend.call("doc.close", params, finish));
            });
        Highlight.forget(id);
        Spell.forget(id);
        if (saveFirst && !saving)
            save({}, result => {
                if (result.ok)
                    root.autoSaved = true;
                send();
            });
        else
            send();
        loaded = false;
        held = true;
        closing = true;
    }

    // The core kept the text and its version; only the pane needs it back.
    function reattach(options, callback, anyVersion) {
        const params = {
            "docId": docId
        };
        if (!anyVersion)
            params.version = version;
        Backend.call("doc.text", params, result => {
            if (result.code === "DESYNC" && !anyVersion) {
                // The core's copy is the only one left, so it is taken as it stands.
                root.reattach(options, callback, true);
                return;
            }
            root.loading = false;
            if (result.error || root.orphaned) {
                callback?.(result);
                root.settle();
                return;
            }
            root.held = false;
            if (!result.ok) {
                // The core no longer has it, so what is on disk is all there is.
                root.docId = "";
                root.load(options, callback);
                return;
            }
            if (result.version !== root.version) {
                root.version = result.version;
                root.recoveredDirty = true;
            }
            root.loaded = true;
            root.textLoaded(result.text);
            callback?.(result);
        });
    }

    function edited(start, removed, text, length, undoSteps, redo) {
        version++;
        Backend.notify("doc.edit", {
            "docId": docId,
            "version": version,
            "start": start,
            "removed": removed,
            "text": text,
            "length": length,
            "undoSteps": undoSteps,
            "redo": redo
        });
        Documents.noteEdit(root);
    }

    // After a "resync" event or a DESYNC save the pane's text wins; conflict means the file changed on disk under it.
    function resync(callback, conflict) {
        if (!editor)
            return;
        Backend.call("doc.reset", {
            "docId": docId,
            "version": version,
            "text": editor.getText(),
            "conflict": conflict === true
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
                root.existed = true;
                root.readOnly = false;
                root.encodingGuessed = root.encodingGuessed && result.encoding === "iso-8859-1";
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
                root.resync(null, true);
            } else if (result.ok) {
                root.applyOpenResult(result);
            }
            callback?.(result);
        });
    }

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
