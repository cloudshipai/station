package manager

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agent_bundle "station/pkg/agent-bundle"
)

func TestManager_Install(t *testing.T) {
	tests := []struct {
		name        string
		setupBundle func(t *testing.T, fs afero.Fs, bundlePath string)
		environment string
		variables   map[string]interface{}
		wantErr     bool
		checkResult func(t *testing.T, result *agent_bundle.InstallResult)
	}{
		{
			name: "successful_installation",
			setupBundle: func(t *testing.T, fs afero.Fs, bundlePath string) {
				createValidTestBundle(t, fs, bundlePath)
			},
			environment: "development",
			variables: map[string]interface{}{
				"CLIENT_NAME": "Test Client",
				"API_KEY":     "test-api-key-123",
				"TIMEOUT":     30,
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *agent_bundle.InstallResult) {
				assert.True(t, result.Success)
				assert.NotZero(t, result.AgentID)
				assert.Equal(t, "Test Client Agent", result.AgentName)
				assert.Equal(t, "development", result.Environment)
				assert.Greater(t, result.ToolsInstalled, 0)
				assert.Contains(t, result.MCPBundles, "filesystem-tools")
			},
		},
		{
			name: "missing_required_variables",
			setupBundle: func(t *testing.T, fs afero.Fs, bundlePath string) {
				createValidTestBundle(t, fs, bundlePath)
			},
			environment: "development",
			variables: map[string]interface{}{
				"CLIENT_NAME": "Test Client",
				// Missing required API_KEY
			},
			wantErr: true,
		},
		{
			name: "invalid_bundle_structure",
			setupBundle: func(t *testing.T, fs afero.Fs, bundlePath string) {
				// Create incomplete bundle
				fs.MkdirAll(bundlePath, 0755)
				// Missing manifest.json
			},
			environment: "development",
			variables: map[string]interface{}{
				"CLIENT_NAME": "Test Client",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test filesystem and bundle
			fs := afero.NewMemMapFs()
			bundlePath := "/test-bundle"
			tt.setupBundle(t, fs, bundlePath)

			// Create manager with mocked dependencies
			manager := &Manager{
				fs:        fs,
				validator: &mockValidator{},
				resolver:  &mockResolver{},
				// Note: agentService and mcpService would be injected in real implementation
			}

			// Execute installation
			result, err := manager.Install(bundlePath, tt.environment, tt.variables)

			if tt.wantErr {
				assert.Error(t, err)
				if result != nil {
					assert.False(t, result.Success)
				}
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				if tt.checkResult != nil {
					tt.checkResult(t, result)
				}
			}
		})
	}
}

func TestManager_Duplicate(t *testing.T) {
	tests := []struct {
		name      string
		agentID   int64
		targetEnv string
		opts      agent_bundle.DuplicateOptions
		wantErr   bool
		checkResult func(t *testing.T, result *agent_bundle.InstallResult)
	}{
		{
			name:      "successful_duplication",
			agentID:   1,
			targetEnv: "production",
			opts: agent_bundle.DuplicateOptions{
				Name: "Production Test Agent",
				Variables: map[string]interface{}{
					"CLIENT_NAME": "Production Client",
					"API_KEY":     "prod-api-key-456",
					"TIMEOUT":     60,
				},
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *agent_bundle.InstallResult) {
				assert.True(t, result.Success)
				assert.NotZero(t, result.AgentID)
				assert.Equal(t, "Production Test Agent", result.AgentName)
				assert.Equal(t, "production", result.Environment)
			},
		},
		{
			name:      "agent_not_found",
			agentID:   999,
			targetEnv: "production",
			opts:      agent_bundle.DuplicateOptions{},
			wantErr:   true,
		},
		{
			name:      "invalid_target_environment",
			agentID:   1,
			targetEnv: "",
			opts:      agent_bundle.DuplicateOptions{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create manager with mocked dependencies
			manager := &Manager{
				fs:        afero.NewMemMapFs(),
				validator: &mockValidator{},
				resolver:  &mockResolver{},
				// Note: agentService would be injected in real implementation
			}

			// Execute duplication
			result, err := manager.Duplicate(tt.agentID, tt.targetEnv, tt.opts)

			if tt.wantErr {
				assert.Error(t, err)
				if result != nil {
					assert.False(t, result.Success)
				}
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				if tt.checkResult != nil {
					tt.checkResult(t, result)
				}
			}
		})
	}
}

