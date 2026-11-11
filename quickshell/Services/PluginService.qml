pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
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
    // Directories the last scan read.
    property var pluginDirs: []
    property bool scanning: false
    // Plugin id to its live EditorPluginComponent.
    property var loadedPlugins: ({})
    // Plugin id to why its component failed to load.
    property var loadErrors: ({})
    // Contributions of loaded plugins, read by the shell chrome.
    // commands: [{pluginId, id, title, icon, shortcut, run}]
    property var commands: []
    // statusItems and sidePanels: [{pluginId, component}]
    property var statusItems: []
    property var sidePanels: []

    signal pluginDataChanged(string pluginId)
    signal pluginStateChanged(string pluginId)
    signal pluginLoaded(string pluginId)
    signal pluginUnloaded(string pluginId)

    property bool _startupScanPending: Object.keys(SettingsData.enabledPlugins || {}).length > 0
    property bool _dataWanted: false
    property var _data: null
    property bool _dataDirty: false
    property var _states: ({})
    property var _stateFiles: ({})
    property var _dirtyStates: ({})

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
        return value === undefined ? defaultValue : value;
    }

    function savePluginData(pluginId, key, value) {
        const data = _pluginData();
        const current = data[pluginId] || {};
        if (JSON.stringify(current[key]) === JSON.stringify(value))
            return true;
        data[pluginId] = Object.assign({}, current, {
            [key]: value
        });
        _dataDirty = true;
        writeTimer.restart();
        pluginDataChanged(pluginId);
        return true;
    }

    function loadPluginState(pluginId, key, defaultValue) {
        const value = _state(pluginId)[key];
        return value === undefined ? defaultValue : value;
    }

    function savePluginState(pluginId, key, value) {
        const state = _state(pluginId);
        if (JSON.stringify(state[key]) === JSON.stringify(value))
            return;
        state[key] = value;
        _markState(pluginId);
    }

    function clearPluginState(pluginId) {
        if (Object.keys(_state(pluginId)).length === 0)
            return;
        _states[pluginId] = {};
        _markState(pluginId);
    }

    function removePluginStateKey(pluginId, key) {
        const state = _state(pluginId);
        if (!(key in state))
            return;
        delete state[key];
        _markState(pluginId);
    }

    function _syncLoaded() {
        for (const id in loadedPlugins) {
            const m = manifest(id);
            if (!m || m.error || !isEnabled(id))
                _unload(id);
        }
        for (const m of availablePlugins) {
            if (isEnabled(m.id))
                _load(m);
        }
    }

    function _load(m) {
        if (loadedPlugins[m.id] || m.error)
            return;
        const component = Qt.createComponent("file://" + m.dir + "/" + m.editor, Component.PreferSynchronous);
        if (component.status !== Component.Ready) {
            _setLoadError(m.id, component.errorString());
            return;
        }
        const instance = component.createObject(root, {
            "pluginId": m.id,
            "pluginService": root,
            "editor": root.editor
        });
        if (!instance) {
            _setLoadError(m.id, component.errorString());
            return;
        }
        if (!(instance instanceof EditorPluginComponent)) {
            instance.destroy();
            _setLoadError(m.id, I18n.tr("The editor component's root is not EditorPluginComponent.", "plugin load error"));
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
        _flush();
        _stateFiles[pluginId]?.destroy();
        delete _stateFiles[pluginId];
        delete _states[pluginId];
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
                    "id": c.id ?? "",
                    "title": c.title ?? "",
                    "icon": c.icon ?? "",
                    "shortcut": c.shortcut ?? "",
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
        commands = nextCommands;
        statusItems = nextStatus;
        sidePanels = nextPanels;
    }

    function _pluginData() {
        if (_data)
            return _data;
        _dataWanted = true;
        _data = _parse(dataFile.text());
        return _data;
    }

    function _state(pluginId) {
        if (_states[pluginId])
            return _states[pluginId];
        const file = stateFileComponent.createObject(root, {
            "path": AppPaths.pluginStateDir + "/" + pluginId + "_state.json"
        }) as FileView;
        _stateFiles[pluginId] = file;
        _states[pluginId] = _parse(file.text());
        return _states[pluginId];
    }

    function _markState(pluginId) {
        _dirtyStates[pluginId] = true;
        writeTimer.restart();
        pluginStateChanged(pluginId);
    }

    function _parse(text) {
        if (!text || !text.trim())
            return {};
        try {
            const value = JSON.parse(text);
            if (value && typeof value === "object" && !Array.isArray(value))
                return value;
        } catch (e) {
            log.warn("ignoring unreadable plugin data:", e);
        }
        return {};
    }

    function _flush() {
        writeTimer.stop();
        if (_dataDirty) {
            _dataDirty = false;
            dataFile.setText(JSON.stringify(_data, null, 2));
        }
        const dirty = _dirtyStates;
        _dirtyStates = {};
        for (const id in dirty)
            _stateFiles[id]?.setText(JSON.stringify(_states[id] || {}, null, 2));
    }

    Component.onCompleted: {
        if (_startupScanPending && Backend.connected) {
            _startupScanPending = false;
            rescan();
        }
    }

    Component.onDestruction: _flush()

    Connections {
        target: Backend
        enabled: root._startupScanPending

        function onConnectedChanged() {
            if (!Backend.connected)
                return;
            root._startupScanPending = false;
            root.rescan();
        }
    }

    Timer {
        id: writeTimer

        interval: EditorMetrics.pluginWriteDelay
        onTriggered: root._flush()
    }

    FileView {
        id: dataFile

        path: root._dataWanted ? AppPaths.pluginSettingsFile : ""
        blockLoading: true
        blockWrites: true
        atomicWrites: true
        printErrors: false
    }

    Component {
        id: stateFileComponent

        FileView {
            blockLoading: true
            blockWrites: true
            atomicWrites: true
            printErrors: false
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

    // What plugins get as `editor`. It never reaches the elevated save path.
    readonly property QtObject editor: QtObject {
        id: editorApi

        readonly property var document: Documents.current
        readonly property var pane: document?.editor ?? null

        readonly property string filePath: document?.path ?? ""
        readonly property string language: document?.lexer ?? ""
        // 1-based, as the status bar shows them.
        readonly property int cursorLine: document?.cursorLine ?? 0
        readonly property int cursorColumn: document?.cursorColumn ?? 0
        readonly property string selectedText: pane?.selectedText ?? ""

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
