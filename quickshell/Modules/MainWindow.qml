pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Services

FloatingWindow {
    id: window

    title: "My App"
    implicitWidth: 420
    implicitHeight: 340
    minimumSize: Qt.size(320, 280)
    color: Style.surface

    onClosed: Backend.quit()

    FloatingWindowControls {
        id: windowControls
        targetWindow: window
    }

    DWindowHeader {
        id: header

        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        controls: windowControls
        title: window.title
        onCloseRequested: Backend.quit()
    }

    Column {
        anchors.centerIn: parent
        spacing: Style.spacingL

        StyledText {
            anchors.horizontalCenter: parent.horizontalCenter
            text: Counter.value
            font.pixelSize: 72
            font.weight: Font.Medium
            color: Style.primary
        }

        Row {
            anchors.horizontalCenter: parent.horizontalCenter
            spacing: Style.spacingS

            DButton {
                text: "Add one"
                iconName: "add"
                enabled: Backend.connected
                onClicked: Counter.add(1)
            }

            DButton {
                text: "Reset"
                variant: "tonal"
                enabled: Backend.connected
                onClicked: Counter.reset()
            }
        }

        StyledText {
            anchors.horizontalCenter: parent.horizontalCenter
            text: Backend.connected ? "backend " + Backend.version : "backend not connected"
            font.pixelSize: Style.fontSizeSmall
            color: Style.surfaceVariantText
        }
    }
}
