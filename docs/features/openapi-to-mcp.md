# OpenAPI to MCP Server Integration

## Overview

Station now supports automatic conversion of OpenAPI 3.0 specifications into MCP (Model Context Protocol) servers. This allows users to expose any REST API as MCP tools that agents can use, without writing custom MCP server code.

## Architecture

### Components

1. **OpenAPI Package** (`pkg/openapi/`)
   - **Parser**: Validates and parses OpenAPI 3.0 specs using kin-openapi library
   - **Converter**: Transforms OpenAPI operations into MCP tool definitions
   - **Service**: Orchestrates parsing, conversion, and config generation
   - **Runtime**: MCP server that executes OpenAPI tools via HTTP requests

2. **Integration Points**
   - **Declarative Sync**: Automatically detects `.openapi.json` files during environment sync
   - **MCP Server Registration**: Each OpenAPI spec becomes an MCP server using `stn openapi-runtime`
   - **Tool Discovery**: Standard MCP tool discovery works with OpenAPI-generated tools
   - **Agent Assignment**: Tools can be assigned to agents like any other MCP tools

### Design Principles

1. **No Separate Binaries**: Everything runs through the main `stn` command
2. **Declarative Configuration**: OpenAPI specs are treated as MCP server definitions
3. **Automatic Conversion**: `.openapi.json` files are automatically processed during sync
4. **Standard MCP Protocol**: OpenAPI tools integrate seamlessly with existing MCP infrastructure
5. **Bundle Support**: OpenAPI specs are first-class citizens in Station bundles
6. **Template Variables**: Support for environment-specific API endpoints and credentials

## Complete User Workflow

### Scenario: Developer with Multiple API Integrations

#### Step 1: Add OpenAPI Specifications

User has 5 API specs they want to use:
- `github.openapi.json` - GitHub API
- `stripe.openapi.json` - Stripe payments
- `sendgrid.openapi.json` - Email service
- `twilio.openapi.json` - SMS service
- `slack.openapi.json` - Slack messaging

User adds them to their environment directory:

```bash
~/.config/station/environments/default/
├── github.openapi.json
├── stripe.openapi.json
├── sendgrid.openapi.json
├── twilio.openapi.json
└── slack.openapi.json
```

**OpenAPI Spec Example** (`github.openapi.json`):

```json
{
  "openapi": "3.0.0",
  "info": {
    "title": "GitHub API",
    "version": "1.0.0"
  },
  "servers": [
    {
      "url": "{{ .GITHUB_API_URL }}",
      "description": "GitHub API"
    }
  ],
  "paths": {
    "/user": {
      "get": {
        "operationId": "getAuthenticatedUser",
        "summary": "Get authenticated user",
        "responses": {
          "200": {
            "description": "User information",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "login": { "type": "string" },
                    "email": { "type": "string" },
                    "name": { "type": "string" }
                  }
                }
              }
            }
          }
        }
      }
    },
    "/repos/{owner}/{repo}": {
      "get": {
        "operationId": "getRepository",
        "summary": "Get repository information",
        "parameters": [
          {
            "name": "owner",
            "in": "path",
            "required": true,
            "schema": { "type": "string" }
          },
          {
            "name": "repo",
            "in": "path",
            "required": true,
            "schema": { "type": "string" }
          }
        ],
        "responses": {
          "200": {
            "description": "Repository information"
          }
        }
      }
    }
  }
}
```

#### Step 2: Sync Environment to Load Tools

```bash
$ stn sync default
```

**What Happens During Sync:**

1. **File Detection**
   ```
   DeclarativeSync.SyncEnvironment()
   └─> syncMCPTemplateFiles()
       ├─> Scan for *.json files
       ├─> Filter out *.openapi.json files
       └─> Process OpenAPI specs separately
   ```

