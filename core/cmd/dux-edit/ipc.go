package main

import (
	"fmt"
	"os"

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
	ValidArgsFunction:     backend.Methods.Complete,
	DisableFlagsInUseLine: true,
	RunE: func(_ *cobra.Command, args []string) error {
		method, _ := backend.Methods.Find(args[0])
		params, err := method.ParseJSONParams(args[1:])
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
