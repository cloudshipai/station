-- Settings queries

-- name: GetSetting :one
SELECT * FROM settings WHERE key = ?;

-- name: GetAllSettings :many
SELECT * FROM settings ORDER BY key;

-- name: SetSetting :exec
INSERT INTO settings (key, value, description) 
VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET
    value = excluded.value,
    updated_at = CURRENT_TIMESTAMP;

-- name: DeleteSetting :exec
DELETE FROM settings WHERE key = ?;