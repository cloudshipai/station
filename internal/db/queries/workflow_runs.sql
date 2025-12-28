-- name: InsertWorkflowRun :one
INSERT INTO workflow_runs (
    run_id,
    workflow_id,
    workflow_version,
    status,
    current_step,
    input,
    context,
    result,
    error,
    summary,
    options,
    last_signal,
    started_at,
    completed_at
) VALUES (
    sqlc.arg(run_id),
    sqlc.arg(workflow_id),
    sqlc.arg(workflow_version),
    sqlc.arg(status),
    sqlc.arg(current_step),
    sqlc.arg(input),
    sqlc.arg(context),
    sqlc.arg(result),
    sqlc.arg(error),
    sqlc.arg(summary),
    sqlc.arg(options),
    sqlc.arg(last_signal),
    COALESCE(sqlc.narg(started_at), CURRENT_TIMESTAMP),
    sqlc.narg(completed_at)
)
RETURNING id, run_id, workflow_id, workflow_version, status, current_step, input, context, result, error, summary, options, last_signal, created_at, updated_at, started_at, completed_at;

-- name: GetWorkflowRun :one
SELECT id, run_id, workflow_id, workflow_version, status, current_step, input, context, result, error, summary, options, last_signal, created_at, updated_at, started_at, completed_at
FROM workflow_runs
WHERE run_id = sqlc.arg(run_id);

-- name: ListWorkflowRuns :many
SELECT id, run_id, workflow_id, workflow_version, status, current_step, input, context, result, error, summary, options, last_signal, created_at, updated_at, started_at, completed_at
FROM workflow_runs
WHERE (sqlc.narg(workflow_id) IS NULL OR workflow_id = sqlc.narg(workflow_id))
  AND (sqlc.narg(status) IS NULL OR status = sqlc.narg(status))
ORDER BY created_at DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset);

-- name: UpdateWorkflowRunStatus :exec
UPDATE workflow_runs
SET
    status = sqlc.arg(status),
    current_step = sqlc.narg(current_step),
    context = COALESCE(sqlc.narg(context), context),
    result = COALESCE(sqlc.narg(result), result),
    error = sqlc.narg(error),
    summary = COALESCE(sqlc.narg(summary), summary),
    options = COALESCE(sqlc.narg(options), options),
    last_signal = COALESCE(sqlc.narg(last_signal), last_signal),
    updated_at = CURRENT_TIMESTAMP,
    completed_at = COALESCE(sqlc.narg(completed_at), completed_at)
WHERE run_id = sqlc.arg(run_id);

-- name: DeleteWorkflowRun :exec
DELETE FROM workflow_runs WHERE run_id = sqlc.arg(run_id);

-- name: DeleteWorkflowRunsByIDs :exec
DELETE FROM workflow_runs WHERE run_id IN (sqlc.slice(run_ids));

-- name: DeleteWorkflowRunsByStatus :exec
DELETE FROM workflow_runs WHERE status = sqlc.arg(status);

-- name: DeleteWorkflowRunsByWorkflowID :exec
DELETE FROM workflow_runs WHERE workflow_id = sqlc.arg(workflow_id);

-- name: DeleteAllWorkflowRuns :exec
DELETE FROM workflow_runs;

-- name: CountWorkflowRuns :one
SELECT COUNT(*) FROM workflow_runs;
