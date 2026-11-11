pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Services
import "../../Common/SyntaxPalette.js" as SyntaxPalette

// The rendered Markdown beside or instead of the editor. The core renders from its mirror; one request is
// in flight at a time and a render that returns behind the document is followed by another.
Item {
    id: root

    required property var doc

    property string html: ""
    property string message: ""
    property bool rendering: false
    property bool stale: true

    readonly property bool shown: visible
    readonly property var paletteNames: Highlight.paletteNames
    readonly property string codeBackground: SyntaxPalette.toHex(Style.chipSurface)
    readonly property string linkColor: SyntaxPalette.toHex(Style.primary)
    readonly property string directory: root.doc.path.substring(0, root.doc.path.lastIndexOf("/"))

    function documentEdited() {
        stale = true;
        if (shown)
            renderTimer.restart();
    }

    function restyle() {
        if (html !== "")
            documentEdited();
    }

    function render() {
        if (!shown || rendering)
            return;
        stale = false;
        rendering = true;
        const doc = root.doc;
        const version = doc.version;
        Markdown.render(doc, paletteNames, codeBackground, linkColor, result => {
            // The preview may be gone, or showing another document, by the time the answer arrives.
            if (root.doc !== doc)
                return;
            root.rendering = false;
            root.apply(result);
            if (doc.version !== version)
                root.stale = true;
            if (root.stale && !renderTimer.running)
                root.render();
        });
    }

    function apply(result) {
        if (result.ok === true) {
            html = result.html;
            message = "";
            return;
        }
        switch (result.code) {
        case "TOOLARGE":
            html = "";
            message = I18n.tr("This document is too large to preview", "markdown preview");
            return;
        case "DESYNC":
            return;
        }
        html = "";
        message = I18n.tr("The preview is not available", "markdown preview, rendering failed");
    }

    function resolve(relative) {
        const parts = (directory + "/" + decodeURIComponent(relative)).split("/");
        const out = [];
        for (const part of parts) {
            switch (part) {
            case "":
            case ".":
                continue;
            case "..":
                out.pop();
                continue;
            }
            out.push(part);
        }
        return "/" + out.join("/");
    }

    function openLink(link) {
        if (link.startsWith("task:")) {
            // The line number is from the text that was rendered; after an edit it may name another task.
            if (!stale && !rendering)
                root.doc.editor?.toggleTask(parseInt(link.substring(5)));
            return;
        }
        if (/^(https?|mailto):/i.test(link)) {
            Qt.openUrlExternally(link);
            return;
        }
        const target = link.split("#")[0];
        if (!/\.(md|markdown)$/i.test(target))
            return;
        if (target.startsWith("file://")) {
            Documents.openPaths([decodeURIComponent(target.substring(7))], 0, 0);
            return;
        }
        if (directory === "" || /^[a-z][a-z0-9+.-]*:/i.test(target))
            return;
        Documents.openPaths([target.startsWith("/") ? target : resolve(target)], 0, 0);
    }

    function copy(text, done) {
        Quickshell.clipboardText = text;
        ToastService.showInfo(done);
    }

    onShownChanged: {
        if (shown && stale)
            render();
    }
    onPaletteNamesChanged: restyle()
    onCodeBackgroundChanged: restyle()
    onLinkColorChanged: restyle()
    Component.onCompleted: render()

    Timer {
        id: renderTimer
        interval: EditorMetrics.previewDelay
        onTriggered: root.render()
    }

    Row {
        id: header

        anchors.top: parent.top
        anchors.right: parent.right
        anchors.margins: Style.spacingS
        spacing: Style.spacingXS

        DButton {
            text: I18n.tr("Copy Text", "markdown preview button")
            variant: "text"
            iconName: "content_copy"
            enabled: root.html !== ""
            onClicked: root.copy(previewText.getText(0, previewText.length), I18n.tr("Text copied", "toast after copying the preview text"))
        }

        DButton {
            text: I18n.tr("Copy HTML", "markdown preview button")
            variant: "text"
            iconName: "code"
            enabled: root.html !== ""
            onClicked: root.copy(root.html, I18n.tr("HTML copied", "toast after copying the preview HTML"))
        }
    }

    StyledText {
        anchors.centerIn: parent
        width: parent.width - Style.spacingXL * 2
        visible: root.message !== ""
        text: root.message
        color: Style.onSurfaceVariant
        horizontalAlignment: Text.AlignHCenter
    }

    DFlickable {
        id: flickable

        anchors.top: header.bottom
        anchors.topMargin: Style.spacingS
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        clip: true
        visible: root.message === ""
        contentWidth: width
        contentHeight: previewText.height

        TextEdit {
            id: previewText

            width: flickable.width
            padding: EditorMetrics.padding
            readOnly: true
            selectByMouse: true
            textFormat: TextEdit.RichText
            wrapMode: TextEdit.Wrap
            text: root.html
            baseUrl: root.directory !== "" ? "file://" + encodeURI(root.directory) + "/" : ""
            color: Style.onSurface
            selectionColor: Style.primary
            selectedTextColor: Style.onPrimary
            font.family: Style.fontFamily
            font.pixelSize: SettingsData.fontSize
            onLinkActivated: link => root.openLink(link)

            HoverHandler {
                cursorShape: previewText.hoveredLink !== "" ? Qt.PointingHandCursor : Qt.IBeamCursor
            }
        }
    }
}
