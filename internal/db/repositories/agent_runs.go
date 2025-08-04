package repositories

import (
	"context"
	"database/sql"
	"station/internal/db/queries"
	"station/pkg/models"
	"time"
)

type AgentRunRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewAgentRunRepo(db *sql.DB) *AgentRunRepo {
	return &AgentRunRepo{
		db:      db,
		queries: queries.New(db),
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
	}
	
	return result
}

func (r *AgentRunRepo) Create(agentID, userID int64, task, finalResponse string, stepsTaken int64, toolCalls, executionSteps *models.JSONArray, status string, completedAt *time.Time) (*models.AgentRun, error) {
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
	
	sqlcRun, err := r.queries.CreateAgentRun(context.Background(), params)
	if err != nil {
		return nil, err
	}
	
	return convertAgentRunFromSQLc(sqlcRun), nil
}

func (r *AgentRunRepo) GetByID(id int64) (*models.AgentRun, error) {
	run, err := r.queries.GetAgentRun(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return convertAgentRunFromSQLc(run), nil
}

func (r *AgentRunRepo) GetByIDWithDetails(id int64) (*models.AgentRunWithDetails, error) {
	row, err := r.queries.GetAgentRunWithDetails(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return convertAgentRunWithDetailsFromSQLc(row), nil
}

func (r *AgentRunRepo) List() ([]*models.AgentRun, error) {
	runs, err := r.queries.ListAgentRuns(context.Background())
	if err != nil {
		return nil, err
	}
	
	var result []*models.AgentRun
	for _, run := range runs {
		result = append(result, convertAgentRunFromSQLc(run))
	}
	
	return result, nil
}

func (r *AgentRunRepo) ListByAgent(agentID int64) ([]*models.AgentRun, error) {
	runs, err := r.queries.ListAgentRunsByAgent(context.Background(), agentID)
	if err != nil {
		return nil, err
	}
	
	var result []*models.AgentRun
	for _, run := range runs {
		result = append(result, convertAgentRunFromSQLc(run))
	}
	
	return result, nil
}

func (r *AgentRunRepo) ListByUser(userID int64) ([]*models.AgentRun, error) {
	runs, err := r.queries.ListAgentRunsByUser(context.Background(), userID)
	if err != nil {
		return nil, err
	}
	
	var result []*models.AgentRun
	for _, run := range runs {
		result = append(result, convertAgentRunFromSQLc(run))
	}
	
	return result, nil
}

func (r *AgentRunRepo) ListRecent(limit int64) ([]*models.AgentRunWithDetails, error) {
	rows, err := r.queries.ListRecentAgentRuns(context.Background(), limit)
	if err != nil {
		return nil, err
	}
	
	var result []*models.AgentRunWithDetails
	for _, row := range rows {
		result = append(result, convertAgentRunWithDetailsFromSQLc(row))
	}
	
	return result, nil
}

// UpdateCompletion updates an existing run record with completion data using SQLC
func (r *AgentRunRepo) UpdateCompletion(id int64, finalResponse string, stepsTaken int64, toolCalls, executionSteps *models.JSONArray, status string, completedAt *time.Time) error {
	params := queries.UpdateAgentRunCompletionParams{
		FinalResponse:  finalResponse,
		StepsTaken:     stepsTaken,
		Status:         status,
		ID:             id,
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
	
	return r.queries.UpdateAgentRunCompletion(context.Background(), params)
}

// UpdateStatus updates only the status of an agent run using SQLC
func (r *AgentRunRepo) UpdateStatus(id int64, status string) error {
	params := queries.UpdateAgentRunStatusParams{
		Status: status,
		ID:     id,
	}
	return r.queries.UpdateAgentRunStatus(context.Background(), params)
}