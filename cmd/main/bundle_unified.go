package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"station/cmd/main/handlers/common"
	"station/internal/services"
)

// Unified bundle command with create and install subcommands
var bundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Bundle management commands",
	Long: `Create and install Station bundles.
	
Subcommands:
  create   Create a bundle from an environment
  install  Install a bundle from URL or file path`,
}

// Bundle create subcommand
var bundleCreateCmd = &cobra.Command{
	Use:   "create <environment>",
	Short: "Create a bundle from an environment",
	Long: `Create a deployable bundle (.tar.gz) from an environment.
This uses the same bundling logic as the API and creates bundles
that are compatible with the bundle API installation endpoints.

Examples:
  stn bundle create default              # Bundle the default environment
  stn bundle create production           # Bundle the production environment  
  stn bundle create default --output my-bundle.tar.gz  # Custom output path`,
	Args: cobra.ExactArgs(1),
	RunE: runBundleCreate,
}

// Bundle install subcommand
var bundleInstallCmd = &cobra.Command{
	Use:   "install <bundle-source> <environment-name>",
	Short: "Install a bundle from URL or file path",
	Long: `Install a bundle from a remote URL or local file path.
This uses the same installation logic as the Station UI.

Examples:
  stn bundle install https://github.com/cloudshipai/registry/releases/download/v1.0.0/devops-security-bundle.tar.gz security
  stn bundle install ./my-bundle.tar.gz production
  stn bundle install /path/to/bundle.tar.gz development`,
	Args: cobra.ExactArgs(2),
	RunE: runBundleInstall,
}

func init() {
	// Add flags to create subcommand
	bundleCreateCmd.Flags().String("output", "", "Output path for bundle (defaults to <environment>.tar.gz)")
	bundleCreateCmd.Flags().Bool("local", true, "Save bundle locally (always true for CLI)")
	
	// Add subcommands to main bundle command
	bundleCmd.AddCommand(bundleCreateCmd)
	bundleCmd.AddCommand(bundleInstallCmd)
}

func runBundleCreate(cmd *cobra.Command, args []string) error {
	environmentName := args[0]
	outputPath, _ := cmd.Flags().GetString("output")

	// Get Station config root
	configRoot, err := common.GetStationConfigRoot()
	if err != nil {
		return fmt.Errorf("failed to get station config root: %w", err)
	}

	// Environment directory path
	envPath := filepath.Join(configRoot, "environments", environmentName)
	
	// Check if environment directory exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("environment '%s' not found at %s", environmentName, envPath)
	}

	// Set default output path if not provided
	if outputPath == "" {
		outputPath = fmt.Sprintf("%s.tar.gz", environmentName)
	}

	fmt.Printf("🗂️  Bundling environment: %s\n", environmentName)
	fmt.Printf("📂 Source path: %s\n", envPath)

	// Create bundle service
	bundleService := services.NewBundleService()
	
	// Validate environment
	if err := bundleService.ValidateEnvironment(envPath); err != nil {
		return fmt.Errorf("environment validation failed: %w", err)
	}

	// Get bundle info for preview
	bundleInfo, err := bundleService.GetBundleInfo(envPath)
	if err != nil {
		return fmt.Errorf("failed to analyze environment: %w", err)
	}

	fmt.Printf("📋 Found:\n")
	fmt.Printf("   🤖 %d agent(s): %v\n", len(bundleInfo.AgentFiles), bundleInfo.AgentFiles)
	fmt.Printf("   🔧 %d MCP config(s): %v\n", len(bundleInfo.MCPConfigs), bundleInfo.MCPConfigs)
	if len(bundleInfo.OtherFiles) > 0 {
		fmt.Printf("   📄 %d other file(s): %v\n", len(bundleInfo.OtherFiles), bundleInfo.OtherFiles)
	}

	// Create tar.gz bundle using the same logic as the API
	tarData, err := bundleService.CreateBundle(envPath)
	if err != nil {
		return fmt.Errorf("failed to create bundle: %w", err)
	}

	// Save to file
	if err := os.WriteFile(outputPath, tarData, 0644); err != nil {
		return fmt.Errorf("failed to save bundle: %w", err)
	}

	fmt.Printf("✅ Bundle created: %s\n", outputPath)
	fmt.Printf("📊 Size: %d bytes\n", len(tarData))
	fmt.Printf("\n🚀 Install with:\n")
	fmt.Printf("   stn bundle install %s <environment-name>\n", outputPath)
	fmt.Printf("   Or use the Station UI Bundle installation\n")

	return nil
}

func runBundleInstall(cmd *cobra.Command, args []string) error {
	bundleSource := args[0]
	environmentName := args[1]

	fmt.Printf("📦 Installing bundle from: %s\n", bundleSource)
	fmt.Printf("🎯 Target environment: %s\n", environmentName)

	// Use BundleService to install bundle directly (no server dependency)
	bundleService := services.NewBundleService()
	result, err := bundleService.InstallBundle(bundleSource, environmentName)
	if err != nil || !result.Success {
		errorMsg := result.Error
		if errorMsg == "" && err != nil {
			errorMsg = err.Error()
		}
		return fmt.Errorf("bundle installation failed: %s", errorMsg)
	}

	fmt.Printf("✅ Bundle installed successfully!\n")
	fmt.Printf("🎯 Environment '%s' is ready to use\n", result.EnvironmentName)
	fmt.Printf("📊 Installed: %d agents, %d MCP configs\n", result.InstalledAgents, result.InstalledMCPs)
	fmt.Printf("\n🔧 Next steps:\n")
	fmt.Printf("   stn sync %s                  # Sync MCP tools\n", result.EnvironmentName)
	fmt.Printf("   stn agent list --env %s     # List available agents\n", result.EnvironmentName)
	fmt.Printf("   open http://localhost:8585   # View in Station UI\n")

	return nil
}