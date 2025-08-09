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
	stationGenkit "station/internal/genkit"

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

// isHTTPSURL checks if the provided URL is a general HTTPS URL
func isHTTPSURL(url string) bool {
	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")
}

// isDocumentationURL checks if the URL points to documentation that might contain MCP configs
func isDocumentationURL(url string) bool {
	documentationKeywords := []string{
		"README", "readme", "docs", "documentation", "guide", "tutorial", 
		"setup", "install", "configuration", "config", "mcp", "model-context-protocol",
	}
	
	urlLower := strings.ToLower(url)
	for _, keyword := range documentationKeywords {
		if strings.Contains(urlLower, keyword) {
			return true
		}
	}
	return false
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

	openaiPlugin := &stationGenkit.StationOpenAI{APIKey: openaiAPIKey}

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

// runEnhancedTurboTaxFlow handles HTTPS URLs with enhanced web content parsing and TurboTax wizard
func (h *LoadHandler) runEnhancedTurboTaxFlow(url, environment, endpoint string) error {
	fmt.Printf("🌐 Analyzing web content from: %s\n", url)

	// Fetch web content
	content, err := h.fetchWebContent(url)
	if err != nil {
		return fmt.Errorf("failed to fetch web content: %w", err)
	}

	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("no content found at the provided URL")
	}

	fmt.Println(getCLIStyles(h.themeManager).Info.Render("📄 Content fetched successfully, analyzing for MCP configurations..."))

	// Extract MCP server blocks from the content
	blocks := h.extractMCPConfigurationsFromContent(content, url)

	if len(blocks) == 0 {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("❌ No MCP server configurations found in the content"))
		fmt.Println(getCLIStyles(h.themeManager).Help.Render("💡 Tip: Make sure the URL contains MCP server configurations in JSON format"))
		return nil
	}

	fmt.Printf("✅ Found %d potential MCP server configuration(s)\n", len(blocks))

	// Launch enhanced TurboTax-style wizard
	fmt.Println("\n" + getCLIStyles(h.themeManager).Info.Render("🧙✨ Launching enhanced TurboTax wizard for MCP configuration..."))

	return h.runTurboTaxWizardWithBlocks(blocks, environment, endpoint)
}

// fetchWebContent fetches content from a web URL
func (h *LoadHandler) fetchWebContent(url string) (string, error) {
	fmt.Printf("📡 Fetching content from: %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error %d when fetching URL", resp.StatusCode)
	}

	// Read content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

// extractMCPConfigurationsFromContent extracts MCP configurations from web content using multiple strategies
func (h *LoadHandler) extractMCPConfigurationsFromContent(content, sourceURL string) []services.MCPServerBlock {
	var allBlocks []services.MCPServerBlock

	// Strategy 1: Look for JSON code blocks with mcpServers
	jsonBlocks := h.extractJSONBlocks(content)
	allBlocks = append(allBlocks, jsonBlocks...)

	// Strategy 2: Look for configuration examples in markdown
	markdownBlocks := h.extractMarkdownConfigBlocks(content)
	allBlocks = append(allBlocks, markdownBlocks...)

	// Strategy 3: Look for npm/npx command examples
	commandBlocks := h.extractCommandBlocks(content)
	allBlocks = append(allBlocks, commandBlocks...)

	// Add source information to context for each block
	for i := range allBlocks {
		if allBlocks[i].Context == "" {
			allBlocks[i].Context = fmt.Sprintf("Source: %s", sourceURL)
		} else {
			allBlocks[i].Context = fmt.Sprintf("%s (Source: %s)", allBlocks[i].Context, sourceURL)
		}
	}

	return allBlocks
}

// extractJSONBlocks extracts JSON blocks containing MCP server configurations
func (h *LoadHandler) extractJSONBlocks(content string) []services.MCPServerBlock {
	blocks := []services.MCPServerBlock{}

	// Enhanced regex patterns for finding JSON MCP configurations
	patterns := []string{
		// Standard JSON blocks with mcpServers
		`\{[^{}]*"mcpServers"[^{}]*\{[^{}]*\}[^{}]*\}`,
		// Larger JSON blocks that might contain nested configurations
		`\{(?:[^{}]|\{[^{}]*\})*"mcpServers"(?:[^{}]|\{[^{}]*\})*\}`,
		// JSON within code blocks
		"```json\\s*(\\{(?:[^{}]|\\{[^{}]*\\})*\"mcpServers\"(?:[^{}]|\\{[^{}]*\\})*\\})\\s*```",
	}

	for _, pattern := range patterns {
		jsonBlockPattern := regexp.MustCompile(pattern)
		matches := jsonBlockPattern.FindAllString(content, -1)

		for i, match := range matches {
			// Clean up the match (remove markdown code block markers if present)
			cleanMatch := strings.TrimSpace(match)
			cleanMatch = strings.TrimPrefix(cleanMatch, "```json")
			cleanMatch = strings.TrimSuffix(cleanMatch, "```")
			cleanMatch = strings.TrimSpace(cleanMatch)

			// Try to parse the JSON
			var configData map[string]interface{}
			if err := json.Unmarshal([]byte(cleanMatch), &configData); err != nil {
				continue
			}

			// Extract server configurations
			if mcpServers, ok := configData["mcpServers"].(map[string]interface{}); ok {
				for serverName, serverConfig := range mcpServers {
					if serverMap, ok := serverConfig.(map[string]interface{}); ok {
						description := h.generateServerDescription(serverName, serverMap, i+1)

						blocks = append(blocks, services.MCPServerBlock{
							ServerName:  serverName,
							Description: description,
							RawBlock:    cleanMatch,
							Context:     "JSON configuration block",
						})
					}
				}
			}
		}
	}

	return blocks
}

