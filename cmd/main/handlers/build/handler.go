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

	log.Printf("Building containerized environment: %s", environmentName)

	configRoot, err := common.GetStationConfigRoot()
	if err != nil {
		return fmt.Errorf("failed to get station config root: %w", err)
	}

	envPath := filepath.Join(configRoot, "environments", environmentName)
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("environment '%s' not found at %s", environmentName, envPath)
	}

	builder := NewEnvironmentBuilder(environmentName, envPath)
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