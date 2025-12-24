package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"station/internal/db/queries"
	"station/pkg/models"
)

// WorkflowRepo manages workflow definition persistence.
type WorkflowRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewWorkflowRepo(db *sql.DB) *WorkflowRepo {
	return &WorkflowRepo{
		db:      db,
		queries: queries.New(db),
	}
}

func (r *WorkflowRepo) GetNextVersion(ctx context.Context, workflowID string) (int64, error) {
	return r.queries.GetNextWorkflowVersion(ctx, workflowID)
}

func (r *WorkflowRepo) Insert(ctx context.Context, workflowID, name, description string, version int64, definition json.RawMessage, status string) (*models.WorkflowDefinition, error) {
	def := sql.NullString{String: string(definition), Valid: len(definition) > 0}
	desc := sql.NullString{String: description, Valid: description != ""}

	row, err := r.queries.InsertWorkflow(ctx, queries.InsertWorkflowParams{
		WorkflowID:  workflowID,
		Name:        name,
		Description: desc,
		Version:     version,
		Definition:  def.String,
		Status:      status,
	})
	if err != nil {
		return nil, err
	}

	return convertWorkflow(row), nil
}

func (r *WorkflowRepo) Get(ctx context.Context, workflowID string, version int64) (*models.WorkflowDefinition, error) {
	row, err := r.queries.GetWorkflow(ctx, queries.GetWorkflowParams{
		WorkflowID: workflowID,
		Version:    version,
	})
	if err != nil {
		return nil, err
	}
	return convertWorkflow(row), nil
}

