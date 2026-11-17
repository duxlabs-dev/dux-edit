pragma ComponentBehavior: Bound

import QtQuick
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Modules.Editor

DListView {
    id: root

    property var pages: []
    property string current: ""

    signal pageSelected(string id)

    function step(delta) {
        const next = Math.max(0, Math.min(count - 1, currentIndex + delta));
        if (next === currentIndex)
            return;
        pageSelected(pages[next].id);
        currentItem?.forceActiveFocus();
    }

    function focusCurrent() {
        currentItem?.forceActiveFocus();
    }

    model: pages
    currentIndex: pages.findIndex(entry => entry.id === current)
    keyNavigationEnabled: false
    spacing: Style.groupedListGap
    Accessible.role: Accessible.List
    Keys.onUpPressed: step(-1)
    Keys.onDownPressed: step(1)

    delegate: DListItem {
        id: item

        required property int index
        required property var modelData

        width: root.width
        height: EditorMetrics.settingsRowMinHeight
        isSelected: root.currentIndex === index
        firstInGroup: index === 0
        lastInGroup: index === root.count - 1
        tooltipText: modelData.hint
        Accessible.name: modelData.title
        Accessible.description: modelData.hint
        onClicked: root.pageSelected(modelData.id)

        DIcon {
            id: icon

            anchors.left: parent.left
            anchors.leftMargin: Style.spacingL
            anchors.verticalCenter: parent.verticalCenter
            name: item.modelData.icon
            size: Style.iconSize
            color: item.isSelected ? Style.accentOnSelectedContainer : Style.primary
        }

        StyledText {
            anchors.left: icon.right
            anchors.leftMargin: Style.spacingM
            anchors.right: parent.right
            anchors.rightMargin: Style.spacingL
            anchors.verticalCenter: parent.verticalCenter
            text: item.modelData.title
            color: item.contentColor
            font.pixelSize: Style.fontSizeMedium
            font.weight: Style.fontWeightMedium
            elide: Text.ElideRight
            horizontalAlignment: Text.AlignLeft
        }
    }
}
