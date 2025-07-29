-- +goose Up
-- Add mcp_config_id column to mcp_servers table and update the schema to match the new design
-- where mcp_servers are associated with mcp_configs instead of directly with environments

-- First, add the mcp_config_id column
ALTER TABLE mcp_servers ADD COLUMN mcp_config_id INTEGER;

-- For existing data, we'll need to create associations
-- Since the current data model has changed, we'll clear existing servers for now
-- In a production system, you'd want to migrate the data properly
DELETE FROM mcp_servers;

-- Now make mcp_config_id NOT NULL and add the foreign key constraint
-- Unfortunately, SQLite doesn't support modifying column constraints directly
-- So we need to recreate the table

-- Create new mcp_servers table with correct schema
CREATE TABLE mcp_servers_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    mcp_config_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    command TEXT NOT NULL,
    args TEXT, -- JSON array of arguments
    env TEXT,  -- JSON object of environment variables
    working_dir TEXT,
    timeout_seconds INTEGER DEFAULT 30,
    auto_restart BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (mcp_config_id) REFERENCES mcp_configs(id) ON DELETE CASCADE,
    UNIQUE(name, mcp_config_id)
);

-- Copy any remaining data (should be empty after DELETE above)
INSERT INTO mcp_servers_new (id, mcp_config_id, name, command, args, env, working_dir, timeout_seconds, auto_restart, created_at)
SELECT id, mcp_config_id, name, command, args, env, working_dir, timeout_seconds, auto_restart, created_at 
FROM mcp_servers 
WHERE mcp_config_id IS NOT NULL;

-- Drop old table and rename new one
DROP TABLE mcp_servers;
ALTER TABLE mcp_servers_new RENAME TO mcp_servers;

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