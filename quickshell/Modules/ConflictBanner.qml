pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Services

// Shown while the current document's file changed or vanished on disk under unsaved edits.
Rectangle {
    id: root

    property EditorDocument doc: null
    readonly property string conflict: doc?.conflict ?? ""
    readonly property bool deleted: conflict === "deleted"

    visible: conflict !== ""
    implicitHeight: visible ? content.implicitHeight + Style.spacingS * 2 : 0
    color: Style.errorContainer

    Row {
        id: content

        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        anchors.leftMargin: Style.spacingM
        anchors.rightMargin: Style.spacingS
        spacing: Style.spacingS

        DIcon {
            anchors.verticalCenter: parent.verticalCenter
            name: root.deleted ? "delete" : "sync_problem"
            size: Style.iconSizeMedium
            color: Style.onErrorContainer
        }

        StyledText {
            anchors.verticalCenter: parent.verticalCenter
            width: content.width - x - actions.width - content.spacing
            text: root.deleted ? I18n.tr("This file was deleted from disk.", "conflict banner message") : I18n.tr("This file changed on disk.", "conflict banner message")
            color: Style.onErrorContainer
            font.pixelSize: Style.fontSizeMedium
            elide: Text.ElideRight
        }

        Row {
            id: actions

            anchors.verticalCenter: parent.verticalCenter
            spacing: Style.spacingXS

            DButton {
                visible: !root.deleted
                text: I18n.tr("Reload From Disk", "conflict banner button that drops unsaved edits")
                variant: "text"
                buttonHeight: Style.buttonHeightXS
                textColor: Style.onErrorContainer
                onClicked: Documents.reloadFromDisk(root.doc, null)
            }

            DButton {
                text: root.deleted ? I18n.tr("Save", "conflict banner button that writes the file again") : I18n.tr("Keep My Edits", "conflict banner button that overwrites the file on disk")
                variant: "filled"
                buttonHeight: Style.buttonHeightXS
                onClicked: Documents.keepMine(root.doc)
            }

            DActionButton {
                anchors.verticalCenter: parent.verticalCenter
                iconName: "close"
                iconSize: Style.iconSizeSmall
                iconColor: Style.onErrorContainer
                tooltipText: I18n.tr("Dismiss", "tooltip on a button that hides a banner")
                onClicked: root.doc.conflict = ""
            }
        }
    }
}
