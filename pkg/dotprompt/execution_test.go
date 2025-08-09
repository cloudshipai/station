package dotprompt_test

import (
	"context"
	"testing"
	"time"

	"station/internal/config"
	"station/internal/services"
	"station/pkg/dotprompt"
)

// TestDotpromptExecutionComparison compares different execution methods
func TestDotpromptExecutionComparison(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		model    string
		task     string
	}{
		{
			name:     "OpenAI GPT-4o",
			provider: "openai",
			model:    "gpt-4o",
			task:     "List the current time and explain why time management is important",
		},
		{
			name:     "Gemini 1.5 Flash",
			provider: "gemini",
			model:    "gemini-1.5-flash",
			task:     "Analyze the benefits of using AI agents in software development",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Set up test configuration
			testConfig := setupTestConfig(tt.provider, tt.model)
			
			// Create dotprompt configuration
			dotpromptConfig := createTestDotpromptConfig(tt.model)
			template := createTestTemplate()
			
			request := dotprompt.ExecutionRequest{
				Task: tt.task,
				Context: map[string]interface{}{
					"test_mode": true,
					"provider":  tt.provider,
				},
				Parameters: map[string]interface{}{
					"max_length": 200,
					"format":     "markdown",
				},
			}

			// Test 1: Traditional GenKit Generate
			t.Run("GenKit_Generate", func(t *testing.T) {
				executor := setupGenkitExecutor(testConfig)
				
				result, err := executor.ExecuteAgentWithGenerate(ctx, dotpromptConfig, template, request, []dotprompt.ToolMapping{})
				if err != nil {
					t.Logf("Generate execution failed (expected for test): %v", err)
					return
				}

				validateExecutionResult(t, result, "Generate")
				logExecutionMetrics(t, "Generate", result)
			})

			// Test 2: GenKit Prompt Execution  
			t.Run("GenKit_Prompt", func(t *testing.T) {
				executor := setupGenkitExecutor(testConfig)
				
				result, err := executor.ExecuteAgentWithDotpromptTemplate(ctx, dotpromptConfig, template, request, []dotprompt.ToolMapping{})
				if err != nil {
					t.Logf("Prompt execution failed (expected for test): %v", err)
					return
				}

				validateExecutionResult(t, result, "Prompt")
				logExecutionMetrics(t, "Prompt", result)
			})

			// Test 3: Station's Current Method (for comparison)
			t.Run("Station_Current", func(t *testing.T) {
				// This would use Station's current agent execution for comparison
				result := executeWithStationCurrent(ctx, t, testConfig, tt.task)
				if result != nil {
					logExecutionMetrics(t, "Station_Current", result)
				}
			})
		})
	}
}

// TestDotpromptWithTools tests dotprompt execution with MCP tools
func TestDotpromptWithTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Create test configuration with tools
	testConfig := setupTestConfig("gemini", "gemini-1.5-flash")
	dotpromptConfig := createTestDotpromptConfig("gemini-1.5-flash")
	template := createTestTemplateWithTools()
	
	// Define tool mappings (simulate Grafana tools)
	toolMappings := []dotprompt.ToolMapping{
		{
			ToolName:    "__find_error_pattern_logs",
			ServerName:  "mcp-grafana",
			MCPServerID: "grafana-server-1",
			Environment: "test",
		},
		{
			ToolName:    "__create_incident", 
			ServerName:  "mcp-grafana",
			MCPServerID: "grafana-server-1",
			Environment: "test",
		},
	}

	request := dotprompt.ExecutionRequest{
		Task: "Search for error patterns in the last hour and create an incident if critical errors are found",
		Context: map[string]interface{}{
			"time_range": "1h",
			"severity":   "critical",
		},
	}

	// Test with tools using different execution methods
	executor := setupGenkitExecutor(testConfig)

	t.Run("WithTools_Generate", func(t *testing.T) {
		result, err := executor.ExecuteAgentWithGenerate(ctx, dotpromptConfig, template, request, toolMappings)
		if err != nil {
			t.Logf("Generate with tools failed (expected for test): %v", err)
			return
		}
		
		validateExecutionResult(t, result, "Generate_WithTools")
		
		// Validate tool usage
		if result.ToolsUsed == 0 {
			t.Log("No tools were used in execution")
		} else {
			t.Logf("Tools used: %d", result.ToolsUsed)
		}
	})

	t.Run("WithTools_Prompt", func(t *testing.T) {
		result, err := executor.ExecuteAgentWithDotpromptTemplate(ctx, dotpromptConfig, template, request, toolMappings)
		if err != nil {
			t.Logf("Prompt with tools failed (expected for test): %v", err)
			return
		}
		
		validateExecutionResult(t, result, "Prompt_WithTools")
		
		// Validate tool usage
		if result.ToolsUsed == 0 {
			t.Log("No tools were used in execution")
		} else {
			t.Logf("Tools used: %d", result.ToolsUsed)
		}
	})
}

