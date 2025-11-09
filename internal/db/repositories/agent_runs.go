package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"station/internal/db/queries"
	"station/pkg/models"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type AgentRunRepo struct {
	db      *sql.DB
	queries *queries.Queries
	tracer  trace.Tracer
}

func NewAgentRunRepo(db *sql.DB) *AgentRunRepo {
	return &AgentRunRepo{
		db:      db,
		queries: queries.New(db),
		tracer:  otel.Tracer("station-database"),
	}
}

// convertAgentRunFromSQLc converts sqlc AgentRun to models.AgentRun
func convertAgentRunFromSQLc(run queries.AgentRun) *models.AgentRun {
	result := &models.AgentRun{
		ID:            run.ID,
		AgentID:       run.AgentID,
		UserID:        run.UserID,
		Task:          run.Task,
		FinalResponse: run.FinalResponse,
		StepsTaken:    run.StepsTaken,
		Status:        run.Status,
	}

	if run.ToolCalls.Valid {
		// Parse JSON string to JSONArray
		var toolCalls models.JSONArray
		if err := (&toolCalls).Scan(run.ToolCalls.String); err == nil {
			result.ToolCalls = &toolCalls
		}
	}

	if run.ExecutionSteps.Valid {
		// Parse JSON string to JSONArray
		var executionSteps models.JSONArray
		if err := (&executionSteps).Scan(run.ExecutionSteps.String); err == nil {
			result.ExecutionSteps = &executionSteps
		}
	}

	if run.StartedAt.Valid {
		result.StartedAt = run.StartedAt.Time
	}

	if run.CompletedAt.Valid {
		result.CompletedAt = &run.CompletedAt.Time
	}

	// Convert response object metadata
	if run.InputTokens.Valid {
		inputTokens := run.InputTokens.Int64
		result.InputTokens = &inputTokens
	}

	if run.OutputTokens.Valid {
		outputTokens := run.OutputTokens.Int64
		result.OutputTokens = &outputTokens
	}

	if run.TotalTokens.Valid {
		totalTokens := run.TotalTokens.Int64
		result.TotalTokens = &totalTokens
	}

	if run.DurationSeconds.Valid {
		durationSeconds := run.DurationSeconds.Float64
		result.DurationSeconds = &durationSeconds
	}

	if run.ModelName.Valid {
		modelName := run.ModelName.String
		result.ModelName = &modelName
	}

	if run.ToolsUsed.Valid {
		toolsUsed := run.ToolsUsed.Int64
		result.ToolsUsed = &toolsUsed
	}

	if run.DebugLogs.Valid {
		// Parse JSON string to JSONArray
		var debugLogs models.JSONArray
		if err := (&debugLogs).Scan(run.DebugLogs.String); err == nil {
			result.DebugLogs = &debugLogs
		}
	}

	return result
}

