package backend

import (
	"context"
	"path/filepath"

	"github.com/duxlabs-dev/dux-edit/core/internal/paths"
	"github.com/duxlabs-dev/dux-edit/core/internal/plugins"
)

const systemPluginDir = "/usr/share/dux/edit/plugins"

func (b *Backend) pluginsList(context.Context, map[string]any) any {
	dirs := []string{systemPluginDir}
	if config, err := paths.ConfigDir(); err == nil {
		dirs = append([]string{filepath.Join(config, "plugins")}, dirs...)
	}
	manifests := plugins.Scan(dirs)
	if manifests == nil {
		manifests = []plugins.Manifest{}
	}
	return map[string]any{"plugins": manifests, "dirs": dirs}
}
