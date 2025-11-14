-- Migration: Add faker tool cache
-- This table stores AI-generated tools for standalone fakers
-- Tools are cached per faker session ID to ensure consistency across restarts

-- +goose Up
CREATE TABLE IF NOT EXISTS faker_tool_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    faker_id TEXT NOT NULL,          -- Unique identifier for this faker instance (e.g., "aws-cloudwatch-faker")
    tool_name TEXT NOT NULL,         -- Name of the tool
    tool_schema TEXT NOT NULL,       -- JSON schema for the tool (description, inputSchema, etc.)
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(faker_id, tool_name)
);

CREATE INDEX IF NOT EXISTS idx_faker_tool_cache_faker_id ON faker_tool_cache(faker_id);

-- +goose Down
DROP TABLE IF NOT EXISTS faker_tool_cache;
