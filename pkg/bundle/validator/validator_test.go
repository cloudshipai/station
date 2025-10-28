package validator

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"station/pkg/bundle"
)

func TestValidator_Validate(t *testing.T) {
	tests := []struct {
		name         string
		setupBundle  func(fs afero.Fs, bundlePath string)
		wantValid    bool
		wantIssues   int
		wantWarnings int
		checkIssues  func(t *testing.T, result *bundle.ValidationResult)
	}{
		{
			name: "valid complete bundle",
			setupBundle: func(fs afero.Fs, bundlePath string) {
				createValidCompleteBundle(t, fs, bundlePath)
			},
			wantValid:    true,
			wantIssues:   0,
			wantWarnings: 0,
		},
		{
			name: "missing manifest file",
			setupBundle: func(fs afero.Fs, bundlePath string) {
				// Create bundle without manifest
				createFile(t, fs, filepath.Join(bundlePath, "template.json"), `{"mcpServers":{}}`)
				createFile(t, fs, filepath.Join(bundlePath, "variables.schema.json"), `{"type":"object","properties":{}}`)
			},
			wantValid:    false,
			wantIssues:   1,
			wantWarnings: 2, // missing README + missing examples
			checkIssues: func(t *testing.T, result *bundle.ValidationResult) {
				// Find the manifest issue among all issues
				found := false
				for _, issue := range result.Issues {
					if issue.Type == "missing_file" && issue.File == "manifest.json" {
						found = true
						break
					}
				}
				assert.True(t, found, "Should have issue for missing manifest.json")
			},
		},
		{
			name: "invalid manifest JSON",
			setupBundle: func(fs afero.Fs, bundlePath string) {
				createFile(t, fs, filepath.Join(bundlePath, "manifest.json"), `{"name": "test", "invalid": json}`)
				createFile(t, fs, filepath.Join(bundlePath, "template.json"), `{"mcpServers":{}}`)
				createFile(t, fs, filepath.Join(bundlePath, "variables.schema.json"), `{"type":"object","properties":{}}`)
			},
			wantValid:    false,
			wantIssues:   1,
			wantWarnings: 2, // missing README + missing examples
			checkIssues: func(t *testing.T, result *bundle.ValidationResult) {
				// Find the JSON error among all issues
				found := false
				for _, issue := range result.Issues {
					if issue.Type == "invalid_json" && issue.File == "manifest.json" {
						found = true
						break
					}
				}
				assert.True(t, found, "Should have issue for invalid JSON in manifest.json")
			},
		},
		{
			name: "manifest missing required fields",
			setupBundle: func(fs afero.Fs, bundlePath string) {
				manifest := bundle.BundleManifest{
					Name: "test-bundle",
					// Missing version, description, author, station_version
				}
				createJSONFile(t, fs, filepath.Join(bundlePath, "manifest.json"), manifest)
				createFile(t, fs, filepath.Join(bundlePath, "template.json"), `{"mcpServers":{}}`)
				createFile(t, fs, filepath.Join(bundlePath, "variables.schema.json"), `{"type":"object","properties":{}}`)
			},
			wantValid:    false,
			wantIssues:   4, // version, description, author, station_version
			wantWarnings: 2, // missing README + missing examples
			checkIssues: func(t *testing.T, result *bundle.ValidationResult) {
				requiredFields := []string{"version", "description", "author", "station_version"}
				foundFields := make(map[string]bool)
				for _, issue := range result.Issues {
					if issue.Type == "missing_required_field" {
						foundFields[issue.Field] = true
					}
				}
				for _, field := range requiredFields {
					assert.True(t, foundFields[field], "Should have issue for missing field: %s", field)
				}
			},
		},
		{
			name: "template without MCP servers",
			setupBundle: func(fs afero.Fs, bundlePath string) {
				createValidManifest(t, fs, bundlePath)
				createFile(t, fs, filepath.Join(bundlePath, "template.json"), `{"some_other_field": "value"}`)
				createFile(t, fs, filepath.Join(bundlePath, "variables.schema.json"), `{"type":"object","properties":{}}`)
			},
			wantValid:    false,
			wantIssues:   1,
			wantWarnings: 2, // missing README + missing examples
			checkIssues: func(t *testing.T, result *bundle.ValidationResult) {
				found := false
				for _, issue := range result.Issues {
					if issue.Type == "missing_mcp_servers" && issue.File == "template.json" {
						found = true
						break
					}
				}
				assert.True(t, found, "Should have issue for missing MCP servers")
			},
		},
		{
			name: "invalid variables schema",
			setupBundle: func(fs afero.Fs, bundlePath string) {
				createValidManifest(t, fs, bundlePath)
				createFile(t, fs, filepath.Join(bundlePath, "template.json"), `{"mcpServers":{}}`)
				createFile(t, fs, filepath.Join(bundlePath, "variables.schema.json"), `{"type":"string"}`) // Should be object
			},
			wantValid:    false,
			wantIssues:   1,
			wantWarnings: 3, // missing README + empty schema + missing examples
			checkIssues: func(t *testing.T, result *bundle.ValidationResult) {
				found := false
				for _, issue := range result.Issues {
					if issue.Type == "invalid_schema" && issue.File == "variables.schema.json" {
						found = true
						break
					}
				}
				assert.True(t, found, "Should have issue for invalid schema")
			},
		},
		{
			name: "template variable not in schema",
			setupBundle: func(fs afero.Fs, bundlePath string) {
				createValidManifest(t, fs, bundlePath)
				// Template uses {{ .API_KEY }} but schema doesn't define it
				createFile(t, fs, filepath.Join(bundlePath, "template.json"),
					`{"mcpServers":{"test":{"env":{"API_KEY":"{{ .API_KEY }}"}}}}`)
				createFile(t, fs, filepath.Join(bundlePath, "variables.schema.json"),
					`{"type":"object","properties":{"OTHER_VAR":{"type":"string"}}}`)
			},
			wantValid:    true, // This is just a warning
			wantIssues:   0,
			wantWarnings: 3, // missing README + undefined variable + missing examples
			checkIssues: func(t *testing.T, result *bundle.ValidationResult) {
				found := false
				for _, warning := range result.Warnings {
					if warning.Type == "undefined_variable" && strings.Contains(warning.Message, "API_KEY") {
						found = true
						break
					}
				}
				assert.True(t, found, "Should have warning for undefined variable API_KEY")
			},
		},
		{
			name: "missing examples directory",
			setupBundle: func(fs afero.Fs, bundlePath string) {
				createValidManifest(t, fs, bundlePath)
				createFile(t, fs, filepath.Join(bundlePath, "template.json"), `{"mcpServers":{}}`)
				createFile(t, fs, filepath.Join(bundlePath, "variables.schema.json"), `{"type":"object","properties":{}}`)
				// No examples directory, no README.md
			},
			wantValid:    true, // Just a warning
			wantIssues:   0,
			wantWarnings: 2, // missing README + missing examples
			checkIssues: func(t *testing.T, result *bundle.ValidationResult) {
				// Find the missing examples warning among all warnings
				found := false
				for _, warning := range result.Warnings {
					if warning.Type == "missing_examples" {
						found = true
						break
					}
				}
				assert.True(t, found, "Should have warning for missing examples directory")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			bundlePath := "/test-bundle"
			validator := NewValidator()

			// Setup test bundle
			_ = fs.MkdirAll(bundlePath, 0755)
			tt.setupBundle(fs, bundlePath)

			// Validate
			result, err := validator.Validate(fs, bundlePath)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Check results
			assert.Equal(t, tt.wantValid, result.Valid, "Expected validity does not match")
			assert.Len(t, result.Issues, tt.wantIssues, "Expected number of issues does not match")
			assert.Len(t, result.Warnings, tt.wantWarnings, "Expected number of warnings does not match")

			// Run custom checks
			if tt.checkIssues != nil {
				tt.checkIssues(t, result)
			}
		})
	}
}

