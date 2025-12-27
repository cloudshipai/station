-- +goose Up
-- Migration: 043_add_notification_logs.sql
-- Description: Add notification_logs table for webhook audit trail

CREATE TABLE IF NOT EXISTS notification_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    log_id TEXT NOT NULL UNIQUE,
    approval_id TEXT NOT NULL,
    event_type TEXT NOT NULL,          -- 'webhook_sent', 'webhook_success', 'webhook_failed', 'approval_decided'
    webhook_url TEXT,
    request_payload TEXT,
    response_status INTEGER,
    response_body TEXT,
    error_message TEXT,
    attempt_number INTEGER DEFAULT 1,
    duration_ms INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (approval_id) REFERENCES workflow_approvals(approval_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_notification_logs_approval ON notification_logs(approval_id);
CREATE INDEX IF NOT EXISTS idx_notification_logs_event_type ON notification_logs(event_type);
CREATE INDEX IF NOT EXISTS idx_notification_logs_created ON notification_logs(created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_notification_logs_created;
DROP INDEX IF EXISTS idx_notification_logs_event_type;
DROP INDEX IF EXISTS idx_notification_logs_approval;
DROP TABLE IF EXISTS notification_logs;
