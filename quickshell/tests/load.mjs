import { readFileSync } from "node:fs";
import vm from "node:vm";

const rgba = (r, g, b, a = 1) => ({ r, g, b, a });
const hex = value => rgba(...[1, 3, 5].map(i => parseInt(value.slice(i, i + 2), 16) / 255));
const Qt = { rgba, color: value => typeof value === "string" ? hex(value) : value };

export { rgba, hex };

// Runs a `.pragma library` file in its own context, the way QML scopes it, with each
// `.import "file" as Name` loaded the same way and bound to Name.
export function load(file, base = import.meta.url) {
    const url = new URL(file, base);
    const source = readFileSync(url, "utf8");
    const context = vm.createContext({ Qt });
    for (const match of source.matchAll(/^\.import\s+"([^"]+)"\s+as\s+(\w+)/gm))
        context[match[2]] = load(match[1], url);
    vm.runInContext(source.replace(/^\.(pragma|import).*$/gm, ""), context);
    return context;
}
