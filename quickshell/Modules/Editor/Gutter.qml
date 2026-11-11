pragma ComponentBehavior: Bound

import QtQuick
import qs.DCommon.Common
import qs.DCommon.Widgets

// Line numbers for the lines the pane built around the viewport. Slots are in editor coordinates; the
// column only moves with contentY, so scrolling inside the built range rebuilds nothing.
Item {
    id: root

    // EditorPane.slots: {line, y, height} or null, 0-based lines.
    property var slots: []
    property int currentLine: 0
    property int lineCount: 1
    property real contentY: 0
    property font font

    readonly property int digits: Math.max(2, String(lineCount).length)

    implicitWidth: metrics.advanceWidth("0".repeat(digits)) + EditorMetrics.gutterPadding * 2
    clip: true

    FontMetrics {
        id: metrics
        font: root.font
    }

    Item {
        y: -root.contentY
        width: root.width

        Repeater {
            model: root.slots.length

            StyledText {
                required property int index

                readonly property var slot: root.slots[index] ?? null
                readonly property int line: slot?.line ?? -1

                visible: slot !== null
                x: EditorMetrics.gutterPadding
                y: slot?.y ?? 0
                width: root.width - EditorMetrics.gutterPadding * 2
                height: slot?.height ?? 0
                text: line + 1
                font.family: root.font.family
                font.pixelSize: root.font.pixelSize
                font.weight: line === root.currentLine ? Style.fontWeightBold : Style.fontWeight
                color: line === root.currentLine ? Style.onSurface : Style.onSurfaceVariant
                horizontalAlignment: Text.AlignRight
                verticalAlignment: Text.AlignTop
                wrapMode: Text.NoWrap
                elide: Text.ElideNone
            }
        }
    }
}
