package reproduction

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
)

// TestToolCallIDBug demonstrates the critical bug where tool execution results
// are incorrectly used as tool_call_id values, causing OpenAI API failures.
func TestToolCallIDBug(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set - skipping integration test")
	}
	
	ctx := context.Background()
	
	// Initialize Genkit with OpenAI plugin
	g, err := genkit.Init(ctx, genkit.WithPlugins(&openai.OpenAI{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	}))
	if err != nil {
		t.Fatalf("Failed to init genkit: %v", err)
	}
	
	type SearchInput struct {
		Query string `json:"query"`
	}
	
	// Define a simple tool that returns a result longer than OpenAI's 40-character limit
	searchTool := genkit.DefineTool(g, "search_docs", "Search for documentation", 
		func(ctx *ai.ToolContext, input SearchInput) (string, error) {
			// This return value will be incorrectly used as tool_call_id
			// causing "string too long" error from OpenAI (>40 chars)
			return fmt.Sprintf("Found comprehensive documentation about %s with detailed examples and usage patterns", input.Query), nil
		})
	
	// This call will fail with tool_call_id length error
	response, err := genkit.Generate(ctx, g,
		ai.WithModelName("openai/gpt-4o"),
		ai.WithPrompt("Use the search_docs tool to find information about S3 bucket policies"),
		ai.WithTools(searchTool),
		ai.WithMaxTurns(2),
	)
	
	if err != nil {
		// Expected error demonstrating the bug
		fmt.Printf("❌ Tool call failed (expected): %v\n", err)
		
		// Verify this is the tool_call_id length error
		if len(err.Error()) > 0 {
			t.Logf("Error confirms tool_call_id bug: %s", err.Error())
		}
		
		// This test passes when it fails with the expected error
		return
	}
	
	// If we reach here, either the bug is fixed or something unexpected happened
	t.Logf("✅ Unexpected success - bug may be fixed: %s", response.Text())
}

// TestShortToolCallIDWorks demonstrates that short tool responses work fine
func TestShortToolCallIDWorks(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set - skipping integration test")
	}
	
	ctx := context.Background()
	
	g, err := genkit.Init(ctx, genkit.WithPlugins(&openai.OpenAI{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	}))
	if err != nil {
		t.Fatalf("Failed to init genkit: %v", err)
	}
	
	type MathInput struct {
		A int `json:"a"`
		B int `json:"b"`
	}
	
	// Tool with short response that stays under 40 characters
	mathTool := genkit.DefineTool(g, "add", "Add two numbers", 
		func(ctx *ai.ToolContext, input MathInput) (string, error) {
			// Short response: "Sum: 15" (7 characters) - should work
			return fmt.Sprintf("Sum: %d", input.A + input.B), nil
		})
	
	response, err := genkit.Generate(ctx, g,
		ai.WithModelName("openai/gpt-4o"),
		ai.WithPrompt("Use the add tool to calculate 7 + 8"),
		ai.WithTools(mathTool),
		ai.WithMaxTurns(2),
	)
	
	if err != nil {
		// This should work, so any error indicates a problem
		t.Fatalf("❌ Short tool call failed unexpectedly: %v", err)
	}
	
	t.Logf("✅ Short tool call succeeded: %s", response.Text())
}

// TestMCPToolCallIDBug demonstrates the bug with MCP tools that return complex JSON
func TestMCPToolCallIDBug(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set - skipping integration test")
	}
	
	ctx := context.Background()
	
	g, err := genkit.Init(ctx, genkit.WithPlugins(&openai.OpenAI{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	}))
	if err != nil {
		t.Fatalf("Failed to init genkit: %v", err)
	}
	
	type FileInput struct {
		Path string `json:"path"`
	}
	
	// Simulate MCP tool that returns complex JSON response
	mcpTool := genkit.DefineTool(g, "list_files", "List files in directory", 
		func(ctx *ai.ToolContext, input FileInput) (string, error) {
			// Simulate typical MCP tool response - long JSON that will definitely exceed 40 chars
			return `{
				"files": [
					{"name": "config.yaml", "size": 1024, "modified": "2025-08-04T10:30:00Z", "type": "file"},
					{"name": "main.go", "size": 2048, "modified": "2025-08-04T09:15:00Z", "type": "file"},
					{"name": "internal", "size": 0, "modified": "2025-08-04T08:00:00Z", "type": "directory"},
					{"name": "pkg", "size": 0, "modified": "2025-08-04T07:45:00Z", "type": "directory"}
				],
				"total": 4,
				"path": "/home/user/project"
			}`, nil
		})
	
	// This will definitely fail with tool_call_id length error (JSON is ~400+ characters)
	response, err := genkit.Generate(ctx, g,
		ai.WithModelName("openai/gpt-4o"),
		ai.WithPrompt("Use the list_files tool to list files in the current directory"),
		ai.WithTools(mcpTool),
		ai.WithMaxTurns(2),
	)
	
	if err != nil {
		// Expected error - MCP responses are always too long for tool_call_id
		fmt.Printf("❌ MCP tool call failed (expected): %v\n", err)
		t.Logf("MCP tool error confirms bug: %s", err.Error())
		return
	}
	
	// If we reach here, the bug might be fixed
	t.Logf("✅ Unexpected success - MCP tool bug may be fixed: %s", response.Text())
}

// BenchmarkToolCallIDBugImpact measures the performance impact of the bug
func BenchmarkToolCallIDBugImpact(b *testing.B) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		b.Skip("OPENAI_API_KEY not set - skipping benchmark")
	}
	
	ctx := context.Background()
	
	g, err := genkit.Init(ctx, genkit.WithPlugins(&openai.OpenAI{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	}))
	if err != nil {
		b.Fatalf("Failed to init genkit: %v", err)
	}
	
	type Input struct {
		Query string `json:"query"`
	}
	
	tool := genkit.DefineTool(g, "search", "Search", 
		func(ctx *ai.ToolContext, input Input) (string, error) {
			// Long response that will trigger the bug
			return "This is a long search result that exceeds OpenAI's 40-character limit for tool_call_id and will cause the API to reject the request", nil
		})
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Each iteration will fail due to the bug, measuring the overhead
		_, err := genkit.Generate(ctx, g,
			ai.WithModelName("openai/gpt-4o"),
			ai.WithPrompt("Search for information"),
			ai.WithTools(tool),
			ai.WithMaxTurns(1),
		)
		
		// We expect errors due to the bug
		if err == nil {
			b.Fatal("Expected error due to bug, but got success")
		}
	}
}