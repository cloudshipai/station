# Station Architecture Overview

Station is a **template-driven AI agent platform** with four primary interfaces, shared core services, and a comprehensive Agent Template System.

## 🏗️ High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    STATION TEMPLATE PLATFORM                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐   │
│  │   CLI   │  │   SSH   │  │   API   │  │   MCP Server    │   │
│  │ Commands│  │ TUI :2222│  │REST :8080│  │ Tools (stdio)   │   │
│  └─────────┘  └─────────┘  └─────────┘  └─────────────────┘   │
│       │            │            │                │             │
│       └────────────┼────────────┼────────────────┘             │
│                    │            │                              │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              CORE SERVICES & TEMPLATE SYSTEM           │   │
│  │                                                         │   │
│  │  Agent Service │ Template System │ Config Service │    │   │
│  │  MCP Service   │ Dependency Mgmt │ Execution Queue │    │   │
│  └─────────────────────────────────────────────────────────┘   │
│                    │                                            │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │           DATA LAYER & TEMPLATE STORAGE                │   │
│  │                                                         │   │
│  │  SQLite DB │ Agent Templates │ File Configs │ Repos    │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## 🎯 Four Primary Interfaces

### 1. **CLI Interface** (`cmd/main/`)
- **Purpose**: Primary user interface for agent management
- **Commands**: `stn agent create`, `stn load`, `stn runs list`, etc.
- **Implementation**: Cobra-based commands with handlers
- **Entry Point**: `cmd/main/main.go`

### 2. **SSH Server** (`internal/ssh/`) 
- **Purpose**: Retro TUI for system administration
- **Port**: 2222 
- **Features**: Terminal UI with agent management, file browsing
- **Implementation**: Charm SSH + Bubble Tea TUI

### 3. **REST API** (`internal/api/`)
- **Purpose**: HTTP API for external integrations
- **Port**: 8080
- **Endpoints**: `/api/v1/agents`, `/api/v1/runs`, etc.
- **Implementation**: Gin-based REST API

### 4. **MCP Server** (`internal/mcp/`)
- **Purpose**: Model Context Protocol server for AI integration
- **Port**: 3000 (or stdio mode)
- **Tools**: 11+ management tools for AI systems
- **Implementation**: mark3labs/mcp-go based server

## 📊 Data Flow Architecture

```
┌─────────────┐    ┌──────────────┐    ┌─────────────┐
│   Request   │───▶│   Handler    │───▶│   Service   │
│ (CLI/SSH/   │    │              │    │             │
│  API/MCP)   │    │ - Validation │    │ - Business  │
│             │    │ - Transform  │    │   Logic     │
└─────────────┘    └──────────────┘    └─────────────┘
                                              │
┌─────────────┐    ┌──────────────┐           ▼
│  Response   │◀───│  Repository  │    ┌─────────────┐
│             │    │              │    │   Models    │
│ - JSON/Text │    │ - SQL Ops    │    │             │
│ - Formatted │    │ - CRUD       │    │ - Agents    │
│   Output    │    │ - Queries    │    │ - Runs      │
└─────────────┘    └──────────────┘    │ - Configs   │
                                       └─────────────┘
```

## 🗂️ Layer Responsibilities

### **Presentation Layer** 
- **CLI Handlers** (`cmd/main/handlers/`): Command processing and user interaction
- **API Controllers** (`internal/api/v1/`): HTTP request/response handling  
- **SSH Apps** (`internal/ssh/apps/`): TUI applications and screens
- **MCP Handlers** (`internal/mcp/`): MCP tool implementations

### **Business Logic Layer**
- **Services** (`internal/services/`): Core business logic and orchestration
- **Agent Service**: Agent lifecycle management  
- **Config Service**: Configuration processing and validation
- **MCP Service**: Tool discovery and execution
- **Webhook Service**: Event notifications

### **Data Access Layer**
- **Repositories** (`internal/db/repositories/`): Data access patterns
- **Models** (`pkg/models/`): Domain entities and structures
- **Database** (`internal/db/`): SQLite connection and migrations
- **File System** (`internal/filesystem/`): File-based configuration

### **Infrastructure Layer**
- **Config Management** (`internal/config/`): Application configuration
- **Templates** (`internal/template/`): Configuration templates
- **Telemetry** (`internal/telemetry/`): Usage analytics
- **Crypto** (`pkg/crypto/`): Encryption and security

## 🚀 Execution Flow Example

**User runs: `stn agent create --name "Test Agent"`**

```
1. CLI Command     │ cobra.Command triggers runAgentCreate()
                   │
2. Handler         │ cmd/main/handlers/agent/handlers.go
                   │ ├─ Validates input parameters  
                   │ ├─ Loads Station configuration
                   │ └─ Routes to local or remote handler
                   │
3. Service         │ internal/services/agent_service_impl.go
                   │ ├─ Business logic validation
                   │ ├─ Default value assignment
                   │ └─ Calls repository layer  
                   │
4. Repository      │ internal/db/repositories/agents.go  
                   │ ├─ SQL query via sqlc
                   │ ├─ Database transaction
                   │ └─ Returns domain model
                   │
5. Response        │ Success/error propagated back up
                   │ Formatted output to user
```

## 🔧 Key Design Patterns

### **Handler Pattern**
- Local/Remote variants for environment flexibility
- Consistent input validation and error handling
- Configuration loading and dependency injection

### **Repository Pattern** 
- sqlc-generated type-safe SQL queries
- Transaction management and connection handling
- Domain model mapping

### **Service Pattern**
- Business logic encapsulation
- Cross-cutting concerns (logging, telemetry)
- Service-to-service communication

### **Configuration Pattern**
- File-based with template variable resolution
- Environment-specific overrides
- GitOps-friendly version control

---
*This overview provides the foundation - dive into specific components for implementation details.*