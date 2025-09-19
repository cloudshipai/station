package build

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"station/cmd/main/handlers/common"
	"station/internal/theme"
)

type BuildHandler struct {
	themeManager *theme.ThemeManager
}

func NewBuildHandler(themeManager *theme.ThemeManager) *BuildHandler {
	return &BuildHandler{
		themeManager: themeManager,
	}
}

func (h *BuildHandler) RunBuildEnvironment(cmd *cobra.Command, args []string) error {
	environmentName := "default"
	if len(args) > 0 {
		environmentName = args[0]
	}

	// Get build configuration flags
	provider, _ := cmd.Flags().GetString("provider")
	model, _ := cmd.Flags().GetString("model")
	cloudshipaiKey, _ := cmd.Flags().GetString("cloudshipai-registration-key")
	cloudshipaiEndpoint, _ := cmd.Flags().GetString("cloudshipai-endpoint")
	installShip, _ := cmd.Flags().GetBool("ship")

	// Validate required flags
	if provider == "" {
		return fmt.Errorf("--provider flag is required (openai, gemini, anthropic)")
	}
	if model == "" {
		return fmt.Errorf("--model flag is required (e.g., gpt-4o-mini, gemini-2.5-flash)")
	}

	log.Printf("Building containerized environment: %s", environmentName)
	log.Printf("AI provider: %s, model: %s", provider, model)

	configRoot, err := common.GetStationConfigRoot()
	if err != nil {
		return fmt.Errorf("failed to get station config root: %w", err)
	}

	envPath := filepath.Join(configRoot, "environments", environmentName)
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("environment '%s' not found at %s", environmentName, envPath)
	}

	// Create build options
	buildOptions := &BuildOptions{
		Provider:            provider,
		Model:              model,
		CloudShipAIKey:     cloudshipaiKey,
		CloudShipAIEndpoint: cloudshipaiEndpoint,
		InstallShip:        installShip,
	}

	builder := NewEnvironmentBuilderWithOptions(environmentName, envPath, buildOptions)
	containerImage, err := builder.Build(context.Background())
	if err != nil {
		return fmt.Errorf("failed to build container: %w", err)
	}

	if strings.HasSuffix(containerImage, ".tar") {
		fmt.Printf("Successfully exported container: %s\n", containerImage)
		fmt.Printf("Load with: docker load < %s\n", containerImage)
		fmt.Printf("Run with: docker run -it station-%s:latest\n", environmentName)
	} else {
		fmt.Printf("Successfully built Docker image: %s\n", containerImage)
		fmt.Printf("Run with: docker run -it %s\n", containerImage)
	}

	return nil
}

func (h *BuildHandler) RunBuildBase(cmd *cobra.Command, args []string) error {
	log.Printf("Building base Station container")

	builder := NewBaseBuilder()
	containerImage, err := builder.Build(context.Background())
	if err != nil {
		return fmt.Errorf("failed to build base container: %w", err)
	}

	if strings.HasSuffix(containerImage, ".tar") {
		fmt.Printf("Successfully exported base container: %s\n", containerImage)
		fmt.Printf("Load with: docker import %s station-base:latest\n", containerImage)
		containerImage = "station-base:latest"
	} else {
		fmt.Printf("Successfully built base Docker image: %s\n", containerImage)
	}
	
	fmt.Printf("\nDeploy with config injection:\n")
	fmt.Printf("docker run -it \\\n")
	fmt.Printf("  -v ./staging/variables.yml:/app/environment/variables.yml \\\n") 
	fmt.Printf("  -v ./staging/config.yml:/app/environment/config.yml \\\n")
	fmt.Printf("  -e OPENAI_API_KEY=$STAGING_OPENAI_KEY \\\n")
	fmt.Printf("  -e ANTHROPIC_API_KEY=$STAGING_ANTHROPIC_KEY \\\n")
	fmt.Printf("  %s\n", containerImage)

	return nil
}