-- +goose Up
-- Add API key support to users table
ALTER TABLE users ADD COLUMN api_key TEXT;
ALTER TABLE users ADD COLUMN is_admin BOOLEAN DEFAULT false;
ALTER TABLE users ADD COLUMN updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- Create unique index for API key (allows NULL values)
CREATE UNIQUE INDEX idx_users_api_key ON users(api_key) WHERE api_key IS NOT NULL;

-- +goose Down
-- Remove API key support
DROP INDEX IF EXISTS idx_users_api_key;
ALTER TABLE users DROP COLUMN IF EXISTS api_key;
ALTER TABLE users DROP COLUMN IF EXISTS is_admin;
ALTER TABLE users DROP COLUMN IF EXISTS updated_at;