package services

import (
	"testing"

	"station/internal/config"
	"station/pkg/dotprompt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodingToolFactory_NewCodingToolFactory(t *testing.T) {
	t.Run("DisabledWhenBackendEmpty", func(t *testing.T) {
		cfg := config.CodingConfig{
			Backend: "",
		}
		factory := NewCodingToolFactory(cfg)
		assert.False(t, factory.IsEnabled())
	})

	t.Run("DisabledWhenBackendNotOpencode", func(t *testing.T) {
		cfg := config.CodingConfig{
			Backend: "unknown",
		}
		factory := NewCodingToolFactory(cfg)
		assert.False(t, factory.IsEnabled())
	})

	t.Run("EnabledWhenBackendOpencode", func(t *testing.T) {
		cfg := config.CodingConfig{
			Backend: "opencode",
			OpenCode: config.CodingOpenCodeConfig{
				URL: "http://localhost:4096",
			},
		}
		factory := NewCodingToolFactory(cfg)
		assert.True(t, factory.IsEnabled())
		assert.NotNil(t, factory.GetBackend())
	})
}

func TestCodingToolFactory_ShouldAddTools(t *testing.T) {
	enabledFactory := NewCodingToolFactory(config.CodingConfig{
		Backend: "opencode",
		OpenCode: config.CodingOpenCodeConfig{
			URL: "http://localhost:4096",
		},
	})

	disabledFactory := NewCodingToolFactory(config.CodingConfig{
		Backend: "",
	})

	tests := []struct {
		name          string
		factory       *CodingToolFactory
		codingConfig  *dotprompt.CodingConfig
		expectAddTool bool
	}{
		{
			name:          "NilConfigNoTools",
			factory:       enabledFactory,
			codingConfig:  nil,
			expectAddTool: false,
		},
		{
			name:          "DisabledConfigNoTools",
			factory:       enabledFactory,
			codingConfig:  &dotprompt.CodingConfig{Enabled: false},
			expectAddTool: false,
		},
		{
			name:          "EnabledConfigWithEnabledFactory",
			factory:       enabledFactory,
			codingConfig:  &dotprompt.CodingConfig{Enabled: true},
			expectAddTool: true,
		},
		{
			name:          "EnabledConfigWithDisabledFactory",
			factory:       disabledFactory,
			codingConfig:  &dotprompt.CodingConfig{Enabled: true},
			expectAddTool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldAdd := tt.factory.ShouldAddTools(tt.codingConfig)
			assert.Equal(t, tt.expectAddTool, shouldAdd)
		})
	}
}

func TestCodingToolFactory_GetCodingTools(t *testing.T) {
	factory := NewCodingToolFactory(config.CodingConfig{
		Backend: "opencode",
		OpenCode: config.CodingOpenCodeConfig{
			URL: "http://localhost:4096",
		},
	})

	t.Run("ReturnsNilWhenConfigDisabled", func(t *testing.T) {
		tools := factory.GetCodingTools(&dotprompt.CodingConfig{Enabled: false})
		assert.Nil(t, tools)
	})

	t.Run("ReturnsFiveToolsWhenEnabled", func(t *testing.T) {
		tools := factory.GetCodingTools(&dotprompt.CodingConfig{Enabled: true})
		require.Len(t, tools, 5)

		toolNames := make(map[string]bool)
		for _, tool := range tools {
			toolNames[tool.Name()] = true
		}

		assert.True(t, toolNames["coding_open"], "Should have coding_open tool")
		assert.True(t, toolNames["code"], "Should have code tool")
		assert.True(t, toolNames["coding_close"], "Should have coding_close tool")
		assert.True(t, toolNames["coding_commit"], "Should have coding_commit tool")
		assert.True(t, toolNames["coding_push"], "Should have coding_push tool")
	})
}

func TestCodingToolFactory_ToolNames(t *testing.T) {
	factory := NewCodingToolFactory(config.CodingConfig{
		Backend: "opencode",
		OpenCode: config.CodingOpenCodeConfig{
			URL: "http://localhost:4096",
		},
	})

	tools := factory.GetCodingTools(&dotprompt.CodingConfig{Enabled: true})
	require.Len(t, tools, 5)

	expectedNames := []string{"coding_open", "code", "coding_close", "coding_commit", "coding_push"}
	for i, tool := range tools {
		assert.Equal(t, expectedNames[i], tool.Name())
	}
}
