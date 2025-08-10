-- +goose Up
-- Revert agents to be environment-specific only
-- This eliminates the cross-environment agent complexity

-- Step 1: Create a backup of current agent-tool relationships for agents that have cross-environment tools
-- We'll need to decide which environment an agent should belong to based on their primary environment

-- Step 2: Remove the cross-environment tables
-- Drop agent_environments table - agents should only belong to one environment (agents.environment_id)
DROP TABLE IF EXISTS agent_environments;

-- Step 3: Update agent_tools to remove environment_id and use tool_id references instead
-- First, create the new simplified agent_tools table
CREATE TABLE agent_tools_simple (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id INTEGER NOT NULL,
    tool_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (tool_id) REFERENCES mcp_tools(id) ON DELETE CASCADE,
    UNIQUE(agent_id, tool_id)
);

-- Step 4: Migrate data from current agent_tools to simplified version
-- Only migrate tools that belong to the agent's primary environment
INSERT INTO agent_tools_simple (agent_id, tool_id, created_at)
SELECT DISTINCT
    at.agent_id,
    t.id as tool_id,
    at.created_at
FROM agent_tools at
JOIN mcp_tools t ON at.tool_name = t.name
JOIN mcp_servers s ON t.mcp_server_id = s.id
JOIN agents a ON at.agent_id = a.id
-- Only include tools from the agent's primary environment
WHERE at.environment_id = a.environment_id
AND s.environment_id = a.environment_id;

-- Step 5: Replace the old agent_tools table
DROP TABLE agent_tools;
ALTER TABLE agent_tools_simple RENAME TO agent_tools;

-- Step 6: Update the mcp_tools queries to be environment-aware through the agent's environment
-- No schema changes needed - the queries will be updated in the application layer

-- +goose Down
-- Restore cross-environment agents (reverses this migration)

-- Step 1: Recreate agent_environments table
CREATE TABLE agent_environments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id INTEGER NOT NULL,
    environment_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    UNIQUE(agent_id, environment_id)
);

-- Step 2: Populate agent_environments from agents.environment_id
INSERT INTO agent_environments (agent_id, environment_id, created_at)
SELECT id, environment_id, created_at
FROM agents;

-- Step 3: Create new agent_tools table with environment context
CREATE TABLE agent_tools_cross_env (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id INTEGER NOT NULL,
    tool_name TEXT NOT NULL,
    environment_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    UNIQUE(agent_id, tool_name, environment_id)
);

-- Step 4: Migrate data from simple agent_tools to cross-environment version
INSERT INTO agent_tools_cross_env (agent_id, tool_name, environment_id, created_at)
SELECT 
    at.agent_id,
    t.name as tool_name,
    a.environment_id,
    at.created_at
FROM agent_tools at
JOIN mcp_tools t ON at.tool_id = t.id
JOIN agents a ON at.agent_id = a.id;

-- Step 5: Replace tables
DROP TABLE agent_tools;
ALTER TABLE agent_tools_cross_env RENAME TO agent_tools;