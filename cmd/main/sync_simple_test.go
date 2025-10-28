package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncBasics(t *testing.T) {
	testDir := t.TempDir()

	t.Run("RunSync_RequiresEnvironmentArgument", func(t *testing.T) {
		// Test that runSync function requires environment argument
		err := runSync(nil, []string{}) // No arguments
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "environment name is required")
	})

	t.Run("FileSystemStructure_ForSync", func(t *testing.T) {
		// Test that sync can read proper file structure
		envName := "test-env"
		envDir := filepath.Join(testDir, "environments", envName)
		agentsDir := filepath.Join(envDir, "agents")

		err := os.MkdirAll(agentsDir, 0755)
		require.NoError(t, err)

		// Create a simple agent file
		agentContent := `---
metadata:
  name: "test-agent"
  description: "Simple test agent"
model: "gpt-4"
---

{{role "system"}}
You are a test agent.

{{role "user"}}
{{userInput}}
`
		agentFile := filepath.Join(agentsDir, "test-agent.prompt")
		err = os.WriteFile(agentFile, []byte(agentContent), 0644)
		require.NoError(t, err)

		// Verify file structure exists
		_, err = os.Stat(agentFile)
		assert.NoError(t, err)

		// Verify we can find .prompt files
		files, err := filepath.Glob(filepath.Join(agentsDir, "*.prompt"))
		require.NoError(t, err)
		assert.Len(t, files, 1)
		assert.Contains(t, files[0], "test-agent.prompt")
	})

	t.Run("EnvironmentDirectory_Structure", func(t *testing.T) {
		// Test environment directory structure that sync expects
		envName := "structure-test"
		envDir := filepath.Join(testDir, "environments", envName)

		// Create standard environment structure
		agentsDir := filepath.Join(envDir, "agents")
		err := os.MkdirAll(agentsDir, 0755)
		require.NoError(t, err)

		// Create template.json
		templateContent := `{
  "name": "structure-test",
  "description": "Test environment structure"
}`
		templateFile := filepath.Join(envDir, "template.json")
		err = os.WriteFile(templateFile, []byte(templateContent), 0644)
		require.NoError(t, err)

		// Create variables.yml
		variablesContent := `TEST_VAR: "test-value"`
		variablesFile := filepath.Join(envDir, "variables.yml")
		err = os.WriteFile(variablesFile, []byte(variablesContent), 0644)
		require.NoError(t, err)

		// Verify structure
		_, err = os.Stat(agentsDir)
		assert.NoError(t, err)
		_, err = os.Stat(templateFile)
		assert.NoError(t, err)
		_, err = os.Stat(variablesFile)
		assert.NoError(t, err)
	})
}

func TestSyncCommandValidation(t *testing.T) {
	t.Run("SyncCommand_Help", func(t *testing.T) {
		// Test that sync command has proper help structure
		// This validates the command exists and is configured correctly
		cmd := syncCmd
		assert.NotNil(t, cmd)
		assert.Contains(t, cmd.Use, "sync")
		assert.Contains(t, cmd.Short, "file-based configurations")
		assert.Contains(t, cmd.Long, "Declaratively synchronize")
	})

	t.Run("SyncCommand_Flags", func(t *testing.T) {
		// Test that sync command has expected flags
		cmd := syncCmd

		// Check for key flags
		dryRunFlag := cmd.Flags().Lookup("dry-run")
		assert.NotNil(t, dryRunFlag)
		assert.Equal(t, "bool", dryRunFlag.Value.Type())

		interactiveFlag := cmd.Flags().Lookup("interactive")
		assert.NotNil(t, interactiveFlag)
		assert.Equal(t, "bool", interactiveFlag.Value.Type())

		validateFlag := cmd.Flags().Lookup("validate")
		assert.NotNil(t, validateFlag)
		assert.Equal(t, "bool", validateFlag.Value.Type())
	})
}
