-- +goose Up
-- Create a default console user for SSH sessions and system operations

-- Insert console user if it doesn't exist (let SQLite auto-assign ID)
INSERT OR IGNORE INTO users (username, public_key, is_admin, created_at, updated_at)
SELECT 'console', 
       'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIC4PNZ7eP0Tf8O4VUW4W4zQGv7QmjVbz8YNLH9+cAB5d console@station',
       true,
       datetime('now'),
       datetime('now')
WHERE NOT EXISTS (SELECT 1 FROM users WHERE username = 'console');

-- Create a system user for cron/scheduled operations if we need one
INSERT OR IGNORE INTO users (id, username, public_key, is_admin, created_at, updated_at)
VALUES (
    0,
    'system',
    'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIC4PNZ7eP0Tf8O4VUW4W4zQGv7QmjVbz8YNLH9+cAB5d system@station',
    true,
    datetime('now'),
    datetime('now')
);

-- +goose Down
-- Remove console users
DELETE FROM users WHERE username IN ('console', 'system');