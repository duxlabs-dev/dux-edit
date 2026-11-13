pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Services

// Shown while the current document's file changed or vanished on disk under unsaved edits, and, before any
// save fails, for a file that cannot be written as it is.
Rectangle {
    id: root

    property EditorDocument doc: null
    readonly property string conflict: doc?.conflict ?? ""
    readonly property bool deleted: conflict === "deleted"
    readonly property bool fileLoaded: (doc?.loaded ?? false) && !doc.isDraft
    readonly property bool unwritable: fileLoaded && !doc.writable && !doc.elevated
    // Its encoding cannot write the file's own bytes back.
    readonly property bool lossy: fileLoaded && doc.readOnly && !unwritable
    readonly property bool readOnlyNotice: conflict === "" && (unwritable || lossy) && !doc.readOnlyNoticeDismissed
    readonly property color contentColor: conflict !== "" ? Style.onErrorContainer : Style.onSecondaryContainer

    readonly property string message: {
        if (deleted)
            return I18n.tr("This file was deleted from disk.", "conflict banner message");
        if (conflict !== "")
            return I18n.tr("This file changed on disk.", "conflict banner message");
        if (unwritable)
            return I18n.tr("You do not have permission to change this file.", "banner message for a file the user cannot write");
        return I18n.tr("Read-only: this file's encoding cannot write some of its bytes back. Save it as UTF-8 to edit it.", "banner message for a file opened read-only because of its encoding");
    }

    visible: conflict !== "" || readOnlyNotice
    implicitHeight: visible ? content.implicitHeight + Style.spacingS * 2 : 0
    color: conflict !== "" ? Style.errorContainer : Style.secondaryContainer

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
            name: root.deleted ? "delete" : (root.conflict !== "" ? "sync_problem" : "lock")
            size: Style.iconSizeMedium
            color: root.contentColor
        }

        StyledText {
            anchors.verticalCenter: parent.verticalCenter
            width: content.width - x - actions.width - content.spacing
            text: root.message
            color: root.contentColor
            font.pixelSize: Style.fontSizeMedium
            elide: Text.ElideRight
        }

        Row {
            id: actions

            anchors.verticalCenter: parent.verticalCenter
            spacing: Style.spacingXS

            DButton {
                visible: root.conflict !== "" && !root.deleted
                text: I18n.tr("Reload From Disk", "conflict banner button that drops unsaved edits")
                variant: "text"
                buttonHeight: Style.buttonHeightXS
                textColor: root.contentColor
                onClicked: Documents.reloadFromDisk(root.doc, null)
            }

            DButton {
                visible: root.conflict !== ""
                text: root.deleted ? I18n.tr("Save", "conflict banner button that writes the file again") : I18n.tr("Keep My Edits", "conflict banner button that overwrites the file on disk")
                variant: "filled"
                buttonHeight: Style.buttonHeightXS
                onClicked: Documents.keepMine(root.doc)
            }

            DButton {
                visible: root.readOnlyNotice && root.unwritable && root.doc.readOnly
                text: I18n.tr("Edit Anyway", "banner button that allows editing a file the user cannot write")
                variant: "text"
                buttonHeight: Style.buttonHeightXS
                textColor: root.contentColor
                onClicked: root.doc.readOnly = false
            }

            DButton {
                visible: root.readOnlyNotice && root.unwritable && root.doc.dirty
                text: I18n.tr("Save as Administrator", "button that writes a file with administrator rights")
                variant: "text"
                buttonHeight: Style.buttonHeightXS
                textColor: root.contentColor
                onClicked: Documents.saveWith(root.doc, {
                    "elevated": true
                }, null, false)
            }

            DButton {
                visible: root.readOnlyNotice
                text: root.lossy ? I18n.tr("Save as UTF-8…", "banner button that saves a copy of a file in UTF-8") : I18n.tr("Save As…", "banner button that saves the document under another name")
                variant: "filled"
                buttonHeight: Style.buttonHeightXS
                onClicked: Documents.saveAs(root.doc, null, root.lossy ? {
                    "encoding": "utf-8",
                    "bom": false
                } : null)
            }

            DActionButton {
                anchors.verticalCenter: parent.verticalCenter
                iconName: "close"
                iconSize: Style.iconSizeSmall
                iconColor: root.contentColor
                tooltipText: I18n.tr("Dismiss", "tooltip on a button that hides a banner")
                onClicked: {
                    if (root.conflict !== "") {
                        root.doc.conflict = "";
                        return;
                    }
                    root.doc.readOnlyNoticeDismissed = true;
                }
            }
        }
    }
}
