package agent_bundle

import (
	"context"
)

// AgentBundleCreator handles creation and export of agent bundles
type AgentBundleCreator interface {
	// Create creates a new agent bundle from scratch
	Create(path string, opts CreateOptions) error
	
	// ExportFromAgent exports an existing agent to a bundle
	ExportFromAgent(agentID int64, path string, opts ExportOptions) error
	
	// AnalyzeDependencies analyzes the dependencies for an agent
	AnalyzeDependencies(agentID int64) (*DependencyAnalysis, error)
	
	// GenerateScaffolding creates the basic bundle structure
	GenerateScaffolding(path string, manifest *AgentBundleManifest) error
}

// AgentBundleValidator handles validation of agent bundles
type AgentBundleValidator interface {
	// Validate performs comprehensive validation of an agent bundle
	Validate(bundlePath string) (*ValidationResult, error)
	
	// ValidateManifest validates the bundle manifest
	ValidateManifest(manifest *AgentBundleManifest) error
	
	// ValidateAgentConfig validates the agent configuration
	ValidateAgentConfig(config *AgentTemplateConfig) error
	
	// ValidateToolMappings ensures tool requirements can be satisfied by MCP dependencies
	ValidateToolMappings(tools []ToolRequirement, mcpBundles []MCPBundleDependency) error
	
	// ValidateDependencies checks that all MCP bundle dependencies are valid and accessible
	ValidateDependencies(dependencies []MCPBundleDependency) error
	
	// ValidateVariables ensures variable definitions are consistent and valid
	ValidateVariables(variables map[string]VariableSpec, templates []string) error
}

// AgentBundleManager handles installation, updates, and lifecycle management of agent bundles
type AgentBundleManager interface {
	// Install installs an agent bundle in the specified environment
	Install(bundlePath string, environment string, vars map[string]interface{}) (*InstallResult, error)
	
	// Duplicate duplicates an existing agent to a new environment with different variables
	Duplicate(agentID int64, targetEnv string, opts DuplicateOptions) (*InstallResult, error)
	
	// Update updates an existing agent from a bundle
	Update(bundlePath string, agentID int64, opts UpdateOptions) error
	
	// Remove removes an agent and optionally cleans up orphaned dependencies
	Remove(agentID int64, opts RemoveOptions) error
	
	// List lists all agent bundles and their status
	List() ([]AgentBundle, error)
	
	// GetStatus gets the status of an installed agent bundle
	GetStatus(agentID int64) (*AgentBundleStatus, error)
}

// DependencyResolver handles MCP bundle dependency resolution and installation
type DependencyResolver interface {
	// Resolve resolves all dependencies for a set of MCP bundle requirements
	Resolve(ctx context.Context, dependencies []MCPBundleDependency, environment string) (*ResolutionResult, error)
	
	// InstallMCPBundles installs the required MCP template bundles
	InstallMCPBundles(ctx context.Context, bundles []MCPBundleRef, environment string) error
	
	// ValidateToolAvailability validates that required tools are available in the environment
	ValidateToolAvailability(ctx context.Context, tools []ToolRequirement, environment string) error
	
	// ResolveConflicts resolves conflicts between different bundle requirements
	ResolveConflicts(conflicts []ToolConflict) (*ConflictResolution, error)
}

// VariableResolver handles multi-source variable resolution for agent templates
type VariableResolver interface {
	// ResolveForAgent resolves variables for agent configuration templates
	ResolveForAgent(templateID int64, clientID string, environment string) (map[string]interface{}, error)
	
	// ResolveForMCP resolves variables for MCP configuration templates
	ResolveForMCP(templateID int64, clientID string, environment string) (map[string]interface{}, error)
	
	// ValidateVariables validates that all required variables can be resolved
	ValidateVariables(requirements map[string]VariableSpec, templateID int64, clientID string, environment string) error
	
	// GetVariableSources returns information about where variables are sourced from
	GetVariableSources(variableNames []string, templateID int64, clientID string, environment string) (map[string]string, error)
}

// Multi-Tenant Template Interfaces

// TemplateSyncService handles template updates and client synchronization
type TemplateSyncService interface {
	// UpdateTemplate updates a template and propagates changes to all client instances
	UpdateTemplate(templateID int64, updates AgentTemplateUpdate) error
	
	// AddClient adds a new client instance to a template
	AddClient(templateID int64, clientID string, variables map[string]string) (*InstallResult, error)
	
	// RemoveClient removes a client instance and cleans up resources
	RemoveClient(templateID int64, clientID string) error
	
	// ListClients lists all client instances for a template
	ListClients(templateID int64) ([]ClientInstance, error)
	
	// SyncClient synchronizes a specific client with the latest template version
	SyncClient(templateID int64, clientID string) error
	
	// GetSyncStatus gets the synchronization status of all clients
	GetSyncStatus(templateID int64) (*SyncStatus, error)
}

