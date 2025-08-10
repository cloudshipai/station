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

-- File config extensions
-- name: CreateMCPToolWithFileConfig :one
INSERT INTO mcp_tools (mcp_server_id, name, description, input_schema, file_config_id)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetMCPToolsByFileConfigID :many
SELECT * FROM mcp_tools
WHERE file_config_id = ?
ORDER BY name;

-- name: DeleteMCPToolsByFileConfigID :exec
DELETE FROM mcp_tools WHERE file_config_id = ?;

-- name: GetMCPToolsWithFileConfigInfo :many
SELECT 
    t.id, t.mcp_server_id, t.name, t.description, t.input_schema, t.created_at,
    s.name as server_name,
    fc.id as file_config_id, fc.config_name, fc.template_path, fc.last_loaded_at
FROM mcp_tools t
JOIN mcp_servers s ON t.mcp_server_id = s.id
LEFT JOIN file_mcp_configs fc ON t.file_config_id = fc.id
WHERE s.environment_id = ?
ORDER BY fc.config_name, s.name, t.name;

-- name: GetOrphanedMCPTools :many
SELECT t.*
FROM mcp_tools t
JOIN mcp_servers s ON t.mcp_server_id = s.id
LEFT JOIN file_mcp_configs fc ON t.file_config_id = fc.id
WHERE s.environment_id = ? AND t.file_config_id IS NOT NULL AND fc.id IS NULL;

-- name: UpdateMCPToolFileConfigReference :exec
UPDATE mcp_tools SET file_config_id = ? WHERE id = ?;

-- name: ClearMCPToolFileConfigReference :exec
UPDATE mcp_tools SET file_config_id = NULL WHERE id = ?;

-- name: GetMCPToolsWithServerCount :one
SELECT COUNT(*) FROM mcp_servers;

-- name: GetMCPToolsWithDetails :many
SELECT 
    t.id, t.mcp_server_id, t.name, t.description, t.input_schema, t.created_at,
    s.name as server_name,
    0 as config_id,
    'server-' || s.name as config_name,
    1 as config_version,
    s.environment_id as environment_id,
    e.name as environment_name
FROM mcp_tools t
JOIN mcp_servers s ON t.mcp_server_id = s.id
JOIN environments e ON s.environment_id = e.id
ORDER BY e.name, s.name, t.name;