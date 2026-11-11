pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell

// DC.Host.cache. Only DCommon's file browser reads it, which the editor does not show, so nothing is persisted.
Singleton {
    property var fileBrowserSettings: ({})

    function saveCache() {
    }
}