2. **OpenAPI Processing** (for each `.openapi.json` file)
   ```
   processOpenAPISpecs()

   For github.openapi.json:
   ├─> Read file content
   ├─> Process template variables: {{ .GITHUB_API_URL }} → https://api.github.com
   ├─> Validate OpenAPI spec using parser
   ├─> Convert to MCP tools:
   │   GET /user → tool: githubgetAuthenticatedUser
   │   GET /repos/{owner}/{repo} → tool: githubgetRepository
   │
   ├─> Generate MCP server configuration:
   │   {
   │     "command": "stn",
   │     "args": ["openapi-runtime", "--config", "inline"],
   │     "env": {
   │       "OPENAPI_MCP_CONFIG": "server:\n  name: github\ntools:\n  - name: githubgetAuthenticatedUser\n    ..."
   │     }
   │   }
   │
   ├─> Save generated config: github-openapi-mcp.json
   ├─> Register MCP server in database
   └─> Discover tools via MCP protocol
   ```

3. **Database State After Sync**
   ```sql
   -- mcp_servers table
   | id | name              | command | args                    |
   |----|-------------------|---------|-------------------------|
   | 1  | github-openapi    | stn     | ["openapi-runtime", ...] |
   | 2  | stripe-openapi    | stn     | ["openapi-runtime", ...] |
   | 3  | sendgrid-openapi  | stn     | ["openapi-runtime", ...] |
   | 4  | twilio-openapi    | stn     | ["openapi-runtime", ...] |
   | 5  | slack-openapi     | stn     | ["openapi-runtime", ...] |

   -- mcp_tools table
   | id | name                       | description          | server_id |
   |----|----------------------------|----------------------|-----------|
   | 1  | githubgetAuthenticatedUser | Get authenticated... | 1         |
   | 2  | githubgetRepository        | Get repository...    | 1         |
   | 3  | stripecreatePayment        | Create payment...    | 2         |
   | 4  | sendgridsendemail          | Send email...        | 3         |
   | 5  | twiliosendSMS              | Send SMS...          | 4         |
   | 6  | slackpostMessage           | Post message...      | 5         |
   ```

#### Step 3: Create Agent with OpenAPI Tools

Create agent configuration file:

```yaml
# ~/.config/station/environments/default/agents/devops-notification-agent.prompt
---
metadata:
  name: "DevOps Notification Agent"
  description: "Monitors GitHub repos and sends notifications"
  tags: ["devops", "notifications", "github"]
model: gpt-4o-mini
max_steps: 10
tools:
  - "__githubgetRepository"
  - "__githubsearchRepositories"
  - "__sendgridsendemail"
  - "__twiliosendSMS"
  - "__slackpostMessage"
---

{{role "system"}}
You are a DevOps monitoring agent that watches GitHub repositories
and sends notifications via email, SMS, or Slack.

When asked to check a repository, use the GitHub API tools.
When sending notifications, choose the appropriate channel (email, SMS, or Slack).

{{role "user"}}
{{userInput}}
```

Sync to register agent:
```bash
$ stn sync default
```

#### Step 4: Run Agent and Execute Tools

```bash
$ stn agent call devops-notification-agent \
  "Check epuerta9/station for new releases and notify team on Slack"
```

**Execution Flow:**

1. **Agent Initialization**
   ```
   AgentExecutionEngine.ExecuteAgent()
   └─> Load agent: devops-notification-agent
       └─> Resolve MCP tools:
           ├─> __githubgetRepository → mcp_tools.id=2 (server_id=1)
           └─> __slackpostMessage → mcp_tools.id=6 (server_id=5)
   ```

