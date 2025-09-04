# Dotprompt Execution Flow - The Brain of Station

This document shows the complete flow of how Station executes agents using the dotprompt system with GenKit.

## 🧠 Complete Execution Flow

```
┌─ Agent Execution Request ─────────────────────────────────┐
│ task: "analyze this codebase for security vulnerabilities" │
│ agent: { name, prompt, tools, max_steps }                  │
│ mcpTools: [filesystem, security scanners, etc.]           │
└────────────────────────────┬──────────────────────────────┘
                             │
┌─ 1. DOTPROMPT COMPILATION ─▼──────────────────────────────┐
│ pkg/dotprompt/genkit_executor.go:ExecuteAgentWith...()    │
│                                                           │
│ Agent Prompt:                      Compiled Dotprompt:   │
│ ┌─────────────────────┐            ┌────────────────────┐ │
│ │ ---                 │   COMPILE  │ ---                │ │
│ │ model: gpt-4o-mini  │  ─────────▶ │ model: gpt-4o-mini │ │
│ │ tools:              │            │ tools: [fs, scan]  │ │
│ │  - __read_file      │            │ ---                │ │
│ │  - __security_scan  │            │                    │ │
│ │ ---                 │            │ {{role "system"}}  │ │
│ │ {{role "system"}}   │            │ You are expert... │ │
│ │ You are expert...   │            │ {{role "user"}}    │ │
│ │ {{role "user"}}     │            │ {{userInput}}      │ │
│ │ {{userInput}}       │            └────────────────────┘ │
│ └─────────────────────┘                                  │
│                                                           │
│ dotprompt.Compile(content) → promptFunc                  │
└────────────────────────────┬──────────────────────────────┘
                             │
┌─ 2. PROMPT RENDERING ──────▼──────────────────────────────┐
│                                                           │
│ Input Data:                        Rendered Messages:     │
│ ┌─────────────────────┐            ┌────────────────────┐ │
│ │ {                   │   RENDER   │ Message[0]:        │ │
│ │   "userInput":      │  ─────────▶ │   Role: "system"   │ │
│ │     "analyze this   │            │   Content: "You    │ │
│ │      codebase..."   │            │   are expert..."   │ │
│ │ }                   │            │ Message[1]:        │ │
│ └─────────────────────┘            │   Role: "user"     │ │
│                                    │   Content: "analyze│ │
│ promptFunc(data) → renderedPrompt  │   this codebase..." │ │
│                                    └────────────────────┘ │
└────────────────────────────┬──────────────────────────────┘
                             │
┌─ 3. GENKIT MESSAGE CONVERSION ─▼─────────────────────────┐
│                                                           │
│ Dotprompt Messages:              GenKit ai.Messages:     │
│ ┌─────────────────────┐            ┌────────────────────┐ │
│ │ [{                  │  CONVERT   │ []*ai.Message{     │ │
│ │   Role: "system",   │  ─────────▶ │   &ai.Message{     │ │
│ │   Parts: [...]      │            │     Role: "system",│ │
│ │  },{                │            │     Content: [...]  │ │
│ │   Role: "user",     │            │   },               │ │
│ │   Parts: [...]      │            │   &ai.Message{     │ │
│ │ }]                  │            │     Role: "user",  │ │
│ └─────────────────────┘            │     Content: [...] │ │
│                                    │   }                │ │
│ convertDotpromptToGenkitMessages() │ }                  │ │
│                                    └────────────────────┘ │
└────────────────────────────┬──────────────────────────────┘
                             │
┌─ 4. MODEL & TOOL CONFIGURATION ──▼───────────────────────┐
│                                                           │
│ Model Resolution:                Tools Setup:             │
│ ┌─────────────────────┐            ┌────────────────────┐ │
│ │ Config.AIProvider:  │            │ MCP Tools:         │ │
│ │   "openai"          │  RESOLVE   │ • __read_text_file │ │
│ │ Config.AIModel:     │  ─────────▶ │ • __list_directory │ │
│ │   "gpt-4o-mini"     │            │ • __security_scan  │ │
│ │                     │            │ • __git_log        │ │
│ │ Result:             │            │ • [25 more tools]  │ │
│ │ "openai/gpt-4o-mini"│            └────────────────────┘ │
│ └─────────────────────┘                                  │
│                                                           │
│ generateOpts = [                                          │
│   ai.WithModelName("openai/gpt-4o-mini"),               │
│   ai.WithMessages(genkitMessages...),                    │
│   ai.WithTools(mcpTools...),                            │
│   ai.WithMaxTurns(30)                                    │
│ ]                                                         │
└────────────────────────────┬──────────────────────────────┘
                             │
┌─ 5. GENKIT GENERATE EXECUTION ──▼────────────────────────┐
│                                                           │
│ 🔥 THE CRITICAL CALL: genkit.Generate(ctx, app, opts...) │
│                                                           │
│ ┌─ GenKit Processing Loop ─────────────────────────────┐  │
│ │                                                     │  │
│ │ Turn 1:                                             │  │
│ │ ┌─ System + User Messages ──┐                       │  │
│ │ │ • "You are expert..."      │  ─────────────────┐   │  │
│ │ │ • "analyze this codebase"  │                   │   │  │
│ │ └────────────────────────────┘                   │   │  │
│ │                                                  │   │  │
│ │ ┌─ AI Model Response ────────┐                   │   │  │
│ │ │ • "I'll analyze by reading │ ◄─────────────────┘   │  │
│ │ │    key files first..."     │                       │  │
│ │ │ • Tool calls:              │                       │  │
│ │ │   - __read_text_file       │                       │  │  
│ │ │     (src/main.go)          │                       │  │
│ │ │   - __list_directory       │                       │  │
│ │ │     (./src/)               │                       │  │
│ │ └────────────────────────────┘                       │  │
│ │                                                     │  │
│ │ Turn 2:                                             │  │
│ │ ┌─ Tool Results ─────────────┐                       │  │
│ │ │ • File contents: "package  │                       │  │
│ │ │   main\nimport..."         │  ─────────────────┐   │  │
│ │ │ • Directory: ["auth.go",   │                   │   │  │
│ │ │   "handler.go", "db.go"]   │                   │   │  │
│ │ └────────────────────────────┘                   │   │  │
│ │                                                  │   │  │
│ │ ┌─ AI Model Response ────────┐                   │   │  │
│ │ │ • "Found potential SQL     │ ◄─────────────────┘   │  │
│ │ │    injection in db.go..."  │                       │  │
│ │ │ • Tool calls:              │                       │  │
│ │ │   - __security_scan        │                       │  │
│ │ │     (db.go)                │                       │  │
│ │ │   - __read_text_file       │                       │  │
│ │ │     (auth.go)              │                       │  │
│ │ └────────────────────────────┘                       │  │
│ │                                                     │  │
│ │ Continues until:                                    │  │
│ │ • AI provides final answer (no more tool calls)    │  │
│ │ • Max turns reached (30)                           │  │
│ │ • Tool call limits hit (25 total, 3 consecutive)   │  │
│ │ • Context timeout (10 minutes)                     │  │
│ └─────────────────────────────────────────────────────┘  │
│                                                           │
│ Returns: ai.ModelResponse{                                │
│   Message: final_response,                                │
│   Usage: { input_tokens, output_tokens },                 │
│   Conversations: [...all_turns...]                        │
│ }                                                         │
└────────────────────────────┬──────────────────────────────┘
                             │
┌─ 6. RESULT PROCESSING ─────▼──────────────────────────────┐
│                                                           │
│ GenKit Response:                 Execution Response:      │
│ ┌─────────────────────┐            ┌────────────────────┐ │
│ │ ai.ModelResponse{   │  EXTRACT   │ ExecutionResponse{ │ │
│ │   Message: "Found 3 │  ─────────▶ │   Success: true,   │ │
│ │     security issues │            │   Response: "Found │ │
│ │     in your code...",│            │     3 security...", │
│ │   Usage: {          │            │   Duration: 45s,   │ │
│ │     InputTokens: 1250│            │   TokenUsage: {...},│ │
│ │     OutputTokens: 830│            │   ToolCalls: 8,    │ │
│ │   },                │            │   StepsTaken: 12   │ │
│ │   Conversations: [...]│           │ }                  │ │
│ │ }                   │            └────────────────────┘ │
│ └─────────────────────┘                                  │
│                                                           │
│ • Extract final response text                             │
│ • Parse token usage statistics                            │
│ • Count tool calls from conversation turns                │
│ • Calculate total duration                                │
│ • Format for database storage                             │
└────────────────────────────┬──────────────────────────────┘
                             │
┌─ 7. DATABASE STORAGE ──────▼──────────────────────────────┐
│                                                           │
│ AgentRuns Table Update:                                   │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ ID: 1234                                               │ │  
│ │ Status: "completed"                                    │ │
│ │ FinalResponse: "Found 3 security issues..."           │ │
│ │ StepsTaken: 12                                         │ │
│ │ InputTokens: 1250                                      │ │
│ │ OutputTokens: 830                                      │ │
│ │ ToolCalls: [...detailed_call_log...]                  │ │
│ │ ExecutionSteps: [...step_by_step_log...]              │ │
│ │ Duration: 45.2 seconds                                 │ │
│ │ ModelName: "openai/gpt-4o-mini"                        │ │
│ │ CompletedAt: 2025-01-15T10:30:45Z                     │ │
│ └─────────────────────────────────────────────────────────┘ │
│                                                           │
│ repos.AgentRuns.UpdateCompletionWithMetadata()           │
└────────────────────────────┬──────────────────────────────┘
                             │
┌─ 8. CLI RESULT DISPLAY ────▼──────────────────────────────┐
│                                                           │
│ 📋 Execution Results                                      │
│ Run ID: 1234                                              │  
│ Status: completed                                         │
│ Started: 2025-01-15T10:29:00Z                            │
│ Completed: 2025-01-15T10:30:45Z (took 1m 45s)           │
│                                                           │
│ Result:                                                   │
│ Found 3 security issues in your codebase:                │
│                                                           │
│ 1. SQL Injection in db.go line 45:                       │
│    query := "SELECT * FROM users WHERE id = " + userID   │
│                                                           │
│ 2. Hardcoded API key in config.go line 12:              │
│    const API_KEY = "sk-abcd1234..."                      │
│                                                           │
│ 3. Missing input validation in auth.go line 67:          │
│    password := r.FormValue("password")                    │
│                                                           │
│ Token Usage:                                              │
│   Input tokens: 1,250                                     │
│   Output tokens: 830                                      │
│                                                           │
│ Tool Calls: 8                                             │
│ ✅ Agent execution completed!                             │
└───────────────────────────────────────────────────────────┘
```

## 🎯 Key Components Explained

**1. Dotprompt Compilation**: Agent's prompt template gets compiled into executable function  
**2. Prompt Rendering**: Template variables (like `{{userInput}}`) get replaced with actual values  
**3. Message Conversion**: Dotprompt format → GenKit's ai.Message format  
**4. Tool Setup**: MCP tools become available to the AI model  
**5. GenKit Generate**: The **CORE** - multi-turn conversation with tool calling  
**6. Result Processing**: Raw AI response → structured execution data  
**7. Storage**: Complete run metadata saved to database  
**8. Display**: Formatted results shown to user  

## 🔥 The Critical Call

The **absolute heart** is `genkit.Generate()` - this is where the AI model:
- Reads the system prompt and user task
- Decides what tools to call  
- Executes tools through MCP
- Processes tool results
- Continues conversation until final answer
- Returns complete conversation history + metadata

## File Locations

- **Execution Engine**: `internal/services/agent_execution_engine.go`
- **Dotprompt Executor**: `pkg/dotprompt/genkit_executor.go`  
- **Agent Handler**: `cmd/main/handlers/agent/local.go`
- **CLI Entry**: `cmd/main/agent.go`

This is Station's **bread and butter** - the complete flow from dotprompt template to final security analysis results!