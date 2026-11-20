pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import qs.DCommon.Common as DC
import "SettingsDefaults.js" as Defaults

// settings.json in the config dir. The Go core never reads it; what it needs arrives as request params.
Singleton {
    id: root

    // Read by Theme and DCommon through DC.Style.settings; Style falls back for the rest.
    readonly property real blurLayerOutlineOpacity: 0

    property alias viewMode: adapter.viewMode
    property alias useMonospace: adapter.useMonospace
    property alias fontFamily: adapter.fontFamily
    property alias fontSize: adapter.fontSize
    property alias showLineNumbers: adapter.showLineNumbers
    property alias highlightCurrentLine: adapter.highlightCurrentLine
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

    property alias density: adapter.density
    property alias radiusStrength: adapter.radiusStrength
    property alias fontSource: adapter.fontSource
    property alias fontWeight: adapter.fontWeight
    property alias fontScale: adapter.fontScale
    property alias floatingWindowTransparency: adapter.floatingWindowTransparency
    property alias floatingWindowForegroundLayers: adapter.floatingWindowForegroundLayers
    property alias floatingWindowForegroundTransparency: adapter.floatingWindowForegroundTransparency
    property alias blurEnabled: adapter.blurEnabled
    property alias motionDuration: adapter.motionDuration
    property alias reduceMotion: adapter.reduceMotion
    property alias springBounce: adapter.springBounce
    property alias enableRippleEffects: adapter.enableRippleEffects
    property alias focusRingEnabled: adapter.focusRingEnabled
    property alias focusRingWidth: adapter.focusRingWidth
    property alias focusRingColor: adapter.focusRingColor
    property alias scrollbarsEnabled: adapter.scrollbarsEnabled
    property alias scrollShadowsEnabled: adapter.scrollShadowsEnabled
    property alias textRenderType: adapter.textRenderType
    property alias textRenderQuality: adapter.textRenderQuality

    readonly property int defaultFontSize: 14
    readonly property int minFontSize: 8
    readonly property int maxFontSize: 48
    readonly property int minTabWidth: 1
    readonly property int maxTabWidth: 8

    function set(key: string, value: var): void {
        if (adapter[key] === undefined)
            return;
        switch (key) {
        case "density":
            setDensity(value);
            return;
        default:
            root[key] = value;
        }
    }

    function hasDefault(key: string): bool {
        return Defaults.has(key);
    }

    function defaultFor(key: string): var {
        return Defaults.value(key, density);
    }

    function isDefault(key: string): bool {
        return root[key] === defaultFor(key);
    }

    function resetToDefault(key: string): void {
        if (!hasDefault(key))
            return;
        root[key] = defaultFor(key);
    }

    function setDensity(next: string): void {
        const moved = Defaults.transition(root, density, next);
        density = next;
        for (const key in moved)
            root[key] = moved[key];
    }

    function setFontSize(size: int): void {
        fontSize = Math.max(minFontSize, Math.min(maxFontSize, size));
    }

    function setSyntaxColor(role: string, color: string): void {
        const next = Object.assign({}, syntaxColors);
        if (color === "")
            delete next[role];
        else
            next[role] = color;
        syntaxColors = next;
    }

    function adoptLegacyAnimationDuration(saved: var): void {
        if (typeof saved?.animationDuration !== "number" || "motionDuration" in saved)
            return;
        motionDuration = Defaults.motionStep(saved.animationDuration);
    }

    function flush(): void {
        store.flush();
    }

    Component.onCompleted: store.read()
    Component.onDestruction: flush()

    DC.JsonStore {
        id: store

        name: "settings"
        path: AppPaths.settingsFile
        parse: text => root.adoptLegacyAnimationDuration(store.objectFrom(text))

        adapter: JsonAdapter {
            id: adapter

            // "simple" | "advanced"
            property string viewMode: "advanced"
            property bool useMonospace: true
            // Empty follows the theme's font for the chosen kind.
            property string fontFamily: ""
            property int fontSize: 14
            property bool showLineNumbers: true
            property bool highlightCurrentLine: true
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

            // "spacious" | "comfortable" | "compact" | "dense"
            property string density: "comfortable"
            property int radiusStrength: 50
            // "auto" follows DMS typography when present; "system" | "custom" (the bundled fonts)
            property string fontSource: "auto"
            property int fontWeight: 400
            property real fontScale: 1.0
            property real floatingWindowTransparency: 1.0
            property bool floatingWindowForegroundLayers: true
            property real floatingWindowForegroundTransparency: 1.0
            property bool blurEnabled: false
            // -1 off, 0..4 shorter to longer, as in DMS
            property int motionDuration: 2
            property bool reduceMotion: false
            // 0 smooth, 1 balanced, 2 playful
            property int springBounce: 1
            property bool enableRippleEffects: true
            property bool focusRingEnabled: true
            property real focusRingWidth: 1.5
            // "primary" | "secondary" | "outline" | "surfaceText"
            property string focusRingColor: "primary"
            property bool scrollbarsEnabled: true
            property bool scrollShadowsEnabled: true
            property int textRenderType: DC.Style.TextRenderType.Qt
            property int textRenderQuality: DC.Style.TextRenderQuality.Default
        }
    }
}
