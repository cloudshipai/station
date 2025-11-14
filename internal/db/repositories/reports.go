package repositories

import (
	"context"
	"database/sql"
	"station/internal/db/queries"
)

type ReportRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewReportRepo(db *sql.DB) *ReportRepo {
	return &ReportRepo{
		db:      db,
		queries: queries.New(db),
	}
}

// CreateReport creates a new report
func (r *ReportRepo) CreateReport(ctx context.Context, params queries.CreateReportParams) (queries.Report, error) {
	return r.queries.CreateReport(ctx, params)
}

// GetByID gets a report by ID
func (r *ReportRepo) GetByID(ctx context.Context, id int64) (queries.Report, error) {
	return r.queries.GetReport(ctx, id)
}

// ListReports lists reports with pagination
func (r *ReportRepo) ListReports(ctx context.Context, params queries.ListReportsParams) ([]queries.Report, error) {
	return r.queries.ListReports(ctx, params)
}

// ListByEnvironment lists reports for an environment
func (r *ReportRepo) ListByEnvironment(ctx context.Context, environmentID int64) ([]queries.Report, error) {
	return r.queries.ListReportsByEnvironment(ctx, environmentID)
}

// UpdateStatus updates report status and progress
func (r *ReportRepo) UpdateStatus(ctx context.Context, params queries.UpdateReportStatusParams) error {
	return r.queries.UpdateReportStatus(ctx, params)
}

// UpdateTeamResults updates team-level evaluation results
func (r *ReportRepo) UpdateTeamResults(ctx context.Context, params queries.UpdateReportTeamResultsParams) error {
	return r.queries.UpdateReportTeamResults(ctx, params)
}

// CompleteReport marks report as completed
func (r *ReportRepo) CompleteReport(ctx context.Context, params queries.CompleteReportParams) error {
	return r.queries.CompleteReport(ctx, params)
}

// FailReport marks report as failed
func (r *ReportRepo) FailReport(ctx context.Context, params queries.FailReportParams) error {
	return r.queries.FailReport(ctx, params)
}

// SetGenerationStarted sets generation started timestamp
func (r *ReportRepo) SetGenerationStarted(ctx context.Context, id int64) error {
	return r.queries.SetReportGenerationStarted(ctx, id)
}

// DeleteReport deletes a report
func (r *ReportRepo) DeleteReport(ctx context.Context, id int64) error {
	return r.queries.DeleteReport(ctx, id)
}

// CreateAgentReportDetail creates agent-level evaluation detail
func (r *ReportRepo) CreateAgentReportDetail(ctx context.Context, params queries.CreateAgentReportDetailParams) (queries.AgentReportDetail, error) {
	return r.queries.CreateAgentReportDetail(ctx, params)
}

// GetAgentReportDetails gets all agent details for a report
func (r *ReportRepo) GetAgentReportDetails(ctx context.Context, reportID int64) ([]queries.AgentReportDetail, error) {
	return r.queries.GetAgentReportDetails(ctx, reportID)
}

// GetAgentReportDetailByAgentID gets agent detail by agent ID
func (r *ReportRepo) GetAgentReportDetailByAgentID(ctx context.Context, reportID, agentID int64) (queries.AgentReportDetail, error) {
	return r.queries.GetAgentReportDetailByAgentID(ctx, queries.GetAgentReportDetailByAgentIDParams{
		ReportID: reportID,
		AgentID:  agentID,
	})
}

// GetLatestByEnvironment gets the most recent report for an environment
func (r *ReportRepo) GetLatestByEnvironment(ctx context.Context, environmentID int64) (queries.Report, error) {
	return r.queries.GetLatestReportByEnvironment(ctx, environmentID)
}
