-- name: CreateMCPConfig :one
INSERT INTO mcp_configs (environment_id, version, config_json, encryption_key_id)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetMCPConfig :one
SELECT * FROM mcp_configs WHERE id = ?;

-- name: GetLatestMCPConfig :one
SELECT * FROM mcp_configs 
WHERE environment_id = ? 
ORDER BY version DESC 
LIMIT 1;

-- name: GetMCPConfigByVersion :one
SELECT * FROM mcp_configs 
WHERE environment_id = ? AND version = ?;

-- name: ListMCPConfigsByEnvironment :many
SELECT * FROM mcp_configs 
WHERE environment_id = ? 
ORDER BY version DESC;

-- name: GetNextMCPConfigVersion :one
SELECT COALESCE(MAX(version), 0) + 1 as next_version
FROM mcp_configs 
WHERE environment_id = ?;