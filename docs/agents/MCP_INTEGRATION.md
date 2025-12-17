# MCP Integration Guide

Station provides comprehensive integration with the Model Context Protocol (MCP), enabling agents to access powerful tools through standardized interfaces. This guide covers MCP server setup, tool discovery, and advanced integration patterns.

## MCP Architecture in Station

### MCP Protocol Overview

Station implements both MCP client and server capabilities:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Claude Code   │───▶│    Station      │───▶│   MCP Servers   │
│  (MCP Client)   │MCP │  (MCP Client    │MCP │  (Tools/APIs)   │
│                 │    │   & Server)     │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │
                       ┌──────▼───────┐
                       │  Station's   │
                       │ MCP Server   │
                       │ (13 tools)   │
                       └──────────────┘
```

### Station's MCP Server

Station itself provides MCP tools via stdio interface:

#### Claude Code

```bash
claude mcp add --transport stdio station -e OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 -- stn stdio
```

#### Claude Desktop

Edit `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "station": {
      "command": "stn",
      "args": ["stdio"],
      "env": {
        "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4318"
      }
    }
  }
}
```

#### Cursor / Other MCP Clients

Add to `.mcp.json` in your project:

```json
{
  "mcpServers": {
    "station": {
      "command": "stn",
      "args": ["stdio"],
      "env": {
        "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4318"
      }
    }
  }
}
```

> **Note**: The `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable enables OpenTelemetry tracing. Station ships with Jaeger for trace visualization at `http://localhost:16686`.

**Available Station Tools**:
- `call_agent` - Execute agents with custom parameters
- `create_agent` - Create new agents with schemas
- `list_agents` - Browse available agents
- `list_environments` - View environment configurations
- `discover_tools` - Find available MCP tools
- `create_bundle_from_environment` - Package environments
- `sync_environment` - Update MCP server connections

## MCP Server Configuration

### Template System

MCP servers are configured in environment `template.json` files with Go template support:

```json
{
  "name": "production-environment",
  "description": "Production environment with security and monitoring tools",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem@latest", 
        "{{ .PROJECT_ROOT }}"
      ],
      "env": {
        "NODE_ENV": "{{ .NODE_ENV | default \"production\" }}"
      }
    },
    "ship-security": {
      "command": "ship",
      "args": ["mcp", "security", "--stdio"],
      "env": {
        "SHIP_LOG_LEVEL": "{{ .LOG_LEVEL }}"
      }
    },
    "database": {
      "command": "{{ .POSTGRES_MCP_PATH }}",
      "args": ["--connection-string", "{{ .DATABASE_URL }}"],
      "condition": "{{ .ENABLE_DATABASE_TOOLS | default \"false\" }}"
    }
  }
}
```

### Variable Resolution

Environment variables are resolved from `variables.yml`:

```yaml
PROJECT_ROOT: "/opt/production/projects"
DATABASE_URL: "postgresql://user:pass@prod-db:5432/app"
LOG_LEVEL: "info"
NODE_ENV: "production"
POSTGRES_MCP_PATH: "/usr/local/bin/postgres-mcp-server"
ENABLE_DATABASE_TOOLS: "true"
```

### Server Lifecycle Management

```bash
# Start MCP servers for environment
stn sync production

# Check server health
stn mcp status --env production

# Restart failed servers
stn mcp restart filesystem --env production

# View server logs
stn mcp logs ship-security --env production --tail

# Test server connectivity
stn mcp test database --env production
```

## Common MCP Server Integrations

### 1. Filesystem Server

**Purpose**: File system operations (read, write, list, search)

**Installation**:
```bash
npm install -g @modelcontextprotocol/server-filesystem
```

**Configuration**:
```json
{
  "filesystem": {
    "command": "npx",
    "args": [
      "-y",
      "@modelcontextprotocol/server-filesystem@latest",
      "{{ .PROJECT_ROOT }}"
    ]
  }
}
```

**Available Tools**:
- `__read_text_file` - Read file contents
- `__write_text_file` - Write file contents  
- `__list_directory` - List directory contents
- `__directory_tree` - Get directory structure
- `__search_files` - Search for files by pattern
- `__get_file_info` - Get file metadata

**Usage in Agents**:
```yaml
tools:
  - "__read_text_file"
  - "__list_directory" 
  - "__directory_tree"
  - "__search_files"
```

### 2. Ship Security Server

**Purpose**: 300+ security tools via Ship CLI integration

**Installation**:
```bash
curl -sSL https://ship.sh/install | bash
```

**Configuration**:
```json
{
  "ship-security": {
    "command": "ship",
    "args": ["mcp", "security", "--stdio"]
  }
}
```

**Available Tool Categories**:
- **Infrastructure Security**: `__checkov_scan_directory`, `__tflint_directory`, `__terrascan_scan`
- **Container Security**: `__trivy_scan_filesystem`, `__hadolint_dockerfile`, `__docker_bench_security`
- **Code Security**: `__semgrep_scan`, `__bandit_scan`, `__eslint_security`
- **Secret Detection**: `__gitleaks_dir`, `__trufflehog_scan`, `__detect_secrets`
- **Cloud Security**: `__scout_suite_aws`, `__kube_bench`, `__kubescape_scan`

