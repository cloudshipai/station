package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
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
