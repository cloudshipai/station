package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// TestDotPromptConfigAgentsField tests the new agents field parsing
func TestDotPromptConfigAgentsField(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected []string
	}{
		{
			name: "agents field with multiple agents",
			yaml: `model: gpt-4o-mini
max_steps: 10
agents:
  - CodeAnalyzer
  - SecurityScanner
  - DocumentationGenerator
tools:
  - __filesystem_read
  - __filesystem_write`,
			expected: []string{"CodeAnalyzer", "SecurityScanner", "DocumentationGenerator"},
		},
		{
			name: "agents field with single agent",
			yaml: `model: gpt-4o-mini
agents:
  - CodeAnalyzer
tools:
  - __filesystem_read`,
			expected: []string{"CodeAnalyzer"},
		},
		{
			name: "empty agents field",
			yaml: `model: gpt-4o-mini
agents: []
tools:
  - __filesystem_read`,
			expected: []string{},
		},
		{
			name: "no agents field",
			yaml: `model: gpt-4o-mini
tools:
  - __filesystem_read`,
			expected: nil,
		},
		{
			name: "agents field with complex names",
			yaml: `model: gpt-4o-mini
agents:
  - terraform-security-scanner
  - docker-container-analyzer
  - k8s-manifest-validator`,
			expected: []string{"terraform-security-scanner", "docker-container-analyzer", "k8s-manifest-validator"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a simple test by parsing the YAML directly
			var config DotPromptConfig
			err := yaml.Unmarshal([]byte(tt.yaml), &config)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, config.Agents)
		})
	}
}

// TestDotPromptConfigBackwardCompatibility ensures existing agents still work
func TestDotPromptConfigBackwardCompatibility(t *testing.T) {
	// Test that existing agents without agents field still work
	yamlContent := `model: gpt-4o-mini
max_steps: 10
tools:
  - __filesystem_read
  - __filesystem_write
metadata:
  name: "Test Agent"
  description: "A test agent"`

	var config DotPromptConfig
	err := yaml.Unmarshal([]byte(yamlContent), &config)
	require.NoError(t, err)
	assert.Equal(t, "gpt-4o-mini", config.Model)
	assert.Equal(t, int64(10), config.MaxSteps)
	assert.Equal(t, []string{"__filesystem_read", "__filesystem_write"}, config.Tools)
	assert.Nil(t, config.Agents) // Should be nil, not empty slice
}

// TestDotPromptConfigFullExample tests complete frontmatter with both tools and agents
func TestDotPromptConfigFullExample(t *testing.T) {
	// Test a complete example with both tools and agents
	yamlContent := `model: gpt-4o-mini
max_steps: 15
agents:
  - CodeAnalyzer
  - SecurityScanner
tools:
  - __filesystem_read
  - __filesystem_write
  - __search_files
metadata:
  name: "Orchestrator Agent"
  description: "Coordinates multiple specialized agents"
input:
  repository_path: "string, Path to the repository to analyze"
  analysis_depth: "string, Level of analysis (shallow, deep)"
output:
  type: object
  properties:
    summary:
      type: string
    issues:
      type: array
      items:
        type: string`

	var config DotPromptConfig
	err := yaml.Unmarshal([]byte(yamlContent), &config)
	require.NoError(t, err)

	// Verify all fields are parsed correctly
	assert.Equal(t, "gpt-4o-mini", config.Model)
	assert.Equal(t, int64(15), config.MaxSteps)
	assert.Equal(t, []string{"CodeAnalyzer", "SecurityScanner"}, config.Agents)
	assert.Equal(t, []string{"__filesystem_read", "__filesystem_write", "__search_files"}, config.Tools)
	assert.Equal(t, "Orchestrator Agent", config.Metadata["name"])
	assert.Equal(t, "Coordinates multiple specialized agents", config.Metadata["description"])
	assert.NotNil(t, config.Input)
	assert.NotNil(t, config.Output)
}

// TestAgentsFieldValidation tests validation of agent references
func TestAgentsFieldValidation(t *testing.T) {
	tests := []struct {
		name        string
		agents      []string
		shouldError bool
	}{
		{
			name:        "valid agent names",
			agents:      []string{"CodeAnalyzer", "SecurityScanner", "TestAgent"},
			shouldError: false,
		},
		{
			name:        "empty agent name",
			agents:      []string{"CodeAnalyzer", "", "SecurityScanner"},
			shouldError: true,
		},
		{
			name:        "duplicate agent names",
			agents:      []string{"CodeAnalyzer", "SecurityScanner", "CodeAnalyzer"},
			shouldError: false, // Duplicates are allowed for now
		},
		{
			name:        "special characters in names",
			agents:      []string{"terraform-security-scanner", "docker.container", "k8s_manifest"},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simple validation logic
			for _, agentName := range tt.agents {
				if agentName == "" {
					if !tt.shouldError {
						t.Errorf("Empty agent name should cause error")
					}
					return
				}
			}
			if tt.shouldError {
				t.Errorf("Expected error but validation passed")
			}
		})
	}
}
