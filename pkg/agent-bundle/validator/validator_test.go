package validator

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agent_bundle "station/pkg/agent-bundle"
)

func TestValidator_Validate(t *testing.T) {
	tests := []struct {
		name        string
		setupBundle func(t *testing.T, fs afero.Fs, path string)
		wantValid   bool
		wantErrors  int
		wantWarnings int
		checkResult func(t *testing.T, result *agent_bundle.ValidationResult)
	}{
		{
			name: "valid_complete_bundle",
			setupBundle: func(t *testing.T, fs afero.Fs, path string) {
				createValidBundle(t, fs, path)
			},
			wantValid:    true,
			wantErrors:   0,
			wantWarnings: 0,
			checkResult: func(t *testing.T, result *agent_bundle.ValidationResult) {
				assert.True(t, result.ManifestValid)
				assert.True(t, result.AgentConfigValid)
				assert.True(t, result.ToolsValid)
				assert.True(t, result.DependenciesValid)
				assert.True(t, result.VariablesValid)
				
				stats := result.Statistics
				assert.Equal(t, 3, stats.TotalVariables)
				assert.Equal(t, 2, stats.RequiredVariables)
				assert.Equal(t, 1, stats.OptionalVariables)
				assert.Equal(t, 1, stats.MCPDependencies)
				assert.Equal(t, 1, stats.RequiredTools)
			},
		},
		{
			name: "missing_manifest_file",
			setupBundle: func(t *testing.T, fs afero.Fs, path string) {
				createValidBundle(t, fs, path)
				// Remove manifest
				fs.Remove(filepath.Join(path, "manifest.json"))
			},
			wantValid:   false,
			wantErrors:  2, // manifest error + agent config will also fail because we continue validation
			wantWarnings: 0,
			checkResult: func(t *testing.T, result *agent_bundle.ValidationResult) {
				assert.False(t, result.ManifestValid)
				assert.Contains(t, result.Errors[0].Message, "manifest.json not found")
				assert.Equal(t, "missing_file", result.Errors[0].Type)
			},
		},
		{
			name: "invalid_manifest_json",
			setupBundle: func(t *testing.T, fs afero.Fs, path string) {
				createValidBundle(t, fs, path)
				// Write invalid JSON
				manifestPath := filepath.Join(path, "manifest.json")
				afero.WriteFile(fs, manifestPath, []byte("{ invalid json }"), 0644)
			},
			wantValid:   false,
			wantErrors:  2, // manifest error + validation continues
			wantWarnings: 0,
			checkResult: func(t *testing.T, result *agent_bundle.ValidationResult) {
				assert.False(t, result.ManifestValid)
				assert.Contains(t, result.Errors[0].Message, "invalid JSON")
				assert.Equal(t, "invalid_json", result.Errors[0].Type)
			},
		},
		{
			name: "manifest_missing_required_fields",
			setupBundle: func(t *testing.T, fs afero.Fs, path string) {
				createValidBundle(t, fs, path)
				// Create manifest missing required fields
				incompleteManifest := map[string]interface{}{
					"name": "test-agent",
					// Missing version, description, author
				}
				manifestData, _ := json.MarshalIndent(incompleteManifest, "", "  ")
				manifestPath := filepath.Join(path, "manifest.json")
				afero.WriteFile(fs, manifestPath, manifestData, 0644)
			},
			wantValid:   false,
			wantErrors:  4, // Missing version, description, author, station_version
			wantWarnings: 0,
			checkResult: func(t *testing.T, result *agent_bundle.ValidationResult) {
				assert.False(t, result.ManifestValid)
				errorMessages := make([]string, len(result.Errors))
				for i, err := range result.Errors {
					errorMessages[i] = err.Message
				}
				// Find the specific error messages (order may vary)
				hasVersionError := false
				hasDescriptionError := false
				hasAuthorError := false
				for _, msg := range errorMessages {
					if contains(msg, "version") {
						hasVersionError = true
					}
					if contains(msg, "description") {
						hasDescriptionError = true
					}
					if contains(msg, "author") {
						hasAuthorError = true
					}
				}
				assert.True(t, hasVersionError, "Should have version error")
				assert.True(t, hasDescriptionError, "Should have description error") 
				assert.True(t, hasAuthorError, "Should have author error")
			},
		},
		{
			name: "agent_config_without_mcp_servers",
			setupBundle: func(t *testing.T, fs afero.Fs, path string) {
				createValidBundle(t, fs, path)
				// Create agent config without proper template structure
				agentConfig := map[string]interface{}{
					"name":        "test-agent",
					"description": "test",
					"prompt":      "simple prompt without templates",
					"max_steps":   5,
				}
				agentData, _ := json.MarshalIndent(agentConfig, "", "  ")
				agentPath := filepath.Join(path, "agent.json")
				afero.WriteFile(fs, agentPath, agentData, 0644)
			},
			wantValid:   true, // This should be valid, just with warnings
			wantErrors:  0,
			wantWarnings: 1, // Warning about no template variables
			checkResult: func(t *testing.T, result *agent_bundle.ValidationResult) {
				assert.True(t, result.AgentConfigValid) // Still valid JSON
				assert.Contains(t, result.Warnings[0].Message, "template variables")
			},
		},
		{
			name: "invalid_variables_schema",
			setupBundle: func(t *testing.T, fs afero.Fs, path string) {
				createValidBundle(t, fs, path)
				// Write invalid variables schema
				invalidSchema := map[string]interface{}{
					"INVALID_VAR": map[string]interface{}{
						"type":        "invalid_type",
						"description": "Invalid variable type",
					},
				}
				schemaData, _ := json.MarshalIndent(invalidSchema, "", "  ")
				schemaPath := filepath.Join(path, "variables.schema.json")
				afero.WriteFile(fs, schemaPath, schemaData, 0644)
			},
			wantValid:   false,
			wantErrors:  2, // One from validateVariablesFile + other validation continues
			wantWarnings: 0,
			checkResult: func(t *testing.T, result *agent_bundle.ValidationResult) {
				assert.False(t, result.VariablesValid)
				assert.Contains(t, result.Errors[0].Message, "invalid variable type")
			},
		},
		{
			name: "template_variable_not_in_schema",
			setupBundle: func(t *testing.T, fs afero.Fs, path string) {
				createValidBundle(t, fs, path)
				// Add template variable that's not in schema
				agentConfig := agent_bundle.AgentTemplateConfig{
					Name:           "test-agent",
					Description:    "Agent for {{ .MISSING_VAR }}",
					Prompt:         "You are helping {{ .CLIENT_NAME }} with {{ .UNDEFINED_VAR }}",
					MaxSteps:       5,
					NameTemplate:   "{{ .CLIENT_NAME }}-agent",
					PromptTemplate: "Working for {{ .CLIENT_NAME }}",
				}
				agentData, _ := json.MarshalIndent(agentConfig, "", "  ")
				agentPath := filepath.Join(path, "agent.json")
				afero.WriteFile(fs, agentPath, agentData, 0644)
			},
			wantValid:   false,
			wantErrors:  2, // MISSING_VAR and UNDEFINED_VAR not in schema
			wantWarnings: 0,
			checkResult: func(t *testing.T, result *agent_bundle.ValidationResult) {
				assert.False(t, result.VariablesValid)
				errorMessages := make([]string, len(result.Errors))
				for i, err := range result.Errors {
					errorMessages[i] = err.Message
				}
				// Should detect both missing variables
				foundMissing := false
				foundUndefined := false
				for _, msg := range errorMessages {
					if contains(msg, "MISSING_VAR") {
						foundMissing = true
					}
					if contains(msg, "UNDEFINED_VAR") {
						foundUndefined = true
					}
				}
				assert.True(t, foundMissing, "Should detect MISSING_VAR")
				assert.True(t, foundUndefined, "Should detect UNDEFINED_VAR")
			},
		},
		{
			name: "missing_examples_directory",
			setupBundle: func(t *testing.T, fs afero.Fs, path string) {
				createValidBundle(t, fs, path)
				// Remove examples directory
				fs.RemoveAll(filepath.Join(path, "examples"))
			},
			wantValid:   true, // Still valid, just a warning
			wantErrors:  0,
			wantWarnings: 1,
			checkResult: func(t *testing.T, result *agent_bundle.ValidationResult) {
				assert.Contains(t, result.Warnings[0].Message, "examples directory")
				assert.Equal(t, "missing_examples", result.Warnings[0].Type)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create filesystem and bundle
			fs := afero.NewMemMapFs()
			bundlePath := "/test-bundle"
			
			// Setup bundle structure
			tt.setupBundle(t, fs, bundlePath)

			// Create validator and validate
			validator := New(fs)
			result, err := validator.Validate(bundlePath)
			
			require.NoError(t, err)
			require.NotNil(t, result)

			// Check basic result expectations
			assert.Equal(t, tt.wantValid, result.Valid, "Validation result should match expected validity")
			assert.Equal(t, tt.wantErrors, len(result.Errors), "Error count should match")
			assert.Equal(t, tt.wantWarnings, len(result.Warnings), "Warning count should match")

			// Run custom checks if provided
			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

func TestValidator_ValidateManifest(t *testing.T) {
	tests := []struct {
		name     string
		manifest *agent_bundle.AgentBundleManifest
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid_manifest",
			manifest: &agent_bundle.AgentBundleManifest{
				Name:           "valid-agent",
				Version:        "1.0.0",
				Description:    "A valid agent",
				Author:         "Test Author",
				AgentType:      "task",
				StationVersion: ">=0.1.0",
				CreatedAt:      time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing_name",
			manifest: &agent_bundle.AgentBundleManifest{
				Version:        "1.0.0",
				Description:    "Missing name",
				Author:         "Test Author",
				AgentType:      "task",
				StationVersion: ">=0.1.0",
				CreatedAt:      time.Now(),
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "invalid_version",
			manifest: &agent_bundle.AgentBundleManifest{
				Name:           "test-agent",
				Version:        "invalid-version",
				Description:    "Invalid version",
				Author:         "Test Author",
				AgentType:      "task",
				StationVersion: ">=0.1.0",
				CreatedAt:      time.Now(),
			},
			wantErr: true,
			errMsg:  "invalid semantic version",
		},
		{
			name: "invalid_agent_type",
			manifest: &agent_bundle.AgentBundleManifest{
				Name:           "test-agent",
				Version:        "1.0.0",
				Description:    "Invalid agent type",
				Author:         "Test Author",
				AgentType:      "invalid_type",
				StationVersion: ">=0.1.0",
				CreatedAt:      time.Now(),
			},
			wantErr: true,
			errMsg:  "invalid agent type",
		},
	}

	validator := New(afero.NewMemMapFs())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateManifest(tt.manifest)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateAgentConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *agent_bundle.AgentTemplateConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid_config",
			config: &agent_bundle.AgentTemplateConfig{
				Name:           "test-agent",
				Description:    "A valid agent config",
				Prompt:         "You are a helpful agent for {{ .CLIENT_NAME }}",
				MaxSteps:       10,
				NameTemplate:   "{{ .CLIENT_NAME }}-agent",
				PromptTemplate: "Working for {{ .CLIENT_NAME }}",
				Version:        "1.0.0",
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing_required_fields",
			config: &agent_bundle.AgentTemplateConfig{
				Name: "test-agent",
				// Missing description, prompt, etc.
			},
			wantErr: true,
			errMsg:  "description is required",
		},
		{
			name: "invalid_max_steps",
			config: &agent_bundle.AgentTemplateConfig{
				Name:        "test-agent",
				Description: "Test agent",
				Prompt:      "Test prompt",
				MaxSteps:    -1, // Invalid negative value
			},
			wantErr: true,
			errMsg:  "max_steps must be positive",
		},
		{
			name: "invalid_template_syntax",
			config: &agent_bundle.AgentTemplateConfig{
				Name:           "test-agent",
				Description:    "Test agent",
				Prompt:         "Invalid template {{ .UNCLOSED",
				MaxSteps:       5,
				NameTemplate:   "{{ .CLIENT_NAME }}-agent",
				PromptTemplate: "Working for {{ .CLIENT_NAME }}",
			},
			wantErr: true,
			errMsg:  "invalid template syntax",
		},
	}

	validator := New(afero.NewMemMapFs())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateAgentConfig(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateToolMappings(t *testing.T) {
	tests := []struct {
		name       string
		tools      []agent_bundle.ToolRequirement
		mcpBundles []agent_bundle.MCPBundleDependency
		wantErr    bool
		errMsg     string
	}{
		{
			name: "valid_tool_mappings",
			tools: []agent_bundle.ToolRequirement{
				{
					Name:       "list_files",
					ServerName: "filesystem",
					MCPBundle:  "filesystem-tools",
					Required:   true,
				},
			},
			mcpBundles: []agent_bundle.MCPBundleDependency{
				{
					Name:     "filesystem-tools",
					Version:  ">=1.0.0",
					Required: true,
				},
			},
			wantErr: false,
		},
		{
			name: "tool_references_missing_bundle",
			tools: []agent_bundle.ToolRequirement{
				{
					Name:       "s3_list",
					ServerName: "aws-s3",
					MCPBundle:  "aws-tools", // Not in bundles list
					Required:   true,
				},
			},
			mcpBundles: []agent_bundle.MCPBundleDependency{
				{
					Name:     "filesystem-tools",
					Version:  ">=1.0.0",
					Required: true,
				},
			},
			wantErr: true,
			errMsg:  "references undefined MCP bundle",
		},
		{
			name: "required_tool_from_optional_bundle",
			tools: []agent_bundle.ToolRequirement{
				{
					Name:       "optional_tool",
					ServerName: "optional-server",
					MCPBundle:  "optional-bundle",
					Required:   true, // Required tool
				},
			},
			mcpBundles: []agent_bundle.MCPBundleDependency{
				{
					Name:     "optional-bundle",
					Version:  ">=1.0.0",
					Required: false, // Optional bundle
				},
			},
			wantErr: true,
			errMsg:  "required tool 'optional_tool' from optional MCP bundle",
		},
	}

	validator := New(afero.NewMemMapFs())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateToolMappings(tt.tools, tt.mcpBundles)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateVariables(t *testing.T) {
	variables := map[string]agent_bundle.VariableSpec{
		"CLIENT_NAME": {
			Type:        "string",
			Description: "Client name",
			Required:    true,
		},
		"API_KEY": {
			Type:        "string",
			Description: "API key",
			Required:    true,
			Sensitive:   true,
		},
		"TIMEOUT": {
			Type:        "number",
			Description: "Timeout value",
			Required:    false,
			Default:     30,
		},
	}

	templates := []string{
		"You are working for {{ .CLIENT_NAME }}",
		"Use API key: {{ .API_KEY }}",
		"Timeout: {{ .TIMEOUT }}",
		"Missing: {{ .UNDEFINED_VAR }}", // This should cause an error
	}

	validator := New(afero.NewMemMapFs())
	err := validator.ValidateVariables(variables, templates)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UNDEFINED_VAR")
	assert.Contains(t, err.Error(), "not defined in schema")
}

// Helper functions

func createValidBundle(t *testing.T, fs afero.Fs, path string) {
	// Create directory
	require.NoError(t, fs.MkdirAll(path, 0755))

	// Create valid manifest.json
	manifest := agent_bundle.AgentBundleManifest{
		Name:           "test-agent",
		Version:        "1.0.0",
		Description:    "A test agent bundle",
		Author:         "Test Author",
		AgentType:      "task",
		StationVersion: ">=0.1.0",
		CreatedAt:      time.Now(),
		MCPBundles: []agent_bundle.MCPBundleDependency{
			{
				Name:        "filesystem-tools",
				Version:     ">=1.0.0",
				Source:      "registry",
				Required:    true,
				Description: "Basic filesystem operations",
			},
		},
		RequiredTools: []agent_bundle.ToolRequirement{
			{
				Name:       "list_files",
				ServerName: "filesystem",
				MCPBundle:  "filesystem-tools",
				Required:   true,
			},
		},
		RequiredVariables: map[string]agent_bundle.VariableSpec{
			"CLIENT_NAME": {
				Type:        "string",
				Description: "Name of the client",
				Required:    true,
			},
			"API_KEY": {
				Type:        "string",
				Description: "API key for service",
				Required:    true,
				Sensitive:   true,
			},
			"TIMEOUT": {
				Type:        "number",
				Description: "Request timeout",
				Required:    false,
				Default:     30,
			},
		},
	}
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	manifestPath := filepath.Join(path, "manifest.json")
	require.NoError(t, afero.WriteFile(fs, manifestPath, manifestData, 0644))

	// Create valid agent.json
	agentConfig := agent_bundle.AgentTemplateConfig{
		Name:           "test-agent",
		Description:    "Agent for {{ .CLIENT_NAME }}",
		Prompt:         "You are helping {{ .CLIENT_NAME }}",
		MaxSteps:       10,
		NameTemplate:   "{{ .CLIENT_NAME }}-agent",
		PromptTemplate: "Working for {{ .CLIENT_NAME }}",
		Version:        "1.0.0",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	agentData, _ := json.MarshalIndent(agentConfig, "", "  ")
	agentPath := filepath.Join(path, "agent.json")
	require.NoError(t, afero.WriteFile(fs, agentPath, agentData, 0644))

	// Create valid tools.json
	toolsConfig := map[string]interface{}{
		"required_tools": manifest.RequiredTools,
		"mcp_bundles":    manifest.MCPBundles,
	}
	toolsData, _ := json.MarshalIndent(toolsConfig, "", "  ")
	toolsPath := filepath.Join(path, "tools.json")
	require.NoError(t, afero.WriteFile(fs, toolsPath, toolsData, 0644))

	// Create valid variables.schema.json
	variablesData, _ := json.MarshalIndent(manifest.RequiredVariables, "", "  ")
	variablesPath := filepath.Join(path, "variables.schema.json")
	require.NoError(t, afero.WriteFile(fs, variablesPath, variablesData, 0644))

	// Create README.md
	readmePath := filepath.Join(path, "README.md")
	require.NoError(t, afero.WriteFile(fs, readmePath, []byte("# Test Agent\n\nA test agent bundle."), 0644))

	// Create examples directory
	examplesPath := filepath.Join(path, "examples")
	require.NoError(t, fs.MkdirAll(examplesPath, 0755))
	
	devExamplePath := filepath.Join(examplesPath, "development.yml")
	require.NoError(t, afero.WriteFile(fs, devExamplePath, []byte("CLIENT_NAME: \"Test Client\"\nAPI_KEY: \"test-key\""), 0644))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		 indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}