2. **Tool Execution: githubgetRepository**
   ```
   Agent decides to call: githubgetRepository
   └─> MCPConnectionManager.CallTool()
       ├─> Find server: github-openapi (server_id=1)
       ├─> Get/create MCP client:
       │   └─> Start subprocess: stn openapi-runtime --config inline
       │       ENV: OPENAPI_MCP_CONFIG="server:\n  name: github\ntools:..."
       │
       ├─> Send MCP request:
       │   {
       │     "jsonrpc": "2.0",
       │     "method": "tools/call",
       │     "params": {
       │       "name": "githubgetRepository",
       │       "arguments": {
       │         "owner": "epuerta9",
       │         "repo": "station"
       │       }
       │     }
       │   }
       │
       └─> OpenAPI Runtime processes:
           ├─> Parse tool: githubgetRepository
           ├─> Lookup operation: GET /repos/{owner}/{repo}
           ├─> Build HTTP request:
           │   GET https://api.github.com/repos/epuerta9/station
           ├─> Execute HTTP request
           ├─> Format response with schema
           └─> Return MCP result:
               {
                 "content": [{
                   "type": "text",
                   "text": "Repository: station\nStars: 125\nLatest release: v0.9.2"
                 }]
               }
   ```

3. **Tool Execution: slackpostMessage**
   ```
   Agent decides to notify team
   └─> Same flow with slack-openapi server
       └─> POST https://slack.com/api/chat.postMessage
           Body: {
             "channel": "#releases",
             "text": "🚀 New Station release v0.9.2 available!"
           }
   ```

4. **Execution Complete**
   ```
   Run saved with metadata:
   ├─> Tools called: 2
   ├─> HTTP requests made: 2
   ├─> Duration: 3.2s
   └─> Status: success
   ```

#### Step 5: Share Setup with Bundle

Create bundle containing OpenAPI specs and agents:

```bash
$ stn bundle create devops-tools --env default
```

**Bundle Structure:**

```
devops-tools.tar.gz
├── template.json
├── variables.yml
├── openapi/
│   ├── github.openapi.json
│   ├── stripe.openapi.json
│   ├── sendgrid.openapi.json
│   ├── twilio.openapi.json
│   └── slack.openapi.json
└── agents/
    └── devops-notification-agent.prompt
```

**Generated `template.json`:**

```json
{
  "name": "DevOps Tools Bundle",
  "version": "1.0.0",
  "description": "GitHub monitoring with multi-channel notifications",
  "station_version": ">=0.9.0",
  "variables": {
    "GITHUB_API_URL": {
      "type": "string",
      "default": "https://api.github.com",
      "description": "GitHub API base URL"
    },
    "SLACK_TOKEN": {
      "type": "string",
      "required": true,
      "description": "Slack API token"
    }
  },
  "openapi_specs": [
    "openapi/github.openapi.json",
    "openapi/stripe.openapi.json",
    "openapi/sendgrid.openapi.json",
    "openapi/twilio.openapi.json",
    "openapi/slack.openapi.json"
  ],
  "agents": [
    {
      "name": "DevOps Notification Agent",
      "file": "agents/devops-notification-agent.prompt",
      "description": "Monitors repos and sends notifications"
    }
  ]
}
```

Share bundle: `devops-tools.tar.gz`

#### Step 6: Buddy Installs Bundle

Buddy receives bundle and installs:

```bash
$ stn bundle install devops-tools.tar.gz --env production
```

**Installation Process:**

1. Extract bundle to temporary directory
2. Read `template.json` and prompt for required variables:
   ```
   Required Variables:
   SLACK_TOKEN: [user enters token]

   Optional Variables:
   GITHUB_API_URL: https://api.github.com [default]
   ```

3. Copy files to environment:
   ```
   ~/.config/station/environments/production/
   ├── github.openapi.json
   ├── stripe.openapi.json
   ├── sendgrid.openapi.json
   ├── twilio.openapi.json
   ├── slack.openapi.json
   ├── variables.yml
   └── agents/
       └── devops-notification-agent.prompt
   ```

4. Run automatic sync:
   ```bash
   $ stn sync production
   ```
   - Detects 5 `.openapi.json` files
   - Converts each to MCP server
   - Registers 5 MCP servers with 58 tools
   - Registers agent with tool bindings

5. Ready to use - buddy can now run:
   ```bash
   $ stn agent call devops-notification-agent \
     "Check kubernetes/kubernetes for CVEs and email security team" \
     --env production
   ```

