package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/firebase/genkit/go/ai" 
	"github.com/firebase/genkit/go/genkit"
)

// MCPServerDiscovery represents the structured output from GitHub analysis
type MCPServerDiscovery struct {
	ServerName     string            `json:"serverName"`     // e.g. "filesystem", "cfn-mcp-server"
	Description    string            `json:"description"`    // What this server does
	Configurations []MCPConfigOption `json:"configurations"` // Multiple setup options
	RequiredEnv    []EnvVariable     `json:"requiredEnv"`    // Environment variables needed
	Type           string            `json:"type"`           // "stdio", "sse", "streamable-http", "docker"
	Repository     string            `json:"repository"`     // GitHub repository URL
	InstallNotes   string            `json:"installNotes"`   // Additional installation notes
}

// MCPConfigOption represents different ways to configure an MCP server
type MCPConfigOption struct {
	Name        string            `json:"name"`        // "NPX Install", "Docker", "Local Build"
	Command     string            `json:"command"`     // "npx", "docker", "node"
	Args        []string          `json:"args"`        // ["@modelcontextprotocol/server-filesystem", "/path"]
	WorkingDir  string            `json:"workingDir"`  // Optional working directory
	Env         map[string]string `json:"env"`         // Environment variables
	Description string            `json:"description"` // How this option works
	Recommended bool              `json:"recommended"` // Is this the recommended approach?
}

// EnvVariable represents an environment variable requirement
type EnvVariable struct {
	Name        string `json:"name"`        // "AWS_REGION"
	Description string `json:"description"` // "AWS region for CloudFormation"
	Required    bool   `json:"required"`    // Is this env var required?
	Default     string `json:"default"`     // Default value if any
	Example     string `json:"example"`     // Example value
}

// GitHubDiscoveryService handles GitHub MCP server discovery
type GitHubDiscoveryService struct {
	genkit *genkit.Genkit
}

// NewGitHubDiscoveryService creates a new GitHub discovery service
func NewGitHubDiscoveryService(genkitApp *genkit.Genkit) *GitHubDiscoveryService {
	return &GitHubDiscoveryService{
		genkit: genkitApp,
	}
}

// DiscoverMCPServer analyzes a GitHub URL to extract MCP server configuration options
func (g *GitHubDiscoveryService) DiscoverMCPServer(ctx context.Context, githubURL string) (*MCPServerDiscovery, error) {
	// Validate and normalize GitHub URL
	parsedURL, err := url.Parse(githubURL)
	if err != nil {
		return nil, fmt.Errorf("invalid GitHub URL: %w", err)
	}

	if parsedURL.Host != "github.com" {
		return nil, fmt.Errorf("URL must be a GitHub repository")
	}

	// Convert to raw GitHub content URLs for better analysis
	rawURL := g.convertToRawURL(githubURL)
	
	// Create the analysis prompt
	prompt := g.buildAnalysisPrompt(githubURL, rawURL)

	// Use Genkit to analyze the GitHub Repository
	response, err := genkit.Generate(ctx, g.genkit,
		ai.WithPrompt(prompt),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to analyze GitHub repository: %w", err)
	}

	// For now, we'll parse the response as JSON manually
	// TODO: Implement proper structured output parsing  
	var discovery MCPServerDiscovery
	if err := json.Unmarshal([]byte(response.Text()), &discovery); err != nil {
		return nil, fmt.Errorf("failed to parse AI response as JSON: %w", err)
	}

	// Post-process and validate the discovery
	discovery.Repository = githubURL
	g.validateAndEnhanceDiscovery(&discovery)

	return &discovery, nil
}

// convertToRawURL converts GitHub tree URLs to raw content URLs
func (g *GitHubDiscoveryService) convertToRawURL(githubURL string) string {
	// Convert https://github.com/user/repo/tree/main/path to https://raw.githubusercontent.com/user/repo/main/path
	rawURL := strings.Replace(githubURL, "github.com", "raw.githubusercontent.com", 1)
	rawURL = strings.Replace(rawURL, "/tree/", "/", 1)
	return rawURL
}

