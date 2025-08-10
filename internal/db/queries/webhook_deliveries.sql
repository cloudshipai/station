-- Webhook delivery queries

-- name: CreateWebhookDelivery :one
INSERT INTO webhook_deliveries (webhook_id, event_type, payload, status, attempt_count)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetWebhookDelivery :one
SELECT * FROM webhook_deliveries WHERE id = ?;

-- name: ListWebhookDeliveries :many
SELECT * FROM webhook_deliveries ORDER BY created_at DESC LIMIT ?;

-- name: ListWebhookDeliveriesByWebhook :many
SELECT * FROM webhook_deliveries WHERE webhook_id = ? ORDER BY created_at DESC LIMIT ?;

-- name: ListPendingDeliveries :many
SELECT * FROM webhook_deliveries WHERE status = 'pending' ORDER BY created_at ASC;

-- name: ListFailedDeliveriesForRetry :many
SELECT * FROM webhook_deliveries 
WHERE status = 'failed' 
    AND next_retry_at IS NOT NULL 
    AND next_retry_at <= CURRENT_TIMESTAMP
ORDER BY next_retry_at ASC;

-- name: UpdateDeliveryStatus :exec
UPDATE webhook_deliveries 
SET status = ?, 
    http_status_code = ?, 
    response_body = ?, 
    response_headers = ?, 
    error_message = ?,
    last_attempt_at = CURRENT_TIMESTAMP,
    delivered_at = CASE WHEN ? = 'success' THEN CURRENT_TIMESTAMP ELSE delivered_at END
WHERE id = ?;

-- name: UpdateDeliveryForRetry :exec
UPDATE webhook_deliveries 
SET attempt_count = attempt_count + 1,
    last_attempt_at = CURRENT_TIMESTAMP,
    next_retry_at = ?,
    status = 'failed',
    http_status_code = ?,
    response_body = ?,
    response_headers = ?,
    error_message = ?
WHERE id = ?;

-- name: MarkDeliveryAsSuccess :exec
UPDATE webhook_deliveries 
SET status = 'success',
    http_status_code = ?,
    response_body = ?,
    response_headers = ?,
    delivered_at = CURRENT_TIMESTAMP,
    last_attempt_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: MarkDeliveryAsFailed :exec
UPDATE webhook_deliveries 
SET status = 'failed',
    error_message = ?,
    last_attempt_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteOldDeliveries :exec
DELETE FROM webhook_deliveries 
WHERE created_at < ? AND status IN ('success', 'failed');

-- name: GetDeliveryStats :one
SELECT 
    COUNT(*) as total_deliveries,
    COUNT(CASE WHEN status = 'success' THEN 1 END) as successful_deliveries,
    COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed_deliveries,
    COUNT(CASE WHEN status = 'pending' THEN 1 END) as pending_deliveries
FROM webhook_deliveries 
WHERE webhook_id = ? AND created_at >= ?;