-- +goose Up
-- Add schedule_variables field to agents table for storing cron execution variables
ALTER TABLE agents ADD COLUMN schedule_variables TEXT DEFAULT NULL;

-- +goose Down
-- Remove schedule_variables field from agents table
ALTER TABLE agents DROP COLUMN schedule_variables;
