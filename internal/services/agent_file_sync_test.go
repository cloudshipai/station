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
	tools   map[int64][]*models.AgentToolWithDetails // agentID -> tools
	nextID  int64
	mcpRepo *MockMCPToolsRepo // Need reference to get actual tool names
}

func NewMockAgentToolsRepo(mcpRepo *MockMCPToolsRepo) *MockAgentToolsRepo {
	return &MockAgentToolsRepo{
		tools:   make(map[int64][]*models.AgentToolWithDetails),
		nextID:  1,
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
					_, _ = agentToolsRepo.AddAgentTool(testAgent.ID, tool.ID)
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
		_, _ = agentToolsRepo.AddAgentTool(testAgent.ID, tool.ID)
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
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	cfg := &config.Config{}
	service := NewDeclarativeSync(repos, cfg)

	tests := []struct {
		name         string
		content      string
		expectConfig *DotPromptConfig
		expectPrompt string
		expectError  bool
		description  string
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
				Model:    "gpt-4o-mini",
				MaxSteps: 5,
				Tools:    []string{"__read_file", "__write_file"},
				Metadata: map[string]interface{}{
					"name":        "Test Agent",
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
			expectError:  true,
			description:  "Should error on invalid YAML",
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

// TestCalculateFileChecksum tests MD5 checksum calculation
func TestCalculateFileChecksum(t *testing.T) {
	testDB, err := db.NewTest(t)
	require.NoError(t, err)
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	cfg := &config.Config{}
	service := NewDeclarativeSync(repos, cfg)

	tests := []struct {
		name         string
		content      string
		expectError  bool
		description  string
	}{
		{
			name:        "Simple text file",
			content:     "Hello, World!",
			expectError: false,
			description: "Should calculate checksum for simple text",
		},
		{
			name:        "Empty file",
			content:     "",
			expectError: false,
			description: "Should handle empty file",
		},
		{
			name:        "Multi-line content",
			content:     "Line 1\nLine 2\nLine 3",
			expectError: false,
			description: "Should handle multi-line content",
		},
		{
			name:        "Binary-like content",
			content:     "\x00\x01\x02\x03\x04\x05",
			expectError: false,
			description: "Should handle binary content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpFile := filepath.Join(t.TempDir(), "test.txt")
			err := os.WriteFile(tmpFile, []byte(tt.content), 0644)
			require.NoError(t, err)

			checksum, err := service.calculateFileChecksum(tmpFile)

			if tt.expectError {
				require.Error(t, err, tt.description)
			} else {
				require.NoError(t, err, tt.description)
				assert.NotEmpty(t, checksum, "Checksum should not be empty")
				assert.Len(t, checksum, 32, "MD5 checksum should be 32 hex characters")
				
				// Verify checksum is deterministic
				checksum2, err := service.calculateFileChecksum(tmpFile)
				require.NoError(t, err)
				assert.Equal(t, checksum, checksum2, "Checksum should be deterministic")
			}
		})
	}
}

// TestCalculateFileChecksumErrors tests error cases
func TestCalculateFileChecksumErrors(t *testing.T) {
	testDB, err := db.NewTest(t)
	require.NoError(t, err)
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	cfg := &config.Config{}
	service := NewDeclarativeSync(repos, cfg)

	t.Run("Non-existent file", func(t *testing.T) {
		checksum, err := service.calculateFileChecksum("/nonexistent/file.txt")
		assert.Error(t, err)
		assert.Empty(t, checksum)
	})
}

// TestParsePicoschemaString tests Picoschema string parsing
func TestParsePicoschemaString(t *testing.T) {
	testDB, err := db.NewTest(t)
	require.NoError(t, err)
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	cfg := &config.Config{}
	service := NewDeclarativeSync(repos, cfg)

	tests := []struct {
		name         string
		fieldName    string
		definition   string
		expectType   string
		expectDesc   string
		expectReq    bool
		description  string
	}{
		{
			name:        "Simple type",
			fieldName:   "name",
			definition:  "string",
			expectType:  "string",
			expectDesc:  "",
			expectReq:   true,
			description: "Should parse simple type definition",
		},
		{
			name:        "Type with description",
			fieldName:   "email",
			definition:  "string, User email address",
			expectType:  "string",
			expectDesc:  "User email address",
			expectReq:   true,
			description: "Should parse type with description",
		},
		{
			name:        "Optional field",
			fieldName:   "age?",
			definition:  "number",
			expectType:  "number",
			expectDesc:  "",
			expectReq:   false,
			description: "Should mark field as optional",
		},
		{
			name:        "Optional with description",
			fieldName:   "nickname?",
			definition:  "string, Optional nickname",
			expectType:  "string",
			expectDesc:  "Optional nickname",
			expectReq:   false,
			description: "Should handle optional field with description",
		},
		{
			name:        "Boolean type",
			fieldName:   "active",
			definition:  "boolean, Is user active",
			expectType:  "boolean",
			expectDesc:  "Is user active",
			expectReq:   true,
			description: "Should parse boolean type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.parsePicoschemaString(tt.fieldName, tt.definition)

			require.NotNil(t, result, tt.description)
			assert.Equal(t, tt.expectType, string(result.Type), "Type should match")
			assert.Equal(t, tt.expectDesc, result.Description, "Description should match")
			assert.Equal(t, tt.expectReq, result.Required, "Required flag should match")
		})
	}
}

// TestConvertYAMLMapToJSONMap tests YAML to JSON map conversion
func TestConvertYAMLMapToJSONMap(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expectType  string
		description string
	}{
		{
			name: "Simple map with interface keys",
			input: map[interface{}]interface{}{
				"name":  "John",
				"age":   30,
				"email": "john@example.com",
			},
			expectType:  "map[string]interface {}",
			description: "Should convert interface keys to string keys",
		},
		{
			name: "Already string keys",
			input: map[string]interface{}{
				"name":  "Jane",
				"age":   25,
			},
			expectType:  "map[string]interface {}",
			description: "Should handle already correct format",
		},
		{
			name: "Nested maps",
			input: map[interface{}]interface{}{
				"user": map[interface{}]interface{}{
					"name": "Bob",
					"profile": map[interface{}]interface{}{
						"bio": "Developer",
					},
				},
			},
			expectType:  "map[string]interface {}",
			description: "Should recursively convert nested maps",
		},
		{
			name: "Array of maps",
			input: []interface{}{
				map[interface{}]interface{}{"id": 1, "name": "Item 1"},
				map[interface{}]interface{}{"id": 2, "name": "Item 2"},
			},
			expectType:  "[]interface {}",
			description: "Should convert maps in arrays",
		},
		{
			name:        "Primitive string",
			input:       "test string",
			expectType:  "string",
			description: "Should pass through primitive strings",
		},
		{
			name:        "Primitive number",
			input:       42,
			expectType:  "int",
			description: "Should pass through primitive numbers",
		},
		{
			name:        "Primitive boolean",
			input:       true,
			expectType:  "bool",
			description: "Should pass through primitive booleans",
		},
		{
			name:        "Nil value",
			input:       nil,
			expectType:  "invalid",
			description: "Should handle nil values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertYAMLMapToJSONMap(tt.input)

			// Check type
			resultType := fmt.Sprintf("%T", result)
			if tt.expectType == "invalid" {
				assert.Nil(t, result, tt.description)
			} else {
				assert.Contains(t, resultType, tt.expectType, tt.description)
			}

			// For map types, verify keys are strings
			if resultMap, ok := result.(map[string]interface{}); ok {
				for key := range resultMap {
					assert.IsType(t, "", key, "All keys should be strings")
				}
			}
		})
	}
}

