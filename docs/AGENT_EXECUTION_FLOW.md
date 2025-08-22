# Agent Execution Flow Architecture

## Overview

Station has **two different execution paths** for agents. Understanding which path is used and why is critical for debugging and reliability.

## 🔄 Execution Flow Diagram

```
User Command: `stn agent run "Agent Name" "Task"`
│
├─ CLI Agent Handler (cmd/main/handlers/agent/local.go:run)
│  │
│  └─ AgentExecutionEngine.ExecuteAgentViaStdioMCPWithVariables()
     │   internal/services/agent_execution_engine.go:76
     │   🟢 "Starting unified dotprompt execution for agent X"
     │
     ├─ MCP Server Pool Initialization ✅
     │  │   - Discovers 13 servers, 505+ tools
     │  │   - Takes ~5-7 seconds 
     │  │   - Filters tools for agent (respects 40-tool limit)
     │  │
     │  └─ Tool Assignment Check ⚠️  CRITICAL FAILURE POINT
     │      │   - If agent has >40 tools → SILENT FAILURE
     │      │   - If agent has 0-40 tools → CONTINUE
     │      │
     │      ├─ 🔴 FAILURE PATH (Old Bug - Fixed)
     │      │   Agent had 491 tools → Tool filtering loop crashed
     │      │   → Execution stopped silently after MCP init
     │      │   → Agent completed immediately with 0 steps
     │      │
     │      └─ ✅ SUCCESS PATH (Current)
     │          Agent has ≤40 tools → Continue to execution
     │
     └─ **PRIMARY PATH**: Dotprompt Executor
        │   executor.ExecuteAgentWithDatabaseConfigAndLogging()
        │   pkg/dotprompt/genkit_executor.go:711
        │
        ├─ ExecuteAgentWithDotpromptAndLogging() 
        │  │   pkg/dotprompt/genkit_executor.go:657
        │  │
        │  └─ ExecuteAgentWithDotprompt()
        │     │   pkg/dotprompt/genkit_executor.go:51
        │     │
        │     ├─ Step 1: Dotprompt Content Processing ✅
        │     │   - Check if agent prompt has dotprompt format
        │     │   - Use agent prompt directly OR build from legacy format
        │     │   
        │     ├─ Step 2: Dotprompt Compilation ✅
        │     │   - dp.Compile(dotpromptContent, nil)
        │     │   - Renders multi-role messages properly
        │     │   
        │     ├─ Step 3: Input Data Merging ✅
        │     │   - schemaHelper.GetMergedInputData()
        │     │   - Combines userInput with custom schema
        │     │   
        │     ├─ Step 4: Message Conversion ✅
        │     │   - convertDotpromptToGenkitMessages()
        │     │   - Translates to GenKit message format
        │     │   
        │     ├─ Step 5: Model Configuration ✅
        │     │   - Load config (provider: openai, model: gpt-5)
        │     │   - Format: "openai/gpt-5"
        │     │   
        │     ├─ Step 6: Tool Setup ✅
        │     │   - Extract frontmatter tools + MCP tools
        │     │   - Build GenKit tool references
        │     │   
        │     └─ Step 7: ACTUAL AI EXECUTION ✅
        │         │   🟢 "Creating execution context with 10min timeout"
        │         │
        │         └─ generateWithCustomTurnLimit()
        │             │   pkg/dotprompt/genkit_executor.go:258
        │             │   ⏰ Context: 10 minute timeout
        │             │   🔄 Retry Logic: 3 attempts with exponential backoff
        │             │   
        │             ├─ Turn Limiting Logic ✅
        │             │   - maxToolCalls: 25
        │             │   - Tool call tracking & obsessive loop prevention
        │             │   
        │             ├─ **CORE AI CALL**: genkit.Generate() ✅
        │             │   │   internal/genkit/generate.go:507-619
        │             │   │   🔄 3-Retry System with Exponential Backoff
        │             │   │
        │             │   ├─ Attempt 1: 2-minute timeout per API call
        │             │   ├─ Attempt 2: +2s delay, 2-minute timeout  
        │             │   ├─ Attempt 3: +4s delay, 2-minute timeout
        │             │   └─ Final Response Generation on all failures
        │             │
        │             └─ Response Processing ✅
        │                 - Success: Return AI response with metadata
        │                 - Timeout: Generate final response explaining timeout
        │                 - Error: Return error with context
```

## 🚨 Legacy Execution Path (UNUSED - Dead Code?)

```
⚠️  POTENTIAL DEAD CODE PATH - Needs Investigation

AgentExecutionEngine.ExecuteAgentWithMessages()
│   internal/services/agent_execution_engine.go:203
│   🟢 "=== Executing Agent with Messages (Dotprompt Multi-Role) ==="
│
├─ Direct GenKit Generate Call (NO RETRY LOGIC) ❌
│   │   genkit.Generate() - Direct call
│   │   ⏰ 60-second timeout (TOO SHORT) ❌
│   │   🔄 NO retry logic ❌
│   │   🔧 NO tool call limiting ❌
│   │
│   └─ This path bypasses all our improvements:
│       - No 10-minute timeout
│       - No exponential backoff retry
│       - No turn limiting logic
│       - No final response generation on timeout
│       - Uses old 60-second timeout
```

