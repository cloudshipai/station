-- +goose Up
-- Add updated_at column to environments table
ALTER TABLE environments ADD COLUMN updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- Update mcp_configs table structure to match new design
-- First, backup existing data (if any)
CREATE TABLE mcp_configs_backup AS SELECT * FROM mcp_configs;

-- Drop the old table
DROP TABLE mcp_configs;

-- Create new mcp_configs table with proper structure
CREATE TABLE mcp_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    config_json TEXT NOT NULL, -- Raw JSON configuration
    encryption_key_id TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    UNIQUE(environment_id, version)
);

-- +goose Down
-- Restore original structure
DROP TABLE IF EXISTS mcp_configs;

-- Restore from backup if it exists
CREATE TABLE mcp_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    mcp_server_id INTEGER NOT NULL,
    config_data TEXT NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (mcp_server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
);

-- Restore data from backup
INSERT INTO mcp_configs SELECT * FROM mcp_configs_backup WHERE EXISTS (SELECT 1 FROM mcp_configs_backup);

-- Drop backup table
DROP TABLE IF EXISTS mcp_configs_backup;

-- Remove added columns
ALTER TABLE environments DROP COLUMN IF EXISTS updated_at;