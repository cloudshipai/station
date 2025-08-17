-- +goose Up
-- Add input_schema field to agents table for custom input variable definitions

ALTER TABLE agents ADD COLUMN input_schema TEXT DEFAULT NULL;

-- +goose Down  
-- Remove input_schema field from agents table

ALTER TABLE agents DROP COLUMN input_schema;