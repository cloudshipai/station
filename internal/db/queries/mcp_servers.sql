-- name: CreateMCPServer :one
INSERT INTO mcp_servers (name, command, args, env, working_dir, timeout_seconds, auto_restart, environment_id, file_config_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetMCPServer :one
SELECT * FROM mcp_servers WHERE id = ?;

-- name: ListMCPServersByEnvironment :many
SELECT * FROM mcp_servers WHERE environment_id = ? ORDER BY name;

-- name: UpdateMCPServer :one
UPDATE mcp_servers 
SET name = ?, command = ?, args = ?, env = ?, working_dir = ?, timeout_seconds = ?, auto_restart = ?, file_config_id = ?
WHERE id = ?
RETURNING *;

-- name: DeleteMCPServer :exec
DELETE FROM mcp_servers WHERE id = ?;

-- name: DeleteMCPServersByEnvironment :exec
DELETE FROM mcp_servers WHERE environment_id = ?;

-- name: DeleteMCPServersByFileConfig :exec
DELETE FROM mcp_servers WHERE file_config_id = ?;

-- name: GetMCPServerByNameAndEnvironment :one
SELECT * FROM mcp_servers WHERE name = ? AND environment_id = ?;