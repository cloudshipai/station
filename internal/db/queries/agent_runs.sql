-- name: CreateAgentRun :one
INSERT INTO agent_runs (agent_id, user_id, task, final_response, steps_taken, tool_calls, execution_steps, status, completed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAgentRun :one
SELECT * FROM agent_runs WHERE id = ?;

-- name: ListAgentRuns :many
SELECT * FROM agent_runs ORDER BY started_at DESC;

-- name: ListAgentRunsByAgent :many
SELECT * FROM agent_runs WHERE agent_id = ? ORDER BY started_at DESC;

-- name: ListAgentRunsByUser :many
SELECT * FROM agent_runs WHERE user_id = ? ORDER BY started_at DESC;

-- name: ListRecentAgentRuns :many
SELECT ar.*, a.name as agent_name, u.username
FROM agent_runs ar
JOIN agents a ON ar.agent_id = a.id
JOIN users u ON ar.user_id = u.id
ORDER BY ar.started_at DESC
LIMIT ?;