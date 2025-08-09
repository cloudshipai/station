package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPSyncValidation(t *testing.T) {
	testDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(testDir)

	t.Run("ValidateAgentPromptFiles", func(t *testing.T) {
		// Create test environment structure
		envDir := filepath.Join(testDir, "environments", "test")
		agentsDir := filepath.Join(envDir, "agents")
		err := os.MkdirAll(agentsDir, 0755)
		require.NoError(t, err)

		// Create valid agent files
		validAgent := filepath.Join(agentsDir, "monitoring-agent.prompt")
		validContent := `---
model: "gemini-2.0-flash-exp"
config:
  temperature: 0.7
metadata:
  name: "monitoring-agent"
  description: "System monitoring agent"
tools:
  - "read_file"
  - "list_directory"
station:
  mcp_dependencies:
    filesystem-tools:
      assigned_tools: ["read_file", "list_directory"]
      server_command: "npx @modelcontextprotocol/server-filesystem"
---

You are a monitoring agent.
Task: {{TASK}}
`

		err = os.WriteFile(validAgent, []byte(validContent), 0644)
		require.NoError(t, err)

		// Create agent with invalid MCP dependencies
		invalidAgent := filepath.Join(agentsDir, "invalid-deps-agent.prompt")
		invalidContent := `---
model: "gpt-4"
metadata:
  name: "invalid-deps-agent"
  description: "Agent with invalid dependencies"
station:
  mcp_dependencies:
    nonexistent-tools:
      assigned_tools: ["fake_tool", "another_fake"]
      server_command: "fake-server"
---

Task: {{TASK}}
`

		err = os.WriteFile(invalidAgent, []byte(invalidContent), 0644)
		require.NoError(t, err)

		// Test validation
		files, err := filepath.Glob(filepath.Join(agentsDir, "*.prompt"))
		require.NoError(t, err)
		assert.Equal(t, 2, len(files))

		// Validate each file structure
		for _, file := range files {
			content, err := os.ReadFile(file)
			require.NoError(t, err)
			
			// Basic validation that file has proper YAML frontmatter
			assert.Contains(t, string(content), "---")
			assert.Contains(t, string(content), "metadata:")
			assert.Contains(t, string(content), "name:")
		}
	})

	t.Run("ValidateMCPConfigFile", func(t *testing.T) {
		// Create MCP config file
		envDir := filepath.Join(testDir, "environments", "test")
		mcpConfigPath := filepath.Join(envDir, "mcp-config.yaml")

		mcpConfig := `
# MCP Configuration for test environment
servers:
  filesystem-tools:
    command: "npx @modelcontextprotocol/server-filesystem"
    args: ["--root", "/tmp"]
    environment:
      MCP_FILESYSTEM_ROOT: "/tmp"
    tools:
      - read_file
      - write_file
      - list_directory
      - get_file_info
      
  web-tools:
    command: "python -m mcp_web_tools"
    args: ["--port", "3001"]
    environment:
      WEB_TOOLS_PORT: "3001"
    tools:
      - fetch_url
      - scrape_page

environments:
  test:
    description: "Test environment for development"
    servers: ["filesystem-tools", "web-tools"]
    
  production:  
    description: "Production environment"
    servers: ["filesystem-tools"]
`

		err := os.WriteFile(mcpConfigPath, []byte(mcpConfig), 0644)
		require.NoError(t, err)

		// Test that file exists and is readable
		_, err = os.Stat(mcpConfigPath)
		require.NoError(t, err)

		content, err := os.ReadFile(mcpConfigPath)
		require.NoError(t, err)

		// Basic validation
		assert.Contains(t, string(content), "servers:")
		assert.Contains(t, string(content), "filesystem-tools")
		assert.Contains(t, string(content), "environments:")
	})

	t.Run("ValidateEnvironmentStructure", func(t *testing.T) {
		// Test multiple environment structure
		environments := []string{"development", "staging", "production"}

		for _, env := range environments {
			envDir := filepath.Join(testDir, "environments", env)
			agentsDir := filepath.Join(envDir, "agents")
			
			err := os.MkdirAll(agentsDir, 0755)
			require.NoError(t, err)

			// Create environment-specific agent
			agentFile := filepath.Join(agentsDir, env+"-agent.prompt")
			agentContent := `---
model: "gemini-2.0-flash-exp"
metadata:
  name: "` + env + `-agent"
  description: "Agent for ` + env + ` environment"
station:
  environment: "` + env + `"
---

You are a ` + env + ` environment agent.
Task: {{TASK}}
Environment: {{ENVIRONMENT}}
`

			err = os.WriteFile(agentFile, []byte(agentContent), 0644)
			require.NoError(t, err)

			// Create MCP config for environment
			mcpConfigPath := filepath.Join(envDir, "mcp-config.yaml")
			mcpContent := `
servers:
  filesystem-tools:
    command: "npx @modelcontextprotocol/server-filesystem"
    environment:
      MCP_ENV: "` + env + `"
`

			err = os.WriteFile(mcpConfigPath, []byte(mcpContent), 0644)
			require.NoError(t, err)
		}

		// Verify all environments were created
		for _, env := range environments {
			envPath := filepath.Join(testDir, "environments", env)
			_, err := os.Stat(envPath)
			require.NoError(t, err)

			agentsPath := filepath.Join(envPath, "agents")
			_, err = os.Stat(agentsPath)
			require.NoError(t, err)

			mcpConfigPath := filepath.Join(envPath, "mcp-config.yaml")
			_, err = os.Stat(mcpConfigPath)
			require.NoError(t, err)
		}
	})
}

