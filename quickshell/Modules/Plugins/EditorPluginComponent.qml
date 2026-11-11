import QtQml

// Root type of an editor plugin's main component. The host sets pluginId, pluginService and editor.
// A QtObject, not an Item: it has no place in a window, and children are helpers such as Process or Timer.
QtObject {
    id: root

    default property list<QtObject> resources

    property string pluginId: ""
    property var pluginService: null
    property var editor: null

    // [{id, title, icon, shortcut, run}], shown in the window menu.
    property var commands: []
    property Component statusItem: null
    property Component sidePanel: null

    onCommandsChanged: pluginService?.collectContributions()
    onStatusItemChanged: pluginService?.collectContributions()
    onSidePanelChanged: pluginService?.collectContributions()
}
