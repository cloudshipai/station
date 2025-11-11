-- Migration: Add Benchmark System (Task-Based + Quality Metrics)
-- Description: Extends reports with concrete task-based benchmarks and quality metrics
-- Date: 2025-11-11
-- Context: Transform abstract scores into evidence-based production readiness assessments

-- ============================================================================
-- BENCHMARK TASKS: Concrete evaluation tasks with measurable success criteria
-- ============================================================================

CREATE TABLE IF NOT EXISTS benchmark_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    category TEXT NOT NULL, -- 'security', 'finops', 'compliance', 'devops', 'custom'
    
    -- Success criteria (JSON with concrete metrics)
    -- Example: {"min_detections": 45, "max_time_minutes": 5, "required_tools": ["checkov", "semgrep"]}
    success_criteria TEXT NOT NULL,
    
    -- Weighting for production readiness calculation (0.0 to 1.0)
    weight REAL NOT NULL DEFAULT 1.0,
    
    -- Task metadata
    environment_id INTEGER, -- NULL = global task, set = environment-specific
    is_active BOOLEAN NOT NULL DEFAULT 1,
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_benchmark_tasks_category ON benchmark_tasks(category);
CREATE INDEX IF NOT EXISTS idx_benchmark_tasks_environment ON benchmark_tasks(environment_id);
CREATE INDEX IF NOT EXISTS idx_benchmark_tasks_active ON benchmark_tasks(is_active);

-- ============================================================================
-- BENCHMARK METRICS: Quality metrics per run (LLM-as-judge evaluation)
-- ============================================================================

CREATE TABLE IF NOT EXISTS benchmark_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id INTEGER NOT NULL,
    metric_type TEXT NOT NULL, -- 'hallucination', 'relevancy', 'task_completion', 'faithfulness', 'toxicity', 'bias'
    
    -- Metric results
    score REAL NOT NULL, -- 0.0 to 1.0 (normalized)
    threshold REAL NOT NULL, -- Expected threshold for this metric
    passed BOOLEAN NOT NULL, -- Did run meet threshold?
    
    -- Evaluation details
    reason TEXT, -- LLM-generated explanation of score
    verdicts TEXT, -- JSON: Array of individual verdicts with reasoning
    
    -- Evidence (links to actual execution data)
    evidence TEXT, -- JSON: {tool_call_ids: [], trace_ids: [], span_ids: []}
    
    -- LLM evaluation metadata
    judge_model TEXT DEFAULT 'gpt-4o-mini',
    judge_tokens INTEGER DEFAULT 0,
    judge_cost REAL DEFAULT 0.0,
    evaluation_duration_ms INTEGER,
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (run_id) REFERENCES agent_runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_benchmark_metrics_run ON benchmark_metrics(run_id);
CREATE INDEX IF NOT EXISTS idx_benchmark_metrics_type ON benchmark_metrics(metric_type);
CREATE INDEX IF NOT EXISTS idx_benchmark_metrics_passed ON benchmark_metrics(passed);
CREATE INDEX IF NOT EXISTS idx_benchmark_metrics_score ON benchmark_metrics(score);

-- ============================================================================
-- TASK EVALUATIONS: Task performance by agent (for competitive rankings)
-- ============================================================================

