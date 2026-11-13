pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.DCommon.Common
import qs.DCommon.FileBrowser
import qs.DCommon.Widgets
import qs.Services

// Advanced shows the full status; Simple only the counts, the save state and the position.
Rectangle {
    id: root

    property EditorDocument doc: null
    property bool simple: false

    readonly property bool hasDoc: doc !== null
    readonly property bool fileBacked: hasDoc && !doc.isDraft
    readonly property bool largeTier: hasDoc && doc.tier === "off" && ["size", "lines", "longLine"].includes(doc.tierReason)
    readonly property var pane: doc?.editor ?? null
    readonly property bool overwrite: pane?.overwrite === true
    readonly property bool readOnly: fileBacked && doc.loaded && (doc.readOnly || (!doc.writable && !doc.elevated))

    // Names as core/internal/textcodec.Canonical reports them.
    readonly property var encodings: [
        {
            "label": "UTF-8",
            "encoding": "utf-8",
            "bom": false
        },
        {
            "label": "UTF-8 BOM",
            "encoding": "utf-8",
            "bom": true
        },
        {
            "label": "UTF-16 LE",
            "encoding": "utf-16le",
            "bom": true
        },
        {
            "label": "UTF-16 BE",
            "encoding": "utf-16be",
            "bom": true
        },
        {
            "label": "ISO-8859-1",
            "encoding": "iso-8859-1",
            "bom": false
        },
        {
            "label": "Windows-1252",
            "encoding": "windows-1252",
            "bom": false
        },
        {
            "label": "ISO-8859-15",
            "encoding": "iso-8859-15",
            "bom": false
        },
        {
            "label": "Windows-1251",
            "encoding": "windows-1251",
            "bom": false
        },
        {
            "label": "KOI8-R",
            "encoding": "koi8-r",
            "bom": false
        },
        {
            "label": "Shift_JIS",
            "encoding": "shift_jis",
            "bom": false
        },
        {
            "label": "EUC-KR",
            "encoding": "euc-kr",
            "bom": false
        },
        {
            "label": "GBK",
            "encoding": "gbk",
            "bom": false
        },
        {
            "label": "Big5",
            "encoding": "big5",
            "bom": false
        }
    ]

    readonly property string encodingLabel: {
        if (!hasDoc)
            return "";
        const known = encodings.find(entry => entry.encoding === doc.encoding && (entry.bom === doc.bom || !entry.encoding.startsWith("utf-8")));
        return known ? known.label : doc.encoding.toUpperCase();
    }

    readonly property string saveState: {
        if (!hasDoc)
            return "";
        if (doc.saving)
            return I18n.tr("Saving…", "status bar save state");
        if (doc.dirty)
            return I18n.tr("Unsaved changes", "status bar save state");
        if (doc.isDraft)
            return "";
        return doc.autoSaved ? I18n.tr("Auto saved", "status bar save state") : I18n.tr("Saved", "status bar save state");
    }

    readonly property string position: {
        if (!hasDoc)
            return "";
        return I18n.tr("Ln %1, Col %2", "status bar cursor position").arg(doc.cursorLine).arg(doc.cursorColumn);
    }

    readonly property string counts: {
        if (!hasDoc)
            return "";
        const total = formatCount(doc.charCount);
        if (doc.selectionLength > 0)
            return I18n.tr("%1 of %2 characters", "status bar selected and total characters").arg(formatCount(doc.selectionLength)).arg(total);
        if (simple)
            return I18n.tr("%1 characters", "status bar character count").arg(total);
        return I18n.tr("%1 characters, %2 lines", "status bar document counts").arg(total).arg(formatCount(doc.lineCount));
    }

    function formatCount(value) {
        return Number(value).toLocaleString(Qt.locale(), "f", 0);
    }

    implicitHeight: Style.buttonHeightXS + Style.spacingXS * 2
    color: Style.hostSurface

    component Chip: StyledButton {
        id: chip

        property alias label: chipText.text
        property alias tooltipText: layer.tooltipText
        property color contentColor: Style.onSurfaceVariant

        function showTooltip() {
            layer.showTooltip();
        }

        anchors.verticalCenter: parent?.verticalCenter
        implicitWidth: chipText.implicitWidth + Style.spacingS * 2
        implicitHeight: Style.buttonHeightXXS
        radius: Style.cornerRadiusS
        color: "transparent"
        focusPolicy: Qt.TabFocus

        StateLayer {
            id: layer

            control: chip
            stateColor: chip.contentColor
            tooltipSide: "top"
        }

        FocusRing {
            visible: chip.visualFocus
        }

        StyledText {
            id: chipText

            anchors.centerIn: parent
            width: Math.min(implicitWidth, Math.max(0, chip.width - Style.spacingS * 2))
            elide: Text.ElideMiddle
            wrapMode: Text.NoWrap
            color: chip.contentColor
            font.pixelSize: Style.fontSizeSmall
        }
    }

    component Info: StyledText {
        anchors.verticalCenter: parent?.verticalCenter
        color: Style.onSurfaceVariant
        font.pixelSize: Style.fontSizeSmall
        leftPadding: Style.spacingS
        rightPadding: Style.spacingS
    }

    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: Style.dividerWidth
        color: Style.outlineVariant
    }

    Row {
        id: left

        anchors.left: parent.left
        anchors.leftMargin: Style.spacingS
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        spacing: Style.spacingXXS
        visible: root.hasDoc

        Chip {
            id: countsInfo

            readonly property string defaultTooltip: I18n.tr("Count Words", "tooltip on the status bar counts")

            label: root.counts
            tooltipText: defaultTooltip
            onHoveredChanged: {
                if (!hovered)
                    tooltipText = defaultTooltip;
            }
            onClicked: {
                const pane = root.pane;
                if (typeof pane?.wordCount !== "function")
                    return;
                const words = pane.wordCount();
                tooltipText = words === 1 ? I18n.tr("1 word", "status bar word count") : I18n.tr("%1 words", "status bar word count; %1 is a number").arg(root.formatCount(words));
                showTooltip();
            }
        }

        Chip {
            id: pathChip

            width: Math.min(implicitWidth, Math.max(0, right.x - left.x - countsInfo.width - left.spacing - Style.spacingS))
            visible: !root.simple && root.fileBacked
            label: root.fileBacked ? AppPaths.shortenHome(root.doc.path) : ""
            tooltipText: I18n.tr("Copy Path", "tooltip on the status bar path")
            onClicked: {
                Quickshell.clipboardText = root.doc.path;
                ToastService.showInfo(I18n.tr("Path copied", "notice after copying a file path"));
            }
        }
    }

    Row {
        id: right

        anchors.right: parent.right
        anchors.rightMargin: Style.spacingS
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        spacing: Style.spacingXXS
        visible: root.hasDoc

        Repeater {
            model: root.simple ? [] : PluginService.statusItems

            Loader {
                required property var modelData

                anchors.verticalCenter: parent.verticalCenter
                sourceComponent: modelData.component
            }
        }

        Info {
            visible: root.largeTier && !root.simple
            text: I18n.tr("Highlighting off for large files", "status bar notice")
        }

        Chip {
            visible: root.largeTier && !root.simple
            label: I18n.tr("Highlight Anyway", "status bar button that highlights a large file")
            contentColor: Style.primary
            onClicked: Highlight.setLexer(root.doc, root.doc.lexerOverride, true)
        }

        Chip {
            visible: root.readOnly
            label: root.doc?.readOnly ? I18n.tr("Read-Only", "status bar state of a document that cannot be edited as it is") : I18n.tr("Not Writable", "status bar state of a file the user may not write")
            contentColor: Style.primary
            tooltipText: I18n.tr("Show Why", "tooltip on the status bar read-only state")
            onClicked: root.doc.readOnlyNoticeDismissed = false
        }

        Info {
            text: root.saveState
            visible: text !== ""
        }

        Info {
            visible: root.overwrite
            text: I18n.tr("OVR", "status bar marker for overwrite mode; keep it short")
        }

        Info {
            text: root.position
        }

        Chip {
            id: encodingChip

            visible: !root.simple
            label: root.encodingLabel
            tooltipText: I18n.tr("Encoding", "tooltip on the status bar encoding")
            onClicked: encodingMenu.openAt(x + right.x, -encodingMenu.implicitHeight)
        }

        Chip {
            id: eolChip

            visible: !root.simple
            label: root.hasDoc ? (root.doc.mixedEol && root.doc.pendingEol === "" ? I18n.tr("Mixed", "status bar line endings when a file mixes them") : (root.doc.pendingEol || root.doc.eol).toUpperCase()) : ""
            tooltipText: I18n.tr("Line Endings", "tooltip on the status bar line endings")
            onClicked: {
                eolPicker.currentValue = root.doc.pendingEol || root.doc.eol;
                eolPicker.openDropdownMenu();
            }
        }

        Chip {
            id: languageChip

            visible: !root.simple
            label: root.doc?.lexer ?? ""
            tooltipText: I18n.tr("Language", "tooltip on the status bar language")
            onClicked: {
                Highlight.loadLexers();
                languagePicker.currentValue = root.doc.lexerOverride || "auto";
                languagePicker.openDropdownMenu();
            }
        }

        Chip {
            id: spellChip

            visible: !root.simple && Spell.available && SettingsData.spellcheckEnabled
            label: Spell.language
            tooltipText: I18n.tr("Spellcheck Language", "tooltip on the status bar spellcheck language")
            onClicked: {
                Spell.loadLanguages();
                spellPicker.currentValue = SettingsData.spellcheckLanguage;
                spellPicker.openDropdownMenu();
            }
        }
    }

    DDropdown {
        id: eolPicker

        showTrigger: false
        popupAnchorItem: eolChip
        openUpwards: true
        options: ["LF", "CRLF", "CR"]
        optionValues: ["lf", "crlf", "cr"]
        onValueChanged: value => Documents.setLineEnding(root.doc, value)
    }

    DDropdown {
        id: languagePicker

        showTrigger: false
        popupAnchorItem: languageChip
        openUpwards: true
        enableFuzzySearch: true
        options: [I18n.tr("Auto", "language picker entry that detects the language")].concat(Highlight.lexers.map(lexer => lexer.name))
        optionValues: ["auto"].concat(Highlight.lexers.map(lexer => lexer.name))
        onValueChanged: value => {
            const name = value === "auto" ? "" : value;
            root.doc.lexerOverride = name;
            Highlight.setLexer(root.doc, name, false);
            Documents.storeSession();
        }
    }

    DDropdown {
        id: spellPicker

        showTrigger: false
        popupAnchorItem: spellChip
        openUpwards: true
        enableFuzzySearch: true
        options: [I18n.tr("Auto", "spellcheck language entry that follows the system locale")].concat(Spell.languages.map(language => language.code))
        optionValues: ["auto"].concat(Spell.languages.map(language => language.code))
        onValueChanged: value => SettingsData.spellcheckLanguage = value
    }

    FileMenu {
        id: encodingMenu

        FileMenuItem {
            enabled: root.fileBacked
            iconName: "refresh"
            text: I18n.tr("Reopen with Encoding", "status bar encoding menu section")
            onClicked: {
                encodingMenu.close();
                encodingPicker.reopen = true;
                encodingPicker.openDropdownMenu();
            }
        }

        FileMenuItem {
            iconName: "save"
            text: I18n.tr("Save with Encoding", "status bar encoding menu section")
            onClicked: {
                encodingMenu.close();
                encodingPicker.reopen = false;
                encodingPicker.openDropdownMenu();
            }
        }
    }

    DDropdown {
        id: encodingPicker

        property bool reopen: false

        showTrigger: false
        popupAnchorItem: encodingChip
        openUpwards: true
        options: root.encodings.map(entry => entry.label)
        onValueChanged: value => {
            const entry = root.encodings.find(candidate => candidate.label === value);
            if (!entry)
                return;
            if (reopen) {
                Documents.reopenWithEncoding(root.doc, entry.encoding);
                return;
            }
            Documents.saveConverted(root.doc, {
                "encoding": entry.encoding,
                "bom": entry.bom
            });
        }
    }
}
