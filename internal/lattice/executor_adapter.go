package lattice

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"station/internal/db"
	"station/internal/db/queries"
	"station/internal/db/repositories"
	"station/internal/lattice/work"
	"station/internal/services"
)

// ExecutorAdapter implements the AgentExecutor interface for lattice remote invocation.
type ExecutorAdapter struct {
	agentService services.AgentServiceInterface
	repos        *repositories.Repositories
	db           *sql.DB
}

func NewExecutorAdapter(agentService services.AgentServiceInterface, repos *repositories.Repositories, db *sql.DB) *ExecutorAdapter {
	return &ExecutorAdapter{
		agentService: agentService,
		repos:        repos,
		db:           db,
	}
}

func (e *ExecutorAdapter) ExecuteAgentByID(ctx context.Context, agentID string, task string) (string, int, error) {
	id, err := strconv.ParseInt(agentID, 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid agent ID '%s': %w", agentID, err)
	}

	userID := int64(1)
	agentRun, err := e.repos.AgentRuns.Create(ctx, id, userID, task, "", 0, nil, nil, "running", nil)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create agent run: %w", err)
	}

	result, err := e.agentService.ExecuteAgentWithRunID(ctx, id, task, agentRun.ID, nil)
	if err != nil {
		completedAt := time.Now()
		errorMsg := err.Error()
		e.repos.AgentRuns.UpdateCompletionWithMetadata(
			ctx, agentRun.ID, errorMsg, 0, nil, nil, "failed", &completedAt,
			nil, nil, nil, nil, nil, nil, &errorMsg,
		)
		return "", 0, fmt.Errorf("agent execution failed: %w", err)
	}

	completedAt := time.Now()
	var stepsTaken int64
	if result.Extra != nil {
		if steps, ok := result.Extra["steps_taken"].(int64); ok {
			stepsTaken = steps
		} else if steps, ok := result.Extra["steps_taken"].(float64); ok {
			stepsTaken = int64(steps)
		}
	}

	e.repos.AgentRuns.UpdateCompletionWithMetadata(
		ctx, agentRun.ID, result.Content, stepsTaken, nil, nil, "completed", &completedAt,
		nil, nil, nil, nil, nil, nil, nil,
	)

	toolCalls := 0
	if result.Extra != nil {
		if tc, ok := result.Extra["tools_used"].(int); ok {
			toolCalls = tc
		} else if tc, ok := result.Extra["tools_used"].(float64); ok {
			toolCalls = int(tc)
		}
	}

	return result.Content, toolCalls, nil
}

func (e *ExecutorAdapter) ExecuteAgentByName(ctx context.Context, agentName string, task string) (string, int, error) {
	q := queries.New(e.db)
	agent, err := q.GetAgentByNameGlobal(ctx, agentName)
	if err != nil {
		return "", 0, fmt.Errorf("agent '%s' not found: %w", agentName, err)
	}

	return e.ExecuteAgentByID(ctx, strconv.FormatInt(agent.ID, 10), task)
}

func (e *ExecutorAdapter) ExecuteAgentByIDWithContext(ctx context.Context, agentID string, task string, orchCtx *work.OrchestratorContext) (string, int64, int, error) {
	id, err := strconv.ParseInt(agentID, 10, 64)
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid agent ID '%s': %w", agentID, err)
	}

	userID := int64(1)

	db.SQLiteWriteMutex.Lock()
	q := queries.New(e.db)
	agentRun, err := q.CreateAgentRunWithOrchestratorContext(ctx, queries.CreateAgentRunWithOrchestratorContextParams{
		AgentID:                 id,
		UserID:                  userID,
		Task:                    task,
		FinalResponse:           "",
		StepsTaken:              0,
		Status:                  "running",
		OrchestratorRunID:       sql.NullString{String: orchCtx.RunID, Valid: orchCtx.RunID != ""},
		ParentOrchestratorRunID: sql.NullString{String: orchCtx.ParentRunID, Valid: orchCtx.ParentRunID != ""},
		OriginatingStationID:    sql.NullString{String: orchCtx.OriginatingStation, Valid: orchCtx.OriginatingStation != ""},
		TraceID:                 sql.NullString{String: orchCtx.TraceID, Valid: orchCtx.TraceID != ""},
		WorkID:                  sql.NullString{String: orchCtx.WorkID, Valid: orchCtx.WorkID != ""},
	})
	db.SQLiteWriteMutex.Unlock()

	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to create agent run: %w", err)
	}

	result, err := e.agentService.ExecuteAgentWithRunID(ctx, id, task, agentRun.ID, nil)
	if err != nil {
		completedAt := time.Now()
		errorMsg := err.Error()
		e.repos.AgentRuns.UpdateCompletionWithMetadata(
			ctx, agentRun.ID, errorMsg, 0, nil, nil, "failed", &completedAt,
			nil, nil, nil, nil, nil, nil, &errorMsg,
		)
		return "", agentRun.ID, 0, fmt.Errorf("agent execution failed: %w", err)
	}

	completedAt := time.Now()
	var stepsTaken int64
	if result.Extra != nil {
		if steps, ok := result.Extra["steps_taken"].(int64); ok {
			stepsTaken = steps
		} else if steps, ok := result.Extra["steps_taken"].(float64); ok {
			stepsTaken = int64(steps)
		}
	}

	e.repos.AgentRuns.UpdateCompletionWithMetadata(
		ctx, agentRun.ID, result.Content, stepsTaken, nil, nil, "completed", &completedAt,
		nil, nil, nil, nil, nil, nil, nil,
	)

	toolCalls := 0
	if result.Extra != nil {
		if tc, ok := result.Extra["tools_used"].(int); ok {
			toolCalls = tc
		} else if tc, ok := result.Extra["tools_used"].(float64); ok {
			toolCalls = int(tc)
		}
	}

	return result.Content, agentRun.ID, toolCalls, nil
}

func (e *ExecutorAdapter) ExecuteAgentByNameWithContext(ctx context.Context, agentName string, task string, orchCtx *work.OrchestratorContext) (string, int64, int, error) {
	q := queries.New(e.db)
	agent, err := q.GetAgentByNameGlobal(ctx, agentName)
	if err != nil {
		return "", 0, 0, fmt.Errorf("agent '%s' not found: %w", agentName, err)
	}

	return e.ExecuteAgentByIDWithContext(ctx, strconv.FormatInt(agent.ID, 10), task, orchCtx)
}

// WorkflowExecutorAdapter implements workflow execution for lattice remote invocation.
type WorkflowExecutorAdapter struct {
	workflowService *services.WorkflowService
	repos           *repositories.Repositories
}

func NewWorkflowExecutorAdapter(workflowService *services.WorkflowService, repos *repositories.Repositories) *WorkflowExecutorAdapter {
	return &WorkflowExecutorAdapter{
		workflowService: workflowService,
		repos:           repos,
	}
}

func (w *WorkflowExecutorAdapter) ExecuteWorkflow(ctx context.Context, workflowID string, input map[string]string) (string, string, error) {
	if w.workflowService == nil {
		return "", "", fmt.Errorf("workflow service not available")
	}

	inputMap := make(map[string]interface{})
	for k, v := range input {
		inputMap[k] = v
	}

	inputJSON, err := json.Marshal(inputMap)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal input: %w", err)
	}

	req := services.StartWorkflowRunRequest{
		WorkflowID: workflowID,
		Version:    0,
		Input:      inputJSON,
	}

	run, _, err := w.workflowService.StartRun(ctx, req)
	if err != nil {
		return "", "", fmt.Errorf("failed to start workflow: %w", err)
	}

	return run.RunID, "started", nil
}
