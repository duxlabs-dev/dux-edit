pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.DCommon.Common
import qs.Common
import qs.Services
// Also makes the scanner write the qmldir for qs.Modules.Plugins, which otherwise only plugins import.
import qs.Modules.Plugins
import qs.Modules.Editor

// Editor plugin host. Function names match the DMS PluginService where a DMS plugin would call them.
Singleton {
    id: root

    readonly property var log: Log.scoped("PluginService")

    // Manifests from plugins.list: [{id, name, description, version, author, icon, dir, editor, settings, error}]
    property var availablePlugins: []
    property var pluginDirs: []
    property bool scanning: false
    // Plugin id to its live EditorPluginComponent.
    property var loadedPlugins: ({})
    // Plugin id to why its component failed to load.
    property var loadErrors: ({})
    // Contributions of loaded plugins, read by the shell chrome. Empty while suspended.
    // commands: [{pluginId, id, title, icon, shortcut, run}], shortcut a key sequence or ""
    readonly property var commands: _commands
    // statusItems and sidePanels: [{pluginId, component}]
    readonly property var statusItems: _statusItems
    readonly property var sidePanels: _sidePanels

    signal pluginDataChanged(string pluginId)
    signal pluginStateChanged(string pluginId)
    signal pluginLoaded(string pluginId)
    signal pluginUnloaded(string pluginId)

    property bool _startupScanPending: Object.keys(SettingsData.enabledPlugins || {}).length > 0
    property bool _suspended: false
    // The plugin being created, whose storage calls from onCompleted must keep their store.
    property string _loading: ""
    property var _commands: []
    property var _statusItems: []
    property var _sidePanels: []
    property bool _dataWanted: false
    property var _data: null
    property var _states: ({})
    property var _stateStores: ({})

    function suspend() {
        if (_suspended)
            return;
        _suspended = true;
        for (const id in loadedPlugins)
            _unload(id);
        _releaseData();
    }

    function resume() {
        if (!_suspended)
            return;
        _suspended = false;
        if (_startupScanPending)
            _startupScan();
        else
            _syncLoaded();
    }

    // Scans only on request: when the settings page opens, after an install, and at startup when a plugin is enabled.
    function rescan() {
        scanning = true;
        Backend.call("plugins.list", null, result => {
            root.scanning = false;
            if (result.error) {
                root.log.warn("plugins.list failed:", result.error);
                return;
            }
            root.availablePlugins = result.plugins || [];
            root.pluginDirs = result.dirs || [];
            root._syncLoaded();
        });
    }

    function manifest(pluginId) {
        return availablePlugins.find(m => m.id === pluginId) ?? null;
    }

    function isEnabled(pluginId) {
        return !!(SettingsData.enabledPlugins || {})[pluginId];
    }

    function enablePlugin(pluginId) {
        const enabled = Object.assign({}, SettingsData.enabledPlugins || {});
        enabled[pluginId] = true;
        SettingsData.set("enabledPlugins", enabled);
        const m = manifest(pluginId);
        if (m)
            _load(m);
    }

    function disablePlugin(pluginId) {
        const enabled = Object.assign({}, SettingsData.enabledPlugins || {});
        delete enabled[pluginId];
        SettingsData.set("enabledPlugins", enabled);
        _unload(pluginId);
    }

    function getPluginPath(pluginId) {
        return manifest(pluginId)?.dir ?? "";
    }

    function loadPluginData(pluginId, key, defaultValue) {
        const value = _pluginData()[pluginId]?.[key];
        _releaseIdle(pluginId);
        return value === undefined ? defaultValue : value;
    }

    function savePluginData(pluginId, key, value) {
        const data = _pluginData();
        const current = data[pluginId] || {};
        if (JSON.stringify(current[key]) !== JSON.stringify(value)) {
            data[pluginId] = Object.assign({}, current, {
                [key]: value
            });
            dataStore.save();
            pluginDataChanged(pluginId);
        }
        _releaseIdle(pluginId);
        return true;
    }

    function loadPluginState(pluginId, key, defaultValue) {
        const value = _state(pluginId)[key];
        _releaseIdle(pluginId);
        return value === undefined ? defaultValue : value;
    }

    function savePluginState(pluginId, key, value) {
        const state = _state(pluginId);
        if (JSON.stringify(state[key]) !== JSON.stringify(value)) {
            state[key] = value;
            _markState(pluginId);
        }
        _releaseIdle(pluginId);
    }

    function clearPluginState(pluginId) {
        if (Object.keys(_state(pluginId)).length > 0) {
            _states[pluginId] = {};
            _markState(pluginId);
        }
        _releaseIdle(pluginId);
    }

    function removePluginStateKey(pluginId, key) {
        const state = _state(pluginId);
        if (key in state) {
            delete state[key];
            _markState(pluginId);
        }
        _releaseIdle(pluginId);
    }

    function _startupScan() {
        if (!Backend.connected)
            return;
        _startupScanPending = false;
        rescan();
    }

    function _syncLoaded() {
        for (const id in loadedPlugins) {
            const m = manifest(id);
            if (_suspended || !m || m.error || !isEnabled(id))
                _unload(id);
        }
        for (const m of availablePlugins) {
            if (isEnabled(m.id))
                _load(m);
        }
    }

    function _load(m) {
        if (_suspended || loadedPlugins[m.id] || m.error)
            return;
        const component = Qt.createComponent("file://" + m.dir + "/" + m.editor, Component.PreferSynchronous);
        if (component.status !== Component.Ready) {
            _setLoadError(m.id, component.errorString());
            return;
        }
        _loading = m.id;
        const instance = component.createObject(root, {
            "pluginId": m.id,
            "pluginService": root,
            "editor": root.editor
        });
        _loading = "";
        if (!instance) {
            _setLoadError(m.id, component.errorString());
            _releaseIdle(m.id);
            return;
        }
        if (!(instance instanceof EditorPluginComponent)) {
            instance.destroy();
            _setLoadError(m.id, I18n.tr("The editor component's root is not EditorPluginComponent.", "plugin load error"));
            _releaseIdle(m.id);
            return;
        }
        _setLoadError(m.id, "");
        const loaded = Object.assign({}, loadedPlugins);
        loaded[m.id] = instance;
        loadedPlugins = loaded;
        collectContributions();
        log.info("loaded plugin", m.id);
        pluginLoaded(m.id);
    }

    function _unload(pluginId) {
        const instance = loadedPlugins[pluginId];
        if (!instance)
            return;
        const loaded = Object.assign({}, loadedPlugins);
        delete loaded[pluginId];
        loadedPlugins = loaded;
        collectContributions();
        pluginUnloaded(pluginId);
        instance.destroy();
        dataStore.flush();
        _releaseState(pluginId);
    }

    function _setLoadError(pluginId, message) {
        if ((loadErrors[pluginId] ?? "") === message)
            return;
        if (message)
            log.warn("cannot load plugin", pluginId + ":", message);
        const errors = Object.assign({}, loadErrors);
        if (message)
            errors[pluginId] = message;
        else
            delete errors[pluginId];
        loadErrors = errors;
    }

    // EditorPluginComponent calls this when a plugin changes what it contributes.
    function collectContributions() {
        const nextCommands = [];
        const nextStatus = [];
        const nextPanels = [];
        for (const id in loadedPlugins) {
            const plugin = loadedPlugins[id];
            for (const c of plugin.commands || []) {
                if (!c || typeof c.run !== "function")
                    continue;
                nextCommands.push({
                    "pluginId": id,
                    "id": String(c.id ?? ""),
                    "title": String(c.title ?? ""),
                    "icon": String(c.icon ?? ""),
                    "shortcut": typeof c.shortcut === "string" ? c.shortcut : "",
                    "run": c.run
                });
            }
            if (plugin.statusItem)
                nextStatus.push({
                    "pluginId": id,
                    "component": plugin.statusItem
                });
            if (plugin.sidePanel)
                nextPanels.push({
                    "pluginId": id,
                    "component": plugin.sidePanel
                });
        }
        _commands = nextCommands;
        _statusItems = nextStatus;
        _sidePanels = nextPanels;
    }

    function _pluginData() {
        if (_data)
            return _data;
        _dataWanted = true;
        _data = dataStore.objectFrom(dataStore.read().trim()) ?? {};
        return _data;
    }

    function _state(pluginId) {
        if (_states[pluginId])
            return _states[pluginId];
        const store = stateStoreComponent.createObject(root, {
            "path": AppPaths.pluginStateDir + "/" + pluginId + "_state.json",
            "serialize": () => JSON.stringify(root._states[pluginId] || {}, null, 2)
        });
        _stateStores[pluginId] = store;
        _states[pluginId] = store.objectFrom(store.read().trim()) ?? {};
        return _states[pluginId];
    }

    function _markState(pluginId) {
        _stateStores[pluginId].save();
        pluginStateChanged(pluginId);
    }

    // Keeps no store for a plugin that is not loaded, such as one saving from its onDestruction after suspend().
    function _releaseIdle(pluginId) {
        if (_suspended)
            _releaseData();
        if (!loadedPlugins[pluginId] && pluginId !== _loading)
            _releaseState(pluginId);
    }

    function _releaseData() {
        dataStore.flush();
        _data = null;
        _dataWanted = false;
    }

    function _releaseState(pluginId) {
        const store = _stateStores[pluginId];
        if (!store)
            return;
        store.flush();
        // An empty path waits for a write in flight, which FileView does not do when destroyed.
        store.path = "";
        store.destroy();
        delete _stateStores[pluginId];
        delete _states[pluginId];
    }

    Component.onCompleted: {
        if (_startupScanPending)
            _startupScan();
    }

    Component.onDestruction: {
        dataStore.flush();
        for (const id in _stateStores)
            _stateStores[id].flush();
    }

    Connections {
        target: Backend
        enabled: root._startupScanPending && !root._suspended

        function onConnectedChanged() {
            root._startupScan();
        }
    }

    JsonStore {
        id: dataStore

        name: "plugin settings"
        path: root._dataWanted ? AppPaths.pluginSettingsFile : ""
        interval: EditorMetrics.pluginWriteDelay
        serialize: () => JSON.stringify(root._data || {}, null, 2)
    }

    Component {
        id: stateStoreComponent

        JsonStore {
            name: "plugin state"
            interval: EditorMetrics.pluginWriteDelay
        }
    }

    // Every save of an open document, for editor.documentSaved; only while a plugin is loaded.
    Instantiator {
        active: Object.keys(root.loadedPlugins).length > 0
        model: active ? Documents.tabs : []

        delegate: Connections {
            required property var modelData

            target: modelData

            function onSaved() {
                editorApi.documentSaved(modelData.path);
            }
        }
    }

    // What plugins get as `editor`. It never reaches the elevated save path. The pane clears
    // `document.editor` when it is destroyed, so `pane` never holds a dead editor.
    readonly property QtObject editor: QtObject {
        id: editorApi

        readonly property var document: Documents.current
        readonly property var pane: document?.editor ?? null

        readonly property string filePath: document?.path ?? ""
        readonly property string language: document?.lexer ?? ""
        // 1-based, as the status bar shows them.
        readonly property int cursorLine: document?.cursorLine ?? 0
        readonly property int cursorColumn: document?.cursorColumn ?? 0
        // Only while a plugin is loaded, since it copies the selection. The version re-reads it after each
        // edit: the selection can move before the pane's mirror has the edit.
        readonly property string selectedText: {
            if (Object.keys(root.loadedPlugins).length === 0 || !pane)
                return "";
            const start = pane.selectionStart;
            const end = pane.selectionEnd;
            return document.version >= 0 && end > start ? pane.textRange(start, end) : "";
        }

        signal activeDocumentChanged
        signal documentSaved(string path)

        onDocumentChanged: activeDocumentChanged()

        function getText() {
            return pane ? pane.getText() : "";
        }

        function insertText(text) {
            pane?.insertText(text);
        }

        function replaceSelection(text) {
            pane?.replaceSelection(text);
        }

        // Offsets are UTF-16 units, as TextEdit counts them.
        function replaceRange(start, end, text) {
            pane?.replaceRange(start, end, text);
        }

        function openFile(path, line) {
            Documents.openPaths([path], line ?? 0, 0);
        }
    }
}
