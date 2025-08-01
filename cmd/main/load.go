package main

import (
	"github.com/spf13/cobra"
	"station/cmd/main/handlers"
)

// runLoad implements the "stn load" command
func runLoad(cmd *cobra.Command, args []string) error {
	loadHandler := handlers.NewLoadHandler(themeManager)
	return loadHandler.RunLoad(cmd, args)
}