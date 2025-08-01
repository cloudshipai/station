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

// convertAgentRunWithDetailsFromSQLc converts sqlc ListRecentAgentRunsRow to models.AgentRunWithDetails
func convertAgentRunWithDetailsFromSQLc(row queries.ListRecentAgentRunsRow) *models.AgentRunWithDetails {
	baseRun := queries.AgentRun{
		ID:             row.ID,
		AgentID:        row.AgentID,
		UserID:         row.UserID,
		Task:           row.Task,
		FinalResponse:  row.FinalResponse,
		StepsTaken:     row.StepsTaken,
		ToolCalls:      row.ToolCalls,
		ExecutionSteps: row.ExecutionSteps,
		Status:         row.Status,
		StartedAt:      row.StartedAt,
		CompletedAt:    row.CompletedAt,
	}
	
	result := &models.AgentRunWithDetails{
		AgentRun:  *convertAgentRunFromSQLc(baseRun),
		AgentName: row.AgentName,
		Username:  row.Username,
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
	query := `SELECT id, agent_id, user_id, task, final_response, steps_taken, tool_calls, execution_steps, status, started_at, completed_at 
			  FROM agent_runs WHERE id = ?`
	
	var run models.AgentRun
	err := r.db.QueryRow(query, id).Scan(
		&run.ID, &run.AgentID, &run.UserID, &run.Task, &run.FinalResponse, &run.StepsTaken,
		&run.ToolCalls, &run.ExecutionSteps, &run.Status, &run.StartedAt, &run.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &run, nil
}

func (r *AgentRunRepo) GetByIDWithDetails(id int64) (*models.AgentRunWithDetails, error) {
	query := `SELECT ar.id, ar.agent_id, ar.user_id, ar.task, ar.final_response, ar.steps_taken, 
					 ar.tool_calls, ar.execution_steps, ar.status, ar.started_at, ar.completed_at,
					 a.name as agent_name, u.username
			  FROM agent_runs ar
			  JOIN agents a ON ar.agent_id = a.id
			  JOIN users u ON ar.user_id = u.id
			  WHERE ar.id = ?`
	
	var run models.AgentRunWithDetails
	err := r.db.QueryRow(query, id).Scan(
		&run.ID, &run.AgentID, &run.UserID, &run.Task, &run.FinalResponse, &run.StepsTaken,
		&run.ToolCalls, &run.ExecutionSteps, &run.Status, &run.StartedAt, &run.CompletedAt,
		&run.AgentName, &run.Username,
	)
	if err != nil {
		return nil, err
	}
	
	return &run, nil
}

func (r *AgentRunRepo) List() ([]*models.AgentRun, error) {
	query := `SELECT id, agent_id, user_id, task, final_response, steps_taken, tool_calls, execution_steps, status, started_at, completed_at 
			  FROM agent_runs ORDER BY started_at DESC`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var runs []*models.AgentRun
	for rows.Next() {
		var run models.AgentRun
		err := rows.Scan(&run.ID, &run.AgentID, &run.UserID, &run.Task, &run.FinalResponse, &run.StepsTaken,
			&run.ToolCalls, &run.ExecutionSteps, &run.Status, &run.StartedAt, &run.CompletedAt)
		if err != nil {
			return nil, err
		}
		runs = append(runs, &run)
	}
	
	return runs, rows.Err()
}

func (r *AgentRunRepo) ListByAgent(agentID int64) ([]*models.AgentRun, error) {
	query := `SELECT id, agent_id, user_id, task, final_response, steps_taken, tool_calls, execution_steps, status, started_at, completed_at 
			  FROM agent_runs WHERE agent_id = ? ORDER BY started_at DESC`
	
	rows, err := r.db.Query(query, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var runs []*models.AgentRun
	for rows.Next() {
		var run models.AgentRun
		err := rows.Scan(&run.ID, &run.AgentID, &run.UserID, &run.Task, &run.FinalResponse, &run.StepsTaken,
			&run.ToolCalls, &run.ExecutionSteps, &run.Status, &run.StartedAt, &run.CompletedAt)
		if err != nil {
			return nil, err
		}
		runs = append(runs, &run)
	}
	
	return runs, rows.Err()
}

func (r *AgentRunRepo) ListByUser(userID int64) ([]*models.AgentRun, error) {
	query := `SELECT id, agent_id, user_id, task, final_response, steps_taken, tool_calls, execution_steps, status, started_at, completed_at 
			  FROM agent_runs WHERE user_id = ? ORDER BY started_at DESC`
	
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var runs []*models.AgentRun
	for rows.Next() {
		var run models.AgentRun
		err := rows.Scan(&run.ID, &run.AgentID, &run.UserID, &run.Task, &run.FinalResponse, &run.StepsTaken,
			&run.ToolCalls, &run.ExecutionSteps, &run.Status, &run.StartedAt, &run.CompletedAt)
		if err != nil {
			return nil, err
		}
		runs = append(runs, &run)
	}
	
	return runs, rows.Err()
}

func (r *AgentRunRepo) ListRecent(limit int64) ([]*models.AgentRunWithDetails, error) {
	query := `SELECT ar.id, ar.agent_id, ar.user_id, ar.task, ar.final_response, ar.steps_taken, 
					 ar.tool_calls, ar.execution_steps, ar.status, ar.started_at, ar.completed_at,
					 a.name as agent_name, u.username
			  FROM agent_runs ar
			  JOIN agents a ON ar.agent_id = a.id
			  JOIN users u ON ar.user_id = u.id
			  ORDER BY ar.started_at DESC
			  LIMIT ?`
	
	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var runs []*models.AgentRunWithDetails
	for rows.Next() {
		var run models.AgentRunWithDetails
		err := rows.Scan(&run.ID, &run.AgentID, &run.UserID, &run.Task, &run.FinalResponse, &run.StepsTaken,
			&run.ToolCalls, &run.ExecutionSteps, &run.Status, &run.StartedAt, &run.CompletedAt,
			&run.AgentName, &run.Username)
		if err != nil {
			return nil, err
		}
		runs = append(runs, &run)
	}
	
	return runs, rows.Err()
}

// UpdateCompletion updates an existing run record with completion data
func (r *AgentRunRepo) UpdateCompletion(id int64, finalResponse string, stepsTaken int64, toolCalls, executionSteps *models.JSONArray, status string, completedAt *time.Time) error {
	query := `UPDATE agent_runs 
			  SET final_response = ?, steps_taken = ?, tool_calls = ?, execution_steps = ?, status = ?, completed_at = ?
			  WHERE id = ?`
	
	_, err := r.db.Exec(query, finalResponse, stepsTaken, toolCalls, executionSteps, status, completedAt, id)
	return err
}

// UpdateStatus updates only the status of an agent run
func (r *AgentRunRepo) UpdateStatus(id int64, status string) error {
	query := `UPDATE agent_runs SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := r.db.Exec(query, status, id)
	return err
}