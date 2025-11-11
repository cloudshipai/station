-- Migration: Add Reports System
-- Description: Environment-wide report generation with LLM evaluation
-- Date: 2025-01-11

-- Reports table: Main report metadata and results
CREATE TABLE IF NOT EXISTS reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT,
    environment_id INTEGER NOT NULL,
    
    -- Criteria (stored as JSON)
    team_criteria TEXT NOT NULL,           -- JSON: {goal, criteria: {name: {weight, description, threshold}}}
    agent_criteria TEXT,                   -- JSON: {agent_id: {criteria: {...}}}
    
    -- Generation status
    status TEXT NOT NULL DEFAULT 'pending', -- 'pending', 'generating_team', 'generating_agents', 'completed', 'failed'
    progress INTEGER DEFAULT 0,             -- 0-100
    current_step TEXT,                      -- Human-readable current step (e.g., "Evaluating agent 3/14")
    
    -- Team-level results
    executive_summary TEXT,                 -- High-level overview (2-3 paragraphs)
    team_score REAL,                        -- 0-10 overall environment score
    team_reasoning TEXT,                    -- Why that team score?
    team_criteria_scores TEXT,              -- JSON: {criterion: {score, reasoning}}
    
    -- Agent-specific results summary (for quick access)
    agent_reports TEXT,                     -- JSON: {agent_id: {score, summary}}
    
    -- Report metadata
    total_runs_analyzed INTEGER DEFAULT 0,
    total_agents_analyzed INTEGER DEFAULT 0,
    generation_duration_seconds REAL,
    generation_started_at TIMESTAMP,
    generation_completed_at TIMESTAMP,
    
    -- LLM usage tracking
    total_llm_tokens INTEGER DEFAULT 0,
    total_llm_cost REAL DEFAULT 0.0,
    judge_model TEXT DEFAULT 'gpt-4o-mini', -- Which LLM used as judge
    
    -- Error handling
    error_message TEXT,                     -- Error if generation failed
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_reports_environment ON reports(environment_id);
CREATE INDEX IF NOT EXISTS idx_reports_status ON reports(status);
CREATE INDEX IF NOT EXISTS idx_reports_created_at ON reports(created_at DESC);

-- Agent report details table: Detailed per-agent analysis
CREATE TABLE IF NOT EXISTS agent_report_details (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id INTEGER NOT NULL,
    agent_id INTEGER NOT NULL,
    agent_name TEXT NOT NULL,
    
    -- Evaluation results
    score REAL NOT NULL,                    -- 0-10 score for this agent
    passed BOOLEAN NOT NULL DEFAULT 0,      -- Did agent meet thresholds?
    reasoning TEXT,                         -- Overall reasoning for score
    
    -- Criteria breakdown (stored as JSON)
    criteria_scores TEXT,                   -- JSON: {criterion: {score, reasoning, examples}}
    
    -- Run analysis
    runs_analyzed INTEGER DEFAULT 0,        -- How many runs examined
    run_ids TEXT,                           -- Comma-separated run IDs
    
    -- Performance metrics (calculated from runs)
    avg_duration_seconds REAL,
    avg_tokens INTEGER,
    avg_cost REAL,
    success_rate REAL,                      -- Percentage of successful runs (0-1)
    
    -- Findings (stored as JSON arrays)
    strengths TEXT,                         -- JSON: ["strength 1", "strength 2", ...]
    weaknesses TEXT,                        -- JSON: ["weakness 1", "weakness 2", ...]
    recommendations TEXT,                   -- JSON: ["recommendation 1", ...]
    
    -- Telemetry insights (stored as JSON)
    telemetry_summary TEXT,                 -- JSON: {avg_spans, tool_usage, etc}
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (report_id) REFERENCES reports(id) ON DELETE CASCADE,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_report_details_report ON agent_report_details(report_id);
CREATE INDEX IF NOT EXISTS idx_agent_report_details_agent ON agent_report_details(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_report_details_score ON agent_report_details(score DESC);

-- Trigger to update reports.updated_at
CREATE TRIGGER IF NOT EXISTS update_reports_timestamp 
AFTER UPDATE ON reports
BEGIN
    UPDATE reports SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