func (r *WorkflowRepo) GetLatest(ctx context.Context, workflowID string) (*models.WorkflowDefinition, error) {
	row, err := r.queries.GetLatestWorkflow(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	return convertWorkflow(row), nil
}

func (r *WorkflowRepo) ListLatest(ctx context.Context) ([]*models.WorkflowDefinition, error) {
	rows, err := r.queries.ListLatestWorkflows(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*models.WorkflowDefinition, 0, len(rows))
	for _, row := range rows {
		result = append(result, convertWorkflow(row))
	}
	return result, nil
}

func (r *WorkflowRepo) ListVersions(ctx context.Context, workflowID string) ([]*models.WorkflowDefinition, error) {
	rows, err := r.queries.ListWorkflowVersions(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	result := make([]*models.WorkflowDefinition, 0, len(rows))
	for _, row := range rows {
		result = append(result, convertWorkflow(row))
	}
	return result, nil
}

func (r *WorkflowRepo) Disable(ctx context.Context, workflowID string) error {
	return r.queries.DisableWorkflow(ctx, workflowID)
}

func convertWorkflow(row queries.Workflow) *models.WorkflowDefinition {
	var desc *string
	if row.Description.Valid {
		value := row.Description.String
		desc = &value
	}

	return &models.WorkflowDefinition{
		ID:          row.ID,
		WorkflowID:  row.WorkflowID,
		Name:        row.Name,
		Description: desc,
		Version:     row.Version,
		Definition:  json.RawMessage(row.Definition),
		Status:      row.Status,
		CreatedAt:   nullTimeOrZero(row.CreatedAt),
		UpdatedAt:   nullTimeOrZero(row.UpdatedAt),
	}
}

// WorkflowRunRepo manages workflow run persistence.
type WorkflowRunRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewWorkflowRunRepo(db *sql.DB) *WorkflowRunRepo {
	return &WorkflowRunRepo{
		db:      db,
		queries: queries.New(db),
	}
}

// CreateWorkflowRunParams captures inputs for inserting a workflow run.
type CreateWorkflowRunParams struct {
	RunID           string
	WorkflowID      string
	WorkflowVersion int64
	Status          string
	CurrentStep     *string
	Input           json.RawMessage
	Context         json.RawMessage
	Result          json.RawMessage
	Error           *string
	Summary         *string
	Options         json.RawMessage
	LastSignal      json.RawMessage
	StartedAt       time.Time
	CompletedAt     *time.Time
}

func (r *WorkflowRunRepo) Create(ctx context.Context, params CreateWorkflowRunParams) (*models.WorkflowRun, error) {
	row, err := r.queries.InsertWorkflowRun(ctx, queries.InsertWorkflowRunParams{
		RunID:           params.RunID,
		WorkflowID:      params.WorkflowID,
		WorkflowVersion: params.WorkflowVersion,
		Status:          params.Status,
		CurrentStep:     toNullString(params.CurrentStep),
		Input:           toNullRaw(params.Input),
		Context:         toNullRaw(params.Context),
		Result:          toNullRaw(params.Result),
		Error:           toNullString(params.Error),
		Summary:         toNullString(params.Summary),
		Options:         toNullRaw(params.Options),
		LastSignal:      toNullRaw(params.LastSignal),
		StartedAt:       params.StartedAt,
		CompletedAt:     toNullTime(params.CompletedAt),
	})
	if err != nil {
		return nil, err
	}
	return convertWorkflowRun(row), nil
}

// UpdateWorkflowRunParams defines fields that can be updated for a run.
type UpdateWorkflowRunParams struct {
	RunID       string
	Status      string
	CurrentStep *string
	Context     json.RawMessage
	Result      json.RawMessage
	Error       *string
	Summary     *string
	Options     json.RawMessage
	LastSignal  json.RawMessage
	CompletedAt *time.Time
}

func (r *WorkflowRunRepo) Update(ctx context.Context, params UpdateWorkflowRunParams) error {
	return r.queries.UpdateWorkflowRunStatus(ctx, queries.UpdateWorkflowRunStatusParams{
		Status:      params.Status,
		CurrentStep: toNullString(params.CurrentStep),
		Context:     toNullRaw(params.Context),
		Result:      toNullRaw(params.Result),
		Error:       toNullString(params.Error),
		Summary:     toNullString(params.Summary),
		Options:     toNullRaw(params.Options),
		LastSignal:  toNullRaw(params.LastSignal),
		CompletedAt: toNullTime(params.CompletedAt),
		RunID:       params.RunID,
	})
}

func (r *WorkflowRunRepo) Get(ctx context.Context, runID string) (*models.WorkflowRun, error) {
	row, err := r.queries.GetWorkflowRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	return convertWorkflowRun(row), nil
}

func (r *WorkflowRunRepo) List(ctx context.Context, workflowID, status string, limit int64) ([]*models.WorkflowRun, error) {
	params := queries.ListWorkflowRunsParams{
		WorkflowID: nil,
		Status:     nil,
		Limit:      limit,
	}
	if workflowID != "" {
		params.WorkflowID = workflowID
	}
	if status != "" {
		params.Status = status
	}

	rows, err := r.queries.ListWorkflowRuns(ctx, params)
	if err != nil {
		return nil, err
	}

	result := make([]*models.WorkflowRun, 0, len(rows))
	for _, row := range rows {
		result = append(result, convertWorkflowRun(row))
	}
	return result, nil
}

func convertWorkflowRun(row queries.WorkflowRun) *models.WorkflowRun {
	var currentStep *string
	if row.CurrentStep.Valid {
		value := row.CurrentStep.String
		currentStep = &value
	}
	var errMsg *string
	if row.Error.Valid {
		value := row.Error.String
		errMsg = &value
	}
	var summary *string
	if row.Summary.Valid {
		value := row.Summary.String
		summary = &value
	}

	return &models.WorkflowRun{
		ID:              row.ID,
		RunID:           row.RunID,
		WorkflowID:      row.WorkflowID,
		WorkflowVersion: row.WorkflowVersion,
		Status:          row.Status,
		CurrentStep:     currentStep,
		Input:           rawOrNil(row.Input),
		Context:         rawOrNil(row.Context),
		Result:          rawOrNil(row.Result),
		Error:           errMsg,
		Summary:         summary,
		Options:         rawOrNil(row.Options),
		LastSignal:      rawOrNil(row.LastSignal),
		CreatedAt:       nullTimeOrZero(row.CreatedAt),
		UpdatedAt:       nullTimeOrZero(row.UpdatedAt),
		StartedAt:       nullTimeOrZero(row.StartedAt),
		CompletedAt:     nullTimePtr(row.CompletedAt),
	}
}

// WorkflowRunStepRepo manages step history for runs.
type WorkflowRunStepRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewWorkflowRunStepRepo(db *sql.DB) *WorkflowRunStepRepo {
	return &WorkflowRunStepRepo{
		db:      db,
		queries: queries.New(db),
	}
}

// CreateWorkflowRunStepParams defines inputs for step insertion.
type CreateWorkflowRunStepParams struct {
	RunID       string
	StepID      string
	Attempt     int64
	Status      string
	Input       json.RawMessage
	Output      json.RawMessage
	Error       *string
	Metadata    json.RawMessage
	StartedAt   *time.Time
	CompletedAt *time.Time
}

func (r *WorkflowRunStepRepo) Create(ctx context.Context, params CreateWorkflowRunStepParams) (*models.WorkflowRunStep, error) {
	row, err := r.queries.InsertWorkflowRunStep(ctx, queries.InsertWorkflowRunStepParams{
		RunID:       params.RunID,
		StepID:      params.StepID,
		Attempt:     params.Attempt,
		Status:      params.Status,
		Input:       toNullRaw(params.Input),
		Output:      toNullRaw(params.Output),
		Error:       toNullString(params.Error),
		Metadata:    toNullRaw(params.Metadata),
		StartedAt:   nullOrDefault(params.StartedAt),
		CompletedAt: toNullTime(params.CompletedAt),
	})
	if err != nil {
		return nil, err
	}
	return convertWorkflowRunStep(row), nil
}

// UpdateWorkflowRunStepParams defines fields allowed for step updates.
type UpdateWorkflowRunStepParams struct {
	RunID       string
	StepID      string
	Attempt     int64
	Status      string
	Output      json.RawMessage
	Error       *string
	Metadata    json.RawMessage
	CompletedAt *time.Time
}

func (r *WorkflowRunStepRepo) Update(ctx context.Context, params UpdateWorkflowRunStepParams) error {
	return r.queries.UpdateWorkflowRunStep(ctx, queries.UpdateWorkflowRunStepParams{
		Status:      params.Status,
		Output:      toNullRaw(params.Output),
		Error:       toNullString(params.Error),
		Metadata:    toNullRaw(params.Metadata),
		CompletedAt: toNullTime(params.CompletedAt),
		RunID:       params.RunID,
		StepID:      params.StepID,
		Attempt:     params.Attempt,
	})
}

func (r *WorkflowRunStepRepo) ListByRun(ctx context.Context, runID string) ([]*models.WorkflowRunStep, error) {
	rows, err := r.queries.ListWorkflowRunSteps(ctx, runID)
	if err != nil {
		return nil, err
	}

	result := make([]*models.WorkflowRunStep, 0, len(rows))
	for _, row := range rows {
		result = append(result, convertWorkflowRunStep(row))
	}
	return result, nil
}

func (r *WorkflowRunStepRepo) Get(ctx context.Context, runID, stepID string, attempt int64) (*models.WorkflowRunStep, error) {
	row, err := r.queries.GetWorkflowRunStep(ctx, queries.GetWorkflowRunStepParams{
		RunID:   runID,
		StepID:  stepID,
		Attempt: attempt,
	})
	if err != nil {
		return nil, err
	}
	return convertWorkflowRunStep(row), nil
}

func (r *WorkflowRunStepRepo) IsCompleted(ctx context.Context, runID, stepID string, attempt int64) (bool, error) {
	completed, err := r.queries.IsStepCompleted(ctx, queries.IsStepCompletedParams{
		RunID:   runID,
		StepID:  stepID,
		Attempt: attempt,
	})
	if err != nil {
		return false, err
	}
	return completed != 0, nil
}

func convertWorkflowRunStep(row queries.WorkflowRunStep) *models.WorkflowRunStep {
	var errMsg *string
	if row.Error.Valid {
		value := row.Error.String
		errMsg = &value
	}

	return &models.WorkflowRunStep{
		ID:          row.ID,
		RunID:       row.RunID,
		StepID:      row.StepID,
		Attempt:     row.Attempt,
		Status:      row.Status,
		Input:       rawOrNil(row.Input),
		Output:      rawOrNil(row.Output),
		Error:       errMsg,
		Metadata:    rawOrNil(row.Metadata),
		StartedAt:   nullTimeOrZero(row.StartedAt),
		CompletedAt: nullTimePtr(row.CompletedAt),
	}
}

func toNullString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func toNullRaw(raw json.RawMessage) sql.NullString {
	if len(raw) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: string(raw), Valid: true}
}

func toNullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func parseTimeString(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t, err := parseTimeFormats(s.String)
	if err != nil {
		return nil
	}
	return &t
}

func parseTimeFormats(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 +0000 UTC",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("unable to parse time")
}

func rawOrNil(value sql.NullString) json.RawMessage {
	if !value.Valid || value.String == "" {
		return nil
	}
	return json.RawMessage(value.String)
}

func nullTimeOrZero(value sql.NullTime) time.Time {
	if value.Valid {
		return value.Time
	}
	return time.Time{}
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if value.Valid {
		return &value.Time
	}
	return nil
}

func nullOrDefault(value *time.Time) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

// WorkflowRunEventRepo manages workflow run event persistence for audit trails.
type WorkflowRunEventRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewWorkflowRunEventRepo(db *sql.DB) *WorkflowRunEventRepo {
	return &WorkflowRunEventRepo{
		db:      db,
		queries: queries.New(db),
	}
}

type CreateWorkflowRunEventParams struct {
	RunID     string
	EventType string
	StepID    *string
	Payload   *string
	Actor     *string
}

func (r *WorkflowRunEventRepo) Insert(ctx context.Context, params CreateWorkflowRunEventParams) (*models.WorkflowRunEvent, error) {
	seq, err := r.queries.GetNextEventSeq(ctx, params.RunID)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.InsertWorkflowRunEvent(ctx, queries.InsertWorkflowRunEventParams{
		RunID:     params.RunID,
		Seq:       seq,
		EventType: params.EventType,
		StepID:    toNullString(params.StepID),
		Payload:   toNullString(params.Payload),
		Actor:     toNullString(params.Actor),
	})
	if err != nil {
		return nil, err
	}

	return convertWorkflowRunEvent(row), nil
}

func (r *WorkflowRunEventRepo) GetNextSeq(ctx context.Context, runID string) (int64, error) {
	return r.queries.GetNextEventSeq(ctx, runID)
}

func (r *WorkflowRunEventRepo) ListByRun(ctx context.Context, runID string) ([]*models.WorkflowRunEvent, error) {
	rows, err := r.queries.ListWorkflowRunEvents(ctx, runID)
	if err != nil {
		return nil, err
	}

	result := make([]*models.WorkflowRunEvent, 0, len(rows))
	for _, row := range rows {
		result = append(result, convertWorkflowRunEvent(row))
	}
	return result, nil
}

func (r *WorkflowRunEventRepo) ListByType(ctx context.Context, runID, eventType string) ([]*models.WorkflowRunEvent, error) {
	rows, err := r.queries.ListWorkflowRunEventsByType(ctx, queries.ListWorkflowRunEventsByTypeParams{
		RunID:     runID,
		EventType: eventType,
	})
	if err != nil {
		return nil, err
	}

	result := make([]*models.WorkflowRunEvent, 0, len(rows))
	for _, row := range rows {
		result = append(result, convertWorkflowRunEvent(row))
	}
	return result, nil
}

func convertWorkflowRunEvent(row queries.WorkflowRunEvent) *models.WorkflowRunEvent {
	var stepID *string
	if row.StepID.Valid {
		value := row.StepID.String
		stepID = &value
	}
	var payload *string
	if row.Payload.Valid {
		value := row.Payload.String
		payload = &value
	}
	var actor *string
	if row.Actor.Valid {
		value := row.Actor.String
		actor = &value
	}

	return &models.WorkflowRunEvent{
		ID:        row.ID,
		RunID:     row.RunID,
		Seq:       row.Seq,
		EventType: row.EventType,
		StepID:    stepID,
		Payload:   payload,
		Actor:     actor,
		CreatedAt: nullTimeOrZero(row.CreatedAt),
	}
}

// WorkflowApprovalRepo manages workflow approval persistence.
type WorkflowApprovalRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewWorkflowApprovalRepo(db *sql.DB) *WorkflowApprovalRepo {
	return &WorkflowApprovalRepo{
		db:      db,
		queries: queries.New(db),
	}
}

type CreateWorkflowApprovalParams struct {
	ApprovalID  string
	RunID       string
	StepID      string
	Message     string
	SummaryPath *string
	Approvers   *string
	TimeoutAt   *time.Time
}

func (r *WorkflowApprovalRepo) Create(ctx context.Context, params CreateWorkflowApprovalParams) (*models.WorkflowApproval, error) {
	row, err := r.queries.InsertWorkflowApproval(ctx, queries.InsertWorkflowApprovalParams{
		ApprovalID:  params.ApprovalID,
		RunID:       params.RunID,
		StepID:      params.StepID,
		Message:     params.Message,
		SummaryPath: toNullString(params.SummaryPath),
		Approvers:   toNullString(params.Approvers),
		Status:      models.ApprovalStatusPending,
		TimeoutAt:   toNullTime(params.TimeoutAt),
	})
	if err != nil {
		return nil, err
	}

	return convertWorkflowApproval(row), nil
}

func (r *WorkflowApprovalRepo) Get(ctx context.Context, approvalID string) (*models.WorkflowApproval, error) {
	row, err := r.queries.GetWorkflowApproval(ctx, approvalID)
	if err != nil {
		return nil, err
	}
	return convertWorkflowApproval(row), nil
}

func (r *WorkflowApprovalRepo) ListByRun(ctx context.Context, runID string) ([]*models.WorkflowApproval, error) {
	rows, err := r.queries.ListWorkflowApprovals(ctx, runID)
	if err != nil {
		return nil, err
	}

	result := make([]*models.WorkflowApproval, 0, len(rows))
	for _, row := range rows {
		result = append(result, convertWorkflowApproval(row))
	}
	return result, nil
}

func (r *WorkflowApprovalRepo) ListPending(ctx context.Context, limit int64) ([]*models.WorkflowApproval, error) {
	rows, err := r.queries.ListPendingApprovals(ctx, limit)
	if err != nil {
		return nil, err
	}

	result := make([]*models.WorkflowApproval, 0, len(rows))
	for _, row := range rows {
		result = append(result, convertWorkflowApproval(row))
	}
	return result, nil
}

func (r *WorkflowApprovalRepo) Approve(ctx context.Context, approvalID string, decidedBy *string, reason *string) error {
	return r.queries.ApproveWorkflowApproval(ctx, queries.ApproveWorkflowApprovalParams{
		ApprovalID:     approvalID,
		DecidedBy:      toNullString(decidedBy),
		DecisionReason: toNullString(reason),
	})
}

func (r *WorkflowApprovalRepo) Reject(ctx context.Context, approvalID string, decidedBy *string, reason *string) error {
	return r.queries.RejectWorkflowApproval(ctx, queries.RejectWorkflowApprovalParams{
		ApprovalID:     approvalID,
		DecidedBy:      toNullString(decidedBy),
		DecisionReason: toNullString(reason),
	})
}

func (r *WorkflowApprovalRepo) TimeoutExpired(ctx context.Context) error {
	return r.queries.TimeoutExpiredApprovals(ctx)
}

func convertWorkflowApproval(row queries.WorkflowApproval) *models.WorkflowApproval {
	var summaryPath *string
	if row.SummaryPath.Valid {
		value := row.SummaryPath.String
		summaryPath = &value
	}
	var approvers *string
	if row.Approvers.Valid {
		value := row.Approvers.String
		approvers = &value
	}
	var decidedBy *string
	if row.DecidedBy.Valid {
		value := row.DecidedBy.String
		decidedBy = &value
	}
	var decisionReason *string
	if row.DecisionReason.Valid {
		value := row.DecisionReason.String
		decisionReason = &value
	}

	return &models.WorkflowApproval{
		ID:             row.ID,
		ApprovalID:     row.ApprovalID,
		RunID:          row.RunID,
		StepID:         row.StepID,
		Message:        row.Message,
		SummaryPath:    summaryPath,
		Approvers:      approvers,
		Status:         row.Status,
		DecidedBy:      decidedBy,
		DecidedAt:      nullTimePtr(row.DecidedAt),
		DecisionReason: decisionReason,
		TimeoutAt:      nullTimePtr(row.TimeoutAt),
		CreatedAt:      nullTimeOrZero(row.CreatedAt),
		UpdatedAt:      nullTimeOrZero(row.UpdatedAt),
	}
}

type WorkflowScheduleRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewWorkflowScheduleRepo(db *sql.DB) *WorkflowScheduleRepo {
	return &WorkflowScheduleRepo{
		db:      db,
		queries: queries.New(db),
	}
}

type CreateWorkflowScheduleParams struct {
	WorkflowID      string
	WorkflowVersion int64
	CronExpression  string
	Timezone        string
	Enabled         bool
	Input           json.RawMessage
	NextRunAt       *time.Time
}

func (r *WorkflowScheduleRepo) Create(ctx context.Context, params CreateWorkflowScheduleParams) (*models.WorkflowSchedule, error) {
	query := `INSERT INTO workflow_schedules (workflow_id, workflow_version, cron_expression, timezone, enabled, input, next_run_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id, workflow_id, workflow_version, cron_expression, timezone, enabled, input, last_run_at, next_run_at, created_at, updated_at`

	var nextRunAtStr sql.NullString
	if params.NextRunAt != nil {
		nextRunAtStr = sql.NullString{String: params.NextRunAt.UTC().Format(time.RFC3339), Valid: true}
	}

	row := r.db.QueryRowContext(ctx, query,
		params.WorkflowID, params.WorkflowVersion, params.CronExpression,
		params.Timezone, params.Enabled, toNullRaw(params.Input), nextRunAtStr)

	var s models.WorkflowSchedule
	var input, lastRunAt, nextRunAt, createdAt, updatedAt sql.NullString
	if err := row.Scan(&s.ID, &s.WorkflowID, &s.WorkflowVersion, &s.CronExpression, &s.Timezone, &s.Enabled, &input, &lastRunAt, &nextRunAt, &createdAt, &updatedAt); err != nil {
		return nil, err
	}

	if input.Valid {
		s.Input = json.RawMessage(input.String)
	}
	s.LastRunAt = parseTimeString(lastRunAt)
	s.NextRunAt = parseTimeString(nextRunAt)
	if createdAt.Valid {
		if t, err := parseTimeFormats(createdAt.String); err == nil {
			s.CreatedAt = t
		}
	}
	if updatedAt.Valid {
		if t, err := parseTimeFormats(updatedAt.String); err == nil {
			s.UpdatedAt = t
		}
	}
	return &s, nil
}

func (r *WorkflowScheduleRepo) Get(ctx context.Context, workflowID string, version int64) (*models.WorkflowSchedule, error) {
	row, err := r.queries.GetWorkflowSchedule(ctx, queries.GetWorkflowScheduleParams{
		WorkflowID:      workflowID,
		WorkflowVersion: version,
	})
	if err != nil {
		return nil, err
	}
	return convertWorkflowSchedule(row), nil
}

func (r *WorkflowScheduleRepo) GetByID(ctx context.Context, id int64) (*models.WorkflowSchedule, error) {
	row, err := r.queries.GetWorkflowScheduleByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return convertWorkflowSchedule(row), nil
}

func (r *WorkflowScheduleRepo) ListEnabled(ctx context.Context) ([]*models.WorkflowSchedule, error) {
	rows, err := r.queries.ListEnabledSchedules(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*models.WorkflowSchedule, 0, len(rows))
	for _, row := range rows {
		result = append(result, convertWorkflowSchedule(row))
	}
	return result, nil
}

func (r *WorkflowScheduleRepo) ListDue(ctx context.Context, now time.Time) ([]*models.WorkflowSchedule, error) {
	query := `SELECT id, workflow_id, workflow_version, cron_expression, timezone, enabled, input, last_run_at, next_run_at, created_at, updated_at 
		FROM workflow_schedules 
		WHERE enabled = 1 AND next_run_at <= ? 
		ORDER BY next_run_at ASC`

	nowStr := now.UTC().Format(time.RFC3339)
	rows, err := r.db.QueryContext(ctx, query, nowStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*models.WorkflowSchedule
	for rows.Next() {
		var s models.WorkflowSchedule
		var input, lastRunAt, nextRunAt, createdAt, updatedAt sql.NullString

		if err := rows.Scan(&s.ID, &s.WorkflowID, &s.WorkflowVersion, &s.CronExpression, &s.Timezone, &s.Enabled, &input, &lastRunAt, &nextRunAt, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		if input.Valid {
			s.Input = json.RawMessage(input.String)
		}
		s.LastRunAt = parseTimeString(lastRunAt)
		s.NextRunAt = parseTimeString(nextRunAt)
		if createdAt.Valid {
			if t, err := parseTimeFormats(createdAt.String); err == nil {
				s.CreatedAt = t
			}
		}
		if updatedAt.Valid {
			if t, err := parseTimeFormats(updatedAt.String); err == nil {
				s.UpdatedAt = t
			}
		}
		result = append(result, &s)
	}
	return result, rows.Err()
}

func (r *WorkflowScheduleRepo) UpdateLastRun(ctx context.Context, id int64, lastRunAt, nextRunAt time.Time) error {
	query := `UPDATE workflow_schedules SET last_run_at = ?, next_run_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query,
		lastRunAt.UTC().Format(time.RFC3339),
		nextRunAt.UTC().Format(time.RFC3339),
		id)
	return err
}

func (r *WorkflowScheduleRepo) SetEnabled(ctx context.Context, workflowID string, version int64, enabled bool) error {
	return r.queries.UpdateScheduleEnabled(ctx, queries.UpdateScheduleEnabledParams{
		Enabled:         enabled,
		WorkflowID:      workflowID,
		WorkflowVersion: version,
	})
}

func (r *WorkflowScheduleRepo) Delete(ctx context.Context, workflowID string, version int64) error {
	return r.queries.DeleteWorkflowSchedule(ctx, queries.DeleteWorkflowScheduleParams{
		WorkflowID:      workflowID,
		WorkflowVersion: version,
	})
}

func (r *WorkflowScheduleRepo) DeleteByWorkflowID(ctx context.Context, workflowID string) error {
	return r.queries.DeleteWorkflowScheduleByWorkflowID(ctx, workflowID)
}

func convertWorkflowSchedule(row queries.WorkflowSchedule) *models.WorkflowSchedule {
	return &models.WorkflowSchedule{
		ID:              row.ID,
		WorkflowID:      row.WorkflowID,
		WorkflowVersion: row.WorkflowVersion,
		CronExpression:  row.CronExpression,
		Timezone:        row.Timezone,
		Enabled:         row.Enabled,
		Input:           rawOrNil(row.Input),
		LastRunAt:       nullTimePtr(row.LastRunAt),
		NextRunAt:       nullTimePtr(row.NextRunAt),
		CreatedAt:       nullTimeOrZero(row.CreatedAt),
		UpdatedAt:       nullTimeOrZero(row.UpdatedAt),
	}
}
