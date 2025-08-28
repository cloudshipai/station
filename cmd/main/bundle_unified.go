package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"station/cmd/main/handlers/common"
	"station/internal/services"
)

// Unified bundle command that uses the same logic as the API
var bundleCmd = &cobra.Command{
	Use:   "bundle <environment>",
	Short: "Create a bundle from an environment",
	Long: `Create a deployable bundle (.tar.gz) from an environment.
This uses the same bundling logic as the API and creates bundles
that are compatible with the bundle API installation endpoints.

Examples:
  stn bundle default              # Bundle the default environment
  stn bundle production           # Bundle the production environment
  stn bundle default --output my-bundle.tar.gz  # Custom output path`,
	Args: cobra.ExactArgs(1),
	RunE: runBundle,
}

func init() {
	bundleCmd.Flags().String("output", "", "Output path for bundle (defaults to <environment>.tar.gz)")
	bundleCmd.Flags().Bool("local", true, "Save bundle locally (always true for CLI)")
}

func runBundle(cmd *cobra.Command, args []string) error {
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
	fmt.Printf("   Station API: POST /bundles/install {\"bundle_location\": \"%s\", \"environment_name\": \"new-env\", \"source\": \"file\"}\n", outputPath)
	fmt.Printf("   Or copy to another Station instance and use the UI Bundle installation\n")

	return nil
}