# Workflow Tracing Fix - End-to-End Observability

**Status**: COMPLETE  
**Created**: 2025-12-28  
**Last Updated**: 2025-12-28

## TL;DR

Fixed workflow run spans not appearing in Jaeger. Root cause: consumer uses `WorkflowServiceAdapter` which bypassed telemetry. Solution: wire telemetry to adapter, call `EndRunSpan()` on completion/failure.

## Problem Statement

Workflow-level spans are **MISSING** from Jaeger traces. Agent execution traces appear correctly, but there are no `workflow.run.*` or `workflow.step.*` spans to provide the parent context.

### Current State (Broken)
```
agent.execute_with_run_id           (orphaned - no parent)
  └── agent_execution_engine.execute
       └── mcp.load_tools
       └── dotprompt.execute
            └── generate            (LLM call)
```

### Desired State (Fixed)
```
workflow.run.batch-processor                    (root span)
  └── workflow.step.initialize                  (child)
  └── workflow.step.process_items               (child)
       └── agent.execute_with_run_id            (child)
            └── agent_execution_engine.execute  (child)
                 └── mcp.load_tools             (child)
                 └── dotprompt.execute          (child)
                      └── generate              (child - LLM call)
```

## Root Cause Analysis

The tracing infrastructure exists but is not being used correctly:

| Component | Location | Status |
|-----------|----------|--------|
| `StartRunSpan()` | `internal/workflows/runtime/telemetry.go:105` | **NEVER CALLED** |
| `StartStepSpan()` | `internal/workflows/runtime/telemetry.go:178` | Called in consumer.go:264 |
| `NATSTraceCarrier` | `internal/workflows/runtime/telemetry.go:232-305` | Exists |
| Workflow start | `internal/services/workflow_service.go:265` (`StartRun`) | **No span created** |

**Root Cause**: 
1. When a workflow starts in `WorkflowService.StartRun()`, no workflow run span is created
2. Step spans are created but have no parent workflow span, so they appear as disconnected traces
3. The trace context is propagated through NATS but there's no root span to connect everything

## Files Involved

```
# Workflow telemetry (has unused StartRunSpan)
internal/workflows/runtime/telemetry.go

# Workflow service (starts runs, needs to create run span)
internal/services/workflow_service.go

# Consumer (creates step spans, needs parent context)
internal/workflows/runtime/consumer.go

# NATS engine (publishes steps with trace context)
internal/workflows/runtime/nats_engine.go

# Jaeger handler (already fixed - uses in-memory storage)
cmd/main/handlers/jaeger.go
```

## Implementation Plan

### Task 1: Add telemetry field to WorkflowService struct
**File**: `internal/services/workflow_service.go`
**Status**: [x] DONE

Add telemetry field to struct:
```go
type WorkflowService struct {
    // ... existing fields ...
    telemetry *runtime.WorkflowTelemetry  // ADD THIS
}
```

Add setter method:
```go
func (s *WorkflowService) SetTelemetry(t *runtime.WorkflowTelemetry) {
    s.telemetry = t
}
```

### Task 2: Wire up telemetry in service initialization
**Status**: [x] DONE

Updated files:
- `cmd/main/workflow.go` - `startCLIWorkflowConsumer()` now accepts telemetry param
- `cmd/main/stdio.go` - `startStdioWorkflowConsumer()` now accepts telemetry param  
- `internal/api/v1/base.go` - All 3 constructors create and wire telemetry

Find where `NewWorkflowServiceWithEngine` or `NewWorkflowService` is called and pass/set the telemetry instance.

Likely locations:
- `cmd/main/` (main application setup)
- Dependency injection setup

### Task 3: Call StartRunSpan() in StartRun() method
**File**: `internal/services/workflow_service.go`
**Status**: [x] DONE

```go
func (s *WorkflowService) StartRun(ctx context.Context, req StartWorkflowRunRequest) (*models.WorkflowRun, workflows.ValidationResult, error) {
    // ... validation ...
    runID := uuid.NewString()
    
    // ADD THIS: Create workflow run span
    if s.telemetry != nil {
        ctx = s.telemetry.StartRunSpan(ctx, runID, parsed.Name)
    }
    
    // ... create run in DB ...
    
    if startStep != "" && s.engine != nil {
        step := plan.Steps[startStep]
        // This already propagates trace context via NATS
        if err := s.engine.PublishStepWithTrace(ctx, runID, startStep, step); err != nil {
            // ...
        }
    }
}
```

### Task 4: Call EndRunSpan() in completion methods
**File**: `internal/services/workflow_service.go`
**Status**: [x] DONE

