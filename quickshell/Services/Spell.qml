pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.Services
import "../Common/LineIndex.js" as LineIndex

// Misspelled ranges for the lines around each document's viewport.
Singleton {
    id: root

    property bool available: false
    // "" | "NOHUNSPELL" | "NODICT"
    property string unavailableReason: ""
    // Resolved dictionary code, for example en_US.
    property string language: ""
    // [{code, path}], loaded on demand by loadLanguages().
    property var languages: []

    readonly property bool enabled: SettingsData.spellcheckEnabled
    readonly property string requestedLanguage: SettingsData.spellcheckLanguage
    readonly property bool strings: SettingsData.spellcheckStrings
    readonly property bool connected: Backend.connected
    readonly property bool active: enabled && available

    // docId -> LineMap of flat [col, len, ...] arrays.
    readonly property var stores: LineIndex.createStores(2)

    signal rangesChanged(string docId)

    // A LineMap: get(line) returns [col, len, ...] or undefined.
    function ranges(docId) {
        return stores.get(docId);
    }

    function configure() {
        if (!connected)
            return;
        Backend.call("spell.configure", {
            "enabled": enabled,
            "language": requestedLanguage,
            "strings": strings
        }, result => {
            root.available = result.ok === true && result.available === true;
            root.language = result.language || "";
            root.unavailableReason = result.reason || "";
        });
    }

    function loadLanguages() {
        if (languages.length > 0)
            return;
        Backend.call("spell.languages", null, result => root.languages = result.languages || []);
    }

    // cursorLine and cursorColumn (0-based, UTF-16) mark the word being typed; -1 for none.
    function view(doc, first, last, cursorLine, cursorColumn) {
        if (!active)
            return;
        stores.open(doc.docId).keep(first, last);
        Backend.notify("spell.view", {
            "docId": doc.docId,
            "first": first,
            "last": last,
            "cursorLine": cursorLine,
            "cursorColumn": cursorColumn
        });
    }

    function suggest(word, callback) {
        Backend.call("spell.suggest", {
            "word": word
        }, result => callback(result.suggestions || []));
    }

    function add(word) {
        Backend.call("spell.add", {
            "word": word
        }, null);
    }

    function ignore(word) {
        Backend.call("spell.ignore", {
            "word": word
        }, null);
    }

    function linesEdited(docId, version, change) {
        stores.edited(docId, version, change);
    }

    function forget(docId) {
        stores.forget(docId);
    }

    onConnectedChanged: configure()
    onEnabledChanged: configure()
    onRequestedLanguageChanged: configure()
    onStringsChanged: configure()
    onActiveChanged: {
        if (active)
            return;
        stores.clear();
    }

    Component.onCompleted: {
        Backend.subscribe("spell", event => {
            if (event.type !== "ranges" || !event.lines || !Documents.byDocId(event.docId))
                return;
            root.stores.open(event.docId).merge(event.version, event.first, event.lines);
            root.rangesChanged(event.docId);
        });
    }
}
