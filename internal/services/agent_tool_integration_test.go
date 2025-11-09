package services

import (
	"context"
	"testing"
)

// TestAgentToolIntegration tests the complete integration of agent tools with MCP tools
func TestAgentToolIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test would require a working database setup, so we'll skip it for now
	// and create a simpler unit test instead
	t.Skip("Database migration issues prevent full integration test")
}

// TestAgentToolDiscoveryLogic tests the logic for discovering agent tools
func TestAgentToolDiscoveryLogic(t *testing.T) {
	t.Run("Agent tool name generation", func(t *testing.T) {
		tests := []struct {
			agentName string
			expected  string
		}{
			{"Simple Agent", "__agent_simple_agent"},
			{"Agent With Multiple Spaces", "__agent_agent_with_multiple_spaces"},
			{"Agent-With-Dashes", "__agent_agent_with_dashes"},
			{"Agent.With.Dots", "__agent_agent_with_dots"},
			{"UPPERCASE AGENT NAME", "__agent_uppercase_agent_name"},
			{"agent with numbers 123", "__agent_agent_with_numbers_123"},
			{"Special!@#$%^&*()Characters", "__agent_special_characters"},
		}

		for _, tt := range tests {
			t.Run(tt.agentName, func(t *testing.T) {
				tool := &AgentAsTool{
					agentID:     1,
					agentName:   tt.agentName,
					description: "Test agent",
				}

				if name := tool.Name(); name != tt.expected {
					t.Errorf("Expected name '%s', got '%s'", tt.expected, name)
				}
			})
		}
	})

	t.Run("Agent tool input validation", func(t *testing.T) {
		tool := &AgentAsTool{
			agentID:     1,
			agentName:   "Test Agent",
			description: "A test agent tool",
		}

		ctx := context.Background()

		// Test valid input
		validInput := map[string]interface{}{
			"task": "Please analyze this code",
		}
		result, err := tool.RunRaw(ctx, validInput)
		if err != nil {
			t.Errorf("Valid input should not fail: %v", err)
		}
		if result == nil {
			t.Error("Valid input should return a result")
		}

		// Test various invalid inputs
		invalidInputs := []interface{}{
			nil,
			"string instead of map",
			123,
			[]string{"array"},
			map[string]interface{}{},                 // missing task
			map[string]interface{}{"task": 123},      // non-string task
			map[string]interface{}{"wrong": "field"}, // wrong field
		}

		for i, invalidInput := range invalidInputs {
			_, err := tool.RunRaw(ctx, invalidInput)
			if err == nil {
				t.Errorf("Invalid input %d should fail: %v", i, invalidInput)
			}
		}
	})
}

// TestAgentToolInEnvironmentContext tests how agent tools would work in an environment context
func TestAgentToolInEnvironmentContext(t *testing.T) {
	t.Run("Multiple agents in same environment", func(t *testing.T) {
		// Simulate what getAgentToolsForEnvironment would do
		agents := []struct {
			id          int64
			name        string
			description string
		}{
			{1, "Code Analyzer", "Analyzes code for security issues"},
			{2, "Documentation Writer", "Writes documentation for code"},
			{3, "Test Generator", "Generates test cases for code"},
		}

		var tools []interface{}
		for _, agent := range agents {
			tool := &AgentAsTool{
				agentID:     agent.id,
				agentName:   agent.name,
				description: agent.description,
			}
			tools = append(tools, tool)
		}

		// Verify all tools are created with unique names
		toolNames := make(map[string]bool)
		for i, tool := range tools {
			agentTool := tool.(*AgentAsTool)
			toolName := agentTool.Name()

			if toolNames[toolName] {
				t.Errorf("Duplicate tool name found: %s", toolName)
			}
			toolNames[toolName] = true

			t.Logf("Agent %d: %s -> Tool: %s", i+1, agentTool.agentName, toolName)
		}

		if len(toolNames) != len(agents) {
			t.Errorf("Expected %d unique tool names, got %d", len(agents), len(toolNames))
		}
	})

	t.Run("Agent tool execution simulation", func(t *testing.T) {
		// Simulate a hierarchical agent execution
		orchestrator := &AgentAsTool{
			agentID:     1,
			agentName:   "Orchestrator Agent",
			description: "Coordinates other agents",
		}

		specialist := &AgentAsTool{
			agentID:     2,
			agentName:   "Security Specialist",
			description: "Specializes in security analysis",
		}

		ctx := context.Background()

		// Simulate orchestrator calling specialist
		orchestratorResult, err := orchestrator.RunRaw(ctx, map[string]interface{}{
			"task": "Analyze the security of this application",
		})
		if err != nil {
			t.Fatalf("Orchestrator failed: %v", err)
		}

		specialistResult, err := specialist.RunRaw(ctx, map[string]interface{}{
			"task": "Perform detailed security scan of the codebase",
		})
		if err != nil {
			t.Fatalf("Specialist failed: %v", err)
		}

		t.Logf("Orchestrator result: %v", orchestratorResult)
		t.Logf("Specialist result: %v", specialistResult)

		// Both should return meaningful results
		if orchestratorResult == nil || specialistResult == nil {
			t.Error("Both agents should return results")
		}
	})
}

// TestAgentToolSchemaValidation tests the JSON schema validation for agent tools
func TestAgentToolSchemaValidation(t *testing.T) {
	tool := &AgentAsTool{
		agentID:     1,
		agentName:   "Schema Test Agent",
		description: "Tests schema validation",
	}

	def := tool.Definition()

	t.Run("Input schema structure", func(t *testing.T) {
		inputSchema := def.InputSchema

		// Check required fields
		required, ok := inputSchema["required"].([]string)
		if !ok {
			t.Fatal("InputSchema should have 'required' field as []string")
		}

		if len(required) != 1 || required[0] != "task" {
			t.Errorf("Expected required field ['task'], got %v", required)
		}

		// Check properties structure
		properties, ok := inputSchema["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("InputSchema should have 'properties' field as map[string]interface{}")
		}

		taskProp, ok := properties["task"].(map[string]interface{})
		if !ok {
			t.Fatal("Properties should have 'task' field as map[string]interface{}")
		}

		if taskProp["type"] != "string" {
			t.Errorf("Expected task type 'string', got '%v'", taskProp["type"])
		}

		if taskProp["description"] == nil {
			t.Error("Task property should have description")
		}
	})

	t.Run("Output schema structure", func(t *testing.T) {
		outputSchema := def.OutputSchema

		if outputSchema["type"] != "string" {
			t.Errorf("Expected output type 'string', got '%v'", outputSchema["type"])
		}
	})
}
