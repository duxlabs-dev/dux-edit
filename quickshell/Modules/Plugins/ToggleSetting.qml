import QtQuick
import qs.DCommon.Widgets
import "SettingsLookup.js" as SettingsLookup

DToggle {
    id: root

    required property string settingKey
    required property string label
    property bool defaultValue: false
    property bool value: defaultValue

    width: parent ? parent.width : 0
    text: label
    checked: value

    function loadValue() {
        const settings = SettingsLookup.find(root.parent);
        if (settings)
            value = settings.loadValue(settingKey, defaultValue);
    }

    Component.onCompleted: loadValue()

    onToggled: isChecked => {
        value = isChecked;
        SettingsLookup.find(root.parent)?.saveValue(settingKey, isChecked);
    }
}
