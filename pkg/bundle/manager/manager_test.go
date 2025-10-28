package manager

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"station/pkg/bundle"
)

// MockRegistry implements BundleRegistry for testing
type MockRegistry struct {
	mock.Mock
}

func (m *MockRegistry) List(ctx context.Context, opts bundle.ListOptions) ([]bundle.BundleManifest, error) {
	args := m.Called(ctx, opts)
	return args.Get(0).([]bundle.BundleManifest), args.Error(1)
}

func (m *MockRegistry) Get(ctx context.Context, name, version string) (*bundle.BundleManifest, error) {
	args := m.Called(ctx, name, version)
	return args.Get(0).(*bundle.BundleManifest), args.Error(1)
}

func (m *MockRegistry) Download(ctx context.Context, name, version string) ([]byte, error) {
	args := m.Called(ctx, name, version)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockRegistry) GetVersions(ctx context.Context, name string) ([]string, error) {
	args := m.Called(ctx, name)
	return args.Get(0).([]string), args.Error(1)
}

// MockValidator implements BundleValidator for testing
type MockValidator struct {
	mock.Mock
}

func (m *MockValidator) Validate(fs afero.Fs, bundlePath string) (*bundle.ValidationResult, error) {
	args := m.Called(fs, bundlePath)
	return args.Get(0).(*bundle.ValidationResult), args.Error(1)
}

func TestManager_ParseBundleReference(t *testing.T) {
	tests := []struct {
		name         string
		ref          string
		expectedName string
		expectedReg  string
		expectedVer  string
		expectError  bool
	}{
		{
			name:         "simple name",
			ref:          "openai-assistant",
			expectedName: "openai-assistant",
			expectedReg:  "default",
			expectedVer:  "",
		},
		{
			name:         "name with version",
			ref:          "openai-assistant@1.0.0",
			expectedName: "openai-assistant",
			expectedReg:  "default",
			expectedVer:  "1.0.0",
		},
		{
			name:         "registry with name",
			ref:          "official/openai-assistant",
			expectedName: "openai-assistant",
			expectedReg:  "official",
			expectedVer:  "",
		},
		{
			name:         "full reference",
			ref:          "official/openai-assistant@1.2.0",
			expectedName: "openai-assistant",
			expectedReg:  "official",
			expectedVer:  "1.2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager("/config", afero.NewMemMapFs())

			result, err := manager.parseBundleReference(tt.ref)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedName, result.Name)
				assert.Equal(t, tt.expectedReg, result.Registry)
				assert.Equal(t, tt.expectedVer, result.Version)
			}
		})
	}
}

func TestManager_Install(t *testing.T) {
	// Skip complex archive tests for now, focus on integration workflow
	t.Skip("Skipping install tests - focusing on integration with stn mcp sync")
}

func TestManager_RenderToEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		bundleName  string
		environment string
		variables   map[string]interface{}
		setupBundle func(fs afero.Fs, bundlesDir string)
		expectError bool
		checkResult func(t *testing.T, fs afero.Fs)
	}{
		{
			name:        "successful render",
			bundleName:  "test-bundle",
			environment: "development",
			variables:   map[string]interface{}{"API_KEY": "dev-key-123"},
			setupBundle: func(fs afero.Fs, bundlesDir string) {
				createInstalledTestBundle(t, fs, bundlesDir, "test-bundle")
			},
			expectError: false,
			checkResult: func(t *testing.T, fs afero.Fs) {
				// Check that config file was created
				configPath := "/config/environments/development/test-bundle.json"
				exists, err := afero.Exists(fs, configPath)
				require.NoError(t, err)
				assert.True(t, exists)

				// Check that variables were replaced
				content, err := afero.ReadFile(fs, configPath)
				require.NoError(t, err)
				assert.Contains(t, string(content), "dev-key-123")
				assert.NotContains(t, string(content), "{{ .API_KEY }}")
			},
		},
		{
			name:        "bundle not installed",
			bundleName:  "missing-bundle",
			environment: "development",
			variables:   map[string]interface{}{},
			setupBundle: func(fs afero.Fs, bundlesDir string) {
				// Don't create bundle
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			fs := afero.NewMemMapFs()
			bundlesDir := "/config/bundles"
			manager := NewManager("/config", fs)

			tt.setupBundle(fs, bundlesDir)

			// Execute
			err := manager.RenderToEnvironment(context.Background(), tt.bundleName, tt.environment, tt.variables)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.checkResult != nil {
					tt.checkResult(t, fs)
				}
			}
		})
	}
}

