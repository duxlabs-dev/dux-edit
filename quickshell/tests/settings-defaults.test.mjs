import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { load } from "./load.mjs";

const Defaults = load("../Common/SettingsDefaults.js");

test("an unknown density falls back to the comfortable defaults", () => {
    assert.equal(Defaults.value("focusRingWidth", "bogus"), Defaults.value("focusRingWidth", "comfortable"));
    assert.equal(Defaults.value("notASetting", "comfortable"), undefined);
});

test("a density switch moves only settings still on the outgoing default", () => {
    const settings = {};
    for (const key in Defaults.DEFAULTS)
        settings[key] = Defaults.value(key, "comfortable");
    settings.focusRingWidth = 3;
    const moved = Defaults.transition(settings, "comfortable", "dense");
    assert.equal("focusRingWidth" in moved, false);
    assert.equal(moved.radiusStrength, Defaults.value("radiusStrength", "dense"));
    assert.equal(Object.keys(Defaults.transition(settings, "dense", "dense")).length, 0);
});

test("the comfortable defaults match the settings file defaults", () => {
    const source = readFileSync(new URL("../Common/SettingsData.qml", import.meta.url), "utf8");
    for (const key in Defaults.DEFAULTS) {
        const match = source.match(new RegExp(`^\\s+property (?!alias )\\w+ ${key}: (.+)$`, "m"));
        assert.ok(match, `${key} is not in the settings adapter`);
        assert.equal(Number(match[1]), Defaults.value(key, "comfortable"), key);
    }
});

test("an old millisecond animation duration maps to the nearest DMS motion step", () => {
    assert.equal(Defaults.motionStep(0), -1);
    assert.equal(Defaults.motionStep(250), 0);
    assert.equal(Defaults.motionStep(400), 1);
    assert.equal(Defaults.motionStep(500), 2);
    assert.equal(Defaults.motionStep(650), 3);
    assert.equal(Defaults.motionStep(1000), 4);
});
