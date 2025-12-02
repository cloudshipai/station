-- +goose Up
-- Migration: Add memory topic support to agents
-- This enables CloudShip memory integration for agents

-- Add memory_topic_key column to agents table
ALTER TABLE agents ADD COLUMN memory_topic_key TEXT;

-- Add memory_max_tokens column with default value
ALTER TABLE agents ADD COLUMN memory_max_tokens INTEGER DEFAULT 2000;

-- Add index for faster lookups by memory topic
CREATE INDEX IF NOT EXISTS idx_agents_memory_topic_key ON agents(memory_topic_key) WHERE memory_topic_key IS NOT NULL;

-- +goose Down
-- Remove memory topic columns
DROP INDEX IF EXISTS idx_agents_memory_topic_key;
ALTER TABLE agents DROP COLUMN IF EXISTS memory_max_tokens;
ALTER TABLE agents DROP COLUMN IF EXISTS memory_topic_key;
