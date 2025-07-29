-- name: CreateMCPServer :one
INSERT INTO mcp_servers (config_id, server_name, server_url)
VALUES (?, ?, ?)
RETURNING *;

-- name: GetMCPServer :one
SELECT * FROM mcp_servers WHERE id = ?;

-- name: ListMCPServersByConfig :many
SELECT * FROM mcp_servers WHERE config_id = ? ORDER BY server_name;

-- name: ListMCPServersByEnvironment :many
SELECT s.* FROM mcp_servers s
JOIN mcp_configs c ON s.config_id = c.id
WHERE c.environment_id = ? AND c.version = (
    SELECT MAX(mc.version) FROM mcp_configs mc WHERE mc.environment_id = c.environment_id
)
ORDER BY s.server_name;

-- name: DeleteMCPServersByConfig :exec
DELETE FROM mcp_servers WHERE config_id = ?;