.pragma library

// The PluginSettings that holds a setting control, however deep the control sits inside it.
function find(item) {
    for (let current = item; current; current = current.parent) {
        if (typeof current.saveValue === "function" && typeof current.loadValue === "function" && current.pluginId !== undefined)
            return current;
    }
    return null;
}

// Reads a control's stored value into control.value; returns false outside a PluginSettings.
function load(control) {
    const settings = find(control.parent);
    if (!settings)
        return false;
    control.value = settings.loadValue(control.settingKey, control.defaultValue);
    return true;
}

function save(control, value) {
    control.value = value;
    const settings = find(control.parent);
    if (settings)
        settings.saveValue(control.settingKey, value);
}

// Calls loadValue() on every setting control below item, however deeply nested.
function reload(item) {
    for (const child of item.children) {
        if (typeof child.loadValue === "function")
            child.loadValue();
        else
            reload(child);
    }
}
