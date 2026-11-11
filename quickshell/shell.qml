//@ pragma Env QT_WAYLAND_DISABLE_WINDOWDECORATION=1
//@ pragma Env QSG_RENDER_LOOP=threaded
//@ pragma Env QT_QUICK_CONTROLS_STYLE=Material
//@ pragma UseQApplication
//@ pragma AppId dev.duxlabs.edit
//@ pragma RenderControl
//@ pragma DiskCache

pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import qs.Common
import qs.DCommon.Common as DC
import qs.Modules
import qs.Services

ShellRoot {
    id: root

    readonly property string appId: "dev.duxlabs.edit"

    readonly property var ownToplevel: ToplevelManager.toplevels.values.find(toplevel => toplevel.appId === appId) ?? null

    function showWindow() {
        if (!windowLoader.active) {
            windowLoader.active = true;
            return;
        }
        if (!ownToplevel)
            return;
        if (ownToplevel.minimized)
            ownToplevel.minimized = false;
        ownToplevel.activate();
    }

    // Hiding destroys the window and its editors; open documents survive in the session and their swaps.
    function hideWindow() {
        if (!windowLoader.active)
            return;
        Documents.suspend();
        windowLoader.active = false;
    }

    function closeWindow() {
        if (SettingsData.closeBehavior === "background") {
            hideWindow();
            return;
        }
        Documents.confirmQuit(() => {
            Documents.suspend();
            Backend.quit();
        });
    }

    Component.onCompleted: {
        Quickshell.watchFiles = Quickshell.env("DUX_EDIT_DISABLE_HOT_RELOAD") !== "1";
        DC.Log.backend = Log;
        DC.I18n.backend = I18n;
        DC.Paths.backend = AppPaths;
        DC.Style.settings = SettingsData;
        DC.Style.theme = Theme;
        DC.Host.session = SessionService;
        DC.Host.cache = CacheData;
        Theme.watching = Qt.binding(() => windowLoader.active);
    }

    Connections {
        target: Backend

        function onShowRequested() {
            root.showWindow();
        }
        function onHideRequested() {
            root.hideWindow();
        }
        function onToggleRequested() {
            if (windowLoader.active)
                root.hideWindow();
            else
                root.showWindow();
        }
        function onOpenRequested(request) {
            if (request.simple)
                Documents.simpleOverride = true;
            root.showWindow();
            Documents.openPaths(request.paths, request.line, request.column);
        }
        function onImportRequested(result) {
            root.showWindow();
            Documents.applyImport(result);
        }
    }

    LazyLoader {
        id: windowLoader

        active: Quickshell.env("DUX_EDIT_START_HIDDEN") !== "1"

        MainWindow {
            onCloseRequested: root.closeWindow()
        }
    }
}
