pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common

// Asks the core once, when blur is first wanted, whether the compositor can blur behind windows.
Singleton {
    id: root

    property bool available: false
    property bool probed: false
    property bool probing: false

    readonly property bool requested: SettingsData.blurEnabled
    readonly property bool enabled: available && requested

    function probe(): void {
        if (probed || probing || !Backend.connected)
            return;
        probing = true;
        Backend.call("system.blur", null, result => {
            root.probing = false;
            if (result.error)
                return;
            root.available = result.supported === true;
            root.probed = true;
        });
    }

    onRequestedChanged: {
        if (requested)
            probe();
    }

    Component.onCompleted: {
        if (requested)
            probe();
    }

    Connections {
        target: Backend

        function onConnectedChanged() {
            if (Backend.connected && root.requested)
                root.probe();
        }
    }
}
