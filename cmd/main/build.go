package main

import (
	"context"

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

Two build modes:
  1. Fast deployment builds (--skip-sync): 0.1s builds, sync at runtime
  2. Full builds: Complete with sync, 15-20 min (for local development)

Examples:
  # Fast deployment build (recommended for Fly.io/K8s/etc)
  stn build env default --skip-sync

  # Full build with sync during build
  stn build env default --provider openai --model gpt-4o-mini

  # Custom image name for registry
  stn build env myenv --skip-sync --image myorg/station-myenv --tag v1.0.0

Fast builds:
  - Uses production base image (ghcr.io/cloudshipai/station:latest)
  - Copies environment files only
  - Syncs agents/MCP at runtime using environment secrets
  - Perfect for deployment to Fly.io, K8s, Cloud Run, etc.

Full builds:
  - Runs sync during build with AI provider
  - Includes all tools and dependencies
  - Suitable for local development/testing`,
		Args: cobra.RangeArgs(0, 1),
		RunE: runBuildEnvironment,
	}

	buildRuntimeCmd = &cobra.Command{
		Use:   "runtime",
		Short: "Build runtime container for stn up/down",
		Long: `Build a minimal runtime container for use with 'stn up' and 'stn down'.
This container is designed to mount existing local configuration and database,
rather than initializing new ones.

By default, pulls the latest image from GitHub Container Registry.
Use --local to build from source instead.

Examples:
  stn build runtime              # Pull from GHCR
  stn build runtime --local      # Build from source locally
  stn build runtime --no-ship    # Pull without Ship CLI`,
		RunE: runBuildRuntime,
	}
)

func init() {
	buildCmd.AddCommand(buildBaseCmd)
	buildCmd.AddCommand(buildEnvironmentCmd)
	buildCmd.AddCommand(buildRuntimeCmd)

	// Add flags for environment build command
	buildEnvironmentCmd.Flags().Bool("skip-sync", false, "Skip sync during build (fast deployment builds, sync at runtime)")
	buildEnvironmentCmd.Flags().String("image", "", "Custom image name (default: station-<environment>)")
	buildEnvironmentCmd.Flags().String("tag", "latest", "Image tag")
	buildEnvironmentCmd.Flags().String("provider", "", "AI provider for full builds (openai, gemini, anthropic)")
	buildEnvironmentCmd.Flags().String("model", "", "AI model for full builds")
	buildEnvironmentCmd.Flags().String("cloudshipai-registration-key", "", "CloudShip AI registration key for telemetry")
	buildEnvironmentCmd.Flags().String("cloudshipai-endpoint", "127.0.0.1:50051", "CloudShip AI endpoint")
	buildEnvironmentCmd.Flags().Bool("ship", false, "Install Ship CLI for security tools (full builds only)")

	// Provider and model only required for full builds (not skip-sync)
	// Validation happens in handler based on skip-sync flag

	// Add flags for runtime build command
	buildRuntimeCmd.Flags().Bool("no-ship", false, "Do not install Ship CLI")
	buildRuntimeCmd.Flags().Bool("local", false, "Build locally instead of pulling from GHCR")
}

// runBuildBase builds a base Station container
func runBuildBase(cmd *cobra.Command, args []string) error {
	buildHandler := build.NewBuildHandler(nil)
	return buildHandler.RunBuildBase(cmd, args)
}

// runBuildEnvironment builds a containerized environment
func runBuildEnvironment(cmd *cobra.Command, args []string) error {
	buildHandler := build.NewBuildHandler(nil)
	return buildHandler.RunBuildEnvironment(cmd, args)
}

// runBuildRuntime builds a runtime container for stn up/down
func runBuildRuntime(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	noShip, _ := cmd.Flags().GetBool("no-ship")
	buildLocal, _ := cmd.Flags().GetBool("local")

	builder := build.NewRuntimeBuilder(&build.RuntimeBuildOptions{
		ImageName:   "station-runtime:latest",
		InstallShip: !noShip,
		BuildLocal:  buildLocal,
	})

	return builder.Build(ctx)
}
