import QtQuick
import qs.DCommon.Common
import qs.DCommon.Widgets

SettingsRow {
    id: root

    property string text: ""
    property string description: ""
    property int value: 0
    property alias minimum: slider.minimum
    property alias maximum: slider.maximum
    property alias step: slider.step
    property alias unit: slider.unit
    // The value is stored times 10^decimals; the label divides it back.
    property alias decimals: slider.decimals

    signal sliderValueChanged(int newValue)

    title: text
    subtitle: description
    onValueChanged: slider.value = value

    DSlider {
        id: slider

        width: Style.sidebarWidth
        enabled: root.enabled
        wheelEnabled: false
        Accessible.name: root.text
        Component.onCompleted: value = root.value
        onSliderValueChanged: newValue => root.sliderValueChanged(newValue)
    }
}
