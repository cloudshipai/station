-- name: CreateAgent :one
INSERT INTO agents (name, description, prompt, max_steps, environment_id, created_by, input_schema, cron_schedule, is_scheduled, schedule_enabled)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAgent :one
SELECT * FROM agents WHERE id = ?;

-- name: GetAgentByName :one
SELECT * FROM agents WHERE name = ?;

-- name: ListAgents :many
SELECT * FROM agents ORDER BY name;

-- name: ListAgentsByEnvironment :many
SELECT * FROM agents WHERE environment_id = ? ORDER BY name;

-- name: ListAgentsByUser :many
SELECT * FROM agents WHERE created_by = ? ORDER BY name;

-- name: UpdateAgent :exec
UPDATE agents SET name = ?, description = ?, prompt = ?, max_steps = ?, input_schema = ?, cron_schedule = ?, is_scheduled = ?, schedule_enabled = ? WHERE id = ?;

-- name: UpdateAgentPrompt :exec
UPDATE agents SET prompt = ? WHERE id = ?;

-- name: DeleteAgent :exec
DELETE FROM agents WHERE id = ?;

-- name: ListScheduledAgents :many
SELECT * FROM agents WHERE is_scheduled = TRUE AND schedule_enabled = TRUE ORDER BY next_scheduled_run;

-- name: UpdateAgentScheduleTime :exec
UPDATE agents SET last_scheduled_run = ?, next_scheduled_run = ? WHERE id = ?;

-- name: GetAgentBySchedule :one
SELECT * FROM agents WHERE id = ? AND is_scheduled = TRUE AND schedule_enabled = TRUE;

-- name: GetAgentWithTools :many
SELECT 
    a.id as agent_id,
    a.name as agent_name,
    a.description as agent_description,
    a.prompt as agent_prompt,
    a.max_steps as agent_max_steps,
    a.environment_id as agent_environment_id,
    a.created_by as agent_created_by,
    a.is_scheduled as agent_is_scheduled,
    a.schedule_enabled as agent_schedule_enabled,
    a.input_schema as agent_input_schema,
    a.created_at as agent_created_at,
    a.updated_at as agent_updated_at,
    ms.id as mcp_server_id,
    ms.name as mcp_server_name,
    mt.id as tool_id,
    mt.name as tool_name,
    mt.description as tool_description,
    mt.input_schema as tool_input_schema
FROM agents a
LEFT JOIN agent_tools at ON a.id = at.agent_id
LEFT JOIN mcp_tools mt ON at.tool_id = mt.id
LEFT JOIN mcp_servers ms ON mt.mcp_server_id = ms.id
WHERE a.id = ?
ORDER BY ms.name, mt.name;