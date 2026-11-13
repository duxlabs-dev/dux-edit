pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Services

// Shortcuts grouped by area with keycaps, filtered as you type.
SettingsPage {
    id: root

    readonly property string query: search.text.trim().toLowerCase()
    readonly property var pluginEntries: PluginService.commands.filter(command => !!command.shortcut).map(command => ({
                "label": command.title,
                "keys": [command.shortcut]
            }))
    readonly property var sections: Shortcuts.groups.map(group => ({
                "label": group.label,
                "entries": Shortcuts.inGroup(group.id)
            })).concat([
        {
            "label": I18n.tr("Plugins", "shortcut group"),
            "entries": pluginEntries
        }
    ]).map(section => ({
                "label": section.label,
                "entries": section.entries.filter(entry => root.matches(entry))
            })).filter(section => section.entries.length > 0)

    function matches(entry) {
        if (query === "")
            return true;
        return entry.label.toLowerCase().includes(query) || entry.keys.some(key => key.toLowerCase().includes(query));
    }

    DSearchField {
        id: search

        width: parent.width
        placeholderText: I18n.tr("Search shortcuts", "placeholder of the shortcut search field")
    }

    StyledText {
        width: parent.width
        visible: root.sections.length === 0
        text: I18n.tr("No shortcuts match.", "empty state of the shortcut search")
        color: Style.surfaceVariantText
        font.pixelSize: Style.fontSizeMedium
        horizontalAlignment: Text.AlignHCenter
    }

    Repeater {
        model: root.sections

        SettingsCard {
            id: card

            required property var modelData

            title: modelData.label

            Repeater {
                model: card.modelData.entries

                SettingsRow {
                    id: row

                    required property var modelData

                    title: modelData.label
                    paddingV: Style.spacingS
                    minHeight: Style.buttonHeightS + Style.spacingS

                    Repeater {
                        model: row.modelData.keys

                        Row {
                            id: sequence

                            required property string modelData
                            required property int index

                            spacing: Style.spacingXXS

                            StyledText {
                                anchors.verticalCenter: parent.verticalCenter
                                visible: sequence.index > 0
                                text: "/"
                                color: Style.surfaceVariantText
                                font.pixelSize: Style.fontSizeSmall
                                rightPadding: Style.spacingXS
                            }

                            Repeater {
                                model: Shortcuts.caps(sequence.modelData)

                                DKeycap {
                                    required property string modelData

                                    text: modelData
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}
