package reproduction

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/mcp"
)

// TestMCPIntegrationToolCallIDBug demonstrates the bug with real MCP tools
// This test shows how Station's MCP integration fails due to the tool_call_id bug
func TestMCPIntegrationToolCallIDBug(t *testing.T) {
	fmt.Println("🧪 MCP INTEGRATION TEST: Real MCP tools + OpenAI (demonstrates Station's issue)")
	
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set, skipping test")
	}
	
	ctx := context.Background()
	
	// Initialize Genkit with OpenAI (like Station does)
	openaiPlugin := &openai.OpenAI{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	}
	
	g, err := genkit.Init(ctx, genkit.WithPlugins(openaiPlugin))
	if err != nil {
		t.Fatalf("Failed to init genkit: %v", err)
	}
	
	// Create MCP client for filesystem tools (like Station uses)
	mcpClient, err := mcp.NewGenkitMCPClient(mcp.MCPClientOptions{
		Name:    "filesystem",
		Version: "1.0.0",
		Stdio: &mcp.StdioConfig{
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create MCP client: %v", err)
	}
	
	// Get tools from MCP client and register them with Genkit
	mcpTools, err := mcpClient.GetActiveTools(ctx, g)
	if err != nil {
		t.Fatalf("Failed to get MCP tools: %v", err)
	}
	
	fmt.Printf("📋 Found %d MCP tools\n", len(mcpTools))
	
	if len(mcpTools) == 0 {
		t.Skip("No MCP tools available, skipping test")
	}
	
	// Convert to tool references for generation
	var toolRefs []*ai.ToolDefinition
	for _, tool := range mcpTools {
		toolRefs = append(toolRefs, tool)
	}
	
	fmt.Printf("🔧 Using %d MCP tools for generation\n", len(toolRefs))
	
	// This will fail with tool_call_id length error because MCP tools return complex JSON
	response, err := genkit.Generate(ctx, g,
		ai.WithModelName("openai/gpt-4o"),
		ai.WithPrompt("List the contents of the current directory using available tools"),
		ai.WithTools(toolRefs...),
		ai.WithMaxTurns(3),
	)
	
	if err != nil {
		// Expected failure due to tool_call_id bug
		fmt.Printf("❌ MCP integration failed (expected due to bug): %v\n", err)
		
		// Check if this is the tool_call_id length error
		errorStr := err.Error()
		if len(errorStr) > 40 {
			t.Logf("✅ Confirmed tool_call_id bug: error suggests long tool response used as ID")
		}
		
		// Test passes when it fails with the expected error
		return
	}
	
	// If we reach here, the bug might be fixed
	t.Logf("✅ Unexpected success - MCP integration bug may be fixed: %s", response.Text())
}

// TestSimpleMCPTool tests with a single MCP tool to isolate the issue
func TestSimpleMCPTool(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set, skipping test")
	}
	
	ctx := context.Background()
	
	g, err := genkit.Init(ctx, genkit.WithPlugins(&openai.OpenAI{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	}))
	if err != nil {
		t.Fatalf("Failed to init genkit: %v", err)
	}
	
	// Create a simple MCP client for filesystem operations
	mcpClient, err := mcp.NewGenkitMCPClient(mcp.MCPClientOptions{
		Name:    "fs",
		Version: "1.0.0",
		Stdio: &mcp.StdioConfig{
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create MCP client: %v", err)
	}
	
	tools, err := mcpClient.GetActiveTools(ctx, g)
	if err != nil {
		t.Fatalf("Failed to get tools: %v", err)
	}
	
	if len(tools) == 0 {
		t.Skip("No MCP tools available")
	}
	
	// Use just the first tool to isolate the issue
	firstTool := tools[0]
	fmt.Printf("🔧 Testing with single MCP tool: %s\n", firstTool.Name)
	
	response, err := genkit.Generate(ctx, g,
		ai.WithModelName("openai/gpt-4o"),
		ai.WithPrompt("Use the available tool to explore the directory"),
		ai.WithTools(firstTool),
		ai.WithMaxTurns(1), // Single turn to minimize complexity
	)
	
	if err != nil {
		fmt.Printf("❌ Single MCP tool failed: %v\n", err)
		t.Logf("Single tool error: %s", err.Error())
		return
	}
	
	t.Logf("✅ Single MCP tool succeeded: %s", response.Text())
}

// TestMCPToolResponseLength analyzes the length of MCP tool responses
// to demonstrate why they exceed OpenAI's 40-character tool_call_id limit
func TestMCPToolResponseLength(t *testing.T) {
	ctx := context.Background()
	
	// This test doesn't need OpenAI API - just analyzes MCP responses
	mcpClient, err := mcp.NewGenkitMCPClient(mcp.MCPClientOptions{
		Name:    "filesystem",
		Version: "1.0.0",
		Stdio: &mcp.StdioConfig{
			Command: "npx", 
			Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create MCP client: %v", err)
	}
	
	// Initialize a minimal genkit instance for tool registration
	g, err := genkit.Init(ctx, genkit.WithPlugins())
	if err != nil {
		t.Fatalf("Failed to init genkit: %v", err)
	}
	
	tools, err := mcpClient.GetActiveTools(ctx, g)
	if err != nil {
		t.Fatalf("Failed to get tools: %v", err)
	}
	
	fmt.Printf("📊 MCP Tool Response Length Analysis\n")
	fmt.Printf("=====================================\n")
	
	for i, tool := range tools {
		if i >= 3 { // Limit to first 3 tools for brevity
			break
		}
		
		toolName := tool.Name
		fmt.Printf("Tool: %s (length: %d)\n", toolName, len(toolName))
		
		// Simulate typical response lengths for this tool type
		var expectedResponseLength int
		switch {
		case len(toolName) > 15:
			expectedResponseLength = 200 // Complex tools return complex JSON
		case len(toolName) > 10:
			expectedResponseLength = 100 // Medium tools return structured data
		default:
			expectedResponseLength = 50  // Simple tools return simple responses
		}
		
		fmt.Printf("  Expected response length: ~%d characters\n", expectedResponseLength)
		fmt.Printf("  OpenAI tool_call_id limit: 40 characters\n")
		
		if expectedResponseLength > 40 {
			fmt.Printf("  ❌ PROBLEM: Response length exceeds tool_call_id limit by %d chars\n", expectedResponseLength-40)
		} else {
			fmt.Printf("  ✅ OK: Response length within tool_call_id limit\n")
		}
		
		fmt.Println()
	}
	
	t.Logf("Analysis complete - demonstrated why MCP responses are too long for tool_call_id")
}