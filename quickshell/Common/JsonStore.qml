pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import qs.Services as App

// One JSON file written atomically, debounced by a single-shot timer that each change re-arms.
QtObject {
    id: root

    property string path: ""
    property string name: "store"
    property int interval: 400
    property var serialize: null
    property var parse: null
    property alias adapter: file.adapter
    property bool loaded: false

    function save() {
        if (!loaded || path === "")
            return;
        _debounce.restart();
    }

    // Writes a pending change now: the process may exit before the debounce fires.
    function flush() {
        if (!_debounce.running)
            return;
        _debounce.stop();
        file.blockWrites = true;
        _commit();
        file.blockWrites = false;
    }

    function read() {
        return file.text() ?? "";
    }

    function objectFrom(text) {
        if (text === "")
            return null;
        try {
            const data = JSON.parse(text);
            if (data && typeof data === "object" && !Array.isArray(data))
                return data;
        } catch (e) {
            App.Log.scoped(name).warn(`ignoring unreadable ${name}: ${e}`);
        }
        return null;
    }

    function _commit() {
        if (path === "")
            return;
        if (serialize) {
            file.setText(serialize());
            return;
        }
        file.writeAdapter();
    }

    readonly property Timer _debounce: Timer {
        interval: root.interval
        onTriggered: root._commit()
    }

    readonly property FileView _file: FileView {
        id: file

        path: root.path
        blockLoading: true
        atomicWrites: true
        printErrors: false

        onLoaded: {
            root.parse?.(text() ?? "");
            root.loaded = true;
        }
        onLoadFailed: root.loaded = true
        onAdapterUpdated: root.save()
    }
}
