# Workflow Engine Debug Session

**Date**: 2025-12-25  
**Focus**: Station Workflow Engine Runtime Bugs  
**Status**: In Progress

---

## Session Summary

We are debugging and verifying the **Station workflow engine** (Go-based) located in `/home/epuerta/sandbox/cloudship-sandbox/station/`. We've identified and fixed critical bugs that were preventing complex DevOps workflows from executing correctly.

---

## Bugs Fixed

### Bug #1: Parallel Executor - Agent Step Classification

**File**: `internal/workflows/runtime/parallel_executor.go`

**Problem**: Agent steps inside parallel branches were treated as no-ops because the executor's `classifyBranchState()` function didn't recognize `"agent"` as a valid branch state type.

**Fix**: Added `case "agent":` to `classifyBranchState()` function.

**Impact**: Parallel branches containing agent steps now execute correctly.

---

### Bug #2: Inject Executor - Data Output Format

**File**: `internal/workflows/runtime/inject_executor.go`

**Problem**: The `Output` was returning a wrapped object containing `injected_path` instead of the raw injected data. This caused workflows using `resultPath` (like `resultPath: incident`) to nest data unexpectedly (e.g., `incident.injected_data.severity`). Subsequent `switch` steps looking for `incident.severity` were failing to find the variable.

**Fix**: Modified the `Output` to return the raw `injectedData` map instead of a wrapped object.

**Technical Decision**: We decided to make `inject` return raw data to make variable access via `resultPath` predictable. If a workflow needs to know the path, that metadata should stay in the step metadata, not the output payload.

**Impact**: Data injection now follows the expected path structure.

---

## Verification Results

| Workflow | Status | Notes |
|----------|--------|-------|
| `incident-response-pipeline` | ✅ PASSED | Verified parallel agent execution and switch routing based on severity |
| `daily-infrastructure-health` | ✅ PASSED | Verified `foreach` loop with 5 agent iterations and parallel steps |
| `e2e-parallel-diagnostics` | ✅ PASSED | Run `827a982d-2f75-438c-96e3-4d8800327414` |
| `e2e-switch-routing` | ❌ FAILED | Run `79ea0da9-fbf9-4737-bb01-18c35865f966` - see Bug #3 below |
| `deployment-validation` | ⏳ PENDING | Not yet tested |

---

## Current Bug Under Investigation

### Bug #3: Switch Executor - Starlark Condition Evaluation

**Status**: 🔴 Under Investigation

**Error**: 
```
"condition evaluation failed: eval error: condition:1:1: undefined: val"
```

**Workflow**: `e2e-switch-routing.workflow.yaml`

**Context**:
- The workflow uses `dataPath: alert.severity` 
- Conditions use `_value == 'critical'`, `_value == 'high'`, etc.
- The switch executor sets `_value` and `result` variables before evaluation (line 65-66 in switch_executor.go)
- But Starlark returns `undefined: val` (not `undefined: _value`)

**Observations**:
1. The error mentions `val` (3 chars) not `_value` (6 chars)
2. This suggests either:
   - Some string processing is modifying the expression
   - A different code path is being used
   - There's a parsing issue in Starlark

**Run Context (from API)**:
```json
{
  "context": {
    "alert": {
      "severity": "high",
      "source": "prometheus",
      "alert_name": "HighErrorRate",
      ...
    }
  }
}
```

The data is correctly injected (`alert.severity = "high"`), but the switch condition evaluation is failing.

**Key Files**:
- `internal/workflows/runtime/switch_executor.go` - Switch step executor
- `internal/workflows/runtime/starlark_eval.go` - Starlark expression evaluator

**Next Steps**:
1. Add debug logging to see exact expression being evaluated
2. Verify Starlark globals dict contains `_value`
3. Check if expression is being modified before evaluation
4. Consider testing with simpler expression format

---

## Technical Architecture

### Workflow Runtime Flow

```
1. Workflow YAML loaded → parsed into states
2. NATS consumer receives workflow run request
3. Workflow engine creates run context
4. For each step:
   a. Executor selected based on step type
   b. Step executed with run context
   c. Output stored in context
   d. Next step determined from result
5. Run completes (success/failure)
```

### Key Executors

| Type | Executor | Purpose |
|------|----------|---------|
| `inject` | InjectExecutor | Inject static data into context |
| `switch` | SwitchExecutor | Conditional branching based on data |
| `parallel` | ParallelExecutor | Run multiple branches concurrently |
| `agent` | AgentExecutor | Execute AI agent tasks |
| `foreach` | ForeachExecutor | Iterate over collections |
| `operation` | OperationExecutor | Generic operations |
| `timer` | TimerExecutor | Wait for duration |
| `trycatch` | TryCatchExecutor | Error handling |

### Expression Evaluation

The workflow engine uses **Starlark** for expression evaluation:

```go
// In starlark_eval.go
func (e *StarlarkEvaluator) EvaluateCondition(expression string, data map[string]interface{}) (bool, error) {
    globals := e.convertToStarlark(data)  // Converts Go map to Starlark globals
    expr, err := syntax.ParseExpr("condition", expression, 0)
    result, err := starlark.EvalExprOptions(&fileOpts, thread, expr, globals)
    // ...
}
```

For switch conditions with `dataPath`:
1. Extract value at dataPath from context
2. Set `_value` and `result` in globals
3. Evaluate each condition expression
4. First matching condition determines next step

---

## Files Modified This Session

| File | Change |
|------|--------|
| `internal/workflows/runtime/parallel_executor.go` | Added `case "agent":` to classifyBranchState() |
| `internal/workflows/runtime/inject_executor.go` | Changed Output to return raw data instead of wrapped object |

---

## Test Commands

```bash
# Check server status
lsof -i :8585

# List workflow runs
curl -s "http://localhost:8585/api/v1/workflow-runs?limit=5" | jq '.runs[] | {run_id, workflow_id, status}'

# Get specific run details
curl -s "http://localhost:8585/api/v1/workflow-runs/<run_id>" | jq '.'

# Trigger a workflow
curl -X POST "http://localhost:8585/api/v1/workflows/<workflow_id>/runs" \
  -H "Content-Type: application/json" \
  -d '{"input": {}}'

# Rebuild station
cd /home/epuerta/sandbox/cloudship-sandbox/station && go build -o bin/stn ./cmd/stn

# Restart server (kill and restart)
pkill -f "stn serve" && ./bin/stn serve --config config.yaml &
```

---

## Continuation Prompt

When resuming this session:

1. **Check Port 8585**: Ensure the server is running (`lsof -i :8585`)
2. **Current Bug**: Switch condition evaluation failing with `undefined: val`
3. **Hypothesis**: The Starlark expression `_value == 'critical'` is being evaluated but `_value` is not in globals
4. **Debug Approach**: 
   - Add logging to switch_executor.go to print evalData keys
   - Check if `raw.DataPath` is being correctly parsed
   - Verify the Starlark evaluator receives the correct globals

---

**Last Updated**: 2025-12-25 14:20 CST
