-- Users table
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    public_key TEXT NOT NULL,
    is_admin BOOLEAN NOT NULL DEFAULT FALSE,
    api_key TEXT UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Environments table for isolated agent environments
CREATE TABLE environments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_by INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- MCP configurations (encrypted)
CREATE TABLE mcp_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL,
    config_name TEXT NOT NULL DEFAULT 'default',
    version INTEGER NOT NULL DEFAULT 1,
    config_json TEXT NOT NULL, -- Raw JSON configuration
    encryption_key_id TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    UNIQUE(environment_id, config_name, version)
);

-- MCP servers
CREATE TABLE mcp_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    command TEXT NOT NULL,
    args TEXT, -- JSON array of arguments
    env TEXT,  -- JSON object of environment variables
    working_dir TEXT,
    timeout_seconds INTEGER DEFAULT 30,
    auto_restart BOOLEAN DEFAULT true,
    environment_id INTEGER NOT NULL,
    file_config_id INTEGER REFERENCES file_mcp_configs(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    UNIQUE(name, environment_id)
);

-- Tools discovered from MCP servers
CREATE TABLE mcp_tools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    mcp_server_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    input_schema TEXT, -- JSON schema for tool inputs
    file_config_id INTEGER REFERENCES file_mcp_configs(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (mcp_server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE,
    UNIQUE(name, mcp_server_id)
);

-- File-based MCP configurations
CREATE TABLE file_mcp_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL,
    config_name TEXT NOT NULL,
    template_path TEXT NOT NULL,
    variables_path TEXT,
    template_specific_vars_path TEXT, -- Path to template-specific variables
    last_loaded_at TIMESTAMP,
    template_hash TEXT, -- SHA256 hash for change detection
    variables_hash TEXT, -- SHA256 hash for change detection
    template_vars_hash TEXT, -- SHA256 hash for template-specific vars
    metadata TEXT, -- JSON metadata about the template
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    UNIQUE(environment_id, config_name)
);

-- Model providers table
CREATE TABLE model_providers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE, -- e.g., "openai", "anthropic", "ollama"
    display_name TEXT NOT NULL, -- e.g., "OpenAI", "Anthropic", "Ollama"
    base_url TEXT NOT NULL,
    api_key TEXT, -- Can be NULL for providers like Ollama
    headers TEXT, -- JSON object of custom headers
    enabled BOOLEAN DEFAULT true,
    is_default BOOLEAN DEFAULT false,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Models table
CREATE TABLE models (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id INTEGER NOT NULL,
    model_id TEXT NOT NULL, -- e.g., "gpt-4o", "claude-3-5-sonnet-20241022"
    name TEXT NOT NULL, -- Display name e.g., "GPT-4 Omni", "Claude 3.5 Sonnet"
    context_size INTEGER NOT NULL,
    max_tokens INTEGER NOT NULL,
    supports_tools BOOLEAN DEFAULT false,
    input_cost REAL DEFAULT 0.0, -- Cost per 1M tokens
    output_cost REAL DEFAULT 0.0, -- Cost per 1M tokens
    enabled BOOLEAN DEFAULT true,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (provider_id) REFERENCES model_providers(id) ON DELETE CASCADE,
    UNIQUE(provider_id, model_id)
);

-- AI Agents
CREATE TABLE agents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    prompt TEXT NOT NULL, -- system prompt/config for the agent
    max_steps INTEGER NOT NULL DEFAULT 5,
    environment_id INTEGER NOT NULL,
    created_by INTEGER NOT NULL,
    model_id INTEGER REFERENCES models(id),
    input_schema TEXT DEFAULT NULL, -- JSON schema for custom input variables
    output_schema TEXT DEFAULT NULL, -- JSON schema for output format
    output_schema_preset TEXT DEFAULT NULL, -- Predefined schema preset (e.g., finops)
    app TEXT, -- CloudShip app classification (e.g., finops, security, deployments)
    app_subtype TEXT CHECK (app_subtype IS NULL OR app_subtype IN ('investigations', 'opportunities', 'projections', 'inventory', 'events')), -- CloudShip app_subtype classification
    cron_schedule TEXT DEFAULT NULL, -- Cron expression for scheduling
    is_scheduled BOOLEAN DEFAULT FALSE,
    last_scheduled_run DATETIME DEFAULT NULL,
    next_scheduled_run DATETIME DEFAULT NULL,
    schedule_enabled BOOLEAN DEFAULT FALSE,
    schedule_variables TEXT DEFAULT NULL, -- JSON object of variables for scheduled execution
    -- CloudShip Memory Integration
    memory_topic_key TEXT DEFAULT NULL, -- Memory topic key for context injection (e.g., sre-prod-core)
    memory_max_tokens INTEGER DEFAULT 2000, -- Max tokens for memory context injection
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments (id),
    FOREIGN KEY (created_by) REFERENCES users (id)
);

-- Agent-Tool relationships (many-to-many) - environment-specific via tool reference
CREATE TABLE agent_tools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id INTEGER NOT NULL,
    tool_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (tool_id) REFERENCES mcp_tools(id) ON DELETE CASCADE,
    UNIQUE(agent_id, tool_id)
);

-- Agent to agent relationships (hierarchical agent orchestration)
CREATE TABLE agent_agents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_agent_id INTEGER NOT NULL,
    child_agent_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (child_agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    UNIQUE(parent_agent_id, child_agent_id),
    CHECK(parent_agent_id != child_agent_id)
);

