# MCP Dependency Resolver Integration

This example shows how to integrate the real MCP dependency resolver for production deployments.

## Overview

The dependency resolver handles:
- MCP bundle availability checking
- Tool conflict detection and resolution  
- Installation order determination
- Server status validation

## Integration Example

```go
package main

import (
    "context"
    
    "github.com/spf13/afero"
    "station/pkg/agent-bundle/manager"
    "station/pkg/agent-bundle/validator" 
    "station/pkg/agent-bundle/resolver"
)

// Production resolver setup
func createProductionManager(repos *repositories.Repositories) *manager.Manager {
    fs := afero.NewOsFs()
    bundleValidator := validator.New(fs)
    
    // Create tool repository implementation
    toolRepo := &ProductionMCPToolRepository{
        repos: repos,
    }
    
    // Create bundle registry implementation
    bundleRegistry := &ProductionBundleRegistry{
        registryURL: "https://mcp-registry.example.com",
    }
    
    // Create real resolver
    realResolver := resolver.New(toolRepo, bundleRegistry)
    
    // Create manager with database access
    return manager.NewWithDatabaseAccess(
        fs, 
        bundleValidator, 
        realResolver,
        repos.Agents,
        repos.AgentTools,
        repos.Environments,
    )
}

// Tool repository implementation
type ProductionMCPToolRepository struct {
    repos *repositories.Repositories
}

func (r *ProductionMCPToolRepository) ListAvailableTools(environment string) ([]*resolver.ToolInfo, error) {
    // Query database for MCP servers and their tools in the environment
    // Return list of available tools with status information
}

func (r *ProductionMCPToolRepository) GetServerInfo(serverName, environment string) (*resolver.ServerInfo, error) {
    // Get specific MCP server information and status
}

func (r *ProductionMCPToolRepository) IsServerRunning(serverName, environment string) (bool, error) {
    // Check if MCP server is currently running
}

// Bundle registry implementation  
type ProductionBundleRegistry struct {
    registryURL string
}

func (r *ProductionBundleRegistry) GetBundle(name, version string) (*resolver.BundleInfo, error) {
    // Fetch bundle information from registry
}

func (r *ProductionBundleRegistry) ListVersions(name string) ([]string, error) {
    // Get available versions for a bundle
}

func (r *ProductionBundleRegistry) IsAvailable(name, version string) (bool, error) {
    // Check if specific bundle version is available
}
```

## Features

### **Dependency Resolution**
- ✅ Bundle availability checking against registry
- ✅ Version constraint resolution  
- ✅ Tool conflict detection
- ✅ Installation order determination

### **Tool Validation** 
- ✅ Server status checking
- ✅ Tool availability validation
- ✅ Alternative tool resolution
- ✅ Environment-specific validation

### **Conflict Resolution**
- ✅ Automatic conflict resolution strategies
- ✅ User-configurable preferences
- ✅ Warning generation for conflicts
- ✅ Fallback to alternative tools

## Configuration

### **Registry Configuration**
```yaml
# Station config
mcp:
  registry:
    url: "https://mcp-registry.example.com"
    auth_token: "${MCP_REGISTRY_TOKEN}"
    timeout: 30s
    
  resolver:
    strategy: "prefer_latest"  # "prefer_latest", "prefer_stable"
    conflict_resolution: "auto"  # "auto", "manual", "strict"
    install_timeout: 300s
```

### **Environment-Specific Settings**
```yaml
environments:
  production:
    mcp:
      strict_validation: true
      require_signed_bundles: true
      allowed_sources: ["registry", "internal"]
      
  development:  
    mcp:
      strict_validation: false
      allow_local_bundles: true
      allowed_sources: ["*"]
```

## Error Handling

The resolver provides detailed error information:

```go
result, err := resolver.Resolve(ctx, dependencies, "production")
if err != nil {
    return fmt.Errorf("dependency resolution failed: %w", err)
}

if !result.Success {
    // Handle missing bundles
    for _, missing := range result.MissingBundles {
        log.Printf("Missing bundle: %s@%s", missing.Name, missing.Version)
    }
    
    // Handle conflicts  
    if len(result.Conflicts) > 0 {
        resolution, err := resolver.ResolveConflicts(result.Conflicts)
        if err != nil {
            return fmt.Errorf("conflict resolution failed: %w", err)
        }
        
        // Apply conflict resolution
        for tool, preferredBundle := range resolution.Resolutions {
            log.Printf("Using %s from bundle %s", tool, preferredBundle)
        }
    }
}
```

## Testing

```go
func TestDependencyResolver(t *testing.T) {
    // Mock implementations for testing
    mockToolRepo := &MockMCPToolRepository{}
    mockRegistry := &MockBundleRegistry{}
    
    resolver := resolver.New(mockToolRepo, mockRegistry)
    
    // Test dependency resolution
    deps := []agent_bundle.MCPBundleDependency{
        {Name: "filesystem-tools", Version: ">=1.0.0", Required: true},
        {Name: "web-tools", Version: "^2.1.0", Required: false},
    }
    
    result, err := resolver.Resolve(context.Background(), deps, "test")
    assert.NoError(t, err)
    assert.True(t, result.Success)
    assert.Len(t, result.ResolvedBundles, 2)
}
```

## Migration from Mock

To migrate from mock resolver to production:

1. **Implement Repository Interfaces**:
   - `MCPToolRepository` for tool/server queries
   - `BundleRegistry` for bundle availability

2. **Update Manager Creation**:
   ```go
   // Replace mock resolver
   realResolver := resolver.New(toolRepo, bundleRegistry)
   manager := manager.NewWithDatabaseAccess(fs, validator, realResolver, ...)
   ```

3. **Configure Registry Access**:
   - Set registry URL and credentials
   - Configure timeout and retry policies
   - Set up caching for performance

4. **Test Integration**:
   - Verify bundle resolution works
   - Test conflict resolution scenarios
   - Validate error handling