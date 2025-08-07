package bundle

import (
	"context"
	"io"

	"github.com/spf13/afero"
)

// BundleRegistry defines the interface for bundle registries
type BundleRegistry interface {
	// List returns all available bundles in the registry
	List(ctx context.Context, opts ListOptions) ([]BundleManifest, error)
	
	// Get returns metadata for a specific bundle
	Get(ctx context.Context, name, version string) (*BundleManifest, error)
	
	// Download downloads a bundle and returns the zip file content
	Download(ctx context.Context, name, version string) ([]byte, error)
	
	// GetVersions returns all available versions for a bundle
	GetVersions(ctx context.Context, name string) ([]string, error)
}

// BundleUploader defines the interface for registries that support uploads
type BundleUploader interface {
	BundleRegistry
	
	// Upload uploads a bundle to the registry
	Upload(ctx context.Context, bundleData []byte, manifest BundleManifest) error
	
	// Delete removes a bundle from the registry
	Delete(ctx context.Context, name, version string) error
}

// BundleCreator defines the interface for creating new bundles
type BundleCreator interface {
	// Create creates a new bundle with the given options
	Create(fs afero.Fs, bundlePath string, opts CreateOptions) error
}

// BundleValidator defines the interface for validating bundles
type BundleValidator interface {
	// Validate validates a bundle and returns any issues
	Validate(fs afero.Fs, bundlePath string) (*ValidationResult, error)
}

// BundlePackager defines the interface for packaging bundles
type BundlePackager interface {
	// Package creates a zip file from a bundle directory
	Package(fs afero.Fs, bundlePath string) ([]byte, error)
	
	// Unpackage extracts a bundle from zip data
	Unpackage(zipData []byte, targetFS afero.Fs, targetPath string) error
}

// BundleManager defines the main interface for managing bundles
type BundleManager interface {
	// Bundle creation and management
	Create(bundlePath string, opts CreateOptions) error
	Validate(bundlePath string) (*ValidationResult, error)
	Package(bundlePath string) (string, error) // returns path to zip file
	
	// Registry operations
	List(opts ListOptions) ([]BundleManifest, error)
	Install(ref string, opts InstallOptions) error
	Update(name string) error
	Remove(name string) error
	Publish(bundlePath string, opts PublishOptions) error
	
	// Local bundle management
	ListInstalled() ([]InstalledBundle, error)
	GetInstalled(name string) (*InstalledBundle, error)
	
	// Registry management
	AddRegistry(name string, config RegistryConfig) error
	RemoveRegistry(name string) error
	ListRegistries() (map[string]RegistryConfig, error)
}

// VariableResolver defines the interface for resolving template variables
type VariableResolver interface {
	// ResolveVariables resolves variables using the configured hierarchy
	ResolveVariables(ctx context.Context, bundleSchema *Bundle, environment string, templateVars []string) (*VariableResult, error)
	
	// SaveVariables saves variables to the environment configuration
	SaveVariables(environment string, variables map[string]interface{}) error
}

// VariableResult contains the result of variable resolution
type VariableResult struct {
	Resolved         map[string]interface{} `json:"resolved"`
	Missing          []string               `json:"missing,omitempty"`
	PromptsRequired  []VariablePrompt       `json:"prompts_required,omitempty"`
	Source           map[string]string      `json:"source,omitempty"` // variable -> source (bundle, env, system, prompt)
}

// VariablePrompt represents a variable that needs user input
type VariablePrompt struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Secret      bool        `json:"secret,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
	Required    bool        `json:"required"`
}

// BundleLoader defines the interface for loading bundles for MCP sync
type BundleLoader interface {
	// LoadBundle loads a bundle and prepares it for MCP processing
	LoadBundle(ctx context.Context, bundleRef BundleReference, environment string) (*Bundle, *VariableResult, error)
	
	// ProcessTemplate processes a bundle template with resolved variables
	ProcessTemplate(bundle *Bundle, variables map[string]interface{}) (map[string]interface{}, error)
}

// FileSystemProvider provides abstracted file system access
type FileSystemProvider interface {
	// GetFS returns a file system for the given type and configuration
	GetFS(fsType string, config map[string]interface{}) (afero.Fs, error)
	
	// CreateTempFS creates a temporary file system
	CreateTempFS() (afero.Fs, func(), error) // fs, cleanup, error
}

// HTTPClient defines the interface for HTTP operations (for testing)
type HTTPClient interface {
	Do(req interface{}) (io.ReadCloser, error)
}

// ProgressReporter defines the interface for reporting operation progress
type ProgressReporter interface {
	Start(operation string, total int64)
	Update(current int64)
	Finish()
	Error(err error)
}