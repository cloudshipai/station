package faker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestFakerPassthrough tests that faker correctly proxies without AI enrichment
func TestFakerPassthrough(t *testing.T) {
	// Create MCP client to the faker server
	fakerClient, err := client.NewStdioMCPClient("go", nil,
		"run", "../../cmd/main",
		"faker",
		"--command", "npx",
		"--args", "-y,@modelcontextprotocol/server-filesystem,/tmp",
		"--debug")
	if err != nil {
		t.Fatalf("Failed to create faker client: %v", err)
	}

	// Initialize the client
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: "2024-11-05",
			ClientInfo: mcp.Implementation{
				Name:    "faker-test-client",
				Version: "1.0.0",
			},
		},
	}

	_, err = fakerClient.Initialize(ctx, initReq)
	if err != nil {
		t.Fatalf("Failed to initialize faker client: %v", err)
	}

	t.Log("✓ Faker client initialized")

	// List tools from faker (should proxy from filesystem server)
	toolsResult, err := fakerClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("Failed to list tools: %v", err)
	}

	t.Logf("✓ Faker returned %d tools from target", len(toolsResult.Tools))

	if len(toolsResult.Tools) == 0 {
		t.Fatal("Expected faker to return tools from target filesystem server")
	}

	// Call a tool through the faker
	callReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "list_directory",
			Arguments: map[string]interface{}{
				"path": "/tmp",
			},
		},
	}

	result, err := fakerClient.CallTool(ctx, callReq)
	if err != nil {
		t.Fatalf("Failed to call tool through faker: %v", err)
	}

	if result.IsError {
		t.Fatal("Tool call returned an error")
	}

	if len(result.Content) == 0 {
		t.Fatal("Tool call returned empty content")
	}

	// Log the result
	resultJSON, _ := json.MarshalIndent(result.Content, "", "  ")
	t.Logf("✓ Successfully proxied tool call, got result:\n%s", string(resultJSON))
}

// TestFakerWithAIEnrichment tests that faker enriches responses with AI
func TestFakerWithAIEnrichment(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("Skipping: OPENAI_API_KEY not set")
	}

	// Create MCP client to faker with AI enrichment enabled
	fakerClient, err := client.NewStdioMCPClient("go", nil,
		"run", "../../cmd/main",
		"faker",
		"--command", "npx",
		"--args", "-y,@modelcontextprotocol/server-filesystem,/tmp",
		"--ai-instruction", "Generate realistic filesystem listings with varied file types, sizes, and timestamps",
		"--debug")
	if err != nil {
		t.Fatalf("Failed to create faker client: %v", err)
	}

	// Initialize the client
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: "2024-11-05",
			ClientInfo: mcp.Implementation{
				Name:    "faker-test-client",
				Version: "1.0.0",
			},
		},
	}

	_, err = fakerClient.Initialize(ctx, initReq)
	if err != nil {
		t.Fatalf("Failed to initialize faker client: %v", err)
	}

	t.Log("✓ Faker client initialized with AI enrichment")

	// Call a tool through the faker (should get enriched response)
	callReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "list_directory",
			Arguments: map[string]interface{}{
				"path": "/tmp",
			},
		},
	}

	result, err := fakerClient.CallTool(ctx, callReq)
	if err != nil {
		t.Fatalf("Failed to call tool through faker: %v", err)
	}

	if result.IsError {
		t.Fatal("Tool call returned an error")
	}

	if len(result.Content) == 0 {
		t.Fatal("Tool call returned empty content")
	}

	// Log the enriched result
	enrichedJSON, _ := json.MarshalIndent(result.Content, "", "  ")
	fmt.Printf("\n✓ Successfully enriched tool call result:\n%s\n", string(enrichedJSON))
}
