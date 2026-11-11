pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Services

// Host for ToastService: one snackbar at the bottom of the window.
Item {
    id: root

    readonly property bool motion: !Style.reduceMotion && Style.currentAnimationSpeed !== Style.AnimationSpeed.None

    z: 1000
    visible: bar.opacity > 0

    Rectangle {
        id: bar

        readonly property bool showing: ToastService.active

        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        anchors.bottomMargin: Style.spacingL + (showing ? 0 : Style.spacingS)
        height: Style.fieldHeightLarge
        width: Math.min(parent.width - Style.spacingL * 2, contentRow.implicitWidth + Style.spacingL * 2)
        radius: Style.cornerRadiusXS
        color: Style.inverseSurface
        opacity: showing ? 1 : 0

        Behavior on opacity {
            enabled: root.motion

            DAnim {
                duration: Style.expressiveDurations.expressiveEffects
                easing.bezierCurve: Style.expressiveCurves.expressiveEffects
            }
        }

        Behavior on anchors.bottomMargin {
            enabled: root.motion

            DAnim {
                duration: Style.expressiveDurations.expressiveEffects
                easing.bezierCurve: Style.expressiveCurves.standardDecel
            }
        }

        Row {
            id: contentRow

            anchors.centerIn: parent
            spacing: Style.spacingM

            DIcon {
                anchors.verticalCenter: parent.verticalCenter
                visible: ToastService.level !== "info"
                name: ToastService.level === "error" ? "error" : "warning"
                size: Style.iconSizeMedium
                color: Style.inverseOnSurface
            }

            StyledText {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, root.width - Style.spacingL * 4 - action.width - closeButton.width - contentRow.spacing * 3 - Style.iconSizeMedium)
                text: ToastService.message
                font.pixelSize: Style.fontSizeMedium
                color: Style.inverseOnSurface
                wrapMode: Text.NoWrap
                maximumLineCount: 1
                elide: Text.ElideRight
            }

            DButton {
                id: action

                anchors.verticalCenter: parent.verticalCenter
                visible: ToastService.actionLabel !== ""
                text: ToastService.actionLabel
                buttonHeight: Style.buttonHeightXS
                variant: "text"
                textColor: Style.inverseOnSurface
                focusPolicy: Qt.NoFocus
                onClicked: ToastService.runAction()
            }

            DActionButton {
                id: closeButton

                anchors.verticalCenter: parent.verticalCenter
                iconName: "close"
                iconSize: Style.iconSizeSmall
                iconColor: Style.inverseOnSurface
                focusPolicy: Qt.NoFocus
                tooltipText: I18n.tr("Dismiss", "tooltip on a button that hides a banner")
                onClicked: ToastService.dismiss()
            }
        }
    }
}
