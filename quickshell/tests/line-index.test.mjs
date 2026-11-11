import assert from "node:assert/strict";
import test from "node:test";
import { load } from "./load.mjs";

const { create } = load("../Common/LineIndex.js");

function starts(text) {
    const out = [0];
    for (let i = 0; i < text.length; i++)
        if (text[i] === "\n")
            out.push(i + 1);
    return out;
}

function assertMatches(index, text) {
    const expected = starts(text);
    assert.equal(index.count, expected.length);
    assert.deepEqual(Array.from({ length: index.count }, (_, line) => index.startOf(line)), expected);
    assert.equal(index.length, text.length);
    assert.equal(index.fullText(), text);
}

function edit(index, text, start, removed, inserted) {
    const change = index.applyEdit(start, removed, inserted);
    return [text.slice(0, start) + inserted + text.slice(start + removed), change];
}

test("build and lookups count UTF-16 units and real lines", () => {
    const text = "a😀\n\nlast";
    const index = create(text);
    assertMatches(index, text);
    assert.equal(index.lineAt(0), 0);
    assert.equal(index.lineAt(3), 0);
    assert.equal(index.lineAt(4), 1);
    assert.equal(index.lineAt(5), 2);
    assert.equal(index.lineAt(text.length), 2);
    assert.equal(create("").count, 1);
    assert.equal(create("\n").count, 2);
});

test("applyEdit reports the replaced line range like buffer.Change", () => {
    let text = "one\ntwo\nthree";
    const index = create(text);
    let change;
    [text, change] = edit(index, text, 1, 0, "X");
    assert.deepEqual({ ...change }, { startLine: 0, oldLines: 1, newLines: 1 });
    [text, change] = edit(index, text, 2, 6, "\nA\nB\nC");
    assert.deepEqual({ ...change }, { startLine: 0, oldLines: 2, newLines: 4 });
    assertMatches(index, text);
    // Removing a line break joins two lines.
    [text, change] = edit(index, text, text.indexOf("\n"), 1, "");
    assert.deepEqual({ ...change }, { startLine: 0, oldLines: 2, newLines: 1 });
    assertMatches(index, text);
    // Emoji: surrogate pairs count as two units.
    [text, change] = edit(index, text, text.length, 0, "😀\n😀");
    assertMatches(index, text);
    assert.equal(index.startOf(index.count - 1), text.length - 2);
});

test("random edits agree with a rebuild", () => {
    let seed = 11;
    const random = n => (seed = (seed * 1103515245 + 12345) % 2147483648) % n;
    const alphabet = ["x", "y", "\n", "\n", "😀", " "];
    const word = n => Array.from({ length: n }, () => alphabet[random(alphabet.length)]).join("");
    let text = word(500);
    const index = create(text);
    // Mostly local typing with occasional jumps, to cover the deferred shift in both directions.
    let cursor = 0;
    for (let i = 0; i < 3000; i++) {
        if (random(10) === 0)
            cursor = random(text.length + 1);
        cursor = Math.min(cursor, text.length);
        const removed = random(4) === 0 ? random(Math.min(30, text.length - cursor) + 1) : 0;
        const inserted = word(random(6));
        let change;
        const before = starts(text);
        [text, change] = edit(index, text, cursor, removed, inserted);
        const after = starts(text);
        cursor += inserted.length;
        assert.equal(after.length - before.length, change.newLines - change.oldLines);
        if (i % 50 === 0 || i > 2900)
            assertMatches(index, text);
    }
    assertMatches(index, text);
    for (let offset = 0; offset <= text.length; offset += 7) {
        const line = index.lineAt(offset);
        assert.ok(index.startOf(line) <= offset);
        assert.ok(line === index.count - 1 || index.startOf(line + 1) > offset);
        assert.equal(index.text(offset, offset + 40), text.slice(offset, offset + 40));
        assert.equal(index.lineText(line), text.slice(index.startOf(line), index.endOf(line)));
    }
});

const { createMap } = load("../Common/LineIndex.js");

test("a line map shifts values past an edit and drops the edited lines", () => {
    const map = createMap();
    map.merge(0, 0, ["a", "b", "c", "d"]);
    map.edit(1, 1, 1, 3);
    assert.equal(map.get(0), "a");
    assert.equal(map.get(1), undefined);
    assert.equal(map.get(3), undefined);
    assert.equal(map.get(4), "c");
    assert.equal(map.get(5), "d");
    map.edit(2, 0, 3, 1);
    assert.equal(map.get(0), undefined);
    assert.equal(map.get(2), "c");
});

test("values computed before an edit move through it and never land on edited lines", () => {
    const map = createMap();
    map.edit(1, 2, 1, 2);
    map.edit(2, 0, 1, 1);
    // Version 0 values predate both edits: line 0 and 2 were rewritten, lines past 2 moved down by one.
    map.merge(0, 0, ["zero", "one", "two", "three", "four"]);
    assert.equal(map.get(0), undefined);
    assert.equal(map.get(1), "one");
    assert.equal(map.get(2), undefined);
    assert.equal(map.get(4), "three");
    assert.equal(map.get(5), "four");
    // The core at version 1 already holds the first edit; only the second still applies.
    map.merge(1, 0, ["new zero", "new one", "new two"]);
    assert.equal(map.get(0), undefined);
    assert.equal(map.get(2), "new two");
    map.merge(2, 0, ["fresh"]);
    assert.equal(map.get(0), "fresh");
    assert.equal(map.edits.length, 0);
});

test("a line map that never merges stops tracking edits and refuses values from before them", () => {
    const map = createMap();
    for (let version = 1; version <= 2000; version++)
        map.edit(version, 0, 1, 2);
    assert.ok(map.edits.length <= 512);
    // Line 0 was rewritten by every edit since; a value this old must not land on it.
    map.merge(1, 0, ["stale"]);
    assert.equal(map.get(0), undefined);
    map.merge(2000, 0, ["fresh"]);
    assert.equal(map.get(0), "fresh");
});
