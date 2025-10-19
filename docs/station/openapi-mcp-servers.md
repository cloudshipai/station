# OpenAPI MCP Servers

Station can automatically convert OpenAPI/Swagger specifications into MCP servers, making any REST API instantly available as agent tools. This powerful feature enables agents to interact with any API that provides an OpenAPI specification.

## Table of Contents
- [Overview](#overview)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Template Variables](#template-variables)
- [Creating OpenAPI MCP Servers](#creating-openapi-mcp-servers)
- [Advanced Features](#advanced-features)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)

---

## Overview

### What is an OpenAPI MCP Server?

An OpenAPI MCP Server is a Station component that:
1. **Reads an OpenAPI/Swagger specification** (JSON or YAML)
2. **Converts API endpoints to MCP tools** - Each operation becomes a callable tool
3. **Processes template variables** - Resolves `{{ .VARIABLE }}` syntax from environment config
4. **Handles authentication** - Supports Bearer tokens, API keys, OAuth
5. **Makes API calls** - Executes HTTP requests based on tool invocations

### Benefits

- ✅ **Zero boilerplate** - No custom MCP server code required
- ✅ **Automatic updates** - Changes to OpenAPI spec automatically sync new tools
- ✅ **Type safety** - Request/response schemas from OpenAPI spec
- ✅ **Template variables** - Secure configuration management
- ✅ **Multi-environment** - Different URLs/credentials per environment (dev/staging/prod)

---

## Quick Start

### 1. Prepare Your OpenAPI Spec

Create an OpenAPI 3.0 specification with template variables:

**`station-api.openapi.json`**:
```json
{
  "openapi": "3.0.0",
  "info": {
    "title": "Station Management API",
    "version": "1.0.0"
  },
  "servers": [
    {
      "url": "{{ .STATION_API_URL }}",
      "description": "Station API endpoint"
    }
  ],
  "components": {
    "securitySchemes": {
      "bearerAuth": {
        "type": "http",
        "scheme": "bearer"
      }
    }
  },
  "paths": {
    "/environments": {
      "get": {
        "operationId": "listEnvironments",
        "summary": "List all environments",
        "responses": {
          "200": {
            "description": "Success",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "count": {"type": "integer"},
                    "environments": {
                      "type": "array",
                      "items": {"type": "object"}
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}
```

### 2. Create MCP Server Template

**`station-api.json`**:
```json
{
  "name": "Station Management API",
  "description": "Control Station via REST API",
  "mcpServers": {
    "station-api": {
      "command": "stn",
      "args": [
        "openapi-runtime",
        "--spec",
        "environments/{{ .ENVIRONMENT_NAME }}/station-api.openapi.json"
      ]
    }
  },
  "metadata": {
    "category": "Management",
    "tags": ["station", "api", "management"],
    "openapiSpec": "station-api.openapi.json",
    "requiresOpenAPISpec": true,
    "variables": {
      "ENVIRONMENT_NAME": {
        "description": "Environment name (auto-populated)",
        "default": "{{ .ENVIRONMENT_NAME }}",
        "required": false,
        "autoPopulate": true
      },
      "STATION_API_URL": {
        "description": "Station API endpoint URL",
        "default": "http://localhost:8585/api/v1",
        "required": false
      }
    }
  }
}
```

### 3. Install and Sync

```bash
# Place files in environment directory
cp station-api.json ~/.config/station/environments/default/
cp station-api.openapi.json ~/.config/station/environments/default/

# Sync to discover tools
stn sync default
```

**Output**:
```
✅ Updated MCP server: station-api
MCP SUCCESS: Discovered 1 tools from server 'station-api'
  🔧 Tool 1: __listEnvironments
```

### 4. Use in Agent

```yaml
---
metadata:
  name: "Station Admin"
model: gpt-4o-mini
tools:
  - "__listEnvironments"  # From OpenAPI spec
---

{{role "system"}}
You manage Station environments using the Station API.

{{role "user"}}
{{userInput}}
```

---

## Configuration

### MCP Server Template Structure

```json
{
  "name": "Human-readable name",
  "description": "What this MCP server does",
  "mcpServers": {
    "server-id": {
      "command": "stn",
      "args": [
        "openapi-runtime",
        "--spec",
        "path/to/spec.openapi.json"
      ],
      "env": {
        "API_KEY": "{{ .API_KEY }}"  // Optional environment variables
      }
    }
  },
  "metadata": {
    "category": "Category name",
    "tags": ["tag1", "tag2"],
    "openapiSpec": "spec-file-name.openapi.json",
    "requiresOpenAPISpec": true,
    "variables": {
      "VARIABLE_NAME": {
        "description": "What this variable configures",
        "default": "default-value",
        "required": true,
        "secret": false
      }
    }
  }
}
```

### OpenAPI Runtime Command

The `stn openapi-runtime` command:

**Syntax**:
```bash
stn openapi-runtime --spec <path-to-openapi-spec>
```

**Features**:
- ✅ Loads OpenAPI 3.0 or Swagger 2.0 specifications
- ✅ Processes Go template variables (`{{ .VAR }}`)
- ✅ Converts endpoints to MCP tools (operationId → tool name)
- ✅ Starts MCP server via stdio protocol
- ✅ Handles HTTP requests with authentication

**Template Variable Resolution**:
1. Reads `variables.yml` from environment directory
2. Loads environment variables (overrides variables.yml)
3. Processes OpenAPI spec with Go template engine
4. Converts processed spec to MCP tools

---

## Template Variables

### Variable Sources (Priority Order)

1. **Environment Variables** (highest priority)
2. **variables.yml** (medium priority)
3. **Template defaults** (lowest priority)

### variables.yml Format

**`~/.config/station/environments/default/variables.yml`**:
```yaml
ENVIRONMENT_NAME: default
STATION_API_URL: http://localhost:8585/api/v1
API_KEY: secret-key-12345
```

### Using Variables in OpenAPI Specs

Template variables use Go template syntax: `{{ .VARIABLE_NAME }}`

**Supported Locations**:

**1. Server URLs**:
```json
{
  "servers": [
    {
      "url": "{{ .BASE_URL }}",
      "description": "API endpoint"
    }
  ]
}
```

**2. Security Schemes**:
```json
{
  "components": {
    "securitySchemes": {
      "bearerAuth": {
        "type": "http",
        "scheme": "bearer",
        "description": "API Key: {{ .API_KEY }}"
      }
    }
  }
}
```

**3. Parameter Defaults**:
```json
{
  "parameters": [
    {
      "name": "environment",
      "in": "query",
      "schema": {
        "type": "string",
        "default": "{{ .ENVIRONMENT_NAME }}"
      }
    }
  ]
}
```

**4. Any String Value**:
Template variables work in any JSON string field.

### Auto-Populated Variables

Some variables are automatically populated by Station:

- `ENVIRONMENT_NAME`: Current environment name (e.g., "default", "production")
- `USER`: Current user running the agent

---

## Creating OpenAPI MCP Servers

### Step-by-Step Guide

#### 1. Obtain or Create OpenAPI Spec

**Option A: Use Existing Spec**
Many APIs provide OpenAPI specifications:
- Download from API documentation
- Generate from Swagger UI
- Export from API gateway (Kong, Apigee, etc.)

**Option B: Create Custom Spec**
```json
{
  "openapi": "3.0.0",
  "info": {
    "title": "My API",
    "version": "1.0.0"
  },
  "servers": [
    {"url": "{{ .API_BASE_URL }}"}
  ],
  "paths": {
    "/resource": {
      "get": {
        "operationId": "getResource",
        "summary": "Get resource",
        "responses": {
          "200": {"description": "Success"}
        }
      }
    }
  }
}
```

#### 2. Add Template Variables

Replace hardcoded values with template variables:

**Before**:
```json
{
  "servers": [
    {"url": "https://api.example.com/v1"}
  ]
}
```

**After**:
```json
{
  "servers": [
    {"url": "{{ .API_BASE_URL }}"}
  ]
}
```

#### 3. Create MCP Server Template

```json
{
  "name": "My API",
  "description": "MCP server for My API",
  "mcpServers": {
    "my-api": {
      "command": "stn",
      "args": [
        "openapi-runtime",
        "--spec",
        "environments/{{ .ENVIRONMENT_NAME }}/my-api.openapi.json"
      ]
    }
  },
  "metadata": {
    "openapiSpec": "my-api.openapi.json",
    "variables": {
      "API_BASE_URL": {
        "description": "My API base URL",
        "default": "https://api.example.com/v1",
        "required": true
      }
    }
  }
}
```

#### 4. Configure Variables

**`variables.yml`**:
```yaml
API_BASE_URL: https://api.example.com/v1
API_KEY: your-api-key-here
```

#### 5. Test Runtime

```bash
# Test OpenAPI runtime directly
stn openapi-runtime --spec environments/default/my-api.openapi.json
```

**Expected Output**:
```
=== OpenAPI Runtime MCP Server starting ===
Loading OpenAPI spec from file: environments/default/my-api.openapi.json
Loading variables for environment: default
Loaded 5 variables for template processing
Successfully processed template variables in OpenAPI spec
Registering 3 OpenAPI tools with MCP server
=== OpenAPI Runtime MCP Server registered 3 tools ===
```

#### 6. Sync Environment

```bash
stn sync default
```

**Verify**:
```
MCP SUCCESS: Discovered 3 tools from server 'my-api'
  🔧 Tool 1: __getResource
  🔧 Tool 2: __createResource
  🔧 Tool 3: __deleteResource
```

---

## Advanced Features

### Authentication

**Bearer Token Authentication**:

**OpenAPI Spec**:
```json
{
  "components": {
    "securitySchemes": {
      "bearerAuth": {
        "type": "http",
        "scheme": "bearer"
      }
    }
  },
  "security": [
    {"bearerAuth": []}
  ]
}
```

**variables.yml**:
```yaml
API_TOKEN: your-bearer-token-here
```

**MCP Template** (pass via environment variable):
```json
{
  "mcpServers": {
    "my-api": {
      "command": "stn",
      "args": ["openapi-runtime", "--spec", "my-api.openapi.json"],
      "env": {
        "BEARER_TOKEN": "{{ .API_TOKEN }}"
      }
    }
  }
}
```

### Multiple Environments

**Development**:
```yaml
# environments/dev/variables.yml
API_BASE_URL: http://localhost:3000/api
API_KEY: dev-key-12345
```

**Production**:
```yaml
# environments/prod/variables.yml
API_BASE_URL: https://api.prod.example.com/v1
API_KEY: prod-key-67890
```

Same OpenAPI spec, different runtime configuration!

### OpenAPI Spec Updates

When you update the OpenAPI spec, Station automatically detects changes:

```bash
# Update spec file
vim ~/.config/station/environments/default/my-api.openapi.json

# Re-sync to refresh tools
stn sync default
```

**Station will**:
- ✅ Add new endpoints as tools
- ✅ Remove deleted endpoints
- ✅ Update existing tool schemas
- ✅ Preserve unchanged tools

**Sync Output**:
```
🔧 Tool sync for 'my-api': recreated 5 tools (removed 2 obsolete)
✅ Saved 5 tools for server 'my-api'
```

---

## Examples

### Example 1: GitHub API

**`github-api.openapi.json`** (simplified):
```json
{
  "openapi": "3.0.0",
  "info": {
    "title": "GitHub API",
    "version": "1.0.0"
  },
  "servers": [
    {"url": "https://api.github.com"}
  ],
  "paths": {
    "/repos/{owner}/{repo}/issues": {
      "get": {
        "operationId": "listIssues",
        "parameters": [
          {"name": "owner", "in": "path", "required": true, "schema": {"type": "string"}},
          {"name": "repo", "in": "path", "required": true, "schema": {"type": "string"}}
        ],
        "responses": {
          "200": {"description": "Success"}
        }
      }
    }
  }
}
```

**`github-api.json`**:
```json
{
  "name": "GitHub API",
  "mcpServers": {
    "github": {
      "command": "stn",
      "args": ["openapi-runtime", "--spec", "environments/{{ .ENVIRONMENT_NAME }}/github-api.openapi.json"],
      "env": {
        "GITHUB_TOKEN": "{{ .GITHUB_TOKEN }}"
      }
    }
  },
  "metadata": {
    "openapiSpec": "github-api.openapi.json",
    "variables": {
      "GITHUB_TOKEN": {
        "description": "GitHub personal access token",
        "required": true,
        "secret": true
      }
    }
  }
}
```

**Agent**:
```yaml
---
metadata:
  name: "GitHub Issue Manager"
tools:
  - "__listIssues"
---

{{role "system"}}
You manage GitHub issues using the GitHub API.

{{role "user"}}
List issues for cloudshipai/station repository
```

### Example 2: Analytics API with Complex Authentication

**`analytics-api.openapi.json`**:
```json
{
  "openapi": "3.0.0",
  "info": {
    "title": "Analytics API",
    "version": "2.0.0"
  },
  "servers": [
    {
      "url": "{{ .ANALYTICS_BASE_URL }}",
      "description": "Analytics endpoint"
    }
  ],
  "components": {
    "securitySchemes": {
      "apiKeyAuth": {
        "type": "apiKey",
        "in": "header",
        "name": "X-API-Key"
      }
    }
  },
  "security": [
    {"apiKeyAuth": []}
  ],
  "paths": {
    "/metrics/{metric_id}": {
      "get": {
        "operationId": "getMetric",
        "summary": "Retrieve metric by ID",
        "parameters": [
          {
            "name": "metric_id",
            "in": "path",
            "required": true,
            "schema": {"type": "string"}
          },
          {
            "name": "env",
            "in": "query",
            "schema": {
              "type": "string",
              "default": "{{ .ENVIRONMENT_NAME }}"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Metric data",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "metric_id": {"type": "string"},
                    "value": {"type": "number"},
                    "timestamp": {"type": "string"}
                  }
                }
              }
            }
          }
        }
      }
    },
    "/reports/generate": {
      "post": {
        "operationId": "generateReport",
        "summary": "Generate analytics report",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "report_type": {
                    "type": "string",
                    "enum": ["daily", "weekly", "monthly"]
                  },
                  "metrics": {
                    "type": "array",
                    "items": {"type": "string"}
                  }
                },
                "required": ["report_type", "metrics"]
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Report generated"
          }
        }
      }
    }
  }
}
```

**`analytics-api.json`**:
```json
{
  "name": "Analytics API",
  "description": "Company analytics and reporting",
  "mcpServers": {
    "analytics": {
      "command": "stn",
      "args": [
        "openapi-runtime",
        "--spec",
        "environments/{{ .ENVIRONMENT_NAME }}/analytics-api.openapi.json"
      ]
    }
  },
  "metadata": {
    "category": "Analytics",
    "tags": ["analytics", "metrics", "reporting"],
    "openapiSpec": "analytics-api.openapi.json",
    "variables": {
      "ANALYTICS_BASE_URL": {
        "description": "Analytics API base URL",
        "default": "https://analytics.company.com/api/v1",
        "required": true
      },
      "ANALYTICS_API_KEY": {
        "description": "Analytics API authentication key",
        "required": true,
        "secret": true
      }
    }
  }
}
```

**`variables.yml`**:
```yaml
ANALYTICS_BASE_URL: https://analytics.company.com/api/v1
ANALYTICS_API_KEY: api-key-secret-xyz789
```

**Agent**:
```yaml
---
metadata:
  name: "Analytics Reporter"
  description: "Generate analytics reports and query metrics"
model: gpt-4o-mini
max_steps: 8
tools:
  - "__getMetric"
  - "__generateReport"
---

{{role "system"}}
You are an analytics assistant that helps users query metrics and generate reports.

Use the Analytics API tools to:
- Retrieve specific metrics by ID
- Generate daily, weekly, or monthly reports
- Analyze trends and provide insights

{{role "user"}}
{{userInput}}
```

---

## Troubleshooting

### Issue: "MCP SUCCESS: Discovered 0 tools"

**Cause**: OpenAPI spec template variables not resolved correctly.

**Solution**:
1. Check `variables.yml` has required variables:
   ```bash
   cat ~/.config/station/environments/default/variables.yml
   ```

2. Test openapi-runtime directly:
   ```bash
   stn openapi-runtime --spec environments/default/your-api.openapi.json
   ```

3. Look for template processing logs:
   ```
   Loaded X variables for template processing
   Successfully processed template variables in OpenAPI spec
   ```

### Issue: "unsupported protocol scheme"

**Cause**: Template variable in server URL not resolved (literal `{{ .VAR }}`).

**Solution**:
1. Add missing variable to `variables.yml`:
   ```yaml
   API_BASE_URL: https://api.example.com
   ```

2. Re-sync environment:
   ```bash
   stn sync default
   ```

### Issue: Tool calls fail with authentication errors

**Cause**: API key or bearer token not configured.

**Solution**:
1. Add authentication variable to `variables.yml`:
   ```yaml
   API_KEY: your-api-key-here
   ```

2. Pass via environment variable in MCP template:
   ```json
   {
     "env": {
       "BEARER_TOKEN": "{{ .API_KEY }}"
     }
   }
   ```

3. Verify OpenAPI spec has security scheme:
   ```json
   {
     "components": {
       "securitySchemes": {
         "bearerAuth": {
           "type": "http",
           "scheme": "bearer"
         }
       }
     }
   }
   ```

### Issue: OpenAPI spec updates not reflected

**Cause**: Tools cached from previous sync.

**Solution**:
```bash
# Re-sync environment to refresh tools
stn sync default
```

**Verify**:
```
🔧 Tool sync for 'your-api': recreated X tools (removed Y obsolete)
```

---

## Next Steps

- [Create a Station Admin Agent →](./station-admin-agent.md)
- [Learn about Template Variables →](./templates.md)
- [Explore Agent Development →](./agent-development.md)
- [Browse MCP Tools →](./mcp-tools.md)
