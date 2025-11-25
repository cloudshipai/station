-- Migration: Add session_id to faker_tool_cache for consistency tracking
-- This allows faker to reuse the same session across multiple subprocess invocations

-- +goose Up
ALTER TABLE faker_tool_cache ADD COLUMN session_id TEXT;

CREATE INDEX IF NOT EXISTS idx_faker_tool_cache_session_id ON faker_tool_cache(session_id);

-- +goose Down
DROP INDEX IF EXISTS idx_faker_tool_cache_session_id;
ALTER TABLE faker_tool_cache DROP COLUMN session_id;
