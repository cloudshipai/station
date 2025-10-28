package local

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"station/pkg/bundle"
)

func TestLocalRegistry_List(t *testing.T) {
	tests := []struct {
		name            string
		setupRegistry   func(fs afero.Fs, registryPath string)
		opts            bundle.ListOptions
		expectedBundles int
		expectError     bool
		checkBundles    func(t *testing.T, bundles []bundle.BundleManifest)
	}{
		{
			name: "empty registry",
			setupRegistry: func(fs afero.Fs, registryPath string) {
				_ = fs.MkdirAll(registryPath, 0755)
			},
			opts:            bundle.ListOptions{},
			expectedBundles: 0,
			expectError:     false,
		},
		{
			name: "registry with multiple bundles",
			setupRegistry: func(fs afero.Fs, registryPath string) {
				createTestBundle(t, fs, registryPath, "openai-assistant", "1.0.0", []string{"ai", "llm"})
				createTestBundle(t, fs, registryPath, "openai-assistant", "1.1.0", []string{"ai", "llm"})
				createTestBundle(t, fs, registryPath, "github-tools", "2.0.0", []string{"git", "dev"})
			},
			opts:            bundle.ListOptions{},
			expectedBundles: 2,
			expectError:     false,
			checkBundles: func(t *testing.T, bundles []bundle.BundleManifest) {
				names := make(map[string]bool)
				for _, b := range bundles {
					names[b.Name] = true
				}
				assert.True(t, names["openai-assistant"])
				assert.True(t, names["github-tools"])

				// Should return latest versions
				for _, b := range bundles {
					if b.Name == "openai-assistant" {
						assert.Equal(t, "1.1.0", b.Version)
					}
				}
			},
		},
		{
			name: "search filter",
			setupRegistry: func(fs afero.Fs, registryPath string) {
				createTestBundle(t, fs, registryPath, "openai-assistant", "1.0.0", []string{"ai"})
				createTestBundle(t, fs, registryPath, "github-tools", "1.0.0", []string{"git"})
			},
			opts:            bundle.ListOptions{Search: "openai"},
			expectedBundles: 1,
			expectError:     false,
			checkBundles: func(t *testing.T, bundles []bundle.BundleManifest) {
				assert.Equal(t, "openai-assistant", bundles[0].Name)
			},
		},
		{
			name: "tags filter",
			setupRegistry: func(fs afero.Fs, registryPath string) {
				createTestBundle(t, fs, registryPath, "ai-bundle", "1.0.0", []string{"ai", "llm"})
				createTestBundle(t, fs, registryPath, "git-bundle", "1.0.0", []string{"git", "dev"})
			},
			opts:            bundle.ListOptions{Tags: []string{"ai"}},
			expectedBundles: 1,
			expectError:     false,
			checkBundles: func(t *testing.T, bundles []bundle.BundleManifest) {
				assert.Equal(t, "ai-bundle", bundles[0].Name)
			},
		},
		{
			name: "registry filter - wrong registry",
			setupRegistry: func(fs afero.Fs, registryPath string) {
				createTestBundle(t, fs, registryPath, "test-bundle", "1.0.0", []string{})
			},
			opts:            bundle.ListOptions{Registry: "other-registry"},
			expectedBundles: 0,
			expectError:     false,
		},
		{
			name: "nonexistent registry path",
			setupRegistry: func(fs afero.Fs, registryPath string) {
				// Don't create the registry path
			},
			opts:            bundle.ListOptions{},
			expectedBundles: 0,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			fs := afero.NewMemMapFs()
			registryPath := "/local-registry"
			registry := NewLocalRegistry("test-registry", registryPath, fs)

			tt.setupRegistry(fs, registryPath)

			// Execute
			bundles, err := registry.List(context.Background(), tt.opts)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, bundles, tt.expectedBundles)
				if tt.checkBundles != nil && len(bundles) > 0 {
					tt.checkBundles(t, bundles)
				}
			}
		})
	}
}

