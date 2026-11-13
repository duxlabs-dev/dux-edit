pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell

// One list feeds the window's Shortcut items and the settings page, so the two cannot drift.
Singleton {
    id: root

    readonly property var groups: [
        {
            "id": "files",
            "label": I18n.tr("Files", "shortcut group")
        },
        {
            "id": "tabs",
            "label": I18n.tr("Tabs", "shortcut group")
        },
        {
            "id": "edit",
            "label": I18n.tr("Edit", "shortcut group")
        },
        {
            "id": "lines",
            "label": I18n.tr("Lines", "shortcut group")
        },
        {
            "id": "find",
            "label": I18n.tr("Find and Navigate", "shortcut group")
        },
        {
            "id": "view",
            "label": I18n.tr("View", "shortcut group")
        }
    ]

    // `keys` are bound and shown, `aliases` are bound only. `unbound` entries are shown only: mostly keys the
    // editor handles itself, which a window Shortcut would take from it. `hidden` entries are bound only.
    readonly property var entries: [
        {
            "id": "new",
            "group": "files",
            "keys": ["Ctrl+N", "Ctrl+T"],
            "label": I18n.tr("New document", "shortcut description")
        },
        {
            "id": "open",
            "group": "files",
            "keys": ["Ctrl+O"],
            "label": I18n.tr("Open files", "shortcut description")
        },
        {
            "id": "save",
            "group": "files",
            "keys": ["Ctrl+S"],
            "label": I18n.tr("Save", "shortcut description")
        },
        {
            "id": "saveAs",
            "group": "files",
            "keys": ["Ctrl+Shift+S"],
            "label": I18n.tr("Save as", "shortcut description")
        },
        {
            "id": "quit",
            "group": "files",
            "keys": ["Ctrl+Q"],
            "label": I18n.tr("Quit", "shortcut description")
        },
        {
            "id": "closeTab",
            "group": "tabs",
            "keys": ["Ctrl+W"],
            "aliases": ["Ctrl+F4"],
            "label": I18n.tr("Close tab", "shortcut description")
        },
        {
            "id": "reopenTab",
            "group": "tabs",
            "keys": ["Ctrl+Shift+T"],
            "label": I18n.tr("Reopen closed tab", "shortcut description")
        },
        {
            "id": "nextTab",
            "group": "tabs",
            "keys": ["Ctrl+Tab", "Ctrl+PgDown"],
            "label": I18n.tr("Next tab", "shortcut description")
        },
        {
            "id": "previousTab",
            "group": "tabs",
            "keys": ["Ctrl+Shift+Tab", "Ctrl+PgUp"],
            "aliases": ["Ctrl+Shift+Backtab"],
            "label": I18n.tr("Previous tab", "shortcut description")
        },
        {
            "id": "moveTabLeft",
            "group": "tabs",
            "keys": ["Ctrl+Shift+PgUp"],
            "label": I18n.tr("Move tab left", "shortcut description")
        },
        {
            "id": "moveTabRight",
            "group": "tabs",
            "keys": ["Ctrl+Shift+PgDown"],
            "label": I18n.tr("Move tab right", "shortcut description")
        },
        {
            "id": "tabNumbers",
            "group": "tabs",
            "keys": ["Alt+1…8"],
            "unbound": true,
            "label": I18n.tr("Go to tab 1 to 8", "shortcut description")
        },
        {
            "id": "tab1",
            "keys": ["Alt+1"],
            "hidden": true
        },
        {
            "id": "tab2",
            "keys": ["Alt+2"],
            "hidden": true
        },
        {
            "id": "tab3",
            "keys": ["Alt+3"],
            "hidden": true
        },
        {
            "id": "tab4",
            "keys": ["Alt+4"],
            "hidden": true
        },
        {
            "id": "tab5",
            "keys": ["Alt+5"],
            "hidden": true
        },
        {
            "id": "tab6",
            "keys": ["Alt+6"],
            "hidden": true
        },
        {
            "id": "tab7",
            "keys": ["Alt+7"],
            "hidden": true
        },
        {
            "id": "tab8",
            "keys": ["Alt+8"],
            "hidden": true
        },
        {
            "id": "lastTab",
            "group": "tabs",
            "keys": ["Alt+9"],
            "label": I18n.tr("Go to the last tab", "shortcut description")
        },
        {
            "id": "undo",
            "group": "edit",
            "keys": ["Ctrl+Z"],
            "unbound": true,
            "label": I18n.tr("Undo", "shortcut description")
        },
        {
            "id": "redo",
            "group": "edit",
            "keys": ["Ctrl+Shift+Z", "Ctrl+Y"],
            "unbound": true,
            "label": I18n.tr("Redo", "shortcut description")
        },
        {
            "id": "copyAll",
            "group": "edit",
            "keys": ["Ctrl+Shift+C"],
            "label": I18n.tr("Copy the whole document", "shortcut description")
        },
        {
            "id": "overwrite",
            "group": "edit",
            "keys": ["Insert"],
            "unbound": true,
            "label": I18n.tr("Toggle overwrite mode", "shortcut description")
        },
        {
            "id": "smartHome",
            "group": "edit",
            "keys": ["Home"],
            "unbound": true,
            "label": I18n.tr("Go to the first character of the line, then its start", "shortcut description")
        },
        {
            "id": "deleteLine",
            "group": "lines",
            "keys": ["Ctrl+D"],
            "unbound": true,
            "label": I18n.tr("Delete line", "shortcut description")
        },
        {
            "id": "duplicateLine",
            "group": "lines",
            "keys": ["Ctrl+Alt+D", "Ctrl+Shift+D"],
            "unbound": true,
            "label": I18n.tr("Duplicate line", "shortcut description")
        },
        {
            "id": "moveLinesUp",
            "group": "lines",
            "keys": ["Alt+Up"],
            "unbound": true,
            "label": I18n.tr("Move lines up", "shortcut description")
        },
        {
            "id": "moveLinesDown",
            "group": "lines",
            "keys": ["Alt+Down"],
            "unbound": true,
            "label": I18n.tr("Move lines down", "shortcut description")
        },
        {
            "id": "selectLine",
            "group": "lines",
            "keys": ["Ctrl+L"],
            "unbound": true,
            "label": I18n.tr("Select line", "shortcut description")
        },
        {
            "id": "joinLines",
            "group": "lines",
            "keys": ["Ctrl+J"],
            "unbound": true,
            "label": I18n.tr("Join lines", "shortcut description")
        },
        {
            "id": "find",
            "group": "find",
            "keys": ["Ctrl+F"],
            "label": I18n.tr("Find", "shortcut description")
        },
        {
            "id": "replace",
            "group": "find",
            "keys": ["Ctrl+H"],
            "label": I18n.tr("Replace", "shortcut description")
        },
        {
            "id": "findNext",
            "group": "find",
            "keys": ["F3"],
            "label": I18n.tr("Find next", "shortcut description")
        },
        {
            "id": "findPrevious",
            "group": "find",
            "keys": ["Shift+F3"],
            "label": I18n.tr("Find previous", "shortcut description")
        },
        {
            "id": "goToLine",
            "group": "find",
            "keys": ["Ctrl+G"],
            "label": I18n.tr("Go to line", "shortcut description")
        },
        {
            "id": "toggleWrap",
            "group": "view",
            "keys": ["Alt+W"],
            "label": I18n.tr("Toggle word wrap for this document", "shortcut description")
        },
        {
            "id": "preview",
            "group": "view",
            "keys": ["Ctrl+P"],
            "label": I18n.tr("Cycle the Markdown preview", "shortcut description")
        },
        {
            "id": "fullscreen",
            "group": "view",
            "keys": ["F11"],
            "label": I18n.tr("Full screen", "shortcut description")
        },
        {
            "id": "zoomIn",
            "group": "view",
            "keys": ["Ctrl++"],
            "aliases": ["Ctrl+="],
            "label": I18n.tr("Zoom in", "shortcut description")
        },
        {
            "id": "zoomOut",
            "group": "view",
            "keys": ["Ctrl+-"],
            "label": I18n.tr("Zoom out", "shortcut description")
        },
        {
            "id": "zoomReset",
            "group": "view",
            "keys": ["Ctrl+0"],
            "label": I18n.tr("Reset zoom", "shortcut description")
        },
        {
            "id": "settings",
            "group": "view",
            "keys": ["Ctrl+,"],
            "label": I18n.tr("Settings", "shortcut description")
        }
    ]

    readonly property var bound: entries.filter(entry => !entry.unbound)

    function inGroup(group: string): var {
        return entries.filter(entry => entry.group === group && !entry.hidden);
    }

    function sequences(entry: var): var {
        return entry.keys.concat(entry.aliases ?? []);
    }

    function hint(id: string): string {
        return entries.find(entry => entry.id === id)?.keys[0] ?? "";
    }

    // "Ctrl+Shift+S" to ["Ctrl", "Shift", "S"]; a trailing "+" is the plus key itself.
    function caps(sequence: string): var {
        const parts = sequence.split("+");
        const out = [];
        for (let i = 0; i < parts.length; i++) {
            if (parts[i] !== "") {
                out.push(parts[i]);
                continue;
            }
            if (i === parts.length - 1)
                out.push("+");
        }
        return out;
    }
}
