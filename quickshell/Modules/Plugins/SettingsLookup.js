.pragma library

// The PluginSettings that holds a setting control, however deep the control sits inside it.
function find(item) {
    for (let current = item; current; current = current.parent) {
        if (typeof current.saveValue === "function" && typeof current.loadValue === "function" && current.pluginId !== undefined)
            return current;
    }
    return null;
}
