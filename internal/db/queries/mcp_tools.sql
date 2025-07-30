-- name: CreateMCPTool :one
INSERT INTO mcp_tools (mcp_server_id, name, description, input_schema)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetMCPTool :one
SELECT * FROM mcp_tools WHERE id = ?;

-- name: ListMCPToolsByServer :many
SELECT * FROM mcp_tools WHERE mcp_server_id = ? ORDER BY name;

-- name: ListMCPToolsByEnvironment :many
SELECT t.* FROM mcp_tools t
JOIN mcp_servers s ON t.server_id = s.id
JOIN mcp_configs c ON s.config_id = c.id
WHERE c.environment_id = ? AND c.version = (
    SELECT MAX(mc.version) FROM mcp_configs mc WHERE mc.environment_id = c.environment_id
)
ORDER BY s.server_name, t.tool_name;

-- name: ListMCPToolsByServerInEnvironment :many
SELECT t.* FROM mcp_tools t
JOIN mcp_servers s ON t.server_id = s.id
JOIN mcp_configs c ON s.config_id = c.id
WHERE c.environment_id = ? AND s.server_name = ? AND c.version = (
    SELECT MAX(mc.version) FROM mcp_configs mc WHERE mc.environment_id = c.environment_id
)
ORDER BY t.tool_name;

-- name: DeleteMCPToolsByServer :exec
DELETE FROM mcp_tools WHERE server_id = ?;