// buildAnalysisPrompt creates the prompt for analyzing GitHub repositories
func (g *GitHubDiscoveryService) buildAnalysisPrompt(githubURL, rawURL string) string {
	return fmt.Sprintf(`You are an expert at analyzing GitHub repositories for MCP (Model Context Protocol) servers and creating configuration wizards.

Your task is to analyze the GitHub repository at: %s

Please fetch and analyze the repository contents, particularly looking for:
1. README files and documentation
2. Package.json, Dockerfile, or other build files
3. Example configurations
4. Environment variable requirements
5. Installation instructions

Based on your analysis, provide a structured JSON response that will be used to create a configuration wizard for users.

IMPORTANT ANALYSIS GUIDELINES:
- Look for multiple installation methods (npx, docker, local build, etc.)
- Identify all required and optional environment variables
- Extract example configurations from README or docs
- Determine the transport type (stdio is most common, some use sse or streamable-http)
- Note any special setup requirements or dependencies
- Identify the most recommended/easiest installation method

COMMON PATTERNS TO LOOK FOR:

NPX Installation:
- Look for "@modelcontextprotocol/server-*" packages
- Args typically include the package name and paths/options
- Example: npx -y @modelcontextprotocol/server-filesystem /path/to/directory

Docker Installation:
- Look for Dockerfile or docker build instructions
- Args for docker run commands including volume mounts
- Example: docker run -i --rm --mount type=bind,src=/path,dst=/projects

UVX Installation (for Python packages):
- Look for uvx commands
- Often used for AWS or official packages
- Example: uvx awslabs.cfn-mcp-server@latest

Environment Variables:
- AWS services: AWS_PROFILE, AWS_REGION, AWS_ACCESS_KEY_ID, etc.
- Database connections: DATABASE_URL, DB_HOST, etc.
- API keys: API_KEY, TOKEN, etc.
- Service endpoints: NOMAD_ADDR, API_ENDPOINT, etc.

Please provide comprehensive configuration options that would allow a user to easily set up this MCP server.

Repository URL: %s
Raw Content URL: %s

Analyze the repository and return a structured JSON response following the MCPServerDiscovery schema.`, githubURL, githubURL, rawURL)
}

// validateAndEnhanceDiscovery validates and enhances the discovery response
func (g *GitHubDiscoveryService) validateAndEnhanceDiscovery(discovery *MCPServerDiscovery) {
	// Set default type if not specified
	if discovery.Type == "" {
		discovery.Type = "stdio" // Most MCP servers use stdio
	}

	// Ensure at least one configuration option exists
	if len(discovery.Configurations) == 0 {
		// Create a basic configuration based on common patterns
		discovery.Configurations = append(discovery.Configurations, MCPConfigOption{
			Name:        "Basic Setup",
			Command:     "npx",
			Args:        []string{"-y", discovery.ServerName},
			Description: "Standard NPX installation",
			Recommended: true,
		})
	}

	// Mark first config as recommended if none are marked
	hasRecommended := false
	for _, config := range discovery.Configurations {
		if config.Recommended {
			hasRecommended = true
			break
		}
	}
	if !hasRecommended && len(discovery.Configurations) > 0 {
		discovery.Configurations[0].Recommended = true
	}

	// Validate required environment variables
	for i := range discovery.RequiredEnv {
		if discovery.RequiredEnv[i].Name == "" {
			continue
		}
		// Add examples for common environment variables
		switch discovery.RequiredEnv[i].Name {
		case "AWS_PROFILE":
			if discovery.RequiredEnv[i].Example == "" {
				discovery.RequiredEnv[i].Example = "default"
			}
		case "AWS_REGION":
			if discovery.RequiredEnv[i].Example == "" {
				discovery.RequiredEnv[i].Example = "us-east-1"
			}
		case "DATABASE_URL":
			if discovery.RequiredEnv[i].Example == "" {
				discovery.RequiredEnv[i].Example = "postgresql://user:pass@localhost:5432/db"
			}
		}
	}
}

// WebSearchTool creates a web search tool for Genkit
func (g *GitHubDiscoveryService) WebSearchTool() ai.Tool {
	return genkit.DefineTool(g.genkit, "web_search", "Search the web and fetch content from URLs",
		func(ctx *ai.ToolContext, input struct {
			URL   string `json:"url"`
			Query string `json:"query,omitempty"`
		}) (struct {
			Content    string `json:"content"`
			StatusCode int    `json:"status_code"`
			URL        string `json:"url"`
		}, error) {
			// Fetch content from URL
			resp, err := http.Get(input.URL)
			if err != nil {
				return struct {
					Content    string `json:"content"`
					StatusCode int    `json:"status_code"`
					URL        string `json:"url"`
				}{}, fmt.Errorf("failed to fetch URL: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return struct {
					Content    string `json:"content"`
					StatusCode int    `json:"status_code"`
					URL        string `json:"url"`
				}{}, fmt.Errorf("HTTP error: %d", resp.StatusCode)
			}

			// Read response body (limit to reasonable size)
			body := make([]byte, 100*1024) // 100KB limit
			n, _ := resp.Body.Read(body)
			content := string(body[:n])

			return struct {
				Content    string `json:"content"`
				StatusCode int    `json:"status_code"`
				URL        string `json:"url"`
			}{
				Content:    content,
				StatusCode: resp.StatusCode,
				URL:        input.URL,
			}, nil
		},
	)
}