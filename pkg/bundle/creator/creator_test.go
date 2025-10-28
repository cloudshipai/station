package creator

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"station/pkg/bundle"
)

func TestCreator_Create(t *testing.T) {
	tests := []struct {
		name    string
		opts    bundle.CreateOptions
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid minimal bundle",
			opts: bundle.CreateOptions{
				Name:        "test-bundle",
				Author:      "Test Author",
				Description: "Test bundle description",
			},
			wantErr: false,
		},
		{
			name: "valid bundle with all options",
			opts: bundle.CreateOptions{
				Name:        "comprehensive-bundle",
				Author:      "Test Author",
				Description: "Comprehensive test bundle",
				License:     "Apache-2.0",
				Repository:  "https://github.com/test/bundle",
				Tags:        []string{"test", "example"},
				Variables: map[string]bundle.VariableSpec{
					"API_KEY": {
						Type:        "string",
						Description: "API key for authentication",
						Required:    true,
						Secret:      true,
					},
					"REGION": {
						Type:        "string",
						Description: "AWS region",
						Default:     "us-east-1",
						Enum:        []string{"us-east-1", "us-west-2"},
					},
				},
				Dependencies: map[string]string{
					"docker":  ">=20.0.0",
					"aws-cli": ">=2.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			opts: bundle.CreateOptions{
				Author:      "Test Author",
				Description: "Test description",
			},
			wantErr: true,
			errMsg:  "bundle name is required",
		},
		{
			name: "missing author",
			opts: bundle.CreateOptions{
				Name:        "test-bundle",
				Description: "Test description",
			},
			wantErr: true,
			errMsg:  "bundle author is required",
		},
		{
			name: "missing description",
			opts: bundle.CreateOptions{
				Name:   "test-bundle",
				Author: "Test Author",
			},
			wantErr: true,
			errMsg:  "bundle description is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			creator := NewCreator()
			bundlePath := "/test-bundle"

			err := creator.Create(fs, bundlePath, tt.opts)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)

			// Verify bundle structure
			assertBundleStructure(t, fs, bundlePath)

			// Verify manifest content
			assertManifestContent(t, fs, bundlePath, tt.opts)

			// Verify template content
			assertTemplateContent(t, fs, bundlePath, tt.opts.Name)

			// Verify variables schema
			assertVariablesSchema(t, fs, bundlePath, tt.opts.Variables)

			// Verify README
			assertREADME(t, fs, bundlePath)

			// Verify examples
			assertExamples(t, fs, bundlePath)
		})
	}
}

func assertBundleStructure(t *testing.T, fs afero.Fs, bundlePath string) {
	expectedFiles := []string{
		"manifest.json",
		"template.json",
		"variables.schema.json",
		"README.md",
		"examples/development.vars.yml",
		"agents/assistant.prompt",
		"agents/specialist.prompt",
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(bundlePath, file)
		exists, err := afero.Exists(fs, filePath)
		require.NoError(t, err)
		assert.True(t, exists, "File %s should exist", file)
	}
}

func assertManifestContent(t *testing.T, fs afero.Fs, bundlePath string, opts bundle.CreateOptions) {
	manifestPath := filepath.Join(bundlePath, "manifest.json")
	data, err := afero.ReadFile(fs, manifestPath)
	require.NoError(t, err)

	var manifest bundle.BundleManifest
	err = json.Unmarshal(data, &manifest)
	require.NoError(t, err)

	assert.Equal(t, opts.Name, manifest.Name)
	assert.Equal(t, opts.Author, manifest.Author)
	assert.Equal(t, opts.Description, manifest.Description)
	assert.Equal(t, "1.0.0", manifest.Version)
	assert.Equal(t, ">=0.1.0", manifest.StationVersion)

	if opts.License != "" {
		assert.Equal(t, opts.License, manifest.License)
	} else {
		assert.Equal(t, "MIT", manifest.License)
	}

	if len(opts.Tags) > 0 {
		assert.Equal(t, opts.Tags, manifest.Tags)
	}

	assert.NotZero(t, manifest.CreatedAt)
}

func assertTemplateContent(t *testing.T, fs afero.Fs, bundlePath, bundleName string) {
	templatePath := filepath.Join(bundlePath, "template.json")
	data, err := afero.ReadFile(fs, templatePath)
	require.NoError(t, err)

	var template map[string]interface{}
	err = json.Unmarshal(data, &template)
	require.NoError(t, err)

	assert.Contains(t, template, "mcpServers")
	mcpServers := template["mcpServers"].(map[string]interface{})
	// Creator always uses "filesystem" as the MCP server name, not the bundle name
	assert.Contains(t, mcpServers, "filesystem")
	assert.Equal(t, bundleName, template["name"])
}

func assertVariablesSchema(t *testing.T, fs afero.Fs, bundlePath string, variables map[string]bundle.VariableSpec) {
	schemaPath := filepath.Join(bundlePath, "variables.schema.json")
	data, err := afero.ReadFile(fs, schemaPath)
	require.NoError(t, err)

	var schema map[string]interface{}
	err = json.Unmarshal(data, &schema)
	require.NoError(t, err)

	assert.Equal(t, "object", schema["type"])
	assert.Contains(t, schema, "properties")

	properties := schema["properties"].(map[string]interface{})

	if len(variables) == 0 {
		// Creator uses ROOT_PATH as the default variable, not EXAMPLE_VAR
		assert.Contains(t, properties, "ROOT_PATH")
	} else {
		for name, spec := range variables {
			assert.Contains(t, properties, name)
			prop := properties[name].(map[string]interface{})
			assert.Equal(t, spec.Type, prop["type"])
			assert.Equal(t, spec.Description, prop["description"])
		}
	}
}

func assertREADME(t *testing.T, fs afero.Fs, bundlePath string) {
	readmePath := filepath.Join(bundlePath, "README.md")
	data, err := afero.ReadFile(fs, readmePath)
	require.NoError(t, err)

	readme := string(data)
	assert.Contains(t, readme, "# ")
	assert.Contains(t, readme, "Installation")
	assert.Contains(t, readme, "Usage")
	assert.Contains(t, readme, "Required Variables")
}

func assertExamples(t *testing.T, fs afero.Fs, bundlePath string) {
	// Creator only creates development.vars.yml, not production.vars.yml
	examplePath := filepath.Join(bundlePath, "examples", "development.vars.yml")
	data, err := afero.ReadFile(fs, examplePath)
	require.NoError(t, err)

	var vars map[string]interface{}
	err = yaml.Unmarshal(data, &vars)
	require.NoError(t, err)

	assert.NotEmpty(t, vars)
}

func TestCreator_CreateExistingDirectory(t *testing.T) {
	fs := afero.NewMemMapFs()
	creator := NewCreator()
	bundlePath := "/existing-bundle"

	// Create directory first
	err := fs.MkdirAll(bundlePath, 0755)
	require.NoError(t, err)

	opts := bundle.CreateOptions{
		Name:        "existing-bundle",
		Author:      "Test Author",
		Description: "Test bundle",
	}

	// Should not fail if directory exists
	err = creator.Create(fs, bundlePath, opts)
	assert.NoError(t, err)
}
