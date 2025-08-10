-- +goose Up
-- Create a default admin console user for SSH sessions and system operations

-- Insert admin console user if it doesn't exist (let SQLite auto-assign ID)
INSERT OR IGNORE INTO users (username, public_key, is_admin, created_at, updated_at)
SELECT 'console', 
       'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIC4PNZ7eP0Tf8O4VUW4W4zQGv7QmjVbz8YNLH9+cAB5d console@station',
       true,
       datetime('now'),
       datetime('now')
WHERE NOT EXISTS (SELECT 1 FROM users WHERE username = 'console');

-- +goose Down
-- Remove console user
DELETE FROM users WHERE username = 'console';