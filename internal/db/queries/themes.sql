-- name: CreateTheme :one
INSERT INTO themes (name, display_name, description, is_built_in, is_default, created_by)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetTheme :one
SELECT * FROM themes WHERE id = ?;

-- name: GetThemeByName :one
SELECT * FROM themes WHERE name = ?;

-- name: ListThemes :many
SELECT * FROM themes ORDER BY is_default DESC, is_built_in DESC, display_name;

-- name: ListBuiltInThemes :many
SELECT * FROM themes WHERE is_built_in = TRUE ORDER BY display_name;

-- name: UpdateTheme :one
UPDATE themes 
SET display_name = ?, description = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: SetDefaultTheme :exec
UPDATE themes SET is_default = (id = ?);

-- name: DeleteTheme :exec
DELETE FROM themes WHERE id = ? AND is_built_in = FALSE;

-- name: CreateThemeColor :one
INSERT INTO theme_colors (theme_id, color_key, color_value, description)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetThemeColors :many
SELECT * FROM theme_colors WHERE theme_id = ? ORDER BY color_key;

-- name: GetThemeColor :one
SELECT * FROM theme_colors WHERE theme_id = ? AND color_key = ?;

-- name: UpdateThemeColor :one
UPDATE theme_colors 
SET color_value = ?, description = ?
WHERE theme_id = ? AND color_key = ?
RETURNING *;

-- name: DeleteThemeColor :exec
DELETE FROM theme_colors WHERE theme_id = ? AND color_key = ?;

-- name: DeleteAllThemeColors :exec
DELETE FROM theme_colors WHERE theme_id = ?;

-- name: SetUserTheme :one
INSERT INTO user_theme_preferences (user_id, theme_id)
VALUES (?, ?)
ON CONFLICT(user_id) DO UPDATE SET
    theme_id = excluded.theme_id,
    applied_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetUserTheme :one
SELECT t.*, utp.applied_at
FROM themes t
JOIN user_theme_preferences utp ON t.id = utp.theme_id
WHERE utp.user_id = ?;

-- name: GetUserThemeWithColors :many
SELECT 
    t.id as theme_id,
    t.name as theme_name,
    t.display_name,
    t.description as theme_description,
    t.is_built_in,
    t.is_default,
    tc.color_key,
    tc.color_value,
    tc.description as color_description,
    utp.applied_at
FROM themes t
JOIN user_theme_preferences utp ON t.id = utp.theme_id
LEFT JOIN theme_colors tc ON t.id = tc.theme_id
WHERE utp.user_id = ?
ORDER BY tc.color_key;

-- name: GetDefaultTheme :one
SELECT * FROM themes WHERE is_default = TRUE LIMIT 1;

-- name: GetDefaultThemeWithColors :many
SELECT 
    t.id as theme_id,
    t.name as theme_name,
    t.display_name,
    t.description as theme_description,
    t.is_built_in,
    t.is_default,
    tc.color_key,
    tc.color_value,
    tc.description as color_description
FROM themes t
LEFT JOIN theme_colors tc ON t.id = tc.theme_id
WHERE t.is_default = TRUE
ORDER BY tc.color_key;