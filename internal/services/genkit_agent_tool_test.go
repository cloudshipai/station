package services

import (
	"context"
	"testing"

	"station/internal/db/repositories"
	"station/pkg/models"

	"github.com/firebase/genkit/go/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenKitAgentToolCreation tests that agent tools are created as proper GenKit tools
func TestGenKitAgentToolCreation(t *testing.T) {
	ctx := context.Background()

	// Create mock service that implements ListAgentsByEnvironment
	mockService := &MockAgentServiceWithList{
		MockAgentService: NewMockAgentService(),
		agents: []*models.Agent{
			{
				ID:            1,
				Name:          "Code Analyzer",
				Description:   "Analyzes code for issues",
				EnvironmentID: 1,
			},
			{
				ID:            2,
				Name:          "Test Generator",
				Description:   "Generates unit tests",
				EnvironmentID: 1,
			},
		},
	}

	for _, agent := range mockService.agents {
		mockService.MockAgentService.AddAgent(agent)
	}

	// Create MCP connection manager (without full repos for simplicity)
	mcpConnManager := &MCPConnectionManager{
		repos:          &repositories.Repositories{},
		toolCache:      make(map[int64]*EnvironmentToolCache),
		agentToolCache: make(map[string]*AgentToolCache),
		agentService:   mockService,
	}

	// Get agent tools using the new implementation
	tools := mcpConnManager.getAgentToolsForEnvironment(ctx, 1, mockService)

	// Should create 2 tools
	require.Len(t, tools, 2, "Should create 2 agent tools")

	// Check first tool
	tool1 := tools[0]
	if named, ok := tool1.(interface{ Name() string }); ok {
		assert.Equal(t, "__agent_code_analyzer", named.Name())
		t.Logf("Tool 1 name: %s", named.Name())
	} else {
		t.Fatal("Tool 1 doesn't implement Name() method")
	}

	// Check second tool
	tool2 := tools[1]
	if named, ok := tool2.(interface{ Name() string }); ok {
		assert.Equal(t, "__agent_test_generator", named.Name())
		t.Logf("Tool 2 name: %s", named.Name())
	} else {
		t.Fatal("Tool 2 doesn't implement Name() method")
	}

	// Test tool execution
	t.Run("Execute GenKit Tool", func(t *testing.T) {
		// Tools created with ai.NewToolWithInputSchema should have RunRaw method
		if executable, ok := tool1.(interface {
			RunRaw(context.Context, interface{}) (interface{}, error)
		}); ok {
			// Create AI tool context
			toolCtx := &ai.ToolContext{
				Context: ctx,
			}

			result, err := executable.RunRaw(toolCtx, map[string]interface{}{
				"task": "Analyze this code for security issues",
			})

			require.NoError(t, err)
			assert.NotNil(t, result)

			// Check result is a string
			resultStr, ok := result.(string)
			assert.True(t, ok, "Result should be a string")
			assert.Contains(t, resultStr, "Code Analyzer")

			t.Logf("Tool execution result: %s", resultStr)
		} else {
			t.Fatal("Tool doesn't implement RunRaw method")
		}
	})

	// Verify tools have proper definitions
	t.Run("Tool Definitions", func(t *testing.T) {
		for i, tool := range tools {
			// Check if tool has Definition method
			if definable, ok := tool.(interface {
				Definition() *ai.ToolDefinition
			}); ok {
				def := definable.Definition()
				require.NotNil(t, def, "Tool %d should have definition", i)

				// Check definition properties
				assert.NotEmpty(t, def.Name, "Tool %d should have name", i)
				assert.NotEmpty(t, def.Description, "Tool %d should have description", i)
				assert.NotNil(t, def.InputSchema, "Tool %d should have input schema", i)

				// Check input schema structure
				if def.InputSchema != nil {
					assert.Equal(t, "object", def.InputSchema["type"], "Schema type should be 'object'")

					if properties, ok := def.InputSchema["properties"].(map[string]interface{}); ok {
						assert.Contains(t, properties, "task", "Schema should have 'task' property")
					}
				}

				t.Logf("Tool %d definition: name=%s, desc=%s", i, def.Name, def.Description)
			} else {
				t.Fatalf("Tool %d doesn't implement Definition() method", i)
			}
		}
	})
}

// TestGenKitToolsVsOldAgentAsTool compares new GenKit tools with old AgentAsTool
func TestGenKitToolsVsOldAgentAsTool(t *testing.T) {
	t.Run("Compare implementations", func(t *testing.T) {
		// Document the differences
		t.Log("OLD IMPLEMENTATION (AgentAsTool):")
		t.Log("  - Custom struct implementing ai.Tool interface")
		t.Log("  - Manual implementation of Name(), Definition(), RunRaw(), etc.")
		t.Log("  - Not registered with GenKit, just added to tools array")
		t.Log("  - GenKit cannot discover these tools during generation")
		t.Log("")
		t.Log("NEW IMPLEMENTATION (ai.NewToolWithInputSchema):")
		t.Log("  - Uses GenKit's built-in tool creation")
		t.Log("  - Automatically implements ai.Tool interface correctly")
		t.Log("  - Properly registered in GenKit's internal structures")
		t.Log("  - GenKit CAN discover these tools during generation")
		t.Log("")
		t.Log("KEY DIFFERENCE:")
		t.Log("  The new tools are created with ai.NewToolWithInputSchema which")
		t.Log("  ensures they are properly integrated with GenKit's tool discovery")
		t.Log("  mechanism, making them available to LLMs during generation.")
	})
}

// MockAgentServiceWithList extends MockAgentService to implement ListAgentsByEnvironment
type MockAgentServiceWithList struct {
	*MockAgentService
	agents []*models.Agent
}

func (m *MockAgentServiceWithList) ListAgentsByEnvironment(ctx context.Context, environmentID int64) ([]*models.Agent, error) {
	var result []*models.Agent
	for _, agent := range m.agents {
		if agent.EnvironmentID == environmentID {
			result = append(result, agent)
		}
	}
	return result, nil
}
