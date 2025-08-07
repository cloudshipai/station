package cli

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"station/pkg/bundle"
)

func TestBundleCLI_CreateBundle(t *testing.T) {
	tests := []struct {
		name        string
		bundlePath  string
		opts        bundle.CreateOptions
		setupFS     func(fs afero.Fs)
		expectError bool
		checkResult func(t *testing.T, fs afero.Fs, bundlePath string)
	}{
		{
			name:       "successful creation",
			bundlePath: "/test/my-bundle",
			opts: bundle.CreateOptions{
				Name:        "my-bundle",
				Author:      "Test Author",
				Description: "Test bundle for CLI",
			},
			setupFS:     func(fs afero.Fs) {},
			expectError: false,
			checkResult: func(t *testing.T, fs afero.Fs, bundlePath string) {
				// Check that required files were created
				files := []string{
					"manifest.json",
					"template.json", 
					"variables.schema.json",
					"README.md",
				}

				for _, file := range files {
					path := filepath.Join(bundlePath, file)
					exists, err := afero.Exists(fs, path)
					require.NoError(t, err)
					assert.True(t, exists, "File should exist: %s", file)
				}

				// Check examples directory
				examplesDir := filepath.Join(bundlePath, "examples")
				exists, err := afero.DirExists(fs, examplesDir)
				require.NoError(t, err)
				assert.True(t, exists, "Examples directory should exist")
			},
		},
		{
			name:       "directory already exists",
			bundlePath: "/test/existing-bundle",
			opts: bundle.CreateOptions{
				Name:        "existing-bundle",
				Author:      "Test Author", 
				Description: "Test bundle",
			},
			setupFS: func(fs afero.Fs) {
				fs.MkdirAll("/test/existing-bundle", 0755)
			},
			expectError: true,
		},
		{
			name:       "missing required options",
			bundlePath: "/test/invalid-bundle",
			opts: bundle.CreateOptions{
				Name: "invalid-bundle",
				// Missing Author and Description
			},
			setupFS:     func(fs afero.Fs) {},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			fs := afero.NewMemMapFs()
			cli := NewBundleCLI(fs)
			tt.setupFS(fs)

			// Execute
			err := cli.CreateBundle(tt.bundlePath, tt.opts)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.checkResult != nil {
					tt.checkResult(t, fs, tt.bundlePath)
				}
			}
		})
	}
}

func TestBundleCLI_ValidateBundle(t *testing.T) {
	tests := []struct {
		name         string
		bundlePath   string
		setupBundle  func(fs afero.Fs, bundlePath string)
		expectError  bool
		expectValid  bool
		expectIssues int
	}{
		{
			name:       "valid bundle",
			bundlePath: "/test/valid-bundle",
			setupBundle: func(fs afero.Fs, bundlePath string) {
				createValidTestBundle(t, fs, bundlePath)
			},
			expectError: false,
			expectValid: true,
			expectIssues: 0,
		},
		{
			name:       "bundle with issues",
			bundlePath: "/test/invalid-bundle",
			setupBundle: func(fs afero.Fs, bundlePath string) {
				// Create bundle with missing manifest
				fs.MkdirAll(bundlePath, 0755)
				createFile(t, fs, filepath.Join(bundlePath, "template.json"), `{"mcpServers":{}}`)
				createFile(t, fs, filepath.Join(bundlePath, "variables.schema.json"), `{"type":"object","properties":{}}`)
			},
			expectError:  false,
			expectValid:  false,
			expectIssues: 1, // Missing manifest
		},
		{
			name:        "bundle does not exist",
			bundlePath:  "/test/nonexistent",
			setupBundle: func(fs afero.Fs, bundlePath string) {},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			fs := afero.NewMemMapFs()
			cli := NewBundleCLI(fs)
			tt.setupBundle(fs, tt.bundlePath)

			// Execute
			summary, err := cli.ValidateBundle(tt.bundlePath)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, summary)
			} else {
				require.NoError(t, err)
				require.NotNil(t, summary)
				assert.Equal(t, tt.expectValid, summary.Valid)
				assert.Len(t, summary.Issues, tt.expectIssues)
				assert.Equal(t, tt.bundlePath, summary.BundlePath)
			}
		})
	}
}

