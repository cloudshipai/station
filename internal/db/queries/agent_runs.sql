-- name: CreateAgentRun :one
INSERT INTO agent_runs (agent_id, user_id, task, final_response, steps_taken, tool_calls, execution_steps, status, completed_at, input_tokens, output_tokens, total_tokens, duration_seconds, model_name, tools_used, debug_logs)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: CreateAgentRunBasic :one
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

-- name: GetAgentRunWithDetails :one
SELECT ar.id, ar.agent_id, ar.user_id, ar.task, ar.final_response, ar.steps_taken, 
       ar.tool_calls, ar.execution_steps, ar.status, ar.started_at, ar.completed_at,
       ar.input_tokens, ar.output_tokens, ar.total_tokens, ar.duration_seconds, ar.model_name, ar.tools_used, ar.debug_logs,
       a.name as agent_name, u.username
FROM agent_runs ar
JOIN agents a ON ar.agent_id = a.id
JOIN users u ON ar.user_id = u.id
WHERE ar.id = ?;

-- name: UpdateAgentRunCompletion :exec
UPDATE agent_runs 
SET final_response = ?, steps_taken = ?, tool_calls = ?, execution_steps = ?, status = ?, completed_at = ?, input_tokens = ?, output_tokens = ?, total_tokens = ?, duration_seconds = ?, model_name = ?, tools_used = ?
WHERE id = ?;

-- name: UpdateAgentRunStatus :exec
UPDATE agent_runs SET status = ? WHERE id = ?;

-- name: UpdateAgentRunDebugLogs :exec
UPDATE agent_runs SET debug_logs = ? WHERE id = ?;