Methods updated with `EndRunSpan()`:
- [x] `CompleteRun()` - ends span with status "completed", no error
- [x] `CancelRun()` - ends span with status "canceled", error message
- [x] `failAfterRejection()` - ends span with status "failed", rejection reason

### Task 5: Fix WorkflowServiceAdapter (CRITICAL FIX)
**File**: `internal/workflows/runtime/adapter.go`
**Status**: [x] DONE

**Problem**: The consumer uses `WorkflowServiceAdapter` (not `WorkflowService`) to complete/fail runs. This adapter was missing telemetry, so `EndRunSpan()` was never called even though `StartRunSpan()` was.

**Solution**:
1. Added `telemetry *WorkflowTelemetry` field to `WorkflowServiceAdapter`
2. Added `SetTelemetry()` method
3. Updated `CompleteRun()` to call `EndRunSpan(ctx, runID, workflowID, "completed", duration, nil)`
4. Updated `FailRun()` to call `EndRunSpan(ctx, runID, workflowID, "failed", duration, err)`

**Wiring** (all updated to call `adapter.SetTelemetry(telemetry)`):
- `internal/api/v1/base.go` - API handlers
- `cmd/main/workflow.go` - CLI workflow consumer
- `cmd/main/stdio.go` - stdio workflow consumer

### Task 6: Fix span name
**File**: `internal/services/workflow_service.go`
**Status**: [x] DONE

Changed `StartRunSpan(ctx, runID, parsed.Name)` to `StartRunSpan(ctx, runID, req.WorkflowID)` because `parsed.Name` was often empty, resulting in spans named `workflow.run.` instead of `workflow.run.batch-processor`.

```go
func (s *WorkflowService) CompleteRun(ctx context.Context, runID string, finalOutput map[string]any) error {
    // ... existing logic ...
    
    // ADD THIS: End workflow run span
    if s.telemetry != nil {
        s.telemetry.EndRunSpan(runID, nil) // nil = success
    }
    
    return nil
}

func (s *WorkflowService) CancelRun(ctx context.Context, runID, reason string) error {
    // ... existing logic ...
    
    // ADD THIS: End workflow run span with error
    if s.telemetry != nil {
        s.telemetry.EndRunSpan(runID, fmt.Errorf("cancelled: %s", reason))
    }
    
    return nil
}
```

### Task 7: Build and Test
**Status**: [x] DONE - Build passes, tests pass

```bash
cd /home/epuerta/sandbox/cloudship-sandbox/station

# Rebuild
make local-install-ui

# Run workflow with tracing
stn workflow run batch-processor --input '{"items": ["test1"]}' --wait

# Verify in Jaeger
curl -s "http://localhost:16686/api/traces?service=station&limit=5" | \
  jq '[.data[].spans[].operationName] | unique'

# Expected: Should now see workflow.run.*, workflow.step.* alongside agent spans
```

## Key Code References

### telemetry.go:105 - StartRunSpan (exists but unused)
```go
func (wt *WorkflowTelemetry) StartRunSpan(ctx context.Context, runID, workflowName string) context.Context {
    ctx, span := wt.tracer.Start(ctx, fmt.Sprintf("workflow.run.%s", workflowName),
        trace.WithSpanKind(trace.SpanKindInternal),
        trace.WithAttributes(
            attribute.String("workflow.run_id", runID),
            attribute.String("workflow.name", workflowName),
        ),
    )
    wt.mu.Lock()
    wt.runSpans[runID] = span
    wt.mu.Unlock()
    // ...
    return ctx
}
```

### telemetry.go - EndRunSpan
```go
func (wt *WorkflowTelemetry) EndRunSpan(runID string, err error) {
    wt.mu.Lock()
    span, exists := wt.runSpans[runID]
    if exists {
        delete(wt.runSpans, runID)
    }
    wt.mu.Unlock()
    
    if exists && span != nil {
        if err != nil {
            span.RecordError(err)
            span.SetStatus(codes.Error, err.Error())
        } else {
            span.SetStatus(codes.Ok, "completed")
        }
        span.End()
    }
}
```

## Completed Work (Prerequisites)

### Jaeger Docker Fix
**Problem**: Jaeger container was crash-looping with badger storage permission errors.

**Solution**: Changed from badger persistent storage to in-memory storage.

**File Modified**: `cmd/main/handlers/jaeger.go`
- Changed `SPAN_STORAGE_TYPE=badger` → `SPAN_STORAGE_TYPE=memory`
- Removed volume mounts that caused permission issues

**Status**: COMPLETED - Jaeger running at `http://localhost:16686`

## Testing Commands

