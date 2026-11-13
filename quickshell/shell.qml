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

    function showWindow() {
        if (!windowLoader.active) {
            windowLoader.active = true;
            return;
        }
        // Looked up on demand: a binding would wake the UI for every window change on the desktop.
        const ownToplevel = ToplevelManager.toplevels.values.find(toplevel => toplevel.appId === appId) ?? null;
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
        if (Backend.lost && Documents.unsafeDocs().length > 0) {
            // The editors hold the only copy now; they must outlive the window's close button.
            ToastService.showError(I18n.tr("Dux Edit lost the connection to its core. Copy your unsaved work elsewhere before closing.", "error when the window is closed while nothing can be saved"));
            return;
        }
        Documents.suspend(null);
        windowLoader.active = false;
    }

    // gone: the compositor already closed the window, so a question needs it back.
    function closeWindow(gone) {
        if (SettingsData.closeBehavior === "background" && !(gone && Backend.lost)) {
            hideWindow();
            return;
        }
        if (gone && Documents.unsafeDocs().length > 0)
            reviveWindow();
        quitApp();
    }

    function quitApp() {
        Documents.confirmQuit(() => root.quit(Backend.lost));
    }

    // A compositor close takes the surface and leaves the window object, with its editors.
    function reviveWindow() {
        if (!windowLoader.item)
            return;
        windowLoader.item.visible = false;
        windowLoader.item.visible = true;
    }

    // Quits once every unsaved document is in its swap; otherwise the window comes back to ask first.
    function quit(discard) {
        Documents.suspend(failed => {
            if (failed.length === 0 || discard) {
                Backend.quit();
                return;
            }
            reviveWindow();
            Documents.resume();
            Documents.confirmQuit(() => root.quit(true));
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
            onCloseRequested: gone => root.closeWindow(gone)
            onQuitRequested: root.quitApp()
        }
    }
}
