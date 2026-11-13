pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Services
import "../../Common/SyntaxPalette.js" as SyntaxPalette

// The syntax colors, grouped and laid out as a grid of swatches instead of one row per role.
SettingsPage {
    id: root

    signal colorEditRequested(string role, string label, string current)

    readonly property var syntaxPalette: Highlight.palette
    readonly property bool customized: Object.keys(SettingsData.syntaxColors).length > 0
    readonly property var roleGroups: [
        {
            "title": I18n.tr("Code", "syntax color group"),
            "roles": ["text", "keyword", "type", "constant", "function", "variable", "string", "escape", "regex", "number", "operator", "punctuation", "comment", "tag", "attribute", "preproc"]
        },
        {
            "title": I18n.tr("Markup", "syntax color group"),
            "roles": ["heading", "emphasis", "strong", "link", "code"]
        },
        {
            "title": I18n.tr("Changes and Errors", "syntax color group"),
            "roles": ["inserted", "deleted", "error"]
        }
    ]

    function roleLabel(role) {
        switch (role) {
        case "text":
            return I18n.tr("Text", "syntax role");
        case "keyword":
            return I18n.tr("Keyword", "syntax role");
        case "type":
            return I18n.tr("Type", "syntax role");
        case "constant":
            return I18n.tr("Constant", "syntax role");
        case "function":
            return I18n.tr("Function", "syntax role");
        case "tag":
            return I18n.tr("Tag", "syntax role");
        case "attribute":
            return I18n.tr("Attribute", "syntax role");
        case "preproc":
            return I18n.tr("Preprocessor", "syntax role");
        case "variable":
            return I18n.tr("Variable", "syntax role");
        case "string":
            return I18n.tr("String", "syntax role");
        case "escape":
            return I18n.tr("Escape Sequence", "syntax role");
        case "regex":
            return I18n.tr("Regular Expression", "syntax role");
        case "number":
            return I18n.tr("Number", "syntax role");
        case "operator":
            return I18n.tr("Operator", "syntax role");
        case "punctuation":
            return I18n.tr("Punctuation", "syntax role");
        case "comment":
            return I18n.tr("Comment", "syntax role");
        case "heading":
            return I18n.tr("Heading", "syntax role");
        case "emphasis":
            return I18n.tr("Emphasis", "syntax role");
        case "strong":
            return I18n.tr("Strong Emphasis", "syntax role");
        case "inserted":
            return I18n.tr("Inserted Line", "syntax role");
        case "deleted":
            return I18n.tr("Deleted Line", "syntax role");
        case "error":
            return I18n.tr("Error", "syntax role");
        case "link":
            return I18n.tr("Link", "syntax role");
        case "code":
            return I18n.tr("Inline Code", "syntax role");
        }
        return role;
    }

    SettingsCard {
        SettingsToggleRow {
            text: I18n.tr("Syntax Highlighting", "setting name")
            checked: SettingsData.highlightEnabled
            onToggled: checked => SettingsData.highlightEnabled = checked
        }

        SettingsRow {
            title: I18n.tr("Custom Colors", "setting name")
            subtitle: I18n.tr("Colors follow the theme until you change them.", "setting description")
            visible: SettingsData.highlightEnabled

            DButton {
                text: I18n.tr("Reset All", "button that drops every custom syntax color")
                variant: "tonal"
                enabled: root.customized
                onClicked: SettingsData.syntaxColors = ({})
            }
        }
    }

    Repeater {
        model: SettingsData.highlightEnabled ? root.roleGroups : []

        SettingsCard {
            id: card

            required property var modelData

            title: modelData.title

            Grid {
                id: grid

                readonly property real tileMinWidth: Style.sidebarWidth

                width: parent.width
                columns: Math.max(1, Math.floor((width + spacing) / (tileMinWidth + spacing)))
                spacing: Style.groupedListGap

                Repeater {
                    model: card.modelData.roles

                    Rectangle {
                        id: tile

                        required property string modelData
                        readonly property string label: root.roleLabel(modelData)
                        readonly property bool overridden: SettingsData.syntaxColors[modelData] !== undefined
                        readonly property string roleColor: root.syntaxPalette[SyntaxPalette.roles.indexOf(modelData)]?.color ?? "transparent"

                        width: (grid.width - grid.spacing * (grid.columns - 1)) / grid.columns
                        height: Style.listItemHeight
                        radius: Style.cornerRadiusM
                        color: Style.cardSurface

                        DColorButton {
                            id: swatch

                            anchors.left: parent.left
                            anchors.leftMargin: Style.spacingS
                            anchors.verticalCenter: parent.verticalCenter
                            swatchColor: tile.roleColor
                            selected: tile.overridden
                            tooltipText: I18n.tr("Change Color", "tooltip on a syntax color swatch")
                            onClicked: root.colorEditRequested(tile.modelData, tile.label, tile.roleColor)
                        }

                        StyledText {
                            anchors.left: swatch.right
                            anchors.leftMargin: Style.spacingS
                            anchors.right: reset.left
                            anchors.verticalCenter: parent.verticalCenter
                            text: tile.label
                            color: Style.surfaceText
                            font.pixelSize: Style.fontSizeMedium
                            elide: Text.ElideRight
                            horizontalAlignment: Text.AlignLeft
                        }

                        DActionButton {
                            id: reset

                            anchors.right: parent.right
                            anchors.rightMargin: Style.spacingXS
                            anchors.verticalCenter: parent.verticalCenter
                            buttonSize: Style.buttonHeightXS
                            width: visible ? buttonSize : 0
                            visible: tile.overridden
                            iconName: "restart_alt"
                            tooltipText: I18n.tr("Use the Theme Color", "tooltip on a button that resets a syntax color")
                            onClicked: SettingsData.setSyntaxColor(tile.modelData, "")
                        }
                    }
                }
            }
        }
    }
}