// convertAgentRunWithDetailsFromSQLc converts sqlc row types to models.AgentRunWithDetails
func convertAgentRunWithDetailsFromSQLc(row interface{}) *models.AgentRunWithDetails {
	var result *models.AgentRunWithDetails

	// Handle both ListRecentAgentRunsRow and GetAgentRunWithDetailsRow
	switch r := row.(type) {
	case queries.ListRecentAgentRunsRow:
		result = &models.AgentRunWithDetails{
			AgentRun: models.AgentRun{
				ID:            r.ID,
				AgentID:       r.AgentID,
				UserID:        r.UserID,
				Task:          r.Task,
				FinalResponse: r.FinalResponse,
				StepsTaken:    r.StepsTaken,
				Status:        r.Status,
			},
			AgentName: r.AgentName,
			Username:  r.Username,
		}

		if r.ToolCalls.Valid {
			var toolCalls models.JSONArray
			if err := (&toolCalls).Scan(r.ToolCalls.String); err == nil {
				result.ToolCalls = &toolCalls
			}
		}

		if r.ExecutionSteps.Valid {
			var executionSteps models.JSONArray
			if err := (&executionSteps).Scan(r.ExecutionSteps.String); err == nil {
				result.ExecutionSteps = &executionSteps
			}
		}

		if r.StartedAt.Valid {
			result.StartedAt = r.StartedAt.Time
		}

		if r.CompletedAt.Valid {
			result.CompletedAt = &r.CompletedAt.Time
		}

		// Add token usage and metadata fields for ListRecentAgentRunsRow
		if r.InputTokens.Valid {
			inputTokens := r.InputTokens.Int64
			result.InputTokens = &inputTokens
		}

		if r.OutputTokens.Valid {
			outputTokens := r.OutputTokens.Int64
			result.OutputTokens = &outputTokens
		}

		if r.TotalTokens.Valid {
			totalTokens := r.TotalTokens.Int64
			result.TotalTokens = &totalTokens
		}

		if r.DurationSeconds.Valid {
			durationSeconds := r.DurationSeconds.Float64
			result.DurationSeconds = &durationSeconds
		}

		if r.ModelName.Valid {
			modelName := r.ModelName.String
			result.ModelName = &modelName
		}

		if r.ToolsUsed.Valid {
			toolsUsed := r.ToolsUsed.Int64
			result.ToolsUsed = &toolsUsed
		}

		if r.DebugLogs.Valid {
			var debugLogs models.JSONArray
			if err := (&debugLogs).Scan(r.DebugLogs.String); err == nil {
				result.DebugLogs = &debugLogs
			}
		}

		if r.Error.Valid {
			errorMsg := r.Error.String
			result.Error = &errorMsg
		}

	case queries.GetAgentRunWithDetailsRow:
		result = &models.AgentRunWithDetails{
			AgentRun: models.AgentRun{
				ID:            r.ID,
				AgentID:       r.AgentID,
				UserID:        r.UserID,
				Task:          r.Task,
				FinalResponse: r.FinalResponse,
				StepsTaken:    r.StepsTaken,
				Status:        r.Status,
			},
			AgentName: r.AgentName,
			Username:  r.Username,
		}

		if r.ToolCalls.Valid {
			var toolCalls models.JSONArray
			if err := (&toolCalls).Scan(r.ToolCalls.String); err == nil {
				result.ToolCalls = &toolCalls
			}
		}

		if r.ExecutionSteps.Valid {
			var executionSteps models.JSONArray
			if err := (&executionSteps).Scan(r.ExecutionSteps.String); err == nil {
				result.ExecutionSteps = &executionSteps
			}
		}

		if r.StartedAt.Valid {
			result.StartedAt = r.StartedAt.Time
		}

		if r.CompletedAt.Valid {
			result.CompletedAt = &r.CompletedAt.Time
		}

		// Add token usage and metadata fields for GetAgentRunWithDetailsRow
		if r.InputTokens.Valid {
			inputTokens := r.InputTokens.Int64
			result.InputTokens = &inputTokens
		}

		if r.OutputTokens.Valid {
			outputTokens := r.OutputTokens.Int64
			result.OutputTokens = &outputTokens
		}

		if r.TotalTokens.Valid {
			totalTokens := r.TotalTokens.Int64
			result.TotalTokens = &totalTokens
		}

		if r.DurationSeconds.Valid {
			durationSeconds := r.DurationSeconds.Float64
			result.DurationSeconds = &durationSeconds
		}

		if r.ModelName.Valid {
			modelName := r.ModelName.String
			result.ModelName = &modelName
		}

		if r.ToolsUsed.Valid {
			toolsUsed := r.ToolsUsed.Int64
			result.ToolsUsed = &toolsUsed
		}

		if r.DebugLogs.Valid {
			var debugLogs models.JSONArray
			if err := (&debugLogs).Scan(r.DebugLogs.String); err == nil {
				result.DebugLogs = &debugLogs
			}
		}
	}

	return result
}

