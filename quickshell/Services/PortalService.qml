pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell

// The desktop light/dark preference, watched by the core over the portal.
Singleton {
    id: root

    // "dark" | "light" | "none"
    property string scheme: "none"
    // No preference, or no portal, stays dark like the rest of the Dux desktop.
    readonly property bool prefersLight: scheme === "light"

    function refresh() {
        Backend.call("system.colorScheme.get", null, result => {
            if (result.error)
                return;
            root.scheme = result.scheme || "none";
        });
    }

    Component.onCompleted: {
        Backend.subscribe("colorScheme", data => root.scheme = data.scheme || "none");
        if (Backend.connected)
            refresh();
    }

    Connections {
        target: Backend

        function onConnectedChanged() {
            if (Backend.connected)
                root.refresh();
        }
    }
}
