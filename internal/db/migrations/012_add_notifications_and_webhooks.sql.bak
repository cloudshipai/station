-- +goose Up
-- Migration: Add notifications and webhooks system
-- Version: 012
-- Description: Add system settings, webhooks configuration, and webhook delivery tracking

-- System settings table
CREATE TABLE settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Insert default notification setting
INSERT INTO settings (key, value, description) VALUES 
('notifications_enabled', 'true', 'Enable/disable webhook notifications for agent runs');

-- Webhook endpoints table
CREATE TABLE webhooks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    secret TEXT, -- Optional webhook secret for security
    enabled BOOLEAN DEFAULT TRUE,
    events TEXT NOT NULL DEFAULT 'agent_run_completed', -- JSON array of event types
    headers TEXT, -- JSON object of custom headers
    timeout_seconds INTEGER DEFAULT 30,
    retry_attempts INTEGER DEFAULT 3,
    created_by INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES users(id),
    UNIQUE(name)
);

-- Webhook delivery tracking table
CREATE TABLE webhook_deliveries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    webhook_id INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    payload TEXT NOT NULL, -- JSON payload sent
    status TEXT NOT NULL DEFAULT 'pending', -- pending, success, failed
    http_status_code INTEGER,
    response_body TEXT,
    response_headers TEXT, -- JSON object
    error_message TEXT,
    attempt_count INTEGER DEFAULT 0,
    last_attempt_at DATETIME,
    next_retry_at DATETIME,
    delivered_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (webhook_id) REFERENCES webhooks(id) ON DELETE CASCADE
);

-- Index for efficient webhook delivery queries
CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries(status);
CREATE INDEX idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);
CREATE INDEX idx_webhook_deliveries_event_type ON webhook_deliveries(event_type);
CREATE INDEX idx_webhook_deliveries_next_retry ON webhook_deliveries(next_retry_at) WHERE status = 'failed' AND next_retry_at IS NOT NULL;

-- +goose Down
DROP TABLE webhook_deliveries;
DROP TABLE webhooks;
DROP TABLE settings;