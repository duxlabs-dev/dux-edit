pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import qs.Common
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Services

// Find and replace for one pane. Matching runs in the core on the mirror; this keeps the flat
// [start, length, ...] ranges of the last answer, shifted through edits until the next one arrives.
Rectangle {
    id: root

    required property var doc
    required property Item editor

    property bool opened: false
    property bool replaceVisible: false
    property var matches: []
    readonly property int matchCount: matches.length / 2
    property int total: 0
    property bool capped: false
    property int current: -1
    property string error: ""
    // Set by typing in the query: the next answer selects the match nearest the cursor.
    property bool jumpOnResult: false
    // Find next (1) or previous (-1) asked for before the matches arrived.
    property int pendingStep: 0
    property int requestSeq: 0

    readonly property var options: ({
            "regex": regexButton.checked,
            "caseSensitive": caseButton.checked,
            "wholeWord": wordButton.checked
        })
    readonly property string counterText: {
        if (findField.text === "" || error !== "")
            return "";
        if (total === 0)
            return I18n.tr("No matches", "find bar, the query matches nothing");
        return I18n.tr("%1/%2", "find bar counter, current match and total").arg(current >= 0 ? current + 1 : 0).arg(capped ? total + "+" : total);
    }

    signal closeRequested
    signal matchesUpdated

    function open(withReplace, prefill) {
        opened = true;
        replaceVisible = withReplace;
        if (prefill !== "")
            findField.text = prefill;
        findField.forceActiveFocus();
        findField.selectAll();
        jumpOnResult = false;
        query();
    }

    // Find next or previous from the editor, keeping the focus there. False without a query.
    function search(forward) {
        if (findField.text === "")
            return false;
        if (opened) {
            step(forward);
            return true;
        }
        opened = true;
        pendingStep = forward ? 1 : -1;
        query();
        return true;
    }

    function close() {
        opened = false;
        pendingStep = 0;
        requestSeq++;
        queryTimer.stop();
        matches = [];
        total = 0;
        current = -1;
        matchesUpdated();
    }

    function query() {
        queryTimer.stop();
        const seq = ++requestSeq;
        const text = findField.text;
        if (text === "") {
            error = "";
            setMatches([], 0, false);
            return;
        }
        const version = doc.version;
        Search.find(doc, text, options, result => {
            if (seq !== root.requestSeq)
                return;
            if (result.ok !== true) {
                root.error = result.code === "BADREGEX" ? (result.message || I18n.tr("Invalid regular expression", "find bar error")) : "";
                root.setMatches([], 0, false);
                return;
            }
            if (root.doc.version !== version) {
                queryTimer.restart();
                return;
            }
            root.error = "";
            root.setMatches(result.ranges || [], result.total || 0, result.capped === true);
        });
    }

    function setMatches(ranges, count, isCapped) {
        matches = ranges;
        total = count;
        capped = isCapped;
        const jump = jumpOnResult;
        const pending = pendingStep;
        jumpOnResult = false;
        pendingStep = 0;
        current = matchAtSelection();
        if (pending !== 0 && matchCount > 0)
            step(pending > 0);
        else if (jump && current < 0 && matchCount > 0)
            select(firstFrom(editor.cursorPosition) % matchCount);
        matchesUpdated();
    }

    function matchAtSelection() {
        const i = firstFrom(editor.selectionStart);
        if (i >= matchCount || matches[i * 2] !== editor.selectionStart || matches[i * 2] + matches[i * 2 + 1] !== editor.selectionEnd)
            return -1;
        return i;
    }

    function firstFrom(offset) {
        let low = 0;
        let high = matchCount;
        while (low < high) {
            const mid = (low + high) >> 1;
            if (matches[mid * 2] < offset)
                low = mid + 1;
            else
                high = mid;
        }
        return low;
    }

    function select(i) {
        current = i;
        editor.selectRange(matches[i * 2], matches[i * 2] + matches[i * 2 + 1]);
    }

    function step(forward) {
        if (matchCount === 0)
            return;
        const hasSelection = editor.selectionEnd > editor.selectionStart;
        if (forward) {
            const i = firstFrom(hasSelection ? editor.selectionStart + 1 : editor.cursorPosition);
            select(i < matchCount ? i : 0);
            return;
        }
        const i = firstFrom(hasSelection ? editor.selectionStart : editor.cursorPosition) - 1;
        select(i >= 0 ? i : matchCount - 1);
    }

    // Keeps the drawn matches on their text until the core answers again.
    function documentEdited(change) {
        if (!opened || findField.text === "")
            return;
        const end = change.start + change.removed;
        const delta = change.text.length - change.removed;
        const shifted = matches;
        let kept = 0;
        for (let i = 0; i + 1 < shifted.length; i += 2) {
            const start = shifted[i];
            const length = shifted[i + 1];
            if (start + length > change.start && start < end)
                continue;
            shifted[kept++] = start + length <= change.start ? start : start + delta;
            shifted[kept++] = length;
        }
        if (kept < shifted.length)
            matches = shifted.slice(0, kept);
        current = -1;
        queryTimer.restart();
    }

    function replaceCurrent() {
        const i = matchAtSelection();
        if (i < 0) {
            step(true);
            return;
        }
        const start = matches[i * 2];
        const length = matches[i * 2 + 1];
        const replacement = replaceField.text;
        if (!options.regex) {
            replaceAt(start, length, replacement);
            return;
        }
        const target = doc;
        const version = target.version;
        Search.expand(target, findField.text, options, replacement, start, length, result => {
            if (result.ok !== true || root.doc !== target || target.version !== version)
                return;
            root.replaceAt(start, length, result.text);
        });
    }

    function replaceAt(start, length, text) {
        editor.replaceRange(start, start + length, text);
        const next = firstFrom(start + text.length);
        if (matchCount > 0)
            select(next < matchCount ? next : 0);
    }

    function replaceAll() {
        if (findField.text === "")
            return;
        const target = doc;
        const version = target.version;
        Search.replaceAll(target, findField.text, options, replaceField.text, result => {
            if (result.ok !== true || result.count === 0 || root.doc !== target || target.version !== version)
                return;
            root.editor.replaceRange(result.start, result.end, result.text);
            ToastService.showInfo(I18n.tr("Replaced %1 matches", "find bar, after replace all").arg(result.count));
        });
    }

    implicitHeight: content.implicitHeight + Style.spacingS * 2
    color: Style.cardSurface
    visible: opened

    onOptionsChanged: {
        if (opened)
            query();
    }

    Keys.onPressed: event => {
        switch (event.key) {
        case Qt.Key_Escape:
            root.closeRequested();
            event.accepted = true;
            return;
        case Qt.Key_Return:
        case Qt.Key_Enter:
            if (replaceField.activeFocus)
                root.replaceCurrent();
            else
                root.step(!(event.modifiers & Qt.ShiftModifier));
            event.accepted = true;
            return;
        }
    }

    Timer {
        id: queryTimer
        interval: EditorMetrics.findDelay
        onTriggered: root.query()
    }

    ColumnLayout {
        id: content

        anchors.fill: parent
        anchors.margins: Style.spacingS
        spacing: Style.spacingS

        RowLayout {
            Layout.fillWidth: true
            spacing: Style.spacingXS

            DTextField {
                id: findField

                Layout.fillWidth: true
                placeholderText: I18n.tr("Find", "find bar placeholder")
                isError: root.error !== ""
                onTextEdited: {
                    root.jumpOnResult = true;
                    queryTimer.restart();
                }
            }

            StyledText {
                text: root.counterText
                color: Style.onSurfaceVariant
                font.pixelSize: Style.fontSizeSmall
                wrapMode: Text.NoWrap
            }

            DIconButton {
                id: caseButton
                iconName: "match_case"
                checkable: true
                tooltipText: I18n.tr("Match case", "find bar toggle")
                onClicked: checked = !checked
            }

            DIconButton {
                id: wordButton
                iconName: "match_word"
                checkable: true
                tooltipText: I18n.tr("Whole word", "find bar toggle")
                onClicked: checked = !checked
            }

            DIconButton {
                id: regexButton
                iconName: "regular_expression"
                checkable: true
                tooltipText: I18n.tr("Regular expression", "find bar toggle")
                onClicked: checked = !checked
            }

            DIconButton {
                iconName: "keyboard_arrow_up"
                enabled: root.matchCount > 0
                tooltipText: I18n.tr("Previous match", "find bar button")
                onClicked: root.step(false)
            }

            DIconButton {
                iconName: "keyboard_arrow_down"
                enabled: root.matchCount > 0
                tooltipText: I18n.tr("Next match", "find bar button")
                onClicked: root.step(true)
            }

            DIconButton {
                iconName: "find_replace"
                checkable: true
                checked: root.replaceVisible
                enabled: !root.doc.readOnly
                tooltipText: I18n.tr("Replace", "find bar button")
                onClicked: root.replaceVisible = !root.replaceVisible
            }

            DIconButton {
                iconName: "close"
                tooltipText: I18n.tr("Close", "find bar button")
                onClicked: root.closeRequested()
            }
        }

        StyledText {
            Layout.fillWidth: true
            visible: root.error !== ""
            text: root.error
            color: Style.error
            font.pixelSize: Style.fontSizeSmall
        }

        RowLayout {
            Layout.fillWidth: true
            visible: root.replaceVisible && !root.doc.readOnly
            spacing: Style.spacingS

            DTextField {
                id: replaceField

                Layout.fillWidth: true
                placeholderText: I18n.tr("Replace with", "find bar replacement placeholder")
            }

            DButton {
                text: I18n.tr("Replace", "find bar button")
                variant: "tonal"
                enabled: root.matchCount > 0
                onClicked: root.replaceCurrent()
            }

            DButton {
                text: I18n.tr("Replace All", "find bar button")
                variant: "tonal"
                enabled: root.matchCount > 0
                onClicked: root.replaceAll()
            }
        }
    }
}
