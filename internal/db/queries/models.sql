-- name: CreateModel :one
INSERT INTO models (
    provider_id, model_id, name, context_size, max_tokens, supports_tools, input_cost, output_cost, enabled
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?
) RETURNING *;

-- name: GetModel :one
SELECT * FROM models WHERE id = ?;

-- name: GetModelByProviderAndModelID :one
SELECT * FROM models WHERE provider_id = ? AND model_id = ?;

-- name: ListModels :many
SELECT m.*, mp.name as provider_name, mp.display_name as provider_display_name
FROM models m
JOIN model_providers mp ON m.provider_id = mp.id
ORDER BY mp.name, m.name;

-- name: ListModelsByProvider :many
SELECT * FROM models WHERE provider_id = ? ORDER BY name;

-- name: ListEnabledModels :many
SELECT m.*, mp.name as provider_name, mp.display_name as provider_display_name
FROM models m
JOIN model_providers mp ON m.provider_id = mp.id
WHERE m.enabled = true AND mp.enabled = true
ORDER BY mp.name, m.name;

-- name: ListToolSupportingModels :many
SELECT m.*, mp.name as provider_name, mp.display_name as provider_display_name
FROM models m
JOIN model_providers mp ON m.provider_id = mp.id
WHERE m.supports_tools = true AND m.enabled = true AND mp.enabled = true
ORDER BY mp.name, m.name;

-- name: UpdateModel :exec
UPDATE models 
SET name = ?, context_size = ?, max_tokens = ?, supports_tools = ?, input_cost = ?, output_cost = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteModel :exec
DELETE FROM models WHERE id = ?;

-- name: DeleteModelsByProvider :exec
DELETE FROM models WHERE provider_id = ?;