package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           appID,
	Short:         "Dux app starter",
	Long:          "Without arguments " + appID + " shows its window and starts the app first when it is not running.",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	PreRunE:       shellApp.ResolveConfig,
	RunE: func(*cobra.Command, []string) error {
		return shellApp.CallOrLaunch("ui.show", nil)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(*cobra.Command, []string) {
		fmt.Printf("%s %s (commit %s, built %s)\n", appID, Version, Commit, BuildTime)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(shellApp.CustomConfigVar(), "config", "c", "", "Path to a UI config dir (containing shell.qml) to use instead of the embedded UI (env: "+envPrefix+"_SHELL_DIR)")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(ipcCmd)
	rootCmd.AddCommand(shellApp.Commands()...)
}
