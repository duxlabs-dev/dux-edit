pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.Modules.Editor
import qs.Services

// The tab list and every flow that can prompt: open, close, save, autosave, session restore and recovery.
Singleton {
    id: root

    // EditorDocument objects in tab order. Reassigned on every change so bindings follow.
    property var tabs: []
    property int currentIndex: -1
    readonly property var current: tabs[currentIndex] ?? null
    // Simple mode asked for by `--simple`, for this run only.
    property bool simpleOverride: false
    readonly property bool simple: simpleOverride || SettingsData.viewMode === "simple"

    // Mirrors `tabs` as {tabId} rows so views keep their delegates, and the editors in them, across tab changes.
    readonly property ListModel tabModel: ListModel {}

    // The shell shows a prompt for these and calls back into closeTab/confirmLargeFile.
    signal closeConfirmationNeeded(var doc)
    signal largeFileConfirmationNeeded(var doc, real size)
    // Any other question: {title, message, actions: [{label, run, danger}], cancelLabel, cancel}
    signal promptRequested(var prompt)

    property int nextTabId: 1
    // True between resume() and suspend(): the window is up and documents may load.
    property bool resumed: false
    property bool restored: false
    property var pendingOpens: []
    // Paths approved past the large-file question, for this run.
    property var approvedLarge: ({})

    readonly property Component documentComponent: Component {
        EditorDocument {}
    }

    function byDocId(docId) {
        return tabs.find(doc => doc.docId === docId) ?? null;
    }

    function byTabId(tabId) {
        return tabs.find(doc => doc.tabId === tabId) ?? null;
    }

    function indexOfPath(path) {
        return tabs.findIndex(doc => doc.path === path);
    }

    function addTab(properties) {
        const doc = documentComponent.createObject(root, Object.assign({
            "tabId": "t" + Date.now() + "-" + nextTabId++
        }, properties));
        tabs = tabs.concat([doc]);
        tabModel.append({
            "tabId": doc.tabId
        });
        return doc;
    }

    function removeTab(doc) {
        const index = tabs.indexOf(doc);
        if (index < 0)
            return;
        if (autosaveDoc === doc) {
            autosaveTimer.stop();
            autosaveDoc = null;
        }
        const wasCurrent = index === currentIndex;
        tabModel.remove(index, 1);
        const remaining = tabs.slice();
        remaining.splice(index, 1);
        tabs = remaining;
        doc.release();
        if (tabs.length === 0) {
            currentIndex = -1;
            newDraft();
            return;
        }
        if (wasCurrent) {
            activate(Math.min(index, tabs.length - 1));
            return;
        }
        if (index < currentIndex)
            currentIndex--;
        storeSession();
    }

    function activate(index) {
        if (index < 0 || index >= tabs.length)
            return;
        const doc = tabs[index];
        if (autosaveDoc && autosaveDoc !== doc)
            autosaveNow(null);
        currentIndex = index;
        doc.lastUsed = ++useCount;
        storeSession();
        if (resumed && !doc.loaded && !doc.loading)
            load(doc, null, null);
        trimEditors();
    }

    property int useCount: 0
    // Text kept in the editors of background tabs, in characters. An editor costs about 80 MB per MB of text.
    readonly property int editorBudget: 4 << 20

    // Unloads like hiding the window: unsaved text goes to the swap, or stays in core when the swap cannot take it.
    function trimEditors() {
        if (!resumed || Backend.lost || !Backend.connected)
            return;
        const background = tabs.filter(doc => doc !== current && doc.loaded).sort((a, b) => b.lastUsed - a.lastUsed);
        let kept = 0;
        let over = false;
        for (const doc of background) {
            over = over || kept + doc.charCount > editorBudget;
            if (!over) {
                kept += doc.charCount;
                continue;
            }
            if (unloadable(doc))
                unload(doc, false, null);
        }
    }

    // Not a document in the middle of a request, one whose swap failed or that has none (read as
    // administrator), and not one with a conflict: a clean deleted file's editor holds its last copy.
    function unloadable(doc) {
        return doc.loaded && !doc.loading && !doc.closing && !doc.saving && !doc.swapFailed && doc.recoverable && doc.conflict === "" && doc.docId !== "" && doc !== autosaveDoc;
    }

    function moveCurrent(step) {
        moveTab(currentIndex, currentIndex + step);
    }

    function draftTitle() {
        const used = tabs.map(doc => doc.title);
        const base = I18n.tr("Untitled", "name of a new, unsaved document");
        if (!used.includes(base))
            return base;
        for (let n = 2;; n++) {
            const title = I18n.tr("Untitled %1", "name of a new, unsaved document; %1 is a number").arg(n);
            if (!used.includes(title))
                return title;
        }
    }

    function newDraft() {
        addTab({
            "title": draftTitle()
        });
        activate(tabs.length - 1);
    }

    // A fresh start's empty draft gives way to the first file opened.
    function pristineDraft() {
        if (tabs.length !== 1)
            return null;
        const doc = tabs[0];
        return doc.isDraft && doc.loaded && !doc.dirty && doc.charCount === 0 ? doc : null;
    }

    // line and column are 1-based and apply to the first path.
    // mustExist: a path from a list of past files, which opens nothing when the file is gone.
    function openPaths(paths, line, column, mustExist) {
        // Held until the session is back, so restored tabs keep their order and the request ends up current.
        if (!resumed || !restored) {
            pendingOpens = pendingOpens.concat([
                {
                    "paths": paths,
                    "line": line,
                    "column": column,
                    "mustExist": mustExist === true
                }
            ]);
            return;
        }
        if (!paths || paths.length === 0) {
            newDraft();
            return;
        }
        const placeholder = pristineDraft();
        let first = -1;
        for (const path of paths) {
            let index = indexOfPath(path);
            if (index < 0) {
                addTab({
                    "path": path,
                    "title": AppPaths.baseName(path),
                    "mustExist": mustExist === true
                });
                index = tabs.length - 1;
            }
            if (first < 0)
                first = index;
        }
        const doc = tabs[first];
        doc.pendingLine = line || 0;
        doc.pendingColumn = column || 0;
        if (doc.loaded && doc.editor && doc.pendingLine > 0) {
            doc.editor.gotoLine(doc.pendingLine, doc.pendingColumn);
            doc.pendingLine = 0;
            doc.pendingColumn = 0;
        }
        activate(first);
        if (placeholder && placeholder !== doc)
            closeTab(tabs.indexOf(placeholder), true);
    }

    // then runs once the document is in.
    function load(doc, options, then) {
        doc.load(options, result => root.handleOpen(doc, options, result, then));
    }

    function handleOpen(doc, options, result, then) {
        if (tabs.indexOf(doc) < 0) {
            // The tab closed while it loaded; the document object may be gone.
            if (result.ok && result.docId)
                Backend.call("doc.close", {
                    "docId": result.docId,
                    "discard": false
                }, null);
            return;
        }
        // Not connected: keep the tab, the next activation tries again.
        if (result.error)
            return;
        // The window went away while this loaded: a hidden window holds no documents.
        if (!resumed) {
            if (result.ok)
                unload(doc, false, null);
            return;
        }
        if (result.ok) {
            loaded(doc, result);
            if (doc.mustExist && result.missing) {
                dropMissing(doc);
                return;
            }
            doc.mustExist = false;
            then?.();
            return;
        }
        switch (result.code) {
        case "CONFIRM_LARGE":
            if (approvedLarge[doc.path]) {
                load(doc, Object.assign({}, options, {
                    "force": true
                }), then);
                return;
            }
            largeFileConfirmationNeeded(doc, result.size || 0);
            return;
        case "NOTFOUND":
            // The swap went away, for example an empty draft that never wrote one.
            if (doc.swapId !== "") {
                doc.swapId = "";
                load(doc, options, then);
                return;
            }
            break;
        case "EACCES":
            if (result.elevation === "available") {
                offerElevatedOpen(doc, options, then);
                return;
            }
            break;
        case "EIO":
        case "LOCKED":
            // Its swap holds unsaved edits that could not be read just now; the tab stays so they are not forgotten.
            if (doc.swapId !== "") {
                ToastService.showError(openErrorText(doc, result));
                return;
            }
            break;
        }
        ToastService.showError(openErrorText(doc, result));
        removeTab(doc);
    }

    function loaded(doc, result) {
        if (doc.lexerOverride !== "")
            Highlight.setLexer(doc, doc.lexerOverride, false);
        if (doc.isDraft && result.title !== doc.title)
            doc.setTitle(doc.title);
        if (!doc.isDraft)
            SessionData.addRecent(doc.path);
        if (doc.conflict === "deleted" && !doc.dirty)
            ToastService.showWarning(I18n.tr("%1 was deleted from disk.", "notice; %1 is a file name").arg(doc.title));
        if (result.elevated)
            ToastService.showWarning(I18n.tr("Opened as administrator. This document has no crash recovery.", "notice when a file was read with administrator rights"));
        storeSession();
    }

    function confirmLargeFile(doc) {
        approvedLarge[doc.path] = true;
        load(doc, {
            "force": true
        }, null);
    }

    function offerElevatedOpen(doc, options, then) {
        promptRequested({
            "title": I18n.tr("Permission Denied", "dialog title when a file cannot be read"),
            "message": I18n.tr("You do not have permission to read %1.", "dialog message; %1 is a file name").arg(doc.title),
            "actions": [
                {
                    "label": I18n.tr("Open as Administrator", "button that reads a file with administrator rights"),
                    "run": () => root.load(doc, Object.assign({}, options, {
                            "elevated": true
                        }), then)
                }
            ],
            "cancel": () => root.removeTab(doc)
        });
    }

    function elevationText(reason) {
        switch (reason) {
        case "NOPOLICY":
            return I18n.tr("Administrator access needs the installed polkit policy.", "reason administrator access is unavailable");
        case "NOPKEXEC":
            return I18n.tr("Administrator access needs pkexec, which is not installed.", "reason administrator access is unavailable");
        case "SANDBOXED":
            return I18n.tr("Administrator access is not available inside a sandbox.", "reason administrator access is unavailable");
        default:
            return I18n.tr("Administrator access is not available.", "reason administrator access is unavailable");
        }
    }

    function openErrorText(doc, result) {
        const name = doc.title;
        switch (result.code) {
        case "BINARY":
            return I18n.tr("%1 is a binary file.", "error opening a file; %1 is a file name").arg(name);
        case "TOOLARGE":
            return I18n.tr("%1 is too large to open.", "error opening a file; %1 is a file name").arg(name);
        case "ISDIR":
            return I18n.tr("%1 is a folder.", "error opening a file; %1 is a file name").arg(name);
        case "LOCKED":
            return I18n.tr("%1 is open in another Dux Edit window.", "error opening a file; %1 is a file name").arg(name);
        case "EACCES":
            return I18n.tr("Cannot read %1.", "error opening a file; %1 is a file name").arg(name) + " " + elevationText(result.elevation);
        default:
            return I18n.tr("Cannot open %1.", "error opening a file; %1 is a file name").arg(name) + (result.message ? " " + result.message : "");
        }
    }

    function dropMissing(doc) {
        const path = doc.path;
        ToastService.showError(I18n.tr("%1 no longer exists.", "error opening a recent file; %1 is a file name").arg(AppPaths.baseName(path)));
        SessionData.removeRecent(path);
        closeTab(tabs.indexOf(doc), true);
        closedTabs = closedTabs.filter(entry => entry.path !== path);
    }

    // force skips the unsaved-changes prompt.
    function closeTab(index, force) {
        const doc = tabs[index];
        if (!doc)
            return;
        const emptyDraft = doc.isDraft && doc.loaded && doc.charCount === 0;
        if (doc.dirty && !emptyDraft && !force) {
            closeConfirmationNeeded(doc);
            return;
        }
        if (doc.loaded || doc.held) {
            doc.close(true);
        } else if (doc.dirty && doc.swapId !== "") {
            // Unloaded since the window was hidden, with edits in its swap.
            Backend.call("recovery.discard", {
                "swapId": doc.swapId
            }, null);
        }
        // Any other unloaded tab's swap stays for recovery: nobody has looked at it this run.
        if (!doc.isDraft)
            rememberClosed(doc, index);
        removeTab(doc);
    }

    property var closedTabs: []
    readonly property int closedTabLimit: 20

    function rememberClosed(doc, index) {
        closedTabs = closedTabs.filter(entry => entry.path !== doc.path).concat([
            {
                "path": doc.path,
                "index": index,
                "cursor": doc.loaded ? doc.cursorPosition : Math.max(0, doc.pendingCursor),
                "scrollY": doc.scrollY,
                "previewMode": doc.previewMode,
                "previewChosen": doc.previewChosen,
                "lexer": doc.lexerOverride,
                "wrap": doc.wrapOverride
            }
        ]).slice(-closedTabLimit);
    }

    function reopenClosedTab() {
        if (!resumed || !restored)
            return;
        while (closedTabs.length > 0) {
            const entry = closedTabs[closedTabs.length - 1];
            closedTabs = closedTabs.slice(0, -1);
            const open = indexOfPath(entry.path);
            if (open >= 0) {
                activate(open);
                return;
            }
            const placeholder = pristineDraft();
            const doc = addTab({
                "path": entry.path,
                "title": AppPaths.baseName(entry.path),
                "pendingCursor": entry.cursor,
                "scrollY": entry.scrollY,
                "previewMode": entry.previewMode,
                "previewChosen": entry.previewChosen,
                "lexerOverride": entry.lexer,
                "wrapOverride": entry.wrap,
                "mustExist": true
            });
            moveTab(tabs.length - 1, Math.min(entry.index, tabs.length - 1));
            activate(tabs.indexOf(doc));
            if (placeholder)
                closeTab(tabs.indexOf(placeholder), true);
            return;
        }
    }

    function closeTabs(docs) {
        for (const doc of docs)
            closeTab(tabs.indexOf(doc), false);
    }

    function closeOthers(keep) {
        activate(tabs.indexOf(keep));
        closeTabs(tabs.filter(doc => doc !== keep));
    }

    function closeToRight(keep) {
        const index = tabs.indexOf(keep);
        if (currentIndex > index)
            activate(index);
        closeTabs(tabs.slice(index + 1));
    }

    function discardChanges(doc) {
        if (!doc?.loaded || doc.isDraft || !doc.dirty)
            return;
        promptRequested({
            "title": I18n.tr("Discard Unsaved Changes?", "dialog title"),
            "message": I18n.tr("%1 goes back to the version on disk. Your unsaved changes are lost.", "dialog message; %1 is a file name").arg(doc.title),
            "actions": [
                {
                    "label": I18n.tr("Discard Changes", "button that drops unsaved edits and reloads the file"),
                    "danger": true,
                    "run": () => {
                        if (root.tabs.includes(doc) && doc.loaded)
                            root.reloadFromDisk(doc, null);
                    }
                }
            ]
        });
    }

    // Edits made while the save ran ask again.
    function saveAndClose(doc) {
        const close = () => root.save(doc, () => root.closeTab(root.tabs.indexOf(doc), false));
        if (doc.loaded) {
            close();
            return;
        }
        load(doc, null, close);
    }

    function moveTab(from, to) {
        if (from === to || from < 0 || to < 0 || from >= tabs.length || to >= tabs.length)
            return;
        const doc = current;
        const reordered = tabs.slice();
        reordered.splice(to, 0, reordered.splice(from, 1)[0]);
        tabModel.move(from, to, 1);
        tabs = reordered;
        currentIndex = tabs.indexOf(doc);
        storeSession();
    }

    function saveCurrent() {
        save(current, null);
    }

    function saveCurrentAs() {
        saveAs(current, null, null);
    }

    // done runs once the document reached the disk.
    function save(doc, done) {
        if (!doc?.loaded)
            return;
        if (doc.isDraft) {
            saveAs(doc, done, null);
            return;
        }
        saveWith(doc, {}, done, false);
    }

    // Saves with a conversion, {encoding, bom} or {eol}; a draft goes through Save As first.
    function saveConverted(doc, options) {
        if (!doc?.loaded)
            return;
        if (doc.isDraft) {
            saveAs(doc, null, options);
            return;
        }
        saveWith(doc, options, null, false);
    }

    function saveAs(doc, done, options) {
        if (!doc?.loaded)
            return;
        const recent = SessionData.recentFiles[0] ?? "";
        Backend.call("dialog.saveFile", {
            "title": I18n.tr("Save As", "file chooser title"),
            "dir": doc.isDraft ? (recent ? AppPaths.dirName(recent) : AppPaths.home) : AppPaths.dirName(doc.path),
            "name": doc.isDraft ? doc.title : AppPaths.baseName(doc.path)
        }, result => {
            if (!root.dialogOk(result))
                return;
            root.saveWith(doc, Object.assign({}, options, {
                "path": result.path
            }), done, false);
        });
    }

    function openWithDialog() {
        const path = current?.path || (SessionData.recentFiles[0] ?? "");
        Backend.call("dialog.openFiles", {
            "title": I18n.tr("Open", "file chooser title"),
            "dir": path ? AppPaths.dirName(path) : "",
            "multiple": true
        }, result => {
            if (!root.dialogOk(result))
                return;
            root.openPaths(result.paths, 0, 0);
        });
    }

    // result.error is a code or the core's own wording, so it goes to the log and the user gets this.
    function requestErrorText(result) {
        if (result.error === "DISCONNECTED")
            return I18n.tr("Dux Edit lost the connection to its core.", "error when the backend process is unreachable");
        console.warn("request failed:", result.error);
        return I18n.tr("Dux Edit could not complete the request.", "error when the backend refused a request");
    }

    function dialogOk(result) {
        if (result.error) {
            ToastService.showError(requestErrorText(result));
            return false;
        }
        if (result.ok)
            return true;
        switch (result.code) {
        case "CANCELED":
            return false;
        case "NOPORTAL":
            ToastService.showError(I18n.tr("No file chooser is available. Install a desktop portal.", "error when the portal file chooser is missing"));
            return false;
        default:
            ToastService.showError(result.message || result.code);
            return false;
        }
    }

    function saveWith(doc, options, done, retried) {
        if (doc.saving)
            return;
        const params = Object.assign({}, options);
        if (doc.pendingEol !== "" && !params.eol)
            params.eol = doc.pendingEol;
        doc.save(params, result => {
            if (result.error) {
                ToastService.showError(root.requestErrorText(result));
                return;
            }
            if (result.ok) {
                doc.autoSaved = false;
                doc.pendingEol = "";
                SessionData.addRecent(result.path);
                root.storeSession();
                done?.();
                return;
            }
            root.handleSaveFailure(doc, params, done, retried, result);
        });
    }

    function handleSaveFailure(doc, options, done, retried, result) {
        const retry = extra => saveWith(doc, Object.assign({}, options, extra), done, retried);
        switch (result.code) {
        case "EXISTS":
            promptRequested({
                "title": I18n.tr("Replace File?", "dialog title when saving over an existing file"),
                "message": I18n.tr("%1 already exists. Replacing it overwrites its contents.", "dialog message; %1 is a file name").arg(AppPaths.baseName(options.path || "")),
                "actions": [
                    {
                        "label": I18n.tr("Replace", "button that overwrites an existing file"),
                        "danger": true,
                        "run": () => retry({
                                "overwrite": true
                            })
                    }
                ]
            });
            return;
        case "CONFLICT":
            doc.conflict = doc.conflict || "modified";
            return;
        case "EACCES":
            if (result.elevation !== "available") {
                ToastService.showError(I18n.tr("You do not have permission to save %1.", "error saving a file; %1 is a file name").arg(doc.title) + " " + elevationText(result.elevation));
                return;
            }
            promptRequested({
                "title": I18n.tr("Permission Denied", "dialog title when a file cannot be written"),
                "message": I18n.tr("You do not have permission to save %1.", "error saving a file; %1 is a file name").arg(doc.title),
                "actions": [
                    {
                        "label": I18n.tr("Save as Administrator", "button that writes a file with administrator rights"),
                        "run": () => retry({
                                "elevated": true
                            })
                    }
                ]
            });
            return;
        case "UNENCODABLE":
            promptRequested({
                "title": I18n.tr("Cannot Save in %1", "dialog title; %1 is a text encoding").arg((result.encoding || doc.encoding).toUpperCase()),
                "message": (result.char ? I18n.tr("%1 (U+%2) cannot be written in this encoding.", "dialog message; %1 is a character, %2 its Unicode code point").arg(result.char).arg(result.char.codePointAt(0).toString(16).toUpperCase().padStart(4, "0")) : I18n.tr("The document has characters this encoding cannot hold.", "dialog message when saving fails because of the encoding")) + (doc.encodingGuessed ? " " + I18n.tr("This file was not valid UTF-8 and was read as ISO-8859-1. Saving as UTF-8 rewrites every accented character as it is shown now.", "warning added to a dialog when the encoding of a file was guessed") : ""),
                "actions": [
                    {
                        "label": I18n.tr("Save as UTF-8", "button that saves in the UTF-8 encoding"),
                        "danger": doc.encodingGuessed,
                        "run": () => retry({
                                "encoding": "utf-8",
                                "bom": false
                            })
                    }
                ]
            });
            return;
        case "DESYNC":
            if (retried) {
                ToastService.showError(I18n.tr("Could not save %1. Try again.", "error saving a file; %1 is a file name").arg(doc.title));
                return;
            }
            doc.resync(reset => {
                if (reset.ok)
                    root.saveWith(doc, options, done, true);
            }, false);
            return;
        default:
            // The write may have stopped partway: a change event for it must not reload over the editor.
            if (result.code === "EIO")
                doc.recoveredDirty = true;
            ToastService.showError(I18n.tr("Could not save %1.", "error saving a file; %1 is a file name").arg(doc.title) + (result.message ? " " + result.message : ""));
        }
    }

    // The conflict banner's "Keep My Edits".
    function keepMine(doc) {
        saveWith(doc, {
            "force": true
        }, null, false);
    }

    function reloadFromDisk(doc, options) {
        doc.pendingCursor = doc.cursorPosition;
        doc.reload(options, result => {
            if (result.error || result.ok)
                return;
            if (result.code === "NOTFOUND") {
                // Gone since the change was reported: the text in the editor may be its last copy.
                doc.conflict = "deleted";
                ToastService.showWarning(I18n.tr("%1 was deleted from disk.", "notice; %1 is a file name").arg(doc.title));
                return;
            }
            ToastService.showError(root.openErrorText(doc, result));
        });
    }

    // Reopening with another encoding drops unsaved edits, so a dirty document asks first.
    function reopenWithEncoding(doc, encoding) {
        const reopen = () => root.reloadFromDisk(doc, {
                "encoding": encoding
            });
        if (!doc.dirty) {
            reopen();
            return;
        }
        promptRequested({
            "title": I18n.tr("Discard Unsaved Changes?", "dialog title"),
            "message": I18n.tr("Reopening %1 with another encoding discards your unsaved changes.", "dialog message; %1 is a file name").arg(doc.title),
            "actions": [
                {
                    "label": I18n.tr("Reopen", "button that reopens a file with another encoding"),
                    "danger": true,
                    "run": reopen
                }
            ]
        });
    }

    function setLineEnding(doc, eol) {
        if (!doc.isDraft && !doc.dirty && doc.loaded) {
            saveWith(doc, {
                "eol": eol
            }, null, false);
            return;
        }
        doc.pendingEol = eol === doc.eol ? "" : eol;
    }

    property var autosaveDoc: null

    function autosaveEligible(doc) {
        return SettingsData.autoSave && !!doc && doc.loaded && doc.dirty && !doc.isDraft && doc.writable && !doc.readOnly && !doc.elevated && doc.conflict === "";
    }

    // Called by a document for each edit, so only a tab the user typed in is ever autosaved.
    function noteEdit(doc) {
        if (doc !== current || !autosaveEligible(doc))
            return;
        autosaveDoc = doc;
        autosaveTimer.restart();
    }

    function autosaveNow(done) {
        autosaveTimer.stop();
        const doc = autosaveDoc;
        autosaveDoc = null;
        if (!autosaveEligible(doc) || doc.saving) {
            done?.();
            return;
        }
        doc.save({}, result => {
            if (result.ok) {
                doc.autoSaved = true;
            } else if (result.code === "CONFLICT") {
                doc.conflict = doc.conflict || "modified";
            } else if (result.code === "DESYNC") {
                doc.resync(null, false);
            } else {
                ToastService.showError(I18n.tr("Autosave failed for %1.", "error; %1 is a file name").arg(doc.title) + " " + (result.error ? root.requestErrorText(result) : result.message || ""));
            }
            done?.();
        });
    }

    readonly property Timer autosaveTimer: Timer {
        interval: EditorMetrics.autosaveDelay
        onTriggered: root.autosaveNow(null)
    }

    function storeSession() {
        if (!resumed)
            return;
        SessionData.tabs = tabs.map(doc => ({
                    "tabId": doc.tabId,
                    "path": doc.path,
                    "swapId": doc.swapId,
                    "title": doc.title,
                    "cursor": doc.loaded ? doc.cursorPosition : Math.max(0, doc.pendingCursor),
                    "scrollY": doc.scrollY,
                    "previewMode": doc.previewMode,
                    "lexer": doc.lexerOverride,
                    "wrap": doc.wrapOverride,
                    "dirty": doc.dirty,
                    "existed": doc.existed
                }));
        SessionData.currentIndex = currentIndex;
    }

    // A hidden window holds no editors. Dirty documents keep their swap; one without a swap stays open in
    // core so its edits are there when the tab is shown again. done hears whether the text is safe.
    function unload(doc, saveFirst, done) {
        doc.pendingCursor = doc.cursorPosition;
        if (doc.dirty && !doc.recoverable) {
            doc.hold();
            done?.(true);
            return;
        }
        doc.stash(saveFirst, done);
    }

    function unsafeDocs() {
        return tabs.filter(doc => doc.dirty && (doc.loaded || doc.held) && (!doc.recoverable || doc.swapFailed || Backend.lost));
    }

    // Quitting drops what no swap holds, so those documents ask first.
    function confirmQuit(proceed) {
        const lost = unsafeDocs();
        if (lost.length === 0) {
            proceed();
            return;
        }
        const elevatedOnly = lost.every(doc => !doc.recoverable);
        let message;
        if (elevatedOnly)
            message = lost.length === 1 ? I18n.tr("%1 was opened as administrator and has no crash recovery. Quitting discards its unsaved changes.", "dialog message; %1 is a file name").arg(lost[0].title) : I18n.tr("%1 documents were opened as administrator and have no crash recovery. Quitting discards their unsaved changes.", "dialog message; %1 is a count").arg(lost.length);
        else
            message = lost.length === 1 ? I18n.tr("The unsaved changes in %1 could not be stored for recovery. Quitting discards them.", "dialog message; %1 is a file name").arg(lost[0].title) : I18n.tr("The unsaved changes in %1 documents could not be stored for recovery. Quitting discards them.", "dialog message; %1 is a count").arg(lost.length);
        promptRequested({
            "title": I18n.tr("Discard Unsaved Changes?", "dialog title"),
            "message": message,
            "actions": [
                {
                    "label": I18n.tr("Discard and Quit", "button that quits and drops unsaved changes"),
                    "danger": true,
                    "run": proceed
                }
            ]
        });
    }

    // The editors may be destroyed as soon as this returns; done gets the documents the core could not put in a swap.
    function suspend(done) {
        if (!resumed) {
            done?.([]);
            return;
        }
        storeSession();
        resumed = false;
        SessionData.flush();
        SettingsData.flush();
        autosaveTimer.stop();
        const saving = autosaveEligible(autosaveDoc) ? autosaveDoc : null;
        autosaveDoc = null;
        PluginService.suspend?.();
        const docs = tabs.filter(doc => doc.loaded);
        const failed = [];
        let waiting = docs.length;
        if (waiting === 0) {
            done?.(failed);
            return;
        }
        for (const doc of docs)
            unload(doc, doc === saving, safe => {
                if (!safe)
                    failed.push(doc);
                if (--waiting === 0)
                    done?.(failed);
            });
    }

    // The window appeared. Documents load once the core is reachable.
    function resume() {
        resumed = true;
        PluginService.resume?.();
        if (Backend.connected)
            start();
    }

    function start() {
        if (!restored) {
            restored = true;
            restoreSession();
        } else {
            activate(Math.max(0, Math.min(currentIndex, tabs.length - 1)));
        }
        const opens = pendingOpens;
        pendingOpens = [];
        for (const request of opens) {
            if (request.imported)
                applyImport(request.imported);
            else
                openPaths(request.paths, request.line, request.column, request.mustExist);
        }
        if (tabs.length === 0)
            newDraft();
    }

    function restoreSession() {
        const saved = SettingsData.restoreSession && Array.isArray(SessionData.tabs) ? SessionData.tabs : [];
        for (const entry of saved) {
            if (!entry || typeof entry !== "object")
                continue;
            addTab({
                "tabId": entry.tabId || ("t" + Date.now() + "-" + nextTabId++),
                "path": entry.path || "",
                "swapId": entry.swapId || "",
                "title": entry.title || (entry.path ? AppPaths.baseName(entry.path) : draftTitle()),
                "pendingCursor": entry.cursor ?? -1,
                "scrollY": entry.scrollY || 0,
                "previewMode": entry.previewMode || "off",
                "previewChosen": !!entry.previewMode,
                "lexerOverride": entry.lexer || "",
                "wrapOverride": typeof entry.wrap === "string" ? entry.wrap : "",
                "recoveredDirty": entry.dirty === true && !!entry.swapId,
                "existed": entry.existed === true
            });
        }
        if (tabs.length > 0)
            activate(Math.max(0, Math.min(SessionData.currentIndex, tabs.length - 1)));
        offerRecovery();
        if (!SessionData.dmsImportOffered && !SessionData.ephemeral)
            checkDmsImport();
    }

    function offerRecovery() {
        Backend.call("recovery.list", null, result => {
            const known = root.tabs.map(doc => doc.swapId).filter(id => id !== "");
            const items = (result.items || []).filter(item => !known.includes(item.swapId));
            if (items.length === 0)
                return;
            ToastService.show(items.length === 1 ? I18n.tr("1 document has unsaved changes from an earlier session.", "recovery notice") : I18n.tr("%1 documents have unsaved changes from an earlier session.", "recovery notice; %1 is a count").arg(items.length), {
                "level": "warning",
                "actionLabel": I18n.tr("Restore", "button that restores unsaved documents"),
                "action": () => root.restoreSwaps(items)
            });
        });
    }

    function restoreSwaps(items) {
        const placeholder = pristineDraft();
        const first = tabs.length;
        for (const item of items)
            addTab({
                "path": item.path || "",
                "swapId": item.swapId,
                "title": item.title || (item.path ? AppPaths.baseName(item.path) : draftTitle())
            });
        activate(first);
        if (placeholder)
            closeTab(tabs.indexOf(placeholder), true);
    }

    function checkDmsImport() {
        Backend.call("import.dms", {
            "dryRun": true
        }, result => {
            if (!result.ok)
                return;
            SessionData.dmsImportOffered = true;
            if (!result.found || (result.tabs || []).length === 0)
                return;
            root.promptRequested({
                "title": I18n.tr("Import DMS Notes?", "dialog title"),
                "message": I18n.tr("Dux Edit found %1 notes from the DMS notepad. Importing copies them; DMS keeps its own.", "dialog message; %1 is a count").arg(result.tabs.length),
                "actions": [
                    {
                        "label": I18n.tr("Import", "button that imports DMS notes"),
                        "run": () => root.importDms()
                    }
                ],
                "cancelLabel": I18n.tr("Not Now", "button that declines an offer")
            });
        });
    }

    function importDms() {
        Backend.call("import.dms", {}, result => {
            if (result.error || !result.ok) {
                ToastService.showError(I18n.tr("Could not import DMS notes.", "error") + (result.message ? " " + result.message : ""));
                return;
            }
            root.applyImport(result);
        });
    }

    // result: {tabs: [{title, path?, swapId?}], current, settings}, from import.dms or a ui import action.
    function applyImport(result) {
        if (!resumed || !restored) {
            pendingOpens = pendingOpens.concat([
                {
                    "imported": result
                }
            ]);
            return;
        }
        SessionData.dmsImportOffered = true;
        const imported = result.tabs || [];
        if (imported.length === 0) {
            ToastService.showInfo(I18n.tr("No DMS notes found.", "notice"));
            return;
        }
        const placeholder = pristineDraft();
        const first = tabs.length;
        for (const tab of imported)
            addTab({
                "path": tab.path || "",
                "swapId": tab.swapId || "",
                "title": tab.title || (tab.path ? AppPaths.baseName(tab.path) : draftTitle())
            });
        const settings = result.settings || {};
        for (const key in settings)
            SettingsData.set(key, settings[key]);
        activate(first + Math.max(0, Math.min(result.current || 0, imported.length - 1)));
        if (placeholder)
            closeTab(tabs.indexOf(placeholder), true);
        ToastService.showInfo(I18n.tr("Imported %1 DMS notes.", "notice; %1 is a count").arg(imported.length));
    }

    Connections {
        target: Backend

        function onConnectedChanged() {
            if (!Backend.connected)
                return;
            // Edits made while the socket was down never reached the core.
            for (const doc of root.tabs) {
                if (doc.loaded)
                    doc.resync(null, false);
            }
            if (root.resumed)
                root.start();
        }
    }

    Component.onCompleted: {
        Backend.subscribe("docs", event => {
            const doc = root.byDocId(event.docId);
            if (!doc)
                return;
            switch (event.type) {
            case "resync":
                doc.resync(null, false);
                return;
            case "swapFailed":
                ToastService.showError(I18n.tr("The unsaved changes in %1 could not be stored for recovery. Save the document, or free some disk space.", "error when the crash-recovery copy of a document cannot be written; %1 is a file name").arg(doc.title));
                return;
            case "changed":
                root.fileChanged(doc, event.kind);
                return;
            }
        });
    }

    function fileChanged(doc, kind) {
        if (doc.dirty) {
            doc.conflict = kind;
            return;
        }
        if (kind === "deleted") {
            doc.conflict = "deleted";
            ToastService.showWarning(I18n.tr("%1 was deleted from disk.", "notice; %1 is a file name").arg(doc.title));
            return;
        }
        reloadFromDisk(doc, null);
    }
}
