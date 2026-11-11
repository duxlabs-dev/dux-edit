package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/duxlabs-dev/dux-edit/core/internal/backend"
)

const ephemeralEnv = envPrefix + "_EPHEMERAL"

var openFlags struct {
	line        int
	column      int
	simple      bool
	newInstance bool
}

// Paths resolve against the caller's working directory, which the running app does not share.
func openParams(args []string) (map[string]any, error) {
	paths := make([]string, 0, len(args))
	for _, arg := range args {
		abs, err := filepath.Abs(arg)
		if err != nil {
			return nil, err
		}
		paths = append(paths, abs)
	}
	return map[string]any{
		"paths":  paths,
		"line":   openFlags.line,
		"column": openFlags.column,
		"simple": openFlags.simple,
	}, nil
}

func openFiles(args []string) error {
	if len(args) == 0 && !openFlags.simple && !openFlags.newInstance {
		return shellApp.CallOrLaunch("ui.show", nil)
	}
	params, err := openParams(args)
	if err != nil {
		return err
	}
	if !openFlags.newInstance {
		return shellApp.CallOrLaunch("ui.open", params)
	}
	return launchNewInstance(params)
}

// The daemon child and its UI inherit our environment; an ephemeral UI neither restores nor writes the session.
func launchNewInstance(params map[string]any) error {
	payload, err := json.Marshal(params)
	if err != nil {
		return err
	}
	if err := os.Setenv(backend.OpenEnv, string(payload)); err != nil {
		return err
	}
	if err := os.Setenv(ephemeralEnv, "1"); err != nil {
		return err
	}
	return shellApp.RunDaemon(false)
}

var errRunAsRoot = errors.New(appID + " does not run as root. Start it as yourself; saving a protected file offers Save as Administrator")

// A root UI would write root-owned settings and swap; elevated saves cover what sudo was for.
func refuseRoot(cmd *cobra.Command, _ []string) error {
	if os.Geteuid() != 0 || os.Getenv("SUDO_USER") == "" {
		return nil
	}
	if !cmd.HasParent() {
		return errRunAsRoot
	}
	switch cmd.Name() {
	case "run", "restart", importDMSCmd.Name():
		return errRunAsRoot
	}
	return nil
}
