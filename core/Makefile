BINARY_NAME=dux-edit
SOURCE_DIR=cmd/$(BINARY_NAME)
BUILD_DIR=bin
SHELL_SRC=../quickshell
EMBED_DIR=internal/shellembed/dist

GO=go

BASE_VERSION=$(or $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//'),0.0.0)
COMMIT_COUNT=$(shell git rev-list --count HEAD 2>/dev/null || echo "0")
COMMIT_HASH=$(shell git rev-parse --short=8 HEAD 2>/dev/null || echo "unknown")
VERSION?=$(BASE_VERSION)+git$(COMMIT_COUNT).$(COMMIT_HASH)
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
COMMIT?=$(COMMIT_HASH)

BUILD_LDFLAGS=-ldflags='-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.Commit=$(COMMIT)'

.PHONY: all build dev qtbase clean-qtbase sync-shell clean test fmt vet lint deps help

all: build

# QTBASE=1 builds the runtime from the dux-qt-base submodule and embeds it in the binary.
# With the default QTBASE=0 the binary loads an installed libdux-qt-base.so.0 at runtime.
QTBASE ?= 0
QTBASE_SRC ?= ../dux-qt-base
QTBASE_BUILD ?= build/dux-qt-base
QTBASE_LIB = $(QTBASE_BUILD)/src/libdux-qt-base.so.0
QTBASE_EMBED_DIR = internal/qtbaseembed/lib
ifeq ($(QTBASE),0)
QTBASE_DEP =
QTBASE_TAG =
else
QTBASE_DEP = qtbase
QTBASE_TAG = withqtbase
endif

qtbase:
	@test -f $(QTBASE_SRC)/CMakeLists.txt || { echo "dux-qt-base missing: run git submodule update --init"; exit 1; }
	@test -f $(QTBASE_BUILD)/build.ninja || cmake -S $(QTBASE_SRC) -B $(QTBASE_BUILD) -GNinja -DCMAKE_BUILD_TYPE=Release \
		-DDISTRIBUTOR=$(BINARY_NAME)-source -DGIT_REVISION=$$(git -C $(QTBASE_SRC) rev-parse --short=8 HEAD 2>/dev/null || echo unknown)
	cmake --build $(QTBASE_BUILD)
	@mkdir -p $(QTBASE_EMBED_DIR)
	@strip --strip-unneeded -o $(QTBASE_EMBED_DIR)/libdux-qt-base.so.0 $(QTBASE_LIB)
	@sha256sum $(QTBASE_EMBED_DIR)/libdux-qt-base.so.0 | cut -c1-16 > $(QTBASE_EMBED_DIR)/.dankrev

clean-qtbase:
	@rm -rf $(QTBASE_BUILD)

# tar -h follows the DCommon symlink into the submodule, go:embed rejects symlinks.
sync-shell:
	@test -f $(SHELL_SRC)/DCommon/Widgets/DIcon.qml || { echo "DCommon missing: run git submodule update --init"; exit 1; }
	@rm -rf $(EMBED_DIR)
	@mkdir -p $(EMBED_DIR)
	@tar -C $(SHELL_SRC) --exclude=.qmlls.ini -chf - . | tar -C $(EMBED_DIR) -xf -
	@cd $(EMBED_DIR) && find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum | cut -c1-16 > .dankrev

build: sync-shell $(QTBASE_DEP)
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO) build -tags 'withshell $(QTBASE_TAG)' $(BUILD_LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./$(SOURCE_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

dev: $(QTBASE_DEP)
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO) build -tags '$(QTBASE_TAG)' -o $(BUILD_DIR)/$(BINARY_NAME) ./$(SOURCE_DIR)

clean:
	@rm -rf $(BUILD_DIR) $(EMBED_DIR) $(QTBASE_EMBED_DIR)

test:
	CGO_ENABLED=0 $(GO) test ./...

fmt:
	$(GO) fmt ./...

vet:
	CGO_ENABLED=0 $(GO) vet ./...

lint:
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run

deps:
	$(GO) mod tidy
	$(GO) mod download

help:
	@echo "Targets: build dev qtbase clean-qtbase sync-shell clean test fmt vet lint deps"
	@echo "QTBASE=1 embeds the dux-qt-base runtime from the submodule; by default $(BINARY_NAME) needs an installed dux-qt-base"
