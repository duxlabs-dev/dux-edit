.pragma library

// Pure text logic behind the editor's line and selection commands. Lines are strings without "\n".

const OPEN = "([{";
const CLOSE = ")]}";

function leadingWhitespace(text) {
    return /^[ \t]*/.exec(text)[0];
}

// Where Home goes: the first non-blank column, or the line start when already there.
function smartHome(text, column) {
    const first = leadingWhitespace(text).length;
    return column === first ? 0 : first;
}

// Adds `unit` to every non-empty line. Returns {lines, removed, added, changed}.
function indentLines(lines, unit) {
    const out = new Array(lines.length);
    const removed = new Array(lines.length).fill(0);
    const added = new Array(lines.length).fill(0);
    let changed = false;
    for (let i = 0; i < lines.length; i++) {
        if (lines[i] === "") {
            out[i] = "";
            continue;
        }
        out[i] = unit + lines[i];
        added[i] = unit.length;
        changed = true;
    }
    return {
        "lines": out,
        "removed": removed,
        "added": added,
        "changed": changed
    };
}

// Removes one level from every line: a leading tab, or up to `width` leading spaces.
function unindentLines(lines, width) {
    const out = new Array(lines.length);
    const removed = new Array(lines.length).fill(0);
    const added = new Array(lines.length).fill(0);
    let changed = false;
    for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        let cut = 0;
        if (line.charCodeAt(0) === 9)
            cut = 1;
        else
            while (cut < width && line.charCodeAt(cut) === 32)
                cut++;
        out[i] = cut > 0 ? line.substring(cut) : line;
        removed[i] = cut;
        changed = changed || cut > 0;
    }
    return {
        "lines": out,
        "removed": removed,
        "added": added,
        "changed": changed
    };
}

// Maps a column after `removed` units at the line start were replaced by `added` units. Column 0 stays put.
function mapColumn(column, removed, added) {
    if (column === 0)
        return 0;
    return Math.max(0, column - removed) + added;
}

function joinLines(lines) {
    const last = lines.length - 1;
    const parts = [];
    for (let i = 0; i <= last; i++) {
        let text = lines[i];
        if (i > 0)
            text = text.replace(/^[ \t]+/, "");
        if (i < last)
            text = text.replace(/[ \t]+$/, "");
        if (text !== "" || i === 0)
            parts.push(text);
    }
    if (parts.length > 1 && parts[0] === "")
        parts.shift();
    return parts.join(" ");
}

function sortLines(lines) {
    return lines.slice().sort((a, b) => a.localeCompare(b));
}

// Apostrophes stay inside a word.
const SEPARATORS = "\\s\\-_.,;:!?()\\[\\]{}\"/\\\\<>|+*=&%$#@~`^";
const WORD_START = new RegExp("(^|[" + SEPARATORS + "])([^" + SEPARATORS + "])", "g");

function titleCase(text) {
    return text.toLowerCase().replace(WORD_START, (match, before, first) => before + first.toUpperCase());
}

// Line commands are planned on a LineIndex; read(start, end) gives the stored text. A plan is
// {start, end, text, anchor, cursor}: replace [start, end), then select anchor to cursor. Null if nothing to do.

// [first, last] lines under the selection; one ending at a line start leaves that line out.
function selectedLines(index, anchor, cursor) {
    const start = Math.min(anchor, cursor);
    const end = Math.max(anchor, cursor);
    const first = index.lineAt(start);
    const last = index.lineAt(end);
    return [first, last > first && end === index.startOf(last) ? last - 1 : last];
}

// The stored text of lines [first, last], or null when a line separator splits them differently.
function readLines(index, read, first, last) {
    const lines = read(index.startOf(first), index.endOf(last)).split("\n");
    return lines.length === last - first + 1 ? lines : null;
}

function reindentPlan(index, read, anchor, cursor, adding, unit, width) {
    const [first, last] = selectedLines(index, anchor, cursor);
    const lines = readLines(index, read, first, last);
    if (!lines)
        return null;
    const result = adding ? indentLines(lines, unit) : unindentLines(lines, width);
    if (!result.changed)
        return null;
    const start = index.startOf(first);
    const end = index.endOf(last);
    const text = result.lines.join("\n");
    const lastStart = start + text.length - result.lines[result.lines.length - 1].length;
    const place = offset => {
        if (offset > end)
            return offset + text.length - (end - start);
        const line = index.lineAt(offset);
        const i = line - first;
        return (i === 0 ? start : lastStart) + mapColumn(offset - index.startOf(line), result.removed[i], result.added[i]);
    };
    return plan(start, end, text, place(anchor), place(cursor));
}

function deleteLinesPlan(index, anchor, cursor) {
    const [first, last] = selectedLines(index, anchor, cursor);
    const column = cursor - index.startOf(index.lineAt(cursor));
    if (last < index.count - 1) {
        const at = index.startOf(first) + Math.min(column, index.lineText(last + 1).length);
        return plan(index.startOf(first), index.startOf(last + 1), "", at, at);
    }
    if (first > 0) {
        const at = index.startOf(first - 1) + Math.min(column, index.lineText(first - 1).length);
        return plan(index.endOf(first - 1), index.endOf(last), "", at, at);
    }
    return index.length > 0 ? plan(0, index.length, "", 0, 0) : null;
}

