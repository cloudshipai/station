-- +goose Up
-- Add app and app_subtype fields to agents table for CloudShip data ingestion routing
ALTER TABLE agents ADD COLUMN app TEXT;
ALTER TABLE agents ADD COLUMN app_subtype TEXT CHECK (
    app_subtype IS NULL OR
    app_subtype IN ('investigations', 'opportunities', 'projections', 'inventory', 'events')
);

-- Create index for faster queries by app/app_subtype combination
CREATE INDEX IF NOT EXISTS idx_agents_app_subtype ON agents(app, app_subtype);

-- +goose Down
DROP INDEX IF EXISTS idx_agents_app_subtype;
ALTER TABLE agents DROP COLUMN app_subtype;
ALTER TABLE agents DROP COLUMN app;
