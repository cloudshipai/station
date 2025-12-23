-- name: InsertWorkflowRunStep :one
INSERT INTO workflow_run_steps (
    run_id,
    step_id,
    attempt,
    status,
    input,
    output,
    error,
    metadata,
    started_at,
    completed_at
) VALUES (
    sqlc.arg(run_id),
    sqlc.arg(step_id),
    sqlc.arg(attempt),
    sqlc.arg(status),
    sqlc.arg(input),
    sqlc.arg(output),
    sqlc.arg(error),
    sqlc.arg(metadata),
    COALESCE(sqlc.narg(started_at), CURRENT_TIMESTAMP),
    sqlc.narg(completed_at)
)
RETURNING id, run_id, step_id, attempt, status, input, output, error, metadata, started_at, completed_at;

-- name: UpdateWorkflowRunStep :exec
UPDATE workflow_run_steps
SET status = sqlc.arg(status),
    output = COALESCE(sqlc.narg(output), output),
    error = sqlc.narg(error),
    metadata = COALESCE(sqlc.narg(metadata), metadata),
    completed_at = COALESCE(sqlc.narg(completed_at), completed_at)
WHERE run_id = sqlc.arg(run_id) AND step_id = sqlc.arg(step_id) AND attempt = sqlc.arg(attempt);

-- name: ListWorkflowRunSteps :many
SELECT id, run_id, step_id, attempt, status, input, output, error, metadata, started_at, completed_at
FROM workflow_run_steps
WHERE run_id = sqlc.arg(run_id)
ORDER BY started_at ASC, attempt ASC;
