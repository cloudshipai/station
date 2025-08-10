-- name: CreateUser :one
INSERT INTO users (username, public_key, is_admin, api_key)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE id = ?;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = ?;

-- name: GetUserByAPIKey :one
SELECT * FROM users WHERE api_key = ?;

-- name: ListUsers :many
SELECT * FROM users ORDER BY username;

-- name: UpdateUser :exec
UPDATE users SET username = ?, is_admin = ? WHERE id = ?;

-- name: UpdateUserAPIKey :exec
UPDATE users SET api_key = ? WHERE id = ?;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = ?;