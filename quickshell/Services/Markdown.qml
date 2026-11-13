pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services

Singleton {
    id: root

    // callback({ok, html, version}) or ({ok: false, code: "TOOLARGE" | "DESYNC"}).
    // palette maps role names to "#rrggbb" so the core never needs the theme.
    function render(doc, palette, codeBackground, linkColor, callback) {
        Backend.call("markdown.render", {
            "docId": doc.docId,
            "version": doc.version,
            "palette": palette,
            "codeBackground": codeBackground,
            "linkColor": linkColor
        }, callback);
    }
}
