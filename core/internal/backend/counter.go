package backend

import (
	"context"
	"sync"

	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

type counter struct {
	mu    sync.Mutex
	value int
}

func (c *counter) get() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

func (c *counter) add(amount int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value += amount
	return c.value
}

func (c *counter) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = 0
}

func (b *Backend) publishCounter(value int) map[string]any {
	state := map[string]any{"value": value}
	b.bus.Publish("counter", state)
	return state
}

func (b *Backend) handleCounterGet(_ context.Context, w *ipc.ConnWriter, req ipc.Request, _ *ipc.Subscriber) {
	w.Respond(req.ID, map[string]any{"value": b.counter.get()})
}

func (b *Backend) handleCounterAdd(_ context.Context, w *ipc.ConnWriter, req ipc.Request, _ *ipc.Subscriber) {
	value := b.counter.add(params.IntOpt(req.Params, "amount", 1))
	w.Respond(req.ID, b.publishCounter(value))
}

func (b *Backend) handleCounterReset(_ context.Context, w *ipc.ConnWriter, req ipc.Request, _ *ipc.Subscriber) {
	b.counter.reset()
	w.Respond(req.ID, b.publishCounter(0))
}
