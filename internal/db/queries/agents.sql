-- name: CreateAgent :one
INSERT INTO agents (name, description, prompt, max_steps, environment_id, created_by, cron_schedule, is_scheduled, schedule_enabled)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
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
UPDATE agents SET name = ?, description = ?, prompt = ?, max_steps = ?, cron_schedule = ?, is_scheduled = ?, schedule_enabled = ? WHERE id = ?;

-- name: UpdateAgentPrompt :exec
UPDATE agents SET prompt = ? WHERE id = ?;

-- name: DeleteAgent :exec
DELETE FROM agents WHERE id = ?;

-- name: ListScheduledAgents :many
SELECT * FROM agents WHERE is_scheduled = TRUE AND schedule_enabled = TRUE ORDER BY next_scheduled_run;

-- name: UpdateAgentScheduleTime :exec
UPDATE agents SET last_scheduled_run = ?, next_scheduled_run = ? WHERE id = ?;

-- name: GetAgentBySchedule :one
SELECT * FROM agents WHERE id = ? AND is_scheduled = TRUE AND schedule_enabled = TRUE;