-- Webhook queries

-- name: CreateWebhook :one
INSERT INTO webhooks (name, url, secret, enabled, events, headers, timeout_seconds, retry_attempts, created_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetWebhook :one
SELECT * FROM webhooks WHERE id = ?;

-- name: GetWebhookByName :one
SELECT * FROM webhooks WHERE name = ?;

-- name: ListWebhooks :many
SELECT * FROM webhooks ORDER BY created_at DESC;

-- name: ListEnabledWebhooks :many
SELECT * FROM webhooks WHERE enabled = TRUE ORDER BY created_at DESC;

-- name: UpdateWebhook :exec
UPDATE webhooks 
SET name = ?, url = ?, secret = ?, enabled = ?, events = ?, headers = ?, 
    timeout_seconds = ?, retry_attempts = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteWebhook :exec
DELETE FROM webhooks WHERE id = ?;

-- name: EnableWebhook :exec
UPDATE webhooks SET enabled = TRUE, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: DisableWebhook :exec
UPDATE webhooks SET enabled = FALSE, updated_at = CURRENT_TIMESTAMP WHERE id = ?;