package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"station/internal/db/queries"
	"station/pkg/models"
)

type AgentToolRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewAgentToolRepo(db *sql.DB) *AgentToolRepo {
	return &AgentToolRepo{
		db:      db,
		queries: queries.New(db),
	}
}

// convertAgentToolFromSQLc converts sqlc AgentTool to models.AgentTool
func convertAgentToolFromSQLc(agentTool queries.AgentTool) *models.AgentTool {
	result := &models.AgentTool{
		ID:      agentTool.ID,
		AgentID: agentTool.AgentID,
		ToolID:  agentTool.ToolID,
	}

	if agentTool.CreatedAt.Valid {
		result.CreatedAt = agentTool.CreatedAt.Time
	}

	return result
}

// convertAgentToolWithDetailsFromSQLc converts sqlc ListAgentToolsRow to models.AgentToolWithDetails
func convertAgentToolWithDetailsFromSQLc(row queries.ListAgentToolsRow) *models.AgentToolWithDetails {
	result := &models.AgentToolWithDetails{
		AgentTool: models.AgentTool{
			ID:      row.ID,
			AgentID: row.AgentID,
			ToolID:  row.ToolID,
		},
		ToolName:        row.ToolName,
		ServerName:      row.ServerName,
		EnvironmentID:   row.EnvironmentID,
		EnvironmentName: row.EnvironmentName,
	}

	if row.CreatedAt.Valid {
		result.CreatedAt = row.CreatedAt.Time
	}

	if row.ToolDescription.Valid {
		result.ToolDescription = row.ToolDescription.String
	}

	if row.ToolSchema.Valid {
		result.ToolSchema = row.ToolSchema.String
	}

	return result
}

// AddAgentTool creates a new agent-tool assignment using tool ID
// Enforces a maximum of 40 tools per agent for performance reasons
func (r *AgentToolRepo) AddAgentTool(agentID int64, toolID int64) (*models.AgentTool, error) {
	// Check current tool count for this agent
	currentTools, err := r.ListAgentTools(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to check current tool count: %w", err)
	}

	// Enforce 40-tool limit per agent
	const MaxToolsPerAgent = 40
	if len(currentTools) >= MaxToolsPerAgent {
		return nil, fmt.Errorf("agent already has maximum allowed tools (%d). Cannot add more tools to prevent performance issues", MaxToolsPerAgent)
	}

	params := queries.AddAgentToolParams{
		AgentID: agentID,
		ToolID:  toolID,
	}

	created, err := r.queries.AddAgentTool(context.Background(), params)
	if err != nil {
		return nil, err
	}

	return convertAgentToolFromSQLc(created), nil
}

// RemoveAgentTool removes a specific agent-tool assignment
func (r *AgentToolRepo) RemoveAgentTool(agentID int64, toolID int64) error {
	params := queries.RemoveAgentToolParams{
		AgentID: agentID,
		ToolID:  toolID,
	}
	return r.queries.RemoveAgentTool(context.Background(), params)
}

// ListAgentTools returns all tools assigned to an agent with details (environment filtered)
func (r *AgentToolRepo) ListAgentTools(agentID int64) ([]*models.AgentToolWithDetails, error) {
	rows, err := r.queries.ListAgentTools(context.Background(), agentID)
	if err != nil {
		return nil, err
	}

	var result []*models.AgentToolWithDetails
	for _, row := range rows {
		result = append(result, convertAgentToolWithDetailsFromSQLc(row))
	}

	return result, nil
}

// ListAvailableToolsForAgent returns tools available in the agent's environment that aren't assigned
func (r *AgentToolRepo) ListAvailableToolsForAgent(agentID int64, agentIDParam int64) ([]*models.AgentToolWithDetails, error) {
	params := queries.ListAvailableToolsForAgentParams{
		ID:      agentID,
		AgentID: agentIDParam,
	}

	rows, err := r.queries.ListAvailableToolsForAgent(context.Background(), params)
	if err != nil {
		return nil, err
	}

	var result []*models.AgentToolWithDetails
	for _, row := range rows {
		// Convert ListAvailableToolsForAgentRow to AgentToolWithDetails
		tool := &models.AgentToolWithDetails{
			ToolName:   row.ToolName,
			ServerName: row.ServerName,
		}

		if row.ToolDescription.Valid {
			tool.ToolDescription = row.ToolDescription.String
		}

		if row.ToolSchema.Valid {
			tool.ToolSchema = row.ToolSchema.String
		}

		result = append(result, tool)
	}

	return result, nil
}

// Clear removes all tool assignments for an agent
func (r *AgentToolRepo) Clear(agentID int64) error {
	return r.queries.ClearAgentTools(context.Background(), agentID)
}

// AddAgentToolTx adds a tool to agent within a transaction
// Note: Does NOT enforce 40-tool limit since that requires a separate query
// The limit should be checked before calling this method within the transaction
func (r *AgentToolRepo) AddAgentToolTx(tx *sql.Tx, agentID int64, toolID int64) (*models.AgentTool, error) {
	params := queries.AddAgentToolParams{
		AgentID: agentID,
		ToolID:  toolID,
	}

	txQueries := r.queries.WithTx(tx)
	created, err := txQueries.AddAgentTool(context.Background(), params)
	if err != nil {
		return nil, err
	}

	return convertAgentToolFromSQLc(created), nil
}

// ClearTx removes all tool assignments for an agent within a transaction
func (r *AgentToolRepo) ClearTx(tx *sql.Tx, agentID int64) error {
	txQueries := r.queries.WithTx(tx)
	return txQueries.ClearAgentTools(context.Background(), agentID)
}
