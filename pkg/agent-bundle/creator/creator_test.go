package creator

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

func TestCreator_Create(t *testing.T) {
	tests := []struct {
		name    string
		opts    agent_bundle.CreateOptions
		wantErr bool
		check   func(t *testing.T, fs afero.Fs, path string)
	}{
		{
			name: "minimal_bundle_creation",
			opts: agent_bundle.CreateOptions{
				Name:        "test-agent",
				Author:      "Test Author",
				Description: "A test agent bundle",
				AgentType:   "task",
			},
			wantErr: false,
			check: func(t *testing.T, fs afero.Fs, path string) {
				// Check manifest.json exists and is valid
				manifestPath := filepath.Join(path, "manifest.json")
				exists, err := afero.Exists(fs, manifestPath)
				require.NoError(t, err)
				assert.True(t, exists)

				// Read and validate manifest
				manifestData, err := afero.ReadFile(fs, manifestPath)
				require.NoError(t, err)

				var manifest agent_bundle.AgentBundleManifest
				err = json.Unmarshal(manifestData, &manifest)
				require.NoError(t, err)

				assert.Equal(t, "test-agent", manifest.Name)
				assert.Equal(t, "Test Author", manifest.Author)
				assert.Equal(t, "A test agent bundle", manifest.Description)
				assert.Equal(t, "task", manifest.AgentType)
				assert.Equal(t, "1.0.0", manifest.Version)
				assert.NotZero(t, manifest.CreatedAt)

				// Check agent.json exists
				agentPath := filepath.Join(path, "agent.json")
				exists, err = afero.Exists(fs, agentPath)
				require.NoError(t, err)
				assert.True(t, exists)

				// Check tools.json exists
				toolsPath := filepath.Join(path, "tools.json")
				exists, err = afero.Exists(fs, toolsPath)
				require.NoError(t, err)
				assert.True(t, exists)

				// Check variables.schema.json exists
				variablesPath := filepath.Join(path, "variables.schema.json")
				exists, err = afero.Exists(fs, variablesPath)
				require.NoError(t, err)
				assert.True(t, exists)

				// Check README.md exists
				readmePath := filepath.Join(path, "README.md")
				exists, err = afero.Exists(fs, readmePath)
				require.NoError(t, err)
				assert.True(t, exists)

				// Check examples directory exists
				examplesPath := filepath.Join(path, "examples")
				exists, err = afero.Exists(fs, examplesPath)
				require.NoError(t, err)
				assert.True(t, exists)

				// Check that README contains agent name
				readmeData, err := afero.ReadFile(fs, readmePath)
				require.NoError(t, err)
				assert.Contains(t, string(readmeData), "test-agent")
			},
		},
		{
			name: "bundle_with_all_options",
			opts: agent_bundle.CreateOptions{
				Name:        "advanced-agent",
				Author:      "Advanced Author",
				Description: "An advanced agent bundle with all features",
				AgentType:   "scheduled",
				Tags:        []string{"aws", "monitoring", "production"},
				Variables: map[string]interface{}{
					"DEFAULT_REGION": "us-east-1",
					"MAX_RETRIES":    3,
					"TIMEOUT":        30,
				},
			},
			wantErr: false,
			check: func(t *testing.T, fs afero.Fs, path string) {
				// Validate manifest has all options
				manifestPath := filepath.Join(path, "manifest.json")
				manifestData, err := afero.ReadFile(fs, manifestPath)
				require.NoError(t, err)

				var manifest agent_bundle.AgentBundleManifest
				err = json.Unmarshal(manifestData, &manifest)
				require.NoError(t, err)

				assert.Equal(t, "advanced-agent", manifest.Name)
				assert.Equal(t, "scheduled", manifest.AgentType)
				assert.Equal(t, []string{"aws", "monitoring", "production"}, manifest.Tags)

				// Check that agent.json has template syntax
				agentPath := filepath.Join(path, "agent.json")
				agentData, err := afero.ReadFile(fs, agentPath)
				require.NoError(t, err)

				var agentConfig agent_bundle.AgentTemplateConfig
				err = json.Unmarshal(agentData, &agentConfig)
				require.NoError(t, err)

				// Should contain template variables
				assert.Contains(t, agentConfig.NameTemplate, "{{ .")
				assert.Contains(t, agentConfig.PromptTemplate, "{{ .")

				// Check variables.schema.json has example variables
				variablesPath := filepath.Join(path, "variables.schema.json")
				variablesData, err := afero.ReadFile(fs, variablesPath)
				require.NoError(t, err)

				var variables map[string]agent_bundle.VariableSpec
				err = json.Unmarshal(variablesData, &variables)
				require.NoError(t, err)

				// Should have at least example variables
				assert.NotEmpty(t, variables)
				_, hasExample := variables["EXAMPLE_VAR"]
				assert.True(t, hasExample)
			},
		},
		{
			name: "missing_required_name",
			opts: agent_bundle.CreateOptions{
				Author:      "Test Author",
				Description: "Missing name",
			},
			wantErr: true,
		},
		{
			name: "missing_required_author",
			opts: agent_bundle.CreateOptions{
				Name:        "test-agent",
				Description: "Missing author",
			},
			wantErr: true,
		},
		{
			name: "directory_already_exists",
			opts: agent_bundle.CreateOptions{
				Name:        "existing-agent",
				Author:      "Test Author",
				Description: "Test existing directory",
			},
			wantErr: true,
			check: func(t *testing.T, fs afero.Fs, path string) {
				// Pre-create the directory
				err := fs.MkdirAll(path, 0755)
				require.NoError(t, err)
				
				// Create a file in it
				testFile := filepath.Join(path, "existing.txt")
				err = afero.WriteFile(fs, testFile, []byte("existing"), 0644)
				require.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create in-memory filesystem for testing
			fs := afero.NewMemMapFs()
			bundlePath := "/test-bundles/" + tt.name

			// Run pre-check if provided
			if tt.check != nil && tt.wantErr {
				tt.check(t, fs, bundlePath)
			}

			// Create the creator
			creator := New(fs, nil)

			// Execute creation
			err := creator.Create(bundlePath, tt.opts)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Run validation checks if provided
				if tt.check != nil {
					tt.check(t, fs, bundlePath)
				}
			}
		})
	}
}

