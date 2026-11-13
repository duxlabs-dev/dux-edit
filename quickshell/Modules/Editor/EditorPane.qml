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
import "../../Common/TextOps.js" as TextOps
import "../../Common/BracketMatch.js" as BracketMatch
import "../../DCommon/Common/Contrast.js" as Contrast

// One loaded tab's editing surface, exposed through doc.editor.
Item {
    id: root

    required property var doc

    readonly property bool canUndo: textEdit.canUndo
    readonly property bool canRedo: textEdit.canRedo
    readonly property int cursorPosition: textEdit.cursorPosition
    readonly property int selectionStart: textEdit.selectionStart
    readonly property int selectionEnd: textEdit.selectionEnd
    readonly property bool overwrite: textEdit.overwriteMode
    property bool redoing: false

    readonly property bool shown: visible
    readonly property bool simple: Documents.simple
    readonly property string lexer: root.doc.lexer
    readonly property string tier: root.doc.tier
    readonly property bool markdown: root.doc.isMarkdown
    readonly property bool prose: EditorMetrics.proseLexers.includes(lexer.toLowerCase())
    // "auto" leaves large text unwrapped: wrapped, every width change lays the whole document out again.
    property bool large: false
    readonly property int wrapLimit: 2 << 20
    readonly property bool autoWrap: SettingsData.wrapMode === "on" || (SettingsData.wrapMode === "auto" && prose && !large)
    // "" | "on" | "off": the document's own choice, over the setting and the size limit.
    readonly property string wrapOverride: root.doc.wrapOverride ?? ""
    readonly property bool wrapping: wrapOverride === "on" || (wrapOverride !== "off" && autoWrap)
    readonly property bool indentSpaces: root.doc.indentStyle ? root.doc.indentStyle === "space" : SettingsData.insertSpaces
    readonly property int indentWidth: root.doc.indentWidth > 0 ? root.doc.indentWidth : Math.max(1, SettingsData.tabWidth)
    readonly property string indentUnit: indentSpaces ? " ".repeat(indentWidth) : "\t"
    property real wrapWidth: 0
    readonly property int resizeDelay: 100
    readonly property string fontFamily: SettingsData.fontFamily || (SettingsData.useMonospace ? Style.monoFontFamily : Style.fontFamily)
    readonly property int fontSize: SettingsData.fontSize
    readonly property real padding: EditorMetrics.padding
    readonly property bool gutterShown: SettingsData.showLineNumbers && !simple
    readonly property real screenWidth: Math.max(1, Screen.width)

    readonly property bool previewOpen: markdown && (root.doc.previewMode === "split" || root.doc.previewMode === "full")
    readonly property bool previewOnly: previewOpen && root.doc.previewMode === "full"

    readonly property bool highlightRequested: SettingsData.highlightEnabled
    readonly property bool highlightOn: highlightRequested && root.doc.tier === "exact"
    readonly property bool spellOn: Spell.active
    readonly property FindBar findBar: findLoader.item as FindBar
    readonly property MarkdownPreview preview: previewLoader.item as MarkdownPreview
    readonly property bool findOpen: findBar?.opened ?? false
    readonly property int matchCount: findBar?.matchCount ?? 0
    // Formats address text blocks by line number; a line separator splits a mirror line inside one block.
    property bool hasSeparators: false
    readonly property bool decorated: (highlightOn || spellOn || matchCount > 0) && !hasSeparators

    // Role 0 keeps the editor's own color; the role after the syntax roles marks matching brackets.
    readonly property var rolePalette: Highlight.palette.map((entry, role) => role === 0 ? ({}) : entry).concat([
        {
            "color": Style.primary,
            "bold": true
        }
    ])
    readonly property int bracketRole: rolePalette.length - 1
    readonly property color matchColor: Contrast.mix(Style.surface, Style.primary, Style.primarySelected.a)

    readonly property bool lineHighlightOn: SettingsData.highlightCurrentLine
    property real lineHighlightY: 0
    property real lineHighlightHeight: 0

    // Slots are indexed by line % capacity so scrolling reuses delegates.
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
            "spellSent": "",
            "pairs": 0,
            "decorated": new Map(),
            "brackets": new Map(),
            "bracketLines": new Set()
        })
    readonly property var bracketMatcher: BracketMatch.create(
        (params, done) => Backend.call("doc.matchBracket", params, done),
        pair => root.applyBrackets(pair))

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

    // Trim unchanged stored text, not Qt's normalized mirror.
    function replaceRange(start, end, text) {
        const change = EditTracker.diff(decorations.text(start, end), text);
        if (!change)
            return;
        edit(start + change.start, change.removed, change.text);
    }

    function edit(start, removed, text) {
        decorations.replace(start, start + removed, text);
    }

    function editable() {
        return !textEdit.readOnly && !mirror.loading;
    }

    function selectionAnchor() {
        return textEdit.cursorPosition === textEdit.selectionStart ? textEdit.selectionEnd : textEdit.selectionStart;
    }

    function readStored(start, end) {
        return decorations.text(start, end);
    }

    function applyPlan(make) {
        if (!editable())
            return;
        const plan = make(mirror.index, root.readStored, selectionAnchor(), textEdit.cursorPosition);
        if (!plan)
            return;
        replaceRange(plan.start, plan.end, plan.text);
        textEdit.select(plan.anchor, plan.cursor);
    }

    function newline() {
        if (!editable())
            return;
        const index = mirror.index;
        const start = textEdit.selectionStart;
        const line = index.lineAt(start);
        const lineStart = index.startOf(line);
        const lead = Math.min(TextOps.leadingWhitespace(index.lineText(line)).length, start - lineStart);
        const text = "\n" + decorations.text(lineStart, lineStart + lead);
        replaceRange(start, textEdit.selectionEnd, text);
        textEdit.cursorPosition = start + text.length;
    }

    function indent() {
        applyPlan((index, read, anchor, cursor) => TextOps.reindentPlan(index, read, anchor, cursor, true, root.indentUnit, root.indentWidth));
    }

    function unindent() {
        applyPlan((index, read, anchor, cursor) => TextOps.reindentPlan(index, read, anchor, cursor, false, root.indentUnit, root.indentWidth));
    }

    function insertSpacesForTab() {
        const index = mirror.index;
        const column = textEdit.selectionStart - index.startOf(index.lineAt(textEdit.selectionStart));
        replaceSelection(" ".repeat(indentWidth - column % indentWidth));
    }

    function deleteLines() {
        applyPlan((index, read, anchor, cursor) => TextOps.deleteLinesPlan(index, anchor, cursor));
    }

    function duplicate() {
        applyPlan(TextOps.duplicatePlan);
    }

    function moveLinesUp() {
        applyPlan((index, read, anchor, cursor) => TextOps.moveLinesPlan(index, read, anchor, cursor, false));
    }

    function moveLinesDown() {
        applyPlan((index, read, anchor, cursor) => TextOps.moveLinesPlan(index, read, anchor, cursor, true));
    }

    function joinLines() {
        applyPlan(TextOps.joinLinesPlan);
    }

    function sortLines() {
        applyPlan(TextOps.sortLinesPlan);
    }

    function selectLine() {
        const range = TextOps.lineSelection(mirror.index, selectionAnchor(), textEdit.cursorPosition);
        textEdit.select(range[0], range[1]);
    }

    function upperCase() {
        changeCase("upper");
    }

    function lowerCase() {
        changeCase("lower");
    }

    function titleCase() {
        changeCase("title");
    }

    // Recases the selection and keeps it selected; the length can change ("ß" becomes "SS").
    function changeCase(kind) {
        const start = textEdit.selectionStart;
        const end = textEdit.selectionEnd;
        if (!editable() || end <= start)
            return;
        const text = decorations.text(start, end);
        let next = text;
        switch (kind) {
        case "upper":
            next = text.toUpperCase();
            break;
        case "lower":
            next = text.toLowerCase();
            break;
        case "title":
            next = TextOps.titleCase(text);
            break;
        }
        const reversed = textEdit.cursorPosition === start;
        replaceRange(start, end, next);
        if (reversed)
            textEdit.select(start + next.length, start);
        else
            textEdit.select(start, start + next.length);
    }

    // The stored text, so no-break spaces and line separators survive.
    function copyAll() {
        Quickshell.clipboardText = decorations.text(0, textEdit.length);
    }

    function toggleOverwrite() {
        textEdit.overwriteMode = !textEdit.overwriteMode;
    }

    function smartHome(extend) {
        const index = mirror.index;
        const line = index.lineAt(textEdit.cursorPosition);
        const lineStart = index.startOf(line);
        const target = lineStart + TextOps.smartHome(index.lineText(line), textEdit.cursorPosition - lineStart);
        if (extend)
            textEdit.moveCursorSelection(target, TextEdit.SelectCharacters);
        else
            textEdit.cursorPosition = target;
    }

    function toggleWrap() {
        if (root.doc.wrapOverride === undefined)
            return;
        const next = !wrapping;
        root.doc.wrapOverride = next === autoWrap ? "" : (next ? "on" : "off");
    }

    function findNext() {
        findStep(true);
    }

    function findPrevious() {
        findStep(false);
    }

    function findStep(forward) {
        if (findBar?.search(forward))
            return;
        openFind(false);
    }

    function wordCount() {
        return TextOps.countWords(mirror.index.lines);
    }

    function handleKey(event) {
        const mods = event.modifiers & (Qt.ControlModifier | Qt.ShiftModifier | Qt.AltModifier | Qt.MetaModifier);
        const plain = (mods & ~Qt.ShiftModifier) === 0;
        switch (event.key) {
        case Qt.Key_Tab:
        case Qt.Key_Backtab:
            event.accepted = tabPressed(event.key === Qt.Key_Backtab || mods === Qt.ShiftModifier, plain);
            return;
        case Qt.Key_Return:
        case Qt.Key_Enter:
            if (!plain || !editable() || textEdit.inputMethodComposing)
                return;
            newline();
            break;
        case Qt.Key_Home:
            if (!plain)
                return;
            smartHome(mods === Qt.ShiftModifier);
            break;
        case Qt.Key_Insert:
            if (mods !== 0)
                return;
            toggleOverwrite();
            break;
        case Qt.Key_D:
            if (mods === Qt.ControlModifier)
                deleteLines();
            else if (mods === (Qt.ControlModifier | Qt.AltModifier) || mods === (Qt.ControlModifier | Qt.ShiftModifier))
                duplicate();
            else
                return;
            break;
        case Qt.Key_Up:
        case Qt.Key_Down:
            if (mods !== Qt.AltModifier)
                return;
            if (event.key === Qt.Key_Down)
                moveLinesDown();
            else
                moveLinesUp();
            break;
        case Qt.Key_L:
            if (mods !== Qt.ControlModifier)
                return;
            selectLine();
            break;
        case Qt.Key_J:
            if (mods !== Qt.ControlModifier)
                return;
            joinLines();
            break;
        case Qt.Key_Z:
            if (mods === Qt.ControlModifier)
                undo();
            else if (mods === (Qt.ControlModifier | Qt.ShiftModifier))
                redo();
            else
                return;
            break;
        case Qt.Key_Y:
            if (mods !== Qt.ControlModifier)
                return;
            redo();
            break;
        case Qt.Key_Escape:
            if (!findOpen)
                return;
            closeFind();
            break;
        default:
            return;
        }
        event.accepted = true;
    }

    function tabPressed(back, plain) {
        if (!plain || !editable())
            return false;
        const index = mirror.index;
        const severalLines = index.lineAt(textEdit.selectionStart) !== index.lineAt(textEdit.selectionEnd);
        if (severalLines || back) {
            if (back)
                unindent();
            else
                indent();
            return true;
        }
        if (!indentSpaces)
            return false;
        insertSpacesForTab();
        return true;
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
        (findLoader.item as FindBar).open(withReplace === true, /[\n\u2029]/.test(selection) ? "" : selection);
    }

    function closeFind() {
        findBar?.close();
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
        redoing = true;
        textEdit.redo();
        redoing = false;
    }

    function selectAll() {
        textEdit.selectAll();
    }

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

    function respell() {
        mirror.cursorLine = -1;
        mirror.spellSent = "";
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
        large = text.length > wrapLimit;
        textEdit.text = text;
        // TextEdit reports NBSP as a space and line separators as "\n"; the lines must match what it reports.
        if (hasSeparators || text.indexOf("\u00a0") >= 0)
            mirror.index = LineIndex.create(textEdit.text);
        mirror.docId = root.doc.docId;
        mirror.sentFirst = -1;
        mirror.sentLast = -1;
        mirror.cursorLine = -1;
        mirror.spellSent = "";
        mirror.decorated.clear();
        mirror.brackets = new Map();
        mirror.bracketLines.clear();
        bracketMatcher.update(null);
        Highlight.forget(mirror.docId);
        Spell.forget(mirror.docId);
        mirror.pairs = surrogatePairs(text);
        root.doc.charCount = mirror.index.length - mirror.pairs;
        root.doc.lineCount = mirror.index.count;
        applyPendingPosition();
        mirror.loading = false;
        updateCursor();
        refreshView(true);
        preview?.documentEdited();
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
        const lines = applyChange({
            "start": position,
            "removed": removed,
            "text": decorations.text(position, position + added)
        }, textEdit.length);
        shiftDecorated(lines);
        bracketMatcher.update(null);
        Qt.callLater(root.updateCursor);
        // Unwrapped, an edit that keeps the line count moves no line, so the gutter stays as built.
        if (lines.oldLines === lines.newLines && !wrapping) {
            decorate();
            return;
        }
        refreshView(true);
        // Blocks added by this edit are not laid out yet, so their rows are measured again after it.
        Qt.callLater(root.remeasure);
    }

    function remeasure() {
        if (mirror.loading || mirror.builtLast < 0)
            return;
        layoutSlots();
    }

    function shiftDecorated(lines) {
        const delta = lines.newLines - lines.oldLines;
        if (delta === 0)
            return;
        const end = lines.startLine + lines.oldLines;
        const moved = new Map();
        for (const [line, entry] of mirror.decorated) {
            if (line < lines.startLine)
                moved.set(line, entry);
            else if (line >= end)
                moved.set(line + delta, entry);
        }
        mirror.decorated = moved;
        const bracketLines = new Set();
        for (const line of mirror.bracketLines)
            bracketLines.add(line < lines.startLine ? line : line >= end ? line + delta : lines.startLine);
        mirror.bracketLines = bracketLines;
    }

    function applyChange(change, length) {
        // The lines keep what TextEdit reports, so later reads compare like with like.
        const shown = change.text.replace(/\u00a0/g, " ").replace(/[\u2028\u2029\ufdd0\ufdd1]/g, "\n");
        const removedPairs = change.removed > 0 ? surrogatePairs(mirror.index.text(change.start, change.start + change.removed)) : 0;
        mirror.pairs += surrogatePairs(change.text) - removedPairs;
        const lines = mirror.index.applyEdit(change.start, change.removed, shown);
        const doc = root.doc;
        doc.edited(change.start, change.removed, change.text, length, decorations.undoSteps(), redoing);
        Highlight.linesEdited(mirror.docId, doc.version, lines);
        Spell.linesEdited(mirror.docId, doc.version, lines);
        if (shown !== change.text)
            hasSeparators = hasSeparators || /[\u2028\u2029\ufdd0\ufdd1]/.test(change.text);
        doc.charCount = length - mirror.pairs;
        doc.lineCount = mirror.index.count;
        findBar?.documentEdited(change);
        preview?.documentEdited();
        return lines;
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
        if (selected <= 0 || mirror.pairs === 0)
            doc.selectionLength = Math.max(0, selected);
        else
            doc.selectionLength = selected - surrogatePairs(mirror.index.text(textEdit.selectionStart, textEdit.selectionEnd));
        currentLine = line;
        mirror.cursorLine = line;
        // On every move, not only a new line: the core skips the word being typed by its column.
        sendSpellView();
        placeLineHighlight();
        updateBrackets();
    }

    function placeLineHighlight() {
        if (!lineHighlightOn || mirror.loading)
            return;
        const index = mirror.index;
        const line = Math.min(currentLine, index.count - 1);
        const top = textEdit.positionToRectangle(index.startOf(line));
        const bottom = wrapping ? textEdit.positionToRectangle(index.endOf(line)) : top;
        lineHighlightY = top.y;
        lineHighlightHeight = bottom.y + bottom.height - top.y;
    }

    function updateBrackets() {
        if (mirror.loading || !shown || hasSeparators || mirror.builtLast < 0 || !mirror.docId) {
            bracketMatcher.update(null);
            return;
        }
        const offset = textEdit.cursorPosition;
        const near = mirror.index.text(Math.max(0, offset - 1), Math.min(mirror.index.length, offset + 1));
        if (!/[()[\]{}]/.test(near)) {
            bracketMatcher.update(null);
            return;
        }
        bracketMatcher.update({
            "docId": mirror.docId,
            "version": root.doc.version,
            "offset": offset,
            "first": mirror.builtFirst,
            "last": mirror.builtLast
        }, lexer + ":" + tier);
    }

    function applyBrackets(pair) {
        const index = mirror.index;
        const next = new Map();
        for (const offset of pair ?? []) {
            const line = index.lineAt(offset);
            const spans = next.get(line) ?? [];
            spans.push(offset - index.startOf(line), 1, bracketRole);
            next.set(line, spans);
        }
        if (sameBrackets(mirror.brackets, next))
            return;
        mirror.brackets = next;
        if (mirror.builtLast < 0)
            return;
        const stores = formatStores();
        let changed = false;
        for (const line of bracketLinesToFormat()) {
            if (stores && (line < mirror.builtFirst || line > mirror.builtLast)) {
                mirror.decorated.delete(line);
                continue;
            }
            changed = decorateLine(line, stores) || changed;
        }
        if (changed && wrapping)
            layoutSlots();
    }

    function sameBrackets(a, b) {
        if (a.size !== b.size)
            return false;
        for (const [line, spans] of b) {
            const other = a.get(line);
            if (!other || other.length !== spans.length || other.some((value, i) => value !== spans[i]))
                return false;
        }
        return true;
    }

    function bracketLinesToFormat() {
        const lines = new Set(mirror.bracketLines);
        for (const line of mirror.brackets.keys())
            lines.add(line);
        return lines;
    }

    function sendSpellView() {
        if (!spellOn || mirror.sentFirst < 0)
            return;
        const line = mirror.cursorLine;
        const column = line >= 0 ? textEdit.cursorPosition - mirror.index.startOf(line) : -1;
        const sent = mirror.sentFirst + ":" + mirror.sentLast + ":" + line + ":" + column;
        if (sent === mirror.spellSent)
            return;
        mirror.spellSent = sent;
        Spell.view(root.doc, mirror.sentFirst, mirror.sentLast, line, column);
    }

    // Find the line above y without positionAt's document-wide block walk.
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
        updateBrackets();
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
        placeLineHighlight();
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
        mirror.spellSent = "";
        refreshView(true);
    }

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
            mirror.bracketLines.clear();
        }
        redecorate();
    }

    function formatStores() {
        if (!decorated)
            return null;
        return {
            "spans": highlightOn ? Highlight.spans(mirror.docId) : null,
            "spell": spellOn ? Spell.ranges(mirror.docId) : null,
            "matches": matchCount > 0 ? findBar.matches : null
        };
    }

    function decorate() {
        if (mirror.builtLast < 0)
            return false;
        const stores = formatStores();
        let changed = false;
        if (!stores) {
            if (mirror.bracketLines.size === 0 && mirror.brackets.size === 0)
                return false;
            for (const line of bracketLinesToFormat())
                changed = decorateLine(line, null) || changed;
            return changed;
        }
        for (let line = mirror.builtFirst; line <= mirror.builtLast; line++)
            changed = decorateLine(line, stores) || changed;
        return changed;
    }

    function decorateLine(line, stores) {
        const index = mirror.index;
        const text = index.lineText(line);
        const spans = stores?.spans?.get(line) ?? null;
        const spell = stores?.spell?.get(line) ?? null;
        const found = stores?.matches ? lineMatches(stores.matches, index.startOf(line), index.endOf(line)) : null;
        const key = found ? found.join(",") : "";
        const bracket = mirror.brackets.get(line) ?? null;
        const cached = mirror.decorated.get(line);
        if (cached && cached.text === text && cached.spans === spans && cached.spell === spell && cached.key === key && cached.bracket === bracket)
            return false;
        mirror.decorated.set(line, {
            "text": text,
            "spans": spans,
            "spell": spell,
            "key": key,
            "bracket": bracket
        });
        if (bracket)
            mirror.bracketLines.add(line);
        else
            mirror.bracketLines.delete(line);
        return decorations.setLine(line, bracket ? (spans ?? []).concat(bracket) : spans ?? [], spell ?? [], found ?? []);
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

    function followWidth() {
        if (wrapWidth === flickable.width)
            return;
        wrapWidth = flickable.width;
        refreshView(true);
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
        bracketMatcher.update(null);
    }
    onLexerChanged: updateBrackets()
    onTierChanged: updateBrackets()
    onDecoratedChanged: restyle()
    onHighlightOnChanged: restyle()
    onSpellOnChanged: resendViews()
    onHighlightRequestedChanged: resendViews()
    onWrappingChanged: refreshView(true)
    onLineHighlightOnChanged: placeLineHighlight()
    onFontFamilyChanged: refreshView(true)
    onFontSizeChanged: refreshView(true)
    onPaddingChanged: refreshView(true)

    Component.onCompleted: root.doc.editor = root
    Component.onDestruction: {
        bracketMatcher.stop();
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

    Timer {
        id: resizeTimer
        interval: root.resizeDelay
        onTriggered: root.followWidth()
    }

    FontMetrics {
        id: fontMetrics
        font.family: root.fontFamily
        font.pixelSize: root.fontSize
    }

    TextDecorations {
        id: decorations

        document: textEdit.textDocument
        palette: root.rolePalette
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
                if (root.wrapping && root.wrapWidth > 0) {
                    resizeTimer.restart();
                    return;
                }
                root.wrapWidth = width;
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

                // Grow unwrapped text a screen at a time to avoid relayout on every width change.
                width: root.wrapping ? root.wrapWidth : Math.ceil((contentWidth + leftPadding + rightPadding + 1) / root.screenWidth) * root.screenWidth
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
                tabStopDistance: Math.max(1, root.indentWidth * fontMetrics.advanceWidth(" "))

                cursorDelegate: DTextCursor {
                    readonly property int editCursorPosition: textEdit.cursorPosition

                    color: Style.primary
                    x: textEdit.cursorRectangle.x
                    y: textEdit.cursorRectangle.y
                    height: textEdit.cursorRectangle.height
                    shown: textEdit.cursorVisible

                    onEditCursorPositionChanged: resetBlink()
                }

                // Qt emits all three on most keystrokes; one update reads them after the edit is mirrored.
                onCursorPositionChanged: Qt.callLater(root.updateCursor)
                onSelectionStartChanged: Qt.callLater(root.updateCursor)
                onSelectionEndChanged: Qt.callLater(root.updateCursor)
                onCursorRectangleChanged: {
                    if (!root.mirror.loading && activeFocus)
                        root.revealRect(cursorRectangle);
                }
                onTabStopDistanceChanged: root.refreshView(true)
                onImplicitHeightChanged: root.refreshView(false)

                Keys.onPressed: event => root.handleKey(event)

                // Under the text: a negative z draws below the parent's own content.
                Rectangle {
                    z: -1
                    x: -textEdit.x
                    y: root.lineHighlightY
                    width: Math.max(textEdit.width, flickable.width)
                    height: root.lineHighlightHeight
                    visible: root.lineHighlightOn && root.doc.loaded
                    color: Style.surfaceTextHover
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
