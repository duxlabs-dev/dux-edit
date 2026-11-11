package main

import (
	"context"

	"github.com/duxlabs-dev/dux-edit/core/internal/backend"
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
	ShowMethod: "ui.show",
})

func bootBackend(ctx context.Context) (shellapp.Backend, error) {
	return backend.Start(ctx, appID, Version)
}

type embeddedShell struct{}

func (embeddedShell) Available() bool { return shellembed.Available() }

func (embeddedShell) Extract(baseDir string) (string, error) { return shellembed.Extract(baseDir) }

func (embeddedShell) Prune(baseDir, keep string) { shellembed.Prune(baseDir, keep) }
