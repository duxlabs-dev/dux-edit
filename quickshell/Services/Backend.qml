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
    property string version: ""

    property var pendingCalls: ({})
    property var topicHandlers: ({})
    property int lastCallId: 0

    signal showRequested

    function call(method, params, callback) {
        if (!connected) {
            callback?.({
                "error": "not connected to the backend"
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
                "error": "disconnected from the backend"
            });
    }

    Component.onCompleted: {
        subscribe("ui", data => {
            if (data.action === "show")
                root.showRequested();
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
                root.failPendingCalls();
                return;
            }
            root.call("subscribe", {
                "topics": Object.keys(root.topicHandlers)
            }, null);
            root.call("version", null, info => root.version = info.version || "");
        }
    }
}
