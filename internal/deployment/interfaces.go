package deployment

import (
	"context"
	"fmt"
	"strings"
)

// DeploymentTarget defines the interface for deployment targets
type DeploymentTarget interface {
	// Name returns the target identifier (e.g., "kubernetes", "nomad", "ansible")
	Name() string

	// Validate checks if the target can be used (CLI tools installed, etc.)
	Validate(ctx context.Context) error

	// GenerateConfig generates deployment configuration files
	// Returns a map of filename -> content
	GenerateConfig(ctx context.Context, config *DeploymentConfig, secrets map[string]string) (map[string]string, error)

	// Deploy executes the deployment
	Deploy(ctx context.Context, config *DeploymentConfig, secrets map[string]string, options DeployOptions) error

	// Destroy tears down the deployment
	Destroy(ctx context.Context, config *DeploymentConfig) error

	// Status returns the current deployment status
	Status(ctx context.Context, config *DeploymentConfig) (*DeploymentStatus, error)
}

// DeployOptions contains options for deployment
type DeployOptions struct {
	DryRun       bool              // Generate configs only, don't deploy
	OutputDir    string            // Directory to write generated configs
	ExtraVars    map[string]string // Additional variables for the deployment
	Namespace    string            // Kubernetes namespace, Nomad namespace, etc.
	Context      string            // Kubernetes context, etc.
	Region       string            // Cloud region
	AutoApprove  bool              // Skip confirmation prompts
	AlwaysOn     bool              // Disable auto-scaling to zero
	Replicas     int               // Number of replicas (default: 1)
	ResourceSize string            // Resource tier: small, medium, large
}

// DeploymentStatus represents the current state of a deployment
type DeploymentStatus struct {
	State       string            // running, stopped, pending, failed, unknown
	Replicas    int               // Current number of running replicas
	Endpoints   []string          // Public endpoints
	InternalURL string            // Internal service URL
	Message     string            // Status message
	Metadata    map[string]string // Target-specific metadata
}

// SecretProvider defines the interface for secret stores
type SecretProvider interface {
	// Name returns the provider identifier (e.g., "aws-secretsmanager", "vault")
	Name() string

	// Validate checks if the provider can be used (credentials, connectivity)
	Validate(ctx context.Context) error

	// GetSecrets fetches secrets from the provider
	// path is provider-specific (e.g., "secret/station/prod" for Vault)
	GetSecrets(ctx context.Context, path string) (map[string]string, error)

	// GetSecret fetches a single secret value
	GetSecret(ctx context.Context, path string, key string) (string, error)

	// ListSecrets lists available secret paths/keys
	ListSecrets(ctx context.Context, path string) ([]string, error)
}

// SecretProviderConfig holds configuration for secret providers
type SecretProviderConfig struct {
	// Provider type: aws-secretsmanager, aws-ssm, vault, gcp-secretmanager, sops
	Provider string

	// Path to secrets (provider-specific)
	// AWS SM: secret name or ARN
	// AWS SSM: parameter path prefix (e.g., /station/prod/)
	// Vault: secret path (e.g., secret/data/station/prod)
	// GCP: secret name (e.g., projects/*/secrets/station-prod)
	// SOPS: path to encrypted file
	Path string

	// Region for cloud providers
	Region string

	// Additional provider-specific options
	Options map[string]string
}

// ParseSecretProviderURI parses a secret provider URI into config
// Formats:
//   - aws-secretsmanager://secret-name
//   - aws-secretsmanager://secret-name?region=us-east-1
//   - aws-ssm:///station/prod/
//   - aws-ssm:///station/prod/?region=us-west-2
//   - vault://secret/data/station/prod
//   - vault://secret/data/station/prod?addr=https://vault.example.com
//   - gcp-secretmanager://projects/my-project/secrets/station-prod
//   - sops://./secrets/prod.enc.yaml
func ParseSecretProviderURI(uri string) (*SecretProviderConfig, error) {
	parts := strings.SplitN(uri, "://", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid secret provider URI: %s (expected scheme://path)", uri)
	}

	provider := parts[0]
	pathAndQuery := parts[1]

	config := &SecretProviderConfig{
		Provider: provider,
		Options:  make(map[string]string),
	}

	queryIdx := strings.Index(pathAndQuery, "?")
	if queryIdx != -1 {
		config.Path = pathAndQuery[:queryIdx]
		queryStr := pathAndQuery[queryIdx+1:]

		for _, param := range strings.Split(queryStr, "&") {
			kv := strings.SplitN(param, "=", 2)
			if len(kv) == 2 {
				key, value := kv[0], kv[1]
				switch key {
				case "region":
					config.Region = value
				default:
					config.Options[key] = value
				}
			}
		}
	} else {
		config.Path = pathAndQuery
	}

	return config, nil
}

// Registry for deployment targets and secret providers
var (
	deploymentTargets = make(map[string]DeploymentTarget)
	secretProviders   = make(map[string]func() SecretProvider)
)

// RegisterDeploymentTarget registers a deployment target
func RegisterDeploymentTarget(target DeploymentTarget) {
	deploymentTargets[target.Name()] = target
}

// GetDeploymentTarget returns a deployment target by name
func GetDeploymentTarget(name string) (DeploymentTarget, bool) {
	target, ok := deploymentTargets[name]
	return target, ok
}

// ListDeploymentTargets returns all registered deployment targets
func ListDeploymentTargets() []string {
	names := make([]string, 0, len(deploymentTargets))
	for name := range deploymentTargets {
		names = append(names, name)
	}
	return names
}

// RegisterSecretProvider registers a secret provider factory
func RegisterSecretProvider(name string, factory func() SecretProvider) {
	secretProviders[name] = factory
}

// GetSecretProvider returns a new instance of a secret provider
func GetSecretProvider(name string) (SecretProvider, bool) {
	factory, ok := secretProviders[name]
	if !ok {
		return nil, false
	}
	return factory(), true
}

// ListSecretProviders returns all registered secret providers
func ListSecretProviders() []string {
	names := make([]string, 0, len(secretProviders))
	for name := range secretProviders {
		names = append(names, name)
	}
	return names
}
