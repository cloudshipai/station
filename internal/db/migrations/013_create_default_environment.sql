-- +goose Up
-- Create a default environment for new Station installations

-- Insert default environment if no environments exist
-- Use the console user as the creator
INSERT OR IGNORE INTO environments (name, description, created_by, created_at, updated_at)
SELECT 'default', 
       'Default environment for development and testing',
       (SELECT id FROM users WHERE username = 'console' LIMIT 1),
       datetime('now'),
       datetime('now')
WHERE NOT EXISTS (SELECT 1 FROM environments);

-- +goose Down
-- Remove default environment (only if it's the only one and named 'default')
DELETE FROM environments 
WHERE name = 'default' 
  AND (SELECT COUNT(*) FROM environments) = 1;