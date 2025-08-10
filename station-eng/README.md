# Station Engineering Documentation

**Technical reference for the Station AI Agent Template Platform codebase**

This documentation provides comprehensive technical mapping of Station's architecture, components, and conventions for LLM contributors and internal engineering use.

## 🎁 **Major Update: Agent Template System (v0.2.0)**

Station has evolved into a **template-driven AI agent platform**. Key architectural additions:
- **Agent Bundle System**: Complete template lifecycle (`pkg/agent-bundle/`)
- **Template Management**: Create, validate, install, export, duplicate templates
- **Multi-Environment Deployment**: Variable-driven configuration across environments
- **Dependency Resolution**: Real MCP dependency management with conflict detection

## 📁 Documentation Structure

```
station-eng/
├── architecture/          # System architecture and design patterns
├── components/            # Detailed component documentation  
├── data/                 # Database, models, and data layer
├── configuration/        # Config management and template systems
├── conventions/          # Code patterns, rules, and best practices
├── guides/              # Development workflows and debugging
├── features/            # Major feature documentation (Agent Templates, etc.)
└── diagrams/           # ASCII diagrams and visual aids
```

## 🚀 Quick Start for LLM Contributors

**New to Station?** Essential reading order:
1. `architecture/overview.md` - Complete system architecture
2. `features/agent-template-system.md` - Template system overview  
3. `components/handlers/README.md` - Handler patterns and structure
4. `conventions/rules.md` - Development standards and patterns

**Working on specific areas?**
- **Agent Templates**: `features/agent-template-system.md` + `pkg/agent-bundle/`
- **CLI/Commands**: `components/handlers/` + handler implementations
- **MCP Integration**: `components/mcp-server/` + `internal/mcp/`
- **Database**: `data/database.md` + sqlc patterns
- **File Configs**: `configuration/file-based.md` + template engine
- **API Endpoints**: `components/api-server/` + REST API patterns

## 🎯 Common Development Scenarios

**"Working on Agent Template System"**
→ Read: `features/agent-template-system.md` + `pkg/agent-bundle/` code analysis

**"Adding new agent bundle commands"**  
→ Read: `components/handlers/README.md` + `cmd/main/handlers/agent/handlers.go`

**"Understanding template variable resolution"**
→ Read: `configuration/file-based.md` + `internal/template/engine.go`

**"Working on MCP dependency resolver"**
→ Read: `components/mcp-server/` + `pkg/agent-bundle/resolver/resolver.go`

**"Database schema changes needed"**
→ Read: `data/database.md` + sqlc patterns in `internal/db/`

**"Configuration loading issues"**
→ Read: `configuration/file-based.md` + `internal/services/file_config_service.go`

**"Adding new API endpoints"**
→ Read: `components/api-server/` + `internal/api/v1/` implementations

## 📋 Key Technical Decisions (Updated v0.2.0)

- **Agent Templates**: Complete template lifecycle with `pkg/agent-bundle` package
- **Database**: SQLite with sqlc for type-safe queries + repository pattern
- **Configuration**: File-based GitOps + Go template engine with variable resolution  
- **Architecture**: Multi-modal (CLI + SSH + API + MCP) with shared services
- **Template System**: Variable substitution, dependency resolution, multi-environment
- **Handlers**: Local/Remote pattern + Agent Bundle management integration

## 🎁 Agent Template System Architecture

```
Agent Template Lifecycle:
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Create    │───▶│   Validate  │───▶│   Install   │───▶│   Manage    │
│             │    │             │    │             │    │             │  
│ - Creator   │    │ - Validator │    │ - Manager   │    │ - Database  │
│ - Templates │    │ - Schema    │    │ - Variables │    │ - Status    │
│ - Scaffold  │    │ - Dependencies│    │ - Resolver │    │ - Export    │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
```

**Core Components:**
- **Creator** (`pkg/agent-bundle/creator/`): Template scaffolding and export  
- **Validator** (`pkg/agent-bundle/validator/`): Comprehensive validation with suggestions
- **Manager** (`pkg/agent-bundle/manager/`): Installation, duplication, CRUD operations
- **Resolver** (`pkg/agent-bundle/resolver/`): MCP dependency resolution and conflict handling

## 🔄 Keep This Updated

This documentation is **designed for LLM contributors** to quickly understand Station's architecture and contribute effectively. When making changes:

1. **Update component docs** when adding new features
2. **Add scenario examples** for common development patterns  
3. **Include code references** with actual file paths
4. **Maintain architecture diagrams** for visual understanding

## 🤖 For AI Contributors

This documentation is optimized for AI systems contributing to Station development:

- **Explicit file paths** and code locations for easy navigation
- **Scenario-based navigation** for common development tasks
- **Architecture patterns** with code examples
- **Complete feature documentation** with implementation details

---

**Station v0.2.0** - Agent Template Platform  
*Last updated: 2025-08-07 - Agent Template System Complete*