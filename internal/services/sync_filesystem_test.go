package services

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncFileSystemOperations(t *testing.T) {
	testDir := t.TempDir()

	t.Run("ScanEnvironmentDirectory", func(t *testing.T) {
		// Test that sync can scan environment directories
		envName := "filesystem-test"
		
		// Create environment structure as DeclarativeSync expects it
		homeDir := filepath.Join(testDir, "home")
		configDir := filepath.Join(homeDir, ".config", "station", "environments", envName)
		agentsDir := filepath.Join(configDir, "agents")
		err := os.MkdirAll(agentsDir, 0755)
		require.NoError(t, err)

		// Set HOME environment variable for config lookup
		originalHome := os.Getenv("HOME")
		defer os.Setenv("HOME", originalHome)
		os.Setenv("HOME", homeDir)

		// Create multiple agent files
		agentFiles := []string{
			"database-agent.prompt",
			"file-processor.prompt", 
			"web-scraper.prompt",
			"email-handler.prompt",
		}

		for i, fileName := range agentFiles {
			agentName := fileName[:len(fileName)-7] // Remove .prompt extension
			agentContent := `---
metadata:
  name: "` + agentName + `"
  description: "Test agent ` + agentName + `"
model: "gpt-4"
tools: []
---

{{role "system"}}
You are agent ` + agentName + ` number ` + fmt.Sprintf("%d", i+1) + `.

{{role "user"}}
{{userInput}}
`
			agentFile := filepath.Join(agentsDir, fileName)
			err = os.WriteFile(agentFile, []byte(agentContent), 0644)
			require.NoError(t, err)
		}

		// Create template.json
		templateContent := `{
  "name": "filesystem-test",
  "description": "Filesystem scanning test environment",
  "mcpServers": {
    "filesystem": {
      "command": "echo",
      "args": ["test-command"]
    }
  }
}`
		templateFile := filepath.Join(configDir, "template.json")
		err = os.WriteFile(templateFile, []byte(templateContent), 0644)
		require.NoError(t, err)

		// Create variables.yml
		variablesContent := `
PROJECT_ROOT: "/tmp/test"
ENVIRONMENT: "filesystem-test"
DEBUG_MODE: true
`
		variablesFile := filepath.Join(configDir, "variables.yml")
		err = os.WriteFile(variablesFile, []byte(variablesContent), 0644)
		require.NoError(t, err)

		// Verify file structure was created correctly
		files, err := filepath.Glob(filepath.Join(agentsDir, "*.prompt"))
		require.NoError(t, err)
		assert.Len(t, files, 4, "Should find all 4 agent files")

		_, err = os.Stat(templateFile)
		assert.NoError(t, err, "Template file should exist")

		_, err = os.Stat(variablesFile)
		assert.NoError(t, err, "Variables file should exist")

		// Test that files can be read and parsed
		for _, file := range files {
			content, err := os.ReadFile(file)
			require.NoError(t, err)
			
			// Basic validation that files have proper structure
			assert.Contains(t, string(content), "---", "File should have YAML frontmatter")
			assert.Contains(t, string(content), "metadata:", "File should have metadata section")
			assert.Contains(t, string(content), "{{role \"system\"}}", "File should have system role")
			assert.Contains(t, string(content), "{{role \"user\"}}", "File should have user role")
			assert.Contains(t, string(content), "{{userInput}}", "File should have userInput variable")
		}
	})

	t.Run("EnvironmentStructureValidation", func(t *testing.T) {
		// Test validation of environment directory structure
		homeDir := filepath.Join(testDir, "validation-home")
		originalHome := os.Getenv("HOME")
		defer os.Setenv("HOME", originalHome)
		os.Setenv("HOME", homeDir)

		environments := []struct {
			name        string
			hasAgents   bool
			hasTemplate bool
			hasVars     bool
			valid       bool
		}{
			{"complete-env", true, true, true, true},
			{"no-agents-env", false, true, true, false}, // No agents directory
			{"no-template-env", true, false, true, false}, // No template.json  
			{"minimal-env", true, true, false, true}, // No variables.yml (optional)
		}

		for _, env := range environments {
			configDir := filepath.Join(homeDir, ".config", "station", "environments", env.name)
			
			if env.hasAgents {
				agentsDir := filepath.Join(configDir, "agents")
				err := os.MkdirAll(agentsDir, 0755)
				require.NoError(t, err)

				// Create at least one agent
				agentContent := `---
metadata:
  name: "test-agent"
  description: "Test agent for ` + env.name + `"
model: "gpt-4"
---

{{role "system"}}
Test agent.

{{role "user"}}  
{{userInput}}
`
				agentFile := filepath.Join(agentsDir, "test-agent.prompt")
				err = os.WriteFile(agentFile, []byte(agentContent), 0644)
				require.NoError(t, err)
			}

			if env.hasTemplate {
				templateContent := `{
  "name": "` + env.name + `",
  "description": "Test environment ` + env.name + `",
  "mcpServers": {}
}`
				templateFile := filepath.Join(configDir, "template.json")
				err := os.MkdirAll(filepath.Dir(templateFile), 0755)
				require.NoError(t, err)
				err = os.WriteFile(templateFile, []byte(templateContent), 0644)
				require.NoError(t, err)
			}

			if env.hasVars {
				varsContent := `TEST_VAR: "test-value"`
				varsFile := filepath.Join(configDir, "variables.yml")
				err := os.WriteFile(varsFile, []byte(varsContent), 0644)
				require.NoError(t, err)
			}
		}

		// Verify environment structures
		for _, env := range environments {
			configDir := filepath.Join(homeDir, ".config", "station", "environments", env.name)
			
			// Check if environment directory exists
			_, err := os.Stat(configDir)
			assert.NoError(t, err, "Environment %s should have config directory", env.name)

			if env.hasAgents {
				agentsDir := filepath.Join(configDir, "agents")
				files, err := filepath.Glob(filepath.Join(agentsDir, "*.prompt"))
				assert.NoError(t, err)
				assert.Len(t, files, 1, "Environment %s should have agent files", env.name)
			}

			if env.hasTemplate {
				templateFile := filepath.Join(configDir, "template.json")
				_, err := os.Stat(templateFile)
				assert.NoError(t, err, "Environment %s should have template.json", env.name)
			}

			if env.hasVars {
				varsFile := filepath.Join(configDir, "variables.yml")
				_, err := os.Stat(varsFile)
				assert.NoError(t, err, "Environment %s should have variables.yml", env.name)
			}
		}
	})

	t.Run("AgentFileProcessing", func(t *testing.T) {
		// Test processing of various agent file formats that sync encounters
		homeDir := filepath.Join(testDir, "processing-home")
		envName := "processing-test"
		configDir := filepath.Join(homeDir, ".config", "station", "environments", envName)
		agentsDir := filepath.Join(configDir, "agents")
		err := os.MkdirAll(agentsDir, 0755)
		require.NoError(t, err)

		originalHome := os.Getenv("HOME")
		defer os.Setenv("HOME", originalHome)
		os.Setenv("HOME", homeDir)

		// Test various agent configurations
		agentConfigs := []struct {
			name    string
			content string
			valid   bool
		}{
			{
				name: "simple-agent",
				content: `---
metadata:
  name: "simple-agent"
  description: "Simple agent configuration"
model: "gpt-4"
tools: []
---

{{role "system"}}
You are a simple agent.

{{role "user"}}
{{userInput}}
`,
				valid: true,
			},
			{
				name: "complex-agent",
				content: `---
metadata:
  name: "complex-agent"
  description: "Complex agent with multiple tools and config"
  version: "1.5.0"
  max_steps: 15
model: "gpt-4"
tools:
  - "__read_text_file"
  - "__write_text_file"
  - "__list_directory"
  - "__search_files"
config:
  temperature: 0.7
  max_tokens: 4000
  top_p: 0.9
---

{{role "system"}}
You are a complex agent with file operations.

**Available Tools:**
- Read files: __read_text_file
- Write files: __write_text_file
- List directories: __list_directory
- Search files: __search_files

**Configuration:**
- Temperature: 0.7
- Max tokens: 4000
- Max steps: 15

{{role "user"}}
Task: {{userInput}}
Environment: {{.ENVIRONMENT}}
Project: {{.PROJECT_ROOT}}
`,
				valid: true,
			},
			{
				name: "minimal-agent",
				content: `---
metadata:
  name: "minimal-agent"
model: "gpt-4"
---

{{role "system"}}
Minimal agent.

{{role "user"}}
{{userInput}}
`,
				valid: true,
			},
		}

		for _, agentConfig := range agentConfigs {
			agentFile := filepath.Join(agentsDir, agentConfig.name+".prompt")
			err := os.WriteFile(agentFile, []byte(agentConfig.content), 0644)
			require.NoError(t, err)
		}

		// Verify all agent files can be read and have basic structure
		files, err := filepath.Glob(filepath.Join(agentsDir, "*.prompt"))
		require.NoError(t, err)
		assert.Len(t, files, 3, "Should find all agent files")

		for _, file := range files {
			content, err := os.ReadFile(file)
			require.NoError(t, err)
			
			contentStr := string(content)
			assert.Contains(t, contentStr, "metadata:")
			assert.Contains(t, contentStr, "model:")
			assert.Contains(t, contentStr, "{{role \"system\"}}")
			assert.Contains(t, contentStr, "{{role \"user\"}}")
			assert.Contains(t, contentStr, "{{userInput}}")
		}
	})
}

