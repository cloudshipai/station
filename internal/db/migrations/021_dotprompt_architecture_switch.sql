-- Migration 021: Switch to dotprompt architecture with name-based primary keys
-- This migration transforms agents from ID-based to name-based addressing
-- and adds support for .prompt file integration

-- 1. Create new agents table with name as primary key
CREATE TABLE agents_new (
    name TEXT NOT NULL,                       -- Primary key: agent name (e.g. "monitoring-agent")
    display_name TEXT NOT NULL,               -- Human-readable name
    description TEXT NOT NULL,
    file_path TEXT,                          -- Path to .prompt file (nullable for backwards compatibility)
    environment_name TEXT NOT NULL,          -- Environment name (replaces environment_id FK)
    
    -- Scheduling (preserved from existing system)
    cron_schedule TEXT,
    is_scheduled BOOLEAN DEFAULT FALSE,
    schedule_enabled BOOLEAN DEFAULT FALSE,
    last_scheduled_run TIMESTAMP,
    next_scheduled_run TIMESTAMP,
    
    -- File integrity and metadata
    checksum_md5 TEXT,                       -- File checksum for change detection
    status TEXT DEFAULT 'active',            -- active, archived, error
    
    -- Audit fields
    created_by TEXT,                         -- User who created the agent
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_executed_at TIMESTAMP,
    execution_count INTEGER DEFAULT 0,
    
    -- Constraints
    PRIMARY KEY (name, environment_name),    -- Composite primary key: unique name per environment
    FOREIGN KEY (environment_name) REFERENCES environments (name) ON DELETE CASCADE,
    
    -- Ensure file_path is unique if provided
    UNIQUE (file_path) WHERE file_path IS NOT NULL
);

-- 2. Create new agent_runs table with name-based references
CREATE TABLE agent_runs_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_name TEXT NOT NULL,
    environment_name TEXT NOT NULL,
    task TEXT NOT NULL,
    status TEXT NOT NULL,
    result TEXT,
    error TEXT,
    
    -- Execution metadata
    duration_ms INTEGER,
    steps_taken INTEGER,
    tools_used INTEGER,
    token_usage TEXT, -- JSON
    tool_calls TEXT,  -- JSON
    execution_steps TEXT, -- JSON
    response_metadata TEXT, -- JSON
    
    -- Audit
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Foreign key to new agents table
    FOREIGN KEY (agent_name, environment_name) REFERENCES agents_new (name, environment_name) ON DELETE CASCADE
);

-- 3. Create new agent_tools table with name-based references
CREATE TABLE agent_tools_new (
    agent_name TEXT NOT NULL,
    environment_name TEXT NOT NULL,
    mcp_config_name TEXT NOT NULL,         -- MCP config providing the tool
    tool_name TEXT NOT NULL,
    assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    PRIMARY KEY (agent_name, environment_name, tool_name),
    FOREIGN KEY (agent_name, environment_name) REFERENCES agents_new (name, environment_name) ON DELETE CASCADE
);

-- 4. Migrate data from old tables to new tables
-- Convert ID-based agents to name-based agents
INSERT INTO agents_new (
    name, display_name, description, environment_name, 
    cron_schedule, is_scheduled, schedule_enabled, 
    last_scheduled_run, next_scheduled_run,
    created_by, created_at, updated_at
)
SELECT 
    a.name,                              -- Use existing name as primary key
    a.name as display_name,              -- Use name as display name initially
    a.description,
    e.name as environment_name,          -- Convert environment_id to environment_name
    a.cron_schedule,
    a.is_scheduled,
    a.schedule_enabled,
    a.last_scheduled_run,
    a.next_scheduled_run,
    a.created_by,
    a.created_at,
    a.updated_at
FROM agents a
JOIN environments e ON a.environment_id = e.id;

-- Migrate agent runs with name-based references
INSERT INTO agent_runs_new (
    agent_name, environment_name, task, status, result, error,
    duration_ms, steps_taken, tools_used, token_usage, tool_calls, execution_steps, response_metadata,
    created_at
)
SELECT 
    a.name as agent_name,
    e.name as environment_name,
    ar.task,
    ar.status,
    ar.result,
    ar.error,
    ar.duration_ms,
    ar.steps_taken,
    ar.tools_used,
    ar.token_usage,
    ar.tool_calls,
    ar.execution_steps,
    ar.response_metadata,
    ar.created_at
FROM agent_runs ar
JOIN agents a ON ar.agent_id = a.id
JOIN environments e ON a.environment_id = e.id;

-- Migrate agent tools with name-based references
-- Note: This assumes we can map tools to MCP configs, simplified for migration
INSERT INTO agent_tools_new (agent_name, environment_name, mcp_config_name, tool_name, assigned_at)
SELECT 
    a.name as agent_name,
    e.name as environment_name,
    'legacy-config' as mcp_config_name,  -- Placeholder for old assignments
    at.tool_name,
    CURRENT_TIMESTAMP as assigned_at
FROM agent_tools at
JOIN agents a ON at.agent_id = a.id
JOIN environments e ON a.environment_id = e.id;

-- 5. Drop old tables and rename new ones
DROP TABLE agent_tools;
DROP TABLE agent_runs;  
DROP TABLE agents;

ALTER TABLE agents_new RENAME TO agents;
ALTER TABLE agent_runs_new RENAME TO agent_runs;
ALTER TABLE agent_tools_new RENAME TO agent_tools;

-- 6. Create indexes for performance
CREATE INDEX idx_agents_environment ON agents (environment_name);
CREATE INDEX idx_agents_status ON agents (status);
CREATE INDEX idx_agents_scheduled ON agents (is_scheduled, schedule_enabled) WHERE is_scheduled = TRUE;
CREATE INDEX idx_agents_file_path ON agents (file_path) WHERE file_path IS NOT NULL;

CREATE INDEX idx_agent_runs_agent ON agent_runs (agent_name, environment_name);
CREATE INDEX idx_agent_runs_created_at ON agent_runs (created_at);
CREATE INDEX idx_agent_runs_status ON agent_runs (status);

CREATE INDEX idx_agent_tools_agent ON agent_tools (agent_name, environment_name);
CREATE INDEX idx_agent_tools_mcp_config ON agent_tools (mcp_config_name);

-- 7. Create trigger to update updated_at timestamp
CREATE TRIGGER update_agents_updated_at 
    AFTER UPDATE ON agents
    FOR EACH ROW
BEGIN
    UPDATE agents SET updated_at = CURRENT_TIMESTAMP WHERE name = NEW.name AND environment_name = NEW.environment_name;
END;

-- Migration complete: Agents now use name-based primary keys and are ready for dotprompt integration