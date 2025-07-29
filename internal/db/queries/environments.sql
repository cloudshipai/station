-- name: CreateEnvironment :one
INSERT INTO environments (name, description)
VALUES (?, ?)
RETURNING *;

-- name: GetEnvironment :one
SELECT * FROM environments WHERE id = ?;

-- name: GetEnvironmentByName :one
SELECT * FROM environments WHERE name = ?;

-- name: ListEnvironments :many
SELECT * FROM environments ORDER BY name;

-- name: UpdateEnvironment :exec
UPDATE environments SET name = ?, description = ? WHERE id = ?;

-- name: DeleteEnvironment :exec
DELETE FROM environments WHERE id = ?;