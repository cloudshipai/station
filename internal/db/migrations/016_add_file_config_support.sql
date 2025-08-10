-- +goose Up
-- +goose StatementBegin

-- Add file-based configuration support to environments table
ALTER TABLE environments ADD COLUMN mcp_config_path TEXT;
ALTER TABLE environments ADD COLUMN variables_path TEXT;

-- Track file-based MCP configurations
CREATE TABLE file_mcp_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL,
    config_name TEXT NOT NULL,
    template_path TEXT NOT NULL,
    variables_path TEXT,
    template_specific_vars_path TEXT, -- Path to template-specific variables
    last_loaded_at TIMESTAMP,
    template_hash TEXT, -- SHA256 hash for change detection
    variables_hash TEXT, -- SHA256 hash for change detection
    template_vars_hash TEXT, -- SHA256 hash for template-specific vars
    metadata TEXT, -- JSON metadata about the template
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    UNIQUE(environment_id, config_name)
);

-- Add index for faster lookups
CREATE INDEX idx_file_mcp_configs_env_name ON file_mcp_configs(environment_id, config_name);
CREATE INDEX idx_file_mcp_configs_last_loaded ON file_mcp_configs(last_loaded_at);

-- Track template variables and their metadata
CREATE TABLE template_variables (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_config_id INTEGER NOT NULL,
    variable_name TEXT NOT NULL,
    variable_type TEXT NOT NULL DEFAULT 'string', -- string, number, boolean, array, object
    required BOOLEAN NOT NULL DEFAULT TRUE,
    default_value TEXT, -- JSON representation of default value
    description TEXT,
    secret BOOLEAN NOT NULL DEFAULT FALSE,
    validation_rules TEXT, -- JSON validation rules
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (file_config_id) REFERENCES file_mcp_configs(id) ON DELETE CASCADE,
    UNIQUE(file_config_id, variable_name)
);

-- Add index for variable lookups
CREATE INDEX idx_template_variables_config_name ON template_variables(file_config_id, variable_name);

-- Configuration loading preferences per environment
CREATE TABLE config_loading_preferences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL,
    prefer_files BOOLEAN NOT NULL DEFAULT TRUE,
    enable_fallback BOOLEAN NOT NULL DEFAULT TRUE,
    auto_migrate BOOLEAN NOT NULL DEFAULT FALSE,
    validate_on_load BOOLEAN NOT NULL DEFAULT TRUE,
    variable_resolution_strategy TEXT NOT NULL DEFAULT 'template_first', -- template_first, global_first, namespaced
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    UNIQUE(environment_id)
);

-- Insert default preferences for existing environments
INSERT INTO config_loading_preferences (environment_id, prefer_files, enable_fallback)
SELECT id, TRUE, TRUE FROM environments;

-- Migration tracking table
CREATE TABLE config_migrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL,
    config_name TEXT NOT NULL,
    migration_type TEXT NOT NULL, -- db_to_file, file_to_db
    source_type TEXT NOT NULL, -- database, file
    target_type TEXT NOT NULL, -- database, file
    status TEXT NOT NULL DEFAULT 'pending', -- pending, completed, failed
    error_message TEXT,
    migrated_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

-- Add index for migration tracking
CREATE INDEX idx_config_migrations_env_config ON config_migrations(environment_id, config_name);
CREATE INDEX idx_config_migrations_status ON config_migrations(status);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop tables in reverse order
DROP TABLE IF EXISTS config_migrations;
DROP TABLE IF EXISTS config_loading_preferences;
DROP TABLE IF EXISTS template_variables;
DROP TABLE IF EXISTS file_mcp_configs;

-- Remove columns from environments table
ALTER TABLE environments DROP COLUMN mcp_config_path;
ALTER TABLE environments DROP COLUMN variables_path;

-- +goose StatementEnd