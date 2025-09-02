# Station Project - Agent Architecture Analysis

## Overview

The Station project implements a sophisticated AI agent execution system with custom OpenAI plugin fixes and comprehensive GenKit integration. This document maps out the agent execution flow, loop mechanisms, and AI integration points.

## Core Components

### 1. Agent Execution Engine
- **File**: `internal/services/agent_execution_engine.go`
- **Main Entry Point**: `AgentExecutionEngine.ExecuteAgentViaStdioMCPWithVariables()`
- **Purpose**: Orchestrates the entire agent execution lifecycle with MCP tools and dotprompt rendering

### 2. GenKit Executor
- **File**: `pkg/dotprompt/genkit_executor.go`
- **Core Agent Loop**: `generateWithCustomTurnLimit()` - Main AI conversation loop
- **Tool Call Tracking**: `ToolCallTracker` monitors tool usage to prevent infinite loops

### 3. Agent Service
- **File**: `internal/services/agent_service_impl.go`
- **Interface Implementation**: Provides standardized agent execution interface
- **Direct Execution**: Uses AgentExecutionEngine directly (no queue complexity)

## Agent Execution Flow

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────────┐
│   User Command  │───→│   CLI Handler    │───→│   Agent Service     │
│  stn agent run  │    │  (cmd/main/)     │    │ (AgentServiceImpl)  │
└─────────────────┘    └──────────────────┘    └─────────────────────┘
                                                           │
                                                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                 Agent Execution Engine                              │
│          (internal/services/agent_execution_engine.go)             │
│                                                                     │
│  ExecuteAgentViaStdioMCPWithVariables()                           │
│  ├─ Load MCP Tools (40 tool limit)                                │
│  ├─ Filter Tools by Agent Config                                  │
│  ├─ Initialize Variables/Context                                   │
│  └─ Execute via GenKit                                            │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    GenKit Executor                                  │
│              (pkg/dotprompt/genkit_executor.go)                    │
│                                                                     │
│  ExecuteAgentWithDatabaseConfigAndLogging()                       │
│  ├─ Setup OpenAI Provider                                         │
│  ├─ Configure Tool Limits                                         │
│  └─ Call generateWithCustomTurnLimit()                           │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      MAIN AGENT LOOP                               │
│                                                                     │
│  generateWithCustomTurnLimit() - 10min timeout                    │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                    Conversation Loop                        │   │
│  │                                                             │   │
│  │  ┌─── genkit.Generate() ←─────────────────────────┐        │   │
│  │  │         │                                      │        │   │
│  │  │         ▼                                      │        │   │
│  │  │  ┌─────────────┐    ┌────────────────┐       │        │   │
│  │  │  │ OpenAI API  │───→│  Tool Calling  │───────┘        │   │
│  │  │  │ (via custom │    │   & Execution  │                 │   │
│  │  │  │   plugin)   │    └────────────────┘                 │   │
│  │  │  └─────────────┘                                       │   │
│  │  │                                                         │   │
│  │  └─── Loop Control: ────────────────────────────────────────   │
│  │       • Turn Count: 40 max                                │   │
│  │       • Tool Calls: 25 max                               │   │
│  │       • Same Tool: 3 consecutive max                     │   │
│  │       • Timeout: 10 minutes                              │   │
│  └─────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

## Detailed Execution Sequence

1. **User Command**: `stn agent run "Agent Name" "Task"`
2. **CLI Handler** → AgentService.ExecuteAgent()
3. **AgentExecutionEngine**.ExecuteAgentViaStdioMCPWithVariables()
4. **MCP Tool Loading & Filtering** (respects 40-tool limit per agent)
5. **GenKitExecutor**.ExecuteAgentWithDatabaseConfigAndLogging()
6. **generateWithCustomTurnLimit()** - THE MAIN AGENT LOOP
7. **genkit.Generate()** with retry logic and turn limits
8. **Tool execution**, response processing, loop detection

## Agent Loop Implementation

### Main Loop Function
- **Location**: `generateWithCustomTurnLimit()` in `genkit_executor.go`
- **Timeout**: 10-minute timeout for complex analysis tasks
- **Retry System**: 3-retry system with exponential backoff (2s, 4s, 8s delays)