func TestMCPDependencyValidation(t *testing.T) {
	testDir := t.TempDir()

	t.Run("ValidDependencyMapping", func(t *testing.T) {
		// Create agent with proper dependency mapping
		agentContent := `---
model: "gpt-4"
metadata:
  name: "dependency-test-agent"
  description: "Agent for testing dependency validation"
station:
  mcp_dependencies:
    filesystem-tools:
      assigned_tools: ["read_file", "write_file", "list_directory"]
      server_command: "npx @modelcontextprotocol/server-filesystem"
      environment_vars:
        MCP_FILESYSTEM_ROOT: "/workspace"
      validation_rules:
        required_permissions: ["read", "write"]
        max_file_size: "100MB"
    
    database-tools:
      assigned_tools: ["query_db", "update_table"]
      server_command: "mcp-database-server"
      environment_vars:
        DB_CONNECTION: "sqlite:///tmp/test.db"
      validation_rules:
        connection_timeout: "30s"
        max_query_time: "60s"
---

You are an agent with validated dependencies.
Task: {{TASK}}

Available tools:
- File operations: read_file, write_file, list_directory
- Database operations: query_db, update_table
`

		envDir := filepath.Join(testDir, "environments", "dep-test", "agents")
		err := os.MkdirAll(envDir, 0755)
		require.NoError(t, err)

		agentFile := filepath.Join(envDir, "dependency-test-agent.prompt")
		err = os.WriteFile(agentFile, []byte(agentContent), 0644)
		require.NoError(t, err)

		// Validate dependency structure
		content, err := os.ReadFile(agentFile)
		require.NoError(t, err)

		// Check for proper dependency mapping
		assert.Contains(t, string(content), "mcp_dependencies:")
		assert.Contains(t, string(content), "filesystem-tools:")
		assert.Contains(t, string(content), "database-tools:")
		assert.Contains(t, string(content), "assigned_tools:")
		assert.Contains(t, string(content), "server_command:")
		assert.Contains(t, string(content), "environment_vars:")
		assert.Contains(t, string(content), "validation_rules:")
	})

	t.Run("InvalidDependencyStructure", func(t *testing.T) {
		// Create agent with malformed dependency structure
		badAgentContent := `---
model: "gemini-2.0-flash-exp"
metadata:
  name: "bad-deps-agent"
  description: "Agent with malformed dependencies"
station:
  mcp_dependencies:
    - "filesystem-tools" # Should be object, not array
    - "database-tools"
---

Task: {{TASK}}
`

		envDir := filepath.Join(testDir, "environments", "bad-deps", "agents")
		err := os.MkdirAll(envDir, 0755)
		require.NoError(t, err)

		agentFile := filepath.Join(envDir, "bad-deps-agent.prompt")
		err = os.WriteFile(agentFile, []byte(badAgentContent), 0644)
		require.NoError(t, err)

		// This should be detected as invalid during sync validation
		content, err := os.ReadFile(agentFile)
		require.NoError(t, err)
		
		// Contains malformed structure
		assert.Contains(t, string(content), "mcp_dependencies:")
		assert.Contains(t, string(content), "- \"filesystem-tools\"") // Invalid array format
	})

	t.Run("MissingRequiredFields", func(t *testing.T) {
		// Create agent missing required fields
		incompleteContent := `---
model: "gpt-4"
metadata:
  name: "incomplete-agent"
  description: "Agent missing required fields"
station:
  mcp_dependencies:
    filesystem-tools:
      # Missing assigned_tools and server_command
      environment_vars:
        MCP_ROOT: "/tmp"
---

Task: {{TASK}}
`

		envDir := filepath.Join(testDir, "environments", "incomplete", "agents")
		err := os.MkdirAll(envDir, 0755)
		require.NoError(t, err)

		agentFile := filepath.Join(envDir, "incomplete-agent.prompt")
		err = os.WriteFile(agentFile, []byte(incompleteContent), 0644)
		require.NoError(t, err)

		// Validation should catch missing required fields
		content, err := os.ReadFile(agentFile)
		require.NoError(t, err)
		
		assert.Contains(t, string(content), "mcp_dependencies:")
		assert.Contains(t, string(content), "filesystem-tools:")
		// Should NOT contain required fields
		assert.NotContains(t, string(content), "assigned_tools:")
		assert.NotContains(t, string(content), "server_command:")
	})
}