// CreateWithMetadata creates a new agent run with response object metadata
func (r *AgentRunRepo) CreateWithMetadata(ctx context.Context, agentID, userID int64, task, finalResponse string, stepsTaken int64, toolCalls, executionSteps *models.JSONArray, status string, completedAt *time.Time, inputTokens, outputTokens, totalTokens *int64, durationSeconds *float64, modelName *string, toolsUsed *int64, parentRunID *int64) (*models.AgentRun, error) {
	params := queries.CreateAgentRunParams{
		AgentID:       agentID,
		UserID:        userID,
		Task:          task,
		FinalResponse: finalResponse,
		StepsTaken:    stepsTaken,
		Status:        status,
	}

	// Convert JSONArray to sql.NullString
	if toolCalls != nil {
		if jsonStr, err := toolCalls.Value(); err == nil {
			if strVal, ok := jsonStr.(string); ok {
				params.ToolCalls = sql.NullString{String: strVal, Valid: true}
			}
		}
	}

	if executionSteps != nil {
		if jsonStr, err := executionSteps.Value(); err == nil {
			if strVal, ok := jsonStr.(string); ok {
				params.ExecutionSteps = sql.NullString{String: strVal, Valid: true}
			}
		}
	}

	if completedAt != nil {
		params.CompletedAt = sql.NullTime{Time: *completedAt, Valid: true}
	}

	// Add response object metadata
	if inputTokens != nil {
		params.InputTokens = sql.NullInt64{Int64: *inputTokens, Valid: true}
	}

	if outputTokens != nil {
		params.OutputTokens = sql.NullInt64{Int64: *outputTokens, Valid: true}
	}

	if totalTokens != nil {
		params.TotalTokens = sql.NullInt64{Int64: *totalTokens, Valid: true}
	}

	if durationSeconds != nil {
		params.DurationSeconds = sql.NullFloat64{Float64: *durationSeconds, Valid: true}
	}

	if modelName != nil {
		params.ModelName = sql.NullString{String: *modelName, Valid: true}
	}

	if toolsUsed != nil {
		params.ToolsUsed = sql.NullInt64{Int64: *toolsUsed, Valid: true}
	}

	if parentRunID != nil {
		params.ParentRunID = sql.NullInt64{Int64: *parentRunID, Valid: true}
	}

	sqlcRun, err := r.queries.CreateAgentRun(ctx, params)
	if err != nil {
		return nil, err
	}

	return convertAgentRunFromSQLc(sqlcRun), nil
}

// Create creates a new agent run (backwards compatibility)
func (r *AgentRunRepo) Create(ctx context.Context, agentID, userID int64, task, finalResponse string, stepsTaken int64, toolCalls, executionSteps *models.JSONArray, status string, completedAt *time.Time) (*models.AgentRun, error) {
	ctx, span := r.tracer.Start(ctx, "db.agent_runs.create",
		trace.WithAttributes(
			attribute.Int64("agent.id", agentID),
			attribute.Int64("user.id", userID),
			attribute.String("run.status", status),
			attribute.Int64("run.steps_taken", stepsTaken),
		),
	)
	defer span.End()
	params := queries.CreateAgentRunBasicParams{
		AgentID:       agentID,
		UserID:        userID,
		Task:          task,
		FinalResponse: finalResponse,
		StepsTaken:    stepsTaken,
		Status:        status,
	}

	// Convert JSONArray to sql.NullString
	if toolCalls != nil {
		if jsonStr, err := toolCalls.Value(); err == nil {
			if strVal, ok := jsonStr.(string); ok {
				params.ToolCalls = sql.NullString{String: strVal, Valid: true}
			}
		}
	}

	if executionSteps != nil {
		if jsonStr, err := executionSteps.Value(); err == nil {
			if strVal, ok := jsonStr.(string); ok {
				params.ExecutionSteps = sql.NullString{String: strVal, Valid: true}
			}
		}
	}

	if completedAt != nil {
		params.CompletedAt = sql.NullTime{Time: *completedAt, Valid: true}
	}

	sqlcRun, err := r.queries.CreateAgentRunBasic(ctx, params)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("db.operation.success", false))
		return nil, err
	}

	span.SetAttributes(
		attribute.Bool("db.operation.success", true),
		attribute.Int64("run.id", sqlcRun.ID),
	)
	return convertAgentRunFromSQLc(sqlcRun), nil
}

