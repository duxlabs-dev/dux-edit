import QtQuick
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Modules.Editor

// One scrolling settings subpage: a centered column of SettingsCards.
DFlickable {
    id: root

    default property alias content: column.data

    clip: true
    contentWidth: width
    contentHeight: column.height + Style.spacingXL

    Column {
        id: column

        anchors.horizontalCenter: parent.horizontalCenter
        width: Math.min(root.width - Style.spacingL * 2, EditorMetrics.settingsPageWidth)
        topPadding: Style.spacingXS
        spacing: Style.spacingL
    }
}
