import QtQuick
import qs.DCommon.Common
import qs.DCommon.Widgets
import "SettingsLookup.js" as SettingsLookup

Column {
    id: root

    required property string settingKey
    required property string label
    property string description: ""
    property string placeholder: ""
    property string defaultValue: ""
    property string value: defaultValue

    width: parent ? parent.width : 0
    spacing: Style.spacingS

    function loadValue() {
        if (SettingsLookup.load(root) && !textField.activeFocus)
            textField.text = value;
    }

    function commit() {
        if (textField.text === value)
            return;
        SettingsLookup.save(root, textField.text);
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

    DTextField {
        id: textField

        width: parent.width
        placeholderText: root.placeholder
        onEditingFinished: root.commit()
        onFocusStateChanged: hasFocus => {
            if (!hasFocus)
                root.commit();
        }
    }
}
