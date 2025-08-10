package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/firebase/genkit/go/ai" 
	"github.com/firebase/genkit/go/genkit"
	stationGenkit "station/internal/genkit"
)

// MCPServerDiscovery represents the structured output from GitHub analysis
type MCPServerDiscovery struct {
	ServerName     string            `json:"serverName"`               // e.g. "filesystem", "cfn-mcp-server"
	Description    string            `json:"description"`              // What this server does
	Configurations []MCPConfigOption `json:"configurations"`           // Multiple setup options
	RequiredEnv    []EnvVariable     `json:"requiredEnv,omitempty"`    // Environment variables needed
	Type           string            `json:"type,omitempty"`           // "stdio", "sse", "streamable-http", "docker"
	Repository     string            `json:"repository,omitempty"`     // GitHub repository URL (set manually)
	InstallNotes   string            `json:"installNotes,omitempty"`   // Additional installation notes
}

// AIDiscoveryResponse represents the response structure expected from AI (without repository field)
type AIDiscoveryResponse struct {
	ServerName     string            `json:"serverName"`               // e.g. "filesystem", "cfn-mcp-server"
	Description    string            `json:"description"`              // What this server does
	Configurations []MCPConfigOption `json:"configurations"`           // Multiple setup options
	RequiredEnv    []EnvVariable     `json:"requiredEnv,omitempty"`    // Environment variables needed
	Type           string            `json:"type,omitempty"`           // "stdio", "sse", "streamable-http", "docker"
	InstallNotes   string            `json:"installNotes,omitempty"`   // Additional installation notes
}

// MCPConfigOption represents different ways to configure an MCP server
type MCPConfigOption struct {
	Name        string            `json:"name"`                   // "NPX Install", "Docker", "Local Build"
	Command     string            `json:"command"`                // "npx", "docker", "node"
	Args        []string          `json:"args,omitempty"`         // ["@modelcontextprotocol/server-filesystem", "/path"]
	WorkingDir  string            `json:"workingDir,omitempty"`   // Optional working directory
	Env         map[string]string `json:"env,omitempty"`          // Environment variables
	Description string            `json:"description,omitempty"`  // How this option works
	Recommended bool              `json:"recommended,omitempty"`  // Is this the recommended approach?
}

// EnvVariable represents an environment variable requirement
type EnvVariable struct {
	Name        string `json:"name"`                   // "AWS_REGION"
	Description string `json:"description,omitempty"`  // "AWS region for CloudFormation"
	Required    bool   `json:"required,omitempty"`     // Is this env var required?
	Default     string `json:"default,omitempty"`      // Default value if any
	Example     string `json:"example,omitempty"`      // Example value
}

// MCPServerBlock represents a raw MCP server configuration block found in README
type MCPServerBlock struct {
	ServerName  string `json:"serverName"`  // Extracted server name
	RawBlock    string `json:"rawBlock"`    // The actual JSON/configuration block
	Description string `json:"description"` // Brief description of what this server does
	Context     string `json:"context"`     // Surrounding context from README
}

// MCPBlocksResponse represents the response from AI analysis of README
type MCPBlocksResponse struct {
	Blocks []MCPServerBlock `json:"blocks"`
}

// GitHubDiscoveryService handles GitHub MCP server discovery
type GitHubDiscoveryService struct {
	genkit       *genkit.Genkit
	openaiPlugin *stationGenkit.StationOpenAI
}

// NewGitHubDiscoveryService creates a new GitHub discovery service
func NewGitHubDiscoveryService(genkitApp *genkit.Genkit, openaiPlugin *stationGenkit.StationOpenAI) *GitHubDiscoveryService {
	return &GitHubDiscoveryService{
		genkit:       genkitApp,
		openaiPlugin: openaiPlugin,
	}
}

