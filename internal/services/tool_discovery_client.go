package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"station/pkg/models"
)

// MCPClient handles connections to MCP servers and tool discovery using any transport
type MCPClient struct{}

func NewMCPClient() *MCPClient {
	return &MCPClient{}
}

// DiscoverToolsFromRenderedConfig connects to MCP servers using rendered JSON config and discovers tools
func (c *MCPClient) DiscoverToolsFromRenderedConfig(renderedConfigJSON string) (map[string][]mcp.Tool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	log.Printf("Discovering tools from rendered MCP configuration")

	// Parse the rendered JSON to extract server configurations
	// We still need minimal parsing to extract individual server configs for mcp-go
	var configData map[string]interface{}
	if err := json.Unmarshal([]byte(renderedConfigJSON), &configData); err != nil {
		return nil, fmt.Errorf("invalid rendered config JSON: %w", err)
	}

	mcpServers, ok := configData["mcpServers"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no mcpServers found in configuration")
	}

	results := make(map[string][]mcp.Tool)

	// Process each server individually
	for serverName, serverConfigRaw := range mcpServers {
		log.Printf("Processing MCP server: %s", serverName)

		// Convert server config to JSON for mcp-go consumption
		serverConfigJSON, err := json.Marshal(serverConfigRaw)
		if err != nil {
			log.Printf("Failed to marshal config for server %s: %v", serverName, err)
			continue
		}

		// Discover tools from this server
		tools, err := c.discoverToolsFromServerConfig(ctx, serverName, serverConfigJSON)
		if err != nil {
			log.Printf("Failed to discover tools from server %s: %v", serverName, err)
			continue
		}

		results[serverName] = tools
		log.Printf("Discovered %d tools from server %s", len(tools), serverName)
	}

	return results, nil
}

// discoverToolsFromServerConfig handles individual server tool discovery
func (c *MCPClient) discoverToolsFromServerConfig(ctx context.Context, serverName string, serverConfigJSON []byte) ([]mcp.Tool, error) {
	// Parse server config to determine transport type
	var serverConfig map[string]interface{}
	if err := json.Unmarshal(serverConfigJSON, &serverConfig); err != nil {
		return nil, fmt.Errorf("invalid server config: %w", err)
	}

	// Create appropriate transport based on server configuration
	mcpTransport, err := c.createTransportFromConfig(serverConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %v", err)
	}

	// Create universal mcp-go client
	mcpClient := client.NewClient(mcpTransport)

	// Start connection
	if err := mcpClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start client: %v", err)
	}
	defer mcpClient.Close()

	// Initialize MCP session
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "Station Tool Discovery",
		Version: "1.0.0",
	}

	serverInfo, err := mcpClient.Initialize(ctx, initRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize: %v", err)
	}

	log.Printf("Connected to MCP server: %s (version %s)", 
		serverInfo.ServerInfo.Name, 
		serverInfo.ServerInfo.Version)

	// Check capabilities
	if serverInfo.Capabilities.Tools == nil {
		log.Printf("Server %s does not support tools", serverName)
		return []mcp.Tool{}, nil
	}

	// Discover tools
	toolsRequest := mcp.ListToolsRequest{}
	toolsResult, err := mcpClient.ListTools(ctx, toolsRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %v", err)
	}

	return toolsResult.Tools, nil
}

// DEPRECATED: Use DiscoverToolsFromRenderedConfig instead
// DiscoverToolsFromServer connects to an MCP server using the appropriate transport and discovers its tools
func (c *MCPClient) DiscoverToolsFromServer(serverConfig models.MCPServerConfig) ([]mcp.Tool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second) // Increased timeout for HTTP transports
	defer cancel()

	// Create appropriate transport based on server configuration
	mcpTransport, err := c.createTransport(serverConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %v", err)
	}

	// Create client with the transport
	mcpClient := client.NewClient(mcpTransport)

	// Start the client
	if err := mcpClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start client: %v", err)
	}
	defer mcpClient.Close()

	// Initialize the client
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "Station Tool Discovery",
		Version: "1.0.0",
	}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}

	serverInfo, err := mcpClient.Initialize(ctx, initRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize: %v", err)
	}

	log.Printf("Connected to MCP server: %s (version %s)", 
		serverInfo.ServerInfo.Name, 
		serverInfo.ServerInfo.Version)

	// Check if server supports tools
	if serverInfo.Capabilities.Tools == nil {
		log.Printf("Server does not support tools")
		return []mcp.Tool{}, nil
	}

	// List available tools
	toolsRequest := mcp.ListToolsRequest{}
	toolsResult, err := mcpClient.ListTools(ctx, toolsRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %v", err)
	}

	log.Printf("Discovered %d tools from MCP server", len(toolsResult.Tools))
	return toolsResult.Tools, nil
}

