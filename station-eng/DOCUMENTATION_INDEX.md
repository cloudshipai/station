# Station Engineering Documentation Index

**Complete technical reference for Station AI Agent Template Platform**

*Based on actual codebase analysis of Station v0.2.0 with Agent Template System - Generated 2025-08-07*

## 📚 Documentation Map

### **🏗️ Architecture Documentation**
| File | Purpose | When to Read |
|------|---------|--------------|
| `architecture/overview.md` | **High-level system architecture** | Starting any new work, need big picture |
| `architecture/layers.md` | **Detailed layer mapping with code refs** | Understanding component relationships |

### **🧩 Component Documentation** 
| File | Purpose | When to Read |
|------|---------|--------------|
| `components/handlers/README.md` | **Handler patterns and structure** | Working on CLI commands, request processing |
| `components/mcp-server/README.md` | **MCP server implementation details** | MCP tool development, AI integration |
| `components/universal-mcp-client/README.md` | **Universal MCP client architecture** | Integrating external MCP servers, transport issues |

### **🎯 Feature Documentation**
| File | Purpose | When to Read |
|------|---------|--------------|
| `features/agent-template-system.md` | **Complete Agent Template System** | Working on templates, understanding core innovation |
| `features/execution-tracing.md` | **Agent execution tracing system** | Understanding agent debugging, telemetry, execution analysis |

### **🗄️ Data Layer Documentation**
| File | Purpose | When to Read |
|------|---------|--------------|
| `data/database.md` | **Database architecture, sqlc patterns** | Schema changes, query optimization |

### **⚙️ Configuration Documentation**
| File | Purpose | When to Read |
|------|---------|--------------|
| `configuration/file-based.md` | **File-based config system** | Config management, template system |

### **📋 Development Standards**
| File | Purpose | When to Read |
|------|---------|--------------|
| `conventions/rules.md` | **Development rules and patterns** | Before making any code changes |
| `guides/development.md` | **Practical development workflows** | Daily development, common tasks |

## 🎯 Quick Reference by Scenario

### **"We're working on Agent Template System"**
**Essential Reading:**
1. `features/agent-template-system.md` - Complete template system overview
2. `components/handlers/README.md` - Handler integration with templates
3. `conventions/rules.md` - Template development patterns

**Key Code Locations:**
- Template core: `pkg/agent-bundle/` (Creator, Validator, Manager, Resolver)
- CLI handlers: `cmd/main/handlers/agent/handlers.go` (bundle commands)
- API integration: `internal/api/v1/agents.go` (template installation)

### **"We're working on agent handlers next"**
**Essential Reading:**
1. `components/handlers/README.md` - Handler architecture and Agent Bundle integration
2. `features/agent-template-system.md` - Template system integration
3. `conventions/rules.md` - Handler naming and structure rules

**Key Code Locations:**
- Handler implementations: `cmd/main/handlers/agent/handlers.go` (includes bundle commands)
- Agent repository: `internal/db/repositories/agents.go`
- Template system: `pkg/agent-bundle/manager/manager.go`

### **"We need to understand the MCP layer"**
**Essential Reading:**
1. `components/mcp-server/README.md` - Station's MCP server (tools for AI systems)
2. `components/universal-mcp-client/README.md` - Universal MCP client (consuming external MCP servers)
3. `configuration/file-based.md` - MCP configuration and transport support

**Key Code Locations:**
- MCP server: `internal/mcp/server.go`, `internal/mcp/tools_setup.go`
- Universal MCP client: `internal/services/tool_discovery_client.go`
- MCP handlers: `internal/mcp/handlers_fixed.go`

### **"We're adding MCP transport support"**
**Essential Reading:**
1. `components/universal-mcp-client/README.md` - Transport architecture and examples
2. `configuration/file-based.md` - Configuration parsing and template formats
3. `conventions/rules.md` - Error handling and logging patterns

**Key Code Locations:**
- Transport factory: `internal/services/tool_discovery_client.go:createTransport()`
- Config parsing: `internal/services/file_config_service.go:LoadAndRenderConfigSimple()`
- Models: `pkg/models/models.go:MCPServerConfig`

### **"We need to debug MCP connections"**
**Essential Reading:**
1. `components/mcp-server/README.md` - Complete MCP server overview
2. `architecture/overview.md` - MCP's role in the platform
3. `conventions/rules.md` - MCP tool development rules

**Key Code Locations:**
- MCP tools: `internal/mcp/tools_setup.go`
- MCP handlers: `internal/mcp/handlers_fixed.go`
- MCP server: `internal/mcp/server.go`

### **"Database schema changes needed"**
**Essential Reading:**
1. `data/database.md` - Database patterns and sqlc usage
2. `conventions/rules.md` - Database development rules
3. `guides/development.md` - Migration workflow

**Key Code Locations:**
- Migrations: `internal/db/migrations/`
- Queries: `internal/db/queries/`
- Repositories: `internal/db/repositories/`

### **"Configuration loading issues"**
**Essential Reading:**
1. `configuration/file-based.md` - File-based configuration system
2. `architecture/layers.md` - Configuration flow through layers

**Key Code Locations:**
- Config service: `internal/services/file_config_service.go`
- Config handlers: `cmd/main/handlers/file_config/`
- Template engine: `internal/template/engine.go`

### **"Adding new CLI commands"**
**Essential Reading:**
1. `components/handlers/README.md` - Handler patterns
2. `guides/development.md` - Adding new commands workflow
3. `conventions/rules.md` - Command naming conventions

**Key Code Locations:**
- Command definitions: `cmd/main/commands.go`
- Command registration: `cmd/main/main.go`
- Handler implementations: `cmd/main/handlers/`