### Loop Control Mechanisms

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Loop Control                                 │
│                                                                     │
│  1. Turn Limiting                                                  │
│     └─ Forces final response after 40 conversation turns          │
│                                                                     │
│  2. Tool Call Tracking                                            │
│     ├─ Max 25 tool calls per conversation                         │
│     ├─ Max 3 consecutive calls to same tool                       │
│     └─ Detection of repetitive patterns                           │
│                                                                     │
│  3. Timeout Handling                                              │
│     ├─ 10-minute overall timeout                                  │
│     ├─ 2-minute per API call timeout                              │
│     └─ Final response generation on timeout                       │
│                                                                     │
│  4. Pattern Detection                                              │
│     ├─ Mass tool calling (>5 tools in single message)            │
│     ├─ Obsessive calling (same tool repeatedly)                   │
│     └─ Information gathering loops (>80% read-only tools)         │
└─────────────────────────────────────────────────────────────────────┘
```

## OpenAI Plugin Integration Points

### Core OpenAI Plugin Implementation
- **`internal/genkit/openai.go`** - Main Station OpenAI plugin wrapper with custom fixes
- **`internal/genkit/compat_oai.go`** - Station's OpenAI compatibility layer 
- **`internal/genkit/generate.go`** - Core generation logic with tool_call_id bug fixes
- **`internal/genkit/README.md`** - Documentation of Station's OpenAI plugin fixes

### OpenAI Plugin Usage Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                     OpenAI Plugin Usage                            │
│                                                                     │
│  Station's Custom OpenAI Plugin (internal/genkit/openai.go)       │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  Fixes for tool_call_id bugs in upstream GenKit           │   │
│  │  ├─ Custom Generate() implementation                       │   │
│  │  ├─ Proper tool calling sequence                          │   │
│  │  └─ Enhanced error handling                               │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  Used in:                                                          │
│  ├─ Agent conversations (main use case)                           │
│  ├─ GitHub repository analysis                                    │
│  ├─ Template placeholder analysis                                 │
│  ├─ Agent planning operations                                     │
│  └─ Any genkit.Generate() call                                   │
└─────────────────────────────────────────────────────────────────────┘
```

### Service Layer Integration
- **`internal/services/genkit_provider.go`** - OpenAI provider setup and configuration
- **`internal/services/github_discovery.go`** - Uses OpenAI for GitHub repo analysis
- **`internal/services/intelligent_placeholder_analyzer.go`** - AI-powered template analysis
- **`internal/services/agent_execution_engine.go`** - Agent execution with OpenAI models

## GenKit Integration Architecture

### Primary GenKit Integration Files
- **`pkg/dotprompt/genkit_executor.go`** - Main executor that uses `genkit.Generate()` for AI operations
- **`internal/services/genkit_provider.go`** - Manages Genkit initialization and AI provider configuration
- **`internal/genkit/generate.go`** - Station's custom OpenAI model generator with tool calling fixes
- **`internal/genkit/openai.go`** - Station's fixed OpenAI plugin implementation

### GenKit Usage Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                        GenKit Usage Flow                           │
│                                                                     │
│  1. Provider Setup (internal/services/genkit_provider.go)         │
│     ├─ OpenAI Provider Registration                               │
│     ├─ Model Configuration (gpt-4o, gpt-4o-mini)                 │
│     └─ API Key Management                                         │
│                            │                                       │
│                            ▼                                       │
│  2. Prompt Management (.genkit/ directory)                        │
│     ├─ DotPrompt files (.prompt)                                 │
│     ├─ Variable interpolation                                     │
│     └─ Template rendering                                         │
│                            │                                       │
│                            ▼                                       │
│  3. Generation Execution                                          │
│     ├─ genkit.Generate() calls                                   │
│     ├─ Tool integration via MCP                                  │
│     ├─ Conversation management                                   │
│     └─ Response processing                                       │
│                            │                                       │
│                            ▼                                       │
│  4. Telemetry & Monitoring                                       │
│     ├─ OpenTelemetry traces                                      │
│     ├─ Execution logging                                         │
│     └─ Performance metrics                                       │
└─────────────────────────────────────────────────────────────────────┘
```

### Configuration & Setup
- **`internal/config/config.go`** - OpenAI API key and configuration management
- **`cmd/main/provider_setup.go`** - OpenAI provider configuration UI
- **`cmd/main/server.go`** - Server-mode OpenAI plugin initialization
- **`cmd/main/develop.go`** - Development-mode OpenAI setup

## Complete System Architecture

```
User Input → CLI/API → Agent Service → Execution Engine
                                          │
                                          ▼
                              ┌─────────────────────┐
                              │   MCP Tool Setup    │
                              │   - 40 tool limit   │
                              │   - Tool filtering  │
                              └─────────────────────┘
                                          │
                                          ▼
                              ┌─────────────────────┐
                              │  GenKit Executor   │
                              │  - OpenAI Plugin   │
                              │  - Custom Fixes    │
                              └─────────────────────┘
                                          │
                                          ▼
                              ┌─────────────────────┐
                              │   Agent Loop       │
                              │   - 40 turns max   │
                              │   - 10min timeout  │
                              │   - Loop prevention │
                              └─────────────────────┘
                                          │
                                          ▼
                              ┌─────────────────────┐
                              │    Response        │
                              │    - Cleanup       │
                              │    - Logging       │
                              │    - Telemetry     │
                              └─────────────────────┘
