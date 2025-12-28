# PRD: Lighthouse Workflow Management

**Status**: Draft  
**Author**: AI Assistant  
**Created**: 2025-12-27  
**Priority**: High  

---

## Overview

This PRD defines the protobuf message definitions and implementation plan for managing Station workflows remotely through CloudShip's Lighthouse service. This enables CloudShip UI and API to create, monitor, and control workflow executions on connected Stations.

## Problem Statement

Currently, workflows can only be managed locally on the Station via:
- CLI commands (`stn workflow create`, `stn workflow run`, etc.)
- MCP tools (`station-team_create_workflow`, `station-team_start_workflow_run`, etc.)
- Local API endpoints

There is no way to manage workflows remotely from CloudShip, which limits:
1. **Central observability**: No visibility into workflow runs across all connected Stations
2. **Remote triggering**: Cannot start workflows from CloudShip UI
3. **Cross-station orchestration**: Cannot coordinate workflows across multiple Stations
4. **Approval workflows**: No centralized approval queue for human-in-the-loop workflows

## Goals

1. **Remote CRUD**: Create, read, update, and delete workflow definitions from CloudShip
2. **Remote execution**: Start, pause, resume, and cancel workflow runs from CloudShip
3. **Centralized observability**: Stream workflow run status and step progress to CloudShip
4. **Human-in-the-loop**: Manage workflow approvals from CloudShip UI
5. **Multi-station**: Support routing to specific stations in multi-station deployments

## Non-Goals

- Workflow definition storage in CloudShip (definitions live on Station)
- Cross-station workflow orchestration (Phase 2)
- Workflow templates/marketplace (future feature)

---

## Technical Design

### Protobuf Message Definitions

Add to `internal/lighthouse/proto/lighthouse.proto`:

#### Enums

```protobuf
// Workflow definition status
enum WorkflowStatus {
    WORKFLOW_STATUS_UNSPECIFIED = 0;
    WORKFLOW_STATUS_ACTIVE = 1;
    WORKFLOW_STATUS_DISABLED = 2;
}

// Workflow run status
enum WorkflowRunStatus {
    WORKFLOW_RUN_STATUS_UNSPECIFIED = 0;
    WORKFLOW_RUN_STATUS_PENDING = 1;
    WORKFLOW_RUN_STATUS_RUNNING = 2;
    WORKFLOW_RUN_STATUS_BLOCKED = 3;      // Waiting for approval or signal
    WORKFLOW_RUN_STATUS_COMPLETED = 4;
    WORKFLOW_RUN_STATUS_FAILED = 5;
    WORKFLOW_RUN_STATUS_CANCELED = 6;
}

// Workflow approval status
enum WorkflowApprovalStatus {
    WORKFLOW_APPROVAL_STATUS_UNSPECIFIED = 0;
    WORKFLOW_APPROVAL_STATUS_PENDING = 1;
    WORKFLOW_APPROVAL_STATUS_APPROVED = 2;
    WORKFLOW_APPROVAL_STATUS_REJECTED = 3;
    WORKFLOW_APPROVAL_STATUS_EXPIRED = 4;
}
```

#### Workflow Definition Messages

```protobuf
// Workflow definition info (lightweight, for listing)
message WorkflowInfo {
    string workflow_id = 1;           // Unique workflow identifier
    string name = 2;                  // Display name
    string description = 3;           // Description
    int64 version = 4;                // Current version number
    WorkflowStatus status = 5;        // active/disabled
    int32 state_count = 6;            // Number of states in workflow
    repeated string agents = 7;       // Agent names used in workflow
    string cron_schedule = 8;         // Cron expression if scheduled
    google.protobuf.Timestamp created_at = 9;
    google.protobuf.Timestamp updated_at = 10;
}

// Full workflow definition
message WorkflowDefinition {
    string workflow_id = 1;
    string name = 2;
    string description = 3;
    int64 version = 4;
    WorkflowStatus status = 5;
    string definition_json = 6;       // Full JSON definition
    google.protobuf.Timestamp created_at = 7;
    google.protobuf.Timestamp updated_at = 8;
}

// Validation issue
message WorkflowValidationIssue {
    string code = 1;                  // Error code (e.g., "MISSING_STATE")
    string path = 2;                  // JSONPath to issue
    string message = 3;               // Human-readable message
    string hint = 4;                  // Suggested fix
}

// Validation result
message WorkflowValidationResult {
    bool valid = 1;
    repeated WorkflowValidationIssue errors = 2;
    repeated WorkflowValidationIssue warnings = 3;
}
```