func TestManager_RenderTemplate(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		variables map[string]interface{}
		expected  string
		wantErr   bool
	}{
		{
			name:     "simple_variable_substitution",
			template: "Hello {{ .CLIENT_NAME }}",
			variables: map[string]interface{}{
				"CLIENT_NAME": "Test Client",
			},
			expected: "Hello Test Client",
			wantErr:  false,
		},
		{
			name:     "multiple_variables",
			template: "Agent for {{ .CLIENT_NAME }} with timeout {{ .TIMEOUT }}",
			variables: map[string]interface{}{
				"CLIENT_NAME": "Acme Corp",
				"TIMEOUT":     30,
			},
			expected: "Agent for Acme Corp with timeout 30",
			wantErr:  false,
		},
		{
			name:     "missing_variable",
			template: "Hello {{ .MISSING_VAR }}",
			variables: map[string]interface{}{
				"CLIENT_NAME": "Test Client",
			},
			expected: "",
			wantErr:  true,
		},
		{
			name:     "invalid_template_syntax",
			template: "Hello {{ .CLIENT_NAME",
			variables: map[string]interface{}{
				"CLIENT_NAME": "Test Client",
			},
			expected: "",
			wantErr:  true,
		},
	}

	manager := &Manager{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := manager.renderTemplate(tt.template, tt.variables)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestManager_ValidateVariables(t *testing.T) {
	schema := map[string]agent_bundle.VariableSpec{
		"CLIENT_NAME": {
			Type:     "string",
			Required: true,
		},
		"API_KEY": {
			Type:     "string",
			Required: true,
		},
		"TIMEOUT": {
			Type:     "number",
			Required: false,
			Default:  30,
		},
	}

	tests := []struct {
		name      string
		variables map[string]interface{}
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid_variables",
			variables: map[string]interface{}{
				"CLIENT_NAME": "Test Client",
				"API_KEY":     "test-key",
				"TIMEOUT":     60,
			},
			wantErr: false,
		},
		{
			name: "missing_required_variable",
			variables: map[string]interface{}{
				"CLIENT_NAME": "Test Client",
				// Missing required API_KEY
			},
			wantErr: true,
			errMsg:  "required variable 'API_KEY' not provided",
		},
		{
			name: "optional_variable_uses_default",
			variables: map[string]interface{}{
				"CLIENT_NAME": "Test Client",
				"API_KEY":     "test-key",
				// TIMEOUT will use default
			},
			wantErr: false,
		},
		{
			name: "invalid_variable_type",
			variables: map[string]interface{}{
				"CLIENT_NAME": "Test Client",
				"API_KEY":     "test-key",
				"TIMEOUT":     "not-a-number",
			},
			wantErr: true,
			errMsg:  "variable 'TIMEOUT' has invalid type",
		},
	}

	manager := &Manager{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validateVariables(tt.variables, schema)

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

func TestManager_ParseBundleReference(t *testing.T) {
	tests := []struct {
		name      string
		reference string
		expected  agent_bundle.BundleReference
		wantErr   bool
	}{
		{
			name:      "simple_name",
			reference: "my-agent",
			expected: agent_bundle.BundleReference{
				Name:     "my-agent",
				Version:  "latest",
				Registry: "default",
			},
			wantErr: false,
		},
		{
			name:      "name_with_version",
			reference: "my-agent@1.2.0",
			expected: agent_bundle.BundleReference{
				Name:     "my-agent",
				Version:  "1.2.0",
				Registry: "default",
			},
			wantErr: false,
		},
		{
			name:      "registry_with_name",
			reference: "company/my-agent",
			expected: agent_bundle.BundleReference{
				Name:     "my-agent",
				Version:  "latest",
				Registry: "company",
			},
			wantErr: false,
		},
		{
			name:      "full_reference",
			reference: "company/my-agent@1.2.0",
			expected: agent_bundle.BundleReference{
				Name:     "my-agent",
				Version:  "1.2.0",
				Registry: "company",
			},
			wantErr: false,
		},
	}

	manager := &Manager{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := manager.ParseBundleReference(tt.reference)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.Name, result.Name)
				assert.Equal(t, tt.expected.Version, result.Version)
				assert.Equal(t, tt.expected.Registry, result.Registry)
			}
		})
	}
}

func TestManager_GetStatus(t *testing.T) {
	t.Skip("GetStatus requires database integration - will implement after core functionality")
}

func TestManager_List(t *testing.T) {
	t.Skip("List requires database integration - will implement after core functionality") 
}

