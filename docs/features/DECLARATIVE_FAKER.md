# Declarative Faker Configuration

## Problem Statement

Currently, faker tools are dynamically generated via AI and cached in the SQLite database. This creates reproducibility issues when bundling environments:

1. **Non-portable**: Tools exist only in the local database
2. **Bundle incompatibility**: When a user installs a bundle with agents that reference faker tools, those tools don't exist in their fresh Station install
3. **Dynamic breakage**: Agent tool assignments break because the faker tools they reference don't exist
4. **Manual recreation**: Users must manually recreate fakers with the exact same tools

## Solution: Declarative Faker Definitions

Create a JSON-based configuration format that defines faker tools explicitly. This allows:
- ✅ Version control of faker configurations
- ✅ Reproducible tool generation across environments
- ✅ Bundle portability with faker definitions
- ✅ Seeding of fresh installs with predefined tools

## Configuration Format

### File: `faker-config.json`

```json
{
  "fakers": {
    "prometheus-metrics": {
      "description": "Prometheus metrics API tools for post-deployment performance analysis",
      "ai_model": "gpt-4o-mini",
      "tools": [
        {
          "name": "query_range",
          "description": "Query time-series metrics over a specified time range",
          "input_schema": {
            "type": "object",
            "properties": {
              "query": {
                "type": "string",
                "description": "PromQL query expression"
              },
              "start": {
                "type": "string",
                "description": "Start time (RFC3339 or Unix timestamp)"
              },
              "end": {
                "type": "string",
                "description": "End time (RFC3339 or Unix timestamp)"
              },
              "step": {
                "type": "string",
                "description": "Query resolution step (e.g., '15s', '1m')"
              }
            },
            "required": ["query", "start", "end"]
          }
        },
        {
          "name": "get_service_latency_p50_p95_p99",
          "description": "Get p50, p95, and p99 latency percentiles for a service",
          "input_schema": {
            "type": "object",
            "properties": {
              "service_name": {
                "type": "string",
                "description": "Name of the service to query"
              },
              "time_range": {
                "type": "string",
                "description": "Time range for the query (e.g., '1h', '24h')"
              }
            },
            "required": ["service_name", "time_range"]
          }
        }
      ]
    },
    "datadog-apm": {
      "description": "Datadog APM tools for trace analysis post-deployment",
      "ai_model": "gpt-4o-mini",
      "tools": [
        {
          "name": "search_traces",
          "description": "Search for traces matching specified criteria",
          "input_schema": {
            "type": "object",
            "properties": {
              "service": {
                "type": "string",
                "description": "Service name to filter traces"
              },
              "start": {
                "type": "integer",
                "description": "Start timestamp (Unix epoch)"
              },
              "end": {
                "type": "integer",
                "description": "End timestamp (Unix epoch)"
              },
              "env": {
                "type": "string",
                "description": "Environment filter (production, staging, etc.)"
              }
            },
            "required": ["service", "start", "end"]
          }
        },
        {
          "name": "get_trace_details",
          "description": "Get detailed information about a specific trace",
          "input_schema": {
            "type": "object",
            "properties": {
              "trace_id": {
                "type": "string",
                "description": "Unique trace identifier"
              }
            },
            "required": ["trace_id"]
          }
        }
      ]
    }
  }
}
```

## Environment Integration

### Directory Structure

```
~/.config/station/environments/cicd-security/
├── template.json           # MCP server configs
├── variables.yml           # Template variables
├── faker-config.json       # ← NEW: Faker tool definitions
└── agents/
    ├── security-scanner.prompt
    └── performance-analyzer.prompt
```

### Bundle Structure

```
security-bundle.tar.gz
├── template.json
├── variables.yml
├── faker-config.json       # ← Included in bundle
└── agents/
    ├── terraform-auditor.prompt
    └── container-scanner.prompt
```

## Implementation Plan

### Phase 1: Faker Config Schema & Validation

**File**: `pkg/faker/config/schema.go`

```go
package config

type FakerConfig struct {
    Fakers map[string]FakerDefinition `json:"fakers"`
}

type FakerDefinition struct {
    Description string         `json:"description"`
    AIModel     string         `json:"ai_model"`
    Tools       []ToolDefinition `json:"tools"`
}

type ToolDefinition struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    InputSchema map[string]interface{} `json:"input_schema"`
}

// LoadFakerConfig loads faker config from JSON file
func LoadFakerConfig(path string) (*FakerConfig, error) {
    // Implementation
}

// ValidateConfig validates faker config structure
func ValidateConfig(config *FakerConfig) error {
    // Ensure tool names are unique
    // Validate JSON schemas
    // Check required fields
}
```

### Phase 2: Declarative Tool Seeding

**File**: `pkg/faker/seed.go`

