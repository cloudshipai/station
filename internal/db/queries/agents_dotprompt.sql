-- Agent queries for dotprompt architecture (name-based primary keys)

-- name: GetAgentByName :one
SELECT * FROM agents WHERE name = ? AND environment_name = ?;

-- name: GetAgentByPath :one
SELECT * FROM agents WHERE file_path = ?;

-- name: ListAgents :many
SELECT * FROM agents ORDER BY name;

-- name: ListAgentsByEnvironment :many  
SELECT * FROM agents WHERE environment_name = ? ORDER BY name;

-- name: ListActiveAgents :many
SELECT * FROM agents WHERE status = 'active' AND environment_name = ? ORDER BY name;

-- name: CreateAgent :one
INSERT INTO agents (
    name, display_name, description, file_path, environment_name,
    cron_schedule, is_scheduled, schedule_enabled, created_by
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateAgent :exec
UPDATE agents 
SET display_name = ?, description = ?, file_path = ?, 
    cron_schedule = ?, is_scheduled = ?, schedule_enabled = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE name = ? AND environment_name = ?;

-- name: UpdateAgentFromFile :exec  
UPDATE agents 
SET display_name = ?, description = ?, checksum_md5 = ?, 
    updated_at = CURRENT_TIMESTAMP
WHERE name = ? AND environment_name = ?;

-- name: UpdateAgentChecksum :exec
UPDATE agents SET checksum_md5 = ?, updated_at = CURRENT_TIMESTAMP 
WHERE name = ? AND environment_name = ?;

-- name: UpdateAgentStatus :exec
UPDATE agents SET status = ?, updated_at = CURRENT_TIMESTAMP 
WHERE name = ? AND environment_name = ?;

-- name: DeleteAgent :exec
DELETE FROM agents WHERE name = ? AND environment_name = ?;

-- name: CountAgentsByEnvironment :one
SELECT COUNT(*) FROM agents WHERE environment_name = ?;

-- name: GetAgentExecutionStats :one
SELECT 
    execution_count,
    last_executed_at,
    (SELECT COUNT(*) FROM agent_runs WHERE agent_name = ? AND environment_name = ?) as total_runs,
    (SELECT COUNT(*) FROM agent_runs WHERE agent_name = ? AND environment_name = ? AND status = 'completed') as successful_runs
FROM agents WHERE name = ? AND environment_name = ?;

-- Agent runs queries (updated for name-based references)

-- name: CreateAgentRun :one
INSERT INTO agent_runs (
    agent_name, environment_name, task, status, result, error,
    duration_ms, steps_taken, tools_used, token_usage, tool_calls, execution_steps, response_metadata
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAgentRun :one
SELECT * FROM agent_runs WHERE id = ?;

-- name: ListAgentRuns :many
SELECT * FROM agent_runs 
WHERE agent_name = ? AND environment_name = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: ListRecentRuns :many
SELECT ar.*, a.display_name as agent_display_name
FROM agent_runs ar
JOIN agents a ON ar.agent_name = a.name AND ar.environment_name = a.environment_name
WHERE ar.environment_name = ?
ORDER BY ar.created_at DESC
LIMIT ?;

-- name: UpdateAgentRunStatus :exec
UPDATE agent_runs 
SET status = ?, result = ?, error = ?, duration_ms = ?
WHERE id = ?;

-- Agent tools queries (updated for MCP config mapping)

-- name: GetAgentTools :many
SELECT mcp_config_name, tool_name, assigned_at
FROM agent_tools 
WHERE agent_name = ? AND environment_name = ?
ORDER BY mcp_config_name, tool_name;

-- name: GetAgentToolsByMCPConfig :many
SELECT tool_name, assigned_at
FROM agent_tools 
WHERE agent_name = ? AND environment_name = ? AND mcp_config_name = ?
ORDER BY tool_name;

-- name: AssignToolToAgent :exec
INSERT OR REPLACE INTO agent_tools (agent_name, environment_name, mcp_config_name, tool_name)
VALUES (?, ?, ?, ?);

-- name: RemoveToolFromAgent :exec
DELETE FROM agent_tools 
WHERE agent_name = ? AND environment_name = ? AND tool_name = ?;

-- name: ClearAgentTools :exec
DELETE FROM agent_tools 
WHERE agent_name = ? AND environment_name = ?;

-- name: ListToolsByMCPConfig :many
SELECT DISTINCT at.agent_name, at.tool_name, a.display_name as agent_display_name
FROM agent_tools at
JOIN agents a ON at.agent_name = a.name AND at.environment_name = a.environment_name
WHERE at.mcp_config_name = ? AND at.environment_name = ?
ORDER BY at.agent_name, at.tool_name;

-- Scheduling queries (preserved functionality)

-- name: ListScheduledAgents :many
SELECT * FROM agents 
WHERE is_scheduled = TRUE AND schedule_enabled = TRUE AND status = 'active'
ORDER BY next_scheduled_run;

-- name: UpdateAgentScheduleTime :exec
UPDATE agents 
SET last_scheduled_run = ?, next_scheduled_run = ?, execution_count = execution_count + 1,
    last_executed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE name = ? AND environment_name = ?;

-- name: GetScheduledAgent :one
SELECT * FROM agents 
WHERE name = ? AND environment_name = ? AND is_scheduled = TRUE AND schedule_enabled = TRUE;

-- File synchronization helpers

-- name: GetAgentsWithFileChanges :many
-- Returns agents where file checksum doesn't match database
SELECT name, environment_name, file_path, checksum_md5
FROM agents 
WHERE file_path IS NOT NULL AND environment_name = ?
ORDER BY name;

-- name: GetOrphanedAgents :many  
-- Returns agents in database that don't have corresponding .prompt files
SELECT name, environment_name, file_path
FROM agents
WHERE file_path IS NOT NULL 
  AND environment_name = ?
  AND NOT EXISTS (SELECT 1 FROM agents WHERE file_path IS NOT NULL);

-- name: MarkAgentSynced :exec
UPDATE agents 
SET checksum_md5 = ?, status = 'active', updated_at = CURRENT_TIMESTAMP
WHERE name = ? AND environment_name = ?;