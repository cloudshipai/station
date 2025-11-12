package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"station/internal/db/queries"
	"station/pkg/models"
)

type AgentAgentRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewAgentAgentRepo(db *sql.DB) *AgentAgentRepo {
	return &AgentAgentRepo{
		db:      db,
		queries: queries.New(db),
	}
}

// convertAgentAgentFromSQLc converts sqlc AgentAgent to models.AgentAgent
func convertAgentAgentFromSQLc(aa queries.AgentAgent) *models.AgentAgent {
	result := &models.AgentAgent{
		ID:            aa.ID,
		ParentAgentID: aa.ParentAgentID,
		ChildAgentID:  aa.ChildAgentID,
	}

	if aa.CreatedAt.Valid {
		result.CreatedAt = aa.CreatedAt.Time
	}

	return result
}

// convertChildAgentFromSQLc converts sqlc ListChildAgentsRow to models.ChildAgent
func convertChildAgentFromSQLc(row queries.ListChildAgentsRow) *models.ChildAgent {
	result := &models.ChildAgent{
		RelationshipID: row.ID,
		ParentAgentID:  row.ParentAgentID,
		ChildAgentID:   row.ChildAgentID,
		ChildAgent: models.Agent{
			ID:            row.ChildID,
			Name:          row.ChildName,
			Description:   row.ChildDescription,
			EnvironmentID: row.EnvironmentID,
		},
	}

	if row.CreatedAt.Valid {
		result.CreatedAt = row.CreatedAt.Time
	}

	return result
}

// AddChildAgent adds a child agent relationship
func (r *AgentAgentRepo) AddChildAgent(parentAgentID, childAgentID int64) (*models.AgentAgent, error) {
	result, err := r.queries.AddChildAgent(context.Background(), queries.AddChildAgentParams{
		ParentAgentID: parentAgentID,
		ChildAgentID:  childAgentID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add child agent: %w", err)
	}

	return convertAgentAgentFromSQLc(result), nil
}

// RemoveChildAgent removes a child agent relationship
func (r *AgentAgentRepo) RemoveChildAgent(parentAgentID, childAgentID int64) error {
	err := r.queries.RemoveChildAgent(context.Background(), queries.RemoveChildAgentParams{
		ParentAgentID: parentAgentID,
		ChildAgentID:  childAgentID,
	})
	if err != nil {
		return fmt.Errorf("failed to remove child agent: %w", err)
	}

	return nil
}

// ListChildAgents lists all child agents for a parent agent
func (r *AgentAgentRepo) ListChildAgents(parentAgentID int64) ([]*models.ChildAgent, error) {
	rows, err := r.queries.ListChildAgents(context.Background(), parentAgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list child agents: %w", err)
	}

	var result []*models.ChildAgent
	for _, row := range rows {
		result = append(result, convertChildAgentFromSQLc(row))
	}

	return result, nil
}

// ListParentAgents lists all parent agents for a child agent
func (r *AgentAgentRepo) ListParentAgents(childAgentID int64) ([]*models.ChildAgent, error) {
	rows, err := r.queries.ListParentAgents(context.Background(), childAgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list parent agents: %w", err)
	}

	var result []*models.ChildAgent
	for _, row := range rows {
		// Reuse ChildAgent model but swap parent/child semantics
		result = append(result, &models.ChildAgent{
			RelationshipID: row.ID,
			ParentAgentID:  row.ParentAgentID,
			ChildAgentID:   row.ChildAgentID,
			ChildAgent: models.Agent{
				ID:            row.ParentID,
				Name:          row.ParentName,
				Description:   row.ParentDescription,
				EnvironmentID: row.EnvironmentID,
			},
		})
	}

	return result, nil
}

// GetChildAgentRelationship gets a specific relationship
func (r *AgentAgentRepo) GetChildAgentRelationship(parentAgentID, childAgentID int64) (*models.AgentAgent, error) {
	result, err := r.queries.GetChildAgentRelationship(context.Background(), queries.GetChildAgentRelationshipParams{
		ParentAgentID: parentAgentID,
		ChildAgentID:  childAgentID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get child agent relationship: %w", err)
	}

	return convertAgentAgentFromSQLc(result), nil
}

// DeleteAllChildAgentsForAgent deletes all child relationships for a parent agent
func (r *AgentAgentRepo) DeleteAllChildAgentsForAgent(parentAgentID int64) error {
	err := r.queries.DeleteAllChildAgentsForAgent(context.Background(), parentAgentID)
	if err != nil {
		return fmt.Errorf("failed to delete all child agents: %w", err)
	}

	return nil
}
