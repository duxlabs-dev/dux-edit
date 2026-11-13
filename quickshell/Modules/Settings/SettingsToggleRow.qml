import QtQuick
import qs.DCommon.Widgets

// Clicking anywhere on the row flips the toggle.
SettingsRow {
    id: root

    property string text: ""
    property string description: ""
    property bool checked: false

    signal toggled(bool checked)

    title: text
    subtitle: description
    clickable: true
    Accessible.role: Accessible.CheckBox
    Accessible.checkable: true
    Accessible.checked: checked
    Accessible.onToggleAction: root.clicked()
    onClicked: toggled(!checked)

    DToggle {
        hideText: true
        text: root.text
        activeFocusOnTab: false
        Accessible.ignored: true
        checked: root.checked
        enabled: root.enabled
        onToggled: value => root.toggled(value)
    }
}
