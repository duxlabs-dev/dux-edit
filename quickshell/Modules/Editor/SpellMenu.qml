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
        iconName: "content_cut"
        text: I18n.tr("Cut", "edit menu action")
        enabled: root.editor.selectionEnd > root.editor.selectionStart && !root.editor.doc.readOnly
        onClicked: {
            root.close();
            root.editor.cut();
        }
    }

    FileMenuItem {
        iconName: "content_copy"
        text: I18n.tr("Copy", "edit menu action")
        enabled: root.editor.selectionEnd > root.editor.selectionStart
        onClicked: {
            root.close();
            root.editor.copy();
        }
    }

    FileMenuItem {
        iconName: "content_paste"
        text: I18n.tr("Paste", "edit menu action")
        enabled: !root.editor.doc.readOnly
        onClicked: {
            root.close();
            root.editor.paste();
        }
    }

    FileMenuItem {
        iconName: "select_all"
        text: I18n.tr("Select All", "edit menu action")
        onClicked: {
            root.close();
            root.editor.selectAll();
        }
    }
}
