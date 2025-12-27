# Human-in-the-Loop Workflows

This guide covers how to implement human approval gates in Station workflows. Human-in-the-loop (HITL) patterns allow workflows to pause execution until a human operator approves or rejects the next action.

## Table of Contents

1. [Overview](#overview)
2. [Defining Approval Steps](#defining-approval-steps)
3. [CLI Commands](#cli-commands)
4. [API Endpoints](#api-endpoints)
5. [Complete Example](#complete-example)
6. [Testing Walkthrough](#testing-walkthrough)
7. [Troubleshooting](#troubleshooting)

---

## Overview

Human approval gates are essential for:
- **Production deployments** - Require human sign-off before deploying to prod
- **Security actions** - Block IPs, revoke access tokens, etc.
- **High-risk operations** - Database migrations, infrastructure changes
- **Compliance workflows** - Audit trails requiring human acknowledgment

### How It Works

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Prior Step    │────▶│  Approval Step  │────▶│   Next Step     │
│   (completes)   │     │  (WAITING)      │     │  (after approve)│
└─────────────────┘     └────────┬────────┘     └─────────────────┘
                                 │
                                 │ Workflow PAUSES here
                                 │
                        ┌────────▼────────┐
                        │  Human Action   │
                        │                 │
                        │ stn workflow    │
                        │ approvals       │
                        │ approve <id>    │
                        └─────────────────┘
```

When a workflow reaches an approval step:
1. Workflow status changes to `waiting_approval`
2. An approval record is created in the database
3. Execution pauses until human action
4. User runs `stn workflow approvals approve <id>` or `reject <id>`
5. Workflow resumes (or terminates on rejection)

---

## Defining Approval Steps

### IMPORTANT: Correct Syntax

There are two ways to define approval steps, but **only one works correctly**:

#### Method 1: Operation with `task: "human.approval"` (CORRECT)

```yaml
- id: request_approval
  type: operation
  input:
    task: "human.approval"           # REQUIRED - triggers approval executor
    message: "Please approve this action"  # REQUIRED - shown to approver
    approvers: ["admin", "oncall"]   # Optional - list of allowed approvers
    timeout_seconds: 3600            # Optional - auto-reject after timeout
  resultPath: approval_result
  transition: next_step
```

This is the **correct** approach because:
- `type: operation` invokes the operation executor
- `input.task: "human.approval"` is checked by `HumanApprovalExecutor`
- `input.message` provides the approval prompt

#### Method 2: `type: human_approval` (DOES NOT WORK AS EXPECTED)

```yaml
# WARNING: This syntax DOES NOT trigger proper approval behavior!
- id: request_approval
  type: human_approval
  prompt: "Please approve this action"  # This field is ignored!
  transition: next_step
```

**Why it doesn't work:**
- `type: human_approval` maps to `StepTypeAwait` in the translator
- But the `HumanApprovalExecutor` checks for `input["task"] == "human.approval"`
- Without that input, the executor returns `{skipped: true}` and the step auto-completes

### Input Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `task` | Yes | Must be `"human.approval"` |
| `message` | Yes | The approval prompt shown to the user |
| `approvers` | No | List of usernames/roles who can approve |
| `timeout_seconds` | No | Auto-reject after this many seconds |

### Output Result

After approval/rejection, the step result contains:

```json
{
  "approved": true,           // or false
  "approver": "admin",        // who approved/rejected
  "comment": "Looks good",    // optional comment
  "decided_at": "2025-12-27T00:00:00Z"
}
```

Access in subsequent steps:
```yaml
- id: check_result
  type: switch
  conditions:
    - if: "hasattr(approval_result, 'approved') and approval_result.approved == True"
      next: proceed
  defaultNext: handle_rejection
```

---

## CLI Commands

### List Pending Approvals

```bash
stn workflow approvals list
```

Output:
```
APPROVAL_ID                     WORKFLOW         STEP              STATUS    CREATED
appr-run_abc123-request_approval  approval-demo   request_approval  pending   2025-12-27 12:00:00
```

### Approve a Step

```bash
stn workflow approvals approve <approval-id>

# With comment
stn workflow approvals approve appr-run_abc123-request_approval --comment "Approved for production"
```

### Reject a Step

```bash
stn workflow approvals reject <approval-id>

# With reason
stn workflow approvals reject appr-run_abc123-request_approval --reason "Requires additional review"
```

---

## API Endpoints

### List Pending Approvals

```bash
GET /api/v1/workflow-approvals
```

Response:
```json
{
  "approvals": [
    {
      "id": "appr-run_abc123-request_approval",
      "run_id": "run_abc123",
      "step_id": "request_approval",
      "workflow_id": "approval-demo",
      "message": "Please approve this action",
      "status": "pending",
      "created_at": "2025-12-27T12:00:00Z"
    }
  ]
}
```

### Approve

```bash
POST /api/v1/workflow-approvals/{id}/approve
Content-Type: application/json

{
  "comment": "Approved for production"
}
```

### Reject

```bash
POST /api/v1/workflow-approvals/{id}/reject
Content-Type: application/json

{
  "reason": "Requires additional review"
}
```

---

## Complete Example

### File Location

```
~/.config/station/environments/default/workflows/approval-demo.workflow.yaml
```

### Workflow Definition

```yaml
# Human Approval Demo Workflow
id: approval-demo
version: "1.0"
name: "Human Approval Demo"
description: "A simple workflow to demonstrate human-in-the-loop approval"
start: greet

states:
  # Step 1: Inject initial data
  - id: greet
    type: inject
    data:
      greeting: "Hello! This workflow will ask for your approval."
    resultPath: intro
    transition: ask_approval

  # Step 2: Human approval gate - workflow PAUSES here
  - id: ask_approval
    type: operation
    input:
      task: "human.approval"
      message: "Please approve this demo workflow to continue. Type 'stn workflow approvals approve <approval-id>' to approve."
      approvers: ["admin"]
      timeout_seconds: 3600
    resultPath: approval
    transition: check_result

  # Step 3: Route based on approval decision
  - id: check_result
    type: switch
    conditions:
      - if: "hasattr(approval, 'approved') and approval.approved == True"
        next: approved
    defaultNext: rejected

  # Step 4a: Approved path
  - id: approved
    type: inject
    data:
      status: "approved"
      message: "You approved the workflow!"
    resultPath: result
    end: true

  # Step 4b: Rejected path
  - id: rejected
    type: inject
    data:
      status: "rejected"
      message: "Workflow was rejected or timed out"
    resultPath: result
    end: true
```

---

## Testing Walkthrough

This walkthrough demonstrates an actual test session from December 2025.

### Step 1: Sync the Workflow

```bash
stn sync default
```

Actual output:
```
Starting sync for environment: default
Starting declarative sync for environment: default
📋 Found 4 agent .prompt files
Synced 1 workflows for environment default
🧹 Cleanup completed: No orphaned resources found
Completed sync for environment default: 4 agents processed, 0 errors

Sync completed for environment: default
  Agents: 4 processed, 0 synced
  MCP Servers: 0 processed, 0 connected
  ✅ All configurations synced successfully
```

### Step 2: Run the Workflow

```bash
stn workflow run approval-demo
```

Actual output:
```
🚀 Starting workflow: approval-demo

⚠️  Warnings:
   • /states/0/input: Input mapping is recommended for every step
   • /states/0/output: Export/output mapping is recommended to persist step results
   ... (validation warnings - these are advisory)

✅ Workflow run started!
   Run ID: c7292c89-71b3-470b-a7f0-5e68daabd1c6
   Status: pending
   Current Step: greet
   Started: 2025-12-27T00:28:56-06:00

💡 Use 'stn workflow inspect c7292c89-71b3-470b-a7f0-5e68daabd1c6' to check progress
```

### Step 3: Check Workflow Status (Paused at Approval)

The workflow consumer (running via `stn serve`) processes steps automatically. After a few seconds:

```bash
stn workflow inspect c7292c89-71b3-470b-a7f0-5e68daabd1c6
```

Actual output:
```
⏸️ Workflow Run: c7292c89-71b3-470b-a7f0-5e68daabd1c6
   Workflow: approval-demo (v1)
   Status: waiting_approval
   Current Step: ask_approval
   Started: 2025-12-27T00:28:56-06:00

Steps (2):
  ✅ greet
     Status: completed
     Started: 2025-12-27T00:29:21-06:00
     Duration: 5.869705ms
  ⏸️ ask_approval
     Status: waiting_approval
     Started: 2025-12-27T00:29:21-06:00
```

**Key observation**: The workflow has paused at `ask_approval` with status `waiting_approval`.

### Step 4: List Pending Approvals

```bash
stn workflow approvals list
```

Actual output:
```
Pending Approvals (1):

⏸️  appr-c7292c89-71b3-470b-a7f0-5e68daabd1c6-ask_approval
   Run: c7292c89-71b3-470b-a7f0-5e68daabd1c6
   Step: ask_approval
   Message: Please approve this demo workflow to continue. Type 'stn workflow approvals approve <approval-id>' to approve.
   Approvers: admin
   Timeout: 2025-12-27T01:29:21-06:00 (in 59m45s)
   Created: 2025-12-27T06:29:21Z

💡 Use 'stn workflow approvals approve <id>' or 'stn workflow approvals reject <id>' to decide
```

### Step 5: Approve the Workflow

```bash
stn workflow approvals approve appr-c7292c89-71b3-470b-a7f0-5e68daabd1c6-ask_approval --comment "Approved via CLI test"
```

Actual output:
```
✅ Approved: appr-c7292c89-71b3-470b-a7f0-5e68daabd1c6-ask_approval
   Run: c7292c89-71b3-470b-a7f0-5e68daabd1c6
   Step: ask_approval
   Status: approved

🚀 Workflow will resume automatically
```

### Step 6: Verify Completion

After approval, the workflow consumer automatically resumes execution:

```bash
stn workflow inspect c7292c89-71b3-470b-a7f0-5e68daabd1c6
```

Expected final output:
```
✅ Workflow Run: c7292c89-71b3-470b-a7f0-5e68daabd1c6
   Workflow: approval-demo (v1)
   Status: completed
   Current Step: approved
   Started: 2025-12-27T00:28:56-06:00

Steps (4):
  ✅ greet - completed
  ✅ ask_approval - completed
  ✅ check_result - completed
  ✅ approved - completed
```

### Important: Running the Workflow Consumer

For workflows to process steps (including approvals), the Station server must be running **continuously**:

```bash
# In one terminal - start the server (keep this running!)
stn serve

# In another terminal - run workflow commands
stn workflow run approval-demo
stn workflow approvals list
stn workflow approvals approve <id>
```

**Critical Notes:**
1. The server must be running BEFORE you start the workflow
2. The server must stay running throughout the workflow lifecycle
3. If the server stops, workflows will pause and need the server to restart to resume
4. The approval command publishes a NATS message - the server's workflow consumer must be running to process it

The server logs will show step processing:
```
Workflow consumer: executing step greet for run c7292c89-... (type: context)
Workflow consumer: scheduling next step ask_approval (type: await) for run c7292c89-...
Workflow consumer: step ask_approval waiting for approval appr-c7292c89-...-ask_approval
```

### Known Behavior: Approval Recorded but Workflow Doesn't Resume

If you approve a workflow but it doesn't resume, this usually means:
1. The server wasn't running when you approved
2. The NATS message was lost because no consumer was listening

**Solution**: Restart `stn serve`. On startup, it will check for pending runs with approved status and resume them.

---

## Troubleshooting

### Approval Step Auto-Skips (Doesn't Pause)

**Symptom**: The workflow completes immediately without waiting for approval.

**Cause**: Using `type: human_approval` instead of the correct pattern.

**Solution**: Use `type: operation` with `input.task: "human.approval"`:

```yaml
# WRONG
- id: approval_step
  type: human_approval
  prompt: "Approve?"

# CORRECT
- id: approval_step
  type: operation
  input:
    task: "human.approval"
    message: "Approve?"
```

### No Approvals Listed

**Symptom**: `stn workflow approvals list` returns empty.

**Cause**: 
1. Workflow not running (check `stn workflow status <run-id>`)
2. Already approved/rejected
3. Wrong step definition

**Solution**: 
1. Check run status: `stn workflow runs`
2. Verify step is actually waiting: check `current_step_id` matches approval step

### Workflow Stuck in `waiting_approval`

**Symptom**: After approving, workflow doesn't resume.

**Cause**: Approval handler didn't trigger workflow resumption.

**Solution**:
1. Check logs: `stn serve --log-level debug`
2. Restart station: `stn serve`
3. Manual check: Query database for run context

```bash
sqlite3 ~/.config/station/station.db \
  "SELECT status, current_step_id FROM workflow_runs WHERE run_id = 'YOUR_RUN_ID'"
```

### Approval ID Format

Approval IDs follow the pattern: `appr-{run_id}-{step_id}`

Example: `appr-run_abc123-ask_approval`

---

## Best Practices

1. **Always include timeout_seconds** - Prevents workflows from hanging indefinitely
   ```yaml
   timeout_seconds: 3600  # 1 hour
   ```

2. **Provide clear messages** - Include context about what's being approved
   ```yaml
   message: |
     Deployment to production:
     - Service: api-gateway
     - Version: v2.1.0
     - Changes: 15 commits
     
     Approve to proceed with deployment.
   ```

3. **Handle rejection paths** - Don't assume approval; always have a rejection handler
   ```yaml
   - id: check_result
     type: switch
     conditions:
       - if: "approval.approved == True"
         next: proceed
     defaultNext: handle_rejection  # ALWAYS have this
   ```

4. **Specify approvers** - For audit trails and access control
   ```yaml
   approvers: ["oncall-sre", "team-lead", "platform-admin"]
   ```

---

## Internal Implementation

For developers: The approval system is implemented in:

| File | Purpose |
|------|---------|
| `internal/workflows/runtime/executor.go` | `HumanApprovalExecutor` (lines 336-530) |
| `internal/workflows/translator.go` | Maps step types to executors |
| `cmd/main/workflow.go` | CLI commands for approvals |
| `internal/db/workflow_queries.go` | Database operations |

The executor checks:
1. `input["task"] == "human.approval"` - if not, returns `{skipped: true}`
2. `input["message"]` - required for the approval prompt
3. Creates approval record via `deps.CreateApproval()`
4. Returns `StepStatusWaitingApproval` to pause workflow

---

## See Also

- [Workflow Authoring Guide](./workflow-authoring-guide.md)
- [Workflows Overview](./workflows.md)
- [Starlark Expressions](../developers/starlark-expressions.md)
