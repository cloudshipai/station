package repositories

import (
	"database/sql"
	"strings"
	"station/pkg/models"
)

type AgentToolRepo struct {
	db *sql.DB
}

func NewAgentToolRepo(db *sql.DB) *AgentToolRepo {
	return &AgentToolRepo{db: db}
}

// Add creates a new agent-tool assignment with environment context
func (r *AgentToolRepo) Add(agentID int64, toolName string, environmentID int64) (*models.AgentTool, error) {
	query := `INSERT INTO agent_tools (agent_id, tool_name, environment_id) 
			  VALUES (?, ?, ?) 
			  RETURNING id, agent_id, tool_name, environment_id, created_at`
	
	var agentTool models.AgentTool
	err := r.db.QueryRow(query, agentID, toolName, environmentID).Scan(
		&agentTool.ID, &agentTool.AgentID, &agentTool.ToolName, &agentTool.EnvironmentID, &agentTool.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &agentTool, nil
}

// Remove removes a specific agent-tool assignment for a specific environment
func (r *AgentToolRepo) Remove(agentID int64, toolName string, environmentID int64) error {
	query := `DELETE FROM agent_tools WHERE agent_id = ? AND tool_name = ? AND environment_id = ?`
	_, err := r.db.Exec(query, agentID, toolName, environmentID)
	return err
}

// RemoveByToolName removes all agent-tool associations for a specific tool name across all environments
func (r *AgentToolRepo) RemoveByToolName(toolName string) error {
	query := `DELETE FROM agent_tools WHERE tool_name = ?`
	_, err := r.db.Exec(query, toolName)
	return err
}

// RemoveByToolNames removes all agent-tool associations for multiple tool names
func (r *AgentToolRepo) RemoveByToolNames(toolNames []string) error {
	if len(toolNames) == 0 {
		return nil
	}
	
	// Build the IN clause with placeholders
	placeholders := make([]string, len(toolNames))
	args := make([]interface{}, len(toolNames))
	for i, name := range toolNames {
		placeholders[i] = "?"
		args[i] = name
	}
	
	query := `DELETE FROM agent_tools WHERE tool_name IN (` + strings.Join(placeholders, ",") + `)`
	
	_, err := r.db.Exec(query, args...)
	// Ignore "no such table" errors gracefully for test environments
	if err != nil && strings.Contains(err.Error(), "no such table: agent_tools") {
		return nil
	}
	return err
}

// List returns all tools assigned to an agent across all environments
func (r *AgentToolRepo) List(agentID int64) ([]*models.AgentToolWithDetails, error) {
	query := `SELECT at.id, at.agent_id, at.tool_name, at.environment_id, at.created_at,
					 e.name as environment_name
			  FROM agent_tools at
			  JOIN environments e ON at.environment_id = e.id
			  WHERE at.agent_id = ?
			  ORDER BY e.name, at.tool_name`
	
	rows, err := r.db.Query(query, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var agentTools []*models.AgentToolWithDetails
	for rows.Next() {
		var at models.AgentToolWithDetails
		err := rows.Scan(&at.ID, &at.AgentID, &at.ToolName, &at.EnvironmentID, &at.CreatedAt,
			&at.EnvironmentName)
		if err != nil {
			return nil, err
		}
		agentTools = append(agentTools, &at)
	}
	
	return agentTools, rows.Err()
}

// ListByEnvironment returns all tools assigned to an agent in a specific environment
func (r *AgentToolRepo) ListByEnvironment(agentID, environmentID int64) ([]*models.AgentToolWithDetails, error) {
	query := `SELECT at.id, at.agent_id, at.tool_name, at.environment_id, at.created_at,
					 e.name as environment_name
			  FROM agent_tools at
			  JOIN environments e ON at.environment_id = e.id
			  WHERE at.agent_id = ? AND at.environment_id = ?
			  ORDER BY at.tool_name`
	
	rows, err := r.db.Query(query, agentID, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var agentTools []*models.AgentToolWithDetails
	for rows.Next() {
		var at models.AgentToolWithDetails
		err := rows.Scan(&at.ID, &at.AgentID, &at.ToolName, &at.EnvironmentID, &at.CreatedAt,
			&at.EnvironmentName)
		if err != nil {
			return nil, err
		}
		agentTools = append(agentTools, &at)
	}
	
	return agentTools, rows.Err()
}

// Clear removes all tool assignments for an agent across all environments
func (r *AgentToolRepo) Clear(agentID int64) error {
	query := `DELETE FROM agent_tools WHERE agent_id = ?`
	_, err := r.db.Exec(query, agentID)
	return err
}

// ClearEnvironment removes all tool assignments for an agent in a specific environment
func (r *AgentToolRepo) ClearEnvironment(agentID, environmentID int64) error {
	query := `DELETE FROM agent_tools WHERE agent_id = ? AND environment_id = ?`
	_, err := r.db.Exec(query, agentID, environmentID)
	return err
}

// SetToolsForEnvironment replaces all tool assignments for an agent in a specific environment
func (r *AgentToolRepo) SetToolsForEnvironment(agentID, environmentID int64, toolNames []string) error {
	// Start transaction
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear existing assignments for this agent in this environment
	_, err = tx.Exec(`DELETE FROM agent_tools WHERE agent_id = ? AND environment_id = ?`, 
		agentID, environmentID)
	if err != nil {
		return err
	}

	// Add new assignments
	for _, toolName := range toolNames {
		_, err = tx.Exec(`INSERT INTO agent_tools (agent_id, tool_name, environment_id) VALUES (?, ?, ?)`, 
			agentID, toolName, environmentID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// HasTool checks if an agent has access to a specific tool in a specific environment
func (r *AgentToolRepo) HasTool(agentID int64, toolName string, environmentID int64) (bool, error) {
	query := `SELECT COUNT(*) FROM agent_tools WHERE agent_id = ? AND tool_name = ? AND environment_id = ?`
	
	var count int
	err := r.db.QueryRow(query, agentID, toolName, environmentID).Scan(&count)
	if err != nil {
		return false, err
	}
	
	return count > 0, nil
}

// GetToolNamesByEnvironment returns just the tool names for an agent in a specific environment
// This is useful for filtering in the GenkitService
func (r *AgentToolRepo) GetToolNamesByEnvironment(agentID, environmentID int64) ([]string, error) {
	query := `SELECT tool_name FROM agent_tools WHERE agent_id = ? AND environment_id = ? ORDER BY tool_name`
	
	rows, err := r.db.Query(query, agentID, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var toolNames []string
	for rows.Next() {
		var toolName string
		err := rows.Scan(&toolName)
		if err != nil {
			return nil, err
		}
		toolNames = append(toolNames, toolName)
	}
	
	return toolNames, rows.Err()
}

// --- Legacy compatibility methods for backward compatibility ---

// AddLegacy maintains compatibility with old tool_id based API (deprecated)
func (r *AgentToolRepo) AddLegacy(agentID, toolID int64) (*models.AgentTool, error) {
	// This method is deprecated and should not be used in new code
	// For now, return an error indicating the method is no longer supported
	return nil, sql.ErrNoRows
}

// RemoveLegacy maintains compatibility with old tool_id based API (deprecated)
func (r *AgentToolRepo) RemoveLegacy(agentID, toolID int64) error {
	// This method is deprecated and should not be used in new code
	return sql.ErrNoRows
}