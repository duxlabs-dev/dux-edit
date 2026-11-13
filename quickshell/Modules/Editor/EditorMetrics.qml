pragma Singleton

import QtQuick
import Quickshell
import qs.DCommon.Common

Singleton {
    readonly property real padding: Style.spacingM
    readonly property real gutterPadding: Style.spacingS
    readonly property real dividerWidth: Style.dividerWidth
    readonly property int spellSuggestions: 6

    // Debounces, each armed by typing or an edit and single-shot.
    readonly property int findDelay: 150
    readonly property int previewDelay: 250
    readonly property int autosaveDelay: 2000
    readonly property int pluginWriteDelay: 400

    readonly property int windowMinWidth: 360
    readonly property int windowMinHeight: 320
    readonly property real settingsPageWidth: Style.dialogMaxWidth * 1.4

    // Lexers that read as prose: "auto" wraps them and leaves code unwrapped.
    readonly property var proseLexers: ["plaintext", "text", "markdown", "restructuredtext", "rst", "asciidoc", "org", "tex", "latex"]
}