#### Workflow Run Messages

```protobuf
// Workflow run info
message WorkflowRunInfo {
    string run_id = 1;                // UUID run identifier
    string workflow_id = 2;           // Parent workflow ID
    int64 workflow_version = 3;       // Version executed
    WorkflowRunStatus status = 4;
    string current_step = 5;          // Current step ID
    string input_json = 6;            // Input JSON
    string result_json = 7;           // Result JSON (if completed)
    string error = 8;                 // Error message (if failed)
    google.protobuf.Timestamp started_at = 9;
    google.protobuf.Timestamp completed_at = 10;
}

// Workflow run step
message WorkflowRunStep {
    string step_id = 1;
    int64 attempt = 2;
    string status = 3;                // running, completed, failed
    string input_json = 4;
    string output_json = 5;
    string error = 6;
    google.protobuf.Timestamp started_at = 7;
    google.protobuf.Timestamp completed_at = 8;
}

// Workflow approval
message WorkflowApproval {
    string approval_id = 1;           // UUID
    string run_id = 2;                // Parent run
    string step_id = 3;               // Step requiring approval
    WorkflowApprovalStatus status = 4;
    string title = 5;                 // Approval title
    string description = 6;           // What's being approved
    string context_json = 7;          // Context for approver
    string approver_id = 8;           // Who approved/rejected
    string comment = 9;               // Approver comment
    google.protobuf.Timestamp created_at = 10;
    google.protobuf.Timestamp expires_at = 11;
    google.protobuf.Timestamp decided_at = 12;
}
```

#### Request/Response Messages

