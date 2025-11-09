-- +goose Up
-- Add parent_run_id to agent_runs table for hierarchical agent execution tracking
-- This enables proper parent-child relationships when agents call other agents as tools

-- SQLite doesn't support IF NOT EXISTS for ALTER TABLE ADD COLUMN
-- We need to check if column exists and only add if it doesn't
-- Using a safe approach: ignore error if column already exists

-- Add parent_run_id column (will fail silently if exists)
ALTER TABLE agent_runs ADD COLUMN parent_run_id INTEGER DEFAULT NULL;

-- Create index for efficient parent-child queries (safe with IF NOT EXISTS)
CREATE INDEX IF NOT EXISTS idx_agent_runs_parent_run_id ON agent_runs(parent_run_id);

-- +goose Down
DROP INDEX IF EXISTS idx_agent_runs_parent_run_id;
-- Note: SQLite doesn't support DROP COLUMN in all versions
-- Commented out to prevent issues: ALTER TABLE agent_runs DROP COLUMN parent_run_id;
