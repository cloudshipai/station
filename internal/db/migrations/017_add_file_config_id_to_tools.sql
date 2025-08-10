-- +goose Up
-- +goose StatementBegin

-- Add file_config_id column to mcp_tools table to link tools to file configs
ALTER TABLE mcp_tools ADD COLUMN file_config_id INTEGER REFERENCES file_mcp_configs(id) ON DELETE SET NULL;

-- Add index for faster lookups of tools by file config
CREATE INDEX idx_mcp_tools_file_config ON mcp_tools(file_config_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop index and column
DROP INDEX IF EXISTS idx_mcp_tools_file_config;
ALTER TABLE mcp_tools DROP COLUMN file_config_id;

-- +goose StatementEnd