-- name: InsertNotificationLog :one
INSERT INTO notification_logs (
    log_id,
    approval_id,
    event_type,
    webhook_url,
    request_payload,
    response_status,
    response_body,
    error_message,
    attempt_number,
    duration_ms
) VALUES (
    sqlc.arg(log_id),
    sqlc.arg(approval_id),
    sqlc.arg(event_type),
    sqlc.narg(webhook_url),
    sqlc.narg(request_payload),
    sqlc.narg(response_status),
    sqlc.narg(response_body),
    sqlc.narg(error_message),
    sqlc.arg(attempt_number),
    sqlc.narg(duration_ms)
)
RETURNING id, log_id, approval_id, event_type, webhook_url, request_payload, response_status, response_body, error_message, attempt_number, duration_ms, created_at;

-- name: GetNotificationLog :one
SELECT id, log_id, approval_id, event_type, webhook_url, request_payload, response_status, response_body, error_message, attempt_number, duration_ms, created_at
FROM notification_logs
WHERE log_id = sqlc.arg(log_id);

-- name: ListNotificationLogsByApproval :many
SELECT id, log_id, approval_id, event_type, webhook_url, request_payload, response_status, response_body, error_message, attempt_number, duration_ms, created_at
FROM notification_logs
WHERE approval_id = sqlc.arg(approval_id)
ORDER BY created_at ASC;

-- name: ListNotificationLogsByEventType :many
SELECT id, log_id, approval_id, event_type, webhook_url, request_payload, response_status, response_body, error_message, attempt_number, duration_ms, created_at
FROM notification_logs
WHERE event_type = sqlc.arg(event_type)
ORDER BY created_at DESC
LIMIT sqlc.arg(limit);

-- name: ListRecentNotificationLogs :many
SELECT id, log_id, approval_id, event_type, webhook_url, request_payload, response_status, response_body, error_message, attempt_number, duration_ms, created_at
FROM notification_logs
ORDER BY created_at DESC
LIMIT sqlc.arg(limit);

-- name: CountNotificationLogsByApproval :one
SELECT COUNT(*) as count
FROM notification_logs
WHERE approval_id = sqlc.arg(approval_id);

-- name: DeleteOldNotificationLogs :exec
DELETE FROM notification_logs
WHERE created_at < sqlc.arg(before);
