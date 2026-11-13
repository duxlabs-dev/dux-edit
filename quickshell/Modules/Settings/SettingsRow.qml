pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Templates as T
import qs.Common
import qs.DCommon.Common
import qs.DCommon.Widgets

// One row of a grouped settings list.
T.Control {
    id: root

    readonly property bool isSettingsRow: true

    property string title: ""
    property string subtitle: ""
    property string iconName: ""
    property bool clickable: false
    property bool showChevron: false
    property real paddingH: Style.spacingL + Style.spacingXS
    property real paddingV: Style.spacingM
    property real minHeight: subtitle !== "" ? Style.listItemTwoLineHeight : Style.listItemHeight
    property color rowColor: Style.cardSurface

    default property alias trailing: trailingSlot.data
    property alias body: bodySlot.data

    readonly property bool hasBody: bodySlot.visibleChildren.length > 0
    readonly property bool hasText: title !== "" || subtitle !== ""
    readonly property real topRadius: edgeRow(true) === root ? Style.groupedListOuterRadius : Style.groupedListInnerRadius
    readonly property real bottomRadius: edgeRow(false) === root ? Style.groupedListOuterRadius : Style.groupedListInnerRadius
    readonly property real disabledOpacity: 0.38

    signal clicked

    function edgeRow(first) {
        const siblings = parent?.visibleChildren;
        if (!siblings)
            return root;
        let last = null;
        for (let i = 0; i < siblings.length; i++) {
            const sibling = siblings[i];
            if (sibling.isSettingsRow !== true)
                continue;
            if (first)
                return sibling;
            last = sibling;
        }
        return last;
    }

    LayoutMirroring.enabled: I18n.isRtl
    LayoutMirroring.childrenInherit: true
    focusPolicy: clickable ? Qt.StrongFocus : Qt.NoFocus
    Accessible.role: clickable ? Accessible.Button : Accessible.NoRole
    Accessible.name: title
    Accessible.description: subtitle
    Accessible.onPressAction: {
        if (clickable && enabled)
            clicked();
    }

    Keys.onPressed: event => {
        if (!clickable || !enabled)
            return;
        switch (event.key) {
        case Qt.Key_Space:
        case Qt.Key_Return:
        case Qt.Key_Enter:
            root.clicked();
            event.accepted = true;
            break;
        }
    }

    width: parent?.width ?? 0
    height: implicitHeight
    implicitHeight: paddingV * 2 + headerLine.height + (hasBody ? bodySlot.height + (headerLine.height > 0 ? Style.spacingM : 0) : 0)

    Rectangle {
        anchors.fill: parent
        color: root.rowColor
        topLeftRadius: root.topRadius
        topRightRadius: root.topRadius
        bottomLeftRadius: root.bottomRadius
        bottomRightRadius: root.bottomRadius
    }

    Rectangle {
        anchors.fill: parent
        visible: root.clickable
        color: Style.surfaceText
        opacity: !root.enabled ? 0 : (clickControl.down ? Style.stateLayerPressed : (clickControl.hovered ? Style.stateLayerHover : 0))
        topLeftRadius: root.topRadius
        topRightRadius: root.topRadius
        bottomLeftRadius: root.bottomRadius
        bottomRightRadius: root.bottomRadius
    }

    DRipple {
        id: ripple

        visible: root.clickable
        rippleColor: Style.surfaceText
        topLeftRadius: root.topRadius
        topRightRadius: root.topRadius
        bottomLeftRadius: root.bottomRadius
        bottomRightRadius: root.bottomRadius
    }

    Rectangle {
        anchors.fill: parent
        anchors.margins: Style.focusRingWidth / 2
        topLeftRadius: Math.max(0, root.topRadius - Style.focusRingWidth / 2)
        topRightRadius: topLeftRadius
        bottomLeftRadius: Math.max(0, root.bottomRadius - Style.focusRingWidth / 2)
        bottomRightRadius: bottomLeftRadius
        color: "transparent"
        border.width: Style.focusRingWidth
        border.color: Style.focusRingColor
        visible: root.visualFocus && root.clickable
    }

    StyledButton {
        id: clickControl

        anchors.fill: parent
        visible: root.clickable
        enabled: root.enabled
        focusPolicy: Qt.NoFocus
        background: null
        Accessible.ignored: true
        onPressedChanged: {
            if (pressed)
                ripple.trigger(pressX, pressY);
        }
        onClicked: root.clicked()
    }

    Column {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.topMargin: root.paddingV
        anchors.leftMargin: root.paddingH
        anchors.rightMargin: root.paddingH
        spacing: Style.spacingM

        Item {
            id: headerLine

            width: parent.width
            implicitHeight: Math.max(textColumn.implicitHeight, trailingArea.implicitHeight, leadingIcon.visible ? leadingIcon.implicitHeight : 0)
            height: Math.max(implicitHeight, (root.hasText || trailingSlot.children.length > 0 ? root.minHeight : 0) - root.paddingV * 2)

            DIcon {
                id: leadingIcon

                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                name: root.iconName
                size: Style.iconSize
                color: Style.primary
                visible: root.iconName !== ""
                opacity: root.enabled ? 1 : root.disabledOpacity
            }

            Column {
                id: textColumn

                anchors.left: leadingIcon.visible ? leadingIcon.right : parent.left
                anchors.leftMargin: leadingIcon.visible ? Style.spacingL : 0
                anchors.right: trailingArea.visible ? trailingArea.left : parent.right
                anchors.rightMargin: trailingArea.visible ? Style.spacingL : 0
                anchors.verticalCenter: parent.verticalCenter
                spacing: Style.spacingXXS
                opacity: root.enabled ? 1 : root.disabledOpacity
                visible: root.hasText

                StyledText {
                    width: parent.width
                    text: root.title
                    font.pixelSize: Style.fontSizeMedium
                    font.weight: Style.fontWeightMedium
                    color: Style.surfaceText
                    wrapMode: Text.WordWrap
                    visible: root.title !== ""
                    horizontalAlignment: Text.AlignLeft
                }

                StyledText {
                    width: parent.width
                    text: root.subtitle
                    font.pixelSize: Style.fontSizeSmall
                    color: Style.surfaceVariantText
                    wrapMode: Text.WordWrap
                    visible: root.subtitle !== ""
                    horizontalAlignment: Text.AlignLeft
                }
            }

            Row {
                id: trailingArea

                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: Style.spacingS
                visible: trailingSlot.children.length > 0 || root.showChevron

                Row {
                    id: trailingSlot

                    spacing: Style.spacingS
                    anchors.verticalCenter: parent.verticalCenter
                }

                DIcon {
                    name: "chevron_right"
                    size: Style.iconSize
                    color: Style.surfaceVariantText
                    rotation: I18n.isRtl ? 180 : 0
                    visible: root.showChevron
                    anchors.verticalCenter: parent.verticalCenter
                }
            }
        }

        Column {
            id: bodySlot

            width: parent.width
            spacing: Style.spacingM
        }
    }
}