func (r *AgentRunRepo) GetByID(ctx context.Context, id int64) (*models.AgentRun, error) {
	ctx, span := r.tracer.Start(ctx, "db.agent_runs.get_by_id",
		trace.WithAttributes(
			attribute.Int64("run.id", id),
		),
	)
	defer span.End()

	run, err := r.queries.GetAgentRun(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("db.operation.success", false))
		return nil, err
	}

	span.SetAttributes(
		attribute.Bool("db.operation.success", true),
		attribute.String("run.status", run.Status),
		attribute.Int64("run.agent_id", run.AgentID),
	)
	return convertAgentRunFromSQLc(run), nil
}

func (r *AgentRunRepo) GetByIDWithDetails(ctx context.Context, id int64) (*models.AgentRunWithDetails, error) {
	row, err := r.queries.GetAgentRunWithDetails(ctx, id)
	if err != nil {
		return nil, err
	}
	return convertAgentRunWithDetailsFromSQLc(row), nil
}

func (r *AgentRunRepo) List(ctx context.Context) ([]*models.AgentRun, error) {
	runs, err := r.queries.ListAgentRuns(ctx)
	if err != nil {
		return nil, err
	}

	var result []*models.AgentRun
	for _, run := range runs {
		result = append(result, convertAgentRunFromSQLc(run))
	}

	return result, nil
}

func (r *AgentRunRepo) ListByAgent(ctx context.Context, agentID int64) ([]*models.AgentRun, error) {
	runs, err := r.queries.ListAgentRunsByAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	var result []*models.AgentRun
	for _, run := range runs {
		result = append(result, convertAgentRunFromSQLc(run))
	}

	return result, nil
}

func (r *AgentRunRepo) ListByUser(ctx context.Context, userID int64) ([]*models.AgentRun, error) {
	runs, err := r.queries.ListAgentRunsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	var result []*models.AgentRun
	for _, run := range runs {
		result = append(result, convertAgentRunFromSQLc(run))
	}

	return result, nil
}

func (r *AgentRunRepo) ListRecent(ctx context.Context, limit int64) ([]*models.AgentRunWithDetails, error) {
	rows, err := r.queries.ListRecentAgentRuns(ctx, limit)
	if err != nil {
		return nil, err
	}

	var result []*models.AgentRunWithDetails
	for _, row := range rows {
		result = append(result, convertAgentRunWithDetailsFromSQLc(row))
	}

	return result, nil
}

