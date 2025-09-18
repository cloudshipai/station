-- +goose Up
-- Add output schema support to agents table
ALTER TABLE agents ADD COLUMN output_schema TEXT DEFAULT NULL;
ALTER TABLE agents ADD COLUMN output_schema_preset TEXT DEFAULT NULL;

-- +goose Down
-- Remove output schema columns
ALTER TABLE agents DROP COLUMN output_schema_preset;
ALTER TABLE agents DROP COLUMN output_schema;