CREATE TABLE IF NOT EXISTS task_evaluations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    agent_id INTEGER NOT NULL,
    report_id INTEGER, -- NULL if standalone evaluation, set if part of report
    
    -- Task performance
    task_score REAL NOT NULL, -- 0-10 score for this task
    completed BOOLEAN NOT NULL DEFAULT 0, -- Did agent complete task successfully?
    
    -- Success criteria evaluation (JSON)
    -- Example: {"min_detections": {"expected": 45, "actual": 47, "passed": true}}
    criteria_results TEXT,
    
    -- Quality metrics summary (averages across runs)
    avg_hallucination REAL,
    avg_relevancy REAL,
    avg_task_completion REAL,
    avg_faithfulness REAL,
    avg_toxicity REAL,
    avg_bias REAL,
    
    -- Performance metrics
    runs_analyzed INTEGER NOT NULL DEFAULT 0,
    run_ids TEXT, -- Comma-separated list of run IDs used for evaluation
    trace_ids TEXT, -- Comma-separated list of Jaeger trace IDs
    
    avg_duration_seconds REAL,
    avg_tokens INTEGER,
    avg_cost REAL,
    tool_calls_count INTEGER DEFAULT 0,
    
    -- Analysis results
    strengths TEXT, -- JSON: Array of identified strengths
    weaknesses TEXT, -- JSON: Array of identified weaknesses
    
    -- Rankings
    rank INTEGER, -- 1st, 2nd, 3rd place for this task
    is_champion BOOLEAN DEFAULT 0, -- Best performer for this task
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (task_id) REFERENCES benchmark_tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (report_id) REFERENCES reports(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_task_evaluations_task ON task_evaluations(task_id);
CREATE INDEX IF NOT EXISTS idx_task_evaluations_agent ON task_evaluations(agent_id);
CREATE INDEX IF NOT EXISTS idx_task_evaluations_report ON task_evaluations(report_id);
CREATE INDEX IF NOT EXISTS idx_task_evaluations_score ON task_evaluations(task_score DESC);
CREATE INDEX IF NOT EXISTS idx_task_evaluations_champion ON task_evaluations(is_champion);
CREATE INDEX IF NOT EXISTS idx_task_evaluations_rank ON task_evaluations(rank);

-- ============================================================================
-- PRODUCTION READINESS: Overall deployment assessment per report
-- ============================================================================

CREATE TABLE IF NOT EXISTS production_readiness (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id INTEGER NOT NULL UNIQUE,
    
    -- Overall scores
    task_completion_score REAL NOT NULL, -- 0-10 (average across all tasks)
    quality_score REAL NOT NULL, -- 0-10 (average of quality metrics)
    production_readiness_score REAL NOT NULL, -- 0-100 (weighted combination)
    
    -- Quality metrics summary (team-wide averages)
    avg_hallucination REAL,
    avg_relevancy REAL,
    avg_task_completion REAL,
    avg_faithfulness REAL,
    avg_toxicity REAL,
    avg_bias REAL,
    
    -- Pass rates (percentage of runs passing thresholds)
    hallucination_pass_rate REAL,
    relevancy_pass_rate REAL,
    task_completion_pass_rate REAL,
    faithfulness_pass_rate REAL,
    toxicity_pass_rate REAL,
    bias_pass_rate REAL,
    
    -- Deployment recommendation
    recommendation TEXT NOT NULL, -- 'PRODUCTION_READY', 'CONDITIONAL_GO', 'NEEDS_IMPROVEMENT', 'NOT_READY'
    risk_level TEXT NOT NULL, -- 'LOW', 'MEDIUM', 'HIGH', 'CRITICAL'
    
    -- Required actions (JSON array)
    required_actions TEXT, -- JSON: ["Add container scanner", "Retire underperforming agent", ...]
    blockers TEXT, -- JSON: ["Critical security failures", "High hallucination rate", ...]
    
    -- Champion agents (JSON)
    -- Example: {"security": {"agent_id": 5, "agent_name": "iac-security-scanner", "score": 9.5}, ...}
    champion_agents TEXT,
    
    -- Underperforming agents (JSON array)
    -- Example: [{"agent_id": 8, "agent_name": "old-scanner", "score": 4.2, "action": "RETIRE"}]
    underperforming_agents TEXT,
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (report_id) REFERENCES reports(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_production_readiness_report ON production_readiness(report_id);
CREATE INDEX IF NOT EXISTS idx_production_readiness_score ON production_readiness(production_readiness_score DESC);
CREATE INDEX IF NOT EXISTS idx_production_readiness_recommendation ON production_readiness(recommendation);

-- ============================================================================
-- TRIGGERS: Auto-update timestamps
-- ============================================================================

CREATE TRIGGER IF NOT EXISTS update_benchmark_tasks_timestamp 
AFTER UPDATE ON benchmark_tasks
BEGIN
    UPDATE benchmark_tasks SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_task_evaluations_timestamp 
AFTER UPDATE ON task_evaluations
BEGIN
    UPDATE task_evaluations SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_production_readiness_timestamp 
AFTER UPDATE ON production_readiness
BEGIN
    UPDATE production_readiness SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- ============================================================================
-- SEED DATA: Common benchmark tasks
-- ============================================================================

-- Security: OWASP Top 10 Detection
INSERT INTO benchmark_tasks (name, description, category, success_criteria, weight) VALUES
(
    'OWASP Top 10 Detection',
    'Detect critical web application vulnerabilities from OWASP Top 10 (Injection, XSS, CSRF, etc.)',
    'security',
    json('{"min_detections": 45, "max_false_positives": 5, "max_time_minutes": 5, "required_tools": ["checkov", "semgrep", "gitleaks"], "coverage": ["SQL injection", "XSS", "CSRF", "auth issues"]}'),
    0.40
);

-- Security: Container Vulnerability Scanning
INSERT INTO benchmark_tasks (name, description, category, success_criteria, weight) VALUES
(
    'Container Vulnerability Scanning',
    'Identify vulnerabilities in Docker containers and base images',
    'security',
    json('{"min_critical_found": 3, "min_high_found": 8, "max_time_minutes": 3, "required_tools": ["trivy", "hadolint"], "scan_targets": ["Dockerfile", "docker-compose.yml"]}'),
    0.30
);

-- FinOps: AWS Cost Anomaly Detection
INSERT INTO benchmark_tasks (name, description, category, success_criteria, weight) VALUES
(
    'AWS Cost Anomaly Detection',
    'Identify unusual AWS spending patterns and recommend optimizations',
    'finops',
    json('{"min_anomalies_found": 3, "min_savings_identified_usd": 500, "max_time_minutes": 3, "accuracy_threshold": 0.85, "required_tools": ["aws-billing", "aws-cost-explorer"]}'),
    0.30
);

-- FinOps: Resource Right-Sizing
INSERT INTO benchmark_tasks (name, description, category, success_criteria, weight) VALUES
(
    'AWS Resource Right-Sizing',
    'Identify over-provisioned EC2, RDS, and ECS resources with optimization recommendations',
    'finops',
    json('{"min_resources_analyzed": 20, "min_optimization_opportunities": 5, "max_time_minutes": 4, "required_tools": ["aws-ec2", "aws-rds", "aws-ecs"]}'),
    0.25
);

-- Compliance: CIS Benchmark Validation
INSERT INTO benchmark_tasks (name, description, category, success_criteria, weight) VALUES
(
    'CIS Benchmark Compliance Check',
    'Validate infrastructure against CIS benchmarks for AWS, Azure, or GCP',
    'compliance',
    json('{"min_checks_performed": 50, "critical_failures_threshold": 0, "max_time_minutes": 6, "required_tools": ["aws-well-architected", "scout-suite"]}'),
    0.35
);

-- DevOps: Terraform Quality Analysis
INSERT INTO benchmark_tasks (name, description, category, success_criteria, weight) VALUES
(
    'Terraform Code Quality',
    'Analyze Terraform for best practices, security issues, and compliance violations',
    'devops',
    json('{"min_files_analyzed": 10, "max_critical_issues": 0, "max_time_minutes": 4, "required_tools": ["tflint", "checkov"], "checks": ["naming", "tagging", "security", "cost"]}'),
    0.25
);

-- ============================================================================
-- VIEWS: Convenient queries for common operations
-- ============================================================================

-- View: Agent rankings per task
CREATE VIEW IF NOT EXISTS v_agent_rankings AS
SELECT 
    te.task_id,
    bt.name AS task_name,
    bt.category,
    te.agent_id,
    te.agent_name,
    te.task_score,
    te.rank,
    te.is_champion,
    te.runs_analyzed,
    te.avg_hallucination,
    te.avg_relevancy,
    te.avg_task_completion,
    te.avg_toxicity,
    te.completed
FROM task_evaluations te
JOIN benchmark_tasks bt ON te.task_id = bt.id
WHERE bt.is_active = 1
ORDER BY te.task_id, te.rank;

-- View: Quality metrics summary per run
CREATE VIEW IF NOT EXISTS v_run_quality_metrics AS
SELECT 
    ar.id AS run_id,
    ar.agent_id,
    a.name AS agent_name,
    ar.status,
    ar.duration,
    MAX(CASE WHEN bm.metric_type = 'hallucination' THEN bm.score END) AS hallucination_score,
    MAX(CASE WHEN bm.metric_type = 'relevancy' THEN bm.score END) AS relevancy_score,
    MAX(CASE WHEN bm.metric_type = 'task_completion' THEN bm.score END) AS task_completion_score,
    MAX(CASE WHEN bm.metric_type = 'faithfulness' THEN bm.score END) AS faithfulness_score,
    MAX(CASE WHEN bm.metric_type = 'toxicity' THEN bm.score END) AS toxicity_score,
    SUM(CASE WHEN bm.passed = 1 THEN 1 ELSE 0 END) AS metrics_passed,
    COUNT(bm.id) AS total_metrics
FROM agent_runs ar
JOIN agents a ON ar.agent_id = a.id
LEFT JOIN benchmark_metrics bm ON ar.id = bm.run_id
GROUP BY ar.id, ar.agent_id, a.name, ar.status, ar.duration;

-- View: Production readiness dashboard
CREATE VIEW IF NOT EXISTS v_production_readiness_dashboard AS
SELECT 
    r.id AS report_id,
    r.name AS report_name,
    e.name AS environment_name,
    pr.production_readiness_score,
    pr.recommendation,
    pr.risk_level,
    pr.task_completion_score,
    pr.quality_score,
    pr.avg_hallucination,
    pr.avg_relevancy,
    pr.avg_toxicity,
    r.total_agents_analyzed,
    r.total_runs_analyzed,
    r.created_at
FROM reports r
JOIN environments e ON r.environment_id = e.id
LEFT JOIN production_readiness pr ON r.id = pr.report_id
WHERE r.status = 'completed'
ORDER BY r.created_at DESC;

-- ============================================================================
-- END MIGRATION
-- ============================================================================