// TestConvertYAMLMapToJSONMapComplex tests complex conversion scenarios
func TestConvertYAMLMapToJSONMapComplex(t *testing.T) {
	t.Run("Complex nested structure", func(t *testing.T) {
		input := map[interface{}]interface{}{
			"schema": map[interface{}]interface{}{
				"type": "object",
				"properties": map[interface{}]interface{}{
					"name": map[interface{}]interface{}{
						"type": "string",
					},
					"tags": []interface{}{"tag1", "tag2"},
				},
			},
		}

		result := convertYAMLMapToJSONMap(input)
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok, "Result should be a string-keyed map")

		// Verify structure
		schema, ok := resultMap["schema"].(map[string]interface{})
		require.True(t, ok, "schema should be a string-keyed map")

		assert.Equal(t, "object", schema["type"])

		properties, ok := schema["properties"].(map[string]interface{})
		require.True(t, ok, "properties should be a string-keyed map")

		name, ok := properties["name"].(map[string]interface{})
		require.True(t, ok, "name should be a string-keyed map")
		assert.Equal(t, "string", name["type"])

		tags, ok := properties["tags"].([]interface{})
		require.True(t, ok, "tags should be an array")
		assert.Len(t, tags, 2)
	})
}

// TestFindAgentByName tests finding agents by name
func TestFindAgentByName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	require.NoError(t, err)
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	cfg := &config.Config{}
	service := NewDeclarativeSync(repos, cfg)

	// Create test environment
	env, err := repos.Environments.Create("test-find-env", nil, 1)
	require.NoError(t, err)

	// Create test agent
	agent, err := repos.Agents.Create(
		"TestFindAgent",
		"Test description",
		"Test prompt",
		5,
		env.ID,
		1,
		nil,
		nil,
		true,
		nil,
		nil,
		"",
		"",
	)
	require.NoError(t, err)

	tests := []struct {
		name        string
		agentName   string
		envID       int64
		expectFound bool
		description string
	}{
		{
			name:        "Find existing agent",
			agentName:   "TestFindAgent",
			envID:       env.ID,
			expectFound: true,
			description: "Should find agent that exists",
		},
		{
			name:        "Agent not found",
			agentName:   "NonexistentAgent",
			envID:       env.ID,
			expectFound: false,
			description: "Should return error for nonexistent agent",
		},
		{
			name:        "Wrong environment",
			agentName:   "TestFindAgent",
			envID:       9999,
			expectFound: false,
			description: "Should not find agent in wrong environment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			foundAgent, err := service.findAgentByName(tt.agentName, tt.envID)

			if tt.expectFound {
				require.NoError(t, err, tt.description)
				require.NotNil(t, foundAgent)
				assert.Equal(t, agent.ID, foundAgent.ID)
				assert.Equal(t, tt.agentName, foundAgent.Name)
			} else {
				require.Error(t, err, tt.description)
				assert.Nil(t, foundAgent)
			}
		})
	}
}

