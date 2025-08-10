-- +goose Up
-- Users table
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    public_key TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Environments table for isolated agent environments
CREATE TABLE environments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_by INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES users(id)
);

-- MCP server configurations
CREATE TABLE mcp_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    command TEXT NOT NULL,
    args TEXT, -- JSON array of arguments
    env TEXT,  -- JSON object of environment variables
    working_dir TEXT,
    timeout_seconds INTEGER DEFAULT 30,
    auto_restart BOOLEAN DEFAULT true,
    environment_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    UNIQUE(name, environment_id)
);

-- MCP configurations (encrypted)
CREATE TABLE mcp_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    mcp_server_id INTEGER NOT NULL,
    config_data TEXT NOT NULL, -- Encrypted configuration data
    key_version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (mcp_server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
);

-- Tools discovered from MCP servers
CREATE TABLE mcp_tools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    mcp_server_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    input_schema TEXT, -- JSON schema for tool inputs
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (mcp_server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE,
    UNIQUE(name, mcp_server_id)
);

-- AI Agents
CREATE TABLE agents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT,
    prompt TEXT NOT NULL, -- System prompt
    max_steps INTEGER DEFAULT 12,
    environment_id INTEGER NOT NULL,
    created_by INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES users(id),
    UNIQUE(name, environment_id)
);

-- Agent-Tool relationships (many-to-many)
CREATE TABLE agent_tools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id INTEGER NOT NULL,
    tool_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (tool_id) REFERENCES mcp_tools(id) ON DELETE CASCADE,
    UNIQUE(agent_id, tool_id)
);

-- Agent execution runs
CREATE TABLE agent_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id INTEGER NOT NULL,
    input TEXT NOT NULL,
    output TEXT,
    status TEXT NOT NULL CHECK (status IN ('running', 'completed', 'failed', 'timeout')),
    steps_taken INTEGER DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS agent_runs;
DROP TABLE IF EXISTS agent_tools;
DROP TABLE IF EXISTS agents;
DROP TABLE IF EXISTS mcp_tools;
DROP TABLE IF EXISTS mcp_configs;
DROP TABLE IF EXISTS mcp_servers;
DROP TABLE IF EXISTS environments;
DROP TABLE IF EXISTS users;