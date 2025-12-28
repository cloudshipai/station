-- +goose Up
-- Add foundational tables for workflow definitions and durable run tracking
CREATE TABLE IF NOT EXISTS workflows (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    version INTEGER NOT NULL,
    definition TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(workflow_id, version)
);

CREATE TABLE IF NOT EXISTS workflow_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL UNIQUE,
    workflow_id TEXT NOT NULL,
    workflow_version INTEGER NOT NULL,
    status TEXT NOT NULL,
    current_step TEXT,
    input TEXT,
    context TEXT,
    result TEXT,
    error TEXT,
    summary TEXT,
    options TEXT,
    last_signal TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_workflow_runs_workflow ON workflow_runs(workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_runs_status ON workflow_runs(status);

CREATE TABLE IF NOT EXISTS workflow_run_steps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL,
    step_id TEXT NOT NULL,
    attempt INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL,
    input TEXT,
    output TEXT,
    error TEXT,
    metadata TEXT,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    UNIQUE(run_id, step_id, attempt),
    FOREIGN KEY (run_id) REFERENCES workflow_runs(run_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_workflow_run_steps_run ON workflow_run_steps(run_id);

-- Audit trail for workflow runs (immutable event log)
CREATE TABLE IF NOT EXISTS workflow_run_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL,
    seq INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    step_id TEXT,
    payload TEXT,
    actor TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(run_id, seq),
    FOREIGN KEY (run_id) REFERENCES workflow_runs(run_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_workflow_run_events_run ON workflow_run_events(run_id);
CREATE INDEX IF NOT EXISTS idx_workflow_run_events_type ON workflow_run_events(event_type);

-- Human approval gates
CREATE TABLE IF NOT EXISTS workflow_approvals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    approval_id TEXT NOT NULL UNIQUE,
    run_id TEXT NOT NULL,
    step_id TEXT NOT NULL,
    message TEXT NOT NULL,
    summary_path TEXT,
    approvers TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    decided_by TEXT,
    decided_at DATETIME,
    decision_reason TEXT,
    timeout_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (run_id) REFERENCES workflow_runs(run_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_workflow_approvals_run ON workflow_approvals(run_id);
CREATE INDEX IF NOT EXISTS idx_workflow_approvals_status ON workflow_approvals(status);

-- +goose Down
DROP TABLE IF EXISTS workflow_approvals;
DROP TABLE IF EXISTS workflow_run_events;
DROP TABLE IF EXISTS workflow_run_steps;
DROP TABLE IF EXISTS workflow_runs;
DROP TABLE IF EXISTS workflows;
