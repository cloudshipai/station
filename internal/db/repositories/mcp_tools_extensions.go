package repositories

import (
	"database/sql"
	"station/pkg/models"
)

// Extension methods for MCPToolRepo to support file-based configs

// CreateWithFileConfig creates a new tool linked to a file config
func (r *MCPToolRepo) CreateWithFileConfig(tool *models.MCPTool, fileConfigID int64) (int64, error) {
	query := `
		INSERT INTO mcp_tools (mcp_server_id, name, description, input_schema, file_config_id)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id
	`
	
	var id int64
	err := r.db.QueryRow(
		query,
		tool.MCPServerID,
		tool.Name,
		tool.Description,
		string(tool.Schema),
		fileConfigID,
	).Scan(&id)
	
	return id, err
}

// CreateWithFileConfigTx creates a new tool linked to a file config within a transaction
func (r *MCPToolRepo) CreateWithFileConfigTx(tx *sql.Tx, tool *models.MCPTool, fileConfigID int64) (int64, error) {
	query := `
		INSERT INTO mcp_tools (mcp_server_id, name, description, input_schema, file_config_id)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id
	`
	
	var id int64
	err := tx.QueryRow(
		query,
		tool.MCPServerID,
		tool.Name,
		tool.Description,
		string(tool.Schema),
		fileConfigID,
	).Scan(&id)
	
	return id, err
}

// GetByFileConfigID gets all tools for a specific file config
func (r *MCPToolRepo) GetByFileConfigID(fileConfigID int64) ([]*models.MCPTool, error) {
	query := `
		SELECT id, mcp_server_id, name, description, input_schema, created_at
		FROM mcp_tools
		WHERE file_config_id = ?
		ORDER BY name
	`
	
	rows, err := r.db.Query(query, fileConfigID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var tools []*models.MCPTool
	for rows.Next() {
		tool := &models.MCPTool{}
		var description sql.NullString
		var schema sql.NullString
		
		err := rows.Scan(
			&tool.ID,
			&tool.MCPServerID,
			&tool.Name,
			&description,
			&schema,
			&tool.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		
		if description.Valid {
			tool.Description = description.String
		}
		if schema.Valid {
			tool.Schema = []byte(schema.String)
		}
		
		tools = append(tools, tool)
	}
	
	return tools, rows.Err()
}

// DeleteByFileConfigID deletes all tools for a specific file config
func (r *MCPToolRepo) DeleteByFileConfigID(fileConfigID int64) error {
	query := `DELETE FROM mcp_tools WHERE file_config_id = ?`
	_, err := r.db.Exec(query, fileConfigID)
	return err
}

// DeleteByFileConfigIDTx deletes all tools for a specific file config within a transaction
func (r *MCPToolRepo) DeleteByFileConfigIDTx(tx *sql.Tx, fileConfigID int64) error {
	query := `DELETE FROM mcp_tools WHERE file_config_id = ?`
	_, err := tx.Exec(query, fileConfigID)
	return err
}

// GetToolsWithFileConfigInfo gets tools with their file config information
func (r *MCPToolRepo) GetToolsWithFileConfigInfo(environmentID int64) ([]*models.MCPToolWithFileConfig, error) {
	query := `
		SELECT 
			t.id, t.mcp_server_id, t.name, t.description, t.input_schema, t.created_at,
			s.name as server_name,
			fc.id as file_config_id, fc.config_name, fc.template_path, fc.last_loaded_at
		FROM mcp_tools t
		JOIN mcp_servers s ON t.mcp_server_id = s.id
		LEFT JOIN file_mcp_configs fc ON t.file_config_id = fc.id
		WHERE s.environment_id = ?
		ORDER BY fc.config_name, s.name, t.name
	`
	
	rows, err := r.db.Query(query, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var tools []*models.MCPToolWithFileConfig
	for rows.Next() {
		tool := &models.MCPToolWithFileConfig{}
		var description sql.NullString
		var schema sql.NullString
		var fileConfigID sql.NullInt64
		var configName sql.NullString
		var templatePath sql.NullString
		var lastLoaded sql.NullTime
		
		err := rows.Scan(
			&tool.ID,
			&tool.MCPServerID,
			&tool.Name,
			&description,
			&schema,
			&tool.CreatedAt,
			&tool.ServerName,
			&fileConfigID,
			&configName,
			&templatePath,
			&lastLoaded,
		)
		if err != nil {
			return nil, err
		}
		
		if description.Valid {
			tool.Description = description.String
		}
		if schema.Valid {
			tool.Schema = []byte(schema.String)
		}
		if fileConfigID.Valid {
			tool.FileConfigID = &fileConfigID.Int64
		}
		if configName.Valid {
			tool.ConfigName = configName.String
		}
		if templatePath.Valid {
			tool.TemplatePath = templatePath.String
		}
		if lastLoaded.Valid {
			tool.LastLoaded = &lastLoaded.Time
		}
		
		tools = append(tools, tool)
	}
	
	return tools, rows.Err()
}

// GetOrphanedTools gets tools that reference non-existent file configs
func (r *MCPToolRepo) GetOrphanedTools(environmentID int64) ([]*models.MCPTool, error) {
	query := `
		SELECT t.id, t.mcp_server_id, t.name, t.description, t.input_schema, t.created_at
		FROM mcp_tools t
		JOIN mcp_servers s ON t.mcp_server_id = s.id
		LEFT JOIN file_mcp_configs fc ON t.file_config_id = fc.id
		WHERE s.environment_id = ? AND t.file_config_id IS NOT NULL AND fc.id IS NULL
	`
	
	rows, err := r.db.Query(query, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var tools []*models.MCPTool
	for rows.Next() {
		tool := &models.MCPTool{}
		var description sql.NullString
		var schema sql.NullString
		
		err := rows.Scan(
			&tool.ID,
			&tool.MCPServerID,
			&tool.Name,
			&description,
			&schema,
			&tool.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		
		if description.Valid {
			tool.Description = description.String
		}
		if schema.Valid {
			tool.Schema = []byte(schema.String)
		}
		
		tools = append(tools, tool)
	}
	
	return tools, rows.Err()
}

// UpdateFileConfigReference updates the file config reference for tools
func (r *MCPToolRepo) UpdateFileConfigReference(toolID, fileConfigID int64) error {
	query := `UPDATE mcp_tools SET file_config_id = ? WHERE id = ?`
	_, err := r.db.Exec(query, fileConfigID, toolID)
	return err
}

// ClearFileConfigReference clears the file config reference for tools
func (r *MCPToolRepo) ClearFileConfigReference(toolID int64) error {
	query := `UPDATE mcp_tools SET file_config_id = NULL WHERE id = ?`
	_, err := r.db.Exec(query, toolID)
	return err
}