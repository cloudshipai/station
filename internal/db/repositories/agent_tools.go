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
		ID:      agentTool.ID,
		AgentID: agentTool.AgentID,
		ToolID:  agentTool.ToolID,
	}
	
	if agentTool.CreatedAt.Valid {
		result.CreatedAt = agentTool.CreatedAt.Time
	}
	
	return result
}

// Add creates a new agent-tool assignment 
func (r *AgentToolRepo) Add(agentID, toolID int64) (*models.AgentTool, error) {
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

// Remove removes a specific agent-tool assignment
func (r *AgentToolRepo) Remove(agentID, toolID int64) error {
	params := queries.RemoveAgentToolParams{
		AgentID: agentID,
		ToolID:  toolID,
	}
	return r.queries.RemoveAgentTool(context.Background(), params)
}

// List returns all tools assigned to an agent with details
func (r *AgentToolRepo) List(agentID int64) ([]queries.ListAgentToolsRow, error) {
	return r.queries.ListAgentTools(context.Background(), agentID)
}

// Clear removes all tool assignments for an agent
func (r *AgentToolRepo) Clear(agentID int64) error {
	return r.queries.ClearAgentTools(context.Background(), agentID)
}