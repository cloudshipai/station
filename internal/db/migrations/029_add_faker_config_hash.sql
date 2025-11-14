-- Migration: Add config hash to faker tool cache
-- This ensures same configuration always generates same cache key
-- Hash is computed from: faker_name + ai_instruction + ai_model

-- +goose Up
-- Create faker_tool_cache table if it doesn't exist (idempotent)
CREATE TABLE IF NOT EXISTS faker_tool_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    faker_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    tool_schema TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(faker_id, tool_name)
);

CREATE INDEX IF NOT EXISTS idx_faker_tool_cache_faker_id ON faker_tool_cache(faker_id);

-- Add config_hash column if it doesn't exist
ALTER TABLE faker_tool_cache ADD COLUMN config_hash TEXT;

-- Create index on config_hash for efficient lookups
CREATE INDEX IF NOT EXISTS idx_faker_tool_cache_config_hash ON faker_tool_cache(config_hash);

-- For existing rows, generate config_hash from faker_id (backward compatibility)
-- New rows will have proper config_hash generated from configuration
UPDATE faker_tool_cache SET config_hash = faker_id WHERE config_hash IS NULL;

-- +goose Down
-- Remove config_hash column and index
DROP INDEX IF EXISTS idx_faker_tool_cache_config_hash;

-- Note: SQLite doesn't support DROP COLUMN directly in older versions
-- For complete rollback, you would need to:
-- 1. Create new table without config_hash
-- 2. Copy data
-- 3. Drop old table
-- 4. Rename new table
-- This is omitted for simplicity since downgrades are rare
