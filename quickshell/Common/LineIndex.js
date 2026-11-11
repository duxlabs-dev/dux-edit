.pragma library

// The pane's copy of the text as lines (UTF-16, without "\n") plus their start offsets. Entries of `starts`
// from `shiftFrom` on are stored without `shift`, so typing in one place costs nothing for the lines after
// it; the shift is written out only between edit sites.
class LineIndex {
    constructor() {
        this.lines = [""];
        this.starts = new Int32Array(64);
        this.count = 1;
        this.length = 0;
        this.shiftFrom = 1;
        this.shift = 0;
    }

    build(text) {
        this.lines = text.split("\n");
        const count = this.lines.length;
        this.starts = new Int32Array(Math.max(64, count + (count >> 2)));
        let at = 0;
        for (let line = 0; line < count; line++) {
            this.starts[line] = at;
            at += this.lines[line].length + 1;
        }
        this.count = count;
        this.length = text.length;
        this.shiftFrom = count;
        this.shift = 0;
    }

    startOf(line) {
        if (line <= 0)
            return 0;
        if (line >= this.count)
            line = this.count - 1;
        return line >= this.shiftFrom ? this.starts[line] + this.shift : this.starts[line];
    }

    // Offset of the "\n" that ends `line`, or the text length for the last line.
    endOf(line) {
        const clamped = Math.max(0, Math.min(line, this.count - 1));
        return this.startOf(clamped) + this.lines[clamped].length;
    }

    lineText(line) {
        return this.lines[line] ?? "";
    }

    // The 0-based line holding `offset`; an offset right after "\n" belongs to the next line.
    lineAt(offset) {
        let low = 0;
        let high = this.count - 1;
        while (low < high) {
            const mid = (low + high + 1) >> 1;
            if (this.startOf(mid) <= offset)
                low = mid;
            else
                high = mid - 1;
        }
        return low;
    }

    // Text in [start, end), read from the lines.
    text(start, end) {
        start = Math.max(0, Math.min(start, this.length));
        end = Math.max(start, Math.min(end, this.length));
        const first = this.lineAt(start);
        const last = this.lineAt(end);
        const from = start - this.startOf(first);
        if (first === last)
            return this.lines[first].substring(from, end - this.startOf(last));
        const parts = [this.lines[first].substring(from)];
        for (let line = first + 1; line < last; line++)
            parts.push(this.lines[line]);
        parts.push(this.lines[last].substring(0, end - this.startOf(last)));
        return parts.join("\n");
    }

    fullText() {
        return this.lines.join("\n");
    }

    addShift(from, to, delta) {
        const starts = this.starts;
        for (let i = from; i < to; i++)
            starts[i] += delta;
    }

    // `removed` units at `start` became `inserted`. Returns {startLine, oldLines, newLines}: lines
    // [startLine, startLine + oldLines) were replaced by newLines lines, like the core's buffer.Change.
    applyEdit(start, removed, inserted) {
        const startLine = this.lineAt(start);
        const endLine = removed > 0 ? this.lineAt(start + removed) : startLine;
        const tail = endLine + 1;
        const lineStart = this.startOf(startLine);

        const head = this.lines[startLine].substring(0, start - lineStart);
        const rest = this.lines[endLine].substring(start + removed - this.startOf(endLine));
        const joined = head + inserted + rest;
        const fresh = inserted.indexOf("\n") < 0 ? [joined] : joined.split("\n");
        this.lines.splice(startLine, tail - startLine, ...fresh);
        this.length += inserted.length - removed;

        if (this.shiftFrom < tail) {
            this.addShift(this.shiftFrom, Math.min(tail, this.count), this.shift);
            this.shiftFrom = tail;
        } else if (this.shiftFrom > tail) {
            this.addShift(tail, this.shiftFrom, -this.shift);
            this.shiftFrom = tail;
        }
        this.shift += inserted.length - removed;

        const moved = fresh.length - (tail - startLine);
        if (moved !== 0) {
            const count = this.count + moved;
            if (count > this.starts.length) {
                const grown = new Int32Array(count + (count >> 1));
                grown.set(this.starts.subarray(0, this.count));
                this.starts = grown;
            }
            this.starts.copyWithin(tail + moved, tail, this.count);
            this.count = count;
            this.shiftFrom += moved;
        }
        // Fresh starts are absolute, so they sit before shiftFrom.
        let at = lineStart;
        for (let i = 1; i < fresh.length; i++) {
            at += fresh[i - 1].length + 1;
            this.starts[startLine + i] = at;
        }
        return {
            "startLine": startLine,
            "oldLines": tail - startLine,
            "newLines": fresh.length
        };
    }
}

const MAX_TRACKED_EDITS = 512;

// Per-line values (spans, misspellings) that follow the pane's edits. Values computed by the core at an
// older version are moved through the edits made since, and dropped for lines those edits touched.
class LineMap {
    constructor() {
        this.lines = new Map();
        this.edits = [];
        this.floor = 0;
    }

    clear() {
        this.lines.clear();
        this.edits = [];
        this.floor = 0;
    }

    get(line) {
        return this.lines.get(line);
    }

    // Lines [startLine, startLine + oldLines) became newLines lines at `version`.
    edit(version, startLine, oldLines, newLines) {
        // Only a merge trims the list, and a document the core sends nothing for never merges. Past the
        // cap, values older than this edit are refused instead of tracked.
        if (this.edits.length >= MAX_TRACKED_EDITS) {
            this.edits = [];
            this.floor = version;
        } else {
            this.edits.push({
                "version": version,
                "startLine": startLine,
                "oldLines": oldLines,
                "newLines": newLines
            });
        }
        const end = startLine + oldLines;
        const delta = newLines - oldLines;
        const moved = new Map();
        for (const [line, value] of this.lines) {
            if (line < startLine)
                moved.set(line, value);
            else if (line >= end)
                moved.set(line + delta, value);
        }
        this.lines = moved;
    }

    // Values for lines first, first + 1, ... as the core saw them at `version`.
    merge(version, first, values) {
        if (version < this.floor)
            return;
        const later = this.edits.filter(edit => edit.version > version);
        this.edits = later;
        for (let i = 0; i < values.length; i++) {
            let line = first + i;
            for (const edit of later) {
                if (line < edit.startLine)
                    continue;
                if (line < edit.startLine + edit.oldLines) {
                    line = -1;
                    break;
                }
                line += edit.newLines - edit.oldLines;
            }
            if (line >= 0)
                this.lines.set(line, values[i]);
        }
    }

    // Keeps [first, last]; the core resends lines that come back into view.
    keep(first, last) {
        for (const line of this.lines.keys()) {
            if (line < first || line > last)
                this.lines.delete(line);
        }
    }
}

function createMap() {
    return new LineMap();
}

function create(text) {
    const index = new LineIndex();
    index.build(text || "");
    return index;
}