CREATE INDEX idx_agent_agents_parent ON agent_agents(parent_agent_id);
CREATE INDEX idx_agent_agents_child ON agent_agents(child_agent_id);

-- Agent execution runs
CREATE TABLE agent_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    task TEXT NOT NULL, -- the input task given to the agent
    final_response TEXT NOT NULL, -- the agent's final response
    steps_taken INTEGER NOT NULL DEFAULT 0,
    tool_calls TEXT, -- JSON array of tool calls made during execution
    execution_steps TEXT, -- JSON array of execution steps
    status TEXT NOT NULL DEFAULT 'completed', -- completed, failed, timeout
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    -- Response object metadata from Station's OpenAI plugin
    input_tokens INTEGER DEFAULT NULL,
    output_tokens INTEGER DEFAULT NULL,
    total_tokens INTEGER DEFAULT NULL,
    duration_seconds REAL DEFAULT NULL,
    model_name TEXT DEFAULT NULL,
    tools_used INTEGER DEFAULT NULL,
    debug_logs TEXT, -- JSON array of debug log entries for real-time progress tracking
    error TEXT DEFAULT NULL, -- Error message when execution fails
    parent_run_id INTEGER DEFAULT NULL, -- Track parent run for hierarchical agent execution
    FOREIGN KEY (agent_id) REFERENCES agents (id),
    FOREIGN KEY (user_id) REFERENCES users (id)
);

-- Themes for UI customization
CREATE TABLE themes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    description TEXT,
    is_built_in BOOLEAN DEFAULT FALSE,
    is_default BOOLEAN DEFAULT FALSE,
    created_by INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES users(id)
);

-- Theme color definitions
CREATE TABLE theme_colors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    theme_id INTEGER NOT NULL,
    color_key TEXT NOT NULL, -- e.g., 'primary', 'secondary', 'background', etc.
    color_value TEXT NOT NULL, -- hex color code
    description TEXT,
    FOREIGN KEY (theme_id) REFERENCES themes(id) ON DELETE CASCADE,
    UNIQUE(theme_id, color_key)
);

-- User theme preferences
CREATE TABLE user_theme_preferences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    theme_id INTEGER NOT NULL,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (theme_id) REFERENCES themes(id),
    UNIQUE(user_id)
);

-- System settings table
CREATE TABLE settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

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
);-- Migration: Add Reports System
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
    
    -- Model filtering
    filter_model TEXT DEFAULT NULL,         -- Filter runs by model (e.g., 'gpt-4o', 'gpt-4o-mini')
    
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
    
    -- Enterprise enhancements (added 2025-11-12)
    best_run_example TEXT,                  -- JSON: {run_id, input, output, tool_calls, duration, why_successful}
    worst_run_example TEXT,                 -- JSON: {run_id, input, output, tool_calls, duration, why_failed}
    tool_usage_analysis TEXT,               -- JSON: [{tool_name, use_count, success_rate, avg_duration}]
    failure_patterns TEXT,                  -- JSON: [{pattern, frequency, examples, impact}]
    improvement_plan TEXT,                  -- JSON: [{issue, recommendation, priority, expected_impact, concrete_example}]
    
    -- Quality metrics from LLM-as-judge evaluations (added 2025-11-18)
    quality_metrics TEXT,                   -- JSON: {avg_task_completion, avg_relevancy, avg_faithfulness, avg_hallucination, avg_toxicity, pass_rates, evaluated_runs, total_runs}
    
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

-- Workflows table for versioned workflow definitions
CREATE TABLE workflows (
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

-- Workflow runs table for execution metadata
CREATE TABLE workflow_runs (
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

CREATE INDEX idx_workflow_runs_workflow ON workflow_runs(workflow_id);
CREATE INDEX idx_workflow_runs_status ON workflow_runs(status);

-- Workflow run steps table to track step attempts and history
CREATE TABLE workflow_run_steps (
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

CREATE INDEX idx_workflow_run_steps_run ON workflow_run_steps(run_id);

CREATE TABLE workflow_run_events (
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

CREATE INDEX idx_workflow_run_events_run ON workflow_run_events(run_id);
CREATE INDEX idx_workflow_run_events_type ON workflow_run_events(event_type);

CREATE TABLE workflow_approvals (
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

CREATE INDEX idx_workflow_approvals_run ON workflow_approvals(run_id);
CREATE INDEX idx_workflow_approvals_status ON workflow_approvals(status);

CREATE TABLE workflow_schedules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_id TEXT NOT NULL,
    workflow_version INTEGER NOT NULL,
    cron_expression TEXT NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    input TEXT,
    last_run_at DATETIME,
    next_run_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(workflow_id, workflow_version)
);

CREATE INDEX idx_workflow_schedules_enabled ON workflow_schedules(enabled);
CREATE INDEX idx_workflow_schedules_next_run ON workflow_schedules(next_run_at);

CREATE TABLE notification_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    log_id TEXT NOT NULL UNIQUE,
    approval_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
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

CREATE INDEX idx_notification_logs_approval ON notification_logs(approval_id);
CREATE INDEX idx_notification_logs_event_type ON notification_logs(event_type);
CREATE INDEX idx_notification_logs_created ON notification_logs(created_at);