```protobuf
// ============================================================================
// WORKFLOW DEFINITION MANAGEMENT
// ============================================================================

// List workflows
message ListWorkflowsManagementRequest {
    string environment = 1;           // Optional environment filter
    WorkflowStatus status = 2;        // Optional status filter
}

message ListWorkflowsManagementResponse {
    repeated WorkflowInfo workflows = 1;
    int32 total_count = 2;
}

// Get workflow
message GetWorkflowManagementRequest {
    string workflow_id = 1;
    int64 version = 2;                // 0 = latest
    string environment = 3;
}

message GetWorkflowManagementResponse {
    WorkflowDefinition workflow = 1;
}

// Create workflow
message CreateWorkflowManagementRequest {
    string workflow_id = 1;
    string name = 2;
    string description = 3;
    string definition_json = 4;       // Full JSON definition
    string environment = 5;
}

message CreateWorkflowManagementResponse {
    bool success = 1;
    WorkflowDefinition workflow = 2;
    WorkflowValidationResult validation = 3;
    string error_message = 4;
}

// Update workflow
message UpdateWorkflowManagementRequest {
    string workflow_id = 1;
    string name = 2;
    string description = 3;
    string definition_json = 4;
    string environment = 5;
}

message UpdateWorkflowManagementResponse {
    bool success = 1;
    WorkflowDefinition workflow = 2;
    WorkflowValidationResult validation = 3;
    string error_message = 4;
}

// Validate workflow (without saving)
message ValidateWorkflowManagementRequest {
    string definition_json = 1;
    string environment = 2;
}

message ValidateWorkflowManagementResponse {
    WorkflowValidationResult validation = 1;
}

// Disable workflow
message DisableWorkflowManagementRequest {
    string workflow_id = 1;
    string environment = 2;
}

message DisableWorkflowManagementResponse {
    bool success = 1;
    string message = 2;
}

// ============================================================================
// WORKFLOW RUN MANAGEMENT
// ============================================================================

// Start workflow run
message StartWorkflowRunManagementRequest {
    string workflow_id = 1;
    int64 version = 2;                // 0 = latest
    string input_json = 3;            // Input data
    string environment = 4;
}

message StartWorkflowRunManagementResponse {
    bool success = 1;
    WorkflowRunInfo run = 2;
    WorkflowValidationResult validation = 3;
    string error_message = 4;
}

// Get workflow run
message GetWorkflowRunManagementRequest {
    string run_id = 1;
    string environment = 2;
}

message GetWorkflowRunManagementResponse {
    WorkflowRunInfo run = 1;
}

// List workflow runs
message ListWorkflowRunsManagementRequest {
    string workflow_id = 1;           // Optional filter
    WorkflowRunStatus status = 2;     // Optional filter
    int64 limit = 3;                  // Max results (default 50)
    string environment = 4;
}

message ListWorkflowRunsManagementResponse {
    repeated WorkflowRunInfo runs = 1;
    int32 total_count = 2;
}

// Get workflow run steps
message GetWorkflowRunStepsManagementRequest {
    string run_id = 1;
    string environment = 2;
}

message GetWorkflowRunStepsManagementResponse {
    repeated WorkflowRunStep steps = 1;
}

// Cancel workflow run
message CancelWorkflowRunManagementRequest {
    string run_id = 1;
    string reason = 2;
    string environment = 3;
}

message CancelWorkflowRunManagementResponse {
    bool success = 1;
    WorkflowRunInfo run = 2;
    string message = 3;
}

// Pause workflow run
message PauseWorkflowRunManagementRequest {
    string run_id = 1;
    string reason = 2;
    string environment = 3;
}

message PauseWorkflowRunManagementResponse {
    bool success = 1;
    WorkflowRunInfo run = 2;
    string message = 3;
}

// Resume workflow run
message ResumeWorkflowRunManagementRequest {
    string run_id = 1;
    string note = 2;
    string environment = 3;
}

message ResumeWorkflowRunManagementResponse {
    bool success = 1;
    WorkflowRunInfo run = 2;
    string message = 3;
}

// ============================================================================
// WORKFLOW APPROVAL MANAGEMENT
// ============================================================================

// List pending approvals
message ListWorkflowApprovalsManagementRequest {
    string run_id = 1;                // Optional filter by run
    int64 limit = 2;                  // Max results (default 50)
    string environment = 3;
}

message ListWorkflowApprovalsManagementResponse {
    repeated WorkflowApproval approvals = 1;
    int32 total_count = 2;
}

// Approve workflow step
message ApproveWorkflowStepManagementRequest {
    string approval_id = 1;
    string approver_id = 2;
    string comment = 3;
    string environment = 4;
}

message ApproveWorkflowStepManagementResponse {
    bool success = 1;
    WorkflowApproval approval = 2;
    string message = 3;
}

// Reject workflow step
message RejectWorkflowStepManagementRequest {
    string approval_id = 1;
    string rejecter_id = 2;
    string reason = 3;
    string environment = 4;
}

message RejectWorkflowStepManagementResponse {
    bool success = 1;
    WorkflowApproval approval = 2;
    string message = 3;
}
```

#### ManagementMessage Updates

Add to the `ManagementMessage.message` oneof:

```protobuf
message ManagementMessage {
    // ... existing fields ...
    
    oneof message {
        // ... existing messages ...
        
        // Workflow Definition Management (40-49)
        ListWorkflowsManagementRequest list_workflows_request = 40;
        ListWorkflowsManagementResponse list_workflows_response = 41;
        GetWorkflowManagementRequest get_workflow_request = 42;
        GetWorkflowManagementResponse get_workflow_response = 43;
        CreateWorkflowManagementRequest create_workflow_request = 44;
        CreateWorkflowManagementResponse create_workflow_response = 45;
        UpdateWorkflowManagementRequest update_workflow_request = 46;
        UpdateWorkflowManagementResponse update_workflow_response = 47;
        ValidateWorkflowManagementRequest validate_workflow_request = 48;
        ValidateWorkflowManagementResponse validate_workflow_response = 49;
        DisableWorkflowManagementRequest disable_workflow_request = 50;
        DisableWorkflowManagementResponse disable_workflow_response = 51;
        
        // Workflow Run Management (52-65)
        StartWorkflowRunManagementRequest start_workflow_run_request = 52;
        StartWorkflowRunManagementResponse start_workflow_run_response = 53;
        GetWorkflowRunManagementRequest get_workflow_run_request = 54;
        GetWorkflowRunManagementResponse get_workflow_run_response = 55;
        ListWorkflowRunsManagementRequest list_workflow_runs_request = 56;
        ListWorkflowRunsManagementResponse list_workflow_runs_response = 57;
        GetWorkflowRunStepsManagementRequest get_workflow_run_steps_request = 58;
        GetWorkflowRunStepsManagementResponse get_workflow_run_steps_response = 59;
        CancelWorkflowRunManagementRequest cancel_workflow_run_request = 60;
        CancelWorkflowRunManagementResponse cancel_workflow_run_response = 61;
        PauseWorkflowRunManagementRequest pause_workflow_run_request = 62;
        PauseWorkflowRunManagementResponse pause_workflow_run_response = 63;
        ResumeWorkflowRunManagementRequest resume_workflow_run_request = 64;
        ResumeWorkflowRunManagementResponse resume_workflow_run_response = 65;
        
        // Workflow Approval Management (66-71)
        ListWorkflowApprovalsManagementRequest list_workflow_approvals_request = 66;
        ListWorkflowApprovalsManagementResponse list_workflow_approvals_response = 67;
        ApproveWorkflowStepManagementRequest approve_workflow_step_request = 68;
        ApproveWorkflowStepManagementResponse approve_workflow_step_response = 69;
        RejectWorkflowStepManagementRequest reject_workflow_step_request = 70;
        RejectWorkflowStepManagementResponse reject_workflow_step_response = 71;
    }
}
```