function duplicatePlan(index, read, anchor, cursor) {
    const start = Math.min(anchor, cursor);
    const end = Math.max(anchor, cursor);
    if (end > start)
        return plan(end, end, read(start, end), anchor, cursor);
    const line = index.lineAt(cursor);
    const lineEnd = index.endOf(line);
    const at = lineEnd + 1 + cursor - index.startOf(line);
    return plan(lineEnd, lineEnd, "\n" + read(index.startOf(line), lineEnd), at, at);
}

function moveLinesPlan(index, read, anchor, cursor, down) {
    const [first, last] = selectedLines(index, anchor, cursor);
    if (down ? last >= index.count - 1 : first === 0)
        return null;
    const from = down ? first : first - 1;
    const to = down ? last + 1 : last;
    const lines = readLines(index, read, from, to);
    if (!lines)
        return null;
    const other = down ? lines.pop() : lines.shift();
    const text = down ? [other].concat(lines).join("\n") : lines.concat([other]).join("\n");
    const shift = down ? other.length + 1 : -(other.length + 1);
    return plan(index.startOf(from), index.endOf(to), text, anchor + shift, cursor + shift);
}

function joinLinesPlan(index, read, anchor, cursor) {
    const [first, selectedLast] = selectedLines(index, anchor, cursor);
    const last = selectedLast > first ? selectedLast : first + 1;
    if (last >= index.count)
        return null;
    const lines = readLines(index, read, first, last);
    if (!lines)
        return null;
    const text = joinLines(lines);
    const start = index.startOf(first);
    const at = start + (selectedLast > first ? text.length : Math.min(text.length, lines[0].replace(/[ \t]+$/, "").length));
    return plan(start, index.endOf(last), text, at, at);
}

function sortLinesPlan(index, read, anchor, cursor) {
    const [first, last] = selectedLines(index, anchor, cursor);
    if (last <= first)
        return null;
    const lines = readLines(index, read, first, last);
    if (!lines)
        return null;
    const start = index.startOf(first);
    const end = index.endOf(last);
    return plan(start, end, sortLines(lines).join("\n"), start, end);
}

// Whole lines under the selection with their line break, as [start, end]; one more line when already whole.
function lineSelection(index, anchor, cursor) {
    const start = Math.min(anchor, cursor);
    const end = Math.max(anchor, cursor);
    const [first, selectedLast] = selectedLines(index, anchor, cursor);
    const lineEnd = line => line + 1 < index.count ? index.startOf(line + 1) : index.endOf(line);
    const whole = end > start && start === index.startOf(first) && end === lineEnd(selectedLast);
    const last = whole ? Math.min(selectedLast + 1, index.count - 1) : selectedLast;
    return [index.startOf(first), lineEnd(last)];
}

function plan(start, end, text, anchor, cursor) {
    return {
        "start": start,
        "end": end,
        "text": text,
        "anchor": anchor,
        "cursor": cursor
    };
}

function countWords(lines) {
    let words = 0;
    for (let i = 0; i < lines.length; i++) {
        const found = lines[i].match(/\S+/g);
        if (found)
            words += found.length;
    }
    return words;
}

// The bracket next to `offset` (before it first) and its partner, as [offset, offset], or null.
// Reads at most `budget` units within [firstLine, lastLine].
function matchBracket(index, offset, firstLine, lastLine, budget) {
    const line = index.lineAt(offset);
    if (line < firstLine || line > lastLine)
        return null;
    const text = index.lineText(line);
    const column = offset - index.startOf(line);
    for (const at of [column - 1, column]) {
        if (at < 0 || at >= text.length)
            continue;
        const ch = text[at];
        const open = OPEN.indexOf(ch);
        const close = CLOSE.indexOf(ch);
        if (open < 0 && close < 0)
            continue;
        const partner = open >= 0 ? scanForward(index, line, at, ch.charCodeAt(0), CLOSE.charCodeAt(open), lastLine, budget) : scanBackward(index, line, at, OPEN.charCodeAt(close), ch.charCodeAt(0), firstLine, budget);
        return partner < 0 ? null : [index.startOf(line) + at, partner];
    }
    return null;
}

function scanForward(index, line, column, open, close, lastLine, budget) {
    let depth = 0;
    for (let l = line; l <= lastLine; l++) {
        const text = index.lineText(l);
        for (let i = l === line ? column : 0; i < text.length; i++) {
            if (--budget < 0)
                return -1;
            const code = text.charCodeAt(i);
            if (code === open)
                depth++;
            else if (code === close && --depth === 0)
                return index.startOf(l) + i;
        }
        if (--budget < 0)
            return -1;
    }
    return -1;
}

function scanBackward(index, line, column, open, close, firstLine, budget) {
    let depth = 0;
    for (let l = line; l >= firstLine; l--) {
        const text = index.lineText(l);
        for (let i = l === line ? column : text.length - 1; i >= 0; i--) {
            if (--budget < 0)
                return -1;
            const code = text.charCodeAt(i);
            if (code === close)
                depth++;
            else if (code === open && --depth === 0)
                return index.startOf(l) + i;
        }
        if (--budget < 0)
            return -1;
    }
    return -1;
}
