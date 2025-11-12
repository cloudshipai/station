-- Migration: Add agent_agents table for agent-to-agent relationships
-- Description: Stores hierarchical relationships where parent agents can call child agents as tools
-- Date: 2025-11-12

CREATE TABLE IF NOT EXISTS agent_agents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_agent_id INTEGER NOT NULL,
    child_agent_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (child_agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    UNIQUE(parent_agent_id, child_agent_id),
    -- Prevent self-referencing (agent cannot call itself)
    CHECK(parent_agent_id != child_agent_id)
);

-- Create index for efficient parent lookups (most common query)
CREATE INDEX IF NOT EXISTS idx_agent_agents_parent ON agent_agents(parent_agent_id);

-- Create index for child lookups (to find which agents use a specific agent)
CREATE INDEX IF NOT EXISTS idx_agent_agents_child ON agent_agents(child_agent_id);
