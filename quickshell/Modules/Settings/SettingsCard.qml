import QtQuick
import qs.DCommon.Common
import qs.DCommon.Widgets

// A titled group of SettingsRows drawn as one grouped list.
Column {
    id: root

    property string title: ""
    default property alias content: group.data

    width: parent?.width ?? 0
    spacing: Style.spacingS

    StyledText {
        width: parent.width
        leftPadding: Style.spacingM
        text: root.title
        visible: root.title !== ""
        color: Style.primary
        font.pixelSize: Style.fontSizeMedium
        font.weight: Style.fontWeightMedium
        horizontalAlignment: Text.AlignLeft
    }

    Column {
        id: group

        width: parent.width
        spacing: Style.groupedListGap
    }
}
