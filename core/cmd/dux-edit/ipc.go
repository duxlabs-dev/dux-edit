package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/spf13/cobra"

	"github.com/duxlabs-dev/dux-edit/core/internal/backend"
)

var ipcCmd = &cobra.Command{
	Use:   "ipc <method> [key=value...]",
	Short: "Send an IPC request to the running " + appID,
	Long: `Send an IPC request to the running ` + appID + ` and print the result as JSON.

  ` + appID + ` ipc doc.open path=/etc/hostname
  ` + appID + ` ipc ui.open paths=/tmp/a paths=/tmp/b
  ` + appID + ` ipc describe

Params are key=value. Values that parse as JSON are sent typed, anything else as a string.
A key given twice is sent as a list, and so is a list param such as paths, even when given once.

Method names and param keys complete with the shell (see '` + appID + ` completion').`,
	Args:                  cobra.MinimumNArgs(1),
	ValidArgsFunction:     completeIPC,
	DisableFlagsInUseLine: true,
	RunE: func(_ *cobra.Command, args []string) error {
		spec, _ := backend.FindMethod(args[0])
		params, err := parseParams(spec, args[1:])
		if err != nil {
			return err
		}
		result, err := callRunning(args[0], params)
		if err != nil {
			return err
		}
		return printJSON(result)
	},
}

func callRunning(method string, params map[string]any) (any, error) {
	socketPath, err := runningSocket()
	if err != nil {
		return nil, err
	}
	client, err := ipc.Dial(socketPath)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	resp, err := client.Call(ipc.Request{ID: 1, Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("ipc error: %s", resp.Error)
	}
	return resp.Result, nil
}

func runningSocket() (string, error) {
	if socketPath := os.Getenv(envPrefix + "_SOCKET"); socketPath != "" {
		return socketPath, nil
	}
	socketPath, err := ipc.FindRunningSocket(appID)
	if err != nil {
		return "", fmt.Errorf("%s is not running: %w", appID, err)
	}
	return socketPath, nil
}

// List params keep their items as strings, so a path like 2024 is not sent as a number.
func parseParams(spec backend.MethodSpec, args []string) (map[string]any, error) {
	if len(args) == 0 {
		return nil, nil
	}
	lists := make(map[string]bool, len(spec.Params))
	for _, p := range spec.Params {
		lists[p.Name] = p.List
	}
	params := make(map[string]any, len(args))
	for _, arg := range args {
		key, raw, ok := strings.Cut(arg, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("expected key=value, got %q", arg)
		}
		var value any
		if json.Unmarshal([]byte(raw), &value) != nil {
			value = raw
		}
		list, isList := value.([]any)
		if lists[key] && !isList {
			list, isList = []any{raw}, true
		}
		prev, seen := params[key]
		switch {
		case seen && isList:
			params[key] = append(asList(prev), list...)
		case seen:
			params[key] = append(asList(prev), value)
		case isList:
			params[key] = list
		default:
			params[key] = value
		}
	}
	return params, nil
}

func asList(value any) []any {
	if list, ok := value.([]any); ok {
		return list
	}
	return []any{value}
}

func completeIPC(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return backend.MethodNames(), cobra.ShellCompDirectiveNoFileComp
	}
	spec, ok := backend.FindMethod(args[0])
	if !ok {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	used := make(map[string]bool, len(args))
	for _, arg := range args[1:] {
		if key, _, found := strings.Cut(arg, "="); found {
			used[key] = true
		}
	}
	comps := make([]string, 0, len(spec.Params))
	for _, p := range spec.Params {
		if used[p.Name] && !p.List {
			continue
		}
		desc := p.Desc
		if p.Required {
			desc = strings.TrimSpace("required " + desc)
		}
		comps = append(comps, p.Name+"=\t"+desc)
	}
	return comps, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
}
