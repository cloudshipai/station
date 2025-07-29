-- name: CreateModelProvider :one
INSERT INTO model_providers (
    name, display_name, base_url, api_key, headers, enabled, is_default
) VALUES (
    ?, ?, ?, ?, ?, ?, ?
) RETURNING *;

-- name: GetModelProvider :one
SELECT * FROM model_providers WHERE id = ?;

-- name: GetModelProviderByName :one
SELECT * FROM model_providers WHERE name = ?;

-- name: ListModelProviders :many
SELECT * FROM model_providers ORDER BY name;

-- name: ListEnabledModelProviders :many
SELECT * FROM model_providers WHERE enabled = true ORDER BY name;

-- name: GetDefaultModelProvider :one
SELECT * FROM model_providers WHERE is_default = true AND enabled = true LIMIT 1;

-- name: UpdateModelProvider :exec
UPDATE model_providers 
SET display_name = ?, base_url = ?, api_key = ?, headers = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: SetDefaultModelProvider :exec
UPDATE model_providers SET is_default = CASE WHEN id = ? THEN true ELSE false END;

-- name: DeleteModelProvider :exec
DELETE FROM model_providers WHERE id = ?;