package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/duxlabs-dev/dux-edit/core/internal/helper"
)

var helperCmd = &cobra.Command{
	Use:                "helper",
	Short:              "Root-side read and save runner started by pkexec (not for direct use)",
	Hidden:             true,
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	Run: func(_ *cobra.Command, args []string) {
		os.Exit(helper.Main(args, os.Stdin, os.Stdout, os.Stderr))
	},
}
