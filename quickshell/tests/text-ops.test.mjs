import assert from "node:assert/strict";
import test from "node:test";
import { load } from "./load.mjs";

const ops = load("../Common/TextOps.js");
const { create } = load("../Common/LineIndex.js");

test("smart home goes to the first non-blank column, then the line start", () => {
    assert.equal(ops.smartHome("    code", 7), 4);
    assert.equal(ops.smartHome("    code", 4), 0);
    assert.equal(ops.smartHome("    code", 0), 4);
    assert.equal(ops.smartHome("\t x", 3), 2);
    assert.equal(ops.smartHome("", 0), 0);
});

test("indent skips empty lines and unindent removes one level", () => {
    const indented = ops.indentLines(["a", "", "  b"], "\t");
    assert.deepEqual([...indented.lines], ["\ta", "", "\t  b"]);
    assert.deepEqual([...indented.added], [1, 0, 1]);
    assert.equal(indented.changed, true);

    const unindented = ops.unindentLines(["\t\ta", "      b", "  c", "d", "\t"], 4);
    assert.deepEqual([...unindented.lines], ["\ta", "  b", "c", "d", ""]);
    assert.deepEqual([...unindented.removed], [1, 4, 2, 0, 1]);
    assert.equal(ops.unindentLines(["a", " \tb"], 4).lines[1], "\tb");
    assert.equal(ops.unindentLines(["a", "b"], 4).changed, false);
});

test("columns follow the indentation change, column 0 stays put", () => {
    assert.equal(ops.mapColumn(0, 0, 4), 0);
    assert.equal(ops.mapColumn(3, 0, 4), 7);
    assert.equal(ops.mapColumn(6, 4, 0), 2);
    assert.equal(ops.mapColumn(2, 4, 0), 0);
});

test("join drops the whitespace around each join and empty lines", () => {
    assert.equal(ops.joinLines(["  a  ", "   b", "", "c  "]), "  a b c  ");
    assert.equal(ops.joinLines(["   ", "x"]), "x");
    assert.equal(ops.joinLines(["a", ""]), "a");
});

test("title case lowers the rest of each word", () => {
    assert.equal(ops.titleCase("hELLO wORLD, it's (dux-edit)"), "Hello World, It's (Dux-Edit)");
    assert.equal(ops.titleCase("one\ntwo"), "One\nTwo");
});

test("words are counted per line", () => {
    assert.equal(ops.countWords(["one two", "", "  three\tfour  "]), 4);
    assert.equal(ops.countWords([""]), 0);
});

test("brackets match across lines and nesting inside the window", () => {
    const text = "f(a[1], {\n  b: (c)\n})";
    const index = create(text);
    const open = text.indexOf("(");
    const close = text.lastIndexOf(")");
    assert.deepEqual([...ops.matchBracket(index, open + 1, 0, 2, 1000)], [open, close]);
    assert.deepEqual([...ops.matchBracket(index, close + 1, 0, 2, 1000)], [close, open]);
    assert.deepEqual([...ops.matchBracket(index, close, 0, 2, 1000)], [close - 1, text.indexOf("{")]);
    assert.deepEqual([...ops.matchBracket(index, open, 0, 2, 1000)], [open, close]);
    const brace = text.indexOf("{");
    assert.deepEqual([...ops.matchBracket(index, brace, 0, 2, 1000)], [brace, text.indexOf("}")]);
    assert.equal(ops.matchBracket(index, text.indexOf("b"), 0, 2, 1000), null);
    assert.equal(ops.matchBracket(index, open + 1, 0, 1, 1000), null);
    assert.equal(ops.matchBracket(index, open + 1, 0, 2, 5), null);
    assert.equal(ops.matchBracket(index, close, 0, 1, 1000), null);
    assert.equal(ops.matchBracket(create("(("), 1, 0, 0, 1000), null);
});

