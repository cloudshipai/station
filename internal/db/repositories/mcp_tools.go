package repositories

import (
	"database/sql"
	"station/pkg/models"
)

type MCPToolRepo struct {
	db *sql.DB
}

func NewMCPToolRepo(db *sql.DB) *MCPToolRepo {
	return &MCPToolRepo{db: db}
}

func (r *MCPToolRepo) Create(tool *models.MCPTool) (int64, error) {
	query := `INSERT INTO mcp_tools (mcp_server_id, name, description, schema) 
			  VALUES (?, ?, ?, ?) 
			  RETURNING id`
	
	var id int64
	err := r.db.QueryRow(query, tool.MCPServerID, tool.Name, tool.Description, tool.Schema).Scan(&id)
	if err != nil {
		return 0, err
	}
	
	return id, nil
}

func (r *MCPToolRepo) GetByID(id int64) (*models.MCPTool, error) {
	query := `SELECT id, mcp_server_id, name, description, schema, created_at 
			  FROM mcp_tools WHERE id = ?`
	
	var tool models.MCPTool
	err := r.db.QueryRow(query, id).Scan(
		&tool.ID, &tool.MCPServerID, &tool.Name, &tool.Description, &tool.Schema, &tool.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &tool, nil
}

func (r *MCPToolRepo) GetByServerID(serverID int64) ([]*models.MCPTool, error) {
	query := `SELECT id, mcp_server_id, name, description, schema, created_at 
			  FROM mcp_tools WHERE mcp_server_id = ? ORDER BY name`
	
	rows, err := r.db.Query(query, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var tools []*models.MCPTool
	for rows.Next() {
		var tool models.MCPTool
		err := rows.Scan(&tool.ID, &tool.MCPServerID, &tool.Name, &tool.Description, &tool.Schema, &tool.CreatedAt)
		if err != nil {
			return nil, err
		}
		tools = append(tools, &tool)
	}
	
	return tools, rows.Err()
}

func (r *MCPToolRepo) GetByEnvironmentID(environmentID int64) ([]*models.MCPTool, error) {
	query := `SELECT t.id, t.mcp_server_id, t.name, t.description, t.schema, t.created_at 
			  FROM mcp_tools t
			  JOIN mcp_servers s ON t.mcp_server_id = s.id
			  JOIN mcp_configs c ON s.mcp_config_id = c.id
			  WHERE c.environment_id = ? AND c.version = (
				  SELECT MAX(version) FROM mcp_configs WHERE environment_id = ?
			  )
			  ORDER BY s.name, t.name`
	
	rows, err := r.db.Query(query, environmentID, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var tools []*models.MCPTool
	for rows.Next() {
		var tool models.MCPTool
		err := rows.Scan(&tool.ID, &tool.MCPServerID, &tool.Name, &tool.Description, &tool.Schema, &tool.CreatedAt)
		if err != nil {
			return nil, err
		}
		tools = append(tools, &tool)
	}
	
	return tools, rows.Err()
}

func (r *MCPToolRepo) GetByServerInEnvironment(environmentID int64, serverName string) ([]*models.MCPTool, error) {
	query := `SELECT t.id, t.mcp_server_id, t.name, t.description, t.schema, t.created_at 
			  FROM mcp_tools t
			  JOIN mcp_servers s ON t.mcp_server_id = s.id
			  JOIN mcp_configs c ON s.mcp_config_id = c.id
			  WHERE c.environment_id = ? AND s.name = ? AND c.version = (
				  SELECT MAX(version) FROM mcp_configs WHERE environment_id = ?
			  )
			  ORDER BY t.name`
	
	rows, err := r.db.Query(query, environmentID, serverName, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var tools []*models.MCPTool
	for rows.Next() {
		var tool models.MCPTool
		err := rows.Scan(&tool.ID, &tool.MCPServerID, &tool.Name, &tool.Description, &tool.Schema, &tool.CreatedAt)
		if err != nil {
			return nil, err
		}
		tools = append(tools, &tool)
	}
	
	return tools, rows.Err()
}

func (r *MCPToolRepo) DeleteByServerID(serverID int64) error {
	query := `DELETE FROM mcp_tools WHERE mcp_server_id = ?`
	_, err := r.db.Exec(query, serverID)
	return err
}