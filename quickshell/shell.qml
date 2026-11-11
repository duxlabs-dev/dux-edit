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

    Component.onCompleted: Quickshell.watchFiles = Quickshell.env("DUX_EDIT_DISABLE_HOT_RELOAD") !== "1"

    Connections {
        target: Backend
        function onShowRequested() {
            root.showWindow();
        }
    }

    LazyLoader {
        id: windowLoader
        active: Quickshell.env("DUX_EDIT_START_HIDDEN") !== "1"

        MainWindow {}
    }
}
