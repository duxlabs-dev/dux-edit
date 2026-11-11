.pragma library
.import "../DCommon/Common/Contrast.js" as Contrast
.import "../DCommon/Common/Hct.js" as Hct

// Wire order of core/internal/highlight/roles.go. A span's role id indexes this list.
const roles = ["text", "keyword", "type", "constant", "function", "tag", "attribute", "preproc", "variable", "string", "escape", "regex", "number", "operator", "punctuation", "comment", "heading", "emphasis", "strong", "inserted", "deleted", "error", "link", "code"];

// Roles colored by a theme token; the rest take a harmonized accent slot (Style.accents).
const TOKEN_ROLES = {
    "keyword": "primary",
    "type": "tertiary",
    "function": "secondary",
    "comment": "onSurfaceVariant",
    "punctuation": "onSurfaceVariant",
    "error": "error",
    "heading": "primary",
    "link": "primary"
};
const ACCENT_ROLES = {
    "constant": "orange",
    "number": "orange",
    "tag": "red",
    "deleted": "red",
    "attribute": "yellow",
    "string": "green",
    "inserted": "green",
    "escape": "teal",
    "regex": "teal",
    "code": "teal",
    "variable": "blue",
    "preproc": "pink",
    "operator": "purple"
};
const BOLD = ["heading", "strong"];
const ITALIC = ["comment", "emphasis"];
const TARGET_RATIO = 4.5;
const GLYPH_TONE_DARK = 80;
const GLYPH_TONE_LIGHT = 40;

function toHex(color) {
    const channel = value => Math.round(Math.max(0, Math.min(1, value)) * 255).toString(16).padStart(2, "0");
    return "#" + channel(color.r) + channel(color.g) + channel(color.b);
}

// The accent's hue and chroma at a glyph tone, stepped away from the background until it reads at 4.5:1.
function retone(accent, background, isLight) {
    const hct = Hct.toHct(accent.onContainer || accent.container);
    let tone = isLight ? GLYPH_TONE_LIGHT : GLYPH_TONE_DARK;
    let color = Hct.fromHct(hct.hue, hct.chroma, tone);
    while (tone > 0 && tone < 100 && Contrast.ratio(color, background) < TARGET_RATIO) {
        tone += isLight ? -1 : 1;
        color = Hct.fromHct(hct.hue, hct.chroma, tone);
    }
    return color;
}

function baseColor(style, name, background) {
    const token = TOKEN_ROLES[name];
    if (token)
        return style[token];
    const slot = ACCENT_ROLES[name];
    const accent = slot && style.accents ? style.accents[slot] : null;
    if (accent)
        return retone(accent, background, style.isLightMode);
    return style.onSurface;
}

// Returns one {color: "#rrggbb", bold, italic} per role, readable on Style.surface. `style` is the DCommon
// Style singleton, `overrides` maps role names to the user's colors, which are taken as they are.
function build(style, overrides) {
    const background = style.surface;
    return roles.map(name => {
        const override = overrides ? overrides[name] : "";
        const color = override ? Qt.color(override) : Contrast.readableOn(background, [baseColor(style, name, background), style.onSurface], TARGET_RATIO);
        return {
            "color": toHex(color),
            "bold": BOLD.includes(name),
            "italic": ITALIC.includes(name)
        };
    });
}

// Role name to "#rrggbb", the shape markdown.render takes.
function names(palette) {
    const out = {};
    roles.forEach((name, role) => out[name] = palette[role].color);
    return out;
}