// extractMarkdownConfigBlocks extracts configuration examples from markdown sections
func (h *LoadHandler) extractMarkdownConfigBlocks(content string) []services.MCPServerBlock {
	blocks := []services.MCPServerBlock{}

	// Look for headers and sections that might contain MCP configurations
	configSectionPattern := regexp.MustCompile(`(?i)##?\s*(configuration|config|setup|installation|usage|example)[^\n]*\n([\s\S]*?)(?:\n##|\n#|$)`)
	matches := configSectionPattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 2 {
			sectionContent := match[2]
			// Look for JSON in this section
			jsonBlocks := h.extractJSONBlocks(sectionContent)
			blocks = append(blocks, jsonBlocks...)
		}
	}

	return blocks
}

// extractCommandBlocks extracts MCP server configurations from command examples
func (h *LoadHandler) extractCommandBlocks(content string) []services.MCPServerBlock {
	blocks := []services.MCPServerBlock{}

	// Look for npx/npm commands that might be MCP servers
	commandPattern := regexp.MustCompile(`(?i)(npx|npm)\s+[^\n]*(@?[a-zA-Z0-9-_./]+)`)
	matches := commandPattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 2 {
			command := strings.TrimSpace(match[0])
			packageName := strings.TrimSpace(match[2])

			// Check if this looks like an MCP server
			if h.looksLikeMCPServer(packageName, command) {
				serverName := h.extractServerNameFromPackage(packageName)
				description := fmt.Sprintf("MCP server from command: %s", command)

				// Generate a basic JSON configuration
				jsonConfig := h.generateJSONFromCommand(serverName, command)

				blocks = append(blocks, services.MCPServerBlock{
					ServerName:  serverName,
					Description: description,
					RawBlock:    jsonConfig,
					Context:     "Generated from command: " + command,
				})
			}
		}
	}

	return blocks
}

// Helper functions

func (h *LoadHandler) generateServerDescription(serverName string, serverMap map[string]interface{}, index int) string {
	if command, hasCommand := serverMap["command"].(string); hasCommand {
		if args, hasArgs := serverMap["args"].([]interface{}); hasArgs && len(args) > 0 {
			return fmt.Sprintf("STDIO server: %s %v", command, args[0])
		}
		return fmt.Sprintf("STDIO server using %s", command)
	} else if url, hasURL := serverMap["url"].(string); hasURL {
		return fmt.Sprintf("HTTP server at %s", url)
	}
	return fmt.Sprintf("MCP server configuration #%d", index)
}

func (h *LoadHandler) looksLikeMCPServer(packageName, command string) bool {
	mcpKeywords := []string{"mcp", "model-context-protocol", "server", "@modelcontextprotocol"}
	
	packageLower := strings.ToLower(packageName)
	commandLower := strings.ToLower(command)
	
	for _, keyword := range mcpKeywords {
		if strings.Contains(packageLower, keyword) || strings.Contains(commandLower, keyword) {
			return true
		}
	}
	
	return false
}

func (h *LoadHandler) extractServerNameFromPackage(packageName string) string {
	// Extract server name from package name
	// @modelcontextprotocol/server-github -> github
	// mcp-sqlite -> sqlite
	
	parts := strings.Split(packageName, "/")
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		if strings.HasPrefix(lastPart, "server-") {
			return strings.TrimPrefix(lastPart, "server-")
		}
		return lastPart
	}
	
	if strings.HasPrefix(packageName, "mcp-") {
		return strings.TrimPrefix(packageName, "mcp-")
	}
	
	return packageName
}

func (h *LoadHandler) generateJSONFromCommand(serverName, command string) string {
	// Parse the command to extract parts
	parts := strings.Fields(command)
	if len(parts) < 2 {
		return ""
	}

	cmd := parts[0] // npm or npx
	var args []string
	
	if len(parts) > 1 {
		args = parts[1:] // everything after npm/npx
	}

	// Generate basic JSON configuration
	config := map[string]interface{}{
		"name": fmt.Sprintf("%s-config", serverName),
		"mcpServers": map[string]interface{}{
			serverName: map[string]interface{}{
				"command": cmd,
				"args":    args,
			},
		},
	}

	jsonBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return ""
	}

	return string(jsonBytes)
}

// runTurboTaxWizardWithBlocks launches the TurboTax wizard with detected MCP server blocks
func (h *LoadHandler) runTurboTaxWizardWithBlocks(blocks []services.MCPServerBlock, environment, endpoint string) error {
	fmt.Printf("🧙 Starting TurboTax wizard with %d MCP server configurations\n", len(blocks))

	// Define available environments
	availableEnvs := []string{"development", "staging", "production"}
	if environment != "" {
		// Add the specified environment if it's not in the list
		found := false
		for _, env := range availableEnvs {
			if env == environment {
				found = true
				break
			}
		}
		if !found {
			availableEnvs = append([]string{environment}, availableEnvs...)
		}
	}

	// Launch the TurboTax-style wizard
	config, selectedEnv, err := services.RunTurboWizardWithTheme(blocks, availableEnvs, h.themeManager)
	if err != nil {
		return fmt.Errorf("failed to run TurboTax wizard: %w", err)
	}

	if config == nil {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🚫 TurboTax wizard cancelled by user"))
		return nil
	}

	// Use selected environment or provided environment
	if environment == "" {
		environment = selectedEnv
	}

	fmt.Printf("✅ Configuration completed! Generated %d server(s) for %s environment\n", len(config.Servers), environment)

	// Upload the generated configuration
	return h.uploadGeneratedConfig(config, environment, endpoint)
}