// DiscoverMCPServerBlocks extracts all MCP server configuration blocks from a README
func (g *GitHubDiscoveryService) DiscoverMCPServerBlocks(ctx context.Context, readmeURL string) ([]MCPServerBlock, error) {
	// Fetch the README content directly from the provided URL
	readmeContent, err := g.fetchDirectURL(readmeURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch README content: %w", err)
	}

	// Create prompt to identify all MCP server blocks
	prompt := g.buildMCPBlockExtractionPrompt(readmeContent)

	// Get model from OpenAI plugin - use GPT-4o for better analysis
	model := g.openaiPlugin.Model(g.genkit, "gpt-4o")
	
	// Use Genkit to extract all MCP server blocks with multi-turn support
	response, err := genkit.Generate(ctx, g.genkit,
		ai.WithModel(model),
		ai.WithPrompt(prompt),
		ai.WithOutputType(MCPBlocksResponse{}),
		ai.WithMaxTurns(5), // Allow multi-step analysis of MCP blocks
	)

	if err != nil {
		return nil, fmt.Errorf("failed to analyze README for MCP blocks: %w", err)
	}

	// DEBUG: Log comprehensive response details for MCP block analysis
	log.Printf("🔍 MCP block analysis response: %s", response.Text())
	if response.Usage != nil {
		log.Printf("🔍 MCP block analysis usage - Input tokens: %d, Output tokens: %d, Total tokens: %d", 
			response.Usage.InputTokens, response.Usage.OutputTokens, 
			response.Usage.InputTokens + response.Usage.OutputTokens)
	}
	
	// Count turns taken during MCP block analysis
	if response.Request != nil && len(response.Request.Messages) > 0 {
		modelMessages := 0
		for _, msg := range response.Request.Messages {
			if msg.Role == ai.RoleModel {
				modelMessages++
			}
		}
		log.Printf("🔍 MCP block analysis took %d turns, %d total messages in conversation", 
			modelMessages, len(response.Request.Messages))
	}

	// Get the structured output
	var blocksResponse MCPBlocksResponse
	if err := response.Output(&blocksResponse); err != nil {
		// Fallback: try to parse as JSON if structured output fails
		if jsonErr := json.Unmarshal([]byte(response.Text()), &blocksResponse); jsonErr != nil {
			return nil, fmt.Errorf("failed to parse structured output: %w\nFallback JSON parse error: %v\nResponse: %s", err, jsonErr, response.Text())
		}
	}

	return blocksResponse.Blocks, nil
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

	// Fetch the GitHub page content directly
	pageContent, err := g.fetchGitHubContent(githubURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GitHub content: %w", err)
	}

	// Create enhanced analysis prompt with repository context
	prompt := g.buildEnhancedAnalysisPrompt(githubURL, pageContent)

	// Get model from OpenAI plugin - use GPT-4o for better analysis
	model := g.openaiPlugin.Model(g.genkit, "gpt-4o")
	
	// Use Genkit to analyze the GitHub Repository with structured JSON output and multi-turn support
	response, err := genkit.Generate(ctx, g.genkit,
		ai.WithModel(model),
		ai.WithPrompt(prompt),
		ai.WithOutputType(AIDiscoveryResponse{}),
		ai.WithMaxTurns(5), // Allow multi-step analysis of GitHub repository
	)

	if err != nil {
		return nil, fmt.Errorf("failed to analyze GitHub repository: %w", err)
	}

	// DEBUG: Log comprehensive response details for GitHub repository analysis
	log.Printf("🔍 GitHub repository analysis response: %s", response.Text())
	if response.Usage != nil {
		log.Printf("🔍 GitHub repository analysis usage - Input tokens: %d, Output tokens: %d, Total tokens: %d", 
			response.Usage.InputTokens, response.Usage.OutputTokens, 
			response.Usage.InputTokens + response.Usage.OutputTokens)
	}
	
	// Count turns taken during GitHub repository analysis
	if response.Request != nil && len(response.Request.Messages) > 0 {
		modelMessages := 0
		for _, msg := range response.Request.Messages {
			if msg.Role == ai.RoleModel {
				modelMessages++
			}
		}
		log.Printf("🔍 GitHub repository analysis took %d turns, %d total messages in conversation", 
			modelMessages, len(response.Request.Messages))
	}

	// Get the structured output directly from Genkit
	var aiResponse AIDiscoveryResponse
	if err := response.Output(&aiResponse); err != nil {
		// Fallback: try to parse as JSON if structured output fails
		if jsonErr := json.Unmarshal([]byte(response.Text()), &aiResponse); jsonErr != nil {
			return nil, fmt.Errorf("failed to parse structured output: %w\nFallback JSON parse error: %v\nResponse: %s", err, jsonErr, response.Text())
		}
	}

	// Convert to full discovery struct
	discovery := MCPServerDiscovery{
		ServerName:     aiResponse.ServerName,
		Description:    aiResponse.Description,
		Configurations: aiResponse.Configurations,
		RequiredEnv:    aiResponse.RequiredEnv,
		Type:           aiResponse.Type,
		Repository:     githubURL, // Set manually
		InstallNotes:   aiResponse.InstallNotes,
	}

	// Post-process and validate the discovery
	g.validateAndEnhanceDiscovery(&discovery)

	return &discovery, nil
}


