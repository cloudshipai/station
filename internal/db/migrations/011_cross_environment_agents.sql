-- +goose Up
-- Add support for cross-environment agents

-- Step 1: Create agent_environments junction table for many-to-many relationship
CREATE TABLE agent_environments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id INTEGER NOT NULL,
    environment_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    UNIQUE(agent_id, environment_id)
);

-- Step 2: Create new agent_tools table with environment context and tool_name instead of tool_id
CREATE TABLE agent_tools_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id INTEGER NOT NULL,
    tool_name TEXT NOT NULL,
    environment_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    UNIQUE(agent_id, tool_name, environment_id)
);

-- Step 3: Migrate existing data from old agent_tools to new structure
-- For existing agents, create agent_environments entries based on their current environment
INSERT INTO agent_environments (agent_id, environment_id, created_at)
SELECT DISTINCT id, environment_id, created_at
FROM agents;

-- Migrate existing agent_tools data to new structure with tool names
INSERT INTO agent_tools_new (agent_id, tool_name, environment_id, created_at)
SELECT 
    at.agent_id,
    t.name as tool_name,
    a.environment_id,
    at.created_at
FROM agent_tools at
JOIN mcp_tools t ON at.tool_id = t.id
JOIN agents a ON at.agent_id = a.id;

-- Step 4: Replace old agent_tools table with new one
DROP TABLE agent_tools;
ALTER TABLE agent_tools_new RENAME TO agent_tools;

-- Step 5: Remove environment_id from agents table since agents can now belong to multiple environments
-- But keep it for backward compatibility for now - we'll handle this in the application layer
-- CREATE TABLE agents_new (
--     id INTEGER PRIMARY KEY AUTOINCREMENT,
--     name TEXT NOT NULL,
--     description TEXT,
--     prompt TEXT NOT NULL,
--     max_steps INTEGER DEFAULT 12,
--     created_by INTEGER NOT NULL,
--     created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
--     updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
--     FOREIGN KEY (created_by) REFERENCES users(id)
-- );

-- For now, we'll keep the environment_id in agents table for backward compatibility
-- The new agent_environments table will be the source of truth for multi-environment access

-- +goose Down
-- Revert cross-environment agents changes

-- Step 1: Recreate original agent_tools table
CREATE TABLE agent_tools_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id INTEGER NOT NULL,
    tool_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (tool_id) REFERENCES mcp_tools(id) ON DELETE CASCADE,
    UNIQUE(agent_id, tool_id)
);

-- Step 2: Migrate data back (this will lose environment context)
-- Note: This is a lossy migration - tools from multiple environments will be lost
INSERT INTO agent_tools_old (agent_id, tool_id, created_at)
SELECT DISTINCT
    at.agent_id,
    t.id as tool_id,
    at.created_at
FROM agent_tools at
JOIN mcp_tools t ON at.tool_name = t.name
-- Only migrate tools from the agent's primary environment
JOIN agents a ON at.agent_id = a.id AND at.environment_id = a.environment_id
LIMIT 1; -- In case of duplicates, take the first one

-- Step 3: Replace tables
DROP TABLE agent_tools;
ALTER TABLE agent_tools_old RENAME TO agent_tools;

-- Step 4: Drop agent_environments table
DROP TABLE agent_environments;