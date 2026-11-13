pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import qs.DCommon.Common

Singleton {
    id: root

    readonly property string socketPath: Quickshell.env("DUX_EDIT_SOCKET")
    readonly property bool connected: socket.linkUp
    // The core was reachable and no longer is: nothing typed from here on can be saved or kept for recovery.
    property bool lost: false

    property var pendingCalls: ({})
    property var topicHandlers: ({})
    property int lastCallId: 0

    signal showRequested
    signal hideRequested
    signal toggleRequested
    // {paths, line, column, simple}; line and column are 1-based, 0 when not given.
    signal openRequested(var request)
    // {tabs, current, settings}: an import.dms result applied from the CLI.
    signal importRequested(var result)

    // A failed call answers {error}: "DISCONNECTED" from here, or the core's own message. Neither is user text.
    function call(method, params, callback) {
        if (!connected) {
            callback?.({
                "error": "DISCONNECTED"
            });
            return;
        }
        const request = {
            "id": ++lastCallId,
            "method": method
        };
        if (params)
            request.params = params;
        if (callback)
            pendingCalls[request.id] = callback;
        socket.send(request);
    }

    // Fire-and-forget: no id, so the core sends no reply line.
    function notify(method, params) {
        if (!connected)
            return;
        socket.send({
            "method": method,
            "params": params
        });
    }

    function subscribe(topic, handler) {
        const handlers = topicHandlers[topic] || [];
        handlers.push(handler);
        topicHandlers[topic] = handlers;
        if (connected && handlers.length === 1)
            call("subscribe", {
                "topics": [topic]
            }, null);
    }

    function quit() {
        if (!connected) {
            Qt.quit();
            return;
        }
        call("ui.quit", null, null);
    }

    function handleLine(line) {
        if (!line)
            return;
        let message;
        try {
            message = JSON.parse(line);
        } catch (e) {
            console.warn("backend sent a line that is not JSON:", line.substring(0, 200));
            return;
        }
        if (message.event) {
            for (const handler of topicHandlers[message.event] || [])
                handler(message.data || {});
            return;
        }
        const callback = pendingCalls[message.id];
        if (!callback)
            return;
        delete pendingCalls[message.id];
        callback(message.error ? {
            "error": message.error
        } : message.result || {});
    }

    function failPendingCalls() {
        const callbacks = pendingCalls;
        pendingCalls = ({});
        for (const id in callbacks)
            callbacks[id]({
                "error": "DISCONNECTED"
            });
    }

    Component.onCompleted: {
        subscribe("ui", data => {
            switch (data.action) {
            case "show":
                root.showRequested();
                return;
            case "hide":
                root.hideRequested();
                return;
            case "toggle":
                root.toggleRequested();
                return;
            case "open":
                root.openRequested(data);
                return;
            case "import":
                root.importRequested(data);
                return;
            }
        });
        socket.connected = socketPath !== "";
    }

    DSocket {
        id: socket

        path: root.socketPath
        connected: false

        parser: SplitParser {
            onRead: line => root.handleLine(line)
        }

        onConnectionStateChanged: {
            if (!linkUp) {
                root.lost = root.lastCallId > 0;
                root.failPendingCalls();
                return;
            }
            root.lost = false;
            root.call("subscribe", {
                "topics": Object.keys(root.topicHandlers)
            }, null);
        }
    }
}
