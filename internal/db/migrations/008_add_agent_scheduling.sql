-- +goose Up
-- Add scheduling fields to agents table
ALTER TABLE agents ADD COLUMN cron_schedule TEXT DEFAULT NULL;
ALTER TABLE agents ADD COLUMN is_scheduled BOOLEAN DEFAULT FALSE;
ALTER TABLE agents ADD COLUMN last_scheduled_run TIMESTAMP DEFAULT NULL;
ALTER TABLE agents ADD COLUMN next_scheduled_run TIMESTAMP DEFAULT NULL;
ALTER TABLE agents ADD COLUMN schedule_enabled BOOLEAN DEFAULT FALSE;

-- Add index for efficient scheduled agent queries
CREATE INDEX idx_agents_scheduled ON agents(is_scheduled, schedule_enabled, next_scheduled_run) WHERE is_scheduled = TRUE AND schedule_enabled = TRUE;

-- +goose Down
-- Remove scheduling fields from agents table
DROP INDEX IF EXISTS idx_agents_scheduled;
ALTER TABLE agents DROP COLUMN schedule_enabled;
ALTER TABLE agents DROP COLUMN next_scheduled_run;
ALTER TABLE agents DROP COLUMN last_scheduled_run;
ALTER TABLE agents DROP COLUMN is_scheduled;
ALTER TABLE agents DROP COLUMN cron_schedule;