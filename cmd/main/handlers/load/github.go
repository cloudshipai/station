package load

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/firebase/genkit/go/genkit"
	oai "github.com/firebase/genkit/go/plugins/compat_oai/openai"

	"station/internal/db"
	"station/internal/services"
)

// isGitHubURL checks if the provided URL is a GitHub repository URL
func isGitHubURL(url string) bool {
	return strings.HasPrefix(url, "https://github.com/") || strings.HasPrefix(url, "http://github.com/")
}

// isDirectReadmeURL checks if the provided URL is a direct README URL
func isDirectReadmeURL(url string) bool {
	return strings.Contains(url, "README.md") &&
		(strings.HasPrefix(url, "https://raw.githubusercontent.com/") ||
			strings.HasPrefix(url, "https://github.com/") ||
			strings.HasPrefix(url, "http://"))
}

// convertToRawGitHubURL converts GitHub blob/tree URLs to raw content URLs
func convertToRawGitHubURL(url string) string {
	// Convert GitHub blob URLs to raw URLs
	// https://github.com/user/repo/blob/branch/path -> https://raw.githubusercontent.com/user/repo/branch/path
	if strings.Contains(url, "github.com") && strings.Contains(url, "/blob/") {
		// Replace github.com with raw.githubusercontent.com and remove /blob/
		url = strings.Replace(url, "github.com", "raw.githubusercontent.com", 1)
		url = strings.Replace(url, "/blob/", "/", 1)
	}

	// Also handle /tree/ URLs (though less common for README files)
	if strings.Contains(url, "github.com") && strings.Contains(url, "/tree/") {
		url = strings.Replace(url, "github.com", "raw.githubusercontent.com", 1)
		url = strings.Replace(url, "/tree/", "/", 1)
	}

	return url
}

// runGitHubDiscoveryFlow handles the GitHub MCP server discovery flow
func (h *LoadHandler) runGitHubDiscoveryFlow(githubURL, environment, endpoint string) error {
	fmt.Printf("🔍 Analyzing GitHub repository: %s\n", githubURL)

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
		return fmt.Errorf("OPENAI_API_KEY environment variable is required for GitHub discovery")
	}

	openaiPlugin := &oai.OpenAI{APIKey: openaiAPIKey}

	// Initialize Genkit with OpenAI plugin for AI analysis
	genkitApp, err := genkit.Init(context.Background(), genkit.WithPlugins(openaiPlugin))
	if err != nil {
		return fmt.Errorf("failed to initialize Genkit: %w", err)
	}

	// Initialize GitHub discovery service
	discoveryService := services.NewGitHubDiscoveryService(genkitApp, openaiPlugin)

	fmt.Println(getCLIStyles(h.themeManager).Info.Render("🤖 Starting AI analysis of repository..."))

	// Discover MCP server configuration
	discovery, err := discoveryService.DiscoverMCPServer(context.Background(), githubURL)
	if err != nil {
		return fmt.Errorf("failed to discover MCP server configuration: %w", err)
	}

	fmt.Printf("✅ Discovered MCP server: %s\n", discovery.ServerName)
	fmt.Printf("📄 Description: %s\n", discovery.Description)
	fmt.Printf("🔧 Found %d configuration option(s)\n", len(discovery.Configurations))

	if len(discovery.RequiredEnv) > 0 {
		fmt.Printf("🔑 Requires %d environment variable(s)\n", len(discovery.RequiredEnv))
	}

	// Launch interactive configuration wizard
	fmt.Println("\n" + getCLIStyles(h.themeManager).Info.Render("🧙 Launching configuration wizard..."))

	wizard := services.NewConfigWizardModel(discovery)
	p := tea.NewProgram(wizard, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run configuration wizard: %w", err)
	}

	// Check if wizard was completed successfully
	wizardModel, ok := finalModel.(*services.ConfigWizardModel)
	if !ok {
		return fmt.Errorf("unexpected model type from wizard")
	}

	if !wizardModel.IsCompleted() {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("Configuration wizard cancelled"))
		return nil
	}

	// Get the final configuration
	finalConfig := wizardModel.GetFinalConfig()
	if finalConfig == nil {
		return fmt.Errorf("no configuration generated from wizard")
	}

	fmt.Printf("✅ Configuration generated: %s\n", finalConfig.Name)

	// Upload the configuration
	return h.uploadGeneratedConfig(finalConfig, environment, endpoint)
}

// runFallbackMCPExtraction handles MCP extraction without AI when OPENAI_API_KEY is not available
func (h *LoadHandler) runFallbackMCPExtraction(readmeURL, environment, endpoint string) error {
	fmt.Printf("🔍 Fetching README content from: %s\n", readmeURL)

	// Fetch README content directly
	resp, err := http.Get(readmeURL)
	if err != nil {
		return fmt.Errorf("failed to fetch README: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error %d fetching README", resp.StatusCode)
	}

	// Read content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read README content: %w", err)
	}

	content := string(body)

	// Parse MCP server blocks using simple pattern matching
	blocks := extractMCPBlocksFromContent(content)

	if len(blocks) == 0 {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("No MCP server configurations found in the README"))
		return nil
	}

	fmt.Printf("✅ Found %d MCP server configuration(s)\n", len(blocks))

	// Launch TurboTax-style wizard
	fmt.Println("\n" + getCLIStyles(h.themeManager).Info.Render("🧙 Launching TurboTax-style configuration wizard..."))

	config, selectedEnv, err := services.RunTurboWizardWithTheme(blocks, []string{"development", "staging", "production"}, h.themeManager)
	if err != nil {
		return fmt.Errorf("failed to run TurboTax wizard: %w", err)
	}

	if config == nil {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("Configuration wizard cancelled"))
		return nil
	}

	// Use selected environment or provided environment
	if environment == "" {
		environment = selectedEnv
	}

	fmt.Printf("✅ Configuration generated with %d server(s) for %s environment\n", len(config.Servers), environment)

	// Upload the configuration (implement this function if needed)
	return h.uploadGeneratedConfig(config, environment, endpoint)
}

// extractMCPBlocksFromContent extracts MCP server blocks using simple pattern matching
func extractMCPBlocksFromContent(content string) []services.MCPServerBlock {
	blocks := []services.MCPServerBlock{}

	// Look for JSON blocks containing mcpServers
	jsonBlockPattern := regexp.MustCompile(`\{[^{}]*"mcpServers"[^{}]*\{[^{}]*\}[^{}]*\}`)
	matches := jsonBlockPattern.FindAllString(content, -1)

	for i, match := range matches {
		// Try to parse the JSON
		var configData map[string]interface{}
		if err := json.Unmarshal([]byte(match), &configData); err != nil {
			continue
		}

		// Extract server configurations
		if mcpServers, ok := configData["mcpServers"].(map[string]interface{}); ok {
			for serverName, serverConfig := range mcpServers {
				if serverMap, ok := serverConfig.(map[string]interface{}); ok {
					description := fmt.Sprintf("MCP server configuration #%d", i+1)

					// Try to determine description from server config
					if command, hasCommand := serverMap["command"].(string); hasCommand {
						description = fmt.Sprintf("STDIO server using %s", command)
					} else if url, hasURL := serverMap["url"].(string); hasURL {
						description = fmt.Sprintf("HTTP server at %s", url)
					}

					blocks = append(blocks, services.MCPServerBlock{
						ServerName:  serverName,
						Description: description,
						RawBlock:    match,
					})
				}
			}
		}
	}

	return blocks
}
