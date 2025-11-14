-- +goose Up
-- Add model_filter column to reports table for filtering runs by model
ALTER TABLE reports ADD COLUMN filter_model TEXT DEFAULT NULL;

-- Add index for faster model_name lookups in agent_runs
CREATE INDEX IF NOT EXISTS idx_agent_runs_model_name ON agent_runs(model_name);
CREATE INDEX IF NOT EXISTS idx_agent_runs_agent_model ON agent_runs(agent_id, model_name);

-- +goose Down
DROP INDEX IF EXISTS idx_agent_runs_agent_model;
DROP INDEX IF EXISTS idx_agent_runs_model_name;
ALTER TABLE reports DROP COLUMN filter_model;
