// `node --test quickshell/tests/` resolves the directory to this file; it loads every test module.
const { readdirSync } = require("node:fs");
const { join } = require("node:path");
const { pathToFileURL } = require("node:url");

for (const name of readdirSync(__dirname).filter(name => name.endsWith(".test.mjs")).sort())
    import(pathToFileURL(join(__dirname, name)).href);
