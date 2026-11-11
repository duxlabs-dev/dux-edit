pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services

Singleton {
    id: root

    property int value: 0

    function add(amount) {
        Backend.call("counter.add", {
            "amount": amount
        }, null);
    }

    function reset() {
        Backend.call("counter.reset", null, null);
    }

    Component.onCompleted: Backend.subscribe("counter", state => root.value = state.value)
}