// fetchDirectURL fetches content from any direct URL (including raw GitHub URLs)
func (g *GitHubDiscoveryService) fetchDirectURL(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch content from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error %d fetching content from %s", resp.StatusCode, url)
	}

	// Read response body (limit to reasonable size for README files)
	body := make([]byte, 500*1024) // 500KB limit for README files
	n, _ := resp.Body.Read(body)
	return string(body[:n]), nil
}

// fetchGitHubContent fetches GitHub README content from raw.githubusercontent.com
func (g *GitHubDiscoveryService) fetchGitHubContent(githubURL string) (string, error) {
	// Convert GitHub URL to raw README URL
	// https://github.com/awslabs/mcp/tree/main/src/aws-knowledge-mcp-server -> 
	// https://raw.githubusercontent.com/awslabs/mcp/main/src/aws-knowledge-mcp-server/README.md
	
	rawURL := g.convertGitHubToRawReadme(githubURL)
	
	resp, err := http.Get(rawURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch README from %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error %d fetching README from %s", resp.StatusCode, rawURL)
	}

	// Read response body (limit to reasonable size)
	body := make([]byte, 100*1024) // 100KB limit for README files
	n, _ := resp.Body.Read(body)
	return string(body[:n]), nil
}

// convertGitHubToRawReadme converts GitHub tree URLs to raw README URLs
func (g *GitHubDiscoveryService) convertGitHubToRawReadme(githubURL string) string {
	// Convert https://github.com/awslabs/mcp/tree/main/src/aws-knowledge-mcp-server to
	// https://raw.githubusercontent.com/awslabs/mcp/main/src/aws-knowledge-mcp-server/README.md
	
	rawURL := githubURL + "/README.md"
	rawURL = strings.Replace(rawURL, "github.com", "raw.githubusercontent.com", 1)
	rawURL = strings.Replace(rawURL, "/tree/", "/", 1)
	
	return rawURL
}

// buildMCPBlockExtractionPrompt creates a prompt to extract all MCP server configuration blocks
func (g *GitHubDiscoveryService) buildMCPBlockExtractionPrompt(readmeContent string) string {
	return fmt.Sprintf(`You are an expert at identifying MCP (Model Context Protocol) server configuration blocks in documentation.

TASK: Identify ALL distinct MCP server configuration blocks in the provided README content.

README CONTENT:
%s

Look for JSON configuration blocks that follow the MCP server pattern:
{
  "mcpServers": {
    "server-name": {
      "command": "...",
      "args": [...],
      "env": {...}
    }
  }
}

Or individual server configurations like:
"server-name": {
  "command": "...",
  "args": [...],
  "env": {...}  
}

IMPORTANT INSTRUCTIONS:
1. Find ALL distinct server configurations (not just the first one)
2. Extract the complete JSON block for each server
3. Identify the server name from the JSON key
4. Include surrounding context (the paragraph or section where you found it)
5. Provide a brief description of what each server does based on the documentation

You MUST respond with ONLY valid JSON in exactly this structure:

{
  "blocks": [
    {
      "serverName": "exact-server-name-from-json",
      "rawBlock": "the complete JSON configuration block as a string",
      "description": "brief description of what this server provides",
      "context": "surrounding text/section from README that provides context"
    }
  ]
}

Find ALL MCP server blocks, not just one. Each distinct server configuration should be a separate block.`, readmeContent)
}

