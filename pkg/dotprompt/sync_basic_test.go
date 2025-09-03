package dotprompt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDotpromptBasics(t *testing.T) {
	testDir := t.TempDir()

	t.Run("RuntimeExtraction_Basic", func(t *testing.T) {
		// Test basic runtime extraction functionality
		agentContent := `---
metadata:
  name: "basic-test-agent"
  description: "Basic test agent"
model: "gpt-4"
tools: []
---

{{role "system"}}
You are a basic test agent.

{{role "user"}}
{{userInput}}
`

		agentFile := filepath.Join(testDir, "basic-test-agent.prompt")
		err := os.WriteFile(agentFile, []byte(agentContent), 0644)
		require.NoError(t, err)

		// Test RuntimeExtraction
		extractor, err := NewRuntimeExtraction(agentFile)
		require.NoError(t, err)
		assert.NotNil(t, extractor)

		// Test basic config extraction
		config := extractor.GetConfig()
		assert.Equal(t, "basic-test-agent", config.Metadata.Name)
		assert.Equal(t, "Basic test agent", config.Metadata.Description)
		assert.Equal(t, "gpt-4", config.Model)
		assert.Empty(t, config.Tools)

		// Test template extraction
		template := extractor.GetTemplate()
		assert.Contains(t, template, "{{role \"system\"}}")
		assert.Contains(t, template, "{{role \"user\"}}")
		assert.Contains(t, template, "{{userInput}}")
		assert.Contains(t, template, "basic test agent")
	})

	t.Run("RuntimeExtraction_WithTools", func(t *testing.T) {
		// Test extraction with tools
		agentContent := `---
metadata:
  name: "tools-test-agent"
  description: "Agent with tools"
model: "gpt-4"
tools:
  - "read_file"
  - "write_file"
  - "list_directory"
---

{{role "system"}}
You have access to file tools.

{{role "user"}}
{{userInput}}
`

		agentFile := filepath.Join(testDir, "tools-test-agent.prompt")
		err := os.WriteFile(agentFile, []byte(agentContent), 0644)
		require.NoError(t, err)

		extractor, err := NewRuntimeExtraction(agentFile)
		require.NoError(t, err)

		config := extractor.GetConfig()
		assert.Equal(t, "tools-test-agent", config.Metadata.Name)
		
		expectedTools := []string{"read_file", "write_file", "list_directory"}
		assert.Equal(t, expectedTools, config.Tools)
	})

	t.Run("RuntimeExtraction_WithMaxSteps", func(t *testing.T) {
		// Test extraction with max_steps in metadata
		agentContent := `---
metadata:
  name: "steps-test-agent"
  description: "Agent with max steps"
  max_steps: 10
model: "gpt-4"
tools: []
---

{{role "system"}}
You can take up to 10 steps.

{{role "user"}}
{{userInput}}
`

		agentFile := filepath.Join(testDir, "steps-test-agent.prompt")
		err := os.WriteFile(agentFile, []byte(agentContent), 0644)
		require.NoError(t, err)

		extractor, err := NewRuntimeExtraction(agentFile)
		require.NoError(t, err)

		config := extractor.GetConfig()
		assert.Equal(t, "steps-test-agent", config.Metadata.Name)
		assert.Equal(t, 10, config.Metadata.MaxSteps)
	})

	t.Run("RuntimeExtraction_FileNotFound", func(t *testing.T) {
		// Test error handling for non-existent file
		nonExistentFile := filepath.Join(testDir, "nonexistent.prompt")
		
		extractor, err := NewRuntimeExtraction(nonExistentFile)
		assert.Error(t, err)
		assert.Nil(t, extractor)
	})

	t.Run("RuntimeExtraction_InvalidYAML", func(t *testing.T) {
		// Test error handling for invalid YAML
		invalidContent := `---
metadata:
  name: "invalid-yaml-agent"
  description: "Agent with broken YAML"
model: "gpt-4"
tools:
  - "tool1
  - "tool2"  # Missing quote - invalid YAML
---

{{role "system"}}
Invalid YAML above.

{{role "user"}}
{{userInput}}
`

		agentFile := filepath.Join(testDir, "invalid-yaml-agent.prompt")
		err := os.WriteFile(agentFile, []byte(invalidContent), 0644)
		require.NoError(t, err)

		extractor, err := NewRuntimeExtraction(agentFile)
		assert.Error(t, err)
		assert.Nil(t, extractor)
	})
}

func TestDotpromptConfigStructure(t *testing.T) {
	t.Run("DotpromptConfig_Fields", func(t *testing.T) {
		// Test DotpromptConfig struct has expected fields
		config := &DotpromptConfig{}
		
		// Test that struct fields exist by setting them
		config.Model = "test-model"
		config.Tools = []string{"test-tool"}
		config.Metadata.Name = "test-name"
		config.Metadata.Description = "test-description"
		config.Metadata.MaxSteps = 5

		assert.Equal(t, "test-model", config.Model)
		assert.Equal(t, []string{"test-tool"}, config.Tools)
		assert.Equal(t, "test-name", config.Metadata.Name)
		assert.Equal(t, "test-description", config.Metadata.Description)
		assert.Equal(t, 5, config.Metadata.MaxSteps)
	})

	t.Run("AgentMetadata_Fields", func(t *testing.T) {
		// Test AgentMetadata struct has expected fields
		metadata := &AgentMetadata{}
		
		metadata.Name = "test-agent"
		metadata.Description = "test description"
		metadata.Version = "1.0.0"
		metadata.MaxSteps = 8
		metadata.AgentID = 123

		assert.Equal(t, "test-agent", metadata.Name)
		assert.Equal(t, "test description", metadata.Description)
		assert.Equal(t, "1.0.0", metadata.Version)
		assert.Equal(t, 8, metadata.MaxSteps)
		assert.Equal(t, int64(123), metadata.AgentID)
	})
}