func TestBundleCLI_PackageBundle(t *testing.T) {
	tests := []struct {
		name          string
		bundlePath    string
		outputPath    string
		validateFirst bool
		setupBundle   func(fs afero.Fs, bundlePath string)
		expectError   bool
		expectSuccess bool
	}{
		{
			name:          "successful packaging without validation",
			bundlePath:    "/test/valid-bundle",
			outputPath:    "/test/output.tar.gz",
			validateFirst: false,
			setupBundle: func(fs afero.Fs, bundlePath string) {
				createValidTestBundle(t, fs, bundlePath)
			},
			expectError:   false,
			expectSuccess: true,
		},
		{
			name:          "packaging with validation - valid bundle",
			bundlePath:    "/test/valid-bundle",
			outputPath:    "/test/output.tar.gz",
			validateFirst: true,
			setupBundle: func(fs afero.Fs, bundlePath string) {
				createValidTestBundle(t, fs, bundlePath)
			},
			expectError:   false,
			expectSuccess: true,
		},
		{
			name:          "packaging with validation - invalid bundle",
			bundlePath:    "/test/invalid-bundle",
			outputPath:    "/test/output.tar.gz",
			validateFirst: true,
			setupBundle: func(fs afero.Fs, bundlePath string) {
				// Create invalid bundle (missing manifest)
				fs.MkdirAll(bundlePath, 0755)
				createFile(t, fs, filepath.Join(bundlePath, "template.json"), `{"mcpServers":{}}`)
			},
			expectError:   false,
			expectSuccess: false, // Should fail validation
		},
		{
			name:          "default output path",
			bundlePath:    "/test/my-bundle",
			outputPath:    "", // Should generate default
			validateFirst: false,
			setupBundle: func(fs afero.Fs, bundlePath string) {
				createValidTestBundle(t, fs, bundlePath)
			},
			expectError:   false,
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			fs := afero.NewMemMapFs()
			cli := NewBundleCLI(fs)
			tt.setupBundle(fs, tt.bundlePath)

			// Execute
			summary, err := cli.PackageBundle(tt.bundlePath, tt.outputPath, tt.validateFirst)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, summary)
				assert.Equal(t, tt.expectSuccess, summary.Success)

				if tt.expectSuccess {
					assert.NotEmpty(t, summary.OutputPath)
					assert.Greater(t, summary.Size, int64(0))
				}

				if tt.validateFirst {
					assert.NotNil(t, summary.ValidationSummary)
				}
			}
		})
	}
}

func TestBundleCLI_VariableAnalysis(t *testing.T) {
	tests := []struct {
		name              string
		bundlePath        string
		setupBundle       func(fs afero.Fs, bundlePath string)
		expectMissingVars []string
	}{
		{
			name:       "template with variables and matching schema",
			bundlePath: "/test/consistent-bundle",
			setupBundle: func(fs afero.Fs, bundlePath string) {
				fs.MkdirAll(bundlePath, 0755)
				createValidManifest(t, fs, bundlePath)
				
				// Template with variables
				template := `{
					"mcpServers": {
						"test": {
							"env": {
								"API_KEY": "{{ .API_KEY }}",
								"REGION": "{{ .AWS_REGION }}"
							}
						}
					}
				}`
				createFile(t, fs, filepath.Join(bundlePath, "template.json"), template)
				
				// Schema with matching variables
				schema := `{
					"type": "object",
					"properties": {
						"API_KEY": {"type": "string"},
						"AWS_REGION": {"type": "string"}
					}
				}`
				createFile(t, fs, filepath.Join(bundlePath, "variables.schema.json"), schema)
			},
			expectMissingVars: []string{}, // Should be consistent
		},
		{
			name:       "template with missing schema variables",
			bundlePath: "/test/inconsistent-bundle",
			setupBundle: func(fs afero.Fs, bundlePath string) {
				fs.MkdirAll(bundlePath, 0755)
				createValidManifest(t, fs, bundlePath)
				
				// Template with variables
				template := `{
					"mcpServers": {
						"test": {
							"env": {
								"API_KEY": "{{ .API_KEY }}",
								"MISSING_VAR": "{{ .MISSING_VAR }}"
							}
						}
					}
				}`
				createFile(t, fs, filepath.Join(bundlePath, "template.json"), template)
				
				// Schema missing MISSING_VAR
				schema := `{
					"type": "object",
					"properties": {
						"API_KEY": {"type": "string"}
					}
				}`
				createFile(t, fs, filepath.Join(bundlePath, "variables.schema.json"), schema)
			},
			expectMissingVars: []string{"MISSING_VAR"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			fs := afero.NewMemMapFs()
			cli := NewBundleCLI(fs)
			tt.setupBundle(fs, tt.bundlePath)

			// Execute
			summary, err := cli.ValidateBundle(tt.bundlePath)

			// Assert
			require.NoError(t, err)
			require.NotNil(t, summary)

			if len(tt.expectMissingVars) > 0 {
				assert.NotNil(t, summary.VariableAnalysis)
				for _, expectedVar := range tt.expectMissingVars {
					assert.Contains(t, summary.VariableAnalysis.MissingInSchema, expectedVar)
				}
			}
		})
	}
}

// Helper functions

func createValidTestBundle(t *testing.T, fs afero.Fs, bundlePath string) {
	fs.MkdirAll(bundlePath, 0755)
	
	createValidManifest(t, fs, bundlePath)
	
	createFile(t, fs, filepath.Join(bundlePath, "template.json"), `{
		"mcpServers": {
			"test-server": {
				"command": "echo",
				"args": ["test"]
			}
		}
	}`)
	
	createFile(t, fs, filepath.Join(bundlePath, "variables.schema.json"), `{
		"type": "object",
		"properties": {}
	}`)
	
	createFile(t, fs, filepath.Join(bundlePath, "README.md"), "# Test Bundle")
	
	// Create examples directory
	fs.MkdirAll(filepath.Join(bundlePath, "examples"), 0755)
}

func createValidManifest(t *testing.T, fs afero.Fs, bundlePath string) {
	manifest := `{
		"name": "test-bundle",
		"version": "1.0.0",
		"description": "Test bundle",
		"author": "Test Author",
		"station_version": ">=0.1.0"
	}`
	createFile(t, fs, filepath.Join(bundlePath, "manifest.json"), manifest)
}

func createFile(t *testing.T, fs afero.Fs, path, content string) {
	err := afero.WriteFile(fs, path, []byte(content), 0644)
	require.NoError(t, err)
}