BINARY_NAME=dux-edit
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

.PHONY: all build dev run clean test fmt vet lint update-common update-qtbase install uninstall help

all: build

build:
	@$(MAKE) -C $(CORE_DIR) build

dev:
	@$(MAKE) -C $(CORE_DIR) dev

run: dev
	@$(BUILD_DIR)/$(BINARY_NAME) run -c $(CURDIR)/$(SHELL_DIR)

clean:
	@$(MAKE) -C $(CORE_DIR) clean

test:
	@$(MAKE) -C $(CORE_DIR) test

fmt:
	@$(MAKE) -C $(CORE_DIR) fmt

vet:
	@$(MAKE) -C $(CORE_DIR) vet

lint:
	@$(MAKE) -C $(CORE_DIR) lint

update-common:
	git submodule update --remote --merge dux-qml-common

update-qtbase:
	git submodule update --remote --merge dux-qt-base

install:
	@test -f $(BUILD_DIR)/$(BINARY_NAME) || { echo "$(BUILD_DIR)/$(BINARY_NAME) not found; run 'make' first"; exit 1; }
	@install -D -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(DESTDIR)$(INSTALL_DIR)/$(BINARY_NAME)
	@install -D -m 644 $(ASSETS_DIR)/$(DESKTOP_ID).svg $(DESTDIR)$(ICON_DIR)/$(DESKTOP_ID).svg
	@install -D -m 644 $(ASSETS_DIR)/$(DESKTOP_ID).desktop $(DESTDIR)$(APPLICATIONS_DIR)/$(DESKTOP_ID).desktop
	@test -n "$(DESTDIR)" || gtk-update-icon-cache -q $(DATA_DIR)/icons/hicolor 2>/dev/null || true
	@test -n "$(DESTDIR)" || update-desktop-database -q $(APPLICATIONS_DIR) 2>/dev/null || true
	@echo "Installed. Launch with '$(BINARY_NAME)' or the desktop entry."

uninstall:
	@rm -f $(DESTDIR)$(INSTALL_DIR)/$(BINARY_NAME)
	@rm -f $(DESTDIR)$(ICON_DIR)/$(DESKTOP_ID).svg
	@rm -f $(DESTDIR)$(APPLICATIONS_DIR)/$(DESKTOP_ID).desktop
	@test -n "$(DESTDIR)" || gtk-update-icon-cache -q $(DATA_DIR)/icons/hicolor 2>/dev/null || true
	@test -n "$(DESTDIR)" || update-desktop-database -q $(APPLICATIONS_DIR) 2>/dev/null || true

help:
	@echo "build          - Release build, UI embedded (QTBASE=1 also embeds the dux-qt-base runtime)"
	@echo "dev            - Fast build without the embedded UI"
	@echo "run            - dev build, then run against quickshell/ with hot reload"
	@echo "test fmt vet lint clean"
	@echo "update-common  - Bump the dux-qml-common submodule"
	@echo "update-qtbase  - Bump the dux-qt-base submodule"
	@echo "install        - Binary, icon and desktop entry under PREFIX=$(PREFIX)"
	@echo "uninstall      - Remove them"
