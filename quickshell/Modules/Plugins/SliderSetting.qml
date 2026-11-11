import QtQuick
import qs.DCommon.Common
import qs.DCommon.Widgets
import "SettingsLookup.js" as SettingsLookup

Column {
    id: root

    required property string settingKey
    required property string label
    property string description: ""
    property int defaultValue: 0
    property int value: defaultValue
    property int minimum: 0
    property int maximum: 100
    property int step: 1
    property string startIcon: ""
    property string endIcon: ""
    property string unit: ""

    width: parent ? parent.width : 0
    spacing: Style.spacingS

    function loadValue() {
        const settings = SettingsLookup.find(root.parent);
        if (settings)
            value = settings.loadValue(settingKey, defaultValue);
    }

    Component.onCompleted: loadValue()

    StyledText {
        width: parent.width
        text: root.label
        font.pixelSize: Style.fontSizeMedium
        font.weight: Style.fontWeightMedium
        color: Style.surfaceText
        wrapMode: Text.Wrap
    }

    StyledText {
        width: parent.width
        visible: root.description !== ""
        text: root.description
        font.pixelSize: Style.fontSizeSmall
        color: Style.surfaceVariantText
        wrapMode: Text.Wrap
    }

    DSlider {
        width: parent.width
        upDownKeysStep: false
        wheelEnabled: false
        value: root.value
        minimum: root.minimum
        maximum: root.maximum
        step: root.step
        startIcon: root.startIcon
        endIcon: root.endIcon
        unit: root.unit
        onSliderValueChanged: newValue => {
            root.value = newValue;
            SettingsLookup.find(root.parent)?.saveValue(root.settingKey, newValue);
        }
    }
}
