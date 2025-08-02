-- name: AddAgentTool :one
INSERT INTO agent_tools (agent_id, tool_id)
VALUES (?, ?)
RETURNING *;

-- name: RemoveAgentTool :exec
DELETE FROM agent_tools WHERE agent_id = ? AND tool_id = ?;

-- name: ListAgentTools :many
SELECT at.id, at.agent_id, at.tool_id, at.created_at, 
       t.name as tool_name, t.description as tool_description, t.input_schema as tool_schema, 
       s.name as server_name, s.environment_id
FROM agent_tools at
JOIN mcp_tools t ON at.tool_id = t.id 
JOIN mcp_servers s ON t.mcp_server_id = s.id
JOIN agents a ON at.agent_id = a.id
-- Ensure tools belong to the agent's environment
WHERE at.agent_id = ? AND s.environment_id = a.environment_id
ORDER BY s.name, t.name;

-- name: ListAvailableToolsForAgent :many
-- List all tools available in the agent's environment that aren't already assigned
SELECT t.id, t.name as tool_name, t.description as tool_description, t.input_schema as tool_schema,
       s.name as server_name
FROM mcp_tools t
JOIN mcp_servers s ON t.mcp_server_id = s.id
JOIN agents a ON s.environment_id = a.environment_id
WHERE a.id = ? 
AND t.id NOT IN (
    SELECT tool_id FROM agent_tools WHERE agent_id = ?
)
ORDER BY s.name, t.name;

-- name: ClearAgentTools :exec
DELETE FROM agent_tools WHERE agent_id = ?;