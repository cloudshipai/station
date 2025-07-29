package repositories

import (
	"database/sql"
	"station/pkg/models"
)

type AgentToolRepo struct {
	db *sql.DB
}

func NewAgentToolRepo(db *sql.DB) *AgentToolRepo {
	return &AgentToolRepo{db: db}
}

func (r *AgentToolRepo) Add(agentID, toolID int64) (*models.AgentTool, error) {
	query := `INSERT INTO agent_tools (agent_id, tool_id) 
			  VALUES (?, ?) 
			  RETURNING id, agent_id, tool_id, created_at`
	
	var agentTool models.AgentTool
	err := r.db.QueryRow(query, agentID, toolID).Scan(
		&agentTool.ID, &agentTool.AgentID, &agentTool.ToolID, &agentTool.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &agentTool, nil
}

func (r *AgentToolRepo) Remove(agentID, toolID int64) error {
	query := `DELETE FROM agent_tools WHERE agent_id = ? AND tool_id = ?`
	_, err := r.db.Exec(query, agentID, toolID)
	return err
}

func (r *AgentToolRepo) List(agentID int64) ([]*models.AgentToolWithDetails, error) {
	query := `SELECT at.id, at.agent_id, at.tool_id, at.created_at, 
					 t.tool_name, t.tool_description, t.tool_schema, s.server_name
			  FROM agent_tools at
			  JOIN mcp_tools t ON at.tool_id = t.id
			  JOIN mcp_servers s ON t.server_id = s.id
			  WHERE at.agent_id = ?
			  ORDER BY s.server_name, t.tool_name`
	
	rows, err := r.db.Query(query, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var agentTools []*models.AgentToolWithDetails
	for rows.Next() {
		var at models.AgentToolWithDetails
		err := rows.Scan(&at.ID, &at.AgentID, &at.ToolID, &at.CreatedAt,
			&at.ToolName, &at.ToolDescription, &at.ToolSchema, &at.ServerName)
		if err != nil {
			return nil, err
		}
		agentTools = append(agentTools, &at)
	}
	
	return agentTools, rows.Err()
}

func (r *AgentToolRepo) Clear(agentID int64) error {
	query := `DELETE FROM agent_tools WHERE agent_id = ?`
	_, err := r.db.Exec(query, agentID)
	return err
}