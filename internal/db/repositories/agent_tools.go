package repositories

import (
	"context"
	"database/sql"
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
		ID:            agentTool.ID,
		AgentID:       agentTool.AgentID,
		ToolName:      agentTool.ToolName,
		EnvironmentID: agentTool.EnvironmentID,
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
			ID:            row.ID,
			AgentID:       row.AgentID,
			ToolName:      row.ToolName,
			EnvironmentID: row.EnvironmentID,
		},
		ToolName:      row.ToolName,
		ServerName:    row.ServerName,
		EnvironmentID: row.EnvironmentID,
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

// Add creates a new agent-tool assignment 
func (r *AgentToolRepo) Add(agentID int64, toolName string, environmentID int64) (*models.AgentTool, error) {
	params := queries.AddAgentToolParams{
		AgentID:       agentID,
		ToolName:      toolName,
		EnvironmentID: environmentID,
	}
	
	created, err := r.queries.AddAgentTool(context.Background(), params)
	if err != nil {
		return nil, err
	}
	
	return convertAgentToolFromSQLc(created), nil
}

// Remove removes a specific agent-tool assignment
func (r *AgentToolRepo) Remove(agentID int64, toolName string, environmentID int64) error {
	params := queries.RemoveAgentToolParams{
		AgentID:       agentID,
		ToolName:      toolName,
		EnvironmentID: environmentID,
	}
	return r.queries.RemoveAgentTool(context.Background(), params)
}

// List returns all tools assigned to an agent with details
func (r *AgentToolRepo) List(agentID int64) ([]*models.AgentToolWithDetails, error) {
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

// Clear removes all tool assignments for an agent
func (r *AgentToolRepo) Clear(agentID int64) error {
	return r.queries.ClearAgentTools(context.Background(), agentID)
}