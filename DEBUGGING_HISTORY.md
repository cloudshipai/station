# Station Debugging History

*This file tracks critical debugging moments and architectural decisions. Not tracked in git to keep private development insights.*

## 2025-09-01: The Great MCP Tool Execution Mystery

### **The Problem**
- Ship MCP tools and all other MCP tools were returning generic "Tool X executed successfully" messages with `duration_ms: 0`
- Agents claimed to successfully perform complex tasks (browser navigation, security scans) but were actually getting mock responses
- Users thought security scanning was working but were getting hallucinated results instead of real vulnerability analysis

### **The Deep Dive Investigation**
After extensive debugging through GenKit's execution flow, we discovered:

1. **GenKit MCP Integration WAS working correctly**
   - MCP servers discovered properly (21 Playwright tools, 10 Ship tools)
   - Tools registered natively with GenKit via `RegisterAction key=/tool/__browser_*`
   - Station's middleware passed tools correctly

2. **The MODULAR-GENERATOR Bug**
   - A component called "MODULAR-GENERATOR" was immediately pre-executing ALL available tools with empty parameters
   - Debug logs showed: `"🔧 MODULAR-GENERATOR: Created tool call for __browser_navigate with EMPTY INPUT - THIS IS THE BUG!"`
   - This was completely wrong behavior - GenKit should wait for LLM to decide which tools to call

3. **The Root Cause Discovery**
   - Located in `/home/epuerta/projects/hack/station/pkg/agent/modular_generator_reference.go:235`
   - The `executeToolsWithProtection()` method had: `Input: map[string]interface{}{}, // ❌ THIS IS THE PROBLEM`
   - It was creating tool calls for ALL available tools instead of waiting for LLM requests

4. **The Architecture Issue**
   - `StationModelGenerator` in `/internal/genkit/generate.go` was calling `agent.NewModularGenerator()`
   - Despite comments saying ModularGenerator was "unused", it was actively being used
   - This created two conflicting execution paths: GenKit's native MCP vs Station's mock executor

### **The Refactor Context**
This bug was introduced during our GenKit refactor which aimed to:
1. ✅ Clean interfaces and modular sections for OpenAI plugin and GenKit integration
2. ✅ Context overflow protection with threshold management  
3. ✅ Closer GenKit native practices
4. ❌ **Solidified agent loop** - This was implemented incorrectly

The ModularGenerator misunderstood the purpose of an agent loop and pre-executed all tools instead of letting the LLM make selective tool calls.

### **The Fix**
**Decision: Remove ModularGenerator entirely and delegate directly to GenKit's native generation**

Rationale:
- The original intent was to work closer with GenKit native practices
- Context protection and turn limiting can be handled by Station's middleware
- GenKit should handle the agent loop and tool execution natively
- This eliminates the dual execution path problem

**Implementation:**
- Modified `StationModelGenerator.Generate()` to call GenKit directly instead of ModularGenerator
- Preserved Station's enhancements (context protection, turn limiting) via middleware
- Let GenKit handle MCP tool execution natively

### **Impact Assessment**
- **Severity**: Critical - All MCP tool execution was broken
- **Security Risk**: Security agents were providing hallucinated results instead of real scans
- **User Impact**: Agents appeared functional but were returning fake data
- **Duration**: Unknown - could have been affecting production systems

### **Lessons Learned**
1. **Agent loops should NOT pre-execute tools** - they should wait for LLM decisions
2. **Mock executors are dangerous** - they can mask real functionality failures
3. **"Reference" code can accidentally be used** - better to remove unused code entirely
4. **Tool execution must return real data** - generic success messages create dangerous illusions
5. **Systematic debugging pays off** - tracing execution flow revealed the exact issue

### **Key Debug Commands Used**
```bash
# Found the compiled messages in binary
strings ~/.local/bin/stn | grep "MODULAR-GENERATOR"

# Traced execution flow through logs
stn agent run "Playwright Agent" "Navigate to google.com" --tail

# Located the problematic code
grep -r "executeToolsWithProtection" --include="*.go"
```

This debugging session took several hours but revealed a critical architecture flaw that was causing all MCP integrations to fail silently while appearing to work.

---
*Entry Date: 2025-09-01*  
*Debugged By: Claude Code Assistant*  
*Impact: Critical system functionality restore*