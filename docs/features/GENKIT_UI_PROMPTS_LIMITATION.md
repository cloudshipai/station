# GenKit UI Prompts Display Limitation

## Issue Summary

When using `stn develop` with the GenKit Developer UI, **prompts do NOT appear in the "Prompts (0)" sidebar**, even though they are successfully loaded and functional.

## Root Cause

**GenKit Go SDK v1.0.3 Limitation**: The `WithPromptDir()` function loads `.prompt` files and makes them executable, but **does NOT register them as discoverable actions** in the reflection API that the GenKit UI queries.

### Evidence

1. **Prompts ARE loaded**: Logs show "Agent prompts automatically loaded from: ~/.config/station/environments/*/agents"
2. **Prompts DO execute**: Trace history shows successful `master_orchestrator` executions with "Executable-prompt" type
3. **Prompts NOT in reflection API**: Querying `http://127.0.0.1:3101/api/actions` returns 110 actions (embedders, models, utils) but **ZERO prompt actions**
4. **Multi-agent delegation works**: Agents successfully call sub-agents via `__agent_*` tools

## Confirmed Working Features

✅ **Prompt Execution**: Use trace history to see past executions  
✅ **Multi-Agent Hierarchy**: Agent tools work (`__agent_data_processor`, etc.)  
✅ **Tool Registration**: All 56 MCP + agent tools registered correctly  
✅ **CLI Execution**: `stn agent run` works perfectly with prompts

## Workarounds

### Option 1: Use Trace History (Recommended)
The GenKit UI **Trace History** section shows all prompt executions:

1. Navigate to `http://localhost:4000`
2. Look at the trace table at the bottom
3. Find rows with type "Executable-prompt" (e.g., `master_orchestrator`)
4. Click on a trace to see execution details

### Option 2: Use CLI Execution
```bash
# Execute agents via CLI instead of UI
stn agent run "master_orchestrator" "Calculate 15 + 20 and format as text" --env agent-hierarchy-demo --tail

# View execution details
stn runs list
stn runs inspect <run-id> -v
```

### Option 3: Test via Reflection API Directly
```bash
# Execute prompt via reflection API
curl -X POST http://127.0.0.1:3101/api/runAction \
  -H "Content-Type: application/json" \
  -d '{
    "key": "/prompt/master_orchestrator",
    "input": {"userInput": "Calculate 10 + 5"}
  }'
```

## Why This Isn't a Station Bug

This is a **GenKit Go SDK architectural limitation**:

1. The Go SDK's `WithPromptDir()` loads prompts internally
2. Prompts are stored in the registry but not exposed as actions
3. The `ai.Prompt` interface does not implement `api.Registerable`
4. Therefore, prompts cannot be registered via `genkit.RegisterAction()`

### Comparison: Node.js vs. Go SDK

- **Node.js SDK**: Prompts appear in UI sidebar ✅
- **Go SDK v1.0.3**: Prompts do NOT appear in UI sidebar ❌

This is a known difference between the SDKs, not a Station implementation issue.

## Testing Multi-Agent Workflows

Despite prompts not showing in the sidebar, **multi-agent hierarchies work perfectly**:

```bash
# Test orchestrator → specialist delegation
stn agent run "master_orchestrator" \
  "Calculate 25 * 4 and format the result as uppercase text" \
  --env agent-hierarchy-demo --tail
```

**Expected behavior:**
1. Master orchestrator analyzes the task
2. Delegates to `data_processor` (math + formatting)
3. Data processor calls `math_calculator` for computation
4. Data processor calls `text_formatter` for uppercase
5. Master returns final formatted result

**Verification:**
```bash
# Check execution trace
stn runs list --limit 1
stn runs inspect <latest-run-id> -v

# Look for tool calls to:
# - __agent_data_processor
# - __agent_math_calculator  
# - __agent_text_formatter
```

## Future Resolution

This limitation may be resolved in future GenKit Go SDK versions. Track these resources:

- **GenKit Go GitHub**: https://github.com/firebase/genkit/tree/main/go
- **GenKit Releases**: https://github.com/firebase/genkit/releases
- **Station Issue**: File enhancement request if needed

## Affected Versions

- **Station**: All versions using GenKit Go SDK
- **GenKit Go SDK**: v1.0.3 (current), likely affects all v1.x versions
- **GenKit UI**: All versions (limitation is in Go SDK, not UI)

## Recommendations

1. **Use trace history** for interactive testing
2. **Use CLI execution** for automated workflows
3. **Don't wait for UI sidebar** - prompts work regardless
4. **Focus on execution quality** - UI display is cosmetic

---

**Status**: ✅ Known Limitation (SDK-level, not Station bug)  
**Workaround**: ✅ Multiple viable alternatives available  
**Impact**: Low (cosmetic UI issue, functionality intact)  
**Date**: 2025-11-11
