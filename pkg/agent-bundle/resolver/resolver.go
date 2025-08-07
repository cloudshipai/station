package resolver

import (
	"context"
	"fmt"
	"strings"

	agent_bundle "station/pkg/agent-bundle"
)

// MCPToolRepository provides access to available MCP tools and servers
type MCPToolRepository interface {
	ListAvailableTools(environment string) ([]*ToolInfo, error)
	GetServerInfo(serverName, environment string) (*ServerInfo, error)
	IsServerRunning(serverName, environment string) (bool, error)
}

// BundleRegistry provides access to MCP bundle registry
type BundleRegistry interface {
	GetBundle(name, version string) (*BundleInfo, error)
	ListVersions(name string) ([]string, error)
	IsAvailable(name, version string) (bool, error)
}

// ToolInfo represents information about an available MCP tool
type ToolInfo struct {
	Name        string
	ServerName  string
	Description string
	Available   bool
}

// ServerInfo represents information about an MCP server
type ServerInfo struct {
	Name        string
	Status      string // "running", "stopped", "error"
	BundleName  string
	Version     string
}

// BundleInfo represents information about an MCP bundle
type BundleInfo struct {
	Name        string
	Version     string
	Available   bool
	Source      string
	Tools       []string
}

// Resolver implements the DependencyResolver interface
type Resolver struct {
	toolRepo     MCPToolRepository
	bundleRegistry BundleRegistry
}

// New creates a new dependency resolver
func New(toolRepo MCPToolRepository, bundleRegistry BundleRegistry) *Resolver {
	return &Resolver{
		toolRepo:     toolRepo,
		bundleRegistry: bundleRegistry,
	}
}

// Resolve resolves all dependencies for a set of MCP bundle requirements
func (r *Resolver) Resolve(ctx context.Context, dependencies []agent_bundle.MCPBundleDependency, environment string) (*agent_bundle.ResolutionResult, error) {
	result := &agent_bundle.ResolutionResult{
		Success:         true,
		ResolvedBundles: []agent_bundle.MCPBundleRef{},
		MissingBundles:  []agent_bundle.MCPBundleDependency{},
		Conflicts:       []agent_bundle.ToolConflict{},
		InstallOrder:    []string{},
	}

	// Track which bundles provide which tools to detect conflicts
	toolProviders := make(map[string][]string) // tool -> []bundle
	bundleMap := make(map[string]agent_bundle.MCPBundleRef)

	for _, dep := range dependencies {
		// Check if bundle is available in registry
		available, err := r.bundleRegistry.IsAvailable(dep.Name, dep.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to check bundle availability: %w", err)
		}

		if !available {
			result.Success = false
			result.MissingBundles = append(result.MissingBundles, dep)
			continue
		}

		// Get bundle information
		bundleInfo, err := r.bundleRegistry.GetBundle(dep.Name, dep.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to get bundle info: %w", err)
		}

		// Create bundle reference
		bundleRef := agent_bundle.MCPBundleRef{
			Name:    dep.Name,
			Version: bundleInfo.Version,
			Source:  bundleInfo.Source,
		}

		bundleMap[dep.Name] = bundleRef

		// Track tool conflicts
		for _, tool := range bundleInfo.Tools {
			if providers, exists := toolProviders[tool]; exists {
				// Tool conflict detected
				conflict := agent_bundle.ToolConflict{
					ToolName:           tool,
					ConflictingBundles: append(providers, dep.Name),
					Resolution:         "auto", // Will be determined by conflict resolver
				}
				result.Conflicts = append(result.Conflicts, conflict)
			} else {
				toolProviders[tool] = []string{dep.Name}
			}
		}
	}

	// Determine install order (simple dependency sorting)
	installOrder := r.determineInstallOrder(dependencies)
	result.InstallOrder = installOrder

	// Build resolved bundles list
	for _, bundleName := range installOrder {
		if bundle, exists := bundleMap[bundleName]; exists {
			result.ResolvedBundles = append(result.ResolvedBundles, bundle)
		}
	}

	// If there are missing bundles but no conflicts, we can still partially succeed
	if len(result.MissingBundles) > 0 && len(result.Conflicts) == 0 {
		result.Success = false
	}

	return result, nil
}