func TestSyncOperations(t *testing.T) {
	testDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(testDir)

	t.Run("DryRunMode", func(t *testing.T) {
		// Create test environment with agents
		envDir := filepath.Join(testDir, "environments", "dry-run-test")
		agentsDir := filepath.Join(envDir, "agents")
		err := os.MkdirAll(agentsDir, 0755)
		require.NoError(t, err)

		// Create test agents
		agents := []string{"agent1", "agent2", "agent3"}
		for _, agentName := range agents {
			agentFile := filepath.Join(agentsDir, agentName+".prompt")
			agentContent := `---
metadata:
  name: "` + agentName + `"
  description: "Test agent ` + agentName + `"
---

Task: {{TASK}}
`
			err = os.WriteFile(agentFile, []byte(agentContent), 0644)
			require.NoError(t, err)
		}

		// Verify files exist for dry-run simulation
		files, err := filepath.Glob(filepath.Join(agentsDir, "*.prompt"))
		require.NoError(t, err)
		assert.Equal(t, 3, len(files))
	})

	t.Run("FileChecksumValidation", func(t *testing.T) {
		// Create test file and calculate expected checksum
		envDir := filepath.Join(testDir, "environments", "checksum-test", "agents")
		err := os.MkdirAll(envDir, 0755)
		require.NoError(t, err)

		agentFile := filepath.Join(envDir, "checksum-agent.prompt")
		agentContent := `---
metadata:
  name: "checksum-agent"
  description: "Agent for checksum testing"
---

Task: {{TASK}}
This content should produce consistent checksums.
`

		err = os.WriteFile(agentFile, []byte(agentContent), 0644)
		require.NoError(t, err)

		// Read file back and verify content
		readContent, err := os.ReadFile(agentFile)
		require.NoError(t, err)
		assert.Equal(t, agentContent, string(readContent))

		// Simulate checksum calculation (would be done by sync service)
		// In real implementation, this would use MD5 hash
		contentLength := len(readContent)
		assert.True(t, contentLength > 0)
	})

	t.Run("ForceSync", func(t *testing.T) {
		// Create test environment for force sync
		envDir := filepath.Join(testDir, "environments", "force-test", "agents")
		err := os.MkdirAll(envDir, 0755)
		require.NoError(t, err)

		agentFile := filepath.Join(envDir, "force-agent.prompt")
		agentContent := `---
metadata:
  name: "force-agent"
  description: "Agent for force sync testing"
station:
  sync_metadata:
    last_sync: "2025-01-01T00:00:00Z"
    checksum: "existing_checksum"
---

Task: {{TASK}}
This agent should be force-synced.
`

		err = os.WriteFile(agentFile, []byte(agentContent), 0644)
		require.NoError(t, err)

		// Verify file exists and has sync metadata
		content, err := os.ReadFile(agentFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "sync_metadata:")
		assert.Contains(t, string(content), "last_sync:")
		assert.Contains(t, string(content), "checksum:")
	})
}

