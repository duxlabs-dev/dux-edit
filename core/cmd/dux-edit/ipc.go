package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/spf13/cobra"
)

var ipcCmd = &cobra.Command{
	Use:   "ipc <method> [key=value...]",
	Short: "Send an IPC request to the running " + appID,
	Long: `Send an IPC request to the running ` + appID + ` and print the result as JSON.

  ` + appID + ` ipc counter.add amount=5

Params are key=value. Values that parse as JSON are sent typed, anything else as a string.`,
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	RunE: func(_ *cobra.Command, args []string) error {
		params, err := parseParams(args[1:])
		if err != nil {
			return err
		}
		socketPath, err := runningSocket()
		if err != nil {
			return err
		}
		client, err := ipc.Dial(socketPath)
		if err != nil {
			return err
		}
		defer client.Close()

		resp, err := client.Call(ipc.Request{ID: 1, Method: args[0], Params: params})
		if err != nil {
			return err
		}
		if resp.Error != "" {
			return fmt.Errorf("ipc error: %s", resp.Error)
		}
		out, err := json.MarshalIndent(resp.Result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	},
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

func parseParams(args []string) (map[string]any, error) {
	if len(args) == 0 {
		return nil, nil
	}
	params := make(map[string]any, len(args))
	for _, arg := range args {
		key, raw, ok := strings.Cut(arg, "=")
		if !ok {
			return nil, fmt.Errorf("expected key=value, got %q", arg)
		}
		var value any
		if json.Unmarshal([]byte(raw), &value) != nil {
			value = raw
		}
		params[key] = value
	}
	return params, nil
}
