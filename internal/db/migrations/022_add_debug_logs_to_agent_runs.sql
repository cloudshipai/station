-- +goose Up
-- Add debug_logs column to agent_runs table for real-time progress tracking
ALTER TABLE agent_runs ADD COLUMN debug_logs TEXT;

-- Add index for efficient querying of runs with debug logs
CREATE INDEX idx_agent_runs_debug_logs ON agent_runs(id) WHERE debug_logs IS NOT NULL;

-- +goose Down
-- Drop the debug_logs column and its index
DROP INDEX IF EXISTS idx_agent_runs_debug_logs;
ALTER TABLE agent_runs DROP COLUMN debug_logs;