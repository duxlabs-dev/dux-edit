.pragma library

const SPLICE_LIMIT = 10000;

// The pane's copy of the text as lines (UTF-16, without "\n") plus start offsets. `starts` from `shiftFrom`
// on omit `shift`, so typing in one place stays cheap; it is written out only between edit sites.
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

    // Returns the edit as a line change, like the core's buffer.Change: {startLine, oldLines, newLines, column, removed, inserted}.
    applyEdit(start, removed, inserted) {
        const startLine = this.lineAt(start);
        const endLine = removed > 0 ? this.lineAt(start + removed) : startLine;
        const tail = endLine + 1;
        const lineStart = this.startOf(startLine);

        const head = this.lines[startLine].substring(0, start - lineStart);
        const rest = this.lines[endLine].substring(start + removed - this.startOf(endLine));
        const joined = head + inserted + rest;
        const fresh = inserted.indexOf("\n") < 0 ? [joined] : joined.split("\n");
        // Spreading a large paste into splice's arguments overflows the JS stack.
        if (fresh.length > SPLICE_LIMIT)
            this.lines = this.lines.slice(0, startLine).concat(fresh, this.lines.slice(tail));
        else
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
        let at = lineStart;
        for (let i = 1; i < fresh.length; i++) {
            at += fresh[i - 1].length + 1;
            this.starts[startLine + i] = at;
        }
        return {
            "startLine": startLine,
            "oldLines": tail - startLine,
            "newLines": fresh.length,
            "column": start - lineStart,
            "removed": removed,
            "inserted": inserted.length
        };
    }
}

const MAX_TRACKED_EDITS = 512;

// Per-line values (spans, misspellings) that follow the pane's edits; stale ones from the core are moved
// through later edits or dropped. With a stride, values are flat [col, len, ...] runs.
class LineMap {
    constructor(stride) {
        this.lines = new Map();
        this.edits = [];
        this.floor = 0;
        this.stride = stride || 0;
    }

    clear() {
        this.lines.clear();
        this.edits = [];
        this.floor = 0;
    }

    get(line) {
        return this.lines.get(line);
    }

    edit(version, startLine, oldLines, newLines, column, removed, inserted) {
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
        if (delta !== 0) {
            const moved = new Map();
            for (const [line, value] of this.lines) {
                if (line < startLine)
                    moved.set(line, value);
                else if (line >= end)
                    moved.set(line + delta, value);
            }
            this.lines = moved;
            return;
        }
        const kept = oldLines === 1 ? this.shifted(this.lines.get(startLine), column, removed, inserted) : undefined;
        for (const line of this.lines.keys()) {
            if (line >= startLine && line < end)
                this.lines.delete(line);
        }
        if (kept !== undefined)
            this.lines.set(startLine, kept);
    }

    shifted(value, column, removed, inserted) {
        const stride = this.stride;
        if (value === undefined || stride < 2 || column === undefined)
            return undefined;
        const end = column + removed;
        const delta = inserted - removed;
        const out = [];
        for (let i = 0; i + stride <= value.length; i += stride) {
            const start = value[i];
            if (start + value[i + 1] <= column)
                out.push(start);
            else if (start >= end)
                out.push(start + delta);
            else
                continue;
            for (let k = 1; k < stride; k++)
                out.push(value[i + k]);
        }
        return out;
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

// One LineMap per document, for a service that keeps per-line values of several documents.
class LineStores {
    constructor(stride) {
        this.maps = new Map();
        this.stride = stride;
    }

    // Null before anything was stored for the document.
    get(docId) {
        return this.maps.get(docId) ?? null;
    }

    open(docId) {
        let map = this.maps.get(docId);
        if (!map) {
            map = new LineMap(this.stride);
            this.maps.set(docId, map);
        }
        return map;
    }

    // `change` is what LineIndex.applyEdit returned for the edit that produced `version`.
    edited(docId, version, change) {
        const map = this.maps.get(docId);
        if (!map)
            return;
        map.edit(version, change.startLine, change.oldLines, change.newLines, change.column, change.removed, change.inserted);
    }

    forget(docId) {
        this.maps.delete(docId);
    }

    clear() {
        for (const map of this.maps.values())
            map.clear();
    }
}

function createMap(stride) {
    return new LineMap(stride);
}

function createStores(stride) {
    return new LineStores(stride);
}

function create(text) {
    const index = new LineIndex();
    index.build(text || "");
    return index;
}
