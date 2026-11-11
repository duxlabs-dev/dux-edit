pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell

// Leveled console logging; DUX_EDIT_LOG_LEVEL picks the threshold. Also serves as DC.Log.backend.
// Never log document text.
Singleton {
    id: root

    enum Level {
        Debug,
        Info,
        Warn,
        Error
    }

    readonly property int level: _parseLevel(Quickshell.env("DUX_EDIT_LOG_LEVEL"))

    function scoped(module) {
        return {
            "debug": (...args) => root._emit(Log.Level.Debug, module, args),
            "info": (...args) => root._emit(Log.Level.Info, module, args),
            "warn": (...args) => root._emit(Log.Level.Warn, module, args),
            "error": (...args) => root._emit(Log.Level.Error, module, args)
        };
    }

    function _parseLevel(name) {
        switch ((name || "").toLowerCase()) {
        case "debug":
            return Log.Level.Debug;
        case "warn":
        case "warning":
            return Log.Level.Warn;
        case "error":
            return Log.Level.Error;
        default:
            return Log.Level.Info;
        }
    }

    function _stringify(value) {
        if (typeof value === "string")
            return value;
        if (value instanceof Error)
            return value.toString();
        try {
            return JSON.stringify(value);
        } catch (e) {
            return String(value);
        }
    }

    function _emit(lvl, module, args) {
        if (lvl < level)
            return;
        const body = `[${module}] ` + args.map(_stringify).join(" ");
        switch (lvl) {
        case Log.Level.Debug:
            console.debug(body);
            return;
        case Log.Level.Info:
            console.info(body);
            return;
        case Log.Level.Warn:
            console.warn(body);
            return;
        default:
            console.error(body);
        }
    }
}