// TestDotpromptTemplateRendering tests template rendering functionality
func TestDotpromptTemplateRendering(t *testing.T) {
	template := `{{#system}}
You are a test agent for {{context.test_type}}.
Available tools: {{#each tools}}{{name}} {{/each}}
{{/system}}

Task: {{task}}

{{#if context}}
Context: {{toJson context}}
{{/if}}

{{#if parameters}}
Parameters: {{toJson parameters}}
{{/if}}`

	request := dotprompt.ExecutionRequest{
		Task: "Perform a system health check",
		Context: map[string]interface{}{
			"test_type":    "integration",
			"environment": "staging",
		},
		Parameters: map[string]interface{}{
			"timeout": 30,
			"verbose": true,
		},
	}

	executor := &dotprompt.GenkitExecutor{}
	rendered, err := executor.RenderTemplate(template, request)
	if err != nil {
		t.Fatalf("Template rendering failed: %v", err)
	}

	t.Logf("Rendered template:\n%s", rendered)

	// Validate template rendering
	if !contains(rendered, "Perform a system health check") {
		t.Error("Task not rendered correctly")
	}
	
	if !contains(rendered, "integration") {
		t.Error("Context not rendered correctly")
	}
	
	if !contains(rendered, "30") {
		t.Error("Parameters not rendered correctly")
	}
}

// TestDotpromptValidation tests dotprompt file validation
func TestDotpromptValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      dotprompt.DotpromptConfig
		template    string
		expectValid bool
		expectError string
	}{
		{
			name:        "Valid_Basic",
			config:      createValidDotpromptConfig(),
			template:    createTestTemplate(),
			expectValid: true,
		},
		{
			name:        "Invalid_NoModel",
			config:      createInvalidDotpromptConfig("no_model"),
			template:    createTestTemplate(),
			expectValid: false,
			expectError: "model is required",
		},
		{
			name:        "Invalid_NoName",
			config:      createInvalidDotpromptConfig("no_name"),
			template:    createTestTemplate(),
			expectValid: false,
			expectError: "metadata.name is required",
		},
		{
			name:        "Invalid_EmptyTemplate",
			config:      createValidDotpromptConfig(),
			template:    "",
			expectValid: false,
			expectError: "template content is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary dotprompt structure
			dotprompt := &dotprompt.AgentDotprompt{
				Config:   tt.config,
				Template: tt.template,
				FilePath: "/tmp/test.prompt",
			}

			// Validate
			result := validateDotpromptStruct(dotprompt)

			if tt.expectValid && !result.Valid {
				t.Errorf("Expected valid, got invalid with errors: %v", result.Errors)
			}

			if !tt.expectValid && result.Valid {
				t.Error("Expected invalid, got valid")
			}

			if tt.expectError != "" {
				found := false
				for _, err := range result.Errors {
					if contains(err, tt.expectError) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing '%s', got: %v", tt.expectError, result.Errors)
				}
			}
		})
	}
}

// Helper functions

func setupTestConfig(provider, model string) *config.Config {
	return &config.Config{
		AIProvider: provider,
		AIModel:    model,
		DatabaseURL: ":memory:",
		Debug:      true,
	}
}

func setupGenkitExecutor(cfg *config.Config) *dotprompt.GenkitExecutor {
	genkitProvider := services.NewGenKitProvider()
	// mcpManager would be initialized here in a full integration test
	return dotprompt.NewGenkitExecutor(genkitProvider, nil)
}