## Technical Details

### OpenAPI Runtime Server

The `stn openapi-runtime` command implements a complete MCP server that:

1. **Reads Configuration** from `OPENAPI_MCP_CONFIG` environment variable
2. **Implements MCP Protocol** over stdio:
   - `initialize` - Returns server info and capabilities
   - `tools/list` - Returns available OpenAPI tools
   - `tools/call` - Executes HTTP requests based on OpenAPI spec
3. **Executes HTTP Requests**:
   - Builds request from OpenAPI operation definition
   - Substitutes path parameters: `/repos/{owner}/{repo}` → `/repos/epuerta9/station`
   - Adds query parameters from tool arguments
   - Includes request body for POST/PUT/PATCH operations
   - Handles authentication (Bearer, API Key, Basic Auth)
4. **Formats Responses**:
   - Uses OpenAPI schema to structure response
   - Includes helpful metadata about response fields
   - Returns errors in MCP format

### File Naming Convention

- **OpenAPI Spec Files**: `*.openapi.json` (e.g., `github.openapi.json`)
- **Generated MCP Configs**: `*-openapi-mcp.json` (e.g., `github-openapi-mcp.json`)
- **MCP Server Names**: `*-openapi` (e.g., `github-openapi`)

### Template Variable Support

OpenAPI specs can use Go template syntax for environment-specific values:

```json
{
  "servers": [
    {
      "url": "{{ .API_BASE_URL }}",
      "description": "API Server"
    }
  ],
  "components": {
    "securitySchemes": {
      "bearerAuth": {
        "type": "http",
        "scheme": "bearer",
        "bearerFormat": "{{ .API_TOKEN }}"
      }
    }
  }
}
```

Variables are resolved during sync from:
1. `variables.yml` file in environment directory
2. Environment variables
3. User prompts (for bundle installation)

## API Endpoints (Future)

The following API endpoints will be added for UI integration:

### Add OpenAPI Spec
```http
POST /api/v1/environments/{env}/openapi-specs
Content-Type: application/json

{
  "name": "github-api",
  "spec": "{ OpenAPI spec JSON }",
  "variables": {
    "API_BASE_URL": "{{ .GITHUB_API_URL }}"
  }
}
```

### List OpenAPI Specs
```http
GET /api/v1/environments/{env}/openapi-specs
```

### Update OpenAPI Spec
```http
PUT /api/v1/environments/{env}/openapi-specs/{name}
```

### Delete OpenAPI Spec
```http
DELETE /api/v1/environments/{env}/openapi-specs/{name}
```

## Current Status

### ✅ Completed

- OpenAPI package with parser, converter, and runtime
- Fixed kin-openapi v0.127.0 compatibility issues
- Integrated OpenAPI detection into declarative sync
- Template variable processing for OpenAPI specs
- MCP server registration and config generation
- `stn openapi-runtime` command implementation
- MCP protocol support (initialize, tools/list, tools/call)
- HTTP request execution from OpenAPI definitions

### ⚠️ Known Issues

- MCP client connection to `stn openapi-runtime` subprocess needs debugging
- Runtime works standalone but connection manager reports "client not initialized"
- Issue appears to be in stdio communication between Station's MCP connection manager and the OpenAPI runtime subprocess

### 🔜 Planned

- Fix MCP client subprocess connection
- Add API endpoints for UI integration
- Write comprehensive tests
- Add bundle creation/installation support for OpenAPI specs
- Support for OpenAPI 3.1 specifications
- Advanced authentication handling (OAuth2 flows)
- Response caching and rate limiting

## Examples

See `docs/examples/openapi/` for complete working examples of:
- GitHub API integration
- Stripe payments
- SendGrid email
- Twilio SMS
- Slack messaging

## References

- [OpenAPI Specification 3.0](https://swagger.io/specification/)
- [MCP Protocol Documentation](https://modelcontextprotocol.io)
- [kin-openapi Library](https://github.com/getkin/kin-openapi)