func TestCreator_GenerateScaffolding(t *testing.T) {
	fs := afero.NewMemMapFs()
	creator := New(fs, nil)
	bundlePath := "/test-bundle"

	manifest := &agent_bundle.AgentBundleManifest{
		Name:        "test-agent",
		Version:     "1.0.0",
		Description: "A test agent for scaffolding",
		Author:      "Test Author",
		AgentType:   "task",
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
				Pattern:     "^[A-Za-z0-9]{32}$",
			},
		},
		CreatedAt: time.Now(),
	}

	err := creator.GenerateScaffolding(bundlePath, manifest)
	require.NoError(t, err)

	// Validate all expected files exist
	expectedFiles := []string{
		"manifest.json",
		"agent.json",
		"tools.json",
		"variables.schema.json",
		"README.md",
		"examples/development.yml",
		"examples/production.yml",
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(bundlePath, file)
		exists, err := afero.Exists(fs, filePath)
		require.NoError(t, err, "File should exist: %s", file)
		assert.True(t, exists, "File should exist: %s", file)
	}

	// Validate manifest.json content
	manifestPath := filepath.Join(bundlePath, "manifest.json")
	manifestData, err := afero.ReadFile(fs, manifestPath)
	require.NoError(t, err)

	var savedManifest agent_bundle.AgentBundleManifest
	err = json.Unmarshal(manifestData, &savedManifest)
	require.NoError(t, err)

	assert.Equal(t, manifest.Name, savedManifest.Name)
	assert.Equal(t, manifest.Version, savedManifest.Version)
	assert.Equal(t, manifest.Description, savedManifest.Description)
	assert.Equal(t, manifest.Author, savedManifest.Author)
	assert.Equal(t, manifest.AgentType, savedManifest.AgentType)

	// Validate variables.schema.json has the required variables
	variablesPath := filepath.Join(bundlePath, "variables.schema.json")
	variablesData, err := afero.ReadFile(fs, variablesPath)
	require.NoError(t, err)

	var variables map[string]agent_bundle.VariableSpec
	err = json.Unmarshal(variablesData, &variables)
	require.NoError(t, err)

	clientVar, exists := variables["CLIENT_NAME"]
	assert.True(t, exists)
	assert.Equal(t, "string", clientVar.Type)
	assert.True(t, clientVar.Required)

	apiVar, exists := variables["API_KEY"]
	assert.True(t, exists)
	assert.Equal(t, "string", apiVar.Type)
	assert.True(t, apiVar.Required)
	assert.True(t, apiVar.Sensitive)
	assert.Equal(t, "^[A-Za-z0-9]{32}$", apiVar.Pattern)

	// Validate agent.json has Go template syntax
	agentPath := filepath.Join(bundlePath, "agent.json")
	agentData, err := afero.ReadFile(fs, agentPath)
	require.NoError(t, err)

	var agentConfig agent_bundle.AgentTemplateConfig
	err = json.Unmarshal(agentData, &agentConfig)
	require.NoError(t, err)

	// Should use Go template syntax {{ .VAR }}
	assert.Contains(t, agentConfig.NameTemplate, "{{ .")
	assert.Contains(t, agentConfig.PromptTemplate, "{{ .")
	
	// Check example files contain variable placeholders
	devExamplePath := filepath.Join(bundlePath, "examples/development.yml")
	devData, err := afero.ReadFile(fs, devExamplePath)
	require.NoError(t, err)

	devContent := string(devData)
	assert.Contains(t, devContent, "CLIENT_NAME:")
	assert.Contains(t, devContent, "API_KEY:")
}

