# GenKit v1.2.0 Tool Registration Incompatibility

**Status**: Known Issue  
**Priority**: Medium  
**Affected Versions**: GenKit v1.2.0+  
**Current Workaround**: Stay on GenKit v1.1.0  
**Date Identified**: 2025-11-24  

## Problem Summary

Station is currently pinned to GenKit v1.1.0 because v1.2.0 introduces a breaking change in how dynamic tools are registered and looked up, causing "tool not found" errors during agent execution.

## Root Cause

GenKit v1.2.0 introduced a change in PR #3753 ("Register dynamic tools using sub-registrygers") that modified the tool registration behavior in `go/ai/prompt.go`:

```go
// BEFORE (v1.1.0):
for _, t := range newTools {
    t.Register(p.registry)  // Register in parent registry
}

// AFTER (v1.2.0):
r = r.NewChild()  // Create child registry
for _, t := range newTools {
    t.Register(r)  // Register in child registry
}
```

### Impact on Station

Station's execution flow:
1. **Tool Registration**: We call `genkit.RegisterAction(genkitApp, tool)` which registers tools in the **parent registry**
2. **Tool Lookup**: GenKit v1.2.0 prompt execution looks up tools in a **child registry** created with `r.NewChild()`
3. **Result**: Tools registered in parent registry cannot be found in child registry → "tool not found" errors

### Error Message

```
❌ Agent execution failed: dotprompt execution failed: dotprompt.Execute() failed: 
ai.GenerateWithRequest: tool "acm_describe_certificate" not found
```

## Verification Tests

### ✅ GenKit v1.1.0 (Working)

Both OpenAI and Gemini work perfectly:

```bash
# OpenAI test
stn agent run 4 "Test tool calling with OpenAI"
✅ Agent execution completed
Tool Calls: 3
Result: Successfully created and verified /tmp/model-test.txt

# Gemini test  
stn agent run 4 "Test tool calling with Gemini"
✅ Agent execution completed
Tool Calls: 3
Result: Tool calling test was successful
```

### ❌ GenKit v1.2.0 (Broken)

Both providers fail with same "tool not found" error despite tools being registered correctly:

```bash
DEBUG: Registering tool: acm_describe_certificate (type: *services.StrippedPrefixTool, def.Name: acm_describe_certificate)
DEBUG: Registered 9 tools, skipped 0 already-registered tools for agent infra_sre
❌ Agent execution failed: tool "acm_describe_certificate" not found
```

## Attempted Fixes (Failed)

### 1. Tool Name Prefix Stripping
**Hypothesis**: Thought GenKit/Gemini was stripping `__` prefix from tool names  
**Approach**: Created `StrippedPrefixTool` wrapper to expose tools without `__` prefix  
**Result**: Failed - tools still not found because the issue is registry-based, not name-based  

### 2. Custom Tool Registration
**Hypothesis**: Thought tool `Register()` method needed modification  
**Approach**: Modified `StrippedPrefixTool.Register()` to use different registration patterns  
**Result**: Failed - tools registered in wrong registry regardless of registration method  

## Solution Paths

### Option 1: Stay on GenKit v1.1.0 (Current)
**Pros**:
- ✅ Works perfectly with OpenAI and Gemini
- ✅ No code changes needed
- ✅ Stable and tested

**Cons**:
- ❌ Miss out on GenKit v1.2.0+ features and bug fixes
- ❌ Security updates in newer versions

**Status**: **Currently Implemented**

### Option 2: Adapt to GenKit v1.2.0 Child Registry Pattern
**Required Changes**:

1. **Modify Tool Registration**: Instead of calling `genkit.RegisterAction()` globally, we need to pass tools differently to prompt execution
2. **Update Execution Flow**: Modify `pkg/dotprompt/genkit_executor.go` to register tools in a way compatible with child registries
3. **Possible Approach**:
   ```go
   // Instead of pre-registering tools with RegisterAction:
   // genkit.RegisterAction(genkitApp, tool)
   
   // Pass raw tools directly to Execute via WithTools:
   resp, err := agentPrompt.Execute(ctx,
       ai.WithInput(inputMap),
       ai.WithMaxTurns(maxTurns),
       ai.WithModelName(modelName),
       ai.WithTools(mcpTools...))  // GenKit should handle registration internally
   ```

