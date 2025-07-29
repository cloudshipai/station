-- name: AddAgentTool :one
INSERT INTO agent_tools (agent_id, tool_id)
VALUES (?, ?)
RETURNING *;

-- name: RemoveAgentTool :exec
DELETE FROM agent_tools WHERE agent_id = ? AND tool_id = ?;

-- name: ListAgentTools :many
SELECT at.*, t.tool_name, t.tool_description, t.tool_schema, s.server_name
FROM agent_tools at
JOIN mcp_tools t ON at.tool_id = t.id
JOIN mcp_servers s ON t.server_id = s.id
WHERE at.agent_id = ?
ORDER BY s.server_name, t.tool_name;

-- name: ClearAgentTools :exec
DELETE FROM agent_tools WHERE agent_id = ?;