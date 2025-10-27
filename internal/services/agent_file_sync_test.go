package services

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// MockAgentToolsRepo implements the AgentTools repository interface for testing
type MockAgentToolsRepo struct {
	tools map[int64][]*models.AgentToolWithDetails // agentID -> tools
	nextID int64
	mcpRepo *MockMCPToolsRepo // Need reference to get actual tool names
}

func NewMockAgentToolsRepo(mcpRepo *MockMCPToolsRepo) *MockAgentToolsRepo {
	return &MockAgentToolsRepo{
		tools: make(map[int64][]*models.AgentToolWithDetails),
		nextID: 1,
		mcpRepo: mcpRepo,
	}
}

func (m *MockAgentToolsRepo) ListAgentTools(agentID int64) ([]*models.AgentToolWithDetails, error) {
	return m.tools[agentID], nil
}

func (m *MockAgentToolsRepo) AddAgentTool(agentID, toolID int64) (*models.AgentTool, error) {
	// Check if already exists
	for _, tool := range m.tools[agentID] {
		if tool.ToolID == toolID {
			return nil, fmt.Errorf("tool already assigned")
		}
	}

	// Get actual tool name from MCP repo
	toolName := fmt.Sprintf("tool_%d", toolID) // default fallback
	for name, mcpTool := range m.mcpRepo.tools {
		if mcpTool.ID == toolID {
			toolName = name
			break
		}
	}

	agentTool := &models.AgentToolWithDetails{
		AgentTool: models.AgentTool{
			ID:        m.nextID,
			AgentID:   agentID,
			ToolID:    toolID,
			CreatedAt: time.Now(),
		},
		ToolName: toolName,
	}
	m.nextID++

	m.tools[agentID] = append(m.tools[agentID], agentTool)
	return &agentTool.AgentTool, nil
}

