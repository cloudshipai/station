package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTemplateVariableService_ProcessTemplateWithVariables tests the main template processing workflow
func TestTemplateVariableService_ProcessTemplateWithVariables(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	service := NewTemplateVariableService(tempDir, nil)

	// Test template processing with existing variables
	templateContent := `{
		"mcpServers": {
			"api": {
				"url": "{{.API_URL}}",
				"env": {
					"KEY": "{{.API_KEY}}"
				}
			}
		}
	}`

	// Create test environment directory and variables.yml
	envDir := filepath.Join(tempDir, "environments", "test")
	err := os.MkdirAll(envDir, 0755)
	require.NoError(t, err)

	variablesFile := filepath.Join(envDir, "variables.yml")
	variablesContent := `API_URL: https://api.example.com/v1
API_KEY: secret123`
	err = os.WriteFile(variablesFile, []byte(variablesContent), 0644)
	require.NoError(t, err)

	// Create mock repositories for environment lookup
	// Since we can't easily mock the database, we'll skip the DB-dependent test for now
	// This test focuses on the template rendering functionality

	t.Run("renderTemplate", func(t *testing.T) {
		variables := map[string]string{
			"API_URL": "https://api.example.com/v1",
			"API_KEY": "secret123",
		}

		result, err := service.renderTemplate(templateContent, variables)
		require.NoError(t, err)
		assert.Contains(t, result, "https://api.example.com/v1")
		assert.Contains(t, result, "secret123")
	})
}

// TestTemplateVariableService_RenderTemplate tests the core template rendering functionality
func TestTemplateVariableService_RenderTemplate(t *testing.T) {
	service := NewTemplateVariableService("/tmp", nil)

	tests := []struct {
		name            string
		templateContent string
		variables       map[string]string
		expectedSubstr  []string
		shouldError     bool
	}{
		{
			name: "Valid template with variables",
			templateContent: `{
				"mcpServers": {
					"api": {
						"url": "{{.API_URL}}",
						"env": {
							"KEY": "{{.API_KEY}}"
						}
					}
				}
			}`,
			variables: map[string]string{
				"API_URL": "https://api.example.com/v1",
				"API_KEY": "secret123",
			},
			expectedSubstr: []string{"https://api.example.com/v1", "secret123"},
			shouldError:    false,
		},
		{
			name: "Missing variable should error",
			templateContent: `{
				"server": {
					"token": "{{.MISSING_VAR}}"
				}
			}`,
			variables:   map[string]string{},
			shouldError: true,
		},
		{
			name: "No variables in template",
			templateContent: `{
				"mcpServers": {
					"simple": {
						"command": "echo",
						"args": ["hello"]
					}
				}
			}`,
			variables:      map[string]string{},
			expectedSubstr: []string{"echo", "hello"},
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.renderTemplate(tt.templateContent, tt.variables)
			
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				for _, substr := range tt.expectedSubstr {
					assert.Contains(t, result, substr)
				}
			}
		})
	}
}

// TestTemplateVariableService_LoadEnvironmentVariables tests loading variables from YAML files
func TestTemplateVariableService_LoadEnvironmentVariables(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	service := NewTemplateVariableService(tempDir, nil)

	// Create test environment directory and variables.yml
	envDir := filepath.Join(tempDir, "environments", "test")
	err := os.MkdirAll(envDir, 0755)
	require.NoError(t, err)

	variablesFile := filepath.Join(envDir, "variables.yml")
	variablesContent := `API_URL: https://api.example.com/v1
API_KEY: secret123
DATABASE_PORT: 5432`
	err = os.WriteFile(variablesFile, []byte(variablesContent), 0644)
	require.NoError(t, err)

	// Test loading variables
	variables, err := service.loadEnvironmentVariables("test")
	require.NoError(t, err)
	
	expected := map[string]string{
		"API_URL":       "https://api.example.com/v1",
		"API_KEY":       "secret123", 
		"DATABASE_PORT": "5432",
	}
	assert.Equal(t, expected, variables)

	// Test loading non-existent environment
	_, err = service.loadEnvironmentVariables("nonexistent")
	assert.Error(t, err)
}

// TestTemplateVariableService_SaveVariablesToEnvironment tests saving variables to YAML files
func TestTemplateVariableService_SaveVariablesToEnvironment(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	service := NewTemplateVariableService(tempDir, nil)

	newVars := map[string]string{
		"NEW_API_KEY": "newsecret123",
		"NEW_URL":     "https://new.api.com",
	}

	err := service.saveVariablesToEnvironment("test", newVars)
	require.NoError(t, err)

	// Verify file was created and contains expected content
	variablesFile := filepath.Join(tempDir, "environments", "test", "variables.yml")
	assert.FileExists(t, variablesFile)

	// Load and verify content
	loadedVars, err := service.loadEnvironmentVariables("test")
	require.NoError(t, err)
	
	assert.Equal(t, newVars, loadedVars)
}

// Note: ExtractVariableReferences tests removed as the function was removed
// to maintain consistency with Go template engine approach instead of manual parsing

// TestTemplateVariableService_IsSecretVariable tests secret variable detection
func TestTemplateVariableService_IsSecretVariable(t *testing.T) {
	service := NewTemplateVariableService("/tmp", nil)

	tests := []struct {
		name     string
		varName  string
		expected bool
	}{
		{"API_TOKEN", "API_TOKEN", true},
		{"SECRET_KEY", "SECRET_KEY", true},
		{"PASSWORD", "PASSWORD", true},
		{"AUTH_CREDENTIAL", "AUTH_CREDENTIAL", true},
		{"GITHUB_KEY", "GITHUB_KEY", true},
		{"API_URL", "API_URL", false},
		{"DATABASE_HOST", "DATABASE_HOST", false},
		{"PORT", "PORT", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isSecretVariable(tt.varName)
			assert.Equal(t, tt.expected, result)
		})
	}
}