func TestValidator_ExtractTemplateVariables(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name     string
		template string
		expected []string
	}{
		{
			name:     "single variable",
			template: `{"env": {"API_KEY": "{{ .API_KEY }}"}}`,
			expected: []string{"API_KEY"},
		},
		{
			name:     "multiple variables",
			template: `{"env": {"API_KEY": "{{ .API_KEY }}", "REGION": "{{ .AWS_REGION }}"}}`,
			expected: []string{"API_KEY", "AWS_REGION"},
		},
		{
			name:     "no variables",
			template: `{"env": {"static": "value"}}`,
			expected: []string{},
		},
		{
			name:     "malformed variables ignored",
			template: `{"env": {"good": "{{ .GOOD_VAR }}", "bad": "{{bad_var}}", "incomplete": "{{INCOMPLETE"}}`,
			expected: []string{"GOOD_VAR"},
		},
		{
			name:     "duplicate variables",
			template: `{"env1": {"API_KEY": "{{ .API_KEY }}"}, "env2": {"key": "{{ .API_KEY }}"}}`,
			expected: []string{"API_KEY"}, // Should be deduplicated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.extractTemplateVariables(tt.template)

			// Sort both slices for comparison since order doesn't matter
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestValidator_IsValidSemver(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		version string
		valid   bool
	}{
		{"1.0.0", true},
		{"0.1.0", true},
		{"10.20.30", true},
		{"1.0", false},
		{"1.0.0.1", false},
		{"v1.0.0", false},
		{"1.0.0-alpha", false},
		{"", false},
		{"abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			result := validator.isValidSemver(tt.version)
			assert.Equal(t, tt.valid, result)
		})
	}
}

