package main

import (
	"os"

	"github.com/duxlabs-dev/dux-edit/core/internal/helper"
	"github.com/duxlabs-dev/dux-edit/core/internal/qtbaseembed"
	"github.com/duxlabs-dev/dux-go/log"
	"github.com/duxlabs-dev/dux-go/qtbase"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	Commit    = "unknown"
)

func main() {
	helper.UnderPkexec()
	log.SetEnvPrefix(envPrefix)
	qtbase.MaybeRunAndExit(qtbaseembed.Locator)
	if err := rootCmd.Execute(); err != nil {
		log.Errorf("%v", err)
		os.Exit(1)
	}
}
