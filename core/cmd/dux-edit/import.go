package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/duxlabs-dev/dux-edit/core/internal/dmsimport"
)

var importDryRun bool

var importDMSCmd = &cobra.Command{
	Use:   "import-dms-notes",
	Short: "Copy the DMS notepad tabs and settings into " + appID,
	Long: `Copy the DMS notepad tabs and settings into ` + appID + `, starting it when it is not running.
Scratch notes become drafts, file tabs open as tabs. DMS files are never changed.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if importDryRun {
			state, config := dmsimport.Dirs()
			result, err := dmsimport.Run(state, config, "", true)
			if err != nil {
				return err
			}
			return printJSON(result)
		}
		if err := shellApp.ResolveConfig(cmd, args); err != nil {
			return err
		}
		// Params make CallOrLaunch wait for a cold start; version ignores them.
		if err := shellApp.CallOrLaunch("version", map[string]any{"wait": true}); err != nil {
			return err
		}
		result, err := callRunning("import.dms", map[string]any{"apply": true})
		if err != nil {
			return err
		}
		return printJSON(result)
	},
}

func init() {
	importDMSCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "List what would be imported without copying anything")
}

func printJSON(v any) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}
