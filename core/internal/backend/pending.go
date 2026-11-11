package backend

import (
	"sync"

	"github.com/duxlabs-dev/dux-go/ipc"
)

// Holds "ui" actions that arrived before the UI subscribed, so a cold start still honors them.
type PendingOpen struct {
	mu       sync.Mutex
	payloads []map[string]any
}

func (p *PendingOpen) Deliver(bus *ipc.EventBus, payload map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.payloads) == 0 && bus.HasSubscriber("ui") {
		bus.Publish("ui", payload)
		return
	}
	p.payloads = append(p.payloads, payload)
}

func (p *PendingOpen) Flush(bus *ipc.EventBus) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, payload := range p.payloads {
		bus.Publish("ui", payload)
	}
	p.payloads = nil
}