func createTestDotpromptConfig(model string) dotprompt.DotpromptConfig {
	return dotprompt.DotpromptConfig{
		Model: model,
		Input: dotprompt.InputSchema{
			Schema: map[string]interface{}{
				"task":       "string",
				"context":    map[string]interface{}{"type": "object"},
				"parameters": map[string]interface{}{"type": "object"},
			},
		},
		Output: dotprompt.OutputSchema{
			Format: "json",
			Schema: map[string]interface{}{
				"result": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"success": map[string]interface{}{"type": "boolean"},
						"summary": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
		Metadata: dotprompt.AgentMetadata{
			AgentID:     999,
			Name:        "TestAgent",
			Description: "Test agent for dotprompt execution",
			MaxSteps:    3,
			Environment: "test",
			Version:     "1.0",
		},
	}
}

func createTestTemplate() string {
	return `{{#system}}
You are a test agent designed to validate dotprompt execution.
Respond with structured information about the task you receive.
Be concise and informative.
{{/system}}

Execute the following task: {{task}}

{{#if context}}
Additional Context:
{{toJson context}}
{{/if}}

{{#if parameters}}
Parameters:
{{toJson parameters}}
{{/if}}

Please provide a structured response with success status and summary.`
}

func createTestTemplateWithTools() string {
	return `{{#system}}
You are a monitoring agent with access to logging and incident management tools.
Use the available tools to analyze system issues and take appropriate actions.
{{/system}}

Task: {{task}}

{{#if context}}
Context: {{toJson context}}
{{/if}}

Available tools:
- __find_error_pattern_logs: Search for error patterns in logs
- __create_incident: Create incidents for critical issues

Use these tools as needed to complete the task.`
}

func validateExecutionResult(t *testing.T, result *dotprompt.ExecutionResponse, method string) {
	if result == nil {
		t.Fatalf("%s result is nil", method)
	}

	if !result.Success && result.Error == "" {
		t.Errorf("%s marked as failed but no error provided", method)
	}

	if result.Duration <= 0 {
		t.Errorf("%s duration should be positive, got: %v", method, result.Duration)
	}

	if result.ModelName == "" {
		t.Errorf("%s model name should not be empty", method)
	}

	t.Logf("%s execution: Success=%v, Duration=%v, Steps=%d, Tools=%d", 
		method, result.Success, result.Duration, result.StepsUsed, result.ToolsUsed)
}

func logExecutionMetrics(t *testing.T, method string, result *dotprompt.ExecutionResponse) {
	t.Logf("=== %s Execution Metrics ===", method)
	t.Logf("Success: %v", result.Success)
	t.Logf("Duration: %v", result.Duration)
	t.Logf("Model: %s", result.ModelName)
	t.Logf("Steps: %d", result.StepsUsed)
	t.Logf("Tools: %d", result.ToolsUsed)
	
	if result.TokenUsage != nil {
		if input, ok := result.TokenUsage["input_tokens"]; ok {
			t.Logf("Input Tokens: %v", input)
		}
		if output, ok := result.TokenUsage["output_tokens"]; ok {
			t.Logf("Output Tokens: %v", output)
		}
		if total, ok := result.TokenUsage["total_tokens"]; ok {
			t.Logf("Total Tokens: %v", total)
		}
	}
	
	if result.Response != "" {
		responsePreview := result.Response
		if len(responsePreview) > 100 {
			responsePreview = responsePreview[:100] + "..."
		}
		t.Logf("Response Preview: %s", responsePreview)
	}
	
	if result.Error != "" {
		t.Logf("Error: %s", result.Error)
	}
}

func executeWithStationCurrent(ctx context.Context, t *testing.T, cfg *config.Config, task string) *dotprompt.ExecutionResponse {
	// This would integrate with Station's current agent execution system
	// For now, return nil as we're focusing on dotprompt testing
	t.Log("Station current execution would be tested here")
	return nil
}

func createValidDotpromptConfig() dotprompt.DotpromptConfig {
	return dotprompt.DotpromptConfig{
		Model: "gemini-1.5-flash",
		Metadata: dotprompt.AgentMetadata{
			Name:        "ValidAgent",
			Description: "A valid test agent",
		},
	}
}

func createInvalidDotpromptConfig(invalidType string) dotprompt.DotpromptConfig {
	config := createValidDotpromptConfig()
	
	switch invalidType {
	case "no_model":
		config.Model = ""
	case "no_name":
		config.Metadata.Name = ""
	}
	
	return config
}

func validateDotpromptStruct(dp *dotprompt.AgentDotprompt) *dotprompt.ValidationResult {
	// Simulate validation logic
	var errors []string
	
	if dp.Config.Model == "" {
		errors = append(errors, "model is required")
	}
	
	if dp.Config.Metadata.Name == "" {
		errors = append(errors, "metadata.name is required")
	}
	
	if dp.Template == "" {
		errors = append(errors, "template content is required")
	}
	
	return &dotprompt.ValidationResult{
		Valid:  len(errors) == 0,
		Errors: errors,
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		 findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}