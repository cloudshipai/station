package dotprompt_test

import (
	"testing"

	"station/internal/services"
	"station/pkg/dotprompt"
)

// TestSimpleDotpromptExecution demonstrates the dotprompt execution approach
func TestSimpleDotpromptExecution(t *testing.T) {
	// Setup executor
	genkitProvider := services.NewGenKitProvider()
	executor := dotprompt.NewGenkitExecutor(genkitProvider, nil)

	// Create dotprompt config (unused in template rendering test)
	_ = dotprompt.DotpromptConfig{
		Model: "googleai/gemini-1.5-flash",
		Config: dotprompt.GenerationConfig{
			Temperature: &[]float32{0.7}[0],
			MaxTokens:   &[]int{150}[0],
		},
		Metadata: dotprompt.AgentMetadata{
			AgentID:     1,
			Name:        "TestAgent",
			Description: "Test agent for execution comparison",
			MaxSteps:    3,
			Environment: "test",
			Version:     "1.0",
		},
	}

	// Create template
	template := `{{#system}}
You are a test assistant. Your task is to respond concisely to user requests.
Be helpful and accurate in your responses.
{{/system}}

Task: {{task}}

{{#if context}}
Context: {{toJson context}}
{{/if}}

{{#if parameters}}
Parameters: {{toJson parameters}}
{{/if}}

Please provide a brief response.`

	request := dotprompt.ExecutionRequest{
		Task: "Explain what artificial intelligence is in one sentence",
		Context: map[string]interface{}{
			"test_mode": true,
			"provider":  "gemini",
		},
		Parameters: map[string]interface{}{
			"max_length": 50,
			"format":     "concise",
		},
	}

	t.Run("Template_Rendering_Only", func(t *testing.T) {
		// Test template rendering without actual GenKit execution
		rendered, err := executor.RenderTemplate(template, request)
		if err != nil {
			t.Fatalf("Template rendering failed: %v", err)
		}

		t.Logf("Rendered template length: %d characters", len(rendered))
		
		// Validate template rendering
		if len(rendered) < 100 {
			t.Errorf("Rendered template seems too short: %d characters", len(rendered))
		}

		// Check that substitutions occurred
		if !stringContains(rendered, "Explain what artificial intelligence is") {
			t.Error("Task not rendered correctly")
		}
		
		if !stringContains(rendered, "test_mode") {
			t.Error("Context not rendered correctly")
		}
		
		if !stringContains(rendered, "concise") {
			t.Error("Parameters not rendered correctly")
		}

		t.Log("✅ Template rendering validation passed")
	})

	// Commented out actual GenKit execution tests since they require real API keys
	// t.Run("Standard_Generate", func(t *testing.T) {
	// 	result, err := executor.ExecuteAgentWithGenerate(ctx, dotpromptConfig, template, request, []dotprompt.ToolMapping{})
	// 	if err != nil {
	// 		t.Logf("Generate execution failed (expected without API keys): %v", err)
	// 		return
	// 	}
	// 	t.Logf("Generate execution successful: %+v", result)
	// })

	// t.Run("Dotprompt_Template", func(t *testing.T) {
	// 	result, err := executor.ExecuteAgentWithDotpromptTemplate(ctx, dotpromptConfig, template, request, []dotprompt.ToolMapping{})
	// 	if err != nil {
	// 		t.Logf("Dotprompt template execution failed (expected without API keys): %v", err)
	// 		return
	// 	}
	// 	t.Logf("Dotprompt template execution successful: %+v", result)
	// })
}

// Helper function (using different name to avoid redeclaration)
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}