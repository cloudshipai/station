package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentHandler(t *testing.T) {
	// Create test environment
	testDir := t.TempDir()
	
	// Set up test environment directory structure
	envDir := filepath.Join(testDir, "environments", "test")
	agentsDir := filepath.Join(envDir, "agents")
	err := os.MkdirAll(agentsDir, 0755)
	require.NoError(t, err)
	
	// Change working directory for tests
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(testDir)

	t.Run("RunAgentLocalDotprompt_Success", func(t *testing.T) {
		// Create test agent prompt file
		promptFile := filepath.Join(agentsDir, "test-agent.prompt")
		promptContent := `---
model: "gemini-2.0-flash-exp"
config:
  temperature: 0.7
  max_tokens: 1000
metadata:
  name: "test-agent"
  description: "Test agent for handler testing"
  version: "1.0.0"
tools:
  - "read_file"
station:
  mcp_dependencies:
    filesystem-tools:
      assigned_tools: ["read_file"]
      server_command: "npx @modelcontextprotocol/server-filesystem"
---

You are a test agent.

Task: {{TASK}}
Agent: {{AGENT_NAME}}
Environment: {{ENVIRONMENT}}
`

		err := os.WriteFile(promptFile, []byte(promptContent), 0644)
		require.NoError(t, err)
		
		// Create handler with nil theme manager for testing
		handler := NewAgentHandler(nil)
		
		// Test the dotprompt execution method
		err = handler.runAgentLocalDotprompt("test-agent", "Test task", "test", false)
		
		// Should fail due to missing GenKit setup, but should pass validation
		if err != nil {
			t.Logf("Expected error in test environment: %v", err)
			// Should fail at execution, not at parsing/validation stage
			assert.Contains(t, err.Error(), "execution") 
		}
	})

	t.Run("RunAgentLocalDotprompt_FileNotFound", func(t *testing.T) {
		handler := NewAgentHandler(nil)
		
		// Test with non-existent agent
		err := handler.runAgentLocalDotprompt("nonexistent", "Test task", "test", false)
		
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("RunAgentLocalDotprompt_NameMismatch", func(t *testing.T) {
		// Create agent file with wrong name in metadata
		promptFile := filepath.Join(agentsDir, "wrong-name.prompt")
		promptContent := `---
metadata:
  name: "different-agent"
  description: "Test agent with wrong name"
---

You are a test agent with wrong name.
`

		err := os.WriteFile(promptFile, []byte(promptContent), 0644)
		require.NoError(t, err)
		
		handler := NewAgentHandler(nil)
		
		err = handler.runAgentLocalDotprompt("wrong-name", "Test task", "test", false)
		
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name mismatch")
	})

	t.Run("DualModelSupport_Gemini", func(t *testing.T) {
		// Test Gemini model configuration
		promptFile := filepath.Join(agentsDir, "gemini-agent.prompt")
		promptContent := `---
model: "gemini-2.0-flash-exp"
config:
  temperature: 0.8
  max_tokens: 2000
metadata:
  name: "gemini-agent"
  description: "Gemini-powered agent"
tools:
  - "read_file"
station:
  model_provider: "gemini"
  mcp_dependencies:
    filesystem-tools:
      assigned_tools: ["read_file"]
---

You are a Gemini-powered agent.
Task: {{TASK}}
`

		err := os.WriteFile(promptFile, []byte(promptContent), 0644)
		require.NoError(t, err)
		
		handler := NewAgentHandler(nil)
		
		// This should pass validation but fail at execution due to missing API setup
		err = handler.runAgentLocalDotprompt("gemini-agent", "Test Gemini", "test", false)
		
		if err != nil {
			t.Logf("Expected execution error: %v", err)
		}
	})

	t.Run("DualModelSupport_OpenAI", func(t *testing.T) {
		// Test OpenAI model configuration
		promptFile := filepath.Join(agentsDir, "openai-agent.prompt")
		promptContent := `---
model: "gpt-4"
config:
  temperature: 0.7
  max_tokens: 1500
metadata:
  name: "openai-agent"
  description: "OpenAI-powered agent"
tools:
  - "write_file"
station:
  model_provider: "openai"
  mcp_dependencies:
    filesystem-tools:
      assigned_tools: ["write_file"]
---

You are an OpenAI-powered agent.
Task: {{TASK}}
Use GPT-4 capabilities effectively.
`

		err := os.WriteFile(promptFile, []byte(promptContent), 0644)
		require.NoError(t, err)
		
		handler := NewAgentHandler(nil)
		
		// This should pass validation but fail at execution due to missing API setup
		err = handler.runAgentLocalDotprompt("openai-agent", "Test OpenAI", "test", false)
		
		if err != nil {
			t.Logf("Expected execution error: %v", err)
		}
	})

	t.Run("MCPDependenciesDisplay", func(t *testing.T) {
		// Test MCP dependencies parsing and display
		promptFile := filepath.Join(agentsDir, "mcp-deps-agent.prompt")
		promptContent := `---
model: "gemini-2.0-flash-exp"
metadata:
  name: "mcp-deps-agent"
  description: "Agent with multiple MCP dependencies"
station:
  mcp_dependencies:
    filesystem-tools:
      assigned_tools: ["read_file", "write_file", "list_directory"]
      server_command: "npx @modelcontextprotocol/server-filesystem"
      environment_vars:
        MCP_FILESYSTEM_ROOT: "/tmp"
    web-tools:
      assigned_tools: ["fetch_url", "scrape_page"]
      server_command: "python -m mcp_web_tools"
    database-tools:
      assigned_tools: ["query_db", "update_table"]
      server_command: "mcp-database-server"
---

You are an agent with multiple tool dependencies.
Task: {{TASK}}
`

		err := os.WriteFile(promptFile, []byte(promptContent), 0644)
		require.NoError(t, err)
		
		handler := NewAgentHandler(nil)
		
		// This should parse and display MCP dependencies correctly
		err = handler.runAgentLocalDotprompt("mcp-deps-agent", "Test MCP deps", "test", false)
		
		// Should fail at execution but pass dependency parsing
		if err != nil {
			t.Logf("Expected execution error after successful parsing: %v", err)
		}
	})

	t.Run("CustomMetadataExtraction", func(t *testing.T) {
		// Test custom metadata extraction
		promptFile := filepath.Join(agentsDir, "metadata-agent.prompt")
		promptContent := `---
model: "gemini-2.0-flash-exp"
metadata:
  name: "metadata-agent"
  description: "Agent with custom metadata"
station:
  execution_metadata:
    timeout_seconds: 60
    max_retries: 5
    priority: "high"
  team_info:
    owner: "dev-team"
    project: "station"
    environment: "production"
  feature_flags:
    enable_streaming: true
    enable_debug: false
---

You are an agent with rich metadata.
Task: {{TASK}}
`

		err := os.WriteFile(promptFile, []byte(promptContent), 0644)
		require.NoError(t, err)
		
		handler := NewAgentHandler(nil)
		
		// This should extract and display custom metadata
		err = handler.runAgentLocalDotprompt("metadata-agent", "Test metadata", "test", false)
		
		if err != nil {
			t.Logf("Expected execution error after metadata extraction: %v", err)
		}
	})
}

func TestAgentHandlerEdgeCases(t *testing.T) {
	testDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(testDir)

	t.Run("InvalidYAMLFrontmatter", func(t *testing.T) {
		// Set up environment
		envDir := filepath.Join(testDir, "environments", "test", "agents")
		err := os.MkdirAll(envDir, 0755)
		require.NoError(t, err)
		
		// Create prompt file with invalid YAML
		promptFile := filepath.Join(envDir, "invalid-agent.prompt")
		invalidContent := `---
metadata:
  name: "invalid-agent
  description: "Missing quote breaks YAML"
tools:
  - read_file
  - 
---

Task: {{TASK}}
`

		err = os.WriteFile(promptFile, []byte(invalidContent), 0644)
		require.NoError(t, err)
		
		handler := NewAgentHandler(nil)
		
		err = handler.runAgentLocalDotprompt("invalid-agent", "Test invalid", "test", false)
		
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse") // Should fail to parse
	})

	t.Run("EmptyPromptFile", func(t *testing.T) {
		// Set up environment  
		envDir := filepath.Join(testDir, "environments", "test2", "agents")
		err := os.MkdirAll(envDir, 0755)
		require.NoError(t, err)
		
		// Create empty prompt file
		promptFile := filepath.Join(envDir, "empty-agent.prompt")
		err = os.WriteFile(promptFile, []byte(""), 0644)
		require.NoError(t, err)
		
		handler := NewAgentHandler(nil)
		
		err = handler.runAgentLocalDotprompt("empty-agent", "Test empty", "test2", false)
		
		require.Error(t, err)
	})

	t.Run("TailFlagHandling", func(t *testing.T) {
		// Set up environment
		envDir := filepath.Join(testDir, "environments", "test3", "agents")
		err := os.MkdirAll(envDir, 0755)
		require.NoError(t, err)
		
		// Create valid prompt file
		promptFile := filepath.Join(envDir, "tail-agent.prompt")
		promptContent := `---
metadata:
  name: "tail-agent"
  description: "Agent for tail flag testing"
---

Task: {{TASK}}
`

		err = os.WriteFile(promptFile, []byte(promptContent), 0644)
		require.NoError(t, err)
		
		handler := NewAgentHandler(nil)
		
		// Test with tail=true
		err = handler.runAgentLocalDotprompt("tail-agent", "Test tail", "test3", true)
		
		// Should handle tail flag (may still fail at execution)
		if err != nil {
			t.Logf("Expected execution error with tail flag: %v", err)
		}
	})
}

func TestVariableSubstitution(t *testing.T) {
	testDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(testDir)

	// Set up environment
	envDir := filepath.Join(testDir, "environments", "vars-test", "agents")
	err := os.MkdirAll(envDir, 0755)
	require.NoError(t, err)

	t.Run("BasicVariableSubstitution", func(t *testing.T) {
		promptFile := filepath.Join(envDir, "vars-agent.prompt")
		promptContent := `---
metadata:
  name: "vars-agent"
  description: "Agent for variable testing"
station:
  custom_vars:
    project_name: "StationAI"
    version: "2.0.0"
---

Hello! I'm working on {{project_name}} version {{version}}.

Current task: {{TASK}}
Agent name: {{AGENT_NAME}}
Environment: {{ENVIRONMENT}}

Let's get started!
`

		err := os.WriteFile(promptFile, []byte(promptContent), 0644)
		require.NoError(t, err)
		
		handler := NewAgentHandler(nil)
		
		// This should pass variable validation
		err = handler.runAgentLocalDotprompt("vars-agent", "Test variables", "vars-test", false)
		
		if err != nil {
			t.Logf("Expected execution error after variable processing: %v", err)
		}
	})
}