// createTransportFromConfig creates transport from generic config map (for rendered configs)
func (c *MCPClient) createTransportFromConfig(serverConfig map[string]interface{}) (transport.Interface, error) {
	// Extract fields from generic config map
	var command, url string
	var args []string
	var env map[string]string

	// Extract command (for stdio transport)
	if cmdValue, ok := serverConfig["command"]; ok {
		if cmdStr, ok := cmdValue.(string); ok {
			command = cmdStr
		}
	}

	// Extract URL (for HTTP/SSE transport)
	if urlValue, ok := serverConfig["url"]; ok {
		if urlStr, ok := urlValue.(string); ok {
			url = urlStr
		}
	}

	// Extract args
	if argsValue, ok := serverConfig["args"]; ok {
		if argsList, ok := argsValue.([]interface{}); ok {
			for _, arg := range argsList {
				if argStr, ok := arg.(string); ok {
					args = append(args, argStr)
				}
			}
		}
	}

	// Extract env
	env = make(map[string]string)
	if envValue, ok := serverConfig["env"]; ok {
		if envMap, ok := envValue.(map[string]interface{}); ok {
			for key, value := range envMap {
				if valStr, ok := value.(string); ok {
					env[key] = valStr
				}
			}
		}
	}

	// Create transport using existing logic
	return c.createTransportFromFields(command, url, args, env)
}

// createTransportFromFields creates transport from individual fields
func (c *MCPClient) createTransportFromFields(command, url string, args []string, env map[string]string) (transport.Interface, error) {
	// Option 1: Stdio transport (subprocess-based)
	if command != "" {
		log.Printf("Creating stdio transport for command: %s", command)
		
		// Convert env map to slice of strings
		var envSlice []string
		for key, value := range env {
			envSlice = append(envSlice, fmt.Sprintf("%s=%s", key, value))
		}
		
		return transport.NewStdio(command, envSlice, args...), nil
	}
	
	// Option 2: URL-based transports (HTTP/SSE)
	if url != "" {
		return c.createHTTPTransport(url, env)
	}
	
	// Option 3: Backwards compatibility - check args for URLs
	for _, arg := range args {
		if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
			return c.createHTTPTransport(arg, env)
		}
	}
	
	return nil, fmt.Errorf("no valid transport configuration found - provide either 'command' for stdio transport or 'url' for HTTP/SSE transport")
}

// createTransport creates the appropriate transport based on the MCP server configuration (DEPRECATED)
func (c *MCPClient) createTransport(serverConfig models.MCPServerConfig) (transport.Interface, error) {
	// Option 1: Stdio transport (subprocess-based)
	if serverConfig.Command != "" {
		log.Printf("Creating stdio transport for command: %s", serverConfig.Command)
		
		// Convert env map to slice of strings
		var envSlice []string
		for key, value := range serverConfig.Env {
			envSlice = append(envSlice, fmt.Sprintf("%s=%s", key, value))
		}
		
		return transport.NewStdio(serverConfig.Command, envSlice, serverConfig.Args...), nil
	}
	
	// Option 2: URL-based transports (HTTP/SSE)
	if serverConfig.URL != "" {
		return c.createHTTPTransport(serverConfig.URL, serverConfig.Env)
	}
	
	// Option 3: Check if we have a URL in the args or other fields for backwards compatibility
	for _, arg := range serverConfig.Args {
		if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
			return c.createHTTPTransport(arg, serverConfig.Env)
		}
	}
	
	return nil, fmt.Errorf("no valid transport configuration found - provide either 'command' for stdio transport or 'url' for HTTP/SSE transport")
}

// createHTTPTransport creates an HTTP-based transport (SSE or Streamable HTTP)
func (c *MCPClient) createHTTPTransport(baseURL string, envVars map[string]string) (transport.Interface, error) {
	// Parse URL to validate format
	_, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL format: %v", err)
	}
	
	log.Printf("Creating HTTP transport for URL: %s", baseURL)
	
	// Prepare SSE client options
	var options []transport.ClientOption
	
	// Add headers from environment variables
	if len(envVars) > 0 {
		headers := make(map[string]string)
		for key, value := range envVars {
			// Convert common env var patterns to HTTP headers
			if strings.HasPrefix(key, "HTTP_") {
				headerName := strings.ReplaceAll(strings.TrimPrefix(key, "HTTP_"), "_", "-")
				headers[headerName] = value
			} else if key == "AUTHORIZATION" || key == "AUTH_TOKEN" {
				headers["Authorization"] = value
			} else if key == "API_KEY" {
				headers["X-API-Key"] = value
			}
		}
		if len(headers) > 0 {
			options = append(options, transport.WithHeaders(headers))
		}
	}
	
	// Use SSE transport for HTTP URLs (most widely supported)
	log.Printf("Using SSE transport for URL: %s", baseURL)
	return transport.NewSSE(baseURL, options...)
}