-- name: AddAgentTool :one
INSERT INTO agent_tools (agent_id, tool_id)
VALUES (?, ?)
RETURNING *;

-- name: RemoveAgentTool :exec
DELETE FROM agent_tools WHERE agent_id = ? AND tool_id = ?;

-- name: ListAgentTools :many
SELECT at.*, t.name as tool_name, t.description as tool_description, t.input_schema as tool_schema, s.name as server_name
FROM agent_tools at
JOIN mcp_tools t ON at.tool_id = t.id
JOIN mcp_servers s ON t.mcp_server_id = s.id
WHERE at.agent_id = ?
ORDER BY s.name, t.name;

-- name: ClearAgentTools :exec
DELETE FROM agent_tools WHERE agent_id = ?;