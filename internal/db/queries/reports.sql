-- name: CreateReport :one
INSERT INTO reports (
    name,
    description,
    environment_id,
    team_criteria,
    agent_criteria,
    judge_model,
    filter_model
) VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetReport :one
SELECT * FROM reports WHERE id = ?;

-- name: ListReports :many
SELECT * FROM reports ORDER BY created_at DESC LIMIT ? OFFSET ?;

-- name: ListReportsByEnvironment :many
SELECT * FROM reports WHERE environment_id = ? ORDER BY created_at DESC;

-- name: UpdateReportStatus :exec
UPDATE reports 
SET status = ?,
    progress = ?,
    current_step = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateReportTeamResults :exec
UPDATE reports
SET executive_summary = ?,
    team_score = ?,
    team_reasoning = ?,
    team_criteria_scores = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: CompleteReport :exec
UPDATE reports
SET status = 'completed',
    progress = 100,
    generation_completed_at = CURRENT_TIMESTAMP,
    generation_duration_seconds = ?,
    total_runs_analyzed = ?,
    total_agents_analyzed = ?,
    total_llm_tokens = ?,
    total_llm_cost = ?,
    agent_reports = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: FailReport :exec
UPDATE reports
SET status = 'failed',
    error_message = ?,
    generation_completed_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: SetReportGenerationStarted :exec
UPDATE reports
SET generation_started_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteReport :exec
DELETE FROM reports WHERE id = ?;

-- name: CreateAgentReportDetail :one
INSERT INTO agent_report_details (
    report_id,
    agent_id,
    agent_name,
    score,
    passed,
    reasoning,
    criteria_scores,
    runs_analyzed,
    run_ids,
    avg_duration_seconds,
    avg_tokens,
    avg_cost,
    success_rate,
    strengths,
    weaknesses,
    recommendations,
    telemetry_summary,
    best_run_example,
    worst_run_example,
    tool_usage_analysis,
    failure_patterns,
    improvement_plan
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAgentReportDetails :many
SELECT * FROM agent_report_details WHERE report_id = ? ORDER BY score DESC;

-- name: GetAgentReportDetail :one
SELECT * FROM agent_report_details WHERE id = ?;

-- name: GetAgentReportDetailByAgentID :one
SELECT * FROM agent_report_details WHERE report_id = ? AND agent_id = ?;

-- name: DeleteAgentReportDetails :exec
DELETE FROM agent_report_details WHERE report_id = ?;

-- name: CountReportsByEnvironment :one
SELECT COUNT(*) FROM reports WHERE environment_id = ?;

-- name: GetLatestReportByEnvironment :one
SELECT * FROM reports 
WHERE environment_id = ? 
ORDER BY created_at DESC 
LIMIT 1;
