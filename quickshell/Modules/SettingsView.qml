pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Modules.Editor
import qs.Modules.Plugins
import qs.Modules.Settings
import qs.Services
import "../DCommon/Common/StockTheme.js" as StockTheme

// In-window settings: a sidebar, or a drill-down list when narrow. Loaded only while open.
Rectangle {
    id: root

    signal closeRequested
    signal colorEditRequested(string role, string label, string current)

    // "" is the category list when narrow; wide shows the first category instead.
    property string page: ""
    property bool forward: true
    readonly property bool wide: width >= EditorMetrics.settingsSplitMinWidth
    readonly property string shownPage: page !== "" || !wide ? page : pages[0].id
    readonly property bool motion: !Style.reduceMotion

    readonly property var pages: [
        {
            "id": "appearance",
            "icon": "palette",
            "title": I18n.tr("Appearance", "settings category"),
            "hint": I18n.tr("View mode, colors, fonts and density", "settings category description")
        },
        {
            "id": "window",
            "icon": "animation",
            "title": I18n.tr("Window and Motion", "settings category"),
            "hint": I18n.tr("Opacity, blur, animations and focus ring", "settings category description")
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

    function pageIndex(id) {
        return pages.findIndex(entry => entry.id === id);
    }

    function open(id) {
        if (id === shownPage)
            return;
        forward = pageIndex(id) > pageIndex(shownPage);
        page = id;
    }

    function back() {
        if (wide || page === "") {
            closeRequested();
            return;
        }
        open("");
    }

    function focusPage() {
        if (wide) {
            sidebar.focusCurrent();
            return;
        }
        forceActiveFocus();
    }

    function pageComponent(id) {
        switch (id) {
        case "appearance":
            return appearancePage;
        case "window":
            return windowPage;
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

    // Solid like the editor while the window is see-through.
    color: Style.foregroundColor(Style.hostSurface, true)
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
            tooltipText: root.wide || root.page === "" ? I18n.tr("Close Settings", "tooltip on the button leaving settings") : I18n.tr("Back", "tooltip on the button leaving a settings page")
            onClicked: root.back()
        }

        StyledText {
            anchors.left: backButton.right
            anchors.leftMargin: Style.spacingS
            anchors.right: parent.right
            anchors.rightMargin: Style.spacingL
            anchors.verticalCenter: parent.verticalCenter
            text: root.pages.find(entry => entry.id === root.shownPage)?.title ?? I18n.tr("Settings", "settings page title")
            color: Style.surfaceText
            font.pixelSize: Style.fontSizeXLarge
            elide: Text.ElideRight
            horizontalAlignment: Text.AlignLeft
        }
    }

    SettingsSidebar {
        id: sidebar

        anchors.left: parent.left
        anchors.top: header.bottom
        anchors.bottom: parent.bottom
        anchors.leftMargin: Style.spacingL
        anchors.topMargin: Style.spacingXS
        width: EditorMetrics.settingsSidebarWidth
        visible: root.wide
        pages: root.pages
        current: root.shownPage
        onPageSelected: id => root.open(id)
    }

    Item {
        id: host

        anchors.left: root.wide ? sidebar.right : parent.left
        anchors.right: parent.right
        anchors.top: header.bottom
        anchors.bottom: parent.bottom
        anchors.leftMargin: root.wide ? Style.spacingS : 0
        clip: true

        Loader {
            id: pageLoader

            width: host.width
            height: host.height
            sourceComponent: root.pageComponent(root.shownPage)
            onLoaded: {
                root.focusPage();
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
                    description: I18n.tr("Simple hides the status details, line numbers and a lone tab.", "setting description")
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

            SettingsCard {
                title: I18n.tr("Typography", "settings section")

                SettingsDropdownRow {
                    text: I18n.tr("Interface Font", "setting name")
                    description: I18n.tr("Automatic follows the Dux Shell fonts when it publishes them.", "setting description")
                    options: [I18n.tr("Automatic", "interface font source"), I18n.tr("System", "interface font source"), I18n.tr("Built-in", "interface font source")]
                    optionValues: ["auto", "system", "custom"]
                    currentValue: SettingsData.fontSource
                    onValueChanged: value => SettingsData.fontSource = value
                }

                SettingsSliderRow {
                    visible: SettingsData.fontSource !== "auto"
                    text: I18n.tr("Font Weight", "setting name")
                    minimum: Font.Thin
                    maximum: Font.Black
                    step: 100
                    value: SettingsData.fontWeight
                    resettable: !SettingsData.isDefault("fontWeight")
                    onResetRequested: SettingsData.resetToDefault("fontWeight")
                    onSliderValueChanged: newValue => SettingsData.fontWeight = newValue
                }

                SettingsSliderRow {
                    visible: SettingsData.fontSource !== "auto"
                    text: I18n.tr("Font Scale", "setting name")
                    minimum: 75
                    maximum: 150
                    unit: "%"
                    value: Math.round(SettingsData.fontScale * 100)
                    resettable: !SettingsData.isDefault("fontScale")
                    onResetRequested: SettingsData.resetToDefault("fontScale")
                    onSliderValueChanged: newValue => SettingsData.fontScale = newValue / 100
                }
            }

            SettingsCard {
                title: I18n.tr("Shape", "settings section")

                SettingsSliderRow {
                    text: I18n.tr("Corner Rounding", "setting name")
                    description: I18n.tr("50 is the %1 default.", "corner rounding description, %1 is Material").arg("Material")
                    minimum: 0
                    maximum: 100
                    value: SettingsData.radiusStrength
                    resettable: !SettingsData.isDefault("radiusStrength")
                    onResetRequested: SettingsData.resetToDefault("radiusStrength")
                    onSliderValueChanged: newValue => SettingsData.radiusStrength = newValue
                }
            }

            SettingsCard {
                title: I18n.tr("Density", "settings section")

                SettingsDropdownRow {
                    text: I18n.tr("Interface Density", "setting name")
                    description: I18n.tr("Room around controls and rows.", "setting description")
                    options: [I18n.tr("Spacious", "interface density"), I18n.tr("Comfortable", "interface density"), I18n.tr("Compact", "interface density"), I18n.tr("Dense", "interface density")]
                    optionValues: ["spacious", "comfortable", "compact", "dense"]
                    currentValue: SettingsData.density
                    onValueChanged: value => SettingsData.setDensity(value)
                }
            }

            SettingsCard {
                title: I18n.tr("Text Rendering", "settings section")

                SettingsDropdownRow {
                    text: I18n.tr("Method", "text rendering setting name")
                    description: I18n.tr("How interface text is drawn. Document text is not affected.", "setting description")
                    options: [I18n.tr("Distance Field", "text rendering method"), I18n.tr("Native", "text rendering method"), I18n.tr("Curves", "text rendering method")]
                    optionValues: [String(Style.TextRenderType.Qt), String(Style.TextRenderType.Native), String(Style.TextRenderType.Curve)]
                    currentValue: String(SettingsData.textRenderType)
                    onValueChanged: value => SettingsData.textRenderType = Number(value)
                }

                SettingsDropdownRow {
                    text: I18n.tr("Quality", "text rendering setting name")
                    enabled: SettingsData.textRenderType === Style.TextRenderType.Qt
                    options: [I18n.tr("Default", "text rendering quality"), I18n.tr("Low", "text rendering quality"), I18n.tr("Normal", "text rendering quality"), I18n.tr("High", "text rendering quality"), I18n.tr("Very High", "text rendering quality")]
                    optionValues: [String(Style.TextRenderQuality.Default), String(Style.TextRenderQuality.Low), String(Style.TextRenderQuality.Normal), String(Style.TextRenderQuality.High), String(Style.TextRenderQuality.VeryHigh)]
                    currentValue: String(SettingsData.textRenderQuality)
                    onValueChanged: value => SettingsData.textRenderQuality = Number(value)
                }
            }
        }
    }

    Component {
        id: windowPage

        SettingsPage {
            Component.onCompleted: BlurService.probe()

            SettingsCard {
                title: I18n.tr("Window", "settings section")

                SettingsSliderRow {
                    text: I18n.tr("Opacity", "window opacity setting name")
                    description: I18n.tr("Lets the desktop show through the window.", "setting description")
                    minimum: 0
                    maximum: 100
                    unit: "%"
                    value: Math.round(SettingsData.floatingWindowTransparency * 100)
                    resettable: !SettingsData.isDefault("floatingWindowTransparency")
                    onResetRequested: SettingsData.resetToDefault("floatingWindowTransparency")
                    onSliderValueChanged: newValue => SettingsData.floatingWindowTransparency = newValue / 100
                }

                SettingsToggleRow {
                    text: I18n.tr("Foreground Layers", "setting name")
                    description: I18n.tr("Keeps the document and panels solid when the window is see-through.", "setting description")
                    checked: SettingsData.floatingWindowForegroundLayers
                    onToggled: checked => SettingsData.floatingWindowForegroundLayers = checked
                }

                SettingsSliderRow {
                    text: I18n.tr("Layer Opacity", "foreground layer opacity setting name")
                    enabled: SettingsData.floatingWindowForegroundLayers
                    minimum: 0
                    maximum: 100
                    unit: "%"
                    value: Math.round(SettingsData.floatingWindowForegroundTransparency * 100)
                    resettable: !SettingsData.isDefault("floatingWindowForegroundTransparency")
                    onResetRequested: SettingsData.resetToDefault("floatingWindowForegroundTransparency")
                    onSliderValueChanged: newValue => SettingsData.floatingWindowForegroundTransparency = newValue / 100
                }

                SettingsToggleRow {
                    visible: BlurService.available
                    text: I18n.tr("Background Blur", "setting name")
                    description: I18n.tr("Blurs what shows through the window. Needs an opacity below 100%.", "setting description")
                    checked: SettingsData.blurEnabled
                    onToggled: checked => SettingsData.blurEnabled = checked
                }

                SettingsRow {
                    visible: BlurService.probed && !BlurService.available
                    title: I18n.tr("Background Blur", "setting name")
                    subtitle: I18n.tr("Your compositor does not support background blur.", "hint shown instead of the background blur toggle")
                }
            }

            SettingsCard {
                title: I18n.tr("Motion", "settings section")

                SettingsDropdownRow {
                    text: I18n.tr("Duration", "animation duration setting name")
                    options: [I18n.tr("Off", "animation duration option"), I18n.tr("Shorter", "animation duration option"), I18n.tr("Short", "animation duration option"), I18n.tr("Medium", "animation duration option"), I18n.tr("Long", "animation duration option"), I18n.tr("Longer", "animation duration option")]
                    optionValues: ["-1", "0", "1", "2", "3", "4"]
                    currentValue: String(SettingsData.motionDuration)
                    onValueChanged: value => SettingsData.motionDuration = Number(value)
                }

                SettingsDropdownRow {
                    visible: !Style.motionOff
                    text: I18n.tr("Spring", "spring bounce setting name")
                    options: [I18n.tr("Smooth", "spring bounce"), I18n.tr("Balanced", "spring bounce"), I18n.tr("Playful", "spring bounce")]
                    optionValues: ["0", "1", "2"]
                    currentValue: String(SettingsData.springBounce)
                    onValueChanged: value => SettingsData.springBounce = Number(value)
                }

                SettingsToggleRow {
                    visible: !Style.motionOff
                    text: I18n.tr("Ripple Effects", "setting name")
                    checked: SettingsData.enableRippleEffects
                    onToggled: checked => SettingsData.enableRippleEffects = checked
                }

                SettingsToggleRow {
                    visible: !Style.motionOff
                    text: I18n.tr("Reduce Motion", "setting name")
                    checked: SettingsData.reduceMotion
                    onToggled: checked => SettingsData.reduceMotion = checked
                }
            }

            SettingsCard {
                title: I18n.tr("Focus Ring", "settings section")

                SettingsToggleRow {
                    text: I18n.tr("Focus Ring", "setting name")
                    description: I18n.tr("Outlines the control that has keyboard focus.", "setting description")
                    checked: SettingsData.focusRingEnabled
                    onToggled: checked => SettingsData.focusRingEnabled = checked
                }

                SettingsSliderRow {
                    text: I18n.tr("Thickness", "focus ring thickness setting name")
                    enabled: SettingsData.focusRingEnabled
                    minimum: 10
                    maximum: 40
                    step: 5
                    decimals: 1
                    unit: "px"
                    value: Math.round(SettingsData.focusRingWidth * 10)
                    resettable: !SettingsData.isDefault("focusRingWidth")
                    onResetRequested: SettingsData.resetToDefault("focusRingWidth")
                    onSliderValueChanged: newValue => SettingsData.focusRingWidth = newValue / 10
                }

                SettingsDropdownRow {
                    text: I18n.tr("Color", "focus ring color setting name")
                    enabled: SettingsData.focusRingEnabled
                    options: [I18n.tr("Primary", "focus ring color"), I18n.tr("Secondary", "focus ring color"), I18n.tr("Outline", "focus ring color"), I18n.tr("Text", "focus ring color")]
                    optionValues: ["primary", "secondary", "outline", "surfaceText"]
                    currentValue: SettingsData.focusRingColor
                    onValueChanged: value => SettingsData.focusRingColor = value
                }
            }

            SettingsCard {
                title: I18n.tr("Scrolling", "settings section")

                SettingsToggleRow {
                    text: I18n.tr("Scrollbars", "setting name")
                    checked: SettingsData.scrollbarsEnabled
                    onToggled: checked => SettingsData.scrollbarsEnabled = checked
                }

                SettingsToggleRow {
                    text: I18n.tr("Scroll Shadows", "setting name")
                    description: I18n.tr("Fades the edges of lists that scroll.", "setting description")
                    checked: SettingsData.scrollShadowsEnabled
                    onToggled: checked => SettingsData.scrollShadowsEnabled = checked
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
