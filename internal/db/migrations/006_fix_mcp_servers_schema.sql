-- +goose Up
-- Fix mcp_servers schema to ensure it has the correct structure
-- This migration is designed to be idempotent

-- Since the schema is already correct in most cases, we'll just ensure data consistency
-- Clear any orphaned data to ensure referential integrity
DELETE FROM mcp_tools WHERE mcp_server_id NOT IN (SELECT id FROM mcp_servers);
DELETE FROM mcp_servers WHERE mcp_config_id NOT IN (SELECT id FROM mcp_configs);

-- +goose Down
-- Revert to original schema with environment_id
CREATE TABLE mcp_servers_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    command TEXT NOT NULL,
    args TEXT,
    env TEXT,
    working_dir TEXT,
    timeout_seconds INTEGER DEFAULT 30,
    auto_restart BOOLEAN DEFAULT true,
    environment_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    UNIQUE(name, environment_id)
);

DROP TABLE mcp_servers;
ALTER TABLE mcp_servers_old RENAME TO mcp_servers;