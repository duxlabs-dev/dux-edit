pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Widgets
import qs.Common
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Services
import "../../Common/EditTracker.js" as EditTracker
import "../../Common/LineIndex.js" as LineIndex
import "../../DCommon/Common/Contrast.js" as Contrast

// The editing surface of one tab: plain-text editor with syntax, spelling and match formats, visible-lines
// gutter, find bar and Markdown preview. The shell keeps one pane per loaded tab and reaches it through doc.editor.
Item {
    id: root

    required property var doc

    readonly property string selectedText: textEdit.selectedText
    readonly property bool canUndo: textEdit.canUndo
    readonly property bool canRedo: textEdit.canRedo
    readonly property int cursorPosition: textEdit.cursorPosition
    readonly property int selectionStart: textEdit.selectionStart
    readonly property int selectionEnd: textEdit.selectionEnd

    readonly property bool shown: visible
    readonly property bool simple: Documents.simple
    readonly property string lexer: root.doc.lexer
    readonly property bool markdown: root.doc.isMarkdown
    readonly property bool prose: EditorMetrics.proseLexers.includes(lexer.toLowerCase())
    readonly property bool wrapping: SettingsData.wrapMode === "on" || (SettingsData.wrapMode === "auto" && prose)
    readonly property string fontFamily: SettingsData.fontFamily || (SettingsData.useMonospace ? Style.monoFontFamily : Style.fontFamily)
    readonly property int fontSize: SettingsData.fontSize
    readonly property real padding: simple ? EditorMetrics.simplePadding : EditorMetrics.padding
    readonly property bool gutterShown: SettingsData.showLineNumbers
    readonly property real proseWidth: fontMetrics.averageCharacterWidth * EditorMetrics.proseColumns + padding * 2
    readonly property bool centered: simple && wrapping && flickable.width > proseWidth
    readonly property real screenWidth: Math.max(1, Screen.width)

    readonly property bool previewOpen: markdown && (root.doc.previewMode === "split" || root.doc.previewMode === "full")
    readonly property bool previewOnly: previewOpen && root.doc.previewMode === "full"

    readonly property bool highlightRequested: SettingsData.highlightEnabled
    readonly property bool highlightOn: highlightRequested && root.doc.tier === "exact"
    readonly property bool spellOn: Spell.active
    readonly property bool findOpen: findLoader.item?.opened ?? false
    readonly property int matchCount: findLoader.item?.matchCount ?? 0
    // Formats address text blocks by line number; a line separator splits a mirror line inside one block.
    property bool hasSeparators: false
    readonly property bool decorated: (highlightOn || spellOn || matchCount > 0) && !hasSeparators

    // Role 0 keeps the editor's own color.
    readonly property var palette: Highlight.palette.map((entry, role) => role === 0 ? ({}) : entry)
    readonly property color matchColor: Contrast.mix(Style.surface, Style.primary, Style.primarySelected.a)

    // Built lines around the viewport, line L in slot L % slots.length: {line, y, height}. Delegates keep
    // their slot while scrolling, so only lines entering the range are laid out again.
    property var slots: []
    property int currentLine: 0

    // Plain JS so per-keystroke writes never go through QML property comparisons.
    readonly property var mirror: ({
            "index": LineIndex.create(""),
            "loading": false,
            "docId": "",
            "builtFirst": 0,
            "builtLast": -1,
            "builtTop": 0,
            "builtBottom": -1,
            "sentFirst": -1,
            "sentLast": -1,
            "cursorLine": -1,
            "pairs": 0,
            "decorated": new Map()
        })

    // The text exactly as the core mirrors it, up to the NBSP and line separator normalization of TextEdit.
    function getText() {
        return textEdit.text;
    }

    // TextEdit lengths are UTF-16 units; the status bar counts code points, so an emoji is one character.
    function surrogatePairs(text) {
        return text.match(/[\udc00-\udfff]/g)?.length ?? 0;
    }

    // Text in [start, end) without reading the whole document.
    function textRange(start, end) {
        return mirror.index.text(start, end);
    }

    function insertText(text) {
        edit(textEdit.cursorPosition, 0, text);
    }

    function replaceSelection(text) {
        replaceRange(textEdit.selectionStart, textEdit.selectionEnd, text);
    }

    // Offsets are UTF-16 units. The common ends are kept, so a pure insert or delete is one undo step.
    function replaceRange(start, end, text) {
        const change = EditTracker.diff(mirror.index.text(start, end), text);
        if (!change)
            return;
        edit(start + change.start, change.removed, change.text);
    }

    function edit(start, removed, text) {
        if (removed > 0)
            textEdit.remove(start, start + removed);
        if (text.length > 0)
            textEdit.insert(start, text);
    }

    // 1-based; column 0 means the start of the line.
    function gotoLine(line, column) {
        const index = mirror.index;
        const target = Math.max(0, Math.min(index.count - 1, line - 1));
        textEdit.cursorPosition = Math.min(index.endOf(target), index.startOf(target) + Math.max(0, column - 1));
        const rect = textEdit.positionToRectangle(textEdit.cursorPosition);
        flickable.contentY = Math.max(0, Math.min(flickable.contentHeight - flickable.height, rect.y - flickable.height / 3));
        focusEditor();
    }

    function openFind(withReplace) {
        const selection = textEdit.selectedText;
        findLoader.active = true;
        findLoader.item.open(withReplace === true, /[\n\u2029]/.test(selection) ? "" : selection);
    }

    function closeFind() {
        findLoader.item?.close();
        focusEditor();
    }

    function focusEditor() {
        if (previewOnly)
            return;
        textEdit.forceActiveFocus();
    }

    function cut() {
        textEdit.cut();
    }

    function copy() {
        textEdit.copy();
    }

    function paste() {
        textEdit.paste();
    }

    function undo() {
        textEdit.undo();
    }

    function redo() {
        textEdit.redo();
    }

    function selectAll() {
        textEdit.selectAll();
    }

    // Selects [start, end) and scrolls it into view; used by the find bar.
    function selectRange(start, end) {
        textEdit.select(start, end);
        revealRect(textEdit.positionToRectangle(start));
    }

    // Flips the Markdown task box on a 0-based source line, as one undoable edit.
    function toggleTask(line) {
        if (line < 0 || line >= mirror.index.count || textEdit.readOnly)
            return;
        const found = /^(\s*(?:>\s*)*(?:[-*+]|\d+[.)])\s+\[)([ xX])\]/.exec(mirror.index.lineText(line));
        if (!found)
            return;
        const at = mirror.index.startOf(line) + found[1].length;
        edit(at, 1, found[2] === " " ? "x" : " ");
    }

    // Asks the core to check the visible lines again, after the dictionary changed.
    function respell() {
        mirror.cursorLine = -1;
        sendSpellView();
    }

    function revealRect(rect) {
        const top = rect.y + textEdit.y;
        const bottom = top + rect.height;
        if (top < flickable.contentY)
            flickable.contentY = Math.max(0, top - root.padding);
        else if (bottom > flickable.contentY + flickable.height)
            flickable.contentY = Math.min(flickable.contentHeight - flickable.height, bottom - flickable.height + root.padding);
        if (root.wrapping)
            return;
        const left = rect.x + textEdit.x;
        if (left < flickable.contentX + root.padding)
            flickable.contentX = Math.max(0, left - root.padding);
        else if (left > flickable.contentX + flickable.width - root.padding)
            flickable.contentX = Math.min(flickable.contentWidth - flickable.width, left - flickable.width + root.padding);
    }

    function applyLoadedText(text) {
        mirror.loading = true;
        hasSeparators = /[\u2028\u2029\ufdd0\ufdd1]/.test(text);
        mirror.index = LineIndex.create(text);
        // Before the text goes in: a gutter that widens afterwards would lay the whole document out again.
        root.doc.lineCount = mirror.index.count;
        textEdit.text = text;
        // TextEdit reports NBSP as a space and line separators as "\n"; the lines must match what it reports.
        if (hasSeparators || text.indexOf("\u00a0") >= 0)
            mirror.index = LineIndex.create(textEdit.text);
        mirror.docId = root.doc.docId;
        mirror.sentFirst = -1;
        mirror.sentLast = -1;
        mirror.cursorLine = -1;
        mirror.decorated.clear();
        Highlight.forget(mirror.docId);
        Spell.forget(mirror.docId);
        mirror.pairs = surrogatePairs(text);
        root.doc.charCount = mirror.index.length - mirror.pairs;
        root.doc.lineCount = mirror.index.count;
        applyPendingPosition();
        mirror.loading = false;
        updateCursor();
        refreshView(true);
        previewLoader.item?.documentEdited();
    }

    function applyPendingPosition() {
        const doc = root.doc;
        if (doc.pendingLine > 0) {
            gotoLine(doc.pendingLine, doc.pendingColumn);
            doc.pendingLine = 0;
            doc.pendingColumn = 0;
            doc.pendingCursor = -1;
            return;
        }
        if (doc.pendingCursor >= 0) {
            textEdit.cursorPosition = Math.min(doc.pendingCursor, textEdit.length);
            doc.pendingCursor = -1;
        }
        flickable.contentY = Math.max(0, Math.min(doc.scrollY, textEdit.height - flickable.height));
    }

    // One replacement per edit, undo and redo included, read back from the document in UTF-16 units.
    function contentsEdited(position, removed, added) {
        if (mirror.loading)
            return;
        applyChange({
            "start": position,
            "removed": removed,
            "text": decorations.text(position, position + added)
        }, textEdit.length);
        updateCursor();
        refreshView(true);
    }

    function applyChange(change, length) {
        // The lines keep what TextEdit reports, so later reads compare like with like.
        const shown = change.text.replace(/\u00a0/g, " ").replace(/[\u2028\u2029\ufdd0\ufdd1]/g, "\n");
        const removedPairs = change.removed > 0 ? surrogatePairs(mirror.index.text(change.start, change.start + change.removed)) : 0;
        mirror.pairs += surrogatePairs(change.text) - removedPairs;
        const lines = mirror.index.applyEdit(change.start, change.removed, shown);
        const doc = root.doc;
        doc.edited(change.start, change.removed, change.text, length);
        Highlight.linesEdited(mirror.docId, lines.startLine, lines.oldLines, lines.newLines, doc.version);
        Spell.linesEdited(mirror.docId, lines.startLine, lines.oldLines, lines.newLines, doc.version);
        if (shown !== change.text)
            hasSeparators = hasSeparators || /[\u2028\u2029\ufdd0\ufdd1]/.test(change.text);
        doc.charCount = length - mirror.pairs;
        doc.lineCount = mirror.index.count;
        findLoader.item?.documentEdited(change);
        previewLoader.item?.documentEdited();
    }

    function updateCursor() {
        if (mirror.loading)
            return;
        const pos = textEdit.cursorPosition;
        const line = mirror.index.lineAt(pos);
        const doc = root.doc;
        doc.cursorPosition = pos;
        doc.cursorLine = line + 1;
        doc.cursorColumn = pos - mirror.index.startOf(line) + 1;
        const selected = textEdit.selectionEnd - textEdit.selectionStart;
        doc.selectionLength = selected > 0 ? selected - surrogatePairs(mirror.index.text(textEdit.selectionStart, textEdit.selectionEnd)) : 0;
        currentLine = line;
        mirror.cursorLine = line;
        // On every move, not only a new line: the core skips the word being typed by its column.
        sendSpellView();
    }

    function sendSpellView() {
        if (!spellOn || mirror.sentFirst < 0)
            return;
        const line = mirror.cursorLine;
        const column = line >= 0 ? textEdit.cursorPosition - mirror.index.startOf(line) : -1;
        Spell.view(root.doc, mirror.sentFirst, mirror.sentLast, line, column);
    }

    // The line whose first row starts at or above y. A binary search over positionToRectangle: positionAt
    // walks the blocks and costs about 6 ms on 2 MB after every edit.
    function visibleLine(y) {
        const index = mirror.index;
        const target = y - textEdit.y;
        let low = 0;
        let high = index.count - 1;
        while (low < high) {
            const mid = (low + high + 1) >> 1;
            if (textEdit.positionToRectangle(index.startOf(mid)).y <= target)
                low = mid;
            else
                high = mid - 1;
        }
        return low;
    }

    // Rebuilds the slots when the viewport leaves the built range, or always with force.
    function refreshView(force) {
        if (mirror.loading || !shown || flickable.height <= 0)
            return;
        const top = flickable.contentY;
        const bottom = top + flickable.height;
        const index = mirror.index;
        const inside = (top >= mirror.builtTop || mirror.builtFirst === 0) && (bottom <= mirror.builtBottom || mirror.builtLast === index.count - 1);
        if (!force && mirror.builtLast >= 0 && inside)
            return;
        const first = visibleLine(top);
        const last = visibleLine(bottom);
        const margin = Math.max(1, Math.ceil((last - first + 1) / 2));
        mirror.builtFirst = Math.max(0, first - margin);
        mirror.builtLast = Math.min(index.count - 1, last + margin);
        for (const line of mirror.decorated.keys()) {
            if (line < mirror.builtFirst || line > mirror.builtLast)
                mirror.decorated.delete(line);
        }
        sendViews(first, last);
        decorate();
        layoutSlots();
    }

    // Gutter geometry for the built lines, measured after their formats are in: bold runs can rewrap a line.
    function layoutSlots() {
        const index = mirror.index;
        const capacity = Math.max(slots.length, mirror.builtLast - mirror.builtFirst + 1);
        const built = new Array(capacity).fill(null);
        for (let line = mirror.builtFirst; line <= mirror.builtLast; line++) {
            const rect = textEdit.positionToRectangle(index.startOf(line));
            built[line % capacity] = {
                "line": line,
                "y": rect.y,
                "height": rect.height
            };
        }
        const end = textEdit.positionToRectangle(index.endOf(mirror.builtLast));
        mirror.builtTop = textEdit.positionToRectangle(index.startOf(mirror.builtFirst)).y;
        mirror.builtBottom = end.y + end.height;
        slots = built;
    }

    function sendViews(first, last) {
        if (first >= mirror.sentFirst && last <= mirror.sentLast && mirror.sentFirst >= 0)
            return;
        const screen = last - first + 1;
        mirror.sentFirst = Math.max(0, first - screen);
        mirror.sentLast = Math.min(mirror.index.count - 1, last + screen);
        if (highlightRequested)
            Highlight.view(root.doc, mirror.sentFirst, mirror.sentLast);
        sendSpellView();
    }

    function resendViews() {
        mirror.sentFirst = -1;
        mirror.sentLast = -1;
        refreshView(true);
    }

    // Applies new spans, spelling or matches to the built lines.
    function redecorate() {
        if (mirror.builtLast < 0)
            return;
        if (decorate() && wrapping)
            layoutSlots();
    }

    // Lines outside the built range keep their formats until they are built again.
    function restyle() {
        mirror.decorated.clear();
        if (!decorated) {
            decorations.clear();
            return;
        }
        redecorate();
    }

    // Formats the built lines whose text, spans, spelling or matches changed. True when any format did.
    function decorate() {
        if (!decorated || mirror.builtLast < 0)
            return false;
        const index = mirror.index;
        const spanStore = highlightOn ? Highlight.spans(mirror.docId) : null;
        const spellStore = spellOn ? Spell.ranges(mirror.docId) : null;
        const matches = matchCount > 0 ? findLoader.item.matches : null;
        let changed = false;
        for (let line = mirror.builtFirst; line <= mirror.builtLast; line++) {
            const text = index.lineText(line);
            const spans = spanStore?.get(line) ?? null;
            const spell = spellStore?.get(line) ?? null;
            const found = matches ? lineMatches(matches, index.startOf(line), index.endOf(line)) : null;
            const key = found ? found.join(",") : "";
            const cached = mirror.decorated.get(line);
            if (cached && cached.text === text && cached.spans === spans && cached.spell === spell && cached.key === key)
                continue;
            mirror.decorated.set(line, {
                "text": text,
                "spans": spans,
                "spell": spell,
                "key": key
            });
            changed = decorations.setLine(line, spans ?? [], spell ?? [], found ?? []) || changed;
        }
        return changed;
    }

    // Matches inside [lineStart, lineStop) as [col, len, ...], or null. `matches` is sorted and flat.
    function lineMatches(matches, lineStart, lineStop) {
        let low = 0;
        let high = matches.length / 2;
        while (low < high) {
            const mid = (low + high) >> 1;
            if (matches[mid * 2] + matches[mid * 2 + 1] <= lineStart)
                low = mid + 1;
            else
                high = mid;
        }
        const out = [];
        for (let i = low * 2; i < matches.length && matches[i] <= lineStop; i += 2) {
            const start = Math.max(matches[i], lineStart);
            const end = Math.min(matches[i] + matches[i + 1], lineStop);
            if (end > start)
                out.push(start - lineStart, end - start);
        }
        return out.length ? out : null;
    }

    function misspelledAt(position) {
        if (!spellOn)
            return null;
        const index = mirror.index;
        const line = index.lineAt(position);
        const ranges = Spell.ranges(mirror.docId)?.get(line);
        if (!ranges)
            return null;
        const column = position - index.startOf(line);
        for (let i = 0; i + 1 < ranges.length; i += 2) {
            if (column < ranges[i] || column > ranges[i] + ranges[i + 1])
                continue;
            const start = index.startOf(line) + ranges[i];
            return {
                "start": start,
                "end": start + ranges[i + 1],
                "word": index.text(start, start + ranges[i + 1])
            };
        }
        return null;
    }

    function openContextMenu(x, y) {
        const position = textEdit.positionAt(x, y);
        if (position < textEdit.selectionStart || position > textEdit.selectionEnd)
            textEdit.cursorPosition = position;
        const point = textEdit.mapToItem(root, x, y);
        spellMenu.show(misspelledAt(position), point.x, point.y);
    }

    function insertSpacesForTab() {
        const index = mirror.index;
        const column = textEdit.selectionStart - index.startOf(index.lineAt(textEdit.selectionStart));
        const width = Math.max(1, SettingsData.tabWidth);
        replaceSelection(" ".repeat(width - column % width));
    }

    onShownChanged: {
        if (shown) {
            refreshView(true);
            return;
        }
        // A hidden tab keeps its text and formats but no gutter items.
        slots = [];
        mirror.builtLast = -1;
        mirror.decorated.clear();
    }
    onDecoratedChanged: restyle()
    onHighlightOnChanged: restyle()
    onSpellOnChanged: resendViews()
    onHighlightRequestedChanged: resendViews()
    onWrappingChanged: refreshView(true)
    onFontFamilyChanged: refreshView(true)
    onFontSizeChanged: refreshView(true)
    onCenteredChanged: refreshView(true)
    onPaddingChanged: refreshView(true)

    Component.onCompleted: root.doc.editor = root
    Component.onDestruction: {
        Highlight.forget(mirror.docId);
        Spell.forget(mirror.docId);
        if (root.doc && root.doc.editor === root)
            root.doc.editor = null;
    }

    Connections {
        target: root.doc
        function onTextLoaded(text) {
            root.applyLoadedText(text);
        }
    }

    Connections {
        target: Highlight
        function onSpansChanged(docId) {
            if (docId === root.mirror.docId && root.highlightOn && root.decorated)
                root.redecorate();
        }
    }

    Connections {
        target: Spell
        function onRangesChanged(docId) {
            if (docId === root.mirror.docId && root.spellOn && root.decorated)
                root.redecorate();
        }
    }

    FontMetrics {
        id: fontMetrics
        font.family: root.fontFamily
        font.pixelSize: root.fontSize
    }

    TextDecorations {
        id: decorations

        document: textEdit.textDocument
        palette: root.palette
        spellColor: Style.error
        matchColor: root.matchColor

        onContentsChanged: (position, removed, added) => root.contentsEdited(position, removed, added)
        onPaletteChanged: root.restyle()
        onSpellColorChanged: root.restyle()
        onMatchColorChanged: root.restyle()
    }

    Loader {
        id: findLoader

        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        active: false
        visible: root.findOpen

        sourceComponent: FindBar {
            doc: root.doc
            editor: root
            onCloseRequested: root.closeFind()
            onMatchesUpdated: root.redecorate()
        }
    }

    Item {
        id: editorArea

        anchors.top: root.findOpen ? findLoader.bottom : parent.top
        anchors.left: parent.left
        anchors.bottom: parent.bottom
        width: !root.previewOpen ? parent.width : root.previewOnly ? 0 : Math.round(parent.width / 2)
        visible: !root.previewOnly

        Gutter {
            id: gutter

            anchors.top: parent.top
            anchors.bottom: parent.bottom
            anchors.left: parent.left
            visible: root.gutterShown
            width: visible ? implicitWidth : 0
            slots: root.slots
            currentLine: root.currentLine
            lineCount: root.doc.lineCount
            contentY: flickable.contentY
            font: textEdit.font
        }

        DFlickable {
            id: flickable

            anchors.top: parent.top
            anchors.bottom: parent.bottom
            anchors.left: gutter.right
            anchors.right: parent.right
            clip: true
            fadeEdges: false
            flickableDirection: root.wrapping ? Flickable.VerticalFlick : Flickable.AutoFlickIfNeeded
            horizontalScrollBar.allowed: !root.wrapping
            contentWidth: root.wrapping ? width : Math.max(width, textEdit.contentWidth + textEdit.leftPadding + textEdit.rightPadding)
            contentHeight: textEdit.height

            onContentYChanged: {
                if (!root.mirror.loading)
                    root.doc.scrollY = contentY;
                root.refreshView(false);
            }
            onHeightChanged: root.refreshView(true)
            onWidthChanged: {
                if (root.wrapping)
                    root.refreshView(true);
            }

            // DFlickable only scrolls vertically by wheel; unwrapped code also takes horizontal wheel input.
            WheelHandler {
                enabled: !root.wrapping
                orientation: Qt.Horizontal
                acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
                onWheel: event => {
                    const delta = event.pixelDelta.x !== 0 ? event.pixelDelta.x : event.angleDelta.x / 8;
                    if (delta === 0)
                        return;
                    flickable.contentX = Math.max(0, Math.min(flickable.contentWidth - flickable.width, flickable.contentX - delta));
                }
            }

            TextEdit {
                id: textEdit

                x: root.centered ? Math.round((flickable.width - width) / 2) : 0
                // Every width change lays the whole document out again (about 1 s on 2 MB), and reading
                // implicitWidth does so twice per edit. Unwrapped, the width grows a screen at a time and
                // ignores window and gutter resizes.
                width: root.wrapping ? (root.centered ? root.proseWidth : flickable.width) : Math.ceil((contentWidth + leftPadding + rightPadding + 1) / root.screenWidth) * root.screenWidth
                height: Math.max(implicitHeight, flickable.height)
                padding: root.padding
                textFormat: TextEdit.PlainText
                wrapMode: root.wrapping ? TextEdit.Wrap : TextEdit.NoWrap
                readOnly: root.doc.readOnly || !root.doc.loaded
                selectByMouse: true
                persistentSelection: true
                color: Style.onSurface
                selectionColor: Style.primary
                selectedTextColor: Style.onPrimary
                font.family: root.fontFamily
                font.pixelSize: root.fontSize
                tabStopDistance: Math.max(1, SettingsData.tabWidth * fontMetrics.advanceWidth(" "))

                cursorDelegate: DTextCursor {
                    readonly property int editCursorPosition: textEdit.cursorPosition

                    color: Style.primary
                    x: textEdit.cursorRectangle.x
                    y: textEdit.cursorRectangle.y
                    height: textEdit.cursorRectangle.height
                    shown: textEdit.cursorVisible

                    onEditCursorPositionChanged: resetBlink()
                }

                onCursorPositionChanged: root.updateCursor()
                onSelectionStartChanged: root.updateCursor()
                onSelectionEndChanged: root.updateCursor()
                onCursorRectangleChanged: {
                    if (!root.mirror.loading && activeFocus)
                        root.revealRect(cursorRectangle);
                }
                onTabStopDistanceChanged: root.refreshView(true)
                onImplicitHeightChanged: root.refreshView(false)

                Keys.onPressed: event => {
                    switch (event.key) {
                    case Qt.Key_Tab:
                        if (!SettingsData.insertSpaces || event.modifiers & (Qt.ControlModifier | Qt.AltModifier | Qt.MetaModifier))
                            return;
                        root.insertSpacesForTab();
                        event.accepted = true;
                        return;
                    case Qt.Key_Escape:
                        if (!root.findOpen)
                            return;
                        root.closeFind();
                        event.accepted = true;
                        return;
                    }
                }

                HoverHandler {
                    cursorShape: Qt.IBeamCursor
                }

                TapHandler {
                    acceptedButtons: Qt.RightButton
                    onTapped: eventPoint => root.openContextMenu(eventPoint.position.x, eventPoint.position.y)
                }

                StyledText {
                    x: textEdit.leftPadding
                    y: textEdit.topPadding
                    width: textEdit.width - textEdit.leftPadding - textEdit.rightPadding
                    visible: root.doc.isDraft && textEdit.length === 0 && !textEdit.inputMethodComposing
                    text: I18n.tr("Start typing…", "placeholder in an empty document")
                    font.family: textEdit.font.family
                    font.pixelSize: textEdit.font.pixelSize
                    color: Style.onSurfaceVariant
                    verticalAlignment: Text.AlignTop
                }
            }
        }
    }

    Rectangle {
        anchors.top: editorArea.top
        anchors.bottom: parent.bottom
        anchors.left: editorArea.right
        width: EditorMetrics.dividerWidth
        visible: root.previewOpen && !root.previewOnly
        color: Style.outlineVariant
    }

    Loader {
        id: previewLoader

        anchors.top: editorArea.top
        anchors.bottom: parent.bottom
        anchors.left: root.previewOnly ? parent.left : editorArea.right
        anchors.leftMargin: root.previewOnly ? 0 : EditorMetrics.dividerWidth
        anchors.right: parent.right
        active: root.previewOpen

        sourceComponent: MarkdownPreview {
            doc: root.doc
        }
    }

    SpellMenu {
        id: spellMenu

        editor: root
    }
}