func TestValidator_IsValidVariableName(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name  string
		valid bool
	}{
		{"API_KEY", true},
		{"AWS_REGION", true},
		{"VAR123", true},
		{"_PRIVATE", true},
		{"SIMPLE", true},
		{"api_key", false}, // lowercase
		{"API-KEY", false}, // hyphen
		{"123VAR", true},   // starts with number (allowed)
		{"", false},        // empty
		{"API KEY", false}, // space
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.isValidVariableName(tt.name)
			assert.Equal(t, tt.valid, result, "Variable name: %s", tt.name)
		})
	}
}

// Helper functions for tests

func createValidCompleteBundle(t *testing.T, fs afero.Fs, bundlePath string) {
	createValidManifest(t, fs, bundlePath)

	template := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"test-server": map[string]interface{}{
				"command": "echo",
				"args":    []string{"test"},
				"env": map[string]string{
					"API_KEY": "{{ .API_KEY }}",
				},
			},
		},
	}
	createJSONFile(t, fs, filepath.Join(bundlePath, "template.json"), template)

	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"API_KEY": map[string]interface{}{
				"type":        "string",
				"description": "API key for authentication",
			},
		},
		"required": []string{"API_KEY"},
	}
	createJSONFile(t, fs, filepath.Join(bundlePath, "variables.schema.json"), schema)

	// Create README
	createFile(t, fs, filepath.Join(bundlePath, "README.md"), "# Test Bundle\n\nThis is a test bundle.")

	// Create examples
	_ = fs.MkdirAll(filepath.Join(bundlePath, "examples"), 0755)
	createFile(t, fs, filepath.Join(bundlePath, "examples", "dev.vars.yml"), "API_KEY: dev-key")
}

func createValidManifest(t *testing.T, fs afero.Fs, bundlePath string) {
	manifest := bundle.BundleManifest{
		Name:           "test-bundle",
		Version:        "1.0.0",
		Description:    "Test bundle for validation",
		Author:         "Test Author",
		License:        "MIT",
		StationVersion: ">=0.1.0",
	}
	createJSONFile(t, fs, filepath.Join(bundlePath, "manifest.json"), manifest)
}

func createFile(t *testing.T, fs afero.Fs, filePath, content string) {
	err := afero.WriteFile(fs, filePath, []byte(content), 0644)
	require.NoError(t, err)
}

func createJSONFile(t *testing.T, fs afero.Fs, filePath string, data interface{}) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	require.NoError(t, err)
	createFile(t, fs, filePath, string(jsonData))
}
