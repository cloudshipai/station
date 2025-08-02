-- +goose Up
-- +goose StatementBegin

-- Remove unused file config tables that were part of the original design
-- but are not needed for our simplified file-based approach
DROP TABLE IF EXISTS template_variables;
DROP TABLE IF EXISTS config_migrations;  
DROP TABLE IF EXISTS config_loading_preferences;

-- Also remove old encrypted config tables that are no longer used
DROP TABLE IF EXISTS mcp_configs;
DROP TABLE IF EXISTS mcp_configs_backup;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Note: This rollback recreates the tables but they would be empty
-- since we're removing unused functionality

-- Recreate template_variables table
CREATE TABLE template_variables (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_config_id INTEGER NOT NULL,
    variable_name TEXT NOT NULL,
    variable_type TEXT NOT NULL DEFAULT 'string',
    required BOOLEAN NOT NULL DEFAULT TRUE,
    default_value TEXT,
    description TEXT,
    secret BOOLEAN NOT NULL DEFAULT FALSE,
    validation_rules TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (file_config_id) REFERENCES file_mcp_configs(id) ON DELETE CASCADE,
    UNIQUE(file_config_id, variable_name)
);

-- Recreate config_loading_preferences table
CREATE TABLE config_loading_preferences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL,
    prefer_files BOOLEAN NOT NULL DEFAULT TRUE,
    enable_fallback BOOLEAN NOT NULL DEFAULT TRUE,
    auto_migrate BOOLEAN NOT NULL DEFAULT FALSE,
    validate_on_load BOOLEAN NOT NULL DEFAULT TRUE,
    variable_resolution_strategy TEXT NOT NULL DEFAULT 'template_first',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    UNIQUE(environment_id)
);

-- Recreate config_migrations table
CREATE TABLE config_migrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL,
    config_name TEXT NOT NULL,
    migration_type TEXT NOT NULL,
    source_type TEXT NOT NULL,
    target_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT,
    migrated_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

-- +goose StatementEnd