## 📊 Codebase Statistics (Analyzed)

### **Project Structure**
- **Total Go files**: 212 files
- **Total lines of code**: ~49,392 lines
- **Handler files**: 50+ files in `cmd/main/handlers/`
- **Service files**: 15+ files in `internal/services/`
- **Repository files**: 10+ files in `internal/db/repositories/`

### **Key Directories**
```
Station Codebase (Verified):
├── cmd/main/                    # CLI interface (20+ files)
│   └── handlers/                # Request handlers (50+ files)
├── internal/
│   ├── mcp/                     # MCP server (10+ files) 
│   ├── services/                # Business logic (15+ files)
│   ├── db/                      # Data layer (30+ files)
│   ├── api/                     # REST API (5+ files)
│   └── ssh/                     # SSH server (5+ files)
├── pkg/                         # Public packages (5+ files)
└── examples/                    # MCP templates (5+ files)
```

### **Database Schema**
- **Core tables**: agents, agent_runs, environments, users
- **MCP tables**: mcp_configs, mcp_tools, agent_tools  
- **Infrastructure**: webhooks, webhook_deliveries
- **Migration files**: 16+ migrations
- **sqlc queries**: 10+ query files

## 🎨 Architecture Patterns (Verified)

### **Handler Pattern**
- **Local/Remote variants** for environment flexibility
- **Consistent error handling** across all handlers
- **Theme management integration** for CLI output
- **Command flag processing** standardized

### **Repository Pattern**
- **sqlc-generated queries** - no hand-written SQL
- **Type-safe database operations** via generated code
- **Consistent CRUD interfaces** across all repositories
- **Transaction support** for complex operations

### **Service Pattern**
- **Business logic encapsulation** separate from handlers
- **Dependency injection** for testability
- **Cross-cutting concerns** (logging, telemetry) handled at service layer

### **Configuration Pattern**
- **File-based with templates** - not database storage
- **Environment-specific variables** with encryption support
- **GitOps-friendly** version control integration
- **AI-powered variable detection** for automation

## 🔧 Technical Decisions (Documented)

### **Database: SQLite + sqlc**
- **Why SQLite**: Embedded, zero-config, ACID compliant, backup-friendly
- **Why sqlc**: Type safety, performance, SQL-first approach, no ORM overhead
- **Pattern**: Write SQL queries, generate Go code, use in repositories

### **Configuration: File-Based**
- **Why File-Based**: GitOps compatibility, version control, team collaboration  
- **Template System**: Go templates with variable resolution
- **Security**: Encrypted variables, environment isolation
- **Pattern**: Templates + Variables → Rendered Configs → MCP Servers

### **MCP Integration: mark3labs/mcp-go**
- **Tools vs Resources**: Tools modify state, Resources provide data
- **11+ Management Tools**: Complete agent lifecycle management
- **Stdio + HTTP modes**: Flexible AI system integration
- **Pattern**: Tool definition → Handler implementation → Business logic

### **Architecture: Multi-Modal**
- **Four Interfaces**: CLI, SSH, API, MCP - shared core services
- **Layer Separation**: Presentation → Handler → Service → Repository → Data
- **Dependency Direction**: Each layer only depends on layers below
- **Pattern**: Request → Validation → Business Logic → Data Access → Response

## 🚀 Development Workflow (Established)

### **Code Changes**
1. **Analyze existing patterns** before implementing
2. **Follow established conventions** from `conventions/rules.md`
3. **Use appropriate tools** (sqlc for DB, templates for config)
4. **Test locally** before committing
5. **Update documentation** if behavior changes

### **Database Changes**  
1. **Create migration** in `internal/db/migrations/`
2. **Update queries** in `internal/db/queries/`
3. **Run sqlc generate** to update Go code
4. **Update repositories** with new methods
5. **Test thoroughly** with existing data

### **MCP Tools**
1. **Define tool** in `tools_setup.go` with parameters
2. **Implement handler** in `handlers_fixed.go`
3. **Follow response patterns** for consistency
4. **Test with AI client** (Claude Code, etc.)
5. **Document tool purpose** and usage

## 📝 Documentation Maintenance

### **Keeping Documentation Current**
- **Code Analysis**: All docs based on actual code inspection
- **Version Tracking**: Update version references when Station releases
- **Pattern Updates**: Adjust patterns when codebase conventions change
- **New Components**: Add docs for new major components

### **Documentation Standards**
- **Code References**: Include actual file paths and line numbers where possible
- **ASCII Diagrams**: Visual representations of architecture and flow
- **Practical Examples**: Real code snippets from the codebase
- **Quick Reference**: Scenario-based navigation for common tasks

---

## 🎯 How to Use This Documentation

### **For New Team Members**
1. Start with `README.md` (this file) for overview
2. Read `architecture/overview.md` for system understanding
3. Review `conventions/rules.md` for development standards
4. Use `guides/development.md` for practical workflows

### **For Specific Tasks**
1. **Check the scenario index above** for your task type
2. **Read the Essential Reading** documents listed
3. **Examine the Key Code Locations** for implementation details
4. **Follow established patterns** from existing code

### **For System Understanding**
1. **Architecture documents** provide the big picture
2. **Component documents** dive into specific areas
3. **Layer mapping** shows how pieces connect
4. **Code analysis** reveals actual implementation patterns

---

*This documentation system provides comprehensive technical reference for Station development. It's designed to get you from "new to the codebase" to "productive contributor" as quickly as possible.*

**Last updated**: 2025-08-07 for Station v0.2.0 with Agent Template System  
**Next update**: When major architectural changes occur or new features are added