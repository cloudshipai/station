-- name: CreateMCPConfig :one
INSERT INTO mcp_configs (environment_id, config_name, version, config_json, encryption_key_id)
VALUES (?, ?, ?, ?, ?)
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

-- name: GetLatestMCPConfigByName :one
SELECT * FROM mcp_configs 
WHERE environment_id = ? AND config_name = ? 
ORDER BY version DESC 
LIMIT 1;

-- name: GetLatestMCPConfigs :many
SELECT * FROM mcp_configs c1
WHERE c1.environment_id = ? 
AND version = (
    SELECT MAX(version) 
    FROM mcp_configs c2 
    WHERE c2.environment_id = c1.environment_id 
    AND c2.config_name = c1.config_name
)
ORDER BY config_name, version DESC;

-- name: GetAllLatestMCPConfigs :many
SELECT * FROM mcp_configs c1
WHERE version = (
    SELECT MAX(version) 
    FROM mcp_configs c2 
    WHERE c2.environment_id = c1.environment_id 
    AND c2.config_name = c1.config_name
)
ORDER BY environment_id, config_name, version DESC;

-- name: GetMCPConfigByVersionAndName :one
SELECT * FROM mcp_configs 
WHERE environment_id = ? AND config_name = ? AND version = ?;

-- name: ListMCPConfigsByConfigName :many
SELECT * FROM mcp_configs 
WHERE environment_id = ? AND config_name = ? 
ORDER BY version DESC;

-- name: ListAllMCPConfigs :many
SELECT * FROM mcp_configs 
ORDER BY environment_id, config_name, version DESC;

-- name: GetMCPConfigsForRotation :many
SELECT id, config_json FROM mcp_configs WHERE encryption_key_id = ?;

-- name: UpdateMCPConfigEncryption :exec
UPDATE mcp_configs SET config_json = ?, encryption_key_id = ? WHERE id = ?;

-- name: DeleteMCPConfig :exec
DELETE FROM mcp_configs WHERE id = ?;

-- name: GetNextMCPConfigVersionByName :one
SELECT COALESCE(MAX(version), 0) + 1 as next_version FROM mcp_configs WHERE environment_id = ? AND config_name = ?;