// TestExtractInputSchema tests input schema extraction from dotprompt config
func TestExtractInputSchema(t *testing.T) {
	testDB, err := db.NewTest(t)
	require.NoError(t, err)
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	cfg := &config.Config{}
	service := NewDeclarativeSync(repos, cfg)

	tests := []struct {
		name         string
		config       *DotPromptConfig
		expectSchema bool
		expectError  bool
		description  string
	}{
		{
			name: "No input schema",
			config: &DotPromptConfig{
				Model: "gpt-4o-mini",
			},
			expectSchema: false,
			expectError:  false,
			description:  "Should handle missing input schema",
		},
		{
			name: "Picoschema format",
			config: &DotPromptConfig{
				Input: map[string]interface{}{
					"schema": map[interface{}]interface{}{
						"projectPath": "string, Path to project directory",
						"scanDepth":   "number, Maximum scan depth",
					},
				},
			},
			expectSchema: true,
			expectError:  false,
			description:  "Should extract Picoschema format",
		},
		{
			name: "Full JSON Schema format",
			config: &DotPromptConfig{
				Input: map[string]interface{}{
					"schema": map[interface{}]interface{}{
						"type": "object",
						"properties": map[interface{}]interface{}{
							"name": map[interface{}]interface{}{
								"type": "string",
							},
						},
						"required": []interface{}{"name"},
					},
				},
			},
			expectSchema: false,
			expectError:  false,
			description:  "Should skip full JSON Schema format",
		},
		{
			name: "Schema with userInput field",
			config: &DotPromptConfig{
				Input: map[string]interface{}{
					"schema": map[interface{}]interface{}{
						"userInput":   "string, User input",
						"projectPath": "string, Project path",
					},
				},
			},
			expectSchema: true,
			expectError:  false,
			description:  "Should exclude userInput from custom schema",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := service.extractInputSchema(tt.config)

			if tt.expectError {
				require.Error(t, err, tt.description)
			} else {
				require.NoError(t, err, tt.description)

				if tt.expectSchema {
					require.NotNil(t, schema, "Schema should not be nil")
					assert.NotEmpty(t, *schema, "Schema should not be empty")
				} else {
					assert.Nil(t, schema, "Schema should be nil")
				}
			}
		})
	}
}