```bash
# Start Jaeger (if not running)
stn serve  # Jaeger auto-launches

# Run a workflow
stn workflow run batch-processor --input '{"items": ["test1", "test2"]}' --wait

# Check Jaeger UI
open http://localhost:16686

# Query traces via API
curl -s "http://localhost:16686/api/traces?service=station&limit=5" | jq '.data[].spans[] | {operation: .operationName, traceID: .traceID}'
```

## Success Criteria

- [x] `workflow.run.*` spans appear in Jaeger
- [x] `workflow.step.*` spans are children of `workflow.run.*` spans
- [x] Agent execution spans are children of step spans
- [x] End-to-end trace visible from workflow start to completion
- [x] Error workflows show proper error status in spans

## Files Modified

| File | Changes |
|------|---------|
| `internal/workflows/runtime/adapter.go` | Added telemetry field, SetTelemetry(), EndRunSpan calls in CompleteRun/FailRun |
| `internal/services/workflow_service.go` | Added telemetry field, SetTelemetry/GetTelemetry, StartRunSpan call, fixed span name to use workflowID |
| `internal/api/v1/base.go` | Wire telemetry to adapter |
| `cmd/main/workflow.go` | Wire telemetry to adapter |
| `cmd/main/stdio.go` | Wire telemetry to adapter |

---

## CloudShip Telemetry Pattern (for org_id / station_id)

When CloudShip management calls are added, workflow spans will need proper org_id for multi-tenant filtering. The existing pattern from `TelemetryService` should be followed.

### How org_id Currently Flows

1. **SpanProcessor (Automatic)**: `cloudShipAttributeProcessor` in `telemetry_service.go:637` adds these attributes to ALL spans:
   - `cloudship.org_id`
   - `cloudship.station_id`  
   - `cloudship.station_name`

2. **Manual Addition**: `TelemetryService.StartSpan()` also adds them explicitly (lines 496-504):
   ```go
   if ts.config.OrgID != "" {
       span.SetAttributes(attribute.String("cloudship.org_id", ts.config.OrgID))
   }
   ```

3. **Authentication Flow**: `SetCloudShipInfo()` is called when station authenticates with CloudShip, updating the config that the processor reads.

### What This Means for Workflow Telemetry

Since `WorkflowTelemetry` uses `otel.Tracer()` (line 41), it gets the global tracer that has the CloudShip processor registered. **org_id is already automatically added to workflow spans** by the processor.

However, for future management calls where we need org_id for:
- NATS subject routing (`org.{org_id}.workflow.{run_id}`)
- Multi-tenant filtering in CloudShip platform
- Audit logging

We should also:
1. Pass org_id through context when available
2. Add it explicitly to workflow spans for clarity
3. Include it in NATS message payloads for routing

### Future: WorkflowTelemetry Enhancement

When adding management call support, enhance `WorkflowTelemetry` to accept org_id:

```go
// Enhanced StartRunSpan with CloudShip context
func (wt *WorkflowTelemetry) StartRunSpan(ctx context.Context, runID, workflowName string, opts ...WorkflowSpanOption) context.Context {
    // Apply options (org_id, station_id, etc.)
    cfg := &workflowSpanConfig{}
    for _, opt := range opts {
        opt(cfg)
    }
    
    attrs := []attribute.KeyValue{
        attribute.String("workflow.run_id", runID),
        attribute.String("workflow.name", workflowName),
    }
    
    // Add CloudShip attributes explicitly for clarity
    if cfg.orgID != "" {
        attrs = append(attrs, attribute.String("cloudship.org_id", cfg.orgID))
    }
    if cfg.stationID != "" {
        attrs = append(attrs, attribute.String("cloudship.station_id", cfg.stationID))
    }
    
    ctx, span := wt.tracer.Start(ctx, fmt.Sprintf("workflow.run.%s", workflowName),
        trace.WithSpanKind(trace.SpanKindInternal),
        trace.WithAttributes(attrs...),
    )
    // ...
}

// Option pattern
type WorkflowSpanOption func(*workflowSpanConfig)

type workflowSpanConfig struct {
    orgID     string
    stationID string
}

func WithOrgID(orgID string) WorkflowSpanOption {
    return func(c *workflowSpanConfig) { c.orgID = orgID }
}
```

### NATS Subject Pattern for Management

When CloudShip sends workflow commands:
```
org.{org_id}.station.{station_id}.workflow.command   # Commands TO station
org.{org_id}.session.{session_id}.workflow.response  # Responses FROM station
```

The org_id must be extracted from CloudShip auth context and propagated through:
1. NATS message headers
2. Trace context
3. Workflow run metadata
