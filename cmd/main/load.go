package main

import (
	"github.com/spf13/cobra"
	"station/cmd/main/handlers/load"
)

// runLoad implements the "stn load" command
func runLoad(cmd *cobra.Command, args []string) error {
	loadHandler := load.NewLoadHandler(themeManager)
	return loadHandler.RunLoad(cmd, args)
}