**Pros**:
- ✅ Stay current with GenKit releases
- ✅ Access to latest features and bug fixes

**Cons**:
- ❌ Requires code refactoring
- ❌ Need thorough testing across all agents
- ❌ Risk of introducing regressions

### Option 3: Report Upstream to GenKit
**Action**: File issue with GenKit team explaining the breaking change impact  
**Expected Outcome**: 
- GenKit might revert the change
- Or provide migration guide for tool registration patterns
- Or document the new expected registration flow

**Link**: https://github.com/firebase/genkit/pull/3753

## Technical Details

### GenKit Version History
- `v0.6.2` → `v1.0.1`: Major upgrade, Station migrated successfully
- `v1.0.1` → `v1.0.3`: Station upgraded successfully  
- `v1.0.3` → `v1.1.0`: Station upgraded successfully (current)
- `v1.1.0` → `v1.2.0`: **Breaks tool registration** ❌

### Key Files Affected
- `pkg/dotprompt/genkit_executor.go`: Tool registration via `genkit.RegisterAction()`
- `internal/services/mcp_connection_manager.go`: MCP tool discovery and wrapping
- `go.mod`: GenKit version pinned to `v1.1.0`

### Relevant Code Locations
```go
// Tool registration (pkg/dotprompt/genkit_executor.go:164)
genkit.RegisterAction(genkitApp, tool)

// Tool usage (pkg/dotprompt/genkit_executor.go:223-227)
resp, err := agentPrompt.Execute(execCtx,
    ai.WithInput(inputMap),
    ai.WithMaxTurns(maxTurns),
    ai.WithModelName(modelName),
    ai.WithTools(mcpTools...))
```

## Testing Checklist for v1.2.0 Compatibility

When attempting to upgrade to GenKit v1.2.0+:

- [ ] Test OpenAI with filesystem tools (Model Test Agent)
- [ ] Test Gemini with filesystem tools (Model Test Agent)
- [ ] Test OpenAI with AWS faker tools (infra_sre agent)
- [ ] Test Gemini with AWS faker tools (infra_sre agent)
- [ ] Test multi-tool agents (Station SRE agents with 6+ tools)
- [ ] Test agent-as-tool (hierarchical agent execution)
- [ ] Verify tool call metadata capture in runs
- [ ] Check Jaeger trace integration for tool calls

## References

- GenKit PR #3753: https://github.com/firebase/genkit/pull/3753
- GenKit v1.2.0 Release: https://github.com/firebase/genkit/releases/tag/go/v1.2.0
- Station Issue Discovery Session: 2025-11-24 debugging session with Claude
- Tool Registration Debug Logs: Shows tools registered correctly but lookup fails

## Action Items

- [ ] Monitor GenKit releases for v1.2.1+ that might address this
- [ ] Consider filing upstream issue if not already reported
- [ ] Document any community workarounds discovered
- [ ] Plan v1.2.0 compatibility work for future sprint

## Notes

This issue was discovered during an investigation into perceived Gemini tool calling incompatibility. Initial hypothesis was that Gemini was stripping `__` prefix from tool names, but testing revealed:

1. **Both OpenAI and Gemini work perfectly with v1.1.0** using `__` prefix
2. **Both OpenAI and Gemini fail identically with v1.2.0** due to registry mismatch
3. The `__` prefix is NOT the issue - it's a GenKit framework change

The failed debugging attempts (tool name stripping) are documented to prevent future confusion about the real root cause.

---

**Last Updated**: 2025-11-24  
**Maintainer**: Station Development Team  
**Current GenKit Version**: v1.1.0 (pinned in go.mod)
