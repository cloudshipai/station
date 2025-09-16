-- Migration 023: Add output schema and preset columns to agents table
-- This allows agents to use schema presets (like finops) and store structured output schemas

-- Add output_schema column for JSON schema storage
ALTER TABLE agents ADD COLUMN output_schema TEXT;

-- Add output_schema_preset column for predefined schema types (finops, etc)
ALTER TABLE agents ADD COLUMN output_schema_preset TEXT;

-- Create index on preset column for fast lookups
CREATE INDEX idx_agents_output_schema_preset ON agents(output_schema_preset) WHERE output_schema_preset IS NOT NULL;