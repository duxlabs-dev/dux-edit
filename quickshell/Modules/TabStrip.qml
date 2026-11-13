pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.DCommon.Common
import qs.DCommon.FileBrowser
import qs.DCommon.Widgets
import qs.Services

// Document tabs as pills: scrolls sideways, drags to reorder, middle-click closes, double-click renames a draft.
// Not a DReorderList, which only lays out vertically.
Item {
    id: row

    property int draggingIndex: -1
    property var order: []
    property var positions: []
    property real dragX: 0
    property real pressOffset: 0
    readonly property real tabSpacing: Style.spacingXS
    readonly property real tabHeight: Style.buttonHeightXS
    readonly property real tabMaxWidth: Style.sidebarWidth
    readonly property bool motion: !Style.reduceMotion && Style.currentAnimationSpeed !== Style.AnimationSpeed.None

    signal renameRequested(var doc)

    implicitHeight: Style.buttonHeightS
    LayoutMirroring.enabled: I18n.isRtl
    LayoutMirroring.childrenInherit: true

    function identityOrder() {
        return Array.from({
            "length": tabRepeater.count
        }, (_, index) => index);
    }

    function layout() {
        let cursor = 0;
        const next = [];
        for (const index of draggingIndex >= 0 ? order : identityOrder()) {
            const item = tabRepeater.itemAt(index);
            if (!item)
                continue;
            next[index] = cursor;
            cursor += item.width + tabSpacing;
        }
        positions = next;
        strapline.width = Math.max(0, cursor - tabSpacing);
    }

    function reset() {
        draggingIndex = -1;
        order = identityOrder();
        layout();
    }

    function physicalX(logical, width) {
        return row.LayoutMirroring.enabled ? strapline.width - logical - width : logical;
    }

    function logicalX(physical) {
        return row.LayoutMirroring.enabled ? strapline.width - physical : physical;
    }

    function begin(index, pointerX) {
        if (!tabRepeater.itemAt(index) || tabRepeater.count < 2)
            return;
        order = identityOrder();
        draggingIndex = index;
        pressOffset = pointerX - (positions[index] ?? 0);
        dragX = positions[index] ?? 0;
    }

    function dragTo(pointerX) {
        const item = tabRepeater.itemAt(draggingIndex);
        if (!item)
            return;
        dragX = Math.max(0, Math.min(strapline.width - item.width, pointerX - pressOffset));
        const from = order.indexOf(draggingIndex);
        let target = from;
        for (let slot = 0; slot < order.length; slot++) {
            const other = tabRepeater.itemAt(order[slot]);
            if (slot === from || !other)
                continue;
            const middle = positions[order[slot]] + other.width / 2;
            if (slot < from && dragX < middle)
                target = Math.min(target, slot);
            if (slot > from && dragX + item.width > middle)
                target = Math.max(target, slot);
        }
        if (target === from)
            return;
        const next = order.slice();
        next.splice(from, 1);
        next.splice(target, 0, draggingIndex);
        order = next;
        layout();
    }

    function finish() {
        const from = draggingIndex;
        if (from < 0)
            return;
        const to = order.indexOf(from);
        draggingIndex = -1;
        if (to !== from)
            Documents.moveTab(from, to);
        reset();
    }

    function reveal(index) {
        const item = tabRepeater.itemAt(index);
        if (!item)
            return;
        if (item.x < strip.contentX) {
            strip.contentX = item.x;
            return;
        }
        if (item.x + item.width > strip.contentX + strip.width)
            strip.contentX = item.x + item.width - strip.width;
    }

    readonly property int currentIndex: Documents.currentIndex
    onCurrentIndexChanged: reveal(currentIndex)

    function openTabMenu(doc, item, point) {
        tabMenu.doc = doc;
        const at = item.mapToItem(row, point.x, point.y);
        tabMenu.openAt(at.x, at.y);
    }

    // Positions are kept by index, so reset after the repeater has moved its items.
    Connections {
        target: Documents.tabModel

        function onRowsMoved() {
            if (row.draggingIndex < 0)
                Qt.callLater(row.reset);
        }
    }

    DFlickable {
        id: strip

        anchors.left: parent.left
        anchors.right: addButton.left
        anchors.rightMargin: row.tabSpacing
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        wheelEnabled: false
        interactive: false
        clip: true
        flickableDirection: Flickable.HorizontalFlick
        contentWidth: strapline.width
        contentHeight: height

        Behavior on contentX {
            enabled: row.motion && row.draggingIndex < 0

            DAnim {
                duration: Style.expressiveDurations.expressiveEffects
                easing.bezierCurve: Style.expressiveCurves.expressiveEffects
            }
        }

        WheelHandler {
            acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
            onWheel: event => {
                const delta = event.angleDelta.y !== 0 ? event.angleDelta.y : event.angleDelta.x;
                strip.contentX = Math.max(0, Math.min(Math.max(0, strip.contentWidth - strip.width), strip.contentX - delta));
            }
        }

        Item {
            id: strapline

            height: strip.height

            Repeater {
                id: tabRepeater

                model: Documents.tabModel
                onItemRemoved: row.reset()

                StyledButton {
                    id: tab

                    required property int index
                    required property string tabId
                    readonly property EditorDocument doc: Documents.byTabId(tabId)
                    readonly property bool current: index === Documents.currentIndex
                    readonly property bool dragging: row.draggingIndex === index
                    readonly property color contentColor: current ? Style.onSelectedContainer : Style.onSurfaceVariant

                    x: row.physicalX(dragging ? row.dragX : (row.positions[index] ?? 0), width)
                    z: dragging ? 1 : 0
                    focusPolicy: Qt.TabFocus
                    implicitWidth: Math.max(Style.tabMinWidth, Math.min(row.tabMaxWidth, title.implicitWidth + dirtyDot.width + closeButton.width + Style.spacingL + Style.spacingXS))
                    y: (row.height - height) / 2
                    implicitHeight: row.tabHeight
                    radius: Style.fullRadius(width, height)
                    color: current ? Style.selectedContainer : (dragging ? Style.chipSurface : "transparent")
                    Accessible.role: Accessible.PageTab
                    Accessible.name: tab.doc?.title ?? ""
                    Accessible.selected: current
                    onWidthChanged: row.layout()
                    Component.onCompleted: row.reset()
                    onClicked: Documents.activate(index)
                    onDoubleClicked: {
                        if (tab.doc?.isDraft)
                            row.renameRequested(tab.doc);
                    }

                    StateLayer {
                        control: tab
                        stateColor: tab.contentColor
                        tooltipText: tab.doc?.path ? AppPaths.shortenHome(tab.doc.path) : null
                    }

                    FocusRing {
                        visible: tab.visualFocus
                    }

                    TapHandler {
                        acceptedButtons: Qt.MiddleButton
                        onTapped: Documents.closeTab(tab.index, false)
                    }

                    TapHandler {
                        acceptedButtons: Qt.RightButton
                        onTapped: eventPoint => row.openTabMenu(tab.doc, tab, eventPoint.position)
                    }

                    Rectangle {
                        id: dirtyDot

                        anchors.left: parent.left
                        anchors.leftMargin: Style.spacingM
                        anchors.verticalCenter: parent.verticalCenter
                        width: visible ? Style.spacingS : 0
                        height: Style.spacingS
                        radius: height / 2
                        color: tab.contentColor
                        visible: tab.doc?.dirty ?? false
                    }

                    StyledText {
                        id: title

                        anchors.left: dirtyDot.right
                        anchors.leftMargin: dirtyDot.visible ? Style.spacingXS : 0
                        anchors.right: closeButton.left
                        anchors.rightMargin: Style.spacingXXS
                        anchors.verticalCenter: parent.verticalCenter
                        text: tab.doc?.title ?? ""
                        color: tab.contentColor
                        font.pixelSize: Style.fontSizeSmall
                        font.weight: tab.current ? Style.fontWeightMedium : Style.fontWeight
                        elide: Text.ElideMiddle
                    }

                    DActionButton {
                        id: closeButton

                        anchors.right: parent.right
                        anchors.rightMargin: Style.spacingXXS
                        anchors.verticalCenter: parent.verticalCenter
                        buttonSize: Style.buttonHeightXXS
                        iconName: "close"
                        iconSize: Style.iconSizeSmall
                        iconColor: tab.contentColor
                        opacity: tab.current || tab.hovered || hovered || visualFocus ? 1 : 0
                        tooltipText: I18n.tr("Close Tab", "tooltip on the button closing a tab")
                        onClicked: Documents.closeTab(tab.index, false)
                    }

                    Behavior on x {
                        enabled: row.draggingIndex >= 0 && !tab.dragging && row.motion

                        DAnim {
                            duration: Style.expressiveDurations.expressiveFastSpatial
                            easing.bezierCurve: Style.expressiveCurves.expressiveFastSpatial
                        }
                    }

                    DragHandler {
                        target: null
                        yAxis.enabled: false
                        enabled: tabRepeater.count > 1
                        onGrabChanged: (transition, point) => {
                            switch (transition) {
                            case PointerDevice.GrabExclusive:
                                row.begin(tab.index, row.logicalX(strapline.mapFromItem(null, point.scenePressPosition.x, 0).x));
                                row.dragTo(row.logicalX(strapline.mapFromItem(null, point.scenePosition.x, 0).x));
                                return;
                            case PointerDevice.UngrabExclusive:
                                row.finish();
                                return;
                            case PointerDevice.CancelGrabExclusive:
                                row.reset();
                                return;
                            }
                        }
                        onCentroidChanged: {
                            if (!active || !tab.dragging)
                                return;
                            row.dragTo(row.logicalX(strapline.mapFromItem(null, centroid.scenePosition.x, 0).x));
                        }
                    }
                }
            }
        }
    }

    DActionButton {
        id: addButton

        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        focusPolicy: Qt.TabFocus
        buttonSize: Style.buttonHeightXS
        iconName: "add"
        iconSize: Style.iconSizeMedium
        iconColor: Style.onSurfaceVariant
        tooltipText: I18n.tr("New Tab", "tooltip on the button opening a new tab")
        onClicked: Documents.newDraft()
    }

    FileMenu {
        id: tabMenu

        property EditorDocument doc: null
        readonly property int index: doc ? Documents.tabs.indexOf(doc) : -1

        width: Style.spacingXL * 12
        onClosed: doc = null

        FileMenuItem {
            iconName: "close"
            text: I18n.tr("Close", "tab menu item")
            shortcutText: Shortcuts.hint("closeTab")
            onClicked: {
                const doc = tabMenu.doc;
                tabMenu.close();
                Documents.closeTab(Documents.tabs.indexOf(doc), false);
            }
        }

        FileMenuItem {
            iconName: "tab_close"
            enabled: Documents.tabs.length > 1
            text: I18n.tr("Close Others", "tab menu item")
            onClicked: {
                const doc = tabMenu.doc;
                tabMenu.close();
                Documents.closeOthers(doc);
            }
        }

        FileMenuItem {
            iconName: "tab_close_right"
            enabled: tabMenu.index >= 0 && tabMenu.index < Documents.tabs.length - 1
            text: I18n.tr("Close to the Right", "tab menu item")
            onClicked: {
                const doc = tabMenu.doc;
                tabMenu.close();
                Documents.closeToRight(doc);
            }
        }

        FileMenuItem {
            iconName: "tab"
            enabled: Documents.closedTabs.length > 0
            text: I18n.tr("Reopen Closed Tab", "tab menu item")
            shortcutText: Shortcuts.hint("reopenTab")
            onClicked: {
                tabMenu.close();
                Documents.reopenClosedTab();
            }
        }

        FileMenuDivider {}

        FileMenuItem {
            iconName: "content_copy"
            visible: tabMenu.doc !== null && !tabMenu.doc.isDraft
            text: I18n.tr("Copy Path", "tab menu item")
            onClicked: {
                const path = tabMenu.doc.path;
                tabMenu.close();
                Quickshell.clipboardText = path;
                ToastService.showInfo(I18n.tr("Path copied", "notice after copying a file path"));
            }
        }

        FileMenuItem {
            iconName: "edit"
            visible: tabMenu.doc?.isDraft ?? false
            text: I18n.tr("Rename…", "tab menu item that names an unsaved document")
            onClicked: {
                const doc = tabMenu.doc;
                tabMenu.close();
                row.renameRequested(doc);
            }
        }
    }
}