**Usage in Agents**:
```yaml
tools:
  - "__checkov_scan_directory"
  - "__trivy_scan_filesystem"
  - "__gitleaks_dir"
  - "__semgrep_scan"
```

### 3. Playwright Web Automation

**Purpose**: Browser automation and web testing

**Installation**:
```bash
npm install -g @modelcontextprotocol/server-playwright
playwright install
```

**Configuration**:
```json
{
  "playwright": {
    "command": "npx",
    "args": [
      "-y",
      "@modelcontextprotocol/server-playwright@latest"
    ],
    "env": {
      "PLAYWRIGHT_HEADLESS": "{{ .HEADLESS_MODE | default \"true\" }}"
    }
  }
}
```

**Available Tools**:
- `__browser_navigate` - Navigate to URL
- `__browser_screenshot` - Take page screenshot
- `__browser_click` - Click page element
- `__browser_type` - Type text in form
- `__browser_evaluate` - Execute JavaScript
- `__browser_get_text` - Extract text content

### 4. Database Servers

**PostgreSQL Server**:
```json
{
  "postgres": {
    "command": "npx",
    "args": [
      "-y", 
      "@modelcontextprotocol/server-postgres@latest"
    ],
    "env": {
      "POSTGRES_URL": "{{ .DATABASE_URL }}"
    }
  }
}
```

**SQLite Server**:
```json
{
  "sqlite": {
    "command": "npx", 
    "args": [
      "-y",
      "@modelcontextprotocol/server-sqlite@latest",
      "{{ .SQLITE_DB_PATH }}"
    ]
  }
}
```

## Tool Discovery and Assignment

### Automatic Tool Discovery

Station automatically discovers tools when syncing environments:

```bash
# Sync environment and discover all tools
stn sync development --verbose

# Output shows tool discovery:
# Discovered 14 filesystem tools
# Discovered 307 Ship security tools  
# Discovered 21 Playwright web tools
# Total: 342 tools available
```

### Tool Assignment to Agents

**Manual Tool Assignment**:
```yaml
# In agent .prompt file
tools:
  - "__read_text_file"      # Filesystem server
  - "__checkov_scan_directory"  # Ship security server
  - "__browser_screenshot"  # Playwright server
```

