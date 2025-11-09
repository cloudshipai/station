package faker

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestMCPContextPropagation tests how MCP SDK propagates context through tool calls
func TestMCPContextPropagation(t *testing.T) {
	fmt.Println("\n=== Testing MCP Context Propagation ===\n")

	// Track what context we receive in the tool handler
	var receivedCtx context.Context
	var receivedDeadline time.Time
	var hasDeadline bool

	// Create a simple MCP faker WITHOUT real target
	faker := &MCPFaker{
		debug: true,
		writeOperations: make(map[string]bool),
		toolSchemas: make(map[string]*mcp.Tool),
	}

	// Override enrichToolResult to capture the context it receives
	originalEnrich := faker.enrichToolResult
	faker.enrichToolResult = func(ctx context.Context, toolName string, result *mcp.CallToolResult) (*mcp.CallToolResult, error) {
		receivedCtx = ctx
		receivedDeadline, hasDeadline = ctx.Deadline()

		fmt.Printf("enrichToolResult received context:\n")
		fmt.Printf("  - Has deadline: %v\n", hasDeadline)
		if hasDeadline {
			fmt.Printf("  - Deadline: %v\n", receivedDeadline)
			fmt.Printf("  - Time until deadline: %v\n", time.Until(receivedDeadline))
		}
		fmt.Printf("  - Is canceled: %v\n", ctx.Err() != nil)
		if ctx.Err() != nil {
			fmt.Printf("  - Error: %v\n", ctx.Err())
		}

		// Call original if it exists
		if originalEnrich != nil {
			return originalEnrich(ctx, toolName, result)
		}
		return result, nil
	}

	// Test 1: What happens when HandleToolCall receives a canceled context?
	fmt.Println("Test 1: HandleToolCall with canceled parent context")
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	request := &mcp.CallToolRequest{
		Params: mcp.CallToolRequestParams{
			Name: "test_tool",
			Arguments: map[string]interface{}{
				"test": "value",
			},
		},
	}

	// This will fail because targetClient is nil, but we can see what context enrichToolResult receives
	_, _ = faker.HandleToolCall(canceledCtx, request)

	// Test 2: What happens with a short timeout?
	fmt.Println("\nTest 2: HandleToolCall with 100ms timeout")
	shortCtx, shortCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer shortCancel()

	_, _ = faker.HandleToolCall(shortCtx, request)

	// Test 3: What happens with a long timeout?
	fmt.Println("\nTest 3: HandleToolCall with 30s timeout")
	longCtx, longCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer longCancel()

	_, _ = faker.HandleToolCall(longCtx, request)
}

// TestActualMCPServerContextFlow tests the real MCP server startup and tool call
func TestActualMCPServerContextFlow(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	fmt.Println("\n=== Testing Real MCP Server Context Flow ===\n")

	// Create a real faker targeting a simple MCP server (filesystem)
	faker, err := NewMCPFaker(
		"npx",
		[]string{"-y", "@modelcontextprotocol/server-filesystem@latest", "/tmp"},
		map[string]string{},
		"Generate realistic test data",
		true, // debug mode
	)
	if err != nil {
		t.Fatalf("Failed to create faker: %v", err)
	}
	defer faker.targetClient.Close()

	// Give target time to start
	time.Sleep(2 * time.Second)

	// Create a request
	request := &mcp.CallToolRequest{
		Params: mcp.CallToolRequestParams{
			Name: "list_directory",
			Arguments: map[string]interface{}{
				"path": "/tmp",
			},
		},
	}

	// Test with different contexts
	fmt.Println("Test 1: Call with canceled context")
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	result, err := faker.HandleToolCall(canceledCtx, request)
	duration := time.Since(start)

	fmt.Printf("  Duration: %v\n", duration)
	fmt.Printf("  Error: %v\n", err)
	fmt.Printf("  Result: %v\n", result != nil)

	fmt.Println("\nTest 2: Call with 100ms timeout")
	shortCtx, shortCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer shortCancel()

	start = time.Now()
	result, err = faker.HandleToolCall(shortCtx, request)
	duration = time.Since(start)

	fmt.Printf("  Duration: %v\n", duration)
	fmt.Printf("  Error: %v\n", err)
	fmt.Printf("  Result: %v\n", result != nil)

	fmt.Println("\nTest 3: Call with 30s timeout (should succeed)")
	longCtx, longCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer longCancel()

	start = time.Now()
	result, err = faker.HandleToolCall(longCtx, request)
	duration = time.Since(start)

	fmt.Printf("  Duration: %v\n", duration)
	fmt.Printf("  Error: %v\n", err)
	fmt.Printf("  Result: %v\n", result != nil)
	if err != nil {
		fmt.Printf("  Error details: %v\n", err)
	}
}

// Helper to print context details
func printContextDetails(ctx context.Context, label string) {
	fmt.Printf("%s:\n", label)
	deadline, hasDeadline := ctx.Deadline()
	fmt.Printf("  Has deadline: %v\n", hasDeadline)
	if hasDeadline {
		fmt.Printf("  Deadline: %v\n", deadline)
		fmt.Printf("  Time until: %v\n", time.Until(deadline))
	}
	fmt.Printf("  Is done: %v\n", ctx.Err() != nil)
	if ctx.Err() != nil {
		fmt.Printf("  Error: %v\n", ctx.Err())
	}
}
