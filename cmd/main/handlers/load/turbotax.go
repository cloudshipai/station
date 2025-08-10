package load

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/firebase/genkit/go/genkit"
	stationGenkit "station/internal/genkit"

	"station/internal/db"
	"station/internal/services"
	"station/internal/services/turbo_wizard"
)

// runTurboTaxMCPFlow handles the TurboTax-style MCP configuration flow
func (h *LoadHandler) runTurboTaxMCPFlow(readmeURL, environment, endpoint string) error {
	// Convert GitHub blob URLs to raw URLs
	readmeURL = convertToRawGitHubURL(readmeURL)
	fmt.Printf("📄 Analyzing README file: %s\n", readmeURL)

	// Initialize Genkit service for discovery
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	// Initialize OpenAI plugin for AI model access
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("⚠️  OPENAI_API_KEY not set, using fallback parser..."))
		return h.runFallbackMCPExtraction(readmeURL, environment, endpoint)
	}

	openaiPlugin := &stationGenkit.StationOpenAI{APIKey: openaiAPIKey}

	// Initialize Genkit with OpenAI plugin for AI analysis
	genkitApp, err := genkit.Init(context.Background(), genkit.WithPlugins(openaiPlugin))
	if err != nil {
		return fmt.Errorf("failed to initialize Genkit: %w", err)
	}

	// Initialize GitHub discovery service
	discoveryService := services.NewGitHubDiscoveryService(genkitApp, openaiPlugin)

	fmt.Println(getCLIStyles(h.themeManager).Info.Render("🤖 Extracting MCP server blocks from README..."))

	// Extract all MCP server blocks from the README
	blocks, err := discoveryService.DiscoverMCPServerBlocks(context.Background(), readmeURL)
	if err != nil {
		return fmt.Errorf("failed to extract MCP server blocks: %w", err)
	}

	if len(blocks) == 0 {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("No MCP server configurations found in the README"))
		return nil
	}

	fmt.Printf("✅ Found %d MCP server configuration(s)\n", len(blocks))

	// Launch TurboTax-style wizard
	fmt.Println("\n" + getCLIStyles(h.themeManager).Info.Render("🧙 Launching TurboTax-style configuration wizard..."))

	// Check if we're in a TTY environment
	if _, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err != nil {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("📋 Non-TTY environment detected, showing configuration preview:"))

		// Show what configurations were found
		for i, block := range blocks {
			fmt.Printf("\n%d. %s - %s\n", i+1, block.ServerName, block.Description)
			fmt.Printf("   Configuration: %s\n", block.RawBlock)
		}

		fmt.Println(getCLIStyles(h.themeManager).Info.Render("\n✨ In a terminal environment, this would launch an interactive TurboTax-style wizard!"))
		return nil
	}

	wizard := services.NewTurboWizardModel(blocks)
	p := tea.NewProgram(wizard, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TurboTax wizard: %w", err)
	}

	// Check if wizard was completed successfully
	// The actual type returned is *turbo_wizard.TurboWizardModel
	wizardModel, ok := finalModel.(*turbo_wizard.TurboWizardModel)
	if !ok {
		fmt.Printf("Debug: received model type: %T\n", finalModel)
		return fmt.Errorf("unexpected model type from wizard: got %T, expected *turbo_wizard.TurboWizardModel", finalModel)
	}

	if wizardModel.IsCancelled() {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("Configuration wizard cancelled"))
		return nil
	}

	if !wizardModel.IsCompleted() {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("Configuration wizard not completed"))
		return nil
	}

	// Get the final configuration
	finalConfig := wizardModel.GetFinalMCPConfig()
	if finalConfig == nil {
		return fmt.Errorf("no configuration generated from wizard")
	}

	fmt.Printf("✅ Configuration generated with %d server(s)\n", len(finalConfig.Servers))

	// Upload the configuration
	return h.uploadGeneratedConfig(finalConfig, environment, endpoint)
}
