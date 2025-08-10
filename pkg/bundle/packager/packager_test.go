package packager

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"station/pkg/bundle"
)

// MockValidator implements BundleValidator for testing
type MockValidator struct {
	mock.Mock
}

func (m *MockValidator) Validate(fs afero.Fs, bundlePath string) (*bundle.ValidationResult, error) {
	args := m.Called(fs, bundlePath)
	return args.Get(0).(*bundle.ValidationResult), args.Error(1)
}

func TestPackager_Package(t *testing.T) {
	tests := []struct {
		name           string
		setupBundle    func(fs afero.Fs, bundlePath string)
		setupValidator func(validator *MockValidator)
		outputPath     string
		wantSuccess    bool
		wantError      bool
		checkArchive   func(t *testing.T, fs afero.Fs, outputPath string)
	}{
		{
			name: "valid bundle packaging",
			setupBundle: func(fs afero.Fs, bundlePath string) {
				createValidTestBundle(t, fs, bundlePath)
			},
			setupValidator: func(validator *MockValidator) {
				validator.On("Validate", mock.Anything, mock.Anything).Return(&bundle.ValidationResult{
					Valid:    true,
					Issues:   []bundle.ValidationIssue{},
					Warnings: []bundle.ValidationIssue{},
				}, nil)
			},
			outputPath:   "/output/test-bundle.tar.gz",
			wantSuccess:  true,
			wantError:    false,
			checkArchive: validateArchiveContents,
		},
		{
			name: "invalid bundle fails validation",
			setupBundle: func(fs afero.Fs, bundlePath string) {
				// Create minimal invalid bundle
				fs.MkdirAll(bundlePath, 0755)
				createFile(t, fs, filepath.Join(bundlePath, "invalid.json"), `{invalid json}`)
			},
			setupValidator: func(validator *MockValidator) {
				validator.On("Validate", mock.Anything, mock.Anything).Return(&bundle.ValidationResult{
					Valid: false,
					Issues: []bundle.ValidationIssue{
						{Type: "invalid_json", File: "invalid.json", Message: "Invalid JSON"},
					},
					Warnings: []bundle.ValidationIssue{},
				}, nil)
			},
			outputPath:  "/output/invalid-bundle.tar.gz",
			wantSuccess: false,
			wantError:   false, // No error, just unsuccessful packaging
		},
		{
			name: "validation error",
			setupBundle: func(fs afero.Fs, bundlePath string) {
				createValidTestBundle(t, fs, bundlePath)
			},
			setupValidator: func(validator *MockValidator) {
				validator.On("Validate", mock.Anything, mock.Anything).Return(
					(*bundle.ValidationResult)(nil), 
					assert.AnError,
				)
			},
			outputPath:  "/output/error-bundle.tar.gz",
			wantSuccess: false,
			wantError:   true,
		},
		{
			name: "bundle with subdirectories",
			setupBundle: func(fs afero.Fs, bundlePath string) {
				createValidTestBundle(t, fs, bundlePath)
				
				// Add examples directory with files
				examplesDir := filepath.Join(bundlePath, "examples")
				fs.MkdirAll(examplesDir, 0755)
				createFile(t, fs, filepath.Join(examplesDir, "dev.yml"), "API_KEY: dev-key")
				createFile(t, fs, filepath.Join(examplesDir, "prod.yml"), "API_KEY: prod-key")
				
				// Add nested directory
				nestedDir := filepath.Join(examplesDir, "advanced")
				fs.MkdirAll(nestedDir, 0755)
				createFile(t, fs, filepath.Join(nestedDir, "config.yml"), "complex: config")
			},
			setupValidator: func(validator *MockValidator) {
				validator.On("Validate", mock.Anything, mock.Anything).Return(&bundle.ValidationResult{
					Valid:    true,
					Issues:   []bundle.ValidationIssue{},
					Warnings: []bundle.ValidationIssue{},
				}, nil)
			},
			outputPath:   "/output/nested-bundle.tar.gz",
			wantSuccess:  true,
			wantError:    false,
			checkArchive: validateNestedArchiveContents,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup filesystem
			fs := afero.NewMemMapFs()
			bundlePath := "/test-bundle"
			
			// Setup bundle
			tt.setupBundle(fs, bundlePath)
			
			// Setup mock validator
			mockValidator := new(MockValidator)
			tt.setupValidator(mockValidator)
			
			// Create packager
			packager := NewPackager(mockValidator)
			
			// Create output directory
			outputDir := filepath.Dir(tt.outputPath)
			fs.MkdirAll(outputDir, 0755)
			
			// Package bundle
			result, err := packager.Package(fs, bundlePath, tt.outputPath)
			
			// Check error expectation
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			
			require.NoError(t, err)
			require.NotNil(t, result)
			
			// Check success expectation
			assert.Equal(t, tt.wantSuccess, result.Success)
			
			if tt.wantSuccess {
				// Check file was created
				exists, err := afero.Exists(fs, tt.outputPath)
				require.NoError(t, err)
				assert.True(t, exists, "Output file should exist")
				
				// Check size is set
				assert.Greater(t, result.Size, int64(0), "Package size should be greater than 0")
				
				// Check output path is correct
				assert.Equal(t, tt.outputPath, result.OutputPath)
				
				// Run custom archive checks if provided
				if tt.checkArchive != nil {
					tt.checkArchive(t, fs, tt.outputPath)
				}
			}
			
			// Check validation result is included
			assert.NotNil(t, result.ValidationResult)
			
			// Verify mock was called
			mockValidator.AssertExpectations(t)
		})
	}
}