// buildEnhancedAnalysisPrompt creates an enhanced prompt for analyzing GitHub repositories
func (g *GitHubDiscoveryService) buildEnhancedAnalysisPrompt(githubURL string, pageContent string) string {
	return fmt.Sprintf(`You are an expert at analyzing GitHub repositories for MCP (Model Context Protocol) servers and understanding their proper configuration structures.

TASK: Analyze the GitHub repository at %s to extract complete MCP server configuration information.

REPOSITORY CONTENT:
%s

Based on the actual repository content provided above, determine the correct server name, installation methods, and configuration options. DO NOT generate generic responses.

MCP SERVER CONFIGURATION STRUCTURE:
MCP servers are configured in mcpServers object:
{
  "mcpServers": {
    "server-name": {
      "command": "npx|docker|python|uvx|node|...",
      "args": ["array", "of", "arguments"],  // CRITICAL: Include ALL needed args
      "env": {
        "ENV_VAR": "value"  // Optional environment variables
      }
    }
  }
}

DEPLOYMENT METHOD PATTERNS:

1. NPX (most common):
   - Command: "npx"
   - Args: ["-y", "@modelcontextprotocol/server-{name}", ...required-args]
   - Look for exact NPX package name in package.json or README

2. UVX (Python):
   - Command: "uvx"  
   - Args: ["mcp-server-{name}", ...args]
   - Look for Python package name

3. Docker:
   - Command: "docker"
   - Args: ["run", "-i", "--rm", "--mount", "type=bind,src=path,dst=/container/path", "image:tag", ...server-args]
   - Include necessary volume mounts

4. Direct execution:
   - Command: "node" or "python"
   - Args: ["path/to/executable", ...args]

CONNECTION PROTOCOLS:
- "stdio": Standard input/output (most common)
- "sse": Server-Sent Events over HTTP (needs port)
- "http": HTTP API (needs port)

CRITICAL ANALYSIS POINTS:
- Extract EXACT package names from npm/PyPI
- Find ALL required arguments (paths, API endpoints, etc.)
- Identify environment variables with examples
- Note port numbers if SSE/HTTP server
- Determine connection protocol type
- Look for multiple deployment options
- Find prerequisite setup steps

CRITICAL: You MUST respond with ONLY valid JSON in exactly this structure. Do NOT include null values:

{
  "serverName": "exact-server-identifier",
  "description": "what tools/capabilities it provides",
  "type": "stdio",
  "configurations": [
    {
      "name": "NPX Install",
      "command": "npx",
      "args": ["-y", "@exact/package-name"],
      "description": "how this option works",
      "recommended": true
    }
  ],
  "requiredEnv": [
    {
      "name": "ENV_VAR_NAME",
      "description": "what this variable does",
      "required": true,
      "example": "example-value"
    }
  ],
  "installNotes": "prerequisites and setup steps"
}

IMPORTANT: 
- command field must be a string like "npx", "docker", "python", etc - NEVER null
- args field must be an array of strings - NEVER null, use empty array [] if no args
- If this is a remote MCP server (like AWS), use type "sse" and include the endpoint URL in args

Analyze the repository content provided and respond with ONLY the JSON structure above.`, 
		githubURL, pageContent)
}


// validateAndEnhanceDiscovery validates and enhances the discovery response
func (g *GitHubDiscoveryService) validateAndEnhanceDiscovery(discovery *MCPServerDiscovery) {
	// Set default type if not specified
	if discovery.Type == "" {
		discovery.Type = "stdio" // Most MCP servers use stdio
	}

	// Ensure at least one configuration option exists and clean up invalid ones
	validConfigs := []MCPConfigOption{}
	for _, config := range discovery.Configurations {
		// Skip configurations with empty command
		if config.Command == "" {
			continue
		}
		// Ensure args is not nil
		if config.Args == nil {
			config.Args = []string{}
		}
		validConfigs = append(validConfigs, config)
	}
	
	discovery.Configurations = validConfigs
	
	if len(discovery.Configurations) == 0 {
		// Create a basic configuration based on common patterns
		if strings.Contains(discovery.ServerName, "aws") {
			// AWS servers are typically remote HTTPS servers
			discovery.Configurations = append(discovery.Configurations, MCPConfigOption{
				Name:        "Remote HTTPS Server",
				Command:     "npx",
				Args:        []string{"-y", "@aws/mcp-client", "https://aws-knowledge-mcp-server.amazonaws.com"},
				Description: "Connect to AWS Knowledge MCP Server",
				Recommended: true,
			})
		} else {
			discovery.Configurations = append(discovery.Configurations, MCPConfigOption{
				Name:        "NPX Install",
				Command:     "npx",
				Args:        []string{"-y", "@modelcontextprotocol/server-" + discovery.ServerName},
				Description: "Standard NPX installation",
				Recommended: true,
			})
		}
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

