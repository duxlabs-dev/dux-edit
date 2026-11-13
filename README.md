# Dux Edit

<div align="center">
  <a href="https://duxlabs.dev">
    <img src="assets/danklogo.svg" alt="Dux Edit" width="200">
  </a>

### A text and document editor for the dux desktop

Built with [dux-qt-base](https://github.com/duxlabs-dev/dux-qt-base) and [Go](https://go.dev/)

[![GitHub License](https://img.shields.io/github/license/duxlabs-dev/dux-edit?style=for-the-badge&labelColor=101418&color=b9c8da)](https://github.com/duxlabs-dev/dux-edit/blob/master/LICENSE)
[![Discord](https://img.shields.io/badge/discord-join?style=for-the-badge&logo=discord&logoColor=ffffff&label=discord&labelColor=101418&color=5865f2)](https://discord.gg/ppWTpKmPgT)
[![Ko-Fi donate](https://img.shields.io/badge/donate-kofi?style=for-the-badge&logo=ko-fi&logoColor=ffffff&label=ko-fi&labelColor=101418&color=f16061&link=https%3A%2F%2Fko-fi.com%2Fduxlabs)](https://ko-fi.com/duxlabs)

</div>

Dux Edit opens any text file without a project around it: notes, config files, scripts, Markdown. It sits between a scratchpad and an IDE, and takes over from the DMS notepad.

- Tabs, drafts that survive a restart, and crash recovery
- Syntax highlighting with [Chroma](https://github.com/alecthomas/chroma), colored from the desktop theme
- Markdown preview, spellcheck, find and replace with regular expressions
- Keeps encodings and line endings as they are, and saves root-owned files through polkit
- Simple and Advanced modes

Dux Edit is a Go binary that carries its own QML UI and runs it on [dux-qt-base](https://github.com/duxlabs-dev/dux-qt-base).

## Build and run

You need Go and dux-qt-base 0.4.3 or newer (the `dux-qt-base` package, `libdux-qt-base0` on Debian and Ubuntu). Spellcheck uses `hunspell` and its dictionaries when they are installed.

```sh
git clone --recurse-submodules https://github.com/duxlabs-dev/dux-edit.git
cd dux-edit
make run
```

`make run` makes a build without the embedded UI and loads `quickshell/` straight from the repo, with hot reload.

```sh
make
sudo make install
```

`make` builds `core/bin/dux-edit` with the UI embedded. `make install` puts the binary, the `dedit` alias, the desktop entry and the icon under `/usr/local`. `make QTBASE=1` also builds dux-qt-base from the submodule and embeds it.

## Using it

```sh
dux-edit                          # show the window, start the app first if it is not running
dux-edit notes.md todo.txt        # open files as tabs in the running app
dux-edit main.go --line 42 --column 7
dux-edit --simple notes.md        # Simple mode
dux-edit --new-instance           # a separate app instead of tabs in the running one
dux-edit run -d                   # run in the background
dux-edit ipc describe             # every socket method
dux-edit kill
```

A file named like a command (`run`, `kill`, `ipc`, `qs`, `version`, `help`, ...) needs a path: `dux-edit ./run`.

## Where things live

| Path | What |
|---|---|
| `~/.config/dux/edit/settings.json` | Settings |
| `~/.config/dux/edit/custom_dictionary.txt` | Words added to the spellcheck dictionary, one per line |
| `~/.config/dux/edit/plugins/` | Plugins |
| `~/.local/state/dux/edit/session.json` | Open tabs (never their text) |
| `~/.local/state/dux/edit/swap/` | Drafts and unsaved changes |

## License

MIT
