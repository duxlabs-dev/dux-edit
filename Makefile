BINARY_NAME=dux-edit
ALIAS_NAME=dedit
DESKTOP_ID=dev.duxlabs.edit
CORE_DIR=core
BUILD_DIR=$(CORE_DIR)/bin
SHELL_DIR=quickshell
ASSETS_DIR=assets
PREFIX ?= /usr/local
DESTDIR ?=
INSTALL_DIR=$(PREFIX)/bin
DATA_DIR=$(PREFIX)/share
ICON_DIR=$(DATA_DIR)/icons/hicolor/scalable/apps
APPLICATIONS_DIR=$(DATA_DIR)/applications
METAINFO_DIR=$(DATA_DIR)/metainfo
# polkit reads actions from /usr/share only, whatever the prefix.
POLKIT_ACTIONS_DIR ?= /usr/share/polkit-1/actions

.PHONY: all build dev run clean test test-js fmt vet lint lint-qml update-common update-qtbase check-bin install install-metainfo install-polkit uninstall help

all: build

build:
	@$(MAKE) -C $(CORE_DIR) build

dev:
	@$(MAKE) -C $(CORE_DIR) dev

run: dev
	@$(BUILD_DIR)/$(BINARY_NAME) run -c $(CURDIR)/$(SHELL_DIR)

clean:
	@$(MAKE) -C $(CORE_DIR) clean

test: test-js
	@$(MAKE) -C $(CORE_DIR) test

test-js:
	@node --test $(SHELL_DIR)/tests/

fmt:
	@$(MAKE) -C $(CORE_DIR) fmt

vet:
	@$(MAKE) -C $(CORE_DIR) vet

lint:
	@$(MAKE) -C $(CORE_DIR) lint

lint-qml:
	@./scripts/qmllint-entrypoints.sh

update-common:
	git submodule update --remote --merge dux-qml-common

update-qtbase:
	git submodule update --remote --merge dux-qt-base

install-metainfo:
	@install -D -m 644 $(ASSETS_DIR)/$(DESKTOP_ID).metainfo.xml $(DESTDIR)$(METAINFO_DIR)/$(DESKTOP_ID).metainfo.xml

# The policy names the installed binary; pkexec refuses any other path.
install-polkit:
	@install -d -m 755 $(DESTDIR)$(POLKIT_ACTIONS_DIR)
	@sed 's|/usr/bin/dux-edit|$(INSTALL_DIR)/$(BINARY_NAME)|g' $(ASSETS_DIR)/$(DESKTOP_ID).policy > $(DESTDIR)$(POLKIT_ACTIONS_DIR)/$(DESKTOP_ID).policy
	@chmod 644 $(DESTDIR)$(POLKIT_ACTIONS_DIR)/$(DESKTOP_ID).policy

check-bin:
	@test -f $(BUILD_DIR)/$(BINARY_NAME) || { echo "$(BUILD_DIR)/$(BINARY_NAME) not found; run 'make' first"; exit 1; }

install: check-bin install-metainfo install-polkit
	@install -D -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(DESTDIR)$(INSTALL_DIR)/$(BINARY_NAME)
	@ln -sf $(BINARY_NAME) $(DESTDIR)$(INSTALL_DIR)/$(ALIAS_NAME)
	@install -D -m 644 $(ASSETS_DIR)/$(DESKTOP_ID).svg $(DESTDIR)$(ICON_DIR)/$(DESKTOP_ID).svg
	@install -D -m 644 $(ASSETS_DIR)/$(DESKTOP_ID).desktop $(DESTDIR)$(APPLICATIONS_DIR)/$(DESKTOP_ID).desktop
	@test -n "$(DESTDIR)" || gtk-update-icon-cache -q $(DATA_DIR)/icons/hicolor 2>/dev/null || true
	@test -n "$(DESTDIR)" || update-desktop-database -q $(APPLICATIONS_DIR) 2>/dev/null || true
	@echo "Installed. Launch with '$(BINARY_NAME)', '$(ALIAS_NAME)' or the desktop entry."

uninstall:
	@rm -f $(DESTDIR)$(INSTALL_DIR)/$(BINARY_NAME) $(DESTDIR)$(INSTALL_DIR)/$(ALIAS_NAME)
	@rm -f $(DESTDIR)$(ICON_DIR)/$(DESKTOP_ID).svg
	@rm -f $(DESTDIR)$(APPLICATIONS_DIR)/$(DESKTOP_ID).desktop
	@rm -f $(DESTDIR)$(METAINFO_DIR)/$(DESKTOP_ID).metainfo.xml
	@rm -f $(DESTDIR)$(POLKIT_ACTIONS_DIR)/$(DESKTOP_ID).policy
	@test -n "$(DESTDIR)" || gtk-update-icon-cache -q $(DATA_DIR)/icons/hicolor 2>/dev/null || true
	@test -n "$(DESTDIR)" || update-desktop-database -q $(APPLICATIONS_DIR) 2>/dev/null || true

help:
	@echo "build          - Release build, UI embedded (QTBASE=1 also embeds the dux-qt-base runtime)"
	@echo "dev            - Fast build without the embedded UI"
	@echo "run            - dev build, then run against quickshell/ with hot reload"
	@echo "lint-qml       - qmllint shell.qml through the Quickshell tooling VFS (needs quickshell/.qmlls.ini)"
	@echo "test fmt vet lint clean"
	@echo "update-common  - Bump the dux-qml-common submodule"
	@echo "update-qtbase  - Bump the dux-qt-base submodule"
	@echo "install        - Binary, icon, desktop entry and metainfo under PREFIX=$(PREFIX), polkit policy in $(POLKIT_ACTIONS_DIR)"
	@echo "uninstall      - Remove them"