func TestPackager_FileSystemErrors(t *testing.T) {
	t.Run("bundle directory does not exist", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		// Don't create the bundle directory
		
		// Setup mock validator to return valid result
		mockValidator := new(MockValidator)
		mockValidator.On("Validate", mock.Anything, mock.Anything).Return(&bundle.ValidationResult{
			Valid:    true,
			Issues:   []bundle.ValidationIssue{},
			Warnings: []bundle.ValidationIssue{},
		}, nil)
		
		packager := NewPackager(mockValidator)
		
		result, err := packager.Package(fs, "/nonexistent-bundle", "/output/test.tar.gz")
		
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "failed to add files to archive")
		}
		assert.Nil(t, result)
	})
}

// Helper functions for testing

func createValidTestBundle(t *testing.T, fs afero.Fs, bundlePath string) {
	// Create bundle directory
	fs.MkdirAll(bundlePath, 0755)
	
	// Create manifest.json
	manifest := `{
  "name": "test-bundle",
  "version": "1.0.0",
  "description": "Test bundle for packaging",
  "author": "Test Author",
  "station_version": ">=0.1.0"
}`
	createFile(t, fs, filepath.Join(bundlePath, "manifest.json"), manifest)
	
	// Create template.json
	template := `{
  "mcpServers": {
    "test-server": {
      "command": "echo",
      "args": ["test"]
    }
  }
}`
	createFile(t, fs, filepath.Join(bundlePath, "template.json"), template)
	
	// Create variables.schema.json
	schema := `{
  "type": "object",
  "properties": {}
}`
	createFile(t, fs, filepath.Join(bundlePath, "variables.schema.json"), schema)
	
	// Create README.md
	createFile(t, fs, filepath.Join(bundlePath, "README.md"), "# Test Bundle\n\nTest bundle for packaging.")
}

func createFile(t *testing.T, fs afero.Fs, path, content string) {
	err := afero.WriteFile(fs, path, []byte(content), 0644)
	require.NoError(t, err)
}

func validateArchiveContents(t *testing.T, fs afero.Fs, outputPath string) {
	// Open and read the archive
	file, err := fs.Open(outputPath)
	require.NoError(t, err)
	defer file.Close()
	
	gzReader, err := gzip.NewReader(file)
	require.NoError(t, err)
	defer gzReader.Close()
	
	tarReader := tar.NewReader(gzReader)
	
	expectedFiles := map[string]bool{
		"manifest.json":           false,
		"template.json":           false,
		"variables.schema.json":   false,
		"README.md":               false,
	}
	
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		
		// Mark file as found
		if _, exists := expectedFiles[header.Name]; exists {
			expectedFiles[header.Name] = true
		}
		
		// Basic header validation
		assert.NotEmpty(t, header.Name)
		assert.NotZero(t, header.ModTime)
		
		if header.Typeflag == tar.TypeReg {
			// Read file content to ensure it's valid
			content, err := io.ReadAll(tarReader)
			require.NoError(t, err)
			assert.NotEmpty(t, content, "File %s should have content", header.Name)
		}
	}
	
	// Check all expected files were found
	for filename, found := range expectedFiles {
		assert.True(t, found, "Expected file %s not found in archive", filename)
	}
}

func validateNestedArchiveContents(t *testing.T, fs afero.Fs, outputPath string) {
	// Open and read the archive
	file, err := fs.Open(outputPath)
	require.NoError(t, err)
	defer file.Close()
	
	gzReader, err := gzip.NewReader(file)
	require.NoError(t, err)
	defer gzReader.Close()
	
	tarReader := tar.NewReader(gzReader)
	
	expectedFiles := map[string]bool{
		"manifest.json":                    false,
		"template.json":                    false,
		"variables.schema.json":            false,
		"README.md":                        false,
		"examples/":                        false,
		"examples/dev.yml":                 false,
		"examples/prod.yml":                false,
		"examples/advanced/":               false,
		"examples/advanced/config.yml":     false,
	}
	
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		
		// Mark file/directory as found
		if _, exists := expectedFiles[header.Name]; exists {
			expectedFiles[header.Name] = true
		}
		
		// Validate directory headers
		if header.Typeflag == tar.TypeDir {
			assert.True(t, header.Name[len(header.Name)-1] == '/', "Directory name should end with /: %s", header.Name)
		}
	}
	
	// Check all expected files/directories were found
	for name, found := range expectedFiles {
		assert.True(t, found, "Expected %s not found in archive", name)
	}
}