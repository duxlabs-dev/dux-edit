package backend

import (
	"context"
	"os"
	"slices"
	"syscall"

	"github.com/duxlabs-dev/dux-go/ipc"
)

const APIVersion = 1

type Backend struct {
	version string
	bus     *ipc.EventBus
	server  *ipc.Server
	done    chan error
	counter counter
}

func Start(ctx context.Context, appID, version string) (*Backend, error) {
	b := &Backend{
		version: version,
		bus:     ipc.NewEventBus(),
		done:    make(chan error, 1),
	}
	b.server = ipc.NewServer(ipc.Config{
		AppName:                appID,
		APIVersion:             APIVersion,
		Capabilities:           []string{"subscribe", "ui", "counter"},
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

func (b *Backend) Close() { b.server.Close() }

func (b *Backend) replayState(topics []string, _ *ipc.Subscriber) {
	if slices.Contains(topics, "counter") {
		b.publishCounter(b.counter.get())
	}
}

func (b *Backend) routes() *ipc.Mux {
	mux := ipc.NewMux()
	mux.Handle("version", b.handleVersion)
	mux.Handle("ui.show", b.handleShow)
	mux.Handle("ui.quit", b.handleQuit)
	mux.Handle("counter.get", b.handleCounterGet)
	mux.Handle("counter.add", b.handleCounterAdd)
	mux.Handle("counter.reset", b.handleCounterReset)
	return mux
}

func (b *Backend) handleVersion(_ context.Context, w *ipc.ConnWriter, req ipc.Request, _ *ipc.Subscriber) {
	w.Respond(req.ID, map[string]any{"version": b.version, "apiVersion": APIVersion})
}

func (b *Backend) handleShow(_ context.Context, w *ipc.ConnWriter, req ipc.Request, _ *ipc.Subscriber) {
	b.bus.Publish("ui", map[string]any{"action": "show"})
	w.Respond(req.ID, map[string]any{"ok": true})
}

func (b *Backend) handleQuit(_ context.Context, w *ipc.ConnWriter, req ipc.Request, _ *ipc.Subscriber) {
	w.Respond(req.ID, map[string]any{"ok": true})
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
}
