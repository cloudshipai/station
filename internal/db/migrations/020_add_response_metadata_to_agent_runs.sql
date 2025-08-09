-- +goose Up
-- Add response object metadata fields to agent_runs table for token usage and performance tracking

ALTER TABLE agent_runs ADD COLUMN input_tokens INTEGER DEFAULT NULL;
ALTER TABLE agent_runs ADD COLUMN output_tokens INTEGER DEFAULT NULL; 
ALTER TABLE agent_runs ADD COLUMN total_tokens INTEGER DEFAULT NULL;
ALTER TABLE agent_runs ADD COLUMN duration_seconds REAL DEFAULT NULL;
ALTER TABLE agent_runs ADD COLUMN model_name TEXT DEFAULT NULL;
ALTER TABLE agent_runs ADD COLUMN tools_used INTEGER DEFAULT NULL;

-- Create indexes for efficient querying on new metadata fields
CREATE INDEX idx_agent_runs_model_name ON agent_runs(model_name);
CREATE INDEX idx_agent_runs_total_tokens ON agent_runs(total_tokens);
CREATE INDEX idx_agent_runs_duration ON agent_runs(duration_seconds);

-- +goose Down
-- Remove response object metadata fields

DROP INDEX IF EXISTS idx_agent_runs_duration;
DROP INDEX IF EXISTS idx_agent_runs_total_tokens;
DROP INDEX IF EXISTS idx_agent_runs_model_name;

ALTER TABLE agent_runs DROP COLUMN tools_used;
ALTER TABLE agent_runs DROP COLUMN model_name;
ALTER TABLE agent_runs DROP COLUMN duration_seconds;
ALTER TABLE agent_runs DROP COLUMN total_tokens;
ALTER TABLE agent_runs DROP COLUMN output_tokens;
ALTER TABLE agent_runs DROP COLUMN input_tokens;