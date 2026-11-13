import assert from "node:assert/strict";
import test from "node:test";
import { load } from "./load.mjs";

const { diff } = load("../Common/EditTracker.js");

function apply(before, change) {
    return before.slice(0, change.start) + change.text + before.slice(change.start + change.removed);
}

function check(before, after) {
    const change = diff(before, after);
    assert.ok(change, `${JSON.stringify(before)} -> ${JSON.stringify(after)}`);
    assert.equal(apply(before, change), after);
    return change;
}

test("equal texts give no change", () => {
    assert.equal(diff("", ""), null);
    assert.equal(diff("same", "same"), null);
});

test("the change is minimal for inserts, deletes and replacements", () => {
    assert.deepEqual({ ...check("hello", "help me") }, { start: 3, removed: 2, text: "p me" });
    assert.deepEqual({ ...check("abc", "abXc") }, { start: 2, removed: 0, text: "X" });
    assert.deepEqual({ ...check("abXc", "abc") }, { start: 2, removed: 1, text: "" });
    assert.deepEqual({ ...check("", "new") }, { start: 0, removed: 0, text: "new" });
    assert.deepEqual({ ...check("gone", "") }, { start: 0, removed: 4, text: "" });
    assert.deepEqual({ ...check("aaaa", "aaaaa") }, { start: 4, removed: 0, text: "a" });
});

test("a surrogate pair is never split", () => {
    // U+1F600 and U+1F601 share their high surrogate.
    const a = "x\u{1F600}y";
    const b = "x\u{1F601}y";
    assert.deepEqual({ ...check(a, b) }, { start: 1, removed: 2, text: "\u{1F601}" });
    // The low surrogates match but the high ones differ (U+1F600 and U+10600 share 0xDE00).
    assert.deepEqual({ ...check("\u{1F600}", "\u{10600}") }, { start: 0, removed: 2, text: "\u{10600}" });
    assert.deepEqual({ ...check("a\u{1F600}\u{1F600}b", "a\u{1F600}b") }, { start: 3, removed: 2, text: "" });
});

test("edits in multi-megabyte texts land at the right offset across chunk boundaries", () => {
    const line = "func main() { return 42 } // ünïcode 😀\n";
    const big = line.repeat(60000);
    for (const at of [0, 1, 1023, 1024, 1025, 4097, big.length >> 1, big.length - 1, big.length]) {
        check(big, big.slice(0, at) + "Z" + big.slice(at));
        if (at < big.length - 3)
            check(big, big.slice(0, at) + big.slice(at + 3));
    }
    const pasted = "pasted\n".repeat(5000);
    check(big, big.slice(0, 777) + pasted + big.slice(999));
});

test("random edits round-trip", () => {
    let seed = 7;
    const random = n => (seed = (seed * 1103515245 + 12345) % 2147483648) % n;
    const alphabet = ["a", "b", "\n", "\t", " ", "é", "\u{1F600}", "\u{1F601}", "中"];
    const word = n => Array.from({ length: n }, () => alphabet[random(alphabet.length)]).join("");
    let text = word(3000);
    for (let i = 0; i < 400; i++) {
        const cut = Array.from(text);
        const at = random(cut.length + 1);
        const removed = random(Math.min(40, cut.length - at + 1));
        cut.splice(at, removed, word(random(20)));
        const next = cut.join("");
        if (next !== text)
            check(text, next);
        text = next;
    }
});
