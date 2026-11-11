import assert from "node:assert/strict";
import test from "node:test";
import { hex, load } from "./load.mjs";

const context = load("../Common/SyntaxPalette.js");
const { derive } = load("../DCommon/Common/Accents.js");
const { ratio } = load("../DCommon/Common/Contrast.js");

function style(isLight, primary, surface) {
    const accents = derive(hex(primary), isLight, null);
    const tone = isLight ? { primary: "#6750a4", secondary: "#625b71", tertiary: "#7d5260", error: "#b3261e", onSurfaceVariant: "#49454f", onSurface: "#1d1b20" }
        : { primary: "#d0bcff", secondary: "#ccc2dc", tertiary: "#efb8c8", error: "#f2b8b5", onSurfaceVariant: "#cac4d0", onSurface: "#e6e0e9" };
    return { isLightMode: isLight, surface: hex(surface), accents, ...Object.fromEntries(Object.entries(tone).map(([k, v]) => [k, hex(v)])) };
}

test("every role reads at 4.5:1 on the editor surface in light and dark themes", () => {
    const themes = [style(false, "#D0BCFF", "#141218"), style(true, "#6750A4", "#fef7ff"), style(false, "#7fbbb3", "#2d353b"), style(true, "#ff2d95", "#ffffff"), style(false, "#9a9a9a", "#3b383e")];
    for (const theme of themes) {
        const palette = context.build(theme, {});
        assert.equal(palette.length, 24);
        palette.forEach((entry, role) => assert.ok(ratio(hex(entry.color), theme.surface) >= 4.5, `role ${role} ${entry.color} on light=${theme.isLightMode}`));
    }
});

test("accent roles keep distinct hues and overrides win", () => {
    const theme = style(false, "#D0BCFF", "#141218");
    const palette = context.build(theme, { "string": "#123456" });
    const names = context.names(palette);
    assert.equal(names.string, "#123456");
    assert.notEqual(names.number, names.tag);
    assert.notEqual(names.escape, names.variable);
});
