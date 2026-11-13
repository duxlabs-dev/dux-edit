import QtQuick
import qs.DCommon.Common
import "SettingsLookup.js" as SettingsLookup

// Root of a plugin's settings component. Setting controls placed inside it read and write the plugin's data.
Item {
    id: root

    required property string pluginId
    property var pluginService: null
    default property list<QtObject> content

    signal settingChanged

    implicitHeight: settingsColumn.implicitHeight
    height: implicitHeight

    function saveValue(key, value) {
        if (!pluginService)
            return;
        pluginService.savePluginData(pluginId, key, value);
        settingChanged();
    }

    function loadValue(key, defaultValue) {
        return pluginService ? pluginService.loadPluginData(pluginId, key, defaultValue) : defaultValue;
    }

    function saveState(key, value) {
        pluginService?.savePluginState(pluginId, key, value);
    }

    function loadState(key, defaultValue) {
        return pluginService ? pluginService.loadPluginState(pluginId, key, defaultValue) : defaultValue;
    }

    function clearState() {
        pluginService?.clearPluginState(pluginId);
    }

    function reloadChildValues() {
        SettingsLookup.reload(settingsColumn);
    }

    onContentChanged: {
        for (const child of content) {
            if (child instanceof Item)
                child.parent = settingsColumn;
        }
    }

    onPluginServiceChanged: reloadChildValues()
    Component.onCompleted: reloadChildValues()

    Connections {
        target: root.pluginService

        function onPluginDataChanged(changedPluginId) {
            if (changedPluginId === root.pluginId)
                root.reloadChildValues();
        }
    }

    Column {
        id: settingsColumn

        width: parent.width
        spacing: Style.spacingM
    }
}
