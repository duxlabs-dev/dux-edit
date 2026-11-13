import { test } from "node:test";
import assert from "node:assert/strict";
import { loadSingleton } from "./load.mjs";

function documents(overrides = {}) {
    const scope = {
        tabs: [],
        currentIndex: -1,
        nextTabId: 1,
        resumed: true,
        restored: true,
        pendingOpens: [],
        SessionData: {},
        AppPaths: { baseName: path => path.split("/").at(-1) },
        ...overrides,
    };
    const context = loadSingleton("../Services/Documents.qml", scope);
    context.pristineDraft = () => null;
    context.activate = index => { context.currentIndex = index; };
    context.addTab = properties => {
        const doc = { pendingLine: 0, pendingColumn: 0, recoveredDirty: false, ...properties };
        context.tabs.push(doc);
        return doc;
    };
    return context;
}

test("each requested file keeps its line and column until activation", () => {
    const context = documents();
    context.openPaths(["/a", "/b"], 3, 0, false, [{ line: 3 }, { line: 9, column: 2 }]);
    assert.equal(context.tabs[0].pendingLine, 3);
    assert.equal(context.tabs[1].pendingLine, 9);
    assert.equal(context.tabs[1].pendingColumn, 2);
    assert.equal(context.currentIndex, 0);
});

test("queued opens preserve per-file positions and legacy first-file fallback", () => {
    const context = documents({ resumed: false });
    context.openPaths(["/a", "/b"], 5, 4, true, [{}, { line: 9 }]);
    context.resumed = true;
    context.start();
    assert.equal(context.tabs[0].pendingLine, 5);
    assert.equal(context.tabs[0].pendingColumn, 4);
    assert.equal(context.tabs[1].pendingLine, 9);
    assert.equal(context.tabs[1].mustExist, true);
});

test("positions on already loaded files are applied to their own editors", () => {
    const moves = [];
    const context = documents({
        tabs: ["/a", "/b"].map(path => ({
            path, loaded: true, editor: { gotoLine: (line, column) => moves.push([path, line, column]) },
        })),
    });
    context.openPaths(["/a", "/b"], 3, 1, false, [{ line: 3, column: 1 }, { line: 9, column: 2 }]);
    assert.deepEqual(moves, [["/a", 3, 1], ["/b", 9, 2]]);
    assert.equal(context.tabs[1].pendingLine, 0);
});

test("restored dirty state comes from swap metadata without loading background tabs", () => {
    let reply;
    const context = documents({
        SessionData: {
            tabs: [
                { path: "/a", swapId: "changed", dirty: false },
                { path: "/b", swapId: "saved", dirty: true },
            ],
            currentIndex: 0,
            dmsImportOffered: true,
        },
        SettingsData: { restoreSession: true },
        Backend: { call: (method, params, callback) => {
            assert.equal(method, "recovery.list");
            reply = callback;
        } },
        ToastService: { show: () => assert.fail("known swaps are not orphan recovery") },
    });
    context.restoreSession();
    reply({ ok: true, items: [{ swapId: "changed", path: "/a" }] });
    assert.deepEqual(context.tabs.map(doc => doc.recoveredDirty), [true, false]);
    assert.ok(context.tabs.every(doc => !doc.loaded && !doc.loading));
    context.storeSession();
    assert.ok(context.SessionData.tabs.every(entry => !Object.hasOwn(entry, "dirty")));
});

test("recovery metadata never clears dirty state held by an active document", () => {
    let reply;
    const context = documents({
        tabs: [
            { swapId: "loaded", loaded: true, recoveredDirty: true },
            { swapId: "loading", loading: true, recoveredDirty: true },
            { swapId: "held", held: true, recoveredDirty: true },
            { swapId: "closing", closing: true, recoveredDirty: true },
        ],
        Backend: { call: (method, params, callback) => { reply = callback; } },
    });
    context.offerRecovery();
    context.tabs[0].loaded = false;
    reply({ ok: true, items: [] });
    assert.ok(context.tabs.every(doc => doc.recoveredDirty));
});

test("an authoritative open result retires pending crash metadata", () => {
    const context = loadSingleton("../Services/EditorDocument.qml", {
        recoveryPending: true,
        title: "",
        existed: false,
        textLoaded: () => {},
    });
    context.applyOpenResult({
        docId: "d1", swapId: "recovered", path: "/a", title: "a",
        encoding: "utf-8", bom: false, eol: "lf", mixedEol: false,
        writable: true, elevated: false, version: 0, dirty: true, text: "",
    });
    assert.equal(context.recoveryPending, false);
    assert.equal(context.recoveredDirty, true);
});

test("Markdown file links require an existing file", () => {
    const opened = [];
    const context = documents();
    context.openPaths = (...args) => opened.push(args);
    context.openMarkdownLink("missing.md", "/notes");
    context.openMarkdownLink("file:///notes/missing%20file.md#section", "/notes");
    context.openMarkdownLink("../other.md", "/notes/sub");
    assert.deepEqual(structuredClone(opened), [
        [["/notes/missing.md"], 0, 0, true],
        [["/notes/missing file.md"], 0, 0, true],
        [["/notes/other.md"], 0, 0, true],
    ]);
});

test("a missing Markdown link is discarded with the existing missing-file error", () => {
    const errors = [];
    const removedRecent = [];
    const closed = [];
    const context = documents({
        closedTabs: [],
        closedTabLimit: 20,
        autosaveDoc: null,
        tabModel: { remove: () => {} },
        SessionData: { removeRecent: path => removedRecent.push(path) },
        I18n: { tr: text => ({ arg: name => text.replace("%1", name) }) },
        ToastService: { showError: text => errors.push(text) },
    });
    context.loaded = () => {};
    context.newDraft = () => {};
    context.openMarkdownLink("missing.md", "/notes");
    const doc = context.tabs[0];
    Object.assign(doc, {
        loaded: true, dirty: false, cursorPosition: 0,
        close: discard => closed.push(discard), release: () => {},
    });
    context.handleOpen(doc, null, { ok: true, missing: true }, null);
    assert.equal(context.tabs.length, 0);
    assert.deepEqual(closed, [true]);
    assert.deepEqual(removedRecent, ["/notes/missing.md"]);
    assert.deepEqual(errors, ["missing.md no longer exists."]);
    assert.equal(context.closedTabs.length, 0);
});
