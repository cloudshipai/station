package manager

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"

	"station/pkg/bundle"
)

// Manager implements the BundleManager interface
type Manager struct {
	fs         afero.Fs
	configDir  string
	bundlesDir string
	registries map[string]bundle.BundleRegistry
	creator    bundle.BundleCreator
	validator  bundle.BundleValidator
	packager   bundle.BundlePackager
}

// NewManager creates a new bundle manager
func NewManager(configDir string, fs afero.Fs) *Manager {
	if fs == nil {
		fs = afero.NewOsFs()
	}

	return &Manager{
		fs:         fs,
		configDir:  configDir,
		bundlesDir: filepath.Join(configDir, "bundles"),
		registries: make(map[string]bundle.BundleRegistry),
	}
}

// SetCreator sets the bundle creator
func (m *Manager) SetCreator(creator bundle.BundleCreator) {
	m.creator = creator
}

// SetValidator sets the bundle validator
func (m *Manager) SetValidator(validator bundle.BundleValidator) {
	m.validator = validator
}

// SetPackager sets the bundle packager
func (m *Manager) SetPackager(packager bundle.BundlePackager) {
	m.packager = packager
}

// Install installs a bundle from a registry and renders it to environment config
func (m *Manager) Install(ctx context.Context, ref string, opts bundle.InstallOptions) error {
	// Parse bundle reference
	bundleRef, err := m.parseBundleReference(ref)
	if err != nil {
		return fmt.Errorf("invalid bundle reference: %w", err)
	}

	// Get registry
	registry, ok := m.registries[bundleRef.Registry]
	if !ok {
		return fmt.Errorf("registry not found: %s", bundleRef.Registry)
	}

	// Download bundle
	bundleData, err := registry.Download(ctx, bundleRef.Name, bundleRef.Version)
	if err != nil {
		return fmt.Errorf("failed to download bundle: %w", err)
	}

	// Extract bundle to temp location
	tempDir := filepath.Join(m.bundlesDir, ".tmp", bundleRef.Name+"-"+bundleRef.Version)
	if err := m.fs.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = m.fs.RemoveAll(tempDir) }() // Cleanup

	if err := m.extractBundle(bundleData, tempDir); err != nil {
		return fmt.Errorf("failed to extract bundle: %w", err)
	}

	// Validate bundle
	if !opts.SkipValidation {
		result, err := m.validator.Validate(m.fs, tempDir)
		if err != nil {
			return fmt.Errorf("failed to validate bundle: %w", err)
		}
		if !result.Valid {
			return fmt.Errorf("bundle validation failed: %d issues found", len(result.Issues))
		}
	}

	// Install bundle to bundles directory
	bundleDir := filepath.Join(m.bundlesDir, bundleRef.Name)
	if err := m.fs.MkdirAll(bundleDir, 0755); err != nil {
		return fmt.Errorf("failed to create bundle directory: %w", err)
	}

	// Copy bundle files
	if err := m.copyDir(tempDir, bundleDir); err != nil {
		return fmt.Errorf("failed to install bundle: %w", err)
	}

	// Create installed bundle record
	installedBundle := bundle.InstalledBundle{
		BundleReference: *bundleRef,
		LocalPath:       bundleDir,
		InstallTime:     time.Now(),
		SourceURL:       fmt.Sprintf("%s/%s@%s", bundleRef.Registry, bundleRef.Name, bundleRef.Version),
	}

	if err := m.saveInstalledBundleRecord(installedBundle); err != nil {
		return fmt.Errorf("failed to save installation record: %w", err)
	}

	return nil
}