func TestLocalRegistry_Get(t *testing.T) {
	tests := []struct {
		name          string
		setupRegistry func(fs afero.Fs, registryPath string)
		bundleName    string
		version       string
		expectError   bool
		checkManifest func(t *testing.T, manifest *bundle.BundleManifest)
	}{
		{
			name: "get latest version",
			setupRegistry: func(fs afero.Fs, registryPath string) {
				createTestBundle(t, fs, registryPath, "test-bundle", "1.0.0", []string{})
				createTestBundle(t, fs, registryPath, "test-bundle", "1.1.0", []string{})
			},
			bundleName:  "test-bundle",
			version:     "",
			expectError: false,
			checkManifest: func(t *testing.T, manifest *bundle.BundleManifest) {
				assert.Equal(t, "test-bundle", manifest.Name)
				assert.Equal(t, "1.1.0", manifest.Version) // Should get latest
			},
		},
		{
			name: "get specific version",
			setupRegistry: func(fs afero.Fs, registryPath string) {
				createTestBundle(t, fs, registryPath, "test-bundle", "1.0.0", []string{})
				createTestBundle(t, fs, registryPath, "test-bundle", "1.1.0", []string{})
			},
			bundleName:  "test-bundle",
			version:     "1.0.0",
			expectError: false,
			checkManifest: func(t *testing.T, manifest *bundle.BundleManifest) {
				assert.Equal(t, "1.0.0", manifest.Version)
			},
		},
		{
			name: "bundle not found",
			setupRegistry: func(fs afero.Fs, registryPath string) {
				// Create empty registry
				_ = fs.MkdirAll(registryPath, 0755)
			},
			bundleName:  "nonexistent-bundle",
			version:     "",
			expectError: true,
		},
		{
			name: "version not found",
			setupRegistry: func(fs afero.Fs, registryPath string) {
				createTestBundle(t, fs, registryPath, "test-bundle", "1.0.0", []string{})
			},
			bundleName:  "test-bundle",
			version:     "2.0.0",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			fs := afero.NewMemMapFs()
			registryPath := "/local-registry"
			registry := NewLocalRegistry("test-registry", registryPath, fs)

			tt.setupRegistry(fs, registryPath)

			// Execute
			manifest, err := registry.Get(context.Background(), tt.bundleName, tt.version)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, manifest)
			} else {
				require.NoError(t, err)
				require.NotNil(t, manifest)
				if tt.checkManifest != nil {
					tt.checkManifest(t, manifest)
				}
			}
		})
	}
}

func TestLocalRegistry_GetVersions(t *testing.T) {
	tests := []struct {
		name             string
		setupRegistry    func(fs afero.Fs, registryPath string)
		bundleName       string
		expectedVersions []string
		expectError      bool
	}{
		{
			name: "single version",
			setupRegistry: func(fs afero.Fs, registryPath string) {
				createTestBundle(t, fs, registryPath, "test-bundle", "1.0.0", []string{})
			},
			bundleName:       "test-bundle",
			expectedVersions: []string{"1.0.0"},
			expectError:      false,
		},
		{
			name: "multiple versions sorted",
			setupRegistry: func(fs afero.Fs, registryPath string) {
				createTestBundle(t, fs, registryPath, "test-bundle", "1.2.0", []string{})
				createTestBundle(t, fs, registryPath, "test-bundle", "1.0.0", []string{})
				createTestBundle(t, fs, registryPath, "test-bundle", "1.1.0", []string{})
			},
			bundleName:       "test-bundle",
			expectedVersions: []string{"1.0.0", "1.1.0", "1.2.0"},
			expectError:      false,
		},
		{
			name: "bundle not found",
			setupRegistry: func(fs afero.Fs, registryPath string) {
				_ = fs.MkdirAll(registryPath, 0755)
			},
			bundleName:  "nonexistent-bundle",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			fs := afero.NewMemMapFs()
			registryPath := "/local-registry"
			registry := NewLocalRegistry("test-registry", registryPath, fs)

			tt.setupRegistry(fs, registryPath)

			// Execute
			versions, err := registry.GetVersions(context.Background(), tt.bundleName)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedVersions, versions)
			}
		})
	}
}

