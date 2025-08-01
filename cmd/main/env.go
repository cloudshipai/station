package main

import (
	"github.com/spf13/cobra"
	"station/cmd/main/handlers"
)

// Environment command definitions
var (
	envCmd = &cobra.Command{
		Use:   "env",
		Short: "Manage environments",
		Long:  "Create, list, update, and delete environments for organizing MCP configurations",
	}

	envListCmd = &cobra.Command{
		Use:   "list",
		Short: "List environments",
		Long:  "List all available environments",
		RunE:  runEnvList,
	}

	envCreateCmd = &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new environment",
		Long:  "Create a new environment with the specified name",
		Args:  cobra.ExactArgs(1),
		RunE:  runEnvCreate,
	}

	envGetCmd = &cobra.Command{
		Use:   "get <name|id>",
		Short: "Get environment details",
		Long:  "Get detailed information about an environment by name or ID",
		Args:  cobra.ExactArgs(1),
		RunE:  runEnvGet,
	}

	envUpdateCmd = &cobra.Command{
		Use:   "update <name|id>",
		Short: "Update an environment",
		Long:  "Update an environment's name or description",
		Args:  cobra.ExactArgs(1),
		RunE:  runEnvUpdate,
	}

	envDeleteCmd = &cobra.Command{
		Use:   "delete <name|id>",
		Short: "Delete an environment",
		Long:  "Delete an environment and all associated configurations",
		Args:  cobra.ExactArgs(1),
		RunE:  runEnvDelete,
	}
)

// runEnvList lists all environments
func runEnvList(cmd *cobra.Command, args []string) error {
	envHandler := handlers.NewEnvironmentHandler(themeManager)
	return envHandler.RunEnvList(cmd, args)
}

// runEnvCreate creates a new environment
func runEnvCreate(cmd *cobra.Command, args []string) error {
	envHandler := handlers.NewEnvironmentHandler(themeManager)
	return envHandler.RunEnvCreate(cmd, args)
}

// runEnvGet gets environment details
func runEnvGet(cmd *cobra.Command, args []string) error {
	envHandler := handlers.NewEnvironmentHandler(themeManager)
	return envHandler.RunEnvGet(cmd, args)
}

// runEnvUpdate updates an environment
func runEnvUpdate(cmd *cobra.Command, args []string) error {
	envHandler := handlers.NewEnvironmentHandler(themeManager)
	return envHandler.RunEnvUpdate(cmd, args)
}

// runEnvDelete deletes an environment
func runEnvDelete(cmd *cobra.Command, args []string) error {
	envHandler := handlers.NewEnvironmentHandler(themeManager)
	return envHandler.RunEnvDelete(cmd, args)
}