package bundle

import (
	"time"
)

// BundleManifest contains metadata about a bundle
type BundleManifest struct {
	Name              string                  `json:"name" yaml:"name"`
	Version           string                  `json:"version" yaml:"version"`
	Description       string                  `json:"description" yaml:"description"`
	Author            string                  `json:"author" yaml:"author"`
	License           string                  `json:"license,omitempty" yaml:"license,omitempty"`
	Repository        string                  `json:"repository,omitempty" yaml:"repository,omitempty"`
	StationVersion    string                  `json:"station_version" yaml:"station_version"`
	CreatedAt         time.Time               `json:"created_at" yaml:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
	Tags              []string                `json:"tags,omitempty" yaml:"tags,omitempty"`
	RequiredVariables map[string]VariableSpec `json:"required_variables,omitempty" yaml:"required_variables,omitempty"`
	Dependencies      map[string]string       `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	ToolsCount        int                     `json:"tools_count,omitempty" yaml:"tools_count,omitempty"`
	DownloadCount     int64                   `json:"download_count,omitempty" yaml:"download_count,omitempty"`
	Checksum          string                  `json:"checksum,omitempty" yaml:"checksum,omitempty"`
	SizeBytes         int64                   `json:"size_bytes,omitempty" yaml:"size_bytes,omitempty"`
}

// VariableSpec defines the specification for a bundle variable
type VariableSpec struct {
	Type        string      `json:"type" yaml:"type"` // string, number, boolean, array
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
	Default     interface{} `json:"default,omitempty" yaml:"default,omitempty"`
	Required    bool        `json:"required" yaml:"required"`
	Secret      bool        `json:"secret,omitempty" yaml:"secret,omitempty"`
	Enum        []string    `json:"enum,omitempty" yaml:"enum,omitempty"`
	Validation  string      `json:"validation,omitempty" yaml:"validation,omitempty"` // regex pattern
}

// Bundle represents a complete bundle with all its components
type Bundle struct {
	Manifest        BundleManifest         `json:"manifest"`
	Template        map[string]interface{} `json:"template"`         // MCP template JSON
	VariablesSchema map[string]interface{} `json:"variables_schema"` // JSON schema for variables
	README          string                 `json:"readme,omitempty"`
	Examples        map[string]string      `json:"examples,omitempty"` // filename -> content
	Tests           map[string]interface{} `json:"tests,omitempty"`    // test configurations
}

// BundleReference represents a reference to a bundle (name@version or registry/name@version)
type BundleReference struct {
	Registry string `json:"registry,omitempty"` // empty means default registry
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"` // empty means latest
}

// String returns the string representation of a bundle reference
func (br BundleReference) String() string {
	ref := br.Name
	if br.Registry != "" && br.Registry != "default" {
		ref = br.Registry + "/" + ref
	}
	if br.Version != "" {
		ref = ref + "@" + br.Version
	}
	return ref
}

// InstalledBundle represents a bundle that has been installed locally
type InstalledBundle struct {
	BundleReference
	LocalPath      string    `json:"local_path"`
	InstallTime    time.Time `json:"install_time"`
	LastUsed       time.Time `json:"last_used,omitempty"`
	SourceURL      string    `json:"source_url,omitempty"`
	SourceChecksum string    `json:"source_checksum,omitempty"`
}

// CreateOptions contains options for creating a new bundle
type CreateOptions struct {
	Name         string                  `json:"name"`
	Author       string                  `json:"author"`
	Description  string                  `json:"description"`
	License      string                  `json:"license,omitempty"`
	Repository   string                  `json:"repository,omitempty"`
	Tags         []string                `json:"tags,omitempty"`
	Variables    map[string]VariableSpec `json:"variables,omitempty"`
	Dependencies map[string]string       `json:"dependencies,omitempty"`
}

// ValidationResult represents the result of bundle validation
type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Issues   []ValidationIssue `json:"issues,omitempty"`
	Warnings []ValidationIssue `json:"warnings,omitempty"`
}

// ValidationIssue represents a validation problem
type ValidationIssue struct {
	Type       string `json:"type"` // e.g., "missing_file", "invalid_json", "schema_violation"
	File       string `json:"file,omitempty"`
	Field      string `json:"field,omitempty"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

// RegistryConfig contains configuration for a bundle registry
type RegistryConfig struct {
	Name    string            `json:"name" yaml:"name"`
	Type    string            `json:"type" yaml:"type"` // "http", "s3", "local"
	URL     string            `json:"url,omitempty" yaml:"url,omitempty"`
	Bucket  string            `json:"bucket,omitempty" yaml:"bucket,omitempty"`
	Region  string            `json:"region,omitempty" yaml:"region,omitempty"`
	Path    string            `json:"path,omitempty" yaml:"path,omitempty"`
	Prefix  string            `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	Auth    map[string]string `json:"auth,omitempty" yaml:"auth,omitempty"`
	Default bool              `json:"default,omitempty" yaml:"default,omitempty"`
}

// BundleConfig contains configuration for the bundle system
type BundleConfig struct {
	CacheDir        string                    `json:"cache_dir" yaml:"cache_dir"`
	Registries      map[string]RegistryConfig `json:"registries" yaml:"registries"`
	DefaultRegistry string                    `json:"default_registry,omitempty" yaml:"default_registry,omitempty"`
}

// ListOptions contains options for listing bundles
type ListOptions struct {
	Registry string   `json:"registry,omitempty"` // filter by specific registry
	Tags     []string `json:"tags,omitempty"`     // filter by tags
	Search   string   `json:"search,omitempty"`   // search term
}

// InstallOptions contains options for installing bundles
type InstallOptions struct {
	Force          bool   `json:"force,omitempty"`           // force reinstall
	Version        string `json:"version,omitempty"`         // specific version
	Registry       string `json:"registry,omitempty"`        // specific registry
	SkipValidation bool   `json:"skip_validation,omitempty"` // skip validation
}

// PublishOptions contains options for publishing bundles
type PublishOptions struct {
	Registry string            `json:"registry,omitempty"` // target registry
	Tags     map[string]string `json:"tags,omitempty"`     // additional tags
	Force    bool              `json:"force,omitempty"`    // overwrite existing
}

// PackageResult contains the result of bundle packaging
type PackageResult struct {
	Success          bool              `json:"success"`
	OutputPath       string            `json:"output_path,omitempty"`
	Size             int64             `json:"size,omitempty"`
	ValidationResult *ValidationResult `json:"validation_result,omitempty"`
	Error            string            `json:"error,omitempty"`
}