// Runs a plan on `text` with the selection written as "[" (anchor) and "|" (cursor), or "|" alone.
function run(make, marked) {
    const anchorAt = marked.indexOf("[");
    const plain = marked.replace("[", "");
    const cursorAt = plain.indexOf("|");
    const text = plain.replace("|", "");
    let anchor = anchorAt < 0 ? cursorAt : anchorAt;
    let cursor = cursorAt;
    if (anchorAt >= 0 && anchorAt > cursorAt)
        anchor--;
    const plan = make(create(text), (start, end) => text.slice(start, end), anchor, cursor);
    if (!plan)
        return null;
    const after = text.slice(0, plan.start) + plan.text + text.slice(plan.end);
    if (plan.anchor === plan.cursor)
        return after.slice(0, plan.cursor) + "|" + after.slice(plan.cursor);
    const marks = [[plan.anchor, "["], [plan.cursor, "|"]].sort((a, b) => b[0] - a[0]);
    let out = after;
    for (const [at, mark] of marks)
        out = out.slice(0, at) + mark + out.slice(at);
    return out;
}

test("reindent moves every selected line and keeps the selection on its text", () => {
    const indent = (index, read, anchor, cursor) => ops.reindentPlan(index, read, anchor, cursor, true, "    ", 4);
    const unindent = (index, read, anchor, cursor) => ops.reindentPlan(index, read, anchor, cursor, false, "    ", 4);
    assert.equal(run(indent, "a\n[b\nc|\nd"), "a\n[    b\n    c|\nd");
    assert.equal(run(indent, "a[b\n\nc|"), "    a[b\n\n    c|");
    assert.equal(run(indent, "[a\nb\n|c"), "[    a\n    b\n|c");
    assert.equal(run(unindent, "    a\n  b|c"), "    a\nb|c");
    assert.equal(run(unindent, "\t[a\n      b|"), "[a\n  b|");
    assert.equal(run(unindent, "  x|y"), "x|y");
    assert.equal(run(unindent, "x|y"), null);
});

test("delete lines keeps the cursor column on the line that takes their place", () => {
    const del = (index, read, anchor, cursor) => ops.deleteLinesPlan(index, anchor, cursor);
    assert.equal(run(del, "one\ntw|o\nthree"), "one\nth|ree");
    assert.equal(run(del, "one\n[two\nthr|ee"), "one|");
    assert.equal(run(del, "on|e"), "|");
    assert.equal(run(del, "a\nlon|g\nb"), "a\nb|");
});

test("duplicate copies the line below or the selection after itself", () => {
    assert.equal(run(ops.duplicatePlan, "a\nb|c\nd"), "a\nbc\nb|c\nd");
    assert.equal(run(ops.duplicatePlan, "x[ab|y"), "x[ab|aby");
});

test("move lines swaps with the neighbor and the selection follows", () => {
    const up = (index, read, anchor, cursor) => ops.moveLinesPlan(index, read, anchor, cursor, false);
    const down = (index, read, anchor, cursor) => ops.moveLinesPlan(index, read, anchor, cursor, true);
    assert.equal(run(up, "one\ntw|o\nthree"), "tw|o\none\nthree");
    assert.equal(run(down, "one\n[two\nth|ree\nfour"), "one\nfour\n[two\nth|ree");
    assert.equal(run(down, "[one\n|two\nthree"), "two\n[one\n|three");
    assert.equal(run(up, "fi|rst\nsecond"), null);
    assert.equal(run(down, "first\nla|st"), null);
});

test("join lines and sort lines replace only the selected lines", () => {
    assert.equal(run(ops.joinLinesPlan, "a  |\n   b\nc"), "a| b\nc");
    assert.equal(run(ops.joinLinesPlan, "[x\ny\nz|"), "x y z|");
    assert.equal(run(ops.joinLinesPlan, "last|"), null);
    assert.equal(run(ops.sortLinesPlan, "[pear\napple\nfig|\nend"), "[apple\nfig\npear|\nend");
    assert.equal(run(ops.sortLinesPlan, "single|"), null);
});

test("select line takes whole lines and grows on whole lines", () => {
    const index = create("one\ntwo\nthree");
    assert.deepEqual([...ops.lineSelection(index, 5, 5)], [4, 8]);
    assert.deepEqual([...ops.lineSelection(index, 4, 8)], [4, 13]);
    assert.deepEqual([...ops.lineSelection(index, 8, 13)], [8, 13]);
});
