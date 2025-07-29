-- +goose Up
-- Fix the unique constraint on mcp_configs to allow multiple configs with same version but different names

-- Create new table with correct constraint
CREATE TABLE mcp_configs_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL,
    config_name TEXT NOT NULL DEFAULT 'default',
    version INTEGER NOT NULL DEFAULT 1,
    config_json TEXT NOT NULL, -- Raw JSON configuration
    encryption_key_id TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    UNIQUE(environment_id, config_name, version)
);

-- Copy existing data
INSERT INTO mcp_configs_new (id, environment_id, config_name, version, config_json, encryption_key_id, created_at, updated_at)
SELECT id, environment_id, config_name, version, config_json, encryption_key_id, created_at, updated_at 
FROM mcp_configs;

-- Drop old table and rename new one
DROP TABLE mcp_configs;
ALTER TABLE mcp_configs_new RENAME TO mcp_configs;

-- +goose Down
-- Revert to original constraint
CREATE TABLE mcp_configs_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    config_json TEXT NOT NULL,
    encryption_key_id TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    UNIQUE(environment_id, version)
);

INSERT INTO mcp_configs_old (id, environment_id, version, config_json, encryption_key_id, created_at, updated_at)
SELECT id, environment_id, version, config_json, encryption_key_id, created_at, updated_at 
FROM mcp_configs;

DROP TABLE mcp_configs;
ALTER TABLE mcp_configs_old RENAME TO mcp_configs;