#### ErrorCode Updates

Add to the `ErrorCode` enum:

```protobuf
enum ErrorCode {
    // ... existing codes ...
    WORKFLOW_NOT_FOUND = 10;
    WORKFLOW_RUN_NOT_FOUND = 11;
    WORKFLOW_APPROVAL_NOT_FOUND = 12;
    WORKFLOW_VALIDATION_ERROR = 13;
    WORKFLOW_ALREADY_EXISTS = 14;
    WORKFLOW_RUN_NOT_CANCELLABLE = 15;
}
```

---

## Implementation Plan

### Phase 1: Protobuf & Station Handlers (Week 1)

#### Work Items

| ID | Task | File | Effort |
|----|------|------|--------|
| 1.1 | Add workflow enums to lighthouse.proto | `internal/lighthouse/proto/lighthouse.proto` | S |
| 1.2 | Add workflow message definitions | `internal/lighthouse/proto/lighthouse.proto` | M |
| 1.3 | Add workflow request/response messages | `internal/lighthouse/proto/lighthouse.proto` | L |
| 1.4 | Update ManagementMessage oneof | `internal/lighthouse/proto/lighthouse.proto` | S |
| 1.5 | Regenerate Go protobuf code | `make proto` | S |
| 1.6 | Implement workflow message handlers in Station | `internal/lighthouse/management_handlers.go` | L |
| 1.7 | Add tests for workflow handlers | `internal/lighthouse/management_handlers_test.go` | M |

### Phase 2: Lighthouse Routing (Week 2)

| ID | Task | File | Effort |
|----|------|------|--------|
| 2.1 | Add workflow routing in Lighthouse connection manager | `cloudshipai/lighthouse/internal/management/` | M |
| 2.2 | Implement workflow command dispatch | `cloudshipai/lighthouse/internal/service/` | M |
| 2.3 | Add tests for workflow routing | `cloudshipai/lighthouse/internal/management/*_test.go` | M |

### Phase 3: Django Integration (Week 3)

| ID | Task | File | Effort |
|----|------|------|--------|
| 3.1 | Update Python protobuf stubs | `cloudshipai/backend/apps/station/proto/` | S |
| 3.2 | Add workflow methods to lighthouse_client.py | `cloudshipai/backend/apps/station/services/lighthouse_client.py` | M |
| 3.3 | Create workflow management API endpoints | `cloudshipai/backend/apps/station/api/workflow_views.py` | M |
| 3.4 | Add tests | `cloudshipai/backend/apps/station/tests/` | M |

### Phase 4: UI Integration (Week 4)

| ID | Task | File | Effort |
|----|------|------|--------|
| 4.1 | Create workflow list page | `cloudshipai/backend/static_src/js/Pages/Workflows/` | M |
| 4.2 | Create workflow detail/run page | `cloudshipai/backend/static_src/js/Pages/Workflows/` | L |
| 4.3 | Create approval queue UI | `cloudshipai/backend/static_src/js/Pages/Workflows/` | M |

---

## Testing Strategy

### Unit Tests
- Protobuf serialization/deserialization
- Handler logic for each request type
- Validation logic

