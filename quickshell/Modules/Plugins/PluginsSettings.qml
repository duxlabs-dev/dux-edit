pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Services

// The plugins section of the settings page. Opening it rescans the plugin directories.
Column {
    id: root

    spacing: Style.spacingM

    Component.onCompleted: PluginService.rescan()

    Item {
        width: parent.width
        height: Math.max(intro.implicitHeight, rescanButton.implicitHeight)

        StyledText {
            id: intro

            anchors.left: parent.left
            anchors.right: rescanButton.left
            anchors.rightMargin: Style.spacingM
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("Plugins run with your user's privileges. Only enable plugins you trust.", "plugin settings intro")
            font.pixelSize: Style.fontSizeSmall
            color: Style.surfaceVariantText
            wrapMode: Text.Wrap
        }

        DButton {
            id: rescanButton

            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            variant: "tonal"
            iconName: "refresh"
            text: I18n.tr("Rescan", "button that looks for newly installed plugins")
            busy: PluginService.scanning
            onClicked: PluginService.rescan()
        }
    }

    StyledText {
        width: parent.width
        visible: PluginService.availablePlugins.length === 0 && !PluginService.scanning
        text: I18n.tr("No plugins found. Install plugins in %1.", "plugin settings empty state; %1 is a folder").arg(AppPaths.shortenHome(PluginService.pluginDirs[0] ?? AppPaths.configDir + "/plugins"))
        font.pixelSize: Style.fontSizeMedium
        color: Style.surfaceVariantText
        wrapMode: Text.Wrap
    }

    Column {
        width: parent.width
        spacing: Style.groupedListGap

        Repeater {
            id: rows

            model: PluginService.availablePlugins

            DListRow {
                id: row

                required property var modelData
                required property int index

                readonly property bool enabledInSettings: PluginService.isEnabled(modelData.id)
                readonly property string problem: modelData.error || (PluginService.loadErrors[modelData.id] ?? "")
                readonly property bool showSettings: enabledInSettings && !problem && modelData.settings !== "" && PluginService.loadedPlugins[modelData.id] !== undefined

                function updateSettings() {
                    if (!showSettings) {
                        settingsLoader.source = "";
                        return;
                    }
                    if (settingsLoader.source.toString() !== "")
                        return;
                    settingsLoader.setSource("file://" + modelData.dir + "/" + modelData.settings, {
                        "pluginId": modelData.id,
                        "pluginService": PluginService
                    });
                }

                onShowSettingsChanged: updateSettings()
                Component.onCompleted: updateSettings()

                width: parent.width
                firstInGroup: index === 0
                lastInGroup: index === rows.count - 1
                padding: Style.spacingM
                implicitHeight: Math.max(Style.listItemHeight, body.implicitHeight + topPadding + bottomPadding)

                contentItem: Column {
                    id: body

                    spacing: Style.spacingM

                    Item {
                        width: parent.width
                        height: Math.max(labels.implicitHeight, toggle.implicitHeight)

                        DIcon {
                            id: icon

                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            name: row.modelData.icon || "extension"
                            size: Style.iconSize
                            color: row.problem ? row.supportingContentColor : row.contentColor
                        }

                        Column {
                            id: labels

                            anchors.left: icon.right
                            anchors.leftMargin: Style.spacingM
                            anchors.right: toggle.left
                            anchors.rightMargin: Style.spacingM
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: Style.spacingXXS

                            StyledText {
                                width: parent.width
                                text: row.modelData.name
                                font.pixelSize: Style.fontSizeMedium
                                font.weight: Style.fontWeightMedium
                                color: row.contentColor
                                elide: Text.ElideRight
                            }

                            StyledText {
                                width: parent.width
                                visible: text !== ""
                                text: row.modelData.description ?? ""
                                font.pixelSize: Style.fontSizeSmall
                                color: row.supportingContentColor
                                wrapMode: Text.Wrap
                            }

                            StyledText {
                                width: parent.width
                                visible: row.problem !== ""
                                text: I18n.tr("Cannot load: %1", "plugin settings; %1 is the reason").arg(row.problem)
                                font.pixelSize: Style.fontSizeSmall
                                color: Style.error
                                wrapMode: Text.Wrap
                            }
                        }

                        DToggle {
                            id: toggle

                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            enabled: !row.modelData.error || row.enabledInSettings
                            checked: row.enabledInSettings
                            Accessible.name: row.modelData.name
                            onToggled: isChecked => isChecked ? PluginService.enablePlugin(row.modelData.id) : PluginService.disablePlugin(row.modelData.id)
                        }
                    }

                    Loader {
                        id: settingsLoader

                        width: parent.width
                        visible: status === Loader.Ready
                    }
                }
            }
        }
    }
}