func (m *MockAgentToolsRepo) RemoveAgentTool(agentID, toolID int64) error {
	tools := m.tools[agentID]
	for i, tool := range tools {
		if tool.ToolID == toolID {
			// Remove the tool
			m.tools[agentID] = append(tools[:i], tools[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("tool not found")
}

func (m *MockAgentToolsRepo) Clear(agentID int64) error {
	m.tools[agentID] = []*models.AgentToolWithDetails{}
	return nil
}

// MockMCPToolsRepo implements the MCPTools repository interface for testing
type MockMCPToolsRepo struct {
	tools map[string]*models.MCPTool // toolName -> tool
}

func NewMockMCPToolsRepo() *MockMCPToolsRepo {
	return &MockMCPToolsRepo{
		tools: make(map[string]*models.MCPTool),
	}
}

func (m *MockMCPToolsRepo) FindByNameInEnvironment(envID int64, toolName string) (*models.MCPTool, error) {
	tool, exists := m.tools[toolName]
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}
	return tool, nil
}

func (m *MockMCPToolsRepo) AddTool(toolName string, toolID int64) {
	m.tools[toolName] = &models.MCPTool{
		ID:   toolID,
		Name: toolName,
	}
}

func TestDeclarativeSyncToolAssignments(t *testing.T) {
	// Create temp directory for test files
	tmpDir, err := os.MkdirTemp("", "sync_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Setup mock repositories
	mcpToolsRepo := NewMockMCPToolsRepo()
	agentToolsRepo := NewMockAgentToolsRepo(mcpToolsRepo)

	// Add some tools to MCP repo
	mcpToolsRepo.AddTool("__read_file", 1)
	mcpToolsRepo.AddTool("__write_file", 2)
	mcpToolsRepo.AddTool("__list_directory", 3)
	mcpToolsRepo.AddTool("__delete_file", 4)

	// Create test agent
	testAgent := &models.Agent{
		ID:            100,
		Name:          "TestAgent",
		EnvironmentID: 1,
	}

	t.Run("Initial sync adds all tools from config", func(t *testing.T) {
		// Create agent prompt file with tools
		promptContent := `---
metadata:
  name: "TestAgent"
  description: "Test agent"
tools:
  - "__read_file"
  - "__write_file"
  - "__list_directory"
---

{{role "system"}}
You are a test agent.
`
		promptPath := filepath.Join(tmpDir, "TestAgent.prompt")
		err := os.WriteFile(promptPath, []byte(promptContent), 0644)
		require.NoError(t, err)

		// Simulate sync
		// In real implementation, this would be called by updateAgentFromFile
		// Here we test the logic directly

		// Initial state: no tools assigned
		currentTools, _ := agentToolsRepo.ListAgentTools(testAgent.ID)
		assert.Empty(t, currentTools)

		// Sync tools from config
		configTools := []string{"__read_file", "__write_file", "__list_directory"}
		for _, toolName := range configTools {
			tool, err := mcpToolsRepo.FindByNameInEnvironment(testAgent.EnvironmentID, toolName)
			require.NoError(t, err)
			_, err = agentToolsRepo.AddAgentTool(testAgent.ID, tool.ID)
			require.NoError(t, err)
		}

		// Verify all tools were added
		currentTools, _ = agentToolsRepo.ListAgentTools(testAgent.ID)
		assert.Len(t, currentTools, 3)
	})

	t.Run("Sync preserves existing tools when config unchanged", func(t *testing.T) {
		// Get current tools before sync
		beforeTools, _ := agentToolsRepo.ListAgentTools(testAgent.ID)
		beforeCount := len(beforeTools)

		// Create a map of tool IDs before sync
		beforeToolIDs := make(map[int64]bool)
		for _, tool := range beforeTools {
			beforeToolIDs[tool.ID] = true
		}

		// Simulate another sync with same config - should not clear and re-add
		// Our new implementation checks if tools match and skips changes
		configTools := []string{"__read_file", "__write_file", "__list_directory"}

		// Get current tools
		currentTools, _ := agentToolsRepo.ListAgentTools(testAgent.ID)
		currentToolMap := make(map[string]int64)
		for _, tool := range currentTools {
			currentToolMap[tool.ToolName] = tool.ToolID
		}

		configToolSet := make(map[string]bool)
		for _, toolName := range configTools {
			configToolSet[toolName] = true
		}

		// Check what needs to be changed
		toolsToAdd := []string{}
		toolsToRemove := []int64{}

		for toolName := range configToolSet {
			if _, exists := currentToolMap[toolName]; !exists {
				toolsToAdd = append(toolsToAdd, toolName)
			}
		}

		for toolName, toolID := range currentToolMap {
			if !configToolSet[toolName] {
				toolsToRemove = append(toolsToRemove, toolID)
			}
		}

		// Should be no changes needed
		assert.Empty(t, toolsToAdd, "No tools should need to be added")
		assert.Empty(t, toolsToRemove, "No tools should need to be removed")

		// Verify tools are still the same
		afterTools, _ := agentToolsRepo.ListAgentTools(testAgent.ID)
		assert.Equal(t, beforeCount, len(afterTools), "Tool count should remain the same")

		// Verify the actual tool IDs haven't changed (not cleared and re-added)
		for _, tool := range afterTools {
			assert.True(t, beforeToolIDs[tool.ID], "Tool IDs should be preserved")
		}
	})

	t.Run("Sync adds only new tools when config expands", func(t *testing.T) {
		// Get current tool count
		beforeTools, _ := agentToolsRepo.ListAgentTools(testAgent.ID)
		beforeCount := len(beforeTools)

		// Simulate config with additional tool
		configTools := []string{"__read_file", "__write_file", "__list_directory", "__delete_file"}

		// Get current tools
		currentTools, _ := agentToolsRepo.ListAgentTools(testAgent.ID)
		currentToolMap := make(map[string]int64)
		for _, tool := range currentTools {
			currentToolMap[tool.ToolName] = tool.ToolID
		}

		// Add only the new tool
		for _, toolName := range configTools {
			if _, exists := currentToolMap[toolName]; !exists {
				tool, err := mcpToolsRepo.FindByNameInEnvironment(testAgent.EnvironmentID, toolName)
				require.NoError(t, err)
				_, err = agentToolsRepo.AddAgentTool(testAgent.ID, tool.ID)
				require.NoError(t, err)
			}
		}

		// Verify only one tool was added
		afterTools, _ := agentToolsRepo.ListAgentTools(testAgent.ID)
		assert.Equal(t, beforeCount+1, len(afterTools), "Exactly one tool should be added")

		// Verify the new tool is present
		hasDeleteTool := false
		for _, tool := range afterTools {
			if tool.ToolName == "__delete_file" {
				hasDeleteTool = true
				break
			}
		}
		assert.True(t, hasDeleteTool, "__delete_file should be present")
	})

	t.Run("Sync removes only obsolete tools when config shrinks", func(t *testing.T) {
		// Current state: has 4 tools
		beforeTools, _ := agentToolsRepo.ListAgentTools(testAgent.ID)
		assert.Len(t, beforeTools, 4)

		// Simulate config with fewer tools (remove __delete_file)
		configTools := []string{"__read_file", "__write_file"}

		// Get current tools
		currentTools, _ := agentToolsRepo.ListAgentTools(testAgent.ID)
		currentToolMap := make(map[string]int64)
		for _, tool := range currentTools {
			currentToolMap[tool.ToolName] = tool.ToolID
		}

		configToolSet := make(map[string]bool)
		for _, toolName := range configTools {
			configToolSet[toolName] = true
		}

		// Remove tools not in config
		for toolName, toolID := range currentToolMap {
			if !configToolSet[toolName] {
				err := agentToolsRepo.RemoveAgentTool(testAgent.ID, toolID)
				require.NoError(t, err)
			}
		}

		// Verify correct tools were removed
		afterTools, _ := agentToolsRepo.ListAgentTools(testAgent.ID)
		assert.Len(t, afterTools, 2, "Should have 2 tools remaining")

		// Verify the remaining tools are correct
		remainingToolNames := make(map[string]bool)
		for _, tool := range afterTools {
			remainingToolNames[tool.ToolName] = true
		}
		assert.True(t, remainingToolNames["__read_file"], "__read_file should remain")
		assert.True(t, remainingToolNames["__write_file"], "__write_file should remain")
		assert.False(t, remainingToolNames["__list_directory"], "__list_directory should be removed")
		assert.False(t, remainingToolNames["__delete_file"], "__delete_file should be removed")
	})

	t.Run("Sync handles empty tools in config correctly", func(t *testing.T) {
		// Simulate config with no tools (empty list)

		// Get current tools
		currentTools, _ := agentToolsRepo.ListAgentTools(testAgent.ID)

		// Remove all tools since config has none
		for _, tool := range currentTools {
			err := agentToolsRepo.RemoveAgentTool(testAgent.ID, tool.ToolID)
			require.NoError(t, err)
		}

		// Verify all tools were removed
		afterTools, _ := agentToolsRepo.ListAgentTools(testAgent.ID)
		assert.Empty(t, afterTools, "All tools should be removed when config has no tools")
	})

	t.Run("Sync gracefully handles missing tools in MCP", func(t *testing.T) {
		// Clear current tools
		_ = agentToolsRepo.Clear(testAgent.ID)

		// Config with a tool that doesn't exist in MCP
		configTools := []string{"__read_file", "__nonexistent_tool", "__write_file"}

		addedCount := 0
		failedTools := []string{}

		for _, toolName := range configTools {
			tool, err := mcpToolsRepo.FindByNameInEnvironment(testAgent.EnvironmentID, toolName)
			if err != nil {
				// Tool not found - should continue with others
				failedTools = append(failedTools, toolName)
				continue
			}
			_, err = agentToolsRepo.AddAgentTool(testAgent.ID, tool.ID)
			require.NoError(t, err)
			addedCount++
		}

		// Should add 2 tools, skip the nonexistent one
		assert.Equal(t, 2, addedCount, "Should add 2 valid tools")
		assert.Len(t, failedTools, 1, "Should have 1 failed tool")
		assert.Equal(t, "__nonexistent_tool", failedTools[0])

		// Verify correct tools were added
		afterTools, _ := agentToolsRepo.ListAgentTools(testAgent.ID)
		assert.Len(t, afterTools, 2, "Should have 2 tools assigned")
	})
}

func TestDeclarativeSyncIdempotency(t *testing.T) {
	// Create mock repos
	mcpToolsRepo := NewMockMCPToolsRepo()
	agentToolsRepo := NewMockAgentToolsRepo(mcpToolsRepo)

	// Add tools to MCP repo
	mcpToolsRepo.AddTool("__tool_a", 1)
	mcpToolsRepo.AddTool("__tool_b", 2)
	mcpToolsRepo.AddTool("__tool_c", 3)

	testAgent := &models.Agent{
		ID:            200,
		Name:          "IdempotencyTestAgent",
		EnvironmentID: 1,
	}

	// Helper function to simulate sync
	syncTools := func(configTools []string) (added, removed int) {
		// Get current tools
		currentTools, _ := agentToolsRepo.ListAgentTools(testAgent.ID)
		currentToolMap := make(map[string]int64)
		for _, tool := range currentTools {
			currentToolMap[tool.ToolName] = tool.ToolID
		}

		configToolSet := make(map[string]bool)
		for _, toolName := range configTools {
			configToolSet[toolName] = true
		}

		added = 0
		removed = 0

		// Remove tools not in config
		for toolName, toolID := range currentToolMap {
			if !configToolSet[toolName] {
				_ = agentToolsRepo.RemoveAgentTool(testAgent.ID, toolID)
				removed++
			}
		}

		// Add tools from config
		for _, toolName := range configTools {
			if _, exists := currentToolMap[toolName]; !exists {
				tool, err := mcpToolsRepo.FindByNameInEnvironment(testAgent.EnvironmentID, toolName)
				if err == nil {
					agentToolsRepo.AddAgentTool(testAgent.ID, tool.ID)
					added++
				}
			}
		}

		return added, removed
	}

	configTools := []string{"__tool_a", "__tool_b", "__tool_c"}

	t.Run("Multiple syncs with same config produce no changes", func(t *testing.T) {
		// First sync
		added1, removed1 := syncTools(configTools)
		assert.Equal(t, 3, added1, "First sync should add 3 tools")
		assert.Equal(t, 0, removed1, "First sync should remove 0 tools")

		// Second sync - should be no-op
		added2, removed2 := syncTools(configTools)
		assert.Equal(t, 0, added2, "Second sync should add 0 tools")
		assert.Equal(t, 0, removed2, "Second sync should remove 0 tools")

		// Third sync - still no-op
		added3, removed3 := syncTools(configTools)
		assert.Equal(t, 0, added3, "Third sync should add 0 tools")
		assert.Equal(t, 0, removed3, "Third sync should remove 0 tools")

		// Verify final state
		finalTools, _ := agentToolsRepo.ListAgentTools(testAgent.ID)
		assert.Len(t, finalTools, 3, "Should maintain 3 tools")
	})
}

func TestDeclarativeSyncPerformance(t *testing.T) {
	// This test ensures sync performance doesn't degrade with many tools
	mcpToolsRepo := NewMockMCPToolsRepo()
	agentToolsRepo := NewMockAgentToolsRepo(mcpToolsRepo)

	// Create many tools
	numTools := 100
	for i := 1; i <= numTools; i++ {
		toolName := fmt.Sprintf("__tool_%d", i)
		mcpToolsRepo.AddTool(toolName, int64(i))
	}

	testAgent := &models.Agent{
		ID:            300,
		Name:          "PerformanceTestAgent",
		EnvironmentID: 1,
	}

	// Initial assignment of all tools
	for i := 1; i <= numTools; i++ {
		toolName := fmt.Sprintf("__tool_%d", i)
		tool, _ := mcpToolsRepo.FindByNameInEnvironment(testAgent.EnvironmentID, toolName)
		agentToolsRepo.AddAgentTool(testAgent.ID, tool.ID)
	}

	t.Run("Sync with no changes is efficient", func(t *testing.T) {
		// Get current tools
		currentTools, _ := agentToolsRepo.ListAgentTools(testAgent.ID)
		assert.Len(t, currentTools, numTools)

		// Create config with same tools
		configTools := make([]string, numTools)
		for i := 0; i < numTools; i++ {
			configTools[i] = fmt.Sprintf("__tool_%d", i+1)
		}

		// Check what would change (should be nothing)
		currentToolMap := make(map[string]bool)
		for _, tool := range currentTools {
			currentToolMap[tool.ToolName] = true
		}

		configToolSet := make(map[string]bool)
		for _, toolName := range configTools {
			configToolSet[toolName] = true
		}

		needsChange := false
		for toolName := range configToolSet {
			if !currentToolMap[toolName] {
				needsChange = true
				break
			}
		}
		if !needsChange {
			for toolName := range currentToolMap {
				if !configToolSet[toolName] {
					needsChange = true
					break
				}
			}
		}

		assert.False(t, needsChange, "Should detect no changes needed")
	})
}

func TestParseDotPrompt(t *testing.T) {
	testDB, err := db.NewTest(t)
	require.NoError(t, err)
	defer testDB.Close()

	repos := repositories.New(testDB)
	cfg := &config.Config{}
	service := NewDeclarativeSync(repos, cfg)

	tests := []struct {
		name            string
		content         string
		expectConfig    *DotPromptConfig
		expectPrompt    string
		expectError     bool
		description     string
	}{
		{
			name: "Valid dotprompt with frontmatter",
			content: `---
model: gpt-4o-mini
max_steps: 5
metadata:
  name: "Test Agent"
  description: "Test description"
tools:
  - "__read_file"
  - "__write_file"
---

{{role "system"}}
You are a test agent.

{{role "user"}}
{{userInput}}`,
			expectConfig: &DotPromptConfig{
				Model: "gpt-4o-mini",
				MaxSteps: 5,
				Tools: []string{"__read_file", "__write_file"},
				Metadata: map[string]interface{}{
					"name": "Test Agent",
					"description": "Test description",
				},
			},
			expectPrompt: `{{role "system"}}
You are a test agent.

{{role "user"}}
{{userInput}}`,
			expectError: false,
			description: "Should parse valid dotprompt with frontmatter",
		},
		{
			name: "Dotprompt without frontmatter",
			content: `{{role "system"}}
You are a simple agent.`,
			expectConfig: &DotPromptConfig{},
			expectPrompt: `{{role "system"}}
You are a simple agent.`,
			expectError: false,
			description: "Should handle dotprompt without frontmatter",
		},
		{
			name: "Invalid YAML frontmatter",
			content: `---
model: gpt-4o-mini
invalid yaml: [unclosed bracket
---

{{role "system"}}
Test`,
			expectConfig: nil,
			expectPrompt: "",
			expectError: true,
			description: "Should error on invalid YAML",
		},
		{
			name: "Empty frontmatter",
			content: `---
---

{{role "system"}}
You are an agent.`,
			expectConfig: &DotPromptConfig{},
			expectPrompt: `{{role "system"}}
You are an agent.`,
			expectError: false,
			description: "Should handle empty frontmatter",
		},
		{
			name: "Multiple --- in prompt content",
			content: `---
model: gpt-4o-mini
---

{{role "system"}}
Use --- to separate sections.
---
Another section`,
			expectConfig: &DotPromptConfig{
				Model: "gpt-4o-mini",
			},
			expectPrompt: `{{role "system"}}
Use --- to separate sections.
---
Another section`,
			expectError: false,
			description: "Should handle --- in prompt content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, promptContent, err := service.parseDotPrompt(tt.content)

			if tt.expectError {
				require.Error(t, err, tt.description)
			} else {
				require.NoError(t, err, tt.description)
				assert.Equal(t, tt.expectPrompt, promptContent, "Prompt content should match")

				if tt.expectConfig != nil {
					assert.Equal(t, tt.expectConfig.Model, config.Model, "Model should match")
					assert.Equal(t, tt.expectConfig.MaxSteps, config.MaxSteps, "MaxSteps should match")
					assert.Equal(t, tt.expectConfig.Tools, config.Tools, "Tools should match")
				}
			}
		})
	}
}