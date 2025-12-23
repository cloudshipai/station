-- name: InsertWorkflowRunEvent :one
INSERT INTO workflow_run_events (
    run_id,
    seq,
    event_type,
    step_id,
    payload,
    actor
) VALUES (
    sqlc.arg(run_id),
    sqlc.arg(seq),
    sqlc.arg(event_type),
    sqlc.narg(step_id),
    sqlc.narg(payload),
    sqlc.narg(actor)
)
RETURNING id, run_id, seq, event_type, step_id, payload, actor, created_at;

-- name: GetNextEventSeq :one
SELECT COALESCE(MAX(seq), 0) + 1 AS next_seq
FROM workflow_run_events
WHERE run_id = sqlc.arg(run_id);

-- name: ListWorkflowRunEvents :many
SELECT id, run_id, seq, event_type, step_id, payload, actor, created_at
FROM workflow_run_events
WHERE run_id = sqlc.arg(run_id)
ORDER BY seq ASC;

-- name: ListWorkflowRunEventsByType :many
SELECT id, run_id, seq, event_type, step_id, payload, actor, created_at
FROM workflow_run_events
WHERE run_id = sqlc.arg(run_id)
  AND event_type = sqlc.arg(event_type)
ORDER BY seq ASC;