// UpdateCompletionWithMetadata updates an existing run record with completion data and response metadata
func (r *AgentRunRepo) UpdateCompletionWithMetadata(ctx context.Context, id int64, finalResponse string, stepsTaken int64, toolCalls, executionSteps *models.JSONArray, status string, completedAt *time.Time, inputTokens, outputTokens, totalTokens *int64, durationSeconds *float64, modelName *string, toolsUsed *int64, errorMsg *string) error {
	ctx, span := r.tracer.Start(ctx, "db.agent_runs.update_completion",
		trace.WithAttributes(
			attribute.Int64("run.id", id),
			attribute.String("run.status", status),
			attribute.Int64("run.steps_taken", stepsTaken),
		),
	)
	defer span.End()
	params := queries.UpdateAgentRunCompletionParams{
		FinalResponse: finalResponse,
		StepsTaken:    stepsTaken,
		Status:        status,
		ID:            id,
	}

	// Convert JSONArray to sql.NullString
	if toolCalls != nil {
		if jsonStr, err := toolCalls.Value(); err == nil {
			if strVal, ok := jsonStr.(string); ok {
				params.ToolCalls = sql.NullString{String: strVal, Valid: true}
			} else if byteVal, ok := jsonStr.([]byte); ok {
				strVal := string(byteVal)
				params.ToolCalls = sql.NullString{String: strVal, Valid: true}
			} else {
			}
		} else {
		}
	} else {
	}

	if executionSteps != nil {
		if jsonStr, err := executionSteps.Value(); err == nil {
			if strVal, ok := jsonStr.(string); ok {
				params.ExecutionSteps = sql.NullString{String: strVal, Valid: true}
			} else if byteVal, ok := jsonStr.([]byte); ok {
				strVal := string(byteVal)
				params.ExecutionSteps = sql.NullString{String: strVal, Valid: true}
			} else {
			}
		} else {
		}
	} else {
	}

	if completedAt != nil {
		params.CompletedAt = sql.NullTime{Time: *completedAt, Valid: true}
	}

	// Add response object metadata
	if inputTokens != nil {
		params.InputTokens = sql.NullInt64{Int64: *inputTokens, Valid: true}
	}

	if outputTokens != nil {
		params.OutputTokens = sql.NullInt64{Int64: *outputTokens, Valid: true}
	}

	if totalTokens != nil {
		params.TotalTokens = sql.NullInt64{Int64: *totalTokens, Valid: true}
	}

	if durationSeconds != nil {
		params.DurationSeconds = sql.NullFloat64{Float64: *durationSeconds, Valid: true}
	}

	if modelName != nil {
		params.ModelName = sql.NullString{String: *modelName, Valid: true}
	}

	if toolsUsed != nil {
		params.ToolsUsed = sql.NullInt64{Int64: *toolsUsed, Valid: true}
	}

	if errorMsg != nil {
		params.Error = sql.NullString{String: *errorMsg, Valid: true}
	}

	err := r.queries.UpdateAgentRunCompletion(ctx, params)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("db.operation.success", false))
		return err
	}

	span.SetAttributes(
		attribute.Bool("db.operation.success", true),
	)
	return nil
}

// UpdateCompletion updates an existing run record (backwards compatibility)
func (r *AgentRunRepo) UpdateCompletion(ctx context.Context, id int64, finalResponse string, stepsTaken int64, toolCalls, executionSteps *models.JSONArray, status string, completedAt *time.Time) error {
	return r.UpdateCompletionWithMetadata(ctx, id, finalResponse, stepsTaken, toolCalls, executionSteps, status, completedAt, nil, nil, nil, nil, nil, nil, nil)
}

// UpdateStatus updates only the status of an agent run using SQLC
func (r *AgentRunRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	params := queries.UpdateAgentRunStatusParams{
		Status: status,
		ID:     id,
	}
	return r.queries.UpdateAgentRunStatus(ctx, params)
}

// UpdateDebugLogs updates the debug_logs field for an agent run
func (r *AgentRunRepo) UpdateDebugLogs(ctx context.Context, id int64, debugLogs *models.JSONArray) error {
	var debugLogsStr sql.NullString

	if debugLogs != nil {
		if jsonStr, err := debugLogs.Value(); err == nil {
			if strVal, ok := jsonStr.(string); ok {
				debugLogsStr = sql.NullString{String: strVal, Valid: true}
			} else if byteVal, ok := jsonStr.([]byte); ok {
				strVal := string(byteVal)
				debugLogsStr = sql.NullString{String: strVal, Valid: true}
			}
		}
	}

	params := queries.UpdateAgentRunDebugLogsParams{
		DebugLogs: debugLogsStr,
		ID:        id,
	}
	return r.queries.UpdateAgentRunDebugLogs(ctx, params)
}

// AppendDebugLog appends a new debug log entry to an existing agent run
func (r *AgentRunRepo) AppendDebugLog(ctx context.Context, id int64, logEntry map[string]interface{}) error {
	run, err := r.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get agent run: %w", err)
	}

	var debugLogs models.JSONArray
	if run.DebugLogs != nil {
		debugLogs = *run.DebugLogs
	} else {
		debugLogs = make(models.JSONArray, 0)
	}

	debugLogs = append(debugLogs, logEntry)
	return r.UpdateDebugLogs(ctx, id, &debugLogs)
}
