// Package backend wires the services to the socket. Routes live in routes.go, with one handler file per area.
package backend

import (
	"context"
	"encoding/json"
	"os"
	"slices"

	"github.com/duxlabs-dev/dux-edit/core/internal/colorscheme"
	"github.com/duxlabs-dev/dux-edit/core/internal/document"
	"github.com/duxlabs-dev/dux-edit/core/internal/elevate"
	"github.com/duxlabs-dev/dux-edit/core/internal/highlight"
	"github.com/duxlabs-dev/dux-edit/core/internal/paths"
	"github.com/duxlabs-dev/dux-edit/core/internal/spell"
	"github.com/duxlabs-dev/dux-go/ipc"
)

const APIVersion = 1

// A pasted or reset buffer travels in one request line; dux-go drops the connection past this size.
const maxLineSize = 96 << 20

// OpenEnv carries a ui.open payload into a --new-instance child.
const OpenEnv = "DUX_EDIT_OPEN"

type Backend struct {
	version string
	bus     *ipc.EventBus
	server  *ipc.Server
	done    chan error
	pending PendingOpen

	docs      *document.Store
	highlight *highlight.Service
	spell     *spell.Service
	scheme    *colorscheme.Watcher
}

func Start(ctx context.Context, appID, version string) (*Backend, error) {
	b := &Backend{
		version: version,
		bus:     ipc.NewEventBus(),
		done:    make(chan error, 1),
	}
	dictionary, err := paths.CustomDictionary()
	if err != nil {
		return nil, err
	}
	b.docs = document.NewStore(b.bus.Publish)
	b.docs.SetElevator(elevate.New())
	b.highlight = highlight.New(b.docs, b.bus.Publish)
	b.spell = spell.New(b.docs, b.highlight, b.bus.Publish, dictionary)
	b.docs.AddListener(b.highlight)
	b.docs.AddListener(b.spell)
	b.scheme = colorscheme.New(b.bus.Publish)
	b.takeOpenEnv()

	b.server = ipc.NewServer(ipc.Config{
		AppName:                appID,
		APIVersion:             APIVersion,
		Capabilities:           []string{"subscribe", "ui", "docs", "highlight", "spell", "search", "markdown", "plugins"},
		MaxLineSize:            maxLineSize,
		DefaultSubscribeTopics: []string{"ui"},
		Bus:                    b.bus,
		OnSubscribe:            b.replayState,
	}, b.routes().ServeIPC)

	if err := b.server.Listen(); err != nil {
		return nil, err
	}
	go func() { b.done <- b.server.Serve(ctx) }()
	return b, nil
}

func (b *Backend) SocketPath() string { return b.server.SocketPath() }

func (b *Backend) Done() <-chan error { return b.done }

// FlushSwaps writes every pending swap now, for an exit that skips Close.
func (b *Backend) FlushSwaps() { b.docs.Flush() }

func (b *Backend) Close() {
	b.docs.Shutdown()
	b.spell.Close()
	b.highlight.Close()
	b.scheme.Close()
	b.server.Close()
}

func (b *Backend) replayState(topics []string, _ *ipc.Subscriber) {
	if slices.Contains(topics, "ui") {
		b.pending.Flush(b.bus)
	}
	if slices.Contains(topics, colorscheme.Topic) {
		b.bus.Publish(colorscheme.Topic, map[string]any{"scheme": b.scheme.Scheme()})
	}
}

// The UI child inherits our environment, so the variable is dropped once read.
func (b *Backend) takeOpenEnv() {
	raw := os.Getenv(OpenEnv)
	if raw == "" {
		return
	}
	_ = os.Unsetenv(OpenEnv)
	var payload map[string]any
	if json.Unmarshal([]byte(raw), &payload) != nil {
		return
	}
	b.uiOpen(context.Background(), payload)
}
