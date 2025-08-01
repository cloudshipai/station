-- +goose Up
-- Create a default environment for new Station installations

-- Insert default environment if no environments exist
-- Use the system user (id=0) as the creator
INSERT OR IGNORE INTO environments (name, description, created_by, created_at, updated_at)
SELECT 'default', 
       'Default environment for development and testing',
       0,
       datetime('now'),
       datetime('now')
WHERE NOT EXISTS (SELECT 1 FROM environments);

-- +goose Down
-- Remove default environment (only if it's the only one and named 'default')
DELETE FROM environments 
WHERE name = 'default' 
  AND (SELECT COUNT(*) FROM environments) = 1;