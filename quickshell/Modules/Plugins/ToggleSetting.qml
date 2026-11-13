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
        SettingsLookup.load(root);
    }

    Component.onCompleted: loadValue()

    onToggled: isChecked => SettingsLookup.save(root, isChecked)
}
