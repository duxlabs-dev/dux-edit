# dux-app-starter

A template for a desktop app built like the Dux Labs apps: a Go binary that carries its own QML UI and runs it on [dux-qt-base](https://github.com/duxlabs-dev/dux-qt-base).

The demo is a counter. The Go backend owns the number, the window shows it, and `dux-edit ipc counter.add` changes it from a terminal while the window is open.

## What's in here

| Path | What it is |
|---|---|
| `core/` | The Go side. [dux-go](https://github.com/duxlabs-dev/dux-go) provides the `run`, `restart` and `kill` commands, supervises the UI process, and serves the unix socket the UI talks to. |
| `quickshell/` | The QML side. `shell.qml` is the entrypoint and `Services/Backend.qml` is the socket client. |
| `dux-qml-common/` | Submodule with the shared widgets and theme tokens, symlinked in as `quickshell/DCommon`. |
| `dux-qt-base/` | Submodule with the runtime. Only built with `QTBASE=1`. |
| `assets/` | Desktop entry and icon. |
| `scripts/format-staged.py` | Formats staged QML with qmlls. |

## Build and run

You need Go and dux-qt-base (the `dux-qt-base` package, `libdux-qt-base0` on Debian and Ubuntu).

```sh
git clone --recurse-submodules https://github.com/you/yourapp.git
cd yourapp
make run
```

`make run` makes a build without the embedded UI and loads `quickshell/` straight from the repo, with hot reload.

```sh
make
sudo make install
```

`make` builds `core/bin/dux-edit` with the UI embedded. `make install` puts the binary, desktop entry and icon under `/usr/local`.

`make QTBASE=1` also builds dux-qt-base from the submodule and embeds it, so the binary runs without the package installed. The build dependencies are listed in [BUILD.md](https://github.com/duxlabs-dev/dux-qt-base/blob/master/BUILD.md).

## Using it

```sh
dux-edit                           # show the window, start the app first if it is not running
dux-edit run                       # run in the foreground
dux-edit run -d                    # run in the background
dux-edit ipc counter.add amount=5
dux-edit ipc counter.reset
dux-edit restart
dux-edit kill
```

## How it works

1. `dux-edit run` starts the backend, which listens on a unix socket in `$XDG_RUNTIME_DIR`.
2. It then starts itself again as `dux-edit qs`. That process loads `libdux-qt-base.so` and runs `shell.qml`, from the copy embedded in the binary or from the dir given with `-c`.
3. The UI gets the socket path in `DUX_EDIT_SOCKET`. `Services/Backend.qml` connects to it, sends requests and subscribes to topics.
4. Messages are one JSON object per line. `{"id":1,"method":"counter.add","params":{"amount":5}}` is answered with `{"id":1,"result":{"value":5}}`, and every subscriber of `counter` gets `{"event":"counter","data":{"value":5}}`.
5. `dux-edit ipc` uses the same socket, so anything the UI can call you can call from a terminal.

## Adding a feature

The counter shows the three pieces a feature needs:

- **Go handlers** in `core/internal/backend/counter.go`, registered in `routes()` in `backend.go`. Publish on the bus when state changes, and add the topic to `replayState` so a UI that subscribes later gets the current state.
- **A QML service** in `quickshell/Services` that subscribes to the topic and exposes it as properties, like `Counter.qml`.
- **A view** that binds to the service, like `Modules/MainWindow.qml`.

## Theme

The widgets take colors, spacing and fonts from `Style` in dux-qml-common. With nothing set it uses its stock theme. To use your own, hand it your singletons at startup as described in [the contract](https://github.com/duxlabs-dev/dux-qml-common#the-contract).

## Renaming

Four names identify the app. From the repo root, with your own values in place of `notes`, `NOTES`, `org.you.notes` and `you/notes`:

```sh
git grep -lE 'dux-edit|DUX_EDIT|dux-app-starter' | xargs sed -i \
  -e 's|duxlabs-dev/dux-edit|you/notes|g' \
  -e 's|com\.example\.dux-edit|org.you.notes|g' \
  -e 's|dux-edit|notes|g' \
  -e 's|DUX_EDIT|NOTES|g'
git mv core/cmd/dux-edit core/cmd/notes
git mv assets/dev.duxlabs.edit.desktop assets/org.you.notes.desktop
git mv assets/dev.duxlabs.edit.svg assets/org.you.notes.svg
```

| Name | Used for |
|---|---|
| `dux-edit` | Binary, socket and pid file names, cache dir |
| `DUX_EDIT` | Prefix of the env vars: `DUX_EDIT_SOCKET`, `DUX_EDIT_SHELL_DIR`, `DUX_EDIT_LOG_LEVEL` |
| `dev.duxlabs.edit` | Wayland app id, desktop entry and icon name |
| `github.com/duxlabs-dev/dux-edit` | Go module path |

## License

MIT
