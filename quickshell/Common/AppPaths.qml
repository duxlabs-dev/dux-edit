pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.DCommon.Common as DC

// The Go core resolves and creates the directories and passes them in the environment.
// Also serves as DC.Paths.backend.
Singleton {
    id: root

    readonly property string home: Quickshell.env("HOME")
    readonly property string configDir: Quickshell.env("DUX_EDIT_CONFIG_DIR")
    readonly property string stateDir: Quickshell.env("DUX_EDIT_STATE_DIR")
    readonly property string cacheDir: Quickshell.env("DUX_EDIT_CACHE_DIR")
    readonly property string settingsFile: configDir + "/settings.json"
    readonly property string sessionFile: stateDir + "/session.json"
    readonly property string pluginSettingsFile: configDir + "/plugin_settings.json"
    readonly property string pluginStateDir: stateDir + "/plugins"

    readonly property url xdgCache: "file://" + (Quickshell.env("XDG_CACHE_HOME") || home + "/.cache")
    readonly property url imagecache: "file://" + cacheDir + "/imagecache"

    // False on a dux-qt-base without iconThemeRevision, whose icon provider fails on "?rev=".
    readonly property bool iconThemeLive: "iconThemeRevision" in Quickshell
    readonly property int iconThemeRevision: iconThemeLive ? Quickshell.iconThemeRevision : 0

    function shortenHome(path: string): string {
        if (path === home)
            return "~";
        if (path.startsWith(home + "/"))
            return "~" + path.slice(home.length);
        return path;
    }

    function baseName(path: string): string {
        return path.slice(path.lastIndexOf("/") + 1);
    }

    function dirName(path: string): string {
        const cut = path.lastIndexOf("/");
        return cut <= 0 ? "/" : path.slice(0, cut);
    }

    function stringify(path: url): string {
        return path.toString().replace(/%20/g, " ");
    }

    function strip(path: url): string {
        return stringify(path).replace("file://", "");
    }

    function resolveIconPath(iconName: string): string {
        if (!iconName)
            return "";
        if (iconName.startsWith("/"))
            return "file://" + iconName;
        return themeIconUrl(iconName);
    }

    // The inline half serves a DCommon that predates themeIconUrl.
    function themeIconUrl(iconName: string): string {
        if (typeof DC.Paths.themeIconUrl === "function")
            return DC.Paths.themeIconUrl(iconName);
        const url = Quickshell.iconPath(iconName, true);
        return url && iconThemeLive ? url + "?rev=" + iconThemeRevision : url;
    }

    function copyPathToClipboard(path: string): void {
        Quickshell.clipboardText = path;
    }
}
