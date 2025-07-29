package repositories

import (
	"database/sql"
	"station/pkg/models"
	"time"
)

type AgentRunRepo struct {
	db *sql.DB
}

func NewAgentRunRepo(db *sql.DB) *AgentRunRepo {
	return &AgentRunRepo{db: db}
}

func (r *AgentRunRepo) Create(agentID, userID int64, task, finalResponse string, stepsTaken int64, toolCalls, executionSteps *models.JSONArray, status string, completedAt *time.Time) (*models.AgentRun, error) {
	query := `INSERT INTO agent_runs (agent_id, user_id, task, final_response, steps_taken, tool_calls, execution_steps, status, completed_at) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) 
			  RETURNING id, agent_id, user_id, task, final_response, steps_taken, tool_calls, execution_steps, status, started_at, completed_at`
	
	var run models.AgentRun
	err := r.db.QueryRow(query, agentID, userID, task, finalResponse, stepsTaken, toolCalls, executionSteps, status, completedAt).Scan(
		&run.ID, &run.AgentID, &run.UserID, &run.Task, &run.FinalResponse, &run.StepsTaken,
		&run.ToolCalls, &run.ExecutionSteps, &run.Status, &run.StartedAt, &run.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &run, nil
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