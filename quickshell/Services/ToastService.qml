pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell

// One toast at a time. showInfo/showWarning/showError match what DMS plugins call.
Singleton {
    id: root

    property string message: ""
    property string actionLabel: ""
    property var action: null
    // "info" | "warning" | "error"
    property string level: "info"
    property bool active: false

    // opts: {level, actionLabel, action, duration}
    function show(msg, opts) {
        message = msg;
        level = opts?.level || "info";
        actionLabel = opts?.actionLabel || "";
        action = typeof opts?.action === "function" ? opts.action : null;
        active = true;
        timer.interval = opts?.duration || (actionLabel !== "" ? 10000 : (level === "info" ? 3000 : 6000));
        timer.restart();
    }

    function dismiss() {
        timer.stop();
        active = false;
        actionLabel = "";
        action = null;
    }

    function runAction() {
        const pending = action;
        dismiss();
        pending?.();
    }

    function showInfo(msg) {
        show(msg, null);
    }

    function showWarning(msg) {
        show(msg, {
            "level": "warning"
        });
    }

    function showError(msg) {
        show(msg, {
            "level": "error"
        });
    }

    readonly property Timer _timer: Timer {
        id: timer
        onTriggered: root.dismiss()
    }
}