// RenderToEnvironment renders an installed bundle to a specific environment configuration
func (m *Manager) RenderToEnvironment(ctx context.Context, bundleName, environment string, variables map[string]interface{}) error {
	// Get installed bundle
	installedBundle, err := m.GetInstalled(bundleName)
	if err != nil {
		return fmt.Errorf("bundle not installed: %w", err)
	}

	// Read bundle manifest
	manifestPath := filepath.Join(installedBundle.LocalPath, "manifest.json")
	manifestData, err := afero.ReadFile(m.fs, manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest bundle.BundleManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Read template
	templatePath := filepath.Join(installedBundle.LocalPath, "template.json")
	templateData, err := afero.ReadFile(m.fs, templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	// Process template with variables
	renderedTemplate, err := m.processTemplate(string(templateData), variables)
	if err != nil {
		return fmt.Errorf("failed to process template: %w", err)
	}

	// Ensure environment directory exists
	envDir := filepath.Join(m.configDir, "environments", environment)
	if err := m.fs.MkdirAll(envDir, 0755); err != nil {
		return fmt.Errorf("failed to create environment directory: %w", err)
	}

	// Write rendered config to environment directory
	configPath := filepath.Join(envDir, bundleName+".json")
	if err := afero.WriteFile(m.fs, configPath, []byte(renderedTemplate), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Update variables.yml if needed
	if len(variables) > 0 {
		if err := m.updateEnvironmentVariables(environment, variables); err != nil {
			return fmt.Errorf("failed to update environment variables: %w", err)
		}
	}

	return nil
}

// List returns all available bundles from all registries
func (m *Manager) List(ctx context.Context, opts bundle.ListOptions) ([]bundle.BundleManifest, error) {
	var allBundles []bundle.BundleManifest

	for registryName, registry := range m.registries {
		// Filter by registry if specified
		if opts.Registry != "" && opts.Registry != registryName {
			continue
		}

		bundles, err := registry.List(ctx, opts)
		if err != nil {
			// Don't fail entire listing for one registry error
			continue
		}

		allBundles = append(allBundles, bundles...)
	}

	return allBundles, nil
}

// ListInstalled returns all locally installed bundles
func (m *Manager) ListInstalled() ([]bundle.InstalledBundle, error) {
	installedPath := filepath.Join(m.bundlesDir, "installed.json")

	exists, err := afero.Exists(m.fs, installedPath)
	if err != nil {
		return nil, err
	}
	if !exists {
		return []bundle.InstalledBundle{}, nil
	}

	data, err := afero.ReadFile(m.fs, installedPath)
	if err != nil {
		return nil, err
	}

	var installed []bundle.InstalledBundle
	if err := json.Unmarshal(data, &installed); err != nil {
		return nil, err
	}

	return installed, nil
}

// GetInstalled returns a specific installed bundle
func (m *Manager) GetInstalled(name string) (*bundle.InstalledBundle, error) {
	installed, err := m.ListInstalled()
	if err != nil {
		return nil, err
	}

	for _, bundle := range installed {
		if bundle.Name == name {
			return &bundle, nil
		}
	}

	return nil, fmt.Errorf("bundle not found: %s", name)
}

// Remove removes an installed bundle and its rendered environment configs
func (m *Manager) Remove(name string) error {
	// Get installed bundle
	installedBundle, err := m.GetInstalled(name)
	if err != nil {
		return fmt.Errorf("bundle not installed: %w", err)
	}

	// Remove rendered configs from all environments
	if err := m.removeRenderedConfigs(name); err != nil {
		return fmt.Errorf("failed to remove environment configs: %w", err)
	}

	// Remove bundle directory
	if err := m.fs.RemoveAll(installedBundle.LocalPath); err != nil {
		return fmt.Errorf("failed to remove bundle files: %w", err)
	}

	// Update installed bundles list
	installed, err := m.ListInstalled()
	if err != nil {
		return fmt.Errorf("failed to read installed bundles: %w", err)
	}

	var filtered []bundle.InstalledBundle
	for _, bundle := range installed {
		if bundle.Name != name {
			filtered = append(filtered, bundle)
		}
	}

	return m.saveInstalledBundleList(filtered)
}

// AddRegistry adds a bundle registry
func (m *Manager) AddRegistry(name string, registry bundle.BundleRegistry) {
	m.registries[name] = registry
}

// RemoveRegistry removes a bundle registry
func (m *Manager) RemoveRegistry(name string) {
	delete(m.registries, name)
}

// Helper methods

func (m *Manager) parseBundleReference(ref string) (*bundle.BundleReference, error) {
	// Parse references like: registry/name@version or name@version
	var registry, name, version string

	// Check for registry prefix
	if strings.Contains(ref, "/") {
		parts := strings.SplitN(ref, "/", 2)
		registry = parts[0]
		ref = parts[1]
	} else {
		registry = "default"
	}

	// Check for version suffix
	if strings.Contains(ref, "@") {
		parts := strings.SplitN(ref, "@", 2)
		name = parts[0]
		version = parts[1]
	} else {
		name = ref
		version = "" // Latest
	}

	return &bundle.BundleReference{
		Registry: registry,
		Name:     name,
		Version:  version,
	}, nil
}

func (m *Manager) extractBundle(data []byte, targetDir string) error {
	// Create gzip reader
	gzReader, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	defer func() { _ = gzReader.Close() }()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		targetPath := filepath.Join(targetDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := m.fs.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := m.fs.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}

			file, err := m.fs.Create(targetPath)
			if err != nil {
				return err
			}

			_, err = io.Copy(file, tarReader)
			file.Close()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *Manager) copyDir(src, dst string) error {
	return afero.Walk(m.fs, src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return m.fs.MkdirAll(dstPath, info.Mode())
		} else {
			return m.copyFile(path, dstPath)
		}
	})
}

func (m *Manager) copyFile(src, dst string) error {
	srcFile, err := m.fs.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	if err := m.fs.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := m.fs.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func (m *Manager) processTemplate(templateContent string, variables map[string]interface{}) (string, error) {
	// Use Go template engine for proper template processing
	tmpl, err := template.New("bundle").Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var result strings.Builder
	err = tmpl.Execute(&result, variables)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return result.String(), nil
}

func (m *Manager) updateEnvironmentVariables(environment string, variables map[string]interface{}) error {
	variablesPath := filepath.Join(m.configDir, "environments", environment, "variables.yml")

	// Read existing variables
	var existingVars map[string]interface{}
	if exists, _ := afero.Exists(m.fs, variablesPath); exists {
		data, err := afero.ReadFile(m.fs, variablesPath)
		if err == nil {
			_ = yaml.Unmarshal(data, &existingVars)
		}
	}

	if existingVars == nil {
		existingVars = make(map[string]interface{})
	}

	// Merge variables
	for key, value := range variables {
		existingVars[key] = value
	}

	// Write back to file
	data, err := yaml.Marshal(existingVars)
	if err != nil {
		return err
	}

	return afero.WriteFile(m.fs, variablesPath, data, 0644)
}

func (m *Manager) saveInstalledBundleRecord(installedBundle bundle.InstalledBundle) error {
	installed, err := m.ListInstalled()
	if err != nil {
		return err
	}

	// Update existing or add new
	found := false
	for i, existing := range installed {
		if existing.Name == installedBundle.Name {
			installed[i] = installedBundle
			found = true
			break
		}
	}

	if !found {
		installed = append(installed, installedBundle)
	}

	return m.saveInstalledBundleList(installed)
}

func (m *Manager) saveInstalledBundleList(installed []bundle.InstalledBundle) error {
	installedPath := filepath.Join(m.bundlesDir, "installed.json")

	if err := m.fs.MkdirAll(m.bundlesDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(installed, "", "  ")
	if err != nil {
		return err
	}

	return afero.WriteFile(m.fs, installedPath, data, 0644)
}

// removeRenderedConfigs removes rendered config files from all environments
func (m *Manager) removeRenderedConfigs(bundleName string) error {
	environmentsDir := filepath.Join(m.configDir, "environments")

	// Check if environments directory exists
	exists, err := afero.DirExists(m.fs, environmentsDir)
	if err != nil || !exists {
		return nil // No environments to clean up
	}

	// List all environment directories
	envs, err := afero.ReadDir(m.fs, environmentsDir)
	if err != nil {
		return fmt.Errorf("failed to read environments directory: %w", err)
	}

	// Remove config file from each environment
	for _, env := range envs {
		if !env.IsDir() {
			continue
		}

		configFile := filepath.Join(environmentsDir, env.Name(), bundleName+".json")
		exists, err := afero.Exists(m.fs, configFile)
		if err != nil {
			continue // Skip on error
		}
		if exists {
			if err := m.fs.Remove(configFile); err != nil {
				// Log error but continue with other environments
				fmt.Printf("Warning: failed to remove %s: %v\n", configFile, err)
			}
		}
	}

	return nil
}
