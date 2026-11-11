pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Modules.Editor
import qs.Modules.Plugins
import qs.Services
import "../Common/StockTheme.js" as StockTheme
import "../Common/SyntaxPalette.js" as SyntaxPalette

// In-window settings page. Loaded only while open.
Rectangle {
    id: root

    signal closeRequested
    signal colorEditRequested(string role, string label, string current)

    readonly property real pageWidth: Math.min(width - Style.spacingL * 2, EditorMetrics.settingsPageWidth)
    readonly property var syntaxPalette: SyntaxPalette.build(Style, SettingsData.syntaxColors)

    readonly property var shortcuts: [
        ["Ctrl+N", I18n.tr("New document", "shortcut description")],
        ["Ctrl+O", I18n.tr("Open files", "shortcut description")],
        ["Ctrl+S", I18n.tr("Save", "shortcut description")],
        ["Ctrl+Shift+S", I18n.tr("Save as", "shortcut description")],
        ["Ctrl+W", I18n.tr("Close tab", "shortcut description")],
        ["Ctrl+Tab", I18n.tr("Next tab", "shortcut description")],
        ["Ctrl+Shift+Tab", I18n.tr("Previous tab", "shortcut description")],
        ["Ctrl+F", I18n.tr("Find", "shortcut description")],
        ["Ctrl+H", I18n.tr("Replace", "shortcut description")],
        ["Ctrl+G", I18n.tr("Go to line", "shortcut description")],
        ["Ctrl+P", I18n.tr("Cycle the Markdown preview", "shortcut description")],
        ["Ctrl++ / Ctrl+-", I18n.tr("Zoom in and out", "shortcut description")],
        ["Ctrl+0", I18n.tr("Reset zoom", "shortcut description")],
        ["Ctrl+,", I18n.tr("Settings", "shortcut description")]
    ]

    function presetLabel(name) {
        switch (name) {
        case "purple":
            return I18n.tr("Purple", "color palette name");
        case "blue":
            return I18n.tr("Blue", "color palette name");
        case "green":
            return I18n.tr("Green", "color palette name");
        case "orange":
            return I18n.tr("Orange", "color palette name");
        case "red":
            return I18n.tr("Red", "color palette name");
        case "cyan":
            return I18n.tr("Cyan", "color palette name");
        case "pink":
            return I18n.tr("Pink", "color palette name");
        case "amber":
            return I18n.tr("Amber", "color palette name");
        case "coral":
            return I18n.tr("Coral", "color palette name");
        case "monochrome":
            return I18n.tr("Monochrome", "color palette name");
        }
        return StockTheme.presetLabel(name);
    }

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

    function setSyntaxColor(role, color) {
        const next = Object.assign({}, SettingsData.syntaxColors);
        if (color === "")
            delete next[role];
        else
            next[role] = color;
        SettingsData.syntaxColors = next;
    }

    color: Style.hostSurface
    Component.onCompleted: Spell.loadLanguages()
    Keys.onEscapePressed: closeRequested()

    component Section: Column {
        id: section

        property string title: ""
        default property alias rows: card.data

        width: root.pageWidth
        spacing: Style.spacingS

        StyledText {
            text: section.title
            color: Style.primary
            font.pixelSize: Style.fontSizeMedium
            font.weight: Style.fontWeightMedium
            leftPadding: Style.spacingM
        }

        Rectangle {
            width: parent.width
            height: card.implicitHeight + Style.spacingS * 2
            radius: Style.cornerRadiusL
            color: Style.cardSurface

            Column {
                id: card

                x: Style.spacingS
                y: Style.spacingS
                width: parent.width - Style.spacingS * 2
            }
        }
    }

    component SettingRow: Item {
        id: settingRow

        property string text: ""
        property string description: ""
        default property alias control: slot.data

        width: parent?.width ?? 0
        implicitHeight: Math.max(Style.listItemHeight, labels.implicitHeight + Style.spacingM * 2, slot.childrenRect.height + Style.spacingM)

        Column {
            id: labels

            anchors.left: parent.left
            anchors.leftMargin: Style.spacingM
            anchors.right: slot.left
            anchors.rightMargin: Style.spacingM
            anchors.verticalCenter: parent.verticalCenter

            StyledText {
                width: parent.width
                text: settingRow.text
                color: Style.surfaceText
                font.pixelSize: Style.fontSizeMedium
                elide: Text.ElideRight
            }

            StyledText {
                width: parent.width
                visible: text !== ""
                text: settingRow.description
                color: Style.surfaceVariantText
                font.pixelSize: Style.fontSizeSmall
                wrapMode: Text.Wrap
            }
        }

        Item {
            id: slot

            anchors.right: parent.right
            anchors.rightMargin: Style.spacingM
            anchors.verticalCenter: parent.verticalCenter
            width: childrenRect.width
            height: childrenRect.height
        }
    }

    Item {
        id: header

        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: Style.buttonHeightS + Style.spacingS * 2

        DActionButton {
            id: back

            anchors.left: parent.left
            anchors.leftMargin: Style.spacingS
            anchors.verticalCenter: parent.verticalCenter
            buttonSize: Style.buttonHeightS
            iconName: "arrow_back"
            tooltipText: I18n.tr("Back", "tooltip on the button leaving the settings page")
            onClicked: root.closeRequested()
        }

        StyledText {
            anchors.left: back.right
            anchors.leftMargin: Style.spacingS
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("Settings", "settings page title")
            color: Style.surfaceText
            font.pixelSize: Style.fontSizeXLarge
        }
    }

    DFlickable {
        id: flick

        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: header.bottom
        anchors.bottom: parent.bottom
        contentWidth: width
        contentHeight: page.implicitHeight + Style.spacingXL
        clip: true

        Column {
            id: page

            x: (flick.width - root.pageWidth) / 2
            width: root.pageWidth
            spacing: Style.spacingL

            Section {
                title: I18n.tr("View", "settings section")

                DDropdown {
                    addHorizontalPadding: true
                    text: I18n.tr("Mode", "setting name")
                    description: I18n.tr("Simple hides the status details, line numbers and a lone tab.", "setting description")
                    options: [I18n.tr("Advanced", "view mode"), I18n.tr("Simple", "view mode")]
                    optionValues: ["advanced", "simple"]
                    currentValue: SettingsData.viewMode
                    onValueChanged: value => SettingsData.viewMode = value
                }

                DDropdown {
                    addHorizontalPadding: true
                    text: I18n.tr("Theme", "setting name")
                    options: [I18n.tr("Follow System", "theme mode"), I18n.tr("Light", "theme mode"), I18n.tr("Dark", "theme mode")]
                    optionValues: ["auto", "light", "dark"]
                    currentValue: SettingsData.themeMode
                    onValueChanged: value => SettingsData.themeMode = value
                }

                DDropdown {
                    addHorizontalPadding: true
                    text: I18n.tr("Colors", "setting name")
                    description: I18n.tr("Dux Shell colors are used when the shell publishes them.", "setting description")
                    options: [I18n.tr("Dux Shell", "color source"), I18n.tr("Preset", "color source")]
                    optionValues: ["auto", "preset"]
                    currentValue: SettingsData.colorSource
                    onValueChanged: value => SettingsData.colorSource = value
                }

                DDropdown {
                    addHorizontalPadding: true
                    text: I18n.tr("Preset", "setting name")
                    description: I18n.tr("Also used when Dux Shell colors are not available.", "setting description")
                    options: StockTheme.presetNames().map(name => root.presetLabel(name))
                    optionValues: StockTheme.presetNames()
                    currentValue: SettingsData.presetTheme
                    onValueChanged: value => SettingsData.presetTheme = value
                }
            }

            Section {
                title: I18n.tr("Editor", "settings section")

                DToggle {
                    text: I18n.tr("Monospace Font", "setting name")
                    checked: SettingsData.useMonospace
                    onToggled: checked => SettingsData.useMonospace = checked
                }

                DDropdown {
                    addHorizontalPadding: true
                    text: I18n.tr("Font Family", "setting name")
                    enableFuzzySearch: true
                    options: [I18n.tr("Default", "font family entry that follows the theme")].concat(Qt.fontFamilies())
                    optionValues: ["default"].concat(Qt.fontFamilies())
                    currentValue: SettingsData.fontFamily || "default"
                    onValueChanged: value => SettingsData.fontFamily = value === "default" ? "" : value
                }

                SettingRow {
                    text: I18n.tr("Font Size", "setting name")

                    DSlider {
                        width: Style.sidebarWidth
                        minimum: SettingsData.minFontSize
                        maximum: SettingsData.maxFontSize
                        value: SettingsData.fontSize
                        onSliderValueChanged: newValue => SettingsData.setFontSize(newValue)
                    }
                }

                DToggle {
                    text: I18n.tr("Line Numbers", "setting name")
                    checked: SettingsData.showLineNumbers
                    onToggled: checked => SettingsData.showLineNumbers = checked
                }

                DDropdown {
                    addHorizontalPadding: true
                    text: I18n.tr("Wrap Lines", "setting name")
                    description: I18n.tr("Automatic wraps prose and Markdown and leaves code unwrapped.", "setting description")
                    options: [I18n.tr("Automatic", "wrap mode"), I18n.tr("Always", "wrap mode"), I18n.tr("Never", "wrap mode")]
                    optionValues: ["auto", "on", "off"]
                    currentValue: SettingsData.wrapMode
                    onValueChanged: value => SettingsData.wrapMode = value
                }

                SettingRow {
                    text: I18n.tr("Tab Width", "setting name")

                    DSlider {
                        width: Style.sidebarWidth
                        minimum: SettingsData.minTabWidth
                        maximum: SettingsData.maxTabWidth
                        value: SettingsData.tabWidth
                        onSliderValueChanged: newValue => SettingsData.tabWidth = newValue
                    }
                }

                DToggle {
                    text: I18n.tr("Insert Spaces", "setting name")
                    description: I18n.tr("The Tab key inserts spaces instead of a tab.", "setting description")
                    checked: SettingsData.insertSpaces
                    onToggled: checked => SettingsData.insertSpaces = checked
                }

                DDropdown {
                    addHorizontalPadding: true
                    text: I18n.tr("Markdown Preview", "setting name")
                    description: I18n.tr("How a Markdown document opens.", "setting description")
                    options: [I18n.tr("Off", "markdown preview mode"), I18n.tr("Side by Side", "markdown preview mode"), I18n.tr("Preview Only", "markdown preview mode")]
                    optionValues: ["off", "split", "full"]
                    currentValue: SettingsData.markdownPreviewMode
                    onValueChanged: value => SettingsData.markdownPreviewMode = value
                }
            }

            Section {
                title: I18n.tr("Files", "settings section")

                DToggle {
                    text: I18n.tr("Autosave", "setting name")
                    description: I18n.tr("Save files two seconds after the last edit.", "setting description")
                    checked: SettingsData.autoSave
                    onToggled: checked => SettingsData.autoSave = checked
                }

                DToggle {
                    text: I18n.tr("Restore Session", "setting name")
                    description: I18n.tr("Reopen the last tabs on start. Unsaved work is always kept.", "setting description")
                    checked: SettingsData.restoreSession
                    onToggled: checked => SettingsData.restoreSession = checked
                }

                DDropdown {
                    addHorizontalPadding: true
                    text: I18n.tr("Closing the Window", "setting name")
                    options: [I18n.tr("Quits Dux Edit", "close behavior"), I18n.tr("Keeps It Running in the Background", "close behavior")]
                    optionValues: ["quit", "background"]
                    currentValue: SettingsData.closeBehavior
                    onValueChanged: value => SettingsData.closeBehavior = value
                }

                SettingRow {
                    text: I18n.tr("DMS Notes", "setting name")
                    description: I18n.tr("Copy the tabs and settings of the DMS notepad.", "setting description")

                    DButton {
                        text: I18n.tr("Import", "button that imports DMS notes")
                        variant: "tonal"
                        onClicked: Documents.importDms()
                    }
                }
            }

            Section {
                title: I18n.tr("Highlighting", "settings section")

                DToggle {
                    text: I18n.tr("Syntax Highlighting", "setting name")
                    checked: SettingsData.highlightEnabled
                    onToggled: checked => SettingsData.highlightEnabled = checked
                }

                Repeater {
                    model: SyntaxPalette.roles

                    SettingRow {
                        id: roleRow

                        required property string modelData
                        required property int index
                        readonly property bool overridden: SettingsData.syntaxColors[modelData] !== undefined

                        text: root.roleLabel(modelData)
                        visible: SettingsData.highlightEnabled

                        Row {
                            spacing: Style.spacingXS

                            DActionButton {
                                anchors.verticalCenter: parent.verticalCenter
                                visible: roleRow.overridden
                                iconName: "restart_alt"
                                tooltipText: I18n.tr("Use the Theme Color", "tooltip on a button that resets a syntax color")
                                onClicked: root.setSyntaxColor(roleRow.modelData, "")
                            }

                            DColorButton {
                                swatchColor: root.syntaxPalette[roleRow.index]?.color ?? "transparent"
                                selected: roleRow.overridden
                                tooltipText: I18n.tr("Change Color", "tooltip on a syntax color swatch")
                                onClicked: root.colorEditRequested(roleRow.modelData, roleRow.text, swatchColor.toString())
                            }
                        }
                    }
                }
            }

            Section {
                title: I18n.tr("Spellcheck", "settings section")

                DToggle {
                    text: I18n.tr("Check Spelling", "setting name")
                    description: Spell.available || Spell.unavailableReason === "" ? "" : (Spell.unavailableReason === "NOHUNSPELL" ? I18n.tr("Install hunspell to check spelling.", "setting description") : I18n.tr("No dictionary is installed for this language.", "setting description"))
                    checked: SettingsData.spellcheckEnabled
                    onToggled: checked => SettingsData.spellcheckEnabled = checked
                }

                DDropdown {
                    addHorizontalPadding: true
                    text: I18n.tr("Language", "setting name")
                    enabled: SettingsData.spellcheckEnabled
                    enableFuzzySearch: true
                    options: [I18n.tr("Follow System", "spellcheck language")].concat(Spell.languages.map(language => language.code))
                    optionValues: ["auto"].concat(Spell.languages.map(language => language.code))
                    currentValue: SettingsData.spellcheckLanguage
                    onValueChanged: value => SettingsData.spellcheckLanguage = value
                }

                DToggle {
                    text: I18n.tr("Check String Literals", "setting name")
                    description: I18n.tr("Also check text inside quotes in code.", "setting description")
                    enabled: SettingsData.spellcheckEnabled
                    checked: SettingsData.spellcheckStrings
                    onToggled: checked => SettingsData.spellcheckStrings = checked
                }
            }

            Section {
                title: I18n.tr("Plugins", "settings section")

                PluginsSettings {
                    width: parent.width
                }
            }

            Section {
                title: I18n.tr("Keyboard Shortcuts", "settings section")

                Repeater {
                    model: root.shortcuts

                    SettingRow {
                        id: shortcutRow

                        required property var modelData

                        text: modelData[1]

                        StyledText {
                            text: shortcutRow.modelData[0]
                            color: Style.surfaceVariantText
                            font.pixelSize: Style.fontSizeSmall
                            isMonospace: true
                        }
                    }
                }
            }
        }
    }
}
