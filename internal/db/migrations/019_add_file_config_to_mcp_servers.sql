-- +goose Up
-- +goose StatementBegin

-- Add file_config_id to mcp_servers table for proper cascade deletion
ALTER TABLE mcp_servers ADD COLUMN file_config_id INTEGER REFERENCES file_mcp_configs(id) ON DELETE CASCADE;

-- Add index for performance
CREATE INDEX idx_mcp_servers_file_config ON mcp_servers(file_config_id);

-- Update existing mcp_servers to link them to file_mcp_configs based on environment and creation time
-- This is a best-effort migration - in practice, orphaned servers will be cleaned up by the sync process

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove the foreign key constraint and column
DROP INDEX IF EXISTS idx_mcp_servers_file_config;
ALTER TABLE mcp_servers DROP COLUMN file_config_id;

-- +goose StatementEnd