// InstallMCPBundles installs the required MCP bundles
func (r *Resolver) InstallMCPBundles(ctx context.Context, bundles []agent_bundle.MCPBundleRef, environment string) error {
	for _, bundle := range bundles {
		// In a real implementation, this would:
		// 1. Download the bundle if needed
		// 2. Install/configure the MCP server
		// 3. Start the server
		// 4. Verify tools are available
		
		// For now, we'll simulate successful installation
		fmt.Printf("Installing MCP bundle: %s@%s in environment %s\n", bundle.Name, bundle.Version, environment)
	}
	
	return nil
}

// ValidateToolAvailability validates that required tools are available in the environment
func (r *Resolver) ValidateToolAvailability(ctx context.Context, tools []agent_bundle.ToolRequirement, environment string) error {
	if r.toolRepo == nil {
		return fmt.Errorf("tool repository not available - cannot validate tool availability")
	}

	// Get all available tools in the environment
	availableTools, err := r.toolRepo.ListAvailableTools(environment)
	if err != nil {
		return fmt.Errorf("failed to list available tools: %w", err)
	}

	// Create lookup map for faster searching
	toolMap := make(map[string]*ToolInfo)
	for _, tool := range availableTools {
		key := fmt.Sprintf("%s:%s", tool.ServerName, tool.Name)
		toolMap[key] = tool
	}

	// Check each required tool
	var missingTools []string
	for _, req := range tools {
		key := fmt.Sprintf("%s:%s", req.ServerName, req.Name)
		
		if tool, exists := toolMap[key]; exists {
			if !tool.Available {
				missingTools = append(missingTools, fmt.Sprintf("%s (server not running)", req.Name))
			}
		} else {
			// Check if any alternatives are available
			alternativeFound := false
			for _, alt := range req.Alternatives {
				altKey := fmt.Sprintf("%s:%s", req.ServerName, alt)
				if altTool, altExists := toolMap[altKey]; altExists && altTool.Available {
					alternativeFound = true
					break
				}
			}
			
			if !alternativeFound {
				if req.Required {
					missingTools = append(missingTools, req.Name)
				}
			}
		}
	}

	if len(missingTools) > 0 {
		return fmt.Errorf("required tools not available: %s", strings.Join(missingTools, ", "))
	}

	return nil
}

// ResolveConflicts resolves conflicts between different bundle requirements
func (r *Resolver) ResolveConflicts(conflicts []agent_bundle.ToolConflict) (*agent_bundle.ConflictResolution, error) {
	resolution := &agent_bundle.ConflictResolution{
		Strategy:    "auto",
		Resolutions: make(map[string]string),
		Warnings:    []string{},
	}

	for _, conflict := range conflicts {
		// For duplicate tools, prefer the first bundle (could be made configurable)
		if len(conflict.ConflictingBundles) > 0 {
			preferred := conflict.ConflictingBundles[0]
			resolution.Resolutions[conflict.ToolName] = preferred
			
			// Add warning about the conflict
			warning := fmt.Sprintf("Tool '%s' provided by multiple bundles: %s. Using '%s'.", 
				conflict.ToolName, strings.Join(conflict.ConflictingBundles, ", "), preferred)
			resolution.Warnings = append(resolution.Warnings, warning)
		} else {
			// Unknown conflict type - add warning
			resolution.Warnings = append(resolution.Warnings, 
				fmt.Sprintf("Unknown conflict for tool '%s'", conflict.ToolName))
		}
	}

	return resolution, nil
}

// determineInstallOrder determines the order to install bundles (simple topological sort)
func (r *Resolver) determineInstallOrder(dependencies []agent_bundle.MCPBundleDependency) []string {
	// Simple implementation - install in dependency order
	// In a real implementation, you'd want proper dependency graph resolution
	
	var order []string
	
	// Separate required and optional dependencies
	var required []string
	var optional []string
	
	for _, dep := range dependencies {
		if dep.Required {
			required = append(required, dep.Name)
		} else {
			optional = append(optional, dep.Name)
		}
	}
	
	// Install required first, then optional
	order = append(order, required...)
	order = append(order, optional...)
	
	return order
}