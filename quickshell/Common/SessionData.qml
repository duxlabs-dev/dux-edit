pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.DCommon.Common as DC

// session.json in the state dir: tab metadata only, never document text.
// A second instance (DUX_EDIT_EPHEMERAL=1) neither restores nor writes it.
Singleton {
    id: root

    readonly property bool ephemeral: Quickshell.env("DUX_EDIT_EPHEMERAL") === "1"
    readonly property int maxRecentFiles: 20

    // [{tabId, path, swapId, title, cursor, scrollY, previewMode, lexer, wrap, existed}]
    property var tabs: []
    property int currentIndex: 0
    property int windowWidth: 960
    property int windowHeight: 720
    property var recentFiles: []
    property bool dmsImportOffered: false

    readonly property var persistedState: ({
            "tabs": tabs,
            "currentIndex": currentIndex,
            "windowWidth": windowWidth,
            "windowHeight": windowHeight,
            "recentFiles": recentFiles,
            "dmsImportOffered": dmsImportOffered
        })

    property bool _applying: false

    function addRecent(path: string): void {
        if (path === "" || recentFiles[0] === path)
            return;
        recentFiles = [path].concat(recentFiles.filter(entry => entry !== path)).slice(0, maxRecentFiles);
    }

    function removeRecent(path: string): void {
        if (!recentFiles.includes(path))
            return;
        recentFiles = recentFiles.filter(entry => entry !== path);
    }

    function clearRecent(): void {
        recentFiles = [];
    }

    function flush(): void {
        store.flush();
    }

    function _apply(text) {
        const data = store.objectFrom(text);
        if (!data)
            return;
        _applying = true;
        for (const key in persistedState) {
            if (data[key] === undefined)
                continue;
            root[key] = data[key];
        }
        _applying = false;
    }

    onPersistedStateChanged: {
        if (_applying)
            return;
        store.save();
    }

    Component.onCompleted: {
        if (!ephemeral)
            store.read();
    }
    Component.onDestruction: flush()

    DC.JsonStore {
        id: store

        name: "session"
        path: root.ephemeral ? "" : AppPaths.sessionFile
        serialize: () => JSON.stringify(root.persistedState, null, 1)
        parse: text => root._apply(text)
    }
}
