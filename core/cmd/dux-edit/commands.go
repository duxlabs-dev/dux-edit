package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   appID + " [file...]",
	Short: "Text and document editor",
	Long: `Without arguments ` + appID + ` shows its window and starts the app first when it is not running.
Files open as tabs in the running app.

A file named like a command (run, kill, ipc, qs, version, helper, import-dms-notes, ...) needs a path: ` + appID + ` ./run`,
	Args:          cobra.ArbitraryArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	// Persistent so it also guards run and restart, whose PreRunE belongs to dux-go.
	PersistentPreRunE: refuseRoot,
	PreRunE:           shellApp.ResolveConfig,
	RunE: func(_ *cobra.Command, args []string) error {
		return openFiles(args)
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
	rootCmd.Flags().IntVarP(&openFlags.line, "line", "l", 0, "Line to jump to in the first file")
	rootCmd.Flags().IntVar(&openFlags.column, "column", 0, "Column to jump to in the first file")
	rootCmd.Flags().BoolVarP(&openFlags.simple, "simple", "s", false, "Open in Simple mode")
	rootCmd.Flags().BoolVarP(&openFlags.newInstance, "new-instance", "n", false, "Start a separate app instead of opening tabs in the running one")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(ipcCmd)
	rootCmd.AddCommand(helperCmd)
	rootCmd.AddCommand(importDMSCmd)
	rootCmd.AddCommand(shellApp.Commands()...)
}
