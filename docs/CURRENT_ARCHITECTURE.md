# Station Current Architecture (2025)

## Overview

Station is a secure, self-hosted platform for creating intelligent multi-environment MCP agents. The system has undergone a major architectural overhaul, migrating from database-based encrypted storage to a modern file-based configuration system with modular CLI architecture.

## 🏗️ Modular CLI Architecture

### Handler Module Structure
Station's CLI has been completely refactored from 5 monolithic files (5,777 lines) into **43 focused modules**, each under 500 lines for maximum maintainability:

```
cmd/main/handlers/
├── agent/ (3 files)           # Agent management
├── file_config/ (16 files)    # File-based configuration  
├── load/ (10 files)           # Configuration loading
├── mcp/ (6 files)             # MCP server operations
├── webhooks/ (8 files)        # Webhook management
└── common/ (1 file)           # Shared utilities
```

### Benefits
- **Single Responsibility**: Each module has one clear purpose
- **Easy Navigation**: Logical grouping by feature area
- **Reduced Conflicts**: Developers work in different modules
- **Simplified Testing**: Each module can be tested independently
- **Clean Dependencies**: No circular imports

## 📁 File-Based Configuration System

### Core Concept
Station now uses a **GitOps-ready file-based configuration system** instead of encrypted SQLite storage:

```
config/
├── environments/
│   ├── development/
│   │   ├── github-tools.json      # Template with {{.Variables}}
│   │   ├── aws-tools.json         # Environment-specific configs
│   │   └── variables.env          # Secret values (gitignored)
│   ├── staging/
│   └── production/
└── templates/
    └── shared/                     # Reusable templates
```

### Key Features
- **GitOps Ready**: All templates version controlled
- **Secret Separation**: Variables stored separately from templates
- **Template System**: Go template syntax with variable substitution
- **Environment Isolation**: Separate configs per environment
- **Auto-Discovery**: GitHub repo analysis for MCP server detection

### Template Example
```json
{
  "name": "GitHub Integration",
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "{{.GitHub.Token}}",
        "GITHUB_REPO_ACCESS": "{{.GitHub.RepoAccess}}"
      }
    }
  }
}
```

## 🔧 Service Architecture

### Core Services
- **FileConfigService**: Template management and variable resolution
- **ToolDiscoveryService**: MCP server connection and tool discovery  
- **AgentService**: Agent creation, execution, and monitoring
- **WebhookService**: Real-time notifications and integrations
- **ExecutionQueueService**: Async agent execution management

### Database Schema (Streamlined)
```sql
-- Core tables (cleaned up)
users, environments, agents, agent_runs
mcp_tools, agent_tools          # Tool assignment system
file_mcp_configs               # File-based config metadata
webhooks, webhook_deliveries   # Notification system

-- Removed old tables
mcp_configs ❌                 # Old encrypted storage
template_variables ❌          # Old variable system  
config_migrations ❌           # Old migration tracking
```

### Data Flow
1. **Template Storage**: File templates stored in filesystem
2. **Variable Resolution**: Environment-specific variables injected
3. **Config Rendering**: Go templates rendered to final configuration
4. **Tool Discovery**: MCP servers connected, tools discovered
5. **Agent Assignment**: Tools assigned to agents based on capabilities

## 🚀 Agent Intelligence System

### Self-Bootstrapping Architecture
Station uses its own MCP server for agent management:

```
Agent Creation Request
       ↓
Station's MCP Server (stdio)
       ↓  
AI Tool Selection (Genkit)
       ↓
Agent Creation with Optimal Tools
```

### Features
- **AI-Driven Tool Selection**: Genkit analyzes requirements and assigns optimal tools
- **Self-Managing**: Station manages itself via its own MCP server
- **Context-Aware Execution**: Dynamic iteration limits based on task complexity
- **Multi-Provider Support**: OpenAI (default), Ollama, Gemini with fallbacks

## 🔐 Security Model

### File-Based Security
- **Template Separation**: Public templates, private variables
- **Environment Isolation**: Separate credential sets per environment
- **GitOps Friendly**: Templates can be version controlled safely
- **Secret Management**: Variables stored in gitignored files

### Access Control
- **Local Mode**: Direct filesystem access
- **Server Mode**: API authentication with admin/user roles
- **Environment Boundaries**: Strict isolation between dev/staging/prod
- **Audit Logging**: All operations tracked and logged

## 🌐 Integration Points

### GitOps Workflow
```bash
# 1. Create template (version controlled)
stn config create github-tools --template

# 2. Set environment variables (not version controlled)  
stn config variables set --env production GitHub.Token=ghp_xxx

# 3. Load configuration
stn load github-tools.json --env production

# 4. Deploy via CI/CD
git commit -m "Add GitHub integration"
git push origin main
```

### MCP Protocol Integration
- **Stdio Mode**: Station provides its own MCP server
- **HTTP Mode**: Traditional client-server MCP communication
- **Tool Discovery**: Automatic MCP server capability detection
- **Version Management**: File-based config versioning

### Webhook Integrations
- **Real-time Notifications**: Agent completion events
- **HMAC Security**: Signed webhook payloads
- **Delivery Tracking**: Full audit trail of webhook deliveries
- **Multi-Platform Support**: Slack, Discord, PagerDuty, custom endpoints

## 📊 Performance Characteristics

### Benchmarks (Current)
| Operation | Time | Status |
|-----------|------|---------|
| System Init | 2.1s | ✅ Excellent |
| Config Loading | 1.8s avg | ✅ Fast |
| Agent Creation | 6.5s avg | ✅ Good |
| Agent Execution | 10.8s avg | ✅ Acceptable |
| Tool Discovery | 1.5s avg | ✅ Excellent |

### Scalability
- **File System**: Scales to thousands of configurations
- **Template Rendering**: Sub-second variable substitution
- **Concurrent Agents**: 5 worker queue (configurable)
- **Tool Discovery**: Parallel MCP server connections

## 🔄 Migration Completed

### What Was Changed
- ✅ **Database Schema**: Removed 5 old tables, streamlined to file-based metadata
- ✅ **Handler Architecture**: 5 large files → 43 focused modules  
- ✅ **Configuration System**: SQLite encryption → File templates + variables
- ✅ **CLI Commands**: Updated all commands for file-based system
- ✅ **Service Layer**: New FileConfigService, updated dependencies
- ✅ **TUI Components**: Migrated to file-based configuration display

### What Stayed the Same
- ✅ **Agent Execution**: Same reliable execution engine
- ✅ **MCP Protocol**: Full compatibility maintained
- ✅ **Webhook System**: Unchanged notification system  
- ✅ **Tool Assignment**: Same flexible tool assignment model
- ✅ **API Endpoints**: Backward compatible API structure

## 🚀 Next Steps

### Immediate Opportunities
1. **Enhanced Load Function**: Improve configuration loading mechanisms
2. **Template Library**: Build shared template repository
3. **Visual Config Editor**: TUI-based template editing
4. **Auto-Discovery**: Enhanced GitHub repo scanning

### Long-term Vision
- **Configuration Marketplace**: Share templates across teams
- **Policy Engine**: Configuration validation and compliance
- **Multi-Cluster**: Deploy agents across multiple Station instances
- **AI Config Generation**: LLM-generated configuration templates

---

**Station's modular, file-based architecture provides a solid foundation for scalable, maintainable AI agent operations while maintaining security and flexibility.**