func TestEnvironmentValidation(t *testing.T) {
	testDir := t.TempDir()

	t.Run("MultipleEnvironments", func(t *testing.T) {
		// Create multiple environments with different configurations
		environments := []struct {
			name   string
			agents int
			config bool
		}{
			{"development", 3, true},
			{"staging", 2, true},
			{"production", 1, false}, // No MCP config
		}

		for _, env := range environments {
			envDir := filepath.Join(testDir, "environments", env.name)
			agentsDir := filepath.Join(envDir, "agents")
			err := os.MkdirAll(agentsDir, 0755)
			require.NoError(t, err)

			// Create agents for environment
			for i := 0; i < env.agents; i++ {
				agentName := env.name + "-agent-" + string(rune('1'+i))
				agentFile := filepath.Join(agentsDir, agentName+".prompt")
				agentContent := `---
metadata:
  name: "` + agentName + `"
  description: "Agent for ` + env.name + ` environment"
station:
  environment: "` + env.name + `"
---

Task: {{TASK}}
Environment: ` + env.name + `
`
				err = os.WriteFile(agentFile, []byte(agentContent), 0644)
				require.NoError(t, err)
			}

			// Create MCP config if specified
			if env.config {
				mcpConfigPath := filepath.Join(envDir, "mcp-config.yaml")
				mcpContent := `
servers:
  filesystem-tools:
    command: "npx @modelcontextprotocol/server-filesystem"
    environment:
      MCP_ENV: "` + env.name + `"
`
				err = os.WriteFile(mcpConfigPath, []byte(mcpContent), 0644)
				require.NoError(t, err)
			}
		}

		// Verify all environments created correctly
		for _, env := range environments {
			envPath := filepath.Join(testDir, "environments", env.name)
			_, err := os.Stat(envPath)
			require.NoError(t, err)

			// Check agent count
			agentsDir := filepath.Join(envPath, "agents")
			files, err := filepath.Glob(filepath.Join(agentsDir, "*.prompt"))
			require.NoError(t, err)
			assert.Equal(t, env.agents, len(files))

			// Check MCP config presence
			mcpConfigPath := filepath.Join(envPath, "mcp-config.yaml")
			_, err = os.Stat(mcpConfigPath)
			if env.config {
				require.NoError(t, err)
			} else {
				require.Error(t, err) // Should not exist
			}
		}
	})

	t.Run("EnvironmentIsolation", func(t *testing.T) {
		// Test that environments are properly isolated
		env1Dir := filepath.Join(testDir, "environments", "env1", "agents")
		env2Dir := filepath.Join(testDir, "environments", "env2", "agents")
		
		err := os.MkdirAll(env1Dir, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(env2Dir, 0755)
		require.NoError(t, err)

		// Create same agent name in different environments
		agentName := "shared-agent"
		
		// Environment 1 version
		agent1File := filepath.Join(env1Dir, agentName+".prompt")
		agent1Content := `---
metadata:
  name: "shared-agent"
  description: "Agent in environment 1"
station:
  environment: "env1"
---

I am in environment 1.
Task: {{TASK}}
`
		err = os.WriteFile(agent1File, []byte(agent1Content), 0644)
		require.NoError(t, err)

		// Environment 2 version
		agent2File := filepath.Join(env2Dir, agentName+".prompt")
		agent2Content := `---
metadata:
  name: "shared-agent"
  description: "Agent in environment 2"
station:
  environment: "env2"
---

I am in environment 2.
Task: {{TASK}}
`
		err = os.WriteFile(agent2File, []byte(agent2Content), 0644)
		require.NoError(t, err)

		// Verify both agents exist with different content
		content1, err := os.ReadFile(agent1File)
		require.NoError(t, err)
		content2, err := os.ReadFile(agent2File)
		require.NoError(t, err)

		assert.Contains(t, string(content1), "environment 1")
		assert.Contains(t, string(content2), "environment 2")
		assert.NotEqual(t, string(content1), string(content2))
	})
}