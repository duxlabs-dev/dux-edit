pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.DCommon.Common
import qs.DCommon.FileBrowser
import qs.DCommon.Widgets
import qs.Modules.Dialogs
import qs.Modules.Editor
import qs.Services

// Header, tab strip, conflict banner, one editor pane per loaded tab, status bar, settings page and dialogs.
FloatingWindow {
    id: window

    // gone is true when the compositor has already closed the window.
    signal closeRequested(bool gone)
    signal quitRequested

    readonly property EditorDocument doc: Documents.current
    readonly property bool hasEditor: doc?.editor ? true : false
    readonly property bool canEdit: hasEditor && !doc.readOnly
    property bool settingsOpen: false
    // An open dialog owns the keyboard; Ctrl+W or Ctrl+Q behind it would act on what it asks about.
    readonly property bool dialogOpen: prompt.opened || inputDialog.opened
    readonly property int recentLimit: 10
    readonly property real menuWidth: Style.spacingXL * 12

    function canCall(name) {
        return typeof doc?.editor?.[name] === "function";
    }

    function callPane(name) {
        if (!canCall(name))
            return;
        settingsOpen = false;
        doc.editor[name]();
    }

    function toggleWrap() {
        if (!canCall("toggleWrap"))
            return;
        doc.editor.toggleWrap();
        Documents.storeSession();
    }

    function menuAction(menu, id) {
        menu.close();
        runAction(id);
    }

    // A menu closing leaves the focus on the window, so an edit made from it gives the focus back.
    function editAction(menu, id) {
        menuAction(menu, id);
        doc?.editor?.focusEditor();
    }

    function openSubmenu(menu) {
        const x = windowMenu.x;
        const y = windowMenu.y;
        windowMenu.close();
        menu.openAt(x, y);
    }

    function droppedPaths(urls) {
        const paths = [];
        for (const url of urls) {
            const text = url.toString();
            const path = text.startsWith("file:///") ? decodeURIComponent(text.substring(7)) : "";
            if (path !== "")
                paths.push(path);
        }
        return paths;
    }

    function cyclePreview() {
        if (!doc?.isMarkdown)
            return;
        const modes = ["off", "split", "full"];
        doc.previewMode = modes[(modes.indexOf(doc.previewMode) + 1) % modes.length];
        doc.previewChosen = true;
        Documents.storeSession();
    }

    function cycleTab(step) {
        const count = Documents.tabs.length;
        if (count < 2)
            return;
        Documents.activate((Documents.currentIndex + step + count) % count);
    }

    function openFind(withReplace) {
        if (!hasEditor)
            return;
        settingsOpen = false;
        doc.editor.openFind(withReplace);
    }

    function goToLine() {
        if (!hasEditor)
            return;
        settingsOpen = false;
        const target = doc;
        inputDialog.acceptLabel = I18n.tr("Go", "button that moves the cursor to a line");
        inputDialog.placeholderText = I18n.tr("Line or line:column", "placeholder of the go to line field");
        inputDialog.show(I18n.tr("Go to Line", "dialog title"), String(target.cursorLine), text => {
            const parts = text.split(/[:,]/).map(part => parseInt(part.trim(), 10));
            if (!(parts[0] > 0) || !target.editor)
                return;
            target.editor.gotoLine(parts[0], parts[1] > 0 ? parts[1] : 0);
        });
    }

    function renameDraft(target) {
        inputDialog.acceptLabel = I18n.tr("Rename", "button that renames a document");
        inputDialog.placeholderText = "";
        inputDialog.show(I18n.tr("Rename Document", "dialog title"), target.title, text => {
            target.setTitle(text.trim());
            Documents.storeSession();
        });
    }

    function editSyntaxColor(role, label, current) {
        inputDialog.acceptLabel = I18n.tr("Apply", "button that applies a color");
        inputDialog.placeholderText = "#rrggbb";
        inputDialog.show(I18n.tr("Color for %1", "dialog title; %1 is a syntax role such as keyword").arg(label), current, text => {
            const color = text.trim();
            if (!/^#[0-9a-fA-F]{6}$/.test(color)) {
                ToastService.showError(I18n.tr("Use a color like #3366cc.", "error for a malformed color"));
                return;
            }
            SettingsData.setSyntaxColor(role, color.toLowerCase());
        });
    }

    function runAction(id) {
        switch (id) {
        case "new":
            Documents.newDraft();
            return;
        case "open":
            Documents.openWithDialog();
            return;
        case "save":
            Documents.saveCurrent();
            return;
        case "saveAs":
            Documents.saveCurrentAs();
            return;
        case "quit":
            quitRequested();
            return;
        case "closeTab":
            Documents.closeTab(Documents.currentIndex, false);
            return;
        case "reopenTab":
            Documents.reopenClosedTab();
            return;
        case "nextTab":
            cycleTab(1);
            return;
        case "previousTab":
            cycleTab(-1);
            return;
        case "moveTabLeft":
            Documents.moveCurrent(-1);
            return;
        case "moveTabRight":
            Documents.moveCurrent(1);
            return;
        case "tab1":
        case "tab2":
        case "tab3":
        case "tab4":
        case "tab5":
        case "tab6":
        case "tab7":
        case "tab8":
            Documents.activate(parseInt(id.substring(3), 10) - 1);
            return;
        case "lastTab":
            Documents.activate(Documents.tabs.length - 1);
            return;
        case "undo":
        case "redo":
        case "copyAll":
        case "findNext":
        case "findPrevious":
        case "deleteLines":
        case "duplicate":
        case "moveLinesUp":
        case "moveLinesDown":
        case "selectLine":
        case "joinLines":
        case "sortLines":
        case "upperCase":
        case "lowerCase":
        case "titleCase":
            callPane(id);
            return;
        case "toggleWrap":
            toggleWrap();
            return;
        case "fullscreen":
            window.fullscreen = !window.fullscreen;
            return;
        case "discardChanges":
            Documents.discardChanges(doc);
            return;
        case "find":
            openFind(false);
            return;
        case "replace":
            openFind(true);
            return;
        case "goToLine":
            goToLine();
            return;
        case "preview":
            cyclePreview();
            return;
        case "zoomIn":
            zoom(1);
            return;
        case "zoomOut":
            zoom(-1);
            return;
        case "zoomReset":
            zoom(0);
            return;
        case "settings":
            settingsOpen = !settingsOpen;
            return;
        }
    }

    function toggleSimple() {
        if (Documents.simple) {
            Documents.simpleOverride = false;
            SettingsData.viewMode = "advanced";
            return;
        }
        SettingsData.viewMode = "simple";
    }

    function zoom(step) {
        SettingsData.setFontSize(step === 0 ? SettingsData.defaultFontSize : SettingsData.fontSize + step);
    }

    title: doc ? doc.title : I18n.tr("Dux Edit", "application name")
    implicitWidth: SessionData.windowWidth
    implicitHeight: SessionData.windowHeight
    minimumSize: Qt.size(EditorMetrics.windowMinWidth, EditorMetrics.windowMinHeight)
    color: Style.hostSurface

    onWidthChanged: {
        if (width > 0)
            SessionData.windowWidth = Math.round(width);
    }
    onHeightChanged: {
        if (height > 0)
            SessionData.windowHeight = Math.round(height);
    }
    onClosed: window.closeRequested(true)
    Component.onCompleted: Documents.resume()

    Instantiator {
        model: Shortcuts.bound

        Shortcut {
            required property var modelData

            enabled: !window.dialogOpen
            sequences: Shortcuts.sequences(modelData)
            onActivated: window.runAction(modelData.id)
        }
    }

    Instantiator {
        model: PluginService.commands.filter(command => !!command.shortcut)

        Shortcut {
            required property var modelData

            enabled: !window.dialogOpen
            sequences: [modelData.shortcut]
            onActivated: modelData.run?.()
        }
    }

    Connections {
        target: Documents

        function onCloseConfirmationNeeded(doc) {
            prompt.ask({
                "title": I18n.tr("Save Changes to %1?", "dialog title; %1 is a document name").arg(doc.title),
                "message": I18n.tr("Your changes will be lost if you don't save them.", "dialog message when closing a tab with unsaved changes"),
                "actions": [
                    {
                        "label": I18n.tr("Don't Save", "button that closes a tab and drops its changes"),
                        "danger": true,
                        "run": () => Documents.closeTab(Documents.tabs.indexOf(doc), true)
                    },
                    {
                        "label": I18n.tr("Save", "button that saves a document"),
                        "run": () => Documents.saveAndClose(doc)
                    }
                ]
            });
        }

        function onLargeFileConfirmationNeeded(doc, size) {
            prompt.ask({
                "title": I18n.tr("Open Large File?", "dialog title"),
                "message": I18n.tr("%1 is %2 MB. Large files use a lot of memory and open without highlighting.", "dialog message; %1 is a file name, %2 a size").arg(doc.title).arg((size / 1048576).toFixed(1)),
                "actions": [
                    {
                        "label": I18n.tr("Open", "button that opens a file"),
                        "run": () => Documents.confirmLargeFile(doc)
                    }
                ],
                "cancel": () => Documents.removeTab(doc)
            });
        }

        function onPromptRequested(request) {
            prompt.ask(request);
        }
    }

    FloatingWindowControls {
        id: windowControls
        targetWindow: window
    }

    Item {
        id: root

        anchors.fill: parent

        DWindowHeader {
            id: header

            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            controls: windowControls
            title: window.title
            titleAlignment: Text.AlignLeft
            onCloseRequested: window.closeRequested(false)

            DActionButton {
                anchors.verticalCenter: parent.verticalCenter
                iconName: "add"
                tooltipText: I18n.tr("New", "tooltip on the button creating a document")
                onClicked: Documents.newDraft()
            }

            DActionButton {
                anchors.verticalCenter: parent.verticalCenter
                iconName: "folder_open"
                tooltipText: I18n.tr("Open", "tooltip on the button opening files")
                onClicked: Documents.openWithDialog()
            }

            DActionButton {
                anchors.verticalCenter: parent.verticalCenter
                iconName: "save"
                enabled: window.doc?.loaded ?? false
                tooltipText: I18n.tr("Save", "tooltip on the save button")
                onClicked: Documents.saveCurrent()
            }

            DActionButton {
                anchors.verticalCenter: parent.verticalCenter
                visible: window.doc?.isMarkdown ?? false
                iconName: window.doc?.previewMode === "off" ? "visibility" : "visibility_off"
                tooltipText: I18n.tr("Preview", "tooltip on the button cycling the Markdown preview")
                onClicked: window.cyclePreview()
            }

            DActionButton {
                id: menuButton

                anchors.verticalCenter: parent.verticalCenter
                iconName: "more_vert"
                tooltipText: I18n.tr("Menu", "tooltip on the window menu button")
                onClicked: {
                    const point = menuButton.mapToItem(root, menuButton.width, menuButton.height);
                    windowMenu.openAt(point.x - windowMenu.width, point.y);
                }
            }
        }

        TabStrip {
            id: tabStrip

            anchors.top: header.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.leftMargin: Style.spacingS
            anchors.rightMargin: Style.spacingS
            visible: !(Documents.simple && Documents.tabs.length === 1)
            height: visible ? implicitHeight : 0
            onRenameRequested: target => window.renameDraft(target)
        }

        ConflictBanner {
            id: banner

            anchors.top: tabStrip.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            doc: window.doc
        }

        Rectangle {
            id: lostBanner

            anchors.top: banner.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            visible: Backend.lost
            implicitHeight: visible ? lostText.implicitHeight + Style.spacingS * 2 : 0
            height: implicitHeight
            color: Style.errorContainer

            StyledText {
                id: lostText

                anchors.left: parent.left
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                anchors.leftMargin: Style.spacingM
                anchors.rightMargin: Style.spacingM
                text: I18n.tr("Dux Edit lost the connection to its core. Nothing can be saved: copy your unsaved work elsewhere, then restart the app.", "banner shown when the backend process died")
                color: Style.onErrorContainer
                font.pixelSize: Style.fontSizeMedium
                wrapMode: Text.WordWrap
            }
        }

        Item {
            id: content

            anchors.top: lostBanner.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: status.top

            Repeater {
                model: Documents.tabModel

                Loader {
                    id: paneLoader

                    required property string tabId
                    required property int index
                    readonly property EditorDocument doc: Documents.byTabId(tabId)

                    anchors.fill: parent
                    // An editor holds roughly 80 MB per MB of text, so tabs load on first use.
                    active: paneLoader.doc !== null && (paneLoader.doc.loaded || paneLoader.doc.loading)
                    visible: paneLoader.index === Documents.currentIndex
                    onLoaded: paneLoader.focusIfShown()
                    onVisibleChanged: paneLoader.focusIfShown()

                    function focusIfShown() {
                        if (!paneLoader.visible || window.settingsOpen)
                            return;
                        paneLoader.item?.focusEditor();
                    }

                    sourceComponent: EditorPane {
                        doc: paneLoader.doc
                    }
                }
            }
        }

        StatusBar {
            id: status

            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            doc: window.doc
            simple: Documents.simple
        }

        Loader {
            id: settingsLoader

            anchors.top: header.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            active: window.settingsOpen
            onLoaded: item.forceActiveFocus()
            onActiveChanged: {
                if (!active)
                    window.doc?.editor?.focusEditor();
            }

            sourceComponent: SettingsView {
                onCloseRequested: window.settingsOpen = false
                onColorEditRequested: (role, label, current) => window.editSyntaxColor(role, label, current)
            }
        }

        // Above the editors, so a dragged file opens instead of landing in the text as its URL.
        DropArea {
            id: fileDrop

            anchors.top: header.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            keys: ["text/uri-list"]
            onEntered: drag => {
                if (window.droppedPaths(drag.urls).length === 0)
                    drag.accepted = false;
            }
            onDropped: drop => {
                const paths = window.droppedPaths(drop.urls);
                if (paths.length === 0)
                    return;
                drop.acceptProposedAction();
                window.settingsOpen = false;
                Documents.openPaths(paths, 0, 0);
            }

            Rectangle {
                anchors.fill: parent
                visible: fileDrop.containsDrag
                color: Style.primaryHover
                border.width: Style.focusRingWidth
                border.color: Style.primary
            }
        }

        FileMenu {
            id: windowMenu

            width: window.menuWidth

            FileMenuItem {
                iconName: "undo"
                enabled: window.hasEditor && window.doc.editor.canUndo
                text: I18n.tr("Undo", "window menu item")
                shortcutText: Shortcuts.hint("undo")
                onClicked: window.editAction(windowMenu, "undo")
            }

            FileMenuItem {
                iconName: "redo"
                enabled: window.hasEditor && window.doc.editor.canRedo
                text: I18n.tr("Redo", "window menu item")
                shortcutText: Shortcuts.hint("redo")
                onClicked: window.editAction(windowMenu, "redo")
            }

            FileMenuItem {
                iconName: "content_copy"
                enabled: window.hasEditor && window.canCall("copyAll")
                text: I18n.tr("Copy All", "window menu item that copies the whole document")
                shortcutText: Shortcuts.hint("copyAll")
                onClicked: window.editAction(windowMenu, "copyAll")
            }

            FileMenuItem {
                iconName: "format_list_bulleted"
                enabled: window.canEdit
                submenu: true
                text: I18n.tr("Lines and Case", "window menu item opening line and letter case commands")
                onClicked: window.openSubmenu(linesMenu)
            }

            FileMenuDivider {}

            FileMenuItem {
                iconName: "history"
                enabled: SessionData.recentFiles.length > 0
                submenu: true
                text: I18n.tr("Open Recent", "window menu item")
                onClicked: window.openSubmenu(recentMenu)
            }

            FileMenuItem {
                iconName: "tab"
                enabled: Documents.closedTabs.length > 0
                text: I18n.tr("Reopen Closed Tab", "window menu item")
                shortcutText: Shortcuts.hint("reopenTab")
                onClicked: window.menuAction(windowMenu, "reopenTab")
            }

            FileMenuItem {
                iconName: "save_as"
                enabled: window.doc?.loaded ?? false
                text: I18n.tr("Save As…", "window menu item")
                shortcutText: Shortcuts.hint("saveAs")
                onClicked: window.menuAction(windowMenu, "saveAs")
            }

            FileMenuItem {
                iconName: "restore_page"
                visible: window.doc !== null && window.doc.loaded && !window.doc.isDraft && window.doc.dirty
                danger: true
                text: I18n.tr("Discard Changes…", "window menu item that reloads the file and drops unsaved edits")
                onClicked: window.menuAction(windowMenu, "discardChanges")
            }

            FileMenuDivider {}

            FileMenuItem {
                iconName: "search"
                enabled: window.hasEditor
                text: I18n.tr("Find", "window menu item")
                shortcutText: Shortcuts.hint("find")
                onClicked: window.menuAction(windowMenu, "find")
            }

            FileMenuItem {
                iconName: "find_replace"
                enabled: window.hasEditor
                text: I18n.tr("Replace", "window menu item")
                shortcutText: Shortcuts.hint("replace")
                onClicked: window.menuAction(windowMenu, "replace")
            }

            FileMenuItem {
                iconName: "format_list_numbered"
                enabled: window.hasEditor
                text: I18n.tr("Go to Line…", "window menu item")
                shortcutText: Shortcuts.hint("goToLine")
                onClicked: window.menuAction(windowMenu, "goToLine")
            }

            FileMenuDivider {}

            FileMenuItem {
                iconName: "wrap_text"
                enabled: window.hasEditor && window.canCall("toggleWrap")
                checked: window.doc?.editor?.wrapping ?? false
                text: I18n.tr("Word Wrap", "window menu item")
                shortcutText: Shortcuts.hint("toggleWrap")
                onClicked: window.editAction(windowMenu, "toggleWrap")
            }

            FileMenuItem {
                iconName: "fullscreen"
                checked: window.fullscreen
                text: I18n.tr("Full Screen", "window menu item")
                shortcutText: Shortcuts.hint("fullscreen")
                onClicked: window.menuAction(windowMenu, "fullscreen")
            }

            FileMenuItem {
                iconName: "view_agenda"
                checked: Documents.simple
                text: I18n.tr("Simple Mode", "window menu item")
                onClicked: {
                    windowMenu.close();
                    window.toggleSimple();
                }
            }

            FileMenuItem {
                iconName: "settings"
                text: I18n.tr("Settings", "window menu item")
                shortcutText: Shortcuts.hint("settings")
                onClicked: {
                    windowMenu.close();
                    window.settingsOpen = true;
                }
            }

            FileMenuItem {
                iconName: "power_settings_new"
                text: I18n.tr("Quit", "window menu item")
                shortcutText: Shortcuts.hint("quit")
                onClicked: window.menuAction(windowMenu, "quit")
            }

            FileMenuDivider {
                visible: PluginService.commands.length > 0
            }

            Repeater {
                model: PluginService.commands

                FileMenuItem {
                    required property var modelData

                    iconName: modelData.icon || "extension"
                    text: modelData.title
                    shortcutText: modelData.shortcut || ""
                    onClicked: {
                        windowMenu.close();
                        modelData.run?.();
                    }
                }
            }
        }

        FileMenu {
            id: recentMenu

            width: window.menuWidth
            parentMenu: windowMenu

            Repeater {
                model: SessionData.recentFiles.slice(0, window.recentLimit)

                FileMenuItem {
                    required property string modelData

                    iconName: "description"
                    text: AppPaths.baseName(modelData) + "  ·  " + AppPaths.shortenHome(AppPaths.dirName(modelData))
                    onClicked: {
                        recentMenu.close();
                        window.settingsOpen = false;
                        Documents.openPaths([modelData], 0, 0, true);
                    }
                }
            }

            FileMenuDivider {}

            FileMenuItem {
                iconName: "delete_sweep"
                text: I18n.tr("Clear Recent", "menu item that forgets the recent files")
                onClicked: {
                    recentMenu.close();
                    SessionData.clearRecent();
                }
            }
        }

        FileMenu {
            id: linesMenu

            width: window.menuWidth
            parentMenu: windowMenu

            FileMenuItem {
                iconName: "content_copy"
                enabled: window.canEdit && window.canCall("duplicate")
                text: I18n.tr("Duplicate Line", "window menu item")
                shortcutText: Shortcuts.hint("duplicateLine")
                onClicked: window.editAction(linesMenu, "duplicate")
            }

            FileMenuItem {
                iconName: "backspace"
                enabled: window.canEdit && window.canCall("deleteLines")
                text: I18n.tr("Delete Line", "window menu item")
                shortcutText: Shortcuts.hint("deleteLine")
                onClicked: window.editAction(linesMenu, "deleteLines")
            }

            FileMenuItem {
                iconName: "arrow_upward"
                enabled: window.canEdit && window.canCall("moveLinesUp")
                text: I18n.tr("Move Lines Up", "window menu item")
                shortcutText: Shortcuts.hint("moveLinesUp")
                onClicked: window.editAction(linesMenu, "moveLinesUp")
            }

            FileMenuItem {
                iconName: "arrow_downward"
                enabled: window.canEdit && window.canCall("moveLinesDown")
                text: I18n.tr("Move Lines Down", "window menu item")
                shortcutText: Shortcuts.hint("moveLinesDown")
                onClicked: window.editAction(linesMenu, "moveLinesDown")
            }

            FileMenuItem {
                iconName: "select_all"
                enabled: window.hasEditor && window.canCall("selectLine")
                text: I18n.tr("Select Line", "window menu item")
                shortcutText: Shortcuts.hint("selectLine")
                onClicked: window.editAction(linesMenu, "selectLine")
            }

            FileMenuItem {
                iconName: "vertical_align_center"
                enabled: window.canEdit && window.canCall("joinLines")
                text: I18n.tr("Join Lines", "window menu item")
                shortcutText: Shortcuts.hint("joinLines")
                onClicked: window.editAction(linesMenu, "joinLines")
            }

            FileMenuItem {
                iconName: "sort"
                enabled: window.canEdit && window.canCall("sortLines")
                text: I18n.tr("Sort Lines", "window menu item")
                onClicked: window.editAction(linesMenu, "sortLines")
            }

            FileMenuDivider {}

            FileMenuItem {
                iconName: "text_fields"
                enabled: window.canEdit && window.canCall("upperCase")
                text: I18n.tr("UPPERCASE", "window menu item that changes the selection to upper case")
                onClicked: window.editAction(linesMenu, "upperCase")
            }

            FileMenuItem {
                iconName: "text_fields"
                enabled: window.canEdit && window.canCall("lowerCase")
                text: I18n.tr("lowercase", "window menu item that changes the selection to lower case")
                onClicked: window.editAction(linesMenu, "lowerCase")
            }

            FileMenuItem {
                iconName: "text_fields"
                enabled: window.canEdit && window.canCall("titleCase")
                text: I18n.tr("Title Case", "window menu item that capitalizes each word of the selection")
                onClicked: window.editAction(linesMenu, "titleCase")
            }
        }

        Toast {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: status.top
            height: Style.fieldHeightLarge + Style.spacingL * 2
        }

        PromptDialog {
            id: prompt

            anchors.fill: parent
            z: 2000
        }

        InputDialog {
            id: inputDialog

            anchors.fill: parent
            z: 2000
        }
    }
}
