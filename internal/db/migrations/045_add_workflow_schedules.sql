-- +goose Up
-- Add workflow_schedules table for cron-triggered workflow execution
CREATE TABLE IF NOT EXISTS workflow_schedules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_id TEXT NOT NULL,
    workflow_version INTEGER NOT NULL,
    cron_expression TEXT NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    enabled INTEGER NOT NULL DEFAULT 1,
    input TEXT,
    last_run_at DATETIME,
    next_run_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(workflow_id, workflow_version),
    FOREIGN KEY (workflow_id) REFERENCES workflows(workflow_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_workflow_schedules_enabled ON workflow_schedules(enabled);
CREATE INDEX IF NOT EXISTS idx_workflow_schedules_next_run ON workflow_schedules(next_run_at);

-- +goose Down
DROP INDEX IF EXISTS idx_workflow_schedules_next_run;
DROP INDEX IF EXISTS idx_workflow_schedules_enabled;
DROP TABLE IF EXISTS workflow_schedules;
