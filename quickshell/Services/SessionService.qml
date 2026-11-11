pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell

// DC.Host.session: the power actions DCommon's session widgets call. An editor offers none of them.
Singleton {
    readonly property bool hibernateSupported: false

    function logout() {
    }

    function suspend() {
    }

    function hibernate() {
    }

    function reboot() {
    }

    function poweroff() {
    }
}
