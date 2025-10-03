-- +goose Up
-- Add error column to agent_runs table for capturing execution errors
ALTER TABLE agent_runs ADD COLUMN error TEXT DEFAULT NULL;

-- +goose Down
-- Remove error column
ALTER TABLE agent_runs DROP COLUMN error;
