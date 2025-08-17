-- Migration: Add input_schema field to agents table
-- This allows agents to define custom input variables beyond the default userInput

ALTER TABLE agents ADD COLUMN input_schema TEXT DEFAULT NULL;

-- input_schema will contain JSON defining additional input variables
-- Example:
-- {
--   "projectPath": {"type": "string", "description": "Path to the project directory"},
--   "environment": {"type": "string", "enum": ["dev", "staging", "prod"], "description": "Target environment"},
--   "enableDebug": {"type": "boolean", "description": "Enable debug mode"}
-- }
--
-- The default userInput: string is always available and doesn't need to be defined
-- Custom schemas are merged with the default at runtime