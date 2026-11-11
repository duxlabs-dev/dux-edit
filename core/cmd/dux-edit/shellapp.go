package main

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/duxlabs-dev/dux-edit/core/internal/backend"
	"github.com/duxlabs-dev/dux-edit/core/internal/paths"
	"github.com/duxlabs-dev/dux-edit/core/internal/qtbaseembed"
	"github.com/duxlabs-dev/dux-edit/core/internal/shellembed"
	"github.com/duxlabs-dev/dux-go/shellapp"
)

const (
	appID     = "dux-edit"
	envPrefix = "DUX_EDIT"
	desktopID = "dev.duxlabs.edit"
)

var shellApp = shellapp.New(shellapp.Config{
	ID:         appID,
	EnvPrefix:  envPrefix,
	QSAppID:    desktopID,
	Version:    Version,
	Embedded:   embeddedShell{},
	QtBase:     qtbaseembed.Extract,
	Boot:       bootBackend,
	ExtraEnv:   func(string) []string { return paths.UIEnv(envPrefix) },
	ShowMethod: "ui.show",
	OnUIExit:   flushSwaps,
})

var running atomic.Pointer[backend.Backend]

func bootBackend(ctx context.Context) (shellapp.Backend, error) {
	b, err := backend.Start(ctx, appID, Version)
	if err != nil {
		return nil, err
	}
	running.Store(b)
	return b, nil
}

// dux-go leaves through os.Exit when the UI dies, so the deferred Close never runs and the swaps go out here.
func flushSwaps(int, time.Duration, string) {
	if b := running.Load(); b != nil {
		b.FlushSwaps()
	}
}

type embeddedShell struct{}

func (embeddedShell) Available() bool { return shellembed.Available() }

func (embeddedShell) Extract(baseDir string) (string, error) { return shellembed.Extract(baseDir) }

func (embeddedShell) Prune(baseDir, keep string) { shellembed.Prune(baseDir, keep) }
