package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"station/pkg/models"
)

// MCPClient handles connections to MCP servers and tool discovery
type MCPClient struct{}

func NewMCPClient() *MCPClient {
	return &MCPClient{}
}

// DiscoverToolsFromServer connects to an MCP server and discovers its tools
func (c *MCPClient) DiscoverToolsFromServer(serverConfig models.MCPServerConfig) ([]mcp.Tool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Convert env map to slice of strings
	var envSlice []string
	for key, value := range serverConfig.Env {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", key, value))
	}

	// Create stdio transport for the server
	stdioTransport := transport.NewStdio(serverConfig.Command, envSlice, serverConfig.Args...)

	// Create client
	mcpClient := client.NewClient(stdioTransport)

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

	log.Printf("Connected to server: %s (version %s)", 
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

	return toolsResult.Tools, nil
}