```go
// SeedToolsFromConfig reads faker-config.json and seeds the tool cache
func SeedToolsFromConfig(ctx context.Context, configPath string, cache toolcache.Cache) error {
    // 1. Load faker-config.json
    // 2. For each faker definition:
    //    - Convert ToolDefinition to mcp.Tool
    //    - Cache tools using fakerID as key
    // 3. Mark as "declarative" vs "AI-generated" in metadata
}
```

### Phase 3: Sync Command Integration

**File**: `cmd/main/faker_sync.go`

```go
// stn faker sync --env <environment>
// Reads faker-config.json and seeds all defined fakers into the tool cache

func syncFakerConfig(cmd *cobra.Command, args []string) error {
    envName := getFlagString(cmd, "env")
    
    // 1. Find environment directory
    envPath := getEnvironmentPath(envName)
    
    // 2. Load faker-config.json
    configPath := filepath.Join(envPath, "faker-config.json")
    fakerConfig, err := config.LoadFakerConfig(configPath)
    
    // 3. Seed all fakers into tool cache
    for fakerID, def := range fakerConfig.Fakers {
        tools := convertToMCPTools(def.Tools)
        cache.SetTools(ctx, fakerID, tools)
    }
    
    fmt.Printf("✅ Synced %d fakers from config\n", len(fakerConfig.Fakers))
}
```

### Phase 4: Bundle Export/Import

**File**: `internal/services/bundle_service.go`

```go
// ExportBundle now includes faker-config.json
func (s *BundleService) ExportBundle(envName string) error {
    // 1. Export template.json, variables.yml, agents/ (existing)
    // 2. Export faker-config.json from environment directory
    // 3. Package into tar.gz
}

// ImportBundle seeds faker tools from config
func (s *BundleService) ImportBundle(bundlePath string, envName string) error {
    // 1. Extract bundle (existing)
    // 2. Copy template.json, variables.yml, agents/ (existing)
    // 3. If faker-config.json exists:
    //    - Copy to environment directory
    //    - Auto-run faker sync to seed tools
}
```

### Phase 5: Hybrid Mode (Declarative + AI)

Support both modes:

1. **Fully Declarative**: All tools defined in `faker-config.json`
   - Fast startup, reproducible
   - No AI calls for tool generation
   - Tools are static

2. **Hybrid**: Base tools from config, AI can extend dynamically
   - Start with declarative tools
   - Allow AI to add new tools at runtime
   - Cache new tools for next run

3. **Fully AI** (current): No config, pure AI generation
   - Maximum flexibility
   - Non-deterministic
   - Slower startup

## Migration Path

### For Existing Fakers

```bash
# Export current AI-generated tools to declarative config
stn faker export --faker-id prometheus-metrics --output faker-config.json

# Result: Converts cached tools to JSON format
```

**File**: `cmd/main/faker_export.go`

```go
func exportFakerToConfig(fakerID string) error {
    // 1. Load tools from cache
    tools, err := cache.GetTools(ctx, fakerID)
    
    // 2. Convert mcp.Tool → ToolDefinition
    toolDefs := convertToToolDefinitions(tools)
    
    // 3. Write to faker-config.json
    config := FakerConfig{
        Fakers: map[string]FakerDefinition{
            fakerID: {
                Tools: toolDefs,
            },
        },
    }
    writeJSON("faker-config.json", config)
}
```

## Benefits

1. **Reproducibility**: Same config = same tools across all installations
2. **Version Control**: Commit `faker-config.json` to Git
3. **Bundle Portability**: Ship complete simulation environments
4. **No AI Dependency**: Can run without API keys if tools are predefined
5. **Faster Startup**: Skip AI tool generation phase
6. **Documentation**: Config serves as tool API documentation

## Example Use Case: CICD Security Bundle

```
cicd-security-bundle/
├── faker-config.json         # Defines ship-security faker with 307 tools
├── template.json             # References "ship-security" MCP server
├── agents/
    └── security-scanner.prompt  # Uses __checkov_scan, __trivy_scan, etc.
```

When installed:
1. `stn template install cicd-security-bundle.tar.gz`
2. Extracts bundle
3. Reads `faker-config.json`
4. Seeds `ship-security` faker with all 307 tools
5. Agent's `__checkov_scan` tool now exists and works!

## Open Questions

1. **Tool Response Templates**: Should we also define example responses in the config?
2. **AI Fallback**: If a tool isn't in config, should AI generate it on-demand?
3. **Versioning**: How to handle faker config schema changes over time?
4. **Validation**: Should we validate tool schemas against JSON Schema Draft 7?

## Next Steps

1. ✅ Design declarative config format (this doc)
2. ⏳ Implement `pkg/faker/config` package
3. ⏳ Add `stn faker sync` command
4. ⏳ Update bundle export/import to include faker configs
5. ⏳ Test with CICD security bundle
6. ⏳ Document migration from AI-generated to declarative
