pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import qs.DCommon.Common as DC

// settings.json in the config dir. The Go core never reads it; what it needs arrives as request params.
Singleton {
    id: root

    // Read by DCommon through DC.Style.settings; not user settings here.
    readonly property bool popoutElevationEnabled: true
    readonly property bool scrollbarsEnabled: true
    readonly property int textRenderType: DC.Style.TextRenderType.Qt
    readonly property int textRenderQuality: DC.Style.TextRenderQuality.Default
    readonly property bool blurBorderEnabled: false
    readonly property real blurBorderOpacity: 0.35
    readonly property real blurLayerOutlineOpacity: 0
    readonly property string blurBorderColor: "outline"
    readonly property string blurBorderCustomColor: "#ffffff"
    readonly property bool reduceMotion: false
    readonly property int springBounce: 1
    readonly property bool enableRippleEffects: true

    property alias viewMode: adapter.viewMode
    property alias useMonospace: adapter.useMonospace
    property alias fontFamily: adapter.fontFamily
    property alias fontSize: adapter.fontSize
    property alias showLineNumbers: adapter.showLineNumbers
    property alias wrapMode: adapter.wrapMode
    property alias tabWidth: adapter.tabWidth
    property alias insertSpaces: adapter.insertSpaces
    property alias autoSave: adapter.autoSave
    property alias restoreSession: adapter.restoreSession
    property alias closeBehavior: adapter.closeBehavior

    property alias highlightEnabled: adapter.highlightEnabled
    property alias syntaxColors: adapter.syntaxColors

    property alias spellcheckEnabled: adapter.spellcheckEnabled
    property alias spellcheckLanguage: adapter.spellcheckLanguage
    property alias spellcheckStrings: adapter.spellcheckStrings

    property alias markdownPreviewMode: adapter.markdownPreviewMode

    property alias enabledPlugins: adapter.enabledPlugins

    property alias themeMode: adapter.themeMode
    property alias colorSource: adapter.colorSource
    property alias presetTheme: adapter.presetTheme
    property alias language: adapter.language

    readonly property int defaultFontSize: 14
    readonly property int minFontSize: 8
    readonly property int maxFontSize: 48
    readonly property int minTabWidth: 1
    readonly property int maxTabWidth: 8

    function set(key: string, value: var): void {
        if (adapter[key] === undefined)
            return;
        root[key] = value;
    }

    function setFontSize(size: int): void {
        fontSize = Math.max(minFontSize, Math.min(maxFontSize, size));
    }

    function flush(): void {
        store.flush();
    }

    Component.onCompleted: store.read()
    Component.onDestruction: flush()

    JsonStore {
        id: store

        name: "settings"
        path: AppPaths.settingsFile

        adapter: JsonAdapter {
            id: adapter

            // "simple" | "advanced"
            property string viewMode: "advanced"
            property bool useMonospace: true
            // Empty follows the theme's font for the chosen kind.
            property string fontFamily: ""
            property int fontSize: 14
            property bool showLineNumbers: true
            // "auto" wraps prose and Markdown and leaves code unwrapped; "on" | "off"
            property string wrapMode: "auto"
            property int tabWidth: 4
            property bool insertSpaces: false
            property bool autoSave: false
            property bool restoreSession: true
            // "quit" | "background"
            property string closeBehavior: "quit"

            property bool highlightEnabled: true
            // Role name to "#rrggbb", applied over the theme-derived palette.
            property var syntaxColors: ({})

            property bool spellcheckEnabled: true
            // "auto" follows the locale.
            property string spellcheckLanguage: "auto"
            property bool spellcheckStrings: false

            // "off" | "split" | "full": what a Markdown tab opens with.
            property string markdownPreviewMode: "off"

            // Plugin id to true.
            property var enabledPlugins: ({})

            // "auto" follows the portal; "light" | "dark"
            property string themeMode: "auto"
            // "auto" follows DMS colors when present; "preset"
            property string colorSource: "auto"
            property string presetTheme: "purple"
            // Empty follows the system locale.
            property string language: ""
        }
    }
}
