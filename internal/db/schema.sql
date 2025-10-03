-- Users table
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    public_key TEXT NOT NULL,
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
    created_by INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- MCP configurations (encrypted)
CREATE TABLE mcp_configs (
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

-- MCP servers
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
    file_config_id INTEGER REFERENCES file_mcp_configs(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    UNIQUE(name, environment_id)
);

-- Tools discovered from MCP servers
CREATE TABLE mcp_tools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    mcp_server_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    input_schema TEXT, -- JSON schema for tool inputs
    file_config_id INTEGER REFERENCES file_mcp_configs(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (mcp_server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE,
    UNIQUE(name, mcp_server_id)
);

-- File-based MCP configurations
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
    input_schema TEXT DEFAULT NULL, -- JSON schema for custom input variables
    output_schema TEXT DEFAULT NULL, -- JSON schema for output format
    output_schema_preset TEXT DEFAULT NULL, -- Predefined schema preset (e.g., finops)
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

-- Agent-Tool relationships (many-to-many) - environment-specific via tool reference
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
    user_id INTEGER NOT NULL,
    task TEXT NOT NULL, -- the input task given to the agent
    final_response TEXT NOT NULL, -- the agent's final response
    steps_taken INTEGER NOT NULL DEFAULT 0,
    tool_calls TEXT, -- JSON array of tool calls made during execution
    execution_steps TEXT, -- JSON array of execution steps
    status TEXT NOT NULL DEFAULT 'completed', -- completed, failed, timeout
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    -- Response object metadata from Station's OpenAI plugin
    input_tokens INTEGER DEFAULT NULL,
    output_tokens INTEGER DEFAULT NULL,
    total_tokens INTEGER DEFAULT NULL,
    duration_seconds REAL DEFAULT NULL,
    model_name TEXT DEFAULT NULL,
    tools_used INTEGER DEFAULT NULL,
    debug_logs TEXT, -- JSON array of debug log entries for real-time progress tracking
    error TEXT DEFAULT NULL, -- Error message when execution fails
    FOREIGN KEY (agent_id) REFERENCES agents (id),
    FOREIGN KEY (user_id) REFERENCES users (id)
);

-- Themes for UI customization
CREATE TABLE themes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    description TEXT,
    is_built_in BOOLEAN DEFAULT FALSE,
    is_default BOOLEAN DEFAULT FALSE,
    created_by INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES users(id)
);

-- Theme color definitions
CREATE TABLE theme_colors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    theme_id INTEGER NOT NULL,
    color_key TEXT NOT NULL, -- e.g., 'primary', 'secondary', 'background', etc.
    color_value TEXT NOT NULL, -- hex color code
    description TEXT,
    FOREIGN KEY (theme_id) REFERENCES themes(id) ON DELETE CASCADE,
    UNIQUE(theme_id, color_key)
);

-- User theme preferences
CREATE TABLE user_theme_preferences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    theme_id INTEGER NOT NULL,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (theme_id) REFERENCES themes(id),
    UNIQUE(user_id)
);

-- System settings table
CREATE TABLE settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Webhook endpoints table
CREATE TABLE webhooks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    secret TEXT, -- Optional webhook secret for security
    enabled BOOLEAN DEFAULT TRUE,
    events TEXT NOT NULL DEFAULT 'agent_run_completed', -- JSON array of event types
    headers TEXT, -- JSON object of custom headers
    timeout_seconds INTEGER DEFAULT 30,
    retry_attempts INTEGER DEFAULT 3,
    created_by INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES users(id),
    UNIQUE(name)
);

-- Webhook delivery tracking table
CREATE TABLE webhook_deliveries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    webhook_id INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    payload TEXT NOT NULL, -- JSON payload sent
    status TEXT NOT NULL DEFAULT 'pending', -- pending, success, failed
    http_status_code INTEGER,
    response_body TEXT,
    response_headers TEXT, -- JSON object
    error_message TEXT,
    attempt_count INTEGER DEFAULT 0,
    last_attempt_at DATETIME,
    next_retry_at DATETIME,
    delivered_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (webhook_id) REFERENCES webhooks(id) ON DELETE CASCADE
);