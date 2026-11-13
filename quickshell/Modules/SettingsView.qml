pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Modules.Plugins
import qs.Modules.Settings
import qs.Services
import "../DCommon/Common/StockTheme.js" as StockTheme

// In-window settings: a list of categories, each opening as a subpage. Loaded only while open.
Rectangle {
    id: root

    signal closeRequested
    signal colorEditRequested(string role, string label, string current)

    // "" is the category list.
    property string page: ""
    property bool forward: true
    readonly property bool motion: !Style.reduceMotion && Style.currentAnimationSpeed !== Style.AnimationSpeed.None

    readonly property var pages: [
        {
            "id": "appearance",
            "icon": "palette",
            "title": I18n.tr("Appearance", "settings category"),
            "hint": I18n.tr("View mode, theme and colors", "settings category description")
        },
        {
            "id": "editor",
            "icon": "edit_note",
            "title": I18n.tr("Editor", "settings category"),
            "hint": I18n.tr("Font, line numbers, wrapping and indentation", "settings category description")
        },
        {
            "id": "files",
            "icon": "folder",
            "title": I18n.tr("Files", "settings category"),
            "hint": I18n.tr("Autosave, session and closing the window", "settings category description")
        },
        {
            "id": "highlight",
            "icon": "format_color_text",
            "title": I18n.tr("Highlighting", "settings category"),
            "hint": I18n.tr("Syntax highlighting and its colors", "settings category description")
        },
        {
            "id": "spell",
            "icon": "spellcheck",
            "title": I18n.tr("Spellcheck", "settings category"),
            "hint": I18n.tr("Language and what gets checked", "settings category description")
        },
        {
            "id": "plugins",
            "icon": "extension",
            "title": I18n.tr("Plugins", "settings category"),
            "hint": I18n.tr("Installed plugins and their settings", "settings category description")
        },
        {
            "id": "shortcuts",
            "icon": "keyboard",
            "title": I18n.tr("Keyboard Shortcuts", "settings category"),
            "hint": I18n.tr("Every shortcut, searchable", "settings category description")
        }
    ]

    function open(id) {
        forward = true;
        page = id;
    }

    function back() {
        if (page === "") {
            closeRequested();
            return;
        }
        forward = false;
        page = "";
    }

    function pageComponent(id) {
        switch (id) {
        case "appearance":
            return appearancePage;
        case "editor":
            return editorPage;
        case "files":
            return filesPage;
        case "highlight":
            return highlightPage;
        case "spell":
            return spellPage;
        case "plugins":
            return pluginsPage;
        case "shortcuts":
            return shortcutsPage;
        }
        return categoriesPage;
    }

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

    color: Style.hostSurface
    Keys.onEscapePressed: back()

    Item {
        id: header

        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: Style.buttonHeightS + Style.spacingS * 2

        DActionButton {
            id: backButton

            anchors.left: parent.left
            anchors.leftMargin: Style.spacingS
            anchors.verticalCenter: parent.verticalCenter
            buttonSize: Style.buttonHeightS
            iconName: "arrow_back"
            tooltipText: I18n.tr("Back", "tooltip on the button leaving a settings page")
            onClicked: root.back()
        }

        StyledText {
            anchors.left: backButton.right
            anchors.leftMargin: Style.spacingS
            anchors.right: parent.right
            anchors.rightMargin: Style.spacingL
            anchors.verticalCenter: parent.verticalCenter
            text: root.pages.find(entry => entry.id === root.page)?.title ?? I18n.tr("Settings", "settings page title")
            color: Style.surfaceText
            font.pixelSize: Style.fontSizeXLarge
            elide: Text.ElideRight
            horizontalAlignment: Text.AlignLeft
        }
    }

    Item {
        id: host

        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: header.bottom
        anchors.bottom: parent.bottom
        clip: true

        Loader {
            id: pageLoader

            width: host.width
            height: host.height
            sourceComponent: root.pageComponent(root.page)
            onLoaded: {
                root.forceActiveFocus();
                if (root.motion)
                    enter.restart();
            }
        }

        ParallelAnimation {
            id: enter

            DAnim {
                target: pageLoader
                property: "x"
                from: (root.forward === I18n.isRtl ? -1 : 1) * Style.spacingXL * 2
                to: 0
                duration: Style.expressiveDurations.expressiveFastSpatial
                easing.bezierCurve: Style.expressiveCurves.expressiveFastSpatial
            }

            DAnim {
                target: pageLoader
                property: "opacity"
                from: 0
                to: 1
                duration: Style.expressiveDurations.expressiveEffects
                easing.bezierCurve: Style.expressiveCurves.expressiveEffects
            }
        }
    }

    Component {
        id: categoriesPage

        SettingsPage {
            SettingsCard {
                Repeater {
                    model: root.pages

                    SettingsRow {
                        required property var modelData

                        iconName: modelData.icon
                        title: modelData.title
                        subtitle: modelData.hint
                        clickable: true
                        showChevron: true
                        onClicked: root.open(modelData.id)
                    }
                }
            }
        }
    }

    Component {
        id: appearancePage

        SettingsPage {
            SettingsCard {
                title: I18n.tr("View", "settings section")

                SettingsDropdownRow {
                    text: I18n.tr("Mode", "setting name")
                    description: I18n.tr("Simple hides the status details, line numbers and a lone tab, and centers wrapped text.", "setting description")
                    options: [I18n.tr("Advanced", "view mode"), I18n.tr("Simple", "view mode")]
                    optionValues: ["advanced", "simple"]
                    currentValue: SettingsData.viewMode
                    onValueChanged: value => SettingsData.viewMode = value
                }
            }

            SettingsCard {
                title: I18n.tr("Colors", "settings section")

                SettingsDropdownRow {
                    text: I18n.tr("Theme", "setting name")
                    options: [I18n.tr("Follow System", "theme mode"), I18n.tr("Light", "theme mode"), I18n.tr("Dark", "theme mode")]
                    optionValues: ["auto", "light", "dark"]
                    currentValue: SettingsData.themeMode
                    onValueChanged: value => SettingsData.themeMode = value
                }

                SettingsDropdownRow {
                    text: I18n.tr("Colors", "setting name")
                    description: I18n.tr("Dux Shell colors are used when the shell publishes them.", "setting description")
                    options: [I18n.tr("Dux Shell", "color source"), I18n.tr("Preset", "color source")]
                    optionValues: ["auto", "preset"]
                    currentValue: SettingsData.colorSource
                    onValueChanged: value => SettingsData.colorSource = value
                }

                SettingsDropdownRow {
                    text: I18n.tr("Preset", "setting name")
                    description: I18n.tr("Also used when Dux Shell colors are not available.", "setting description")
                    options: StockTheme.presetNames().map(name => root.presetLabel(name))
                    optionValues: StockTheme.presetNames()
                    currentValue: SettingsData.presetTheme
                    onValueChanged: value => SettingsData.presetTheme = value
                }
            }
        }
    }

    Component {
        id: editorPage

        SettingsPage {
            id: editorSettings

            readonly property var families: Qt.fontFamilies()

            SettingsCard {
                title: I18n.tr("Font", "settings section")

                SettingsToggleRow {
                    text: I18n.tr("Monospace Font", "setting name")
                    checked: SettingsData.useMonospace
                    onToggled: checked => SettingsData.useMonospace = checked
                }

                SettingsDropdownRow {
                    text: I18n.tr("Font Family", "setting name")
                    enableFuzzySearch: true
                    options: [I18n.tr("Default", "font family entry that follows the theme")].concat(editorSettings.families)
                    optionValues: ["default"].concat(editorSettings.families)
                    currentValue: SettingsData.fontFamily || "default"
                    onValueChanged: value => SettingsData.fontFamily = value === "default" ? "" : value
                }

                SettingsSliderRow {
                    text: I18n.tr("Font Size", "setting name")
                    minimum: SettingsData.minFontSize
                    maximum: SettingsData.maxFontSize
                    value: SettingsData.fontSize
                    onSliderValueChanged: newValue => SettingsData.setFontSize(newValue)
                }
            }

            SettingsCard {
                title: I18n.tr("Text", "settings section")

                SettingsToggleRow {
                    text: I18n.tr("Line Numbers", "setting name")
                    checked: SettingsData.showLineNumbers
                    onToggled: checked => SettingsData.showLineNumbers = checked
                }

                SettingsToggleRow {
                    text: I18n.tr("Highlight Current Line", "setting name")
                    checked: SettingsData.highlightCurrentLine
                    onToggled: checked => SettingsData.highlightCurrentLine = checked
                }

                SettingsDropdownRow {
                    text: I18n.tr("Wrap Lines", "setting name")
                    description: I18n.tr("Automatic wraps prose and Markdown and leaves code unwrapped.", "setting description")
                    options: [I18n.tr("Automatic", "wrap mode"), I18n.tr("Always", "wrap mode"), I18n.tr("Never", "wrap mode")]
                    optionValues: ["auto", "on", "off"]
                    currentValue: SettingsData.wrapMode
                    onValueChanged: value => SettingsData.wrapMode = value
                }

                SettingsSliderRow {
                    text: I18n.tr("Tab Width", "setting name")
                    minimum: SettingsData.minTabWidth
                    maximum: SettingsData.maxTabWidth
                    value: SettingsData.tabWidth
                    onSliderValueChanged: newValue => SettingsData.tabWidth = newValue
                }

                SettingsToggleRow {
                    text: I18n.tr("Insert Spaces", "setting name")
                    description: I18n.tr("The Tab key inserts spaces instead of a tab.", "setting description")
                    checked: SettingsData.insertSpaces
                    onToggled: checked => SettingsData.insertSpaces = checked
                }
            }

            SettingsCard {
                title: I18n.tr("Markdown", "settings section")

                SettingsDropdownRow {
                    text: I18n.tr("Markdown Preview", "setting name")
                    description: I18n.tr("How a Markdown document opens.", "setting description")
                    options: [I18n.tr("Off", "markdown preview mode"), I18n.tr("Side by Side", "markdown preview mode"), I18n.tr("Preview Only", "markdown preview mode")]
                    optionValues: ["off", "split", "full"]
                    currentValue: SettingsData.markdownPreviewMode
                    onValueChanged: value => SettingsData.markdownPreviewMode = value
                }
            }
        }
    }

    Component {
        id: filesPage

        SettingsPage {
            SettingsCard {
                SettingsToggleRow {
                    text: I18n.tr("Autosave", "setting name")
                    description: I18n.tr("Save files two seconds after the last edit.", "setting description")
                    checked: SettingsData.autoSave
                    onToggled: checked => SettingsData.autoSave = checked
                }

                SettingsToggleRow {
                    text: I18n.tr("Restore Session", "setting name")
                    description: I18n.tr("Reopen the last tabs on start. Unsaved work is always kept.", "setting description")
                    checked: SettingsData.restoreSession
                    onToggled: checked => SettingsData.restoreSession = checked
                }

                SettingsDropdownRow {
                    text: I18n.tr("Closing the Window", "setting name")
                    options: [I18n.tr("Quits Dux Edit", "close behavior"), I18n.tr("Keeps It Running in the Background", "close behavior")]
                    optionValues: ["quit", "background"]
                    currentValue: SettingsData.closeBehavior
                    onValueChanged: value => SettingsData.closeBehavior = value
                }
            }

            SettingsCard {
                title: I18n.tr("Import", "settings section")

                SettingsRow {
                    title: I18n.tr("DMS Notes", "setting name")
                    subtitle: I18n.tr("Copy the tabs and settings of the DMS notepad.", "setting description")

                    DButton {
                        text: I18n.tr("Import", "button that imports DMS notes")
                        variant: "tonal"
                        onClicked: Documents.importDms()
                    }
                }
            }
        }
    }

    Component {
        id: highlightPage

        HighlightPage {
            onColorEditRequested: (role, label, current) => root.colorEditRequested(role, label, current)
        }
    }

    Component {
        id: spellPage

        SettingsPage {
            Component.onCompleted: Spell.loadLanguages()

            SettingsCard {
                SettingsToggleRow {
                    text: I18n.tr("Check Spelling", "setting name")
                    description: Spell.available || Spell.unavailableReason === "" ? "" : (Spell.unavailableReason === "NOHUNSPELL" ? I18n.tr("Install hunspell to check spelling.", "setting description") : I18n.tr("No dictionary is installed for this language.", "setting description"))
                    checked: SettingsData.spellcheckEnabled
                    onToggled: checked => SettingsData.spellcheckEnabled = checked
                }

                SettingsDropdownRow {
                    text: I18n.tr("Language", "setting name")
                    enabled: SettingsData.spellcheckEnabled
                    enableFuzzySearch: true
                    options: [I18n.tr("Follow System", "spellcheck language")].concat(Spell.languages.map(language => language.code))
                    optionValues: ["auto"].concat(Spell.languages.map(language => language.code))
                    currentValue: SettingsData.spellcheckLanguage
                    onValueChanged: value => SettingsData.spellcheckLanguage = value
                }

                SettingsToggleRow {
                    text: I18n.tr("Check String Literals", "setting name")
                    description: I18n.tr("Also check text inside quotes in code.", "setting description")
                    enabled: SettingsData.spellcheckEnabled
                    checked: SettingsData.spellcheckStrings
                    onToggled: checked => SettingsData.spellcheckStrings = checked
                }
            }
        }
    }

    Component {
        id: pluginsPage

        SettingsPage {
            PluginsSettings {
                width: parent.width
            }
        }
    }

    Component {
        id: shortcutsPage

        ShortcutsPage {}
    }
}