func TestCreator_ExportFromAgent(t *testing.T) {
	t.Skip("ExportFromAgent requires database integration - will implement after core creation works")
	
	// This test will be implemented when we have database integration
	// It should:
	// 1. Query agent from database
	// 2. Analyze tool dependencies
	// 3. Extract template variables
	// 4. Generate bundle with actual agent configuration
}

func TestCreator_AnalyzeDependencies(t *testing.T) {
	t.Skip("AnalyzeDependencies requires database integration - will implement after core creation works")
	
	// This test will be implemented when we have database integration
	// It should:
	// 1. Query agent tools from database
	// 2. Map tools to MCP servers
	// 3. Identify required MCP bundles
	// 4. Detect conflicts or missing dependencies
}

func TestCreator_CreateWithInvalidFileSystem(t *testing.T) {
	// Test error handling with read-only filesystem
	fs := afero.NewReadOnlyFs(afero.NewMemMapFs())
	creator := New(fs, nil)

	opts := agent_bundle.CreateOptions{
		Name:        "test-agent",
		Author:      "Test Author",
		Description: "Test readonly fs",
	}

	err := creator.Create("/test-bundle", opts)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operation not permitted")
}

func TestCreator_ValidateOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    agent_bundle.CreateOptions
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid_minimal_options",
			opts: agent_bundle.CreateOptions{
				Name:        "valid-agent",
				Author:      "Valid Author",
				Description: "Valid description",
			},
			wantErr: false,
		},
		{
			name: "empty_name",
			opts: agent_bundle.CreateOptions{
				Author:      "Author",
				Description: "Description",
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "empty_author",
			opts: agent_bundle.CreateOptions{
				Name:        "agent",
				Description: "Description",
			},
			wantErr: true,
			errMsg:  "author is required",
		},
		{
			name: "empty_description",
			opts: agent_bundle.CreateOptions{
				Name:   "agent",
				Author: "Author",
			},
			wantErr: true,
			errMsg:  "description is required",
		},
		{
			name: "invalid_agent_type",
			opts: agent_bundle.CreateOptions{
				Name:        "agent",
				Author:      "Author", 
				Description: "Description",
				AgentType:   "invalid_type",
			},
			wantErr: true,
			errMsg:  "invalid agent type",
		},
		{
			name: "valid_agent_types",
			opts: agent_bundle.CreateOptions{
				Name:        "agent",
				Author:      "Author",
				Description: "Description",
				AgentType:   "scheduled", // Should be valid
			},
			wantErr: false,
		},
	}

	fs := afero.NewMemMapFs()
	creator := New(fs, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := creator.validateOptions(tt.opts)

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

// Helper function to check file contents
func checkFileContains(t *testing.T, fs afero.Fs, filePath, expectedContent string) {
	exists, err := afero.Exists(fs, filePath)
	require.NoError(t, err)
	require.True(t, exists, "File should exist: %s", filePath)

	data, err := afero.ReadFile(fs, filePath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, expectedContent, "File %s should contain: %s", filePath, expectedContent)
}

// Helper function to check JSON file structure
func checkJSONFile(t *testing.T, fs afero.Fs, filePath string, target interface{}) {
	exists, err := afero.Exists(fs, filePath)
	require.NoError(t, err)
	require.True(t, exists, "File should exist: %s", filePath)

	data, err := afero.ReadFile(fs, filePath)
	require.NoError(t, err)

	err = json.Unmarshal(data, target)
	require.NoError(t, err, "File should be valid JSON: %s", filePath)
}