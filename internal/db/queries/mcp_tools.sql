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
JOIN mcp_servers s ON t.mcp_server_id = s.id
WHERE s.environment_id = ?
ORDER BY s.name, t.name;

-- name: ListMCPToolsByServerInEnvironment :many
SELECT t.* FROM mcp_tools t
JOIN mcp_servers s ON t.mcp_server_id = s.id
WHERE s.environment_id = ? AND s.name = ?
ORDER BY t.name;

-- name: FindMCPToolByNameInEnvironment :one
SELECT t.* FROM mcp_tools t
JOIN mcp_servers s ON t.mcp_server_id = s.id
WHERE s.environment_id = ? AND t.name = ?
LIMIT 1;

-- name: DeleteMCPToolsByServer :exec
DELETE FROM mcp_tools WHERE mcp_server_id = ?;