### Integration Tests
- Station ↔ Lighthouse bidirectional communication
- Workflow CRUD operations via management channel
- Run lifecycle (start → steps → complete/fail)
- Approval flow (create → approve/reject → resume/fail)

### E2E Tests
- CloudShip UI → Lighthouse → Station → execution → response
- Multi-station routing
- Approval timeout handling

---

## Success Metrics

| Metric | Target |
|--------|--------|
| Remote workflow creation success rate | >99% |
| Workflow run start latency | <500ms |
| Step progress visibility | Real-time (<1s delay) |
| Approval response latency | <200ms |

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Large workflow definitions exceed message size | High | Compress definition_json, add chunking for >1MB |
| Station disconnects mid-run | Medium | Run continues locally, CloudShip shows "offline" status |
| Approval timeout not enforced | Medium | Station-side scheduler handles expiration |
| Version conflicts on concurrent updates | Low | Optimistic locking with version field |

---

## Design Considerations

### Run ID Correlation (Nuanced - Phase 2+)

There's a nuanced coordination problem when CloudShip triggers workflow runs remotely:

#### The Problem

1. **CloudShip sets the workflow run_id**: When `StartWorkflowRunManagementRequest` is sent, CloudShip generates and sets the `run_id` so it can track the workflow run in its database.

2. **Agents execute via management channel**: When workflow steps execute agents, those agent executions ALSO go through the management channel (using `ExecuteAgentManagementRequest`).

3. **Correlation needed**: The agent runs need to be correlated back to the parent workflow run for:
   - Unified observability (see all agent runs under a workflow run)
   - Token/cost aggregation
   - Tracing and debugging

#### Current Agent Execution Pattern

```protobuf
message ExecuteAgentManagementRequest {
    string agent_id = 1;
    string task = 2;
    string environment = 3;
    map<string, string> variables = 4;
    int32 timeout_seconds = 5;
    string run_id = 6;  // Lighthouse sets this for tracking
}
```

#### Proposed Solution (Phase 2)

Add parent workflow correlation to agent execution:

```protobuf
message ExecuteAgentManagementRequest {
    // ... existing fields ...
    string run_id = 6;                    // Agent run ID (set by Lighthouse)
    string parent_workflow_run_id = 7;    // NEW: Parent workflow run (if agent is part of workflow)
    string parent_step_id = 8;            // NEW: Which workflow step triggered this agent
}
```

#### Data Flow

```
CloudShip                    Lighthouse                   Station
    |                            |                            |
    |--StartWorkflowRun--------->|                            |
    |  run_id: "wf-run-123"      |                            |
    |                            |--StartWorkflowRun--------->|
    |                            |  run_id: "wf-run-123"      |
    |                            |                            |
    |                            |                     [Workflow starts]
    |                            |                     [Step 1: agent]
    |                            |                            |
    |                            |<--ExecuteAgent (internal)--|
    |                            |  parent_workflow_run_id:   |
    |                            |    "wf-run-123"            |
    |                            |  parent_step_id: "step1"   |
    |                            |                            |
    |<--AgentRun Event-----------|                            |
    |  workflow_run_id: "wf-123" |                            |
    |  agent_run_id: "ar-456"    |                            |
```

#### Implementation Notes

- **Local execution**: When workflow runs locally (not via management channel), agent runs are correlated via Station's internal tracking.
- **Remote execution**: When CloudShip triggers the workflow, the management channel must preserve the correlation chain.
- **Deferred**: This correlation logic is complex and deferred to Phase 2. Phase 1 focuses on basic CRUD operations.

---

## Future Considerations

1. **Workflow streaming**: Real-time step progress via gRPC streaming
2. **Cross-station workflows**: Orchestrate workflows that span multiple Stations
3. **Workflow templates**: CloudShip-managed workflow templates
4. **Audit logging**: Track all workflow management operations
5. **Run ID correlation**: Full parent-child tracking for workflow → agent runs (see Design Considerations above)

---

## References

- [Existing lighthouse.proto](../internal/lighthouse/proto/lighthouse.proto) - Current protobuf definitions
- [WorkflowService](../internal/services/workflow_service.go) - Station workflow service
- [PRD_WORKFLOW_DX_IMPROVEMENTS.md](./PRD_WORKFLOW_DX_IMPROVEMENTS.md) - Previous workflow improvements
