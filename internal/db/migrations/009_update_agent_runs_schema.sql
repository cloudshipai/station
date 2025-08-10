-- +goose Up
-- Update agent_runs table to match expected schema

-- Create new agent_runs table with correct schema
CREATE TABLE agent_runs_new (
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
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Migrate existing data if any
INSERT INTO agent_runs_new (
    agent_id,
    user_id,
    task,
    final_response,
    steps_taken,
    status,
    started_at,
    completed_at
)
SELECT 
    agent_id,
    1 as user_id, -- Default to user ID 1 (will be console user)
    COALESCE(input, 'Migrated execution') as task,
    COALESCE(output, '') as final_response,
    COALESCE(steps_taken, 0) as steps_taken,
    status,
    started_at,
    completed_at
FROM agent_runs;

-- Drop old table and rename new one
DROP TABLE agent_runs;
ALTER TABLE agent_runs_new RENAME TO agent_runs;

-- Create index for efficient queries
CREATE INDEX idx_agent_runs_agent_id ON agent_runs(agent_id);
CREATE INDEX idx_agent_runs_user_id ON agent_runs(user_id);
CREATE INDEX idx_agent_runs_started_at ON agent_runs(started_at);

-- +goose Down
-- Revert to original schema
CREATE TABLE agent_runs_old (
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

-- Migrate data back
INSERT INTO agent_runs_old (
    agent_id,
    input,
    output,
    status,
    steps_taken,
    started_at,
    completed_at
)
SELECT 
    agent_id,
    task as input,
    final_response as output,
    status,
    steps_taken,
    started_at,
    completed_at
FROM agent_runs;

DROP TABLE agent_runs;
ALTER TABLE agent_runs_old RENAME TO agent_runs;