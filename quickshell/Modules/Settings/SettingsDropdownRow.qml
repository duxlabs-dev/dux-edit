import QtQuick
import qs.DCommon.Common
import qs.DCommon.Widgets

SettingsRow {
    id: root

    property string text: ""
    property string description: ""
    property string currentValue: ""
    property alias options: dropdown.options
    property alias optionValues: dropdown.optionValues
    property alias enableFuzzySearch: dropdown.enableFuzzySearch

    signal valueChanged(string value)

    title: text
    subtitle: description
    onCurrentValueChanged: dropdown.currentValue = currentValue

    DDropdown {
        id: dropdown

        enabled: root.enabled
        backgroundColor: Style.chipSurfaceNested
        Accessible.name: root.text
        width: Math.min(dropdownWidth, root.width - root.paddingH * 2)
        Component.onCompleted: currentValue = root.currentValue
        onValueChanged: value => root.valueChanged(value)
    }
}
