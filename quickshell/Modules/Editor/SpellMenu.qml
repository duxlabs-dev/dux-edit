pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Common
import qs.DCommon.FileBrowser
import qs.Services

// The editor's context menu: spelling suggestions for a misspelled word, then the clipboard actions.
FileMenu {
    id: root

    required property Item editor

    // {start, end, word} of the misspelling under the click, or null.
    property var misspelling: null
    property var suggestions: []
    property bool loadingSuggestions: false
    property int requestSeq: 0

    readonly property bool canEdit: !editor.doc.readOnly
    readonly property bool hasSelection: editor.selectionEnd > editor.selectionStart

    // A Popup is not an Item, so it is held by a property and given the menu's parent to show in.
    property FileMenu subMenu: FileMenu {
        property string kind: ""

        parent: root.parent
        parentMenu: root

        FileMenuItem {
            visible: root.subMenu.kind === "lines"
            iconName: "content_copy"
            text: I18n.tr("Duplicate", "editor context menu, duplicate the line or selection")
            enabled: root.canEdit
            onClicked: root.pick(() => root.editor.duplicate())
        }

        FileMenuItem {
            visible: root.subMenu.kind === "lines"
            iconName: "delete"
            text: I18n.tr("Delete Lines", "editor context menu action")
            enabled: root.canEdit
            onClicked: root.pick(() => root.editor.deleteLines())
        }

        FileMenuItem {
            visible: root.subMenu.kind === "lines"
            iconName: "arrow_upward"
            text: I18n.tr("Move Up", "editor context menu, move lines up")
            enabled: root.canEdit
            onClicked: root.pick(() => root.editor.moveLinesUp())
        }

        FileMenuItem {
            visible: root.subMenu.kind === "lines"
            iconName: "arrow_downward"
            text: I18n.tr("Move Down", "editor context menu, move lines down")
            enabled: root.canEdit
            onClicked: root.pick(() => root.editor.moveLinesDown())
        }

        FileMenuItem {
            visible: root.subMenu.kind === "lines"
            iconName: "format_indent_increase"
            text: I18n.tr("Indent", "editor context menu action")
            enabled: root.canEdit
            onClicked: root.pick(() => root.editor.indent())
        }

        FileMenuItem {
            visible: root.subMenu.kind === "lines"
            iconName: "format_indent_decrease"
            text: I18n.tr("Unindent", "editor context menu action")
            enabled: root.canEdit
            onClicked: root.pick(() => root.editor.unindent())
        }

        FileMenuItem {
            visible: root.subMenu.kind === "lines"
            iconName: "merge_type"
            text: I18n.tr("Join Lines", "editor context menu action")
            enabled: root.canEdit
            onClicked: root.pick(() => root.editor.joinLines())
        }

        FileMenuItem {
            visible: root.subMenu.kind === "lines"
            iconName: "sort_by_alpha"
            text: I18n.tr("Sort Lines", "editor context menu action")
            enabled: root.canEdit && root.hasSelection
            onClicked: root.pick(() => root.editor.sortLines())
        }

        FileMenuItem {
            visible: root.subMenu.kind === "lines"
            iconName: "horizontal_rule"
            text: I18n.tr("Select Line", "editor context menu action")
            onClicked: root.pick(() => root.editor.selectLine())
        }

        FileMenuItem {
            visible: root.subMenu.kind === "case"
            text: I18n.tr("UPPER CASE", "editor context menu, change case")
            onClicked: root.pick(() => root.editor.upperCase())
        }

        FileMenuItem {
            visible: root.subMenu.kind === "case"
            text: I18n.tr("lower case", "editor context menu, change case")
            onClicked: root.pick(() => root.editor.lowerCase())
        }

        FileMenuItem {
            visible: root.subMenu.kind === "case"
            text: I18n.tr("Title Case", "editor context menu, change case")
            onClicked: root.pick(() => root.editor.titleCase())
        }
    }

    function pick(action) {
        close();
        subMenu.close();
        action();
    }

    function openSub(kind) {
        close();
        subMenu.kind = kind;
        subMenu.openAt(x, y);
    }

    function show(found, x, y) {
        misspelling = found;
        suggestions = [];
        loadingSuggestions = found !== null;
        const seq = ++requestSeq;
        if (found)
            Spell.suggest(found.word, list => {
                if (seq !== root.requestSeq)
                    return;
                root.suggestions = list.slice(0, EditorMetrics.spellSuggestions);
                root.loadingSuggestions = false;
            });
        openAt(x, y);
    }

    function replaceWith(word) {
        close();
        const found = misspelling;
        if (!found || editor.textRange(found.start, found.end) !== found.word)
            return;
        editor.replaceRange(found.start, found.end, word);
    }

    Repeater {
        model: root.misspelling ? root.suggestions : []

        FileMenuItem {
            required property string modelData

            text: modelData
            onClicked: root.replaceWith(modelData)
        }
    }

    FileMenuItem {
        visible: root.misspelling !== null && root.suggestions.length === 0
        enabled: false
        text: root.loadingSuggestions ? I18n.tr("Looking up suggestions…", "spelling menu, while suggestions load") : I18n.tr("No suggestions", "spelling menu")
    }

    FileMenuItem {
        visible: root.misspelling !== null
        iconName: "library_add"
        text: I18n.tr("Add to Dictionary", "spelling menu action")
        onClicked: {
            root.close();
            Spell.add(root.misspelling.word);
            root.editor.respell();
        }
    }

    FileMenuItem {
        visible: root.misspelling !== null
        iconName: "visibility_off"
        text: I18n.tr("Ignore", "spelling menu action, for this session")
        onClicked: {
            root.close();
            Spell.ignore(root.misspelling.word);
            root.editor.respell();
        }
    }

    FileMenuDivider {
        visible: root.misspelling !== null
    }

    FileMenuItem {
        iconName: "undo"
        text: I18n.tr("Undo", "edit menu action")
        enabled: root.editor.canUndo && root.canEdit
        onClicked: root.pick(() => root.editor.undo())
    }

    FileMenuItem {
        iconName: "redo"
        text: I18n.tr("Redo", "edit menu action")
        enabled: root.editor.canRedo && root.canEdit
        onClicked: root.pick(() => root.editor.redo())
    }

    FileMenuDivider {}

    FileMenuItem {
        iconName: "content_cut"
        text: I18n.tr("Cut", "edit menu action")
        enabled: root.hasSelection && root.canEdit
        onClicked: root.pick(() => root.editor.cut())
    }

    FileMenuItem {
        iconName: "content_copy"
        text: I18n.tr("Copy", "edit menu action")
        enabled: root.hasSelection
        onClicked: root.pick(() => root.editor.copy())
    }

    FileMenuItem {
        iconName: "content_paste"
        text: I18n.tr("Paste", "edit menu action")
        enabled: root.canEdit
        onClicked: root.pick(() => root.editor.paste())
    }

    FileMenuItem {
        iconName: "select_all"
        text: I18n.tr("Select All", "edit menu action")
        onClicked: root.pick(() => root.editor.selectAll())
    }

    FileMenuDivider {}

    FileMenuItem {
        iconName: "format_list_bulleted"
        text: I18n.tr("Lines", "editor context menu, submenu of line actions")
        submenu: true
        onClicked: root.openSub("lines")
    }

    FileMenuItem {
        iconName: "match_case"
        text: I18n.tr("Change Case", "editor context menu, submenu")
        submenu: true
        enabled: root.hasSelection && root.canEdit
        onClicked: root.openSub("case")
    }
}
