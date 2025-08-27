package main

import (
	"github.com/spf13/cobra"
	"station/cmd/main/handlers/build"
)

// Build command definitions
var (
	buildCmd = &cobra.Command{
		Use:   "build",
		Short: "Build containerized environments",
		Long: `Build Docker containers for Station environments with all dependencies packaged.
Creates portable, deployable containers with:
- Station binary and configuration
- Environment-specific MCP servers and tools  
- Agent configurations and prompts
- Database with proper schema and data`,
	}

	buildBaseCmd = &cobra.Command{
		Use:   "base",
		Short: "Build a base Station container",
		Long: `Build a base Docker container with Station binary and system dependencies.
This creates a reusable base image that can have environment configs injected at runtime.

Examples:
  stn build base                 # Build base Station container`,
		RunE: runBuildBase,
	}

	buildEnvironmentCmd = &cobra.Command{
		Use:   "env [environment]",
		Short: "Build a specific environment container",
		Long: `Build a Docker container for a specific Station environment.
Packages all MCP configurations, agents, and dependencies into a portable container.

Examples:
  stn build env default          # Build default environment container
  stn build env production       # Build production environment container
  stn build env                  # Build default environment container

For staging/production deployments, consider using 'stn build base' and injecting configs at runtime:
  docker run -v ./staging/variables.yml:/app/environment/variables.yml \\
             -e OPENAI_API_KEY=$STAGING_KEY station-base:latest`,
		Args: cobra.RangeArgs(0, 1),
		RunE: runBuildEnvironment,
	}
)

func init() {
	buildCmd.AddCommand(buildBaseCmd)
	buildCmd.AddCommand(buildEnvironmentCmd)
	
	// Add flags for environment build command
	buildEnvironmentCmd.Flags().String("provider", "", "AI provider to configure (openai, gemini, anthropic)")
	buildEnvironmentCmd.Flags().String("model", "", "AI model to use")
	buildEnvironmentCmd.Flags().String("cloudshipai-registration-key", "", "CloudShip AI registration key for telemetry")
	buildEnvironmentCmd.Flags().String("cloudshipai-endpoint", "127.0.0.1:50051", "CloudShip AI endpoint")
	buildEnvironmentCmd.Flags().Bool("ship", false, "Install Ship CLI for security tools")
	
	// Make provider and model required
	buildEnvironmentCmd.MarkFlagRequired("provider")
	buildEnvironmentCmd.MarkFlagRequired("model")
}

// runBuildBase builds a base Station container
func runBuildBase(cmd *cobra.Command, args []string) error {
	buildHandler := build.NewBuildHandler(themeManager)
	return buildHandler.RunBuildBase(cmd, args)
}

// runBuildEnvironment builds a containerized environment
func runBuildEnvironment(cmd *cobra.Command, args []string) error {
	buildHandler := build.NewBuildHandler(themeManager)
	return buildHandler.RunBuildEnvironment(cmd, args)
}