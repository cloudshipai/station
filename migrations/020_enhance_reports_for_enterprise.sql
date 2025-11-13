-- Migration: Enhance reports system for enterprise-grade evaluation
-- This enables companies to make data-driven decisions about agent team performance

-- Add detailed run examples and analysis to agent_report_details
ALTER TABLE agent_report_details ADD COLUMN best_run_example TEXT;      -- JSON: {run_id, input, output, tool_calls, duration, why_successful}
ALTER TABLE agent_report_details ADD COLUMN worst_run_example TEXT;     -- JSON: {run_id, input, output, tool_calls, duration, why_failed}
ALTER TABLE agent_report_details ADD COLUMN tool_usage_analysis TEXT;   -- JSON: {tool_name: {count, success_rate, avg_duration}}
ALTER TABLE agent_report_details ADD COLUMN failure_patterns TEXT;      -- JSON: [{pattern, frequency, examples, impact}]
ALTER TABLE agent_report_details ADD COLUMN improvement_plan TEXT;      -- JSON: [{issue, recommendation, priority, expected_impact, concrete_example}]

-- Add team-level comparative analysis to reports table
ALTER TABLE reports ADD COLUMN performance_trends TEXT;                  -- JSON: {agent_id: {score_trend, reliability_trend, efficiency_trend}}
ALTER TABLE reports ADD COLUMN comparative_analysis TEXT;                -- JSON: {best_performers, worst_performers, key_differentiators}
ALTER TABLE reports ADD COLUMN evidence_summary TEXT;                    -- JSON: [{claim, evidence_runs, confidence_level}]

-- Add run snapshot data for faster report regeneration
CREATE TABLE IF NOT EXISTS report_run_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id INTEGER NOT NULL,
    run_id INTEGER NOT NULL,
    agent_id INTEGER NOT NULL,
    
    -- Captured at report generation time
    task TEXT NOT NULL,
    final_response TEXT NOT NULL,
    tool_calls TEXT,                        -- JSON array of tool calls
    execution_steps TEXT,                   -- JSON array of execution steps
    
    -- Metadata
    status TEXT NOT NULL,
    duration_seconds REAL,
    input_tokens INTEGER,
    output_tokens INTEGER,
    total_tokens INTEGER,
    
    -- Why this run was included
    inclusion_reason TEXT,                  -- 'best_performer', 'worst_performer', 'representative', 'failure_case'
    
    -- Timestamps
    run_started_at DATETIME,
    run_completed_at DATETIME,
    snapshot_created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (report_id) REFERENCES reports(id) ON DELETE CASCADE,
    FOREIGN KEY (run_id) REFERENCES agent_runs(id) ON DELETE SET NULL,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

CREATE INDEX idx_report_run_snapshots_report ON report_run_snapshots(report_id);
CREATE INDEX idx_report_run_snapshots_agent ON report_run_snapshots(agent_id);
CREATE INDEX idx_report_run_snapshots_inclusion ON report_run_snapshots(inclusion_reason);

-- Add report comparison table for version-to-version analysis
CREATE TABLE IF NOT EXISTS report_comparisons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    baseline_report_id INTEGER NOT NULL,    -- Old team version
    comparison_report_id INTEGER NOT NULL,  -- New team version
    
    -- Overall comparison
    score_delta REAL,                       -- Positive = improvement
    summary TEXT,                           -- High-level what changed
    
    -- Detailed deltas (JSON)
    agent_score_changes TEXT,               -- JSON: {agent_id: {old_score, new_score, delta, reason}}
    criteria_changes TEXT,                  -- JSON: {criterion: {old_score, new_score, delta, evidence}}
    performance_changes TEXT,               -- JSON: {speed_delta, cost_delta, reliability_delta}
    
    -- Evidence
    evidence_runs TEXT,                     -- JSON: [{claim, before_run, after_run, improvement_proof}]
    
    -- Timestamps
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (baseline_report_id) REFERENCES reports(id) ON DELETE CASCADE,
    FOREIGN KEY (comparison_report_id) REFERENCES reports(id) ON DELETE CASCADE
);

CREATE INDEX idx_report_comparisons_baseline ON report_comparisons(baseline_report_id);
CREATE INDEX idx_report_comparisons_comparison ON report_comparisons(comparison_report_id);

-- Add metadata for deterministic proof requirements
ALTER TABLE reports ADD COLUMN proof_methodology TEXT;                   -- JSON: {evaluation_approach, evidence_standards, confidence_thresholds}
ALTER TABLE reports ADD COLUMN executive_insights TEXT;                  -- JSON: [{insight, supporting_data, business_impact}]
