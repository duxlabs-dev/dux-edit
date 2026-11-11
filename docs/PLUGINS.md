# Dux Edit plugins

A Dux Edit plugin is a folder with a `plugin.json` manifest and QML files. It uses the same manifest format, storage calls and settings controls as DMS plugins, with one new surface: `editor`. DMS plugins written for the bar, control center, desktop, dashboard or launcher do not load here.

A plugin is arbitrary QML that runs with your user's privileges. Manifest permissions are advisory, not a sandbox. Only install plugins you trust.

## Where plugins live

| Directory | Use |
|---|---|
| `~/.config/dux/edit/plugins/<folder>/` | your plugins (`$XDG_CONFIG_HOME` is honored) |
| `/usr/share/dux/edit/plugins/<folder>/` | plugins installed by packages |

When both hold the same `id`, the one in your config directory wins. Dux Edit never watches these folders: it scans them when the Plugins settings section opens, when you press Rescan, and at startup when at least one plugin is enabled. Enabling a plugin in settings loads it at once; disabling destroys it.

## Manifest

```json
{
  "id": "dateTools",
  "name": "Date Tools",
  "description": "Insert today's date and count words",
  "version": "1.0.0",
  "author": "Your Name",
  "icon": "calendar_today",
  "components": { "editor": "./DateTools.qml" },
  "settings": "./DateToolsSettings.qml",
  "permissions": ["settings_read", "settings_write"],
  "requires_dux_edit": ">=0.1.0"
}
```

| Field | Notes |
|---|---|
| `id` | Required. Letters and digits, starting with a letter (`^[a-zA-Z][a-zA-Z0-9]*$`). It names the plugin's state file. |
| `name`, `description`, `version`, `author` | Shown in settings. `name` falls back to the id. |
| `icon` | A Material Symbols name. |
| `components.editor` | The main QML file. Its root must be `EditorPluginComponent`. |
| `type: "editor"` with `component` | The single-surface form of the same thing. |
| `settings` | Optional settings QML file. Its root must be `PluginSettings`. |
| `permissions` | Listed for users to read. Not enforced. |

Component paths are relative to the plugin folder and must stay inside it. A manifest that cannot load is still listed in settings with the reason: invalid JSON, an invalid id, a path that leaves the folder, no editor surface (for example a DMS bar widget), or the id `dankNotepadModule`, whose features are built into Dux Edit.

## API

### `qs.Modules.Plugins`

| Type | Properties |
|---|---|
| `EditorPluginComponent` | Set by the host: `pluginId`, `pluginService`, `editor`. Yours to set: `commands`, `statusItem`, `sidePanel`. |
| `PluginSettings` | `pluginId` (required), `pluginService`; functions `saveValue(key, value)`, `loadValue(key, default)`, `saveState`, `loadState`, `clearState`; signal `settingChanged`. Setting controls placed anywhere inside it read and write the plugin's data. |
| `StringSetting` | `settingKey`, `label`, `description`, `placeholder`, `defaultValue`, `value` |
| `ToggleSetting` | `settingKey`, `label`, `description`, `defaultValue`, `value` |
| `SelectionSetting` | `settingKey`, `label`, `description`, `options` (`["a", "b"]` or `[{label, value}]`), `defaultValue`, `value` |
| `SliderSetting` | `settingKey`, `label`, `description`, `defaultValue`, `value`, `minimum`, `maximum`, `step`, `startIcon`, `endIcon`, `unit` |

### Contributions

- `commands: [{id, title, icon, shortcut, run}]`: shown in the window menu. `run` is a function. `shortcut` is a key sequence such as `"Ctrl+Alt+D"`.
- `statusItem: Component`: an item shown in the status bar.
- `sidePanel: Component`: a panel shown beside the editor.

Change any of them at runtime and the window picks the change up.

### `pluginService`

| Call | Stored in |
|---|---|
| `loadPluginData(pluginId, key, default)`, `savePluginData(pluginId, key, value)` | `~/.config/dux/edit/plugin_settings.json` |
| `loadPluginState(pluginId, key, default)`, `savePluginState(pluginId, key, value)`, `removePluginStateKey(pluginId, key)`, `clearPluginState(pluginId)` | `~/.local/state/dux/edit/plugins/<id>_state.json` |
| `getPluginPath(pluginId)` | the plugin's folder |

Signals: `pluginDataChanged(pluginId)`, `pluginStateChanged(pluginId)`. Values must be JSON. Writes are batched and only happen when a value changes. Never store document text: settings and state files are not private.

### `editor`

The active document, or empty values when no document is open.

| Member | Notes |
|---|---|
| `filePath` | Empty for an unsaved draft |
| `language` | The highlighter's lexer name, such as `go` or `markdown` |
| `cursorLine`, `cursorColumn` | 1-based, as the status bar shows them |
| `selectedText` | |
| `getText()` | The whole document. Call it when needed, not in a binding: it copies the text. |
| `insertText(text)` | At the cursor |
| `replaceSelection(text)` | |
| `replaceRange(start, end, text)` | Offsets are UTF-16 code units, as QML strings count them |
| `openFile(path, line)` | Opens or switches to a tab; `line` is 1-based, 0 for none |
| `activeDocumentChanged()` | The current tab changed |
| `documentSaved(path)` | Any open document was saved |

Edits go through the editor, so they are undoable and saved like typing.

### Also available

- `qs.Common`: `I18n` and `Theme`.
- `qs.DCommon.Common` and `qs.DCommon.Widgets`: `Style` tokens and the D* widgets. Use them instead of fixed colors and sizes.
- `qs.Services`: `ToastService.showInfo`, `showWarning`, `showError`.
- `Quickshell.Io`: `Process`, `FileView` and friends.

### Not provided

DMS `SettingsData`, `SessionData`, `PopoutService`, `DMSService`, `CompositorService`, bar, popout, desktop, dashboard and launcher surfaces, plugin variants, and the `qs.Widgets` Dank* shims. Plugins get no way to save with administrator rights.

## Example

`~/.config/dux/edit/plugins/dateTools/plugin.json` is the manifest above without the `settings` line. `DateTools.qml`:

```qml
import QtQuick
import qs.Common
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.Modules.Plugins

EditorPluginComponent {
    id: root

    commands: [
        {
            "id": "insertDate",
            "title": I18n.tr("Insert Date", "plugin command"),
            "icon": "calendar_today",
            "shortcut": "Ctrl+Alt+D",
            "run": () => root.editor.insertText(new Date().toISOString().slice(0, 10))
        }
    ]

    statusItem: Component {
        StyledText {
            property int words: -1
            text: words < 0 ? I18n.tr("Count words", "plugin status item") : I18n.tr("%1 words", "plugin status item").arg(words)
            color: Style.surfaceVariantText
            font.pixelSize: Style.fontSizeSmall
            MouseArea {
                anchors.fill: parent
                onClicked: parent.words = root.editor.getText().split(/\s+/).filter(w => w).length
            }
        }
    }
}
```

The word count is read on demand when you click it, so the plugin does nothing while you type.