func TestLocalRegistry_Download(t *testing.T) {
	tests := []struct {
		name          string
		setupRegistry func(fs afero.Fs, registryPath string)
		bundleName    string
		version       string
		expectError   bool
	}{
		{
			name: "archive not found",
			setupRegistry: func(fs afero.Fs, registryPath string) {
				createTestBundle(t, fs, registryPath, "test-bundle", "1.0.0", []string{})
			},
			bundleName:  "test-bundle",
			version:     "1.0.0",
			expectError: true, // No archive file created
		},
		{
			name: "bundle not found",
			setupRegistry: func(fs afero.Fs, registryPath string) {
				_ = fs.MkdirAll(registryPath, 0755)
			},
			bundleName:  "nonexistent-bundle",
			version:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			fs := afero.NewMemMapFs()
			registryPath := "/local-registry"
			registry := NewLocalRegistry("test-registry", registryPath, fs)

			tt.setupRegistry(fs, registryPath)

			// Execute
			data, err := registry.Download(context.Background(), tt.bundleName, tt.version)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, data)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, data)
			}
		})
	}
}

func TestLocalRegistry_WithArchiveFile(t *testing.T) {
	// Setup
	fs := afero.NewMemMapFs()
	registryPath := "/local-registry"
	registry := NewLocalRegistry("test-registry", registryPath, fs)

	// Create bundle with archive file
	bundleName := "test-bundle"
	version := "1.0.0"
	createTestBundle(t, fs, registryPath, bundleName, version, []string{})

	// Create archive file
	archiveData := []byte("mock-archive-data")
	archivePath := filepath.Join(registryPath, bundleName, version, bundleName+"-"+version+".tar.gz")
	err := afero.WriteFile(fs, archivePath, archiveData, 0644)
	require.NoError(t, err)

	// Test download with archive
	data, err := registry.Download(context.Background(), bundleName, version)
	require.NoError(t, err)
	assert.Equal(t, archiveData, data)
}

func TestLocalRegistry_EdgeCases(t *testing.T) {
	t.Run("directory without manifest", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		registryPath := "/local-registry"
		registry := NewLocalRegistry("test-registry", registryPath, fs)

		// Create directory structure without manifest
		bundlePath := filepath.Join(registryPath, "invalid-bundle", "1.0.0")
		fs.MkdirAll(bundlePath, 0755)
		// Don't create manifest.json

		bundles, err := registry.List(context.Background(), bundle.ListOptions{})
		require.NoError(t, err)
		assert.Empty(t, bundles) // Should skip invalid bundles
	})

	t.Run("invalid manifest JSON", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		registryPath := "/local-registry"
		registry := NewLocalRegistry("test-registry", registryPath, fs)

		// Create bundle with invalid manifest
		bundlePath := filepath.Join(registryPath, "invalid-bundle", "1.0.0")
		_ = fs.MkdirAll(bundlePath, 0755)
		manifestPath := filepath.Join(bundlePath, "manifest.json")
		_ = afero.WriteFile(fs, manifestPath, []byte("invalid json"), 0644)

		bundles, err := registry.List(context.Background(), bundle.ListOptions{})
		require.NoError(t, err)
		assert.Empty(t, bundles) // Should skip invalid manifests
	})
}

// Helper functions

func createTestBundle(t *testing.T, fs afero.Fs, registryPath, name, version string, tags []string) {
	bundlePath := filepath.Join(registryPath, name, version)
	err := fs.MkdirAll(bundlePath, 0755)
	require.NoError(t, err)

	manifest := bundle.BundleManifest{
		Name:        name,
		Version:     version,
		Description: "Test bundle: " + name,
		Author:      "Test Author",
		Tags:        tags,
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)

	manifestPath := filepath.Join(bundlePath, "manifest.json")
	err = afero.WriteFile(fs, manifestPath, manifestData, 0644)
	require.NoError(t, err)
}