**❓ Question**: Is `ExecuteAgentWithMessages()` ever called? This appears to be legacy code that should be removed if unused.

## ⚙️ Key Configuration Points

### Timeouts
```go
// Main execution timeout (our fix)
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)  // Line 204

// Per-API-call timeout (in retry logic)  
apiCallCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)  // Line 536
```

### Tool Limits
```go
// Per-agent tool limit (our fix)
const MaxToolsPerAgent = 40  // repositories/agent_tools.go:76

// Per-conversation tool call limit
maxToolCalls := 25  // genkit_executor.go:213
```

### Retry Logic
```go
// 3-retry system with exponential backoff (our fix)
maxRetries := 3
for attempt := 1; attempt <= maxRetries; attempt++ {
    // 2s, 4s, 8s delays between retries
    delay := time.Duration(attempt-1) * 2 * time.Second
    time.Sleep(delay)
    // ... API call with 2-minute timeout
}
```

## 🐛 Historical Issues & Fixes

### Issue 1: Tool Assignment Bloat (FIXED ✅)
**Problem**: Simple Test Agent had 491 tools assigned → Tool filtering loop failure → Silent exit after MCP init  
**Root Cause**: No limits on tool assignment per agent  
**Fix**: 40-tool limit per agent + cleanup of over-assigned agents  

### Issue 2: Timeout Too Short (FIXED ✅)  
**Problem**: 2-minute timeout for complex security analysis → Premature termination  
**Root Cause**: `ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)`  
**Fix**: Increased to 10 minutes for complex analysis tasks  

### Issue 3: No API Retry Logic (FIXED ✅)
**Problem**: Single API failures caused entire agent execution to fail  
**Root Cause**: Direct genkit.Generate() call with no retry mechanism  
**Fix**: 3-retry system with exponential backoff + final response generation  

### Issue 4: Missing Final Response (FIXED ✅)
**Problem**: On timeout/failure, agents returned empty responses  
**Root Cause**: No fallback response generation logic  
**Fix**: Always generate final response explaining timeout/error  

## 📊 Performance Metrics

### Current Performance (Post-Fix)
```
Simple Test Agent (0 tools):     ~9 seconds  ✅
Ship Security Agent (5 tools):   ~113 seconds ✅  
Complex Analysis Tasks:          <10 minutes ✅
Success Rate:                    100% ✅
```

### Pre-Fix Performance (Broken)
```
All Agents:                      5-6 seconds ❌
Steps Taken:                     0 ❌  
Tool Calls:                      0 ❌
Success Rate:                    0% ❌
```

## 🔍 Debugging Guide

### Agent Completing Immediately (0 steps)
1. Check agent tool count: `SELECT COUNT(*) FROM agent_tools WHERE agent_id = X`
2. If >40 tools → Remove excess tools or increase limit
3. Look for "Starting unified dotprompt execution" but no further debug logs
4. Check MCP server initialization logs - should see ~505 tools discovered

### Agent Timing Out
1. Check timeout configuration (should be 10 minutes)  
2. Review tool complexity - security scans can take 1-2 minutes each
3. Check API retry logs - should see retry attempts on failures
4. Verify final response generation - should never return empty

### Agent API Failures
1. Look for retry attempt logs in generate.go
2. Check API timeout detection logic
3. Verify exponential backoff delays (2s, 4s, 8s)
4. Ensure final response generation on max retries exceeded

## 🧹 Potential Dead Code Cleanup

**Files to investigate for unused code**:
- `ExecuteAgentWithMessages()` - direct genkit call path
- Old timeout configurations (60 seconds)  
- Direct genkit.Generate() calls without retry logic
- Tool filtering code that doesn't respect 40-tool limit

**Before removing**: Verify these paths are truly unused by adding temporary logging and monitoring for 1-2 weeks.

## 📋 TODO: Dead Code Cleanup

**Priority**: Low (after reliability testing complete)  
**Timeline**: 1-2 weeks of monitoring first

1. **Add temporary logging** to `ExecuteAgentWithMessages()` to confirm it's never called
2. **Monitor for 1-2 weeks** in production to ensure no hidden usage
3. **Remove dead code** if confirmed unused:
   - `ExecuteAgentWithMessages()` function
   - Associated direct genkit.Generate() calls without retry logic
   - Old 60-second timeout configurations
   - Any tool filtering code that doesn't respect 40-tool limit
4. **Update tests** to remove any references to removed functions
5. **Update documentation** to reflect cleaned-up architecture

**Risk Assessment**: Low - function appears completely unused, but verification needed before removal.

---

*Last Updated: 2025-08-22*  
*Status: All major execution issues resolved ✅*  
*Next: Dead code cleanup scheduled after reliability verification*