**MCP-Based Tool Assignment** (using Station's MCP interface):
```bash
# Use Claude Code with Station MCP to intelligently assign tools
# Claude will analyze agent requirements and assign optimal tools
stn stdio
# Then use Claude Code to interact with Station MCP tools
```

### Tool Filtering and Security

Station applies tool filtering at execution time:

```go
// Only tools assigned to the agent are available
func (s *AgentService) FilterToolsForAgent(agentID int, allTools []MCPTool) []MCPTool {
    assignedTools := s.GetAssignedTools(agentID)
    return filterByAssignment(allTools, assignedTools)
}
```

## Custom MCP Server Development

### Creating Custom MCP Servers

**Go MCP Server Example**:
```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "github.com/cloudshipai/mcp-go"
)

type CustomServer struct {
    mcp.BaseServer
}

func (s *CustomServer) ListTools(ctx context.Context) ([]mcp.Tool, error) {
    return []mcp.Tool{
        {
            Name:        "__custom_analysis",
            Description: "Performs custom data analysis",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "data_path": {
                        "type":        "string", 
                        "description": "Path to data file",
                    },
                    "analysis_type": {
                        "type":        "string",
                        "enum":        []string{"statistical", "ml", "visualization"},
                        "description": "Type of analysis to perform",
                    },
                },
                "required": []string{"data_path", "analysis_type"},
            },
        },
    }, nil
}

func (s *CustomServer) CallTool(ctx context.Context, name string, args map[string]interface{}) (*mcp.ToolResult, error) {
    switch name {
    case "__custom_analysis":
        dataPath := args["data_path"].(string)
        analysisType := args["analysis_type"].(string)
        
        result := performAnalysis(dataPath, analysisType)
        
        return &mcp.ToolResult{
            Content: []mcp.Content{
                {Type: "text", Text: result},
            },
        }, nil
    }
    return nil, fmt.Errorf("unknown tool: %s", name)
}

func main() {
    server := &CustomServer{}
    mcp.Serve(server)
}
```

**Station Configuration for Custom Server**:
```json
{
  "custom-analysis": {
    "command": "/usr/local/bin/custom-mcp-server",
    "args": ["--config", "{{ .ANALYSIS_CONFIG_PATH }}"],
    "env": {
      "ANALYSIS_API_KEY": "{{ .ANALYSIS_API_KEY }}"
    }
  }
}
```

### TypeScript/JavaScript MCP Server

```typescript
import { MCPServer, Tool, ToolCall } from '@modelcontextprotocol/server';

class CustomMCPServer extends MCPServer {
  async listTools(): Promise<Tool[]> {
    return [
      {
        name: '__api_request',
        description: 'Make HTTP API requests',
        inputSchema: {
          type: 'object',
          properties: {
            url: { type: 'string', description: 'API endpoint URL' },
            method: { type: 'string', enum: ['GET', 'POST', 'PUT', 'DELETE'] },
            headers: { type: 'object', description: 'Request headers' },
            body: { type: 'string', description: 'Request body' }
          },
          required: ['url', 'method']
        }
      }
    ];
  }

  async callTool(call: ToolCall): Promise<any> {
    const { name, arguments: args } = call;
    
    if (name === '__api_request') {
      const response = await fetch(args.url, {
        method: args.method,
        headers: args.headers || {},
        body: args.body
      });
      
      return {
        content: [
          {
            type: 'text',
            text: await response.text()
          }
        ]
      };
    }
  }
}

const server = new CustomMCPServer({
  name: 'custom-api-server',
  version: '1.0.0'
});

server.run();
```

## Advanced MCP Patterns

### Conditional Server Loading

Load MCP servers based on environment conditions:

```json
{
  "mcpServers": {
    "development-tools": {
      "command": "debug-mcp-server",
      "condition": "{{ eq .ENVIRONMENT \"development\" }}"
    },
    "production-monitoring": {
      "command": "monitoring-mcp-server",
      "condition": "{{ eq .ENVIRONMENT \"production\" }}"
    },
    "security-scanner": {
      "command": "ship",
      "args": ["mcp", "security", "--stdio"],
      "condition": "{{ or (eq .ENVIRONMENT \"staging\") (eq .ENVIRONMENT \"production\") }}"
    }
  }
}
```

### MCP Server Proxy and Load Balancing

```json
{
  "high-availability-api": {
    "command": "mcp-proxy",
    "args": [
      "--upstream", "{{ .API_SERVER_1 }}",
      "--upstream", "{{ .API_SERVER_2 }}",
      "--upstream", "{{ .API_SERVER_3 }}",
      "--strategy", "round-robin",
      "--health-check", "30s"
    ]
  }
}
```

### MCP Server Chaining

Chain multiple MCP servers for complex workflows:

```json
{
  "data-pipeline": {
    "command": "mcp-chain",
    "args": [
      "--server", "data-ingestion-server",
      "--server", "data-processing-server", 
      "--server", "data-output-server"
    ],
    "env": {
      "CHAIN_CONFIG": "{{ .DATA_PIPELINE_CONFIG }}"
    }
  }
}
```

## Debugging MCP Integration

### Connection Debugging

```bash
# Debug MCP server connection issues
stn mcp debug filesystem --env development

# Test tool calls directly
stn mcp call __read_text_file '{"path": "/tmp/test.txt"}' --env development

# Monitor MCP traffic
stn mcp monitor --env development --verbose

# Export MCP logs for analysis
stn mcp export-logs --env development --format json --output ./mcp-debug.json
```

### Tool Call Tracing

```bash
# Trace agent tool calls in real-time
stn agent run "File Analyzer" "Analyze project structure" --env development --trace-tools

# Show detailed tool call history
stn runs inspect <run-id> --show-tool-calls

# Analyze tool call performance
stn mcp metrics --tools-only --env development
```

### Common MCP Issues

**1. Server Connection Timeouts**:
```bash
# Increase timeout in template.json
{
  "server-name": {
    "command": "slow-mcp-server",
    "timeout": "60s"  // Default is 30s
  }
}
```

**2. Tool Schema Validation Errors**:
```bash
# Validate tool schemas
stn mcp validate-schemas --env development

# Test tool with sample data
stn mcp test-tool __custom_tool --sample-args '{"arg1": "value"}' 
```

**3. Environment Variable Resolution**:
```bash
# Debug variable resolution
stn sync development --dry-run --show-variables

# Test template rendering
stn env render-template development --output ./rendered-config.json
```

## Performance Optimization

### MCP Connection Pooling

```go
// Station automatically pools MCP connections
type MCPConnectionPool struct {
    maxConnections    int
    idleTimeout      time.Duration
    connectionReuse  bool
    healthCheckFreq  time.Duration
}

// Optimized settings for high-throughput environments
pool := &MCPConnectionPool{
    maxConnections:   50,
    idleTimeout:     5 * time.Minute,
    connectionReuse: true,
    healthCheckFreq: 30 * time.Second,
}
```

### Tool Call Caching

```bash
# Enable tool call result caching
stn config set mcp.cache.enabled true
stn config set mcp.cache.ttl 300  # 5 minutes
stn config set mcp.cache.size 1000  # Max cached results
```

### Parallel Tool Execution

Agents can execute multiple tools in parallel:

```yaml
# Agent configuration supports parallel tool calls
execution:
  parallel_tools: true
  max_parallel: 5
  timeout_per_tool: 30s
```

This MCP integration system provides Station with powerful tool access capabilities while maintaining security, performance, and ease of configuration across multi-environment deployments.