-- name: GetNextWorkflowVersion :one
SELECT COALESCE(MAX(version), 0) + 1 AS next_version
FROM workflows
WHERE workflow_id = sqlc.arg(workflow_id);

-- name: InsertWorkflow :one
INSERT INTO workflows (workflow_id, name, description, version, definition, status)
VALUES (sqlc.arg(workflow_id), sqlc.arg(name), sqlc.arg(description), sqlc.arg(version), sqlc.arg(definition), sqlc.arg(status))
RETURNING id, workflow_id, name, description, version, definition, status, created_at, updated_at;

-- name: GetWorkflow :one
SELECT id, workflow_id, name, description, version, definition, status, created_at, updated_at
FROM workflows
WHERE workflow_id = sqlc.arg(workflow_id) AND version = sqlc.arg(version);

-- name: GetLatestWorkflow :one
SELECT id, workflow_id, name, description, version, definition, status, created_at, updated_at
FROM workflows
WHERE workflow_id = sqlc.arg(workflow_id)
ORDER BY version DESC
LIMIT 1;

-- name: ListLatestWorkflows :many
WITH latest AS (
    SELECT workflow_id, MAX(version) AS version
    FROM workflows
    GROUP BY workflow_id
)
SELECT w.id, w.workflow_id, w.name, w.description, w.version, w.definition, w.status, w.created_at, w.updated_at
FROM workflows w
JOIN latest l ON w.workflow_id = l.workflow_id AND w.version = l.version
ORDER BY w.workflow_id;

-- name: ListWorkflowVersions :many
SELECT id, workflow_id, name, description, version, definition, status, created_at, updated_at
FROM workflows
WHERE workflow_id = sqlc.arg(workflow_id)
ORDER BY version DESC;

-- name: DisableWorkflow :exec
UPDATE workflows
SET status = 'disabled', updated_at = CURRENT_TIMESTAMP
WHERE workflow_id = sqlc.arg(workflow_id);
