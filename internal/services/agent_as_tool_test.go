package services

import (
	"context"
	"testing"

	"github.com/firebase/genkit/go/ai"
)

// TestAgentAsTool tests the AgentAsTool implementation
func TestAgentAsTool(t *testing.T) {
	t.Run("Basic tool properties", func(t *testing.T) {
		tool := &AgentAsTool{
			agentID:     1,
			agentName:   "Test Agent",
			description: "A test agent tool",
		}

		// Test Name()
		expectedName := "__agent_test_agent"
		if name := tool.Name(); name != expectedName {
			t.Errorf("Expected name %s, got %s", expectedName, name)
		}

		// Test Description()
		// Description is a field, not a method
		if tool.description != "A test agent tool" {
			t.Errorf("Expected description 'A test agent tool', got '%s'", tool.description)
		}
	})

	t.Run("Tool definition", func(t *testing.T) {
		tool := &AgentAsTool{
			agentID:     1,
			agentName:   "Test Agent",
			description: "A test agent tool",
		}

		def := tool.Definition()
		if def == nil {
			t.Fatal("Definition should not be nil")
		}

		if def.Name != "__agent_test_agent" {
			t.Errorf("Expected definition name '__agent_test_agent', got '%s'", def.Name)
		}

		if def.Description != "A test agent tool" {
			t.Errorf("Expected definition description 'A test agent tool', got '%s'", def.Description)
		}

		// Check input schema
		if def.InputSchema == nil {
			t.Error("InputSchema should not be nil")
		} else {
			inputSchema := def.InputSchema
			if inputSchema["type"] != "object" {
				t.Error("InputSchema type should be 'object'")
			}
			if props, ok := inputSchema["properties"].(map[string]interface{}); ok {
				if _, hasTask := props["task"]; !hasTask {
					t.Error("InputSchema should have 'task' property")
				}
			} else {
				t.Error("InputSchema properties should be a map[string]interface{}")
			}
		}

		// Check output schema
		if def.OutputSchema == nil {
			t.Error("OutputSchema should not be nil")
		} else {
			outputSchema := def.OutputSchema
			if outputSchema["type"] != "string" {
				t.Error("OutputSchema type should be 'string'")
			}
		}
	})

	t.Run("RunRaw with valid input", func(t *testing.T) {
		tool := &AgentAsTool{
			agentID:     1,
			agentName:   "Test Agent",
			description: "A test agent tool",
		}

		ctx := context.Background()
		input := map[string]interface{}{
			"task": "test task",
		}

		result, err := tool.RunRaw(ctx, input)
		if err != nil {
			t.Fatalf("RunRaw failed: %v", err)
		}

		if result == nil {
			t.Fatal("Result should not be nil")
		}

		resultStr, ok := result.(string)
		if !ok {
			t.Fatal("Result should be a string")
		}

		expected := "Agent Test Agent would execute task: test task"
		if resultStr != expected {
			t.Errorf("Expected result '%s', got '%s'", expected, resultStr)
		}
	})

	t.Run("RunRaw with invalid input", func(t *testing.T) {
		tool := &AgentAsTool{
			agentID:     1,
			agentName:   "Test Agent",
			description: "A test agent tool",
		}

		ctx := context.Background()

		// Test with nil input
		_, err := tool.RunRaw(ctx, nil)
		if err == nil {
			t.Error("RunRaw should fail with nil input")
		}

		// Test with non-map input
		_, err = tool.RunRaw(ctx, "invalid input")
		if err == nil {
			t.Error("RunRaw should fail with non-map input")
		}

		// Test with missing task
		_, err = tool.RunRaw(ctx, map[string]interface{}{})
		if err == nil {
			t.Error("RunRaw should fail with missing task")
		}

		// Test with non-string task
		_, err = tool.RunRaw(ctx, map[string]interface{}{
			"task": 123,
		})
		if err == nil {
			t.Error("RunRaw should fail with non-string task")
		}
	})

	t.Run("Respond method", func(t *testing.T) {
		tool := &AgentAsTool{
			agentID:     1,
			agentName:   "Test Agent",
			description: "A test agent tool",
		}

		// Create a mock tool request part
		toolReq := &ai.Part{
			Kind: ai.PartToolRequest,
			ToolRequest: &ai.ToolRequest{
				Name:  "__agent_test_agent",
				Input: map[string]interface{}{"task": "test"},
			},
		}

		outputData := "test output"
		response := tool.Respond(toolReq, outputData, nil)

		if response == nil {
			t.Fatal("Respond should return a Part")
		}

		if !response.IsToolResponse() {
			t.Error("Response should be a tool response")
		}
	})

	t.Run("Restart method", func(t *testing.T) {
		tool := &AgentAsTool{
			agentID:     1,
			agentName:   "Test Agent",
			description: "A test agent tool",
		}

		// Create a mock tool request part
		toolReq := &ai.Part{
			Kind: ai.PartToolRequest,
			ToolRequest: &ai.ToolRequest{
				Name:  "__agent_test_agent",
				Input: map[string]interface{}{"task": "test"},
			},
		}

		restarted := tool.Restart(toolReq, nil)

		if restarted == nil {
			t.Fatal("Restart should return a Part")
		}

		if !restarted.IsToolRequest() {
			t.Error("Restarted part should be a tool request")
		}

		if restarted.ToolRequest == nil {
			t.Fatal("Restarted part should have a ToolRequest")
		}

		if restarted.ToolRequest.Name != "__agent_test_agent" {
			t.Errorf("Expected restarted tool name '__agent_test_agent', got '%s'", restarted.ToolRequest.Name)
		}
	})

	t.Run("Name formatting", func(t *testing.T) {
		tests := []struct {
			agentName string
			expected  string
		}{
			{"Simple Agent", "__agent_simple_agent"},
			{"Agent With Spaces", "__agent_agent_with_spaces"},
			{"Agent-With-Dashes", "__agent_agent-with-dashes"},
			{"Agent.With.Dots", "__agent_agent.with.dots"},
			{"UPPERCASE AGENT", "__agent_uppercase_agent"},
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
}
