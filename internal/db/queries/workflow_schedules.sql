-- name: CreateWorkflowSchedule :one
INSERT INTO workflow_schedules (
    workflow_id,
    workflow_version,
    cron_expression,
    timezone,
    enabled,
    input,
    next_run_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetWorkflowSchedule :one
SELECT * FROM workflow_schedules
WHERE workflow_id = ? AND workflow_version = ?;

-- name: GetWorkflowScheduleByID :one
SELECT * FROM workflow_schedules
WHERE id = ?;

-- name: ListEnabledSchedules :many
SELECT * FROM workflow_schedules
WHERE enabled = TRUE
ORDER BY next_run_at ASC;

-- name: ListDueSchedules :many
SELECT * FROM workflow_schedules
WHERE enabled = TRUE
  AND next_run_at <= ?
ORDER BY next_run_at ASC;

-- name: UpdateScheduleLastRun :exec
UPDATE workflow_schedules
SET last_run_at = ?,
    next_run_at = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateScheduleEnabled :exec
UPDATE workflow_schedules
SET enabled = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE workflow_id = ? AND workflow_version = ?;

-- name: UpdateWorkflowSchedule :exec
UPDATE workflow_schedules
SET cron_expression = ?,
    timezone = ?,
    enabled = ?,
    input = ?,
    next_run_at = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE workflow_id = ? AND workflow_version = ?;

-- name: DeleteWorkflowSchedule :exec
DELETE FROM workflow_schedules
WHERE workflow_id = ? AND workflow_version = ?;

-- name: DeleteWorkflowScheduleByWorkflowID :exec
DELETE FROM workflow_schedules
WHERE workflow_id = ?;
