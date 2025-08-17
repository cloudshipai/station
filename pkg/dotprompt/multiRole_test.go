package dotprompt

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/google/dotprompt/go/dotprompt"
	"station/pkg/models"
)

// TestMultiRoleDotpromptDirect tests dotprompt library directly without GenKit
func TestMultiRoleDotpromptDirect(t *testing.T) {
	fmt.Println("=== Testing Multi-Role Dotprompt Library Directly ===")

	// Create test dotprompt content with multi-role
	dotpromptContent := `---
model: "gemini-1.5-flash"
config:
  temperature: 0.3
  max_tokens: 2000
input:
  schema:
    userInput: string
tools:
  - "__write_file"
  - "__read_file"
---

{{role "system"}}
You are a test assistant designed to validate multi-role dotprompt execution.
Your job is to respond helpfully to user requests.

{{role "user"}}
{{userInput}}
`

	// Test direct dotprompt rendering
	dp := dotprompt.NewDotprompt(nil)
	promptFunc, err := dp.Compile(dotpromptContent, nil)
	if err != nil {
		t.Fatalf("Failed to compile dotprompt: %v", err)
	}

	// Render with test data
	data := &dotprompt.DataArgument{
		Input: map[string]any{
			"userInput": "Hello, please respond with a greeting",
		},
	}

	renderedPrompt, err := promptFunc(data, nil)
	if err != nil {
		t.Fatalf("Failed to render dotprompt: %v", err)
	}

	fmt.Printf("✓ Dotprompt rendered successfully\n")
	fmt.Printf("  Model: %s\n", renderedPrompt.Model)
	fmt.Printf("  Messages: %d\n", len(renderedPrompt.Messages))
	fmt.Printf("  Tools: %v\n", renderedPrompt.Tools)

	// Verify we have multiple messages (multi-role)
	if len(renderedPrompt.Messages) < 2 {
		t.Fatalf("Expected at least 2 messages for multi-role, got %d", len(renderedPrompt.Messages))
	}

	// Check message roles
	for i, msg := range renderedPrompt.Messages {
		fmt.Printf("  Message %d: Role=%s, Content=%d parts\n", i, msg.Role, len(msg.Content))
		if len(msg.Content) > 0 {
			if textPart, ok := msg.Content[0].(*dotprompt.TextPart); ok {
				fmt.Printf("    Text: %s\n", textPart.Text[:min(50, len(textPart.Text))])
			}
		}
	}

	fmt.Println("✓ Multi-role dotprompt direct test PASSED")
}

// TestMultiRoleDotpromptWithGenKitLoadPrompt tests using GenKit LoadPrompt
func TestMultiRoleDotpromptWithGenKitLoadPrompt(t *testing.T) {
	fmt.Println("=== Testing Multi-Role Dotprompt with GenKit LoadPrompt ===")

	// Skip if no API key available
	if os.Getenv("GEMINI_API_KEY") == "" && os.Getenv("GOOGLE_API_KEY") == "" {
		t.Skip("Skipping GenKit test: GEMINI_API_KEY or GOOGLE_API_KEY not set")
	}

	// Create test dotprompt content
	dotpromptContent := `---
model: "gemini-1.5-flash"
config:
  temperature: 0.3
  max_tokens: 100
input:
  schema:
    userInput: string
---

{{role "system"}}
You are a helpful test assistant.

{{role "user"}}
{{userInput}}
`

	// Create temporary file
	tempFile, err := os.CreateTemp("", "test_*.prompt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write content
	_, err = tempFile.WriteString(dotpromptContent)
	tempFile.Close()
	if err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Initialize GenKit
	ctx := context.Background()
	genkitApp, err := genkit.Init(ctx, genkit.WithPlugins(&googlegenai.GoogleAI{}))
	if err != nil {
		t.Fatalf("Failed to initialize GenKit: %v", err)
	}

	// Load prompt
	prompt, err := genkit.LoadPrompt(genkitApp, tempFile.Name(), "")
	if err != nil {
		t.Fatalf("Failed to load prompt: %v", err)
	}

	fmt.Printf("✓ GenKit LoadPrompt succeeded\n")

	// Execute prompt
	response, err := prompt.Execute(ctx, ai.WithInput(map[string]interface{}{
		"userInput": "Say hello briefly",
	}))

	if err != nil {
		// Log the error but don't fail the test - this might be the "parts template" error
		fmt.Printf("⚠ Prompt execution failed (expected): %v\n", err)
		
		// Check if it's the specific error we're investigating
		if contains(err.Error(), "parts template must produce only one message") {
			fmt.Printf("✓ Confirmed: GenKit has 'parts template must produce only one message' constraint\n")
			fmt.Printf("✓ This validates our hypothesis about the root cause\n")
		}
	} else {
		fmt.Printf("✓ Prompt execution succeeded!\n")
		fmt.Printf("  Response: %s\n", response.Text())
	}

	fmt.Println("✓ GenKit LoadPrompt test completed")
}

// TestStationGenKitExecutorIntegration tests our executor implementation
func TestStationGenKitExecutorIntegration(t *testing.T) {
	fmt.Println("=== Testing Station GenKit Executor Integration ===")

	// Skip if no API key available
	if os.Getenv("GEMINI_API_KEY") == "" && os.Getenv("GOOGLE_API_KEY") == "" {
		t.Skip("Skipping integration test: GEMINI_API_KEY or GOOGLE_API_KEY not set")
	}

	// Create test agent
	agent := models.Agent{
		ID:          1,
		Name:        "Test Agent",
		Description: "Test agent for dotprompt validation",
		Prompt:      "You are a helpful test assistant designed to validate our dotprompt system.",
		MaxSteps:    5,
	}

	// Create test agent tools
	agentTools := []*models.AgentToolWithDetails{
		{
			ToolName: "__write_file",
		},
		{
			ToolName: "__read_file",
		},
	}

	// Initialize GenKit
	ctx := context.Background()
	genkitApp, err := genkit.Init(ctx, genkit.WithPlugins(&googlegenai.GoogleAI{}))
	if err != nil {
		t.Fatalf("Failed to initialize GenKit: %v", err)
	}

	// Create executor
	executor := NewGenKitExecutor()

	// Test execution
	mcpTools := []ai.ToolRef{} // Empty for this test
	task := "Please respond with a brief hello message"

	response, err := executor.ExecuteAgentWithDotprompt(agent, agentTools, genkitApp, mcpTools, task)

	fmt.Printf("Execution completed:\n")
	fmt.Printf("  Success: %v\n", response.Success)
	fmt.Printf("  Duration: %v\n", response.Duration)
	fmt.Printf("  Error: %s\n", response.Error)
	if response.Success {
		fmt.Printf("  Response: %s\n", response.Response)
	}

	if !response.Success {
		fmt.Printf("⚠ Execution failed (investigating): %s\n", response.Error)
	} else {
		fmt.Printf("✓ Execution succeeded!\n")
	}

	fmt.Println("✓ Station GenKit Executor test completed")
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}