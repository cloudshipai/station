package local

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/afero"

	"station/pkg/bundle"
)

// LocalRegistry implements BundleRegistry for local filesystem-based registries
type LocalRegistry struct {
	fs   afero.Fs
	path string
	name string
}

// NewLocalRegistry creates a new local registry
func NewLocalRegistry(name, path string, fs afero.Fs) *LocalRegistry {
	if fs == nil {
		fs = afero.NewOsFs()
	}
	return &LocalRegistry{
		name: name,
		path: filepath.Clean(path),
		fs:   fs,
	}
}

// List returns all available bundles in the local registry
func (r *LocalRegistry) List(ctx context.Context, opts bundle.ListOptions) ([]bundle.BundleManifest, error) {
	// Filter by registry name
	if opts.Registry != "" && opts.Registry != r.name {
		return []bundle.BundleManifest{}, nil
	}

	// Check if registry path exists
	exists, err := afero.DirExists(r.fs, r.path)
	if err != nil {
		return nil, fmt.Errorf("failed to check registry path: %w", err)
	}
	if !exists {
		return []bundle.BundleManifest{}, nil // Empty registry
	}

	// Read all bundle directories
	bundleDirs, err := afero.ReadDir(r.fs, r.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read registry directory: %w", err)
	}

	var manifests []bundle.BundleManifest

	for _, bundleDir := range bundleDirs {
		if !bundleDir.IsDir() {
			continue // Skip non-directories
		}

		bundleName := bundleDir.Name()

		// Read bundle versions
		bundlePath := filepath.Join(r.path, bundleName)
		versions, err := r.getBundleVersions(bundlePath)
		if err != nil {
			continue // Skip bundles with errors
		}

		// Get latest version manifest
		if len(versions) == 0 {
			continue
		}

		latestVersion := versions[len(versions)-1] // Sorted, so last is latest
		manifest, err := r.readManifest(bundlePath, latestVersion)
		if err != nil {
			continue // Skip invalid manifests
		}

		// Apply filters
		if !r.matchesFilters(manifest, opts) {
			continue
		}

		manifests = append(manifests, *manifest)
	}

	return manifests, nil
}

// Get returns metadata for a specific bundle
func (r *LocalRegistry) Get(ctx context.Context, name, version string) (*bundle.BundleManifest, error) {
	bundlePath := filepath.Join(r.path, name)

	// Check if bundle exists
	exists, err := afero.DirExists(r.fs, bundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to check bundle path: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("bundle not found")
	}

	// If no version specified, get latest
	if version == "" {
		versions, err := r.getBundleVersions(bundlePath)
		if err != nil {
			return nil, fmt.Errorf("failed to get bundle versions: %w", err)
		}
		if len(versions) == 0 {
			return nil, fmt.Errorf("no versions available for bundle")
		}
		version = versions[len(versions)-1] // Latest version
	}

	return r.readManifest(bundlePath, version)
}

// Download returns the bundle archive data
func (r *LocalRegistry) Download(ctx context.Context, name, version string) ([]byte, error) {
	bundlePath := filepath.Join(r.path, name)

	// If no version specified, get latest
	if version == "" {
		versions, err := r.getBundleVersions(bundlePath)
		if err != nil {
			return nil, fmt.Errorf("failed to get bundle versions: %w", err)
		}
		if len(versions) == 0 {
			return nil, fmt.Errorf("no versions available for bundle")
		}
		version = versions[len(versions)-1]
	}

	// Look for packaged archive first
	archivePath := filepath.Join(bundlePath, version, name+"-"+version+".tar.gz")
	if exists, _ := afero.Exists(r.fs, archivePath); exists {
		return afero.ReadFile(r.fs, archivePath)
	}

	// If no archive, we could package on-demand, but for now return error
	return nil, fmt.Errorf("bundle archive not found")
}

// GetVersions returns all available versions for a bundle
func (r *LocalRegistry) GetVersions(ctx context.Context, name string) ([]string, error) {
	bundlePath := filepath.Join(r.path, name)

	// Check if bundle exists
	exists, err := afero.DirExists(r.fs, bundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to check bundle path: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("bundle not found")
	}

	return r.getBundleVersions(bundlePath)
}

// getBundleVersions returns sorted versions for a bundle
func (r *LocalRegistry) getBundleVersions(bundlePath string) ([]string, error) {
	entries, err := afero.ReadDir(r.fs, bundlePath)
	if err != nil {
		return nil, err
	}

	var versions []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Check if this looks like a version directory by checking for manifest
			manifestPath := filepath.Join(bundlePath, entry.Name(), "manifest.json")
			if exists, _ := afero.Exists(r.fs, manifestPath); exists {
				versions = append(versions, entry.Name())
			}
		}
	}

	// Sort versions (simple string sort for now, could use semantic versioning)
	sort.Strings(versions)
	return versions, nil
}

// readManifest reads a manifest.json file for a specific version
func (r *LocalRegistry) readManifest(bundlePath, version string) (*bundle.BundleManifest, error) {
	manifestPath := filepath.Join(bundlePath, version, "manifest.json")

	exists, err := afero.Exists(r.fs, manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check manifest file: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("manifest file not found")
	}

	data, err := afero.ReadFile(r.fs, manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest bundle.BundleManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

// matchesFilters checks if a manifest matches the given filters
func (r *LocalRegistry) matchesFilters(manifest *bundle.BundleManifest, opts bundle.ListOptions) bool {
	// Search filter
	if opts.Search != "" {
		search := strings.ToLower(opts.Search)
		if !strings.Contains(strings.ToLower(manifest.Name), search) &&
			!strings.Contains(strings.ToLower(manifest.Description), search) {
			return false
		}
	}

	// Tags filter
	if len(opts.Tags) > 0 {
		manifestTags := make(map[string]bool)
		for _, tag := range manifest.Tags {
			manifestTags[strings.ToLower(tag)] = true
		}

		// Check if all requested tags are present
		for _, reqTag := range opts.Tags {
			if !manifestTags[strings.ToLower(reqTag)] {
				return false
			}
		}
	}

	return true
}