```

## Key Configuration Parameters

### Timeouts
- **Main execution**: 10 minutes
- **Per API call**: 2 minutes

### Limits
- **Tools per agent**: 40 (enforced at assignment)
- **Tool calls per conversation**: 25
- **Conversation turns**: 40
- **Consecutive same tool calls**: 3

### Entry Points
- **CLI**: `cmd/main/handlers/agent/local.go`
- **API**: `internal/api/v1/agents.go`
- **MCP**: `internal/mcp/handlers_fixed.go`
- **Scheduler**: `internal/services/scheduler.go`

## Connection Management

### MCP Connection Manager
- **File**: `internal/services/mcp_connection_manager.go`
- **Features**:
  - Connection Pooling: Optional persistent MCP server connections
  - Tool Caching: Environment-specific tool caching with TTL
  - Server Pool: Reuses connections across agent executions
- **Environment Variable**: `STATION_MCP_POOLING=true` enables connection pooling

### Scheduling System
- **File**: `internal/services/scheduler.go`
- **Features**:
  - Cron-based: Uses robfig/cron for agent scheduling
  - Direct Execution: Uses AgentService directly (no queue complexity)
  - Persistent State: Tracks scheduled agents in database

## Custom OpenAI Plugin Features

Station has implemented its own enhanced version of the GenKit OpenAI plugin with critical fixes:

1. **Custom OpenAI Plugin** - Station's own fixed version of GenKit's OpenAI plugin
2. **Multi-turn Agent Conversations** - Fixed tool_call_id bugs for proper OpenAI tool calling  
3. **Multiple AI Providers** - OpenAI, OpenAI-compatible APIs, Gemini support
4. **Progressive Logging** - Real-time OpenAI API call tracking
5. **Agent Execution Engine** - Full agent workflows using OpenAI models
6. **Template & Code Analysis** - AI-powered placeholder detection and GitHub discovery

## Load Handlers (OpenAI-powered)

- **`cmd/main/handlers/load/github.go`** - GitHub discovery with OpenAI analysis
- **`cmd/main/handlers/load/templates.go`** - Template analysis using OpenAI
- **`cmd/main/handlers/load/turbotax.go`** - Tax form processing with AI

## Telemetry & Monitoring

- **`internal/telemetry/genkit_telemetry_client.go`** - Captures Genkit execution traces
- **`internal/telemetry/otel_plugin.go`** - OpenTelemetry integration with Genkit

## Documentation & Bug Reports

- **`docs/bug-reports/STATION_OPENAI_PLUGIN_FIX.md`** - Comprehensive OpenAI plugin fix documentation
- **`docs/bug-reports/genkit-go-openai-tool-call-id-bug.md`** - Original bug analysis

## Architecture Notes

The codebase shows evidence of recent architectural simplification:
- **Removed**: ExecutionQueueService complexity
- **Simplified**: Direct AgentService execution
- **Improved**: Better error handling and timeout management
- **Enhanced**: Turn limiting and loop prevention

This architecture provides a robust, well-controlled agent execution system with proper loop prevention, timeout handling, and resource management.
