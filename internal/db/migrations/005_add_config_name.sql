-- +goose Up
-- Add config_name field to support multiple named configs per environment
ALTER TABLE mcp_configs ADD COLUMN config_name TEXT NOT NULL DEFAULT '';

-- Update unique constraint to include config_name so each named config can have its own version history
DROP INDEX IF EXISTS idx_mcp_configs_env_version;
CREATE UNIQUE INDEX idx_mcp_configs_env_name_version ON mcp_configs(environment_id, config_name, version);

-- +goose Down
-- Remove the unique index
DROP INDEX IF EXISTS idx_mcp_configs_env_name_version;

-- Remove the config_name column
-- Note: SQLite doesn't support DROP COLUMN directly, so we need to recreate the table
CREATE TABLE mcp_configs_backup AS SELECT id, environment_id, version, config_json, encryption_key_id, created_at, updated_at FROM mcp_configs;

DROP TABLE mcp_configs;

CREATE TABLE mcp_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    config_json TEXT NOT NULL,
    encryption_key_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    UNIQUE(environment_id, version)
);

INSERT INTO mcp_configs (id, environment_id, version, config_json, encryption_key_id, created_at, updated_at)
SELECT id, environment_id, version, config_json, encryption_key_id, created_at, updated_at FROM mcp_configs_backup;

DROP TABLE mcp_configs_backup;