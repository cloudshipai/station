package main

import (
	"github.com/spf13/cobra"
	"station/cmd/main/handlers"
)

// Runs command definitions
var (
	runsCmd = &cobra.Command{
		Use:   "runs",
		Short: "Manage agent runs",
		Long:  "List and inspect agent run history",
	}

	runsListCmd = &cobra.Command{
		Use:   "list",
		Short: "List agent runs",
		Long:  "List recent agent runs",
		RunE:  runRunsList,
	}

	runsInspectCmd = &cobra.Command{
		Use:   "inspect <id>",
		Short: "Inspect a run",
		Long:  "Show detailed information about a specific agent run",
		Args:  cobra.ExactArgs(1),
		RunE:  runRunsInspect,
	}
)

// runRunsList lists agent runs
func runRunsList(cmd *cobra.Command, args []string) error {
	runsHandler := handlers.NewRunsHandler(themeManager)
	return runsHandler.RunRunsList(cmd, args)
}

// runRunsInspect inspects a specific run
func runRunsInspect(cmd *cobra.Command, args []string) error {
	runsHandler := handlers.NewRunsHandler(themeManager)
	return runsHandler.RunRunsInspect(cmd, args)
}