// ClientVariableService handles encrypted client-specific variables
type ClientVariableService interface {
	// SetClientVariable sets an encrypted variable for a client
	SetClientVariable(templateID int64, clientID string, name, value string, varType VariableType) error
	
	// GetClientVariables retrieves and decrypts all variables for a client
	GetClientVariables(templateID int64, clientID string) (map[string]string, error)
	
	// GetClientVariable retrieves and decrypts a specific variable for a client
	GetClientVariable(templateID int64, clientID string, name string) (string, error)
	
	// DeleteClientVariable deletes a variable for a client
	DeleteClientVariable(templateID int64, clientID string, name string) error
	
	// ListVariableNames lists all variable names for a client (without values)
	ListVariableNames(templateID int64, clientID string) ([]string, error)
	
	// ValidateClientVariables validates that all required variables are set for a client
	ValidateClientVariables(templateID int64, clientID string, requirements map[string]VariableSpec) error
}

// AgentBundlePackager handles packaging of agent bundles (reuses template bundle infrastructure)
type AgentBundlePackager interface {
	// Package packages an agent bundle into a distributable archive
	Package(bundlePath string, outputPath string, validateFirst bool) (*PackageResult, error)
	
	// Extract extracts a packaged agent bundle
	Extract(packagePath string, outputPath string) error
	
	// Verify verifies the integrity of a packaged bundle
	Verify(packagePath string) error
}

// Supporting Types for Interfaces

// UpdateOptions defines options for updating agents
type UpdateOptions struct {
	PreserveCustomizations bool
	BackupBeforeUpdate    bool
	DryRun               bool
	ForceUpdate          bool
}

// RemoveOptions defines options for removing agents
type RemoveOptions struct {
	CleanupDependencies bool
	Force              bool
	BackupBeforeRemove bool
}

// AgentBundleStatus represents the status of an installed agent bundle
type AgentBundleStatus struct {
	AgentID         int64                  `json:"agent_id"`
	BundleName      string                 `json:"bundle_name"`
	Version         string                 `json:"version"`
	Environment     string                 `json:"environment"`
	InstallDate     string                 `json:"install_date"`
	LastUpdate      string                 `json:"last_update"`
	Status          string                 `json:"status"`
	HealthCheck     BundleHealthCheck      `json:"health_check"`
	Dependencies    []DependencyStatus     `json:"dependencies"`
}

// BundleHealthCheck represents health check information
type BundleHealthCheck struct {
	Healthy           bool     `json:"healthy"`
	ToolsAvailable    int      `json:"tools_available"`
	ToolsMissing      []string `json:"tools_missing"`
	MCPServersRunning int      `json:"mcp_servers_running"`
	MCPServersFailed  []string `json:"mcp_servers_failed"`
	LastCheck         string   `json:"last_check"`
}

// DependencyStatus represents the status of a dependency
type DependencyStatus struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Status    string `json:"status"` // "installed", "missing", "outdated", "failed"
	Source    string `json:"source"`
}

// ResolutionResult represents the result of dependency resolution
type ResolutionResult struct {
	Success         bool                  `json:"success"`
	ResolvedBundles []MCPBundleRef        `json:"resolved_bundles"`
	MissingBundles  []MCPBundleDependency `json:"missing_bundles"`
	Conflicts       []ToolConflict        `json:"conflicts"`
	InstallOrder    []string              `json:"install_order"`
}

// MCPBundleRef represents a reference to a specific MCP bundle version
type MCPBundleRef struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Source    string `json:"source"`
	URL       string `json:"url,omitempty"`
	LocalPath string `json:"local_path,omitempty"`
}

// ConflictResolution represents the resolution of dependency conflicts
type ConflictResolution struct {
	Strategy    string                    `json:"strategy"`
	Resolutions map[string]string         `json:"resolutions"`
	Warnings    []string                  `json:"warnings"`
}

// VariableType represents the type of a variable for encryption and storage
type VariableType string

const (
	VariableTypeAgent  VariableType = "agent"  // Agent-specific variables (encrypted in database)
	VariableTypeMCP    VariableType = "mcp"    // MCP configuration variables (can be database or file)
	VariableTypeSystem VariableType = "system" // System/environment variables (file-based)
)

// ClientInstance represents a client instance of a template
type ClientInstance struct {
	ClientID        string                 `json:"client_id"`
	AgentID         int64                  `json:"agent_id"`
	AgentName       string                 `json:"agent_name"`
	Environment     string                 `json:"environment"`
	Variables       map[string]string      `json:"variables"` // Encrypted values not included
	VariableNames   []string               `json:"variable_names"`
	LastSync        string                 `json:"last_sync"`
	Status          string                 `json:"status"`
	Version         string                 `json:"version"`
}

// SyncStatus represents the synchronization status of template clients
type SyncStatus struct {
	TemplateID       int64                    `json:"template_id"`
	TemplateName     string                   `json:"template_name"`
	TemplateVersion  string                   `json:"template_version"`
	TotalClients     int                      `json:"total_clients"`
	SyncedClients    int                      `json:"synced_clients"`
	OutdatedClients  int                      `json:"outdated_clients"`
	FailedClients    int                      `json:"failed_clients"`
	LastUpdate       string                   `json:"last_update"`
	ClientDetails    []ClientSyncStatus       `json:"client_details"`
}

// ClientSyncStatus represents the sync status of a specific client
type ClientSyncStatus struct {
	ClientID      string `json:"client_id"`
	AgentID       int64  `json:"agent_id"`
	Status        string `json:"status"`        // "synced", "outdated", "failed"
	Version       string `json:"version"`
	LastSync      string `json:"last_sync"`
	Error         string `json:"error,omitempty"`
}