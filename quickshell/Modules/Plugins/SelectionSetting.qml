import QtQuick
import qs.DCommon.Widgets
import "SettingsLookup.js" as SettingsLookup

// options: ["a", "b"] or [{label, value}].
Item {
    id: root

    required property string settingKey
    required property string label
    property string description: ""
    required property var options
    property string defaultValue: ""
    property string value: defaultValue

    width: parent ? parent.width : 0
    implicitWidth: dropdown.implicitWidth
    implicitHeight: dropdown.implicitHeight

    function loadValue() {
        const settings = SettingsLookup.find(root.parent);
        if (settings)
            value = settings.loadValue(settingKey, defaultValue);
    }

    Component.onCompleted: loadValue()

    DDropdown {
        id: dropdown

        width: parent.width
        text: root.label
        description: root.description
        currentValue: root.value
        options: (root.options || []).map(o => typeof o === "object" ? String(o.label) : String(o))
        optionValues: (root.options || []).map(o => typeof o === "object" ? String(o.value) : String(o))
        onValueChanged: newValue => {
            root.value = newValue;
            SettingsLookup.find(root.parent)?.saveValue(root.settingKey, newValue);
        }
    }
}
