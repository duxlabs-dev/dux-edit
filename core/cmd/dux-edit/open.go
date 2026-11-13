package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

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

var (
	fileLineColArg = regexp.MustCompile(`^(.+):(\d+):(\d+):?$`)
	fileLineArg    = regexp.MustCompile(`^(.+):(\d+):?$`)
	plusLineArg    = regexp.MustCompile(`^\+(\d+)(?::(\d+))?$`)
)

func position(line, column string) (int, int, bool) {
	l, err := strconv.Atoi(line)
	if err != nil || l < 1 {
		return 0, 0, false
	}
	c, _ := strconv.Atoi(column)
	return l, max(c, 0), true
}

func existsLiterally(arg string) bool {
	_, err := os.Lstat(arg)
	return err == nil
}

// splitFileArg reads file:LINE[:COL]; a trailing colon, as compilers print it, is accepted.
func splitFileArg(arg string) (string, int, int) {
	if existsLiterally(arg) {
		return arg, 0, 0
	}
	type reading struct {
		path         string
		line, column int
	}
	var readings []reading
	if m := fileLineColArg.FindStringSubmatch(arg); m != nil {
		if line, column, ok := position(m[2], m[3]); ok {
			readings = append(readings, reading{m[1], line, column})
		}
	}
	if m := fileLineArg.FindStringSubmatch(arg); m != nil {
		if line, _, ok := position(m[2], ""); ok {
			readings = append(readings, reading{m[1], line, 0})
		}
	}
	if len(readings) == 0 {
		return arg, 0, 0
	}
	pick := readings[0]
	for _, r := range readings {
		if existsLiterally(r.path) {
			pick = r
			break
		}
	}
	return pick.path, pick.line, pick.column
}

func plusPosition(arg string) (int, int, bool) {
	m := plusLineArg.FindStringSubmatch(arg)
	if m == nil || existsLiterally(arg) {
		return 0, 0, false
	}
	return position(m[1], m[2])
}

// Paths resolve against the caller's working directory, which the running app does not share.
// For the first file, --line wins over +LINE, which wins over file:LINE.
func openParams(args []string) (map[string]any, error) {
	paths := make([]string, 0, len(args))
	positions := make([]backend.OpenPosition, 0, len(args))
	line, column := openFlags.line, openFlags.column
	plusLine, plusColumn := 0, 0
	for _, arg := range args {
		if l, c, ok := plusPosition(arg); ok {
			plusLine, plusColumn = l, c
			continue
		}
		path, l, c := splitFileArg(arg)
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		if len(paths) == 0 && line == 0 {
			line, column = l, c
		}
		paths = append(paths, abs)
		positions = append(positions, backend.OpenPosition{Line: l, Column: c})
	}
	if openFlags.line == 0 && plusLine > 0 {
		line, column = plusLine, plusColumn
	}
	if len(positions) > 0 {
		positions[0] = backend.OpenPosition{Line: line, Column: column}
	}
	return map[string]any{
		"paths":     paths,
		"positions": positions,
		"line":      line,
		"column":    column,
		"simple":    openFlags.simple,
	}, nil
}

func openFiles(args []string) error {
	params, err := openParams(args)
	if err != nil {
		return err
	}
	if len(params["paths"].([]string)) == 0 && !openFlags.simple && !openFlags.newInstance {
		return shellApp.CallOrLaunch("ui.show", nil)
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
