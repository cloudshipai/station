-- Users table
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    is_admin BOOLEAN NOT NULL DEFAULT FALSE,
    api_key TEXT UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Environments table for isolated agent environments
CREATE TABLE environments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- MCP configurations (encrypted)
CREATE TABLE mcp_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    config_json TEXT NOT NULL, -- encrypted JSON blob
    encryption_key_id TEXT NOT NULL, -- reference to encryption key used
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments (id),
    UNIQUE (environment_id, version)
);

-- MCP servers
CREATE TABLE mcp_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    server_name TEXT NOT NULL,
    server_url TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (config_id) REFERENCES mcp_configs (id),
    UNIQUE (config_id, server_name)
);

-- Tools discovered from MCP servers
CREATE TABLE mcp_tools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    tool_name TEXT NOT NULL,
    tool_description TEXT,
    tool_schema TEXT, -- JSON schema for the tool
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (server_id) REFERENCES mcp_servers (id),
    UNIQUE (server_id, tool_name)
);

-- Model providers table
CREATE TABLE model_providers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE, -- e.g., "openai", "anthropic", "ollama"
    display_name TEXT NOT NULL, -- e.g., "OpenAI", "Anthropic", "Ollama"
    base_url TEXT NOT NULL,
    api_key TEXT, -- Can be NULL for providers like Ollama
    headers TEXT, -- JSON object of custom headers
    enabled BOOLEAN DEFAULT true,
    is_default BOOLEAN DEFAULT false,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Models table
CREATE TABLE models (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id INTEGER NOT NULL,
    model_id TEXT NOT NULL, -- e.g., "gpt-4o", "claude-3-5-sonnet-20241022"
    name TEXT NOT NULL, -- Display name e.g., "GPT-4 Omni", "Claude 3.5 Sonnet"
    context_size INTEGER NOT NULL,
    max_tokens INTEGER NOT NULL,
    supports_tools BOOLEAN DEFAULT false,
    input_cost REAL DEFAULT 0.0, -- Cost per 1M tokens
    output_cost REAL DEFAULT 0.0, -- Cost per 1M tokens
    enabled BOOLEAN DEFAULT true,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (provider_id) REFERENCES model_providers(id) ON DELETE CASCADE,
    UNIQUE(provider_id, model_id)
);

-- AI Agents
CREATE TABLE agents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    prompt TEXT NOT NULL, -- system prompt/config for the agent
    max_steps INTEGER NOT NULL DEFAULT 5,
    environment_id INTEGER NOT NULL,
    created_by INTEGER NOT NULL,
    model_id INTEGER REFERENCES models(id),
    cron_schedule TEXT DEFAULT NULL, -- Cron expression for scheduling
    is_scheduled BOOLEAN DEFAULT FALSE,
    last_scheduled_run DATETIME DEFAULT NULL,
    next_scheduled_run DATETIME DEFAULT NULL,
    schedule_enabled BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments (id),
    FOREIGN KEY (created_by) REFERENCES users (id)
);

-- Agent-Tool relationships (many-to-many)
CREATE TABLE agent_tools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id INTEGER NOT NULL,
    tool_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (agent_id) REFERENCES agents (id),
    FOREIGN KEY (tool_id) REFERENCES mcp_tools (id),
    UNIQUE (agent_id, tool_id)
);

-- Agent execution runs
CREATE TABLE agent_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    task TEXT NOT NULL, -- the input task given to the agent
    final_response TEXT NOT NULL, -- the agent's final response
    steps_taken INTEGER NOT NULL DEFAULT 0,
    tool_calls TEXT, -- JSON array of tool calls made during execution
    execution_steps TEXT, -- JSON array of execution steps
    status TEXT NOT NULL DEFAULT 'completed', -- completed, failed, timeout
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    FOREIGN KEY (agent_id) REFERENCES agents (id),
    FOREIGN KEY (user_id) REFERENCES users (id)
);