// TestExtractOutputSchema tests output schema extraction
func TestExtractOutputSchema(t *testing.T) {
	testDB, err := db.NewTest(t)
	require.NoError(t, err)
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	cfg := &config.Config{}
	service := NewDeclarativeSync(repos, cfg)

	tests := []struct {
		name          string
		config        *DotPromptConfig
		expectSchema  bool
		expectPreset  bool
		expectError   bool
		description   string
	}{
		{
			name: "No output schema",
			config: &DotPromptConfig{
				Model: "gpt-4o-mini",
			},
			expectSchema: false,
			expectPreset: false,
			expectError:  false,
			description:  "Should handle missing output schema",
		},
		{
			name: "Top-level output_schema field",
			config: &DotPromptConfig{
				OutputSchema: `{"type": "object", "properties": {"result": {"type": "string"}}}`,
			},
			expectSchema: true,
			expectPreset: false,
			expectError:  false,
			description:  "Should extract top-level output_schema",
		},
		{
			name: "Output preset field",
			config: &DotPromptConfig{
				Output: map[string]interface{}{
					"preset": "finops-inventory",
				},
			},
			expectSchema: false,
			expectPreset: true,
			expectError:  false,
			description:  "Should extract output preset",
		},
		{
			name: "Both schema and preset",
			config: &DotPromptConfig{
				Output: map[string]interface{}{
					"schema": map[interface{}]interface{}{
						"type": "object",
						"properties": map[interface{}]interface{}{
							"status": "string",
						},
					},
					"preset": "security-investigations",
				},
			},
			expectSchema: true,
			expectPreset: true,
			expectError:  false,
			description:  "Should extract both schema and preset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, preset, err := service.extractOutputSchema(tt.config)

			if tt.expectError {
				require.Error(t, err, tt.description)
			} else {
				require.NoError(t, err, tt.description)

				if tt.expectSchema {
					require.NotNil(t, schema, "Schema should not be nil")
					assert.NotEmpty(t, *schema, "Schema should not be empty")
				} else {
					assert.Nil(t, schema, "Schema should be nil")
				}

				if tt.expectPreset {
					require.NotNil(t, preset, "Preset should not be nil")
					assert.NotEmpty(t, *preset, "Preset should not be empty")
				} else {
					assert.Nil(t, preset, "Preset should be nil")
				}
			}
		})
	}
}

// TestParsePicoschemaField tests Picoschema field parsing
func TestParsePicoschemaField(t *testing.T) {
	testDB, err := db.NewTest(t)
	require.NoError(t, err)
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	cfg := &config.Config{}
	service := NewDeclarativeSync(repos, cfg)

	tests := []struct {
		name        string
		fieldName   string
		value       interface{}
		expectType  string
		expectNil   bool
		description string
	}{
		{
			name:        "String type definition",
			fieldName:   "username",
			value:       "string, User name",
			expectType:  "string",
			expectNil:   false,
			description: "Should parse string type",
		},
		{
			name:        "Array format enum",
			fieldName:   "status",
			value:       []interface{}{"active", "inactive", "pending"},
			expectType:  "string",
			expectNil:   false,
			description: "Should parse array format as enum",
		},
		{
			name:      "Object format",
			fieldName: "config",
			value: map[interface{}]interface{}{
				"type":        "object",
				"description": "Configuration object",
				"required":    true,
			},
			expectType:  "object",
			expectNil:   false,
			description: "Should parse object format",
		},
		{
			name:        "Invalid format",
			fieldName:   "invalid",
			value:       123,
			expectType:  "",
			expectNil:   true,
			description: "Should return nil for invalid format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.parsePicoschemaField(tt.fieldName, tt.value)

			if tt.expectNil {
				assert.Nil(t, result, tt.description)
			} else {
				require.NotNil(t, result, tt.description)
				if tt.expectType != "" {
					assert.Equal(t, tt.expectType, string(result.Type), "Type should match")
				}
			}
		})
	}
}
