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

    signal closeRequested

    readonly property EditorDocument doc: Documents.current
    readonly property bool hasEditor: doc?.editor ? true : false
    property bool settingsOpen: false

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
            settingsLoader.item?.setSyntaxColor(role, color.toLowerCase());
        });
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

    title: doc ? doc.title + " - " + I18n.tr("Dux Edit", "application name") : I18n.tr("Dux Edit", "application name")
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
    onClosed: window.closeRequested()
    Component.onCompleted: Documents.resume()

    Shortcut {
        sequences: [StandardKey.Save]
        onActivated: Documents.saveCurrent()
    }

    Shortcut {
        sequences: ["Ctrl+Shift+S"]
        onActivated: Documents.saveCurrentAs()
    }

    Shortcut {
        sequences: [StandardKey.Open]
        onActivated: Documents.openWithDialog()
    }

    Shortcut {
        sequences: [StandardKey.New]
        onActivated: Documents.newDraft()
    }

    Shortcut {
        sequences: [StandardKey.Close]
        onActivated: Documents.closeTab(Documents.currentIndex, false)
    }

    Shortcut {
        sequences: ["Ctrl+Tab", "Ctrl+PgDown"]
        onActivated: window.cycleTab(1)
    }

    Shortcut {
        sequences: ["Ctrl+Shift+Tab", "Ctrl+Shift+Backtab", "Ctrl+PgUp"]
        onActivated: window.cycleTab(-1)
    }

    Shortcut {
        sequences: ["Ctrl+F"]
        onActivated: window.openFind(false)
    }

    Shortcut {
        sequences: ["Ctrl+H"]
        onActivated: window.openFind(true)
    }

    Shortcut {
        sequences: ["Ctrl+G"]
        onActivated: window.goToLine()
    }

    Shortcut {
        sequences: ["Ctrl+P"]
        onActivated: window.cyclePreview()
    }

    Shortcut {
        sequences: ["Ctrl++", "Ctrl+="]
        onActivated: window.zoom(1)
    }

    Shortcut {
        sequences: ["Ctrl+-"]
        onActivated: window.zoom(-1)
    }

    Shortcut {
        sequences: ["Ctrl+0"]
        onActivated: window.zoom(0)
    }

    Shortcut {
        sequences: ["Ctrl+,"]
        onActivated: window.settingsOpen = !window.settingsOpen
    }

    Instantiator {
        model: PluginService.commands.filter(command => !!command.shortcut)

        Shortcut {
            required property var modelData

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
            onCloseRequested: window.closeRequested()

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

        Item {
            id: content

            anchors.top: banner.bottom
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

        FileMenu {
            id: windowMenu

            FileMenuItem {
                iconName: "save_as"
                enabled: window.doc?.loaded ?? false
                text: I18n.tr("Save As…", "window menu item")
                onClicked: {
                    windowMenu.close();
                    Documents.saveCurrentAs();
                }
            }

            FileMenuDivider {}

            FileMenuItem {
                iconName: "search"
                enabled: window.hasEditor
                text: I18n.tr("Find", "window menu item")
                onClicked: {
                    windowMenu.close();
                    window.openFind(false);
                }
            }

            FileMenuItem {
                iconName: "find_replace"
                enabled: window.hasEditor
                text: I18n.tr("Replace", "window menu item")
                onClicked: {
                    windowMenu.close();
                    window.openFind(true);
                }
            }

            FileMenuItem {
                iconName: "format_list_numbered"
                enabled: window.hasEditor
                text: I18n.tr("Go to Line…", "window menu item")
                onClicked: {
                    windowMenu.close();
                    window.goToLine();
                }
            }

            FileMenuDivider {}

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
                onClicked: {
                    windowMenu.close();
                    window.settingsOpen = true;
                }
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
                    onClicked: {
                        windowMenu.close();
                        modelData.run?.();
                    }
                }
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
