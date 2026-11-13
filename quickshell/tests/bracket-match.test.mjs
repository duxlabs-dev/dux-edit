import assert from "node:assert/strict";
import test from "node:test";
import { load } from "./load.mjs";

const { create } = load("../Common/BracketMatch.js");
const request = (changes = {}) => ({docId: "d", version: 0, offset: 1, first: 0, last: 10, ...changes});

function harness() {
    const calls = [];
    const applied = [];
    const matcher = create((params, done) => calls.push({params, done}), pair => applied.push(pair));
    return {calls, applied, matcher};
}

test("cursor moves coalesce behind one request and never apply its stale pair", () => {
    const {calls, applied, matcher} = harness();
    matcher.update(request(), "go:exact");
    matcher.update(request({offset: 2}), "go:exact");
    matcher.update(request({offset: 3}), "go:exact");
    assert.equal(calls.length, 1);
    calls[0].done({ok: true, version: 0, pair: [0, 10]});
    assert.deepEqual(applied, [null, null, null]);
    assert.equal(calls.length, 2);
    assert.equal(calls[1].params.offset, 3);
    calls[1].done({ok: true, version: 0, pair: [2, 8]});
    assert.deepEqual(applied.at(-1), [2, 8]);
    matcher.update(request({offset: 3}), "go:exact");
    assert.equal(calls.length, 2);
});

test("edits, document loads, viewport and lexer changes invalidate old results", () => {
    for (const [next, state] of [
        [request({version: 1}), "go:exact"],
        [request({docId: "other"}), "go:exact"],
        [request({first: 1, last: 11}), "go:exact"],
        [request(), "javascript:exact"],
        [request(), "go:off"]
    ]) {
        const {calls, applied, matcher} = harness();
        matcher.update(request(), "go:exact");
        matcher.update(next, state);
        calls[0].done({ok: true, version: 0, pair: [0, 10]});
        assert.ok(applied.every(pair => pair === null));
        assert.equal(calls.length, 2);
        calls[1].done({ok: true, version: next.version, pair: [1, 9]});
        assert.deepEqual(applied.at(-1), [1, 9]);
    }
});

test("clearing on hide or moving away and destruction ignore pending replies", () => {
    for (const stop of [false, true]) {
        const {calls, applied, matcher} = harness();
        matcher.update(request(), "go:exact");
        if (stop)
            matcher.stop();
        else
            matcher.update(null);
        const count = applied.length;
        calls[0].done({ok: true, version: 0, pair: [0, 10]});
        assert.equal(applied.length, count);
        assert.equal(calls.length, 1);
    }
});

test("newer mirror versions and failed replies cannot paint a pair", () => {
    for (const result of [
        {ok: true, version: 1, pair: [0, 10]},
        {ok: false, code: "SUPERSEDED"},
        {error: "DISCONNECTED"}
    ]) {
        const {calls, applied, matcher} = harness();
        matcher.update(request(), "go:exact");
        calls[0].done(result);
        assert.deepEqual(applied, [null]);
    }
});
