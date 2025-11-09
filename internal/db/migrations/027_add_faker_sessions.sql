-- +goose Up
-- Migration: Add Faker Session Tracking
-- Purpose: Enable stateful faker sessions that maintain consistency across write/read operations
-- This allows fakers to simulate realistic environments across ANY tool (AWS, databases, filesystems, etc.)

-- Faker sessions represent a single faker instance lifecycle
CREATE TABLE IF NOT EXISTS faker_sessions (
    id TEXT PRIMARY KEY,                    -- UUID or timestamp-based unique session ID
    instruction TEXT NOT NULL,              -- Base scenario/story for this faker session
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Faker events track every tool call (write and read) within a session
-- This enables reads to reflect accumulated writes for consistent simulation
CREATE TABLE IF NOT EXISTS faker_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,                -- Name of the MCP tool called
    arguments TEXT NOT NULL,                -- JSON serialized tool arguments
    response TEXT NOT NULL,                 -- JSON serialized tool response
    operation_type TEXT NOT NULL CHECK(operation_type IN ('read', 'write')),
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES faker_sessions(id) ON DELETE CASCADE
);

-- Index for fast session-based event retrieval (ordered by time)
CREATE INDEX IF NOT EXISTS idx_faker_events_session_time
    ON faker_events(session_id, timestamp);

-- Index for operation type filtering
CREATE INDEX IF NOT EXISTS idx_faker_events_operation
    ON faker_events(session_id, operation_type, timestamp);

-- +goose Down
DROP INDEX IF EXISTS idx_faker_events_operation;
DROP INDEX IF NOT EXISTS idx_faker_events_session_time;
DROP TABLE IF EXISTS faker_events;
DROP TABLE IF EXISTS faker_sessions;
