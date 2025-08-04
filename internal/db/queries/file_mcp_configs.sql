-- File-based MCP configuration queries

-- name: CreateFileMCPConfig :one
INSERT INTO file_mcp_configs (
    environment_id, config_name, template_path, variables_path,
    template_specific_vars_path, template_hash, variables_hash,
    template_vars_hash, metadata
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetFileMCPConfig :one
SELECT * FROM file_mcp_configs WHERE id = ?;

-- name: GetFileMCPConfigByEnvironmentAndName :one
SELECT * FROM file_mcp_configs WHERE environment_id = ? AND config_name = ?;

-- name: ListFileMCPConfigsByEnvironment :many
SELECT * FROM file_mcp_configs WHERE environment_id = ? ORDER BY config_name;

-- name: UpdateFileMCPConfigHashes :exec
UPDATE file_mcp_configs
SET template_hash = ?, variables_hash = ?, template_vars_hash = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateFileMCPConfigLastLoadedAt :exec
UPDATE file_mcp_configs
SET last_loaded_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateFileMCPConfigMetadata :exec
UPDATE file_mcp_configs
SET metadata = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteFileMCPConfig :exec
DELETE FROM file_mcp_configs WHERE id = ?;

-- name: DeleteFileMCPConfigByEnvironmentAndName :exec
DELETE FROM file_mcp_configs WHERE environment_id = ? AND config_name = ?;

-- name: GetFileMCPConfigsForChangeDetection :many
SELECT id, environment_id, config_name, template_path, variables_path,
       template_hash, variables_hash, template_vars_hash, last_loaded_at
FROM file_mcp_configs
WHERE environment_id = ?;

-- name: CheckFileMCPConfigChanges :one
SELECT COUNT(*)
FROM file_mcp_configs
WHERE environment_id = ? AND config_name = ?
  AND template_hash = ? AND variables_hash = ? AND template_vars_hash = ?;