func TestManager_Remove(t *testing.T) {
	t.Skip("Remove requires database integration - will implement after core functionality")
}

func TestManager_Update(t *testing.T) {
	t.Skip("Update requires database integration - will implement after core functionality")
}

// Mock implementations for testing

type mockValidator struct{}

func (m *mockValidator) Validate(bundlePath string) (*agent_bundle.ValidationResult, error) {
	return &agent_bundle.ValidationResult{
		Valid:             true,
		ManifestValid:     true,
		AgentConfigValid:  true,
		ToolsValid:        true,
		DependenciesValid: true,
		VariablesValid:    true,
		Statistics: agent_bundle.ValidationStatistics{
			TotalVariables:    3,
			RequiredVariables: 2,
			MCPDependencies:   1,
			RequiredTools:     1,
		},
	}, nil
}

func (m *mockValidator) ValidateManifest(manifest *agent_bundle.AgentBundleManifest) error {
	return nil
}

func (m *mockValidator) ValidateAgentConfig(config *agent_bundle.AgentTemplateConfig) error {
	return nil
}

func (m *mockValidator) ValidateToolMappings(tools []agent_bundle.ToolRequirement, mcpBundles []agent_bundle.MCPBundleDependency) error {
	return nil
}

func (m *mockValidator) ValidateDependencies(dependencies []agent_bundle.MCPBundleDependency) error {
	return nil
}

func (m *mockValidator) ValidateVariables(variables map[string]agent_bundle.VariableSpec, templates []string) error {
	return nil
}

type mockResolver struct{}

func (m *mockResolver) Resolve(ctx context.Context, dependencies []agent_bundle.MCPBundleDependency, environment string) (*agent_bundle.ResolutionResult, error) {
	return &agent_bundle.ResolutionResult{
		Success: true,
		ResolvedBundles: []agent_bundle.MCPBundleRef{
			{
				Name:    "filesystem-tools",
				Version: "1.0.0",
				Source:  "registry",
			},
		},
		MissingBundles: []agent_bundle.MCPBundleDependency{},
		Conflicts:      []agent_bundle.ToolConflict{},
		InstallOrder:   []string{"filesystem-tools"},
	}, nil
}

func (m *mockResolver) InstallMCPBundles(ctx context.Context, bundles []agent_bundle.MCPBundleRef, environment string) error {
	return nil
}

func (m *mockResolver) ValidateToolAvailability(ctx context.Context, tools []agent_bundle.ToolRequirement, environment string) error {
	return nil
}

func (m *mockResolver) ResolveConflicts(conflicts []agent_bundle.ToolConflict) (*agent_bundle.ConflictResolution, error) {
	return &agent_bundle.ConflictResolution{
		Strategy:    "auto",
		Resolutions: make(map[string]string),
		Warnings:    []string{},
	}, nil
}

// Helper functions

func createValidTestBundle(t *testing.T, fs afero.Fs, bundlePath string) {
	require.NoError(t, fs.MkdirAll(bundlePath, 0755))

	// Create manifest.json
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
	writeJSONFile(t, fs, filepath.Join(bundlePath, "manifest.json"), manifest)

	// Create agent.json
	agentConfig := agent_bundle.AgentTemplateConfig{
		Name:           "{{ .CLIENT_NAME }} Agent",
		Description:    "Agent for {{ .CLIENT_NAME }}",
		Prompt:         "You are helping {{ .CLIENT_NAME }} with their tasks",
		MaxSteps:       10,
		NameTemplate:   "{{ .CLIENT_NAME }} Agent",
		PromptTemplate: "Working for {{ .CLIENT_NAME }}",
		Version:        "1.0.0",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	writeJSONFile(t, fs, filepath.Join(bundlePath, "agent.json"), agentConfig)

	// Create variables.schema.json
	writeJSONFile(t, fs, filepath.Join(bundlePath, "variables.schema.json"), manifest.RequiredVariables)

	// Create tools.json
	toolsConfig := map[string]interface{}{
		"required_tools": manifest.RequiredTools,
		"mcp_bundles":    manifest.MCPBundles,
	}
	writeJSONFile(t, fs, filepath.Join(bundlePath, "tools.json"), toolsConfig)
}

func writeJSONFile(t *testing.T, fs afero.Fs, path string, data interface{}) {
	content, err := json.MarshalIndent(data, "", "  ")
	require.NoError(t, err)
	require.NoError(t, afero.WriteFile(fs, path, content, 0644))
}