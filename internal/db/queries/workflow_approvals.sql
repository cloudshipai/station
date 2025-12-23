-- name: InsertWorkflowApproval :one
INSERT INTO workflow_approvals (
    approval_id,
    run_id,
    step_id,
    message,
    summary_path,
    approvers,
    status,
    timeout_at
) VALUES (
    sqlc.arg(approval_id),
    sqlc.arg(run_id),
    sqlc.arg(step_id),
    sqlc.arg(message),
    sqlc.narg(summary_path),
    sqlc.narg(approvers),
    sqlc.arg(status),
    sqlc.narg(timeout_at)
)
RETURNING id, approval_id, run_id, step_id, message, summary_path, approvers, status, decided_by, decided_at, decision_reason, timeout_at, created_at, updated_at;

-- name: GetWorkflowApproval :one
SELECT id, approval_id, run_id, step_id, message, summary_path, approvers, status, decided_by, decided_at, decision_reason, timeout_at, created_at, updated_at
FROM workflow_approvals
WHERE approval_id = sqlc.arg(approval_id);

-- name: ListWorkflowApprovals :many
SELECT id, approval_id, run_id, step_id, message, summary_path, approvers, status, decided_by, decided_at, decision_reason, timeout_at, created_at, updated_at
FROM workflow_approvals
WHERE run_id = sqlc.arg(run_id)
ORDER BY created_at ASC;

-- name: ListPendingApprovals :many
SELECT id, approval_id, run_id, step_id, message, summary_path, approvers, status, decided_by, decided_at, decision_reason, timeout_at, created_at, updated_at
FROM workflow_approvals
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT sqlc.arg(limit);

-- name: ApproveWorkflowApproval :exec
UPDATE workflow_approvals
SET
    status = 'approved',
    decided_by = sqlc.arg(decided_by),
    decided_at = CURRENT_TIMESTAMP,
    decision_reason = sqlc.narg(decision_reason),
    updated_at = CURRENT_TIMESTAMP
WHERE approval_id = sqlc.arg(approval_id)
  AND status = 'pending';

-- name: RejectWorkflowApproval :exec
UPDATE workflow_approvals
SET
    status = 'rejected',
    decided_by = sqlc.arg(decided_by),
    decided_at = CURRENT_TIMESTAMP,
    decision_reason = sqlc.narg(decision_reason),
    updated_at = CURRENT_TIMESTAMP
WHERE approval_id = sqlc.arg(approval_id)
  AND status = 'pending';

-- name: TimeoutExpiredApprovals :exec
UPDATE workflow_approvals
SET
    status = 'timed_out',
    updated_at = CURRENT_TIMESTAMP
WHERE status = 'pending'
  AND timeout_at IS NOT NULL
  AND timeout_at < CURRENT_TIMESTAMP;