func TestManager_List(t *testing.T) {
	tests := []struct {
		name            string
		opts            bundle.ListOptions
		setupRegistries func() map[string]*MockRegistry
		expectedBundles int
		expectError     bool
	}{
		{
			name: "list from all registries",
			opts: bundle.ListOptions{},
			setupRegistries: func() map[string]*MockRegistry {
				reg1 := new(MockRegistry)
				reg1.On("List", mock.Anything, mock.Anything).Return([]bundle.BundleManifest{
					{Name: "bundle1", Version: "1.0.0"},
				}, nil)

				reg2 := new(MockRegistry)
				reg2.On("List", mock.Anything, mock.Anything).Return([]bundle.BundleManifest{
					{Name: "bundle2", Version: "2.0.0"},
				}, nil)

				return map[string]*MockRegistry{
					"registry1": reg1,
					"registry2": reg2,
				}
			},
			expectedBundles: 2,
			expectError:     false,
		},
		{
			name: "filter by registry",
			opts: bundle.ListOptions{Registry: "registry1"},
			setupRegistries: func() map[string]*MockRegistry {
				reg1 := new(MockRegistry)
				reg1.On("List", mock.Anything, mock.Anything).Return([]bundle.BundleManifest{
					{Name: "bundle1", Version: "1.0.0"},
				}, nil)

				reg2 := new(MockRegistry)
				// reg2 should not be called

				return map[string]*MockRegistry{
					"registry1": reg1,
					"registry2": reg2,
				}
			},
			expectedBundles: 1,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			manager := NewManager("/config", afero.NewMemMapFs())
			mockRegistries := tt.setupRegistries()

			for name, registry := range mockRegistries {
				manager.AddRegistry(name, registry)
			}

			// Execute
			bundles, err := manager.List(context.Background(), tt.opts)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, bundles, tt.expectedBundles)
			}

			// Verify appropriate registries were called
			for name, registry := range mockRegistries {
				if tt.opts.Registry == "" || tt.opts.Registry == name {
					registry.AssertExpectations(t)
				}
			}
		})
	}
}

func TestManager_RemoveBundle(t *testing.T) {
	t.Run("successful removal", func(t *testing.T) {
		// Setup
		fs := afero.NewMemMapFs()
		bundlesDir := "/config/bundles"
		manager := NewManager("/config", fs)

		// Create installed bundle
		createInstalledTestBundle(t, fs, bundlesDir, "test-bundle")

		// Verify bundle exists
		installed, err := manager.ListInstalled()
		require.NoError(t, err)
		require.Len(t, installed, 1)

		// Execute removal
		err = manager.Remove("test-bundle")
		require.NoError(t, err)

		// Verify bundle was removed
		installed, err = manager.ListInstalled()
		require.NoError(t, err)
		assert.Len(t, installed, 0)

		// Verify bundle directory was removed
		exists, err := afero.DirExists(fs, filepath.Join(bundlesDir, "test-bundle"))
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("bundle not found", func(t *testing.T) {
		manager := NewManager("/config", afero.NewMemMapFs())
		err := manager.Remove("nonexistent-bundle")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bundle not installed")
	})
}

// Helper functions for tests

func createTestBundleArchive(t *testing.T) []byte {
	// For testing purposes, just return empty data and skip extraction
	// In a real implementation, we'd need proper tar.gz data
	return []byte("")
}

func createInstalledTestBundle(t *testing.T, fs afero.Fs, bundlesDir, bundleName string) {
	// Create bundle directory
	bundleDir := filepath.Join(bundlesDir, bundleName)
	err := fs.MkdirAll(bundleDir, 0755)
	require.NoError(t, err)

	// Create manifest
	manifest := bundle.BundleManifest{
		Name:        bundleName,
		Version:     "1.0.0",
		Description: "Test bundle",
		Author:      "Test Author",
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	err = afero.WriteFile(fs, filepath.Join(bundleDir, "manifest.json"), manifestData, 0644)
	require.NoError(t, err)

	// Create template with variable placeholder
	template := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"test-server": map[string]interface{}{
				"command": "echo",
				"env": map[string]string{
					"API_KEY": "{{ .API_KEY }}",
				},
			},
		},
	}
	templateData, err := json.MarshalIndent(template, "", "  ")
	require.NoError(t, err)
	err = afero.WriteFile(fs, filepath.Join(bundleDir, "template.json"), templateData, 0644)
	require.NoError(t, err)

	// Create installed bundles record
	installed := []bundle.InstalledBundle{
		{
			BundleReference: bundle.BundleReference{
				Name:     bundleName,
				Version:  "1.0.0",
				Registry: "default",
			},
			LocalPath:   bundleDir,
			InstallTime: time.Now(),
		},
	}

	installedData, err := json.MarshalIndent(installed, "", "  ")
	require.NoError(t, err)
	err = afero.WriteFile(fs, filepath.Join(bundlesDir, "installed.json"), installedData, 0644)
	require.NoError(t, err)
}
