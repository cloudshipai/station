package examples

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"station/internal/mcp/adapter"
	mcpTesting "station/internal/mcp/testing"
)

// SimpleProxyExample demonstrates basic proxy functionality
func SimpleProxyExample() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("=== MCP Proxy Adapter Example ===")

	// 1. Create mock MCP servers (simulating real filesystem and database servers)
	fmt.Println("1. Creating mock MCP servers...")
	
	fsServer := mcpTesting.NewMockFileSystemServer()
	dbServer := mcpTesting.NewMockDatabaseServer() 
	webServer := mcpTesting.NewMockWebScraperServer()

	fmt.Printf("   - FileSystem server: %d tools\n", len(fsServer.GetTools()))
	fmt.Printf("   - Database server: %d tools\n", len(dbServer.GetTools()))
	fmt.Printf("   - Web scraper server: %d tools\n", len(webServer.GetTools()))

	// 2. Create proxy server for a specific agent
	fmt.Println("\n2. Creating MCP proxy server for agent...")
	
	agentID := int64(12345)
	// Agent only gets access to specific tools from multiple servers
	selectedTools := []string{"read_file", "query_db", "fetch_url"}
	
	proxyConfig := adapter.ProxyServerConfig{
		Name:        "Agent 12345 Proxy",
		Version:     "1.0.0",
		Description: "Proxy server for agent with selected tools",
	}
	
	proxy := adapter.NewMCPProxyServer(agentID, selectedTools, "production", proxyConfig)
	defer proxy.Close()

	// 3. Test in-process connections (simulating real server connections)
	fmt.Println("\n3. Testing in-process server connections...")
	
	// Create in-process clients to simulate real MCP server connections
	fsClient, err := client.NewInProcessClient(fsServer.GetServer())
	if err != nil {
		return fmt.Errorf("failed to create filesystem client: %w", err)
	}
	defer fsClient.Close()

	dbClient, err := client.NewInProcessClient(dbServer.GetServer())
	if err != nil {
		return fmt.Errorf("failed to create database client: %w", err)
	}
	defer dbClient.Close()

	webClient, err := client.NewInProcessClient(webServer.GetServer())
	if err != nil {
		return fmt.Errorf("failed to create web client: %w", err)
	}
	defer webClient.Close()

	// Start all clients
	if err := fsClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start filesystem client: %w", err)
	}

	if err := dbClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start database client: %w", err)
	}

	if err := webClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start web client: %w", err)
	}

	// Initialize all clients
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "Proxy Test Client",
		Version: "1.0.0",
	}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}

	_, err = fsClient.Initialize(ctx, initRequest)
	if err != nil {
		return fmt.Errorf("failed to initialize filesystem client: %w", err)
	}

	_, err = dbClient.Initialize(ctx, initRequest)
	if err != nil {
		return fmt.Errorf("failed to initialize database client: %w", err)
	}

	_, err = webClient.Initialize(ctx, initRequest)
	if err != nil {
		return fmt.Errorf("failed to initialize web client: %w", err)
	}

	// 4. Test tool discovery from individual servers
	fmt.Println("\n4. Testing tool discovery from individual servers...")

	fsTools, err := fsClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list filesystem tools: %w", err)
	}
	fmt.Printf("   - Filesystem tools: %v\n", getToolNames(fsTools.Tools))

	dbTools, err := dbClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list database tools: %w", err)
	}
	fmt.Printf("   - Database tools: %v\n", getToolNames(dbTools.Tools))

	webTools, err := webClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list web scraper tools: %w", err)
	}
	fmt.Printf("   - Web scraper tools: %v\n", getToolNames(webTools.Tools))

	// 5. Test direct tool calls to individual servers
	fmt.Println("\n5. Testing direct tool calls to individual servers...")

	// Test filesystem tool
	readResult, err := fsClient.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "read_file",
			Arguments: map[string]any{"path": "/test/example.txt"},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to call read_file: %w", err)
	}
	fmt.Printf("   - read_file result: %s\n", getResultText(readResult))

	// Test database tool
	queryResult, err := dbClient.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "query_db",
			Arguments: map[string]any{"sql": "SELECT * FROM users LIMIT 5"},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to call query_db: %w", err)
	}
	fmt.Printf("   - query_db result: %s\n", getResultText(queryResult))

	// Test web scraper tool
	fetchResult, err := webClient.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "fetch_url",
			Arguments: map[string]any{"url": "https://example.com"},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to call fetch_url: %w", err)
	}
	fmt.Printf("   - fetch_url result: %s\n", getResultText(fetchResult))

	// 6. Show proxy server statistics
	fmt.Println("\n6. Proxy server statistics:")
	stats := proxy.GetProxyStats()
	fmt.Printf("   - Agent ID: %v\n", stats["agent_id"])
	fmt.Printf("   - Connected servers: %v\n", stats["connected_servers"])
	fmt.Printf("   - Total tools in registry: %v\n", stats["total_tools"])
	fmt.Printf("   - Tools available to agent: %v\n", stats["available_tools"])

	fmt.Println("\n=== Example completed successfully! ===")
	fmt.Println("\nNext steps:")
	fmt.Println("- Implement real server connections in ClientManager")
	fmt.Println("- Add proxy server serving via stdio/http/sse")
	fmt.Println("- Integrate with Station agent execution")
	fmt.Println("- Add comprehensive error handling and logging")

	return nil
}

// Helper functions

func getToolNames(tools []mcp.Tool) []string {
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name
	}
	return names
}

func getResultText(result *mcp.CallToolResult) string {
	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			// Truncate long results for readability
			text := textContent.Text
			if len(text) > 100 {
				return text[:100] + "..."
			}
			return text
		}
	}
	return "No text content"
}

// RunSimpleProxyExample runs the simple proxy example
func RunSimpleProxyExample() {
	if err := SimpleProxyExample(); err != nil {
		log.Fatalf("Simple proxy example failed: %v", err)
	}
}