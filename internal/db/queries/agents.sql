-- name: CreateAgent :one
INSERT INTO agents (name, description, prompt, max_steps, environment_id, created_by)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAgent :one
SELECT * FROM agents WHERE id = ?;

-- name: GetAgentByName :one
SELECT * FROM agents WHERE name = ?;

-- name: ListAgents :many
SELECT * FROM agents ORDER BY name;

-- name: ListAgentsByEnvironment :many
SELECT * FROM agents WHERE environment_id = ? ORDER BY name;

-- name: ListAgentsByUser :many
SELECT * FROM agents WHERE created_by = ? ORDER BY name;

-- name: UpdateAgent :exec
UPDATE agents SET name = ?, description = ?, prompt = ?, max_steps = ? WHERE id = ?;

-- name: DeleteAgent :exec
DELETE FROM agents WHERE id = ?;