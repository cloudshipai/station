package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"station/internal/db/queries"
	"station/pkg/models"
)

type MCPToolRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewMCPToolRepo(db *sql.DB) *MCPToolRepo {
	return &MCPToolRepo{
		db:      db,
		queries: queries.New(db),
	}
}

// convertMCPToolFromSQLc converts sqlc McpTool to models.MCPTool
func convertMCPToolFromSQLc(tool queries.McpTool) *models.MCPTool {
	result := &models.MCPTool{
		ID:          tool.ID,
		MCPServerID: tool.McpServerID,
		Name:        tool.Name,
		Description: tool.Description.String,
	}
	
	if tool.CreatedAt.Valid {
		result.CreatedAt = tool.CreatedAt.Time
	}
	
	if tool.InputSchema.Valid {
		result.Schema = json.RawMessage(tool.InputSchema.String)
	}
	
	return result
}

// convertMCPToolToSQLc converts models.MCPTool to sqlc CreateMCPToolParams
func convertMCPToolToSQLc(tool *models.MCPTool) queries.CreateMCPToolParams {
	params := queries.CreateMCPToolParams{
		McpServerID: tool.MCPServerID,
		Name:        tool.Name,
		Description: sql.NullString{String: tool.Description, Valid: tool.Description != ""},
	}
	
	if tool.Schema != nil {
		params.InputSchema = sql.NullString{String: string(tool.Schema), Valid: true}
	}
	
	return params
}

func (r *MCPToolRepo) Create(tool *models.MCPTool) (int64, error) {
	params := convertMCPToolToSQLc(tool)
	created, err := r.queries.CreateMCPTool(context.Background(), params)
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

func (r *MCPToolRepo) CreateTx(tx *sql.Tx, tool *models.MCPTool) (int64, error) {
	params := convertMCPToolToSQLc(tool)
	txQueries := r.queries.WithTx(tx)
	created, err := txQueries.CreateMCPTool(context.Background(), params)
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

func (r *MCPToolRepo) GetByID(id int64) (*models.MCPTool, error) {
	tool, err := r.queries.GetMCPTool(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return convertMCPToolFromSQLc(tool), nil
}

func (r *MCPToolRepo) GetByServerID(serverID int64) ([]*models.MCPTool, error) {
	tools, err := r.queries.ListMCPToolsByServer(context.Background(), serverID)
	if err != nil {
		return nil, err
	}
	
	var result []*models.MCPTool
	for _, tool := range tools {
		result = append(result, convertMCPToolFromSQLc(tool))
	}
	
	return result, nil
}

func (r *MCPToolRepo) GetByEnvironmentID(environmentID int64) ([]*models.MCPTool, error) {
	tools, err := r.queries.ListMCPToolsByEnvironment(context.Background(), environmentID)
	if err != nil {
		return nil, err
	}
	
	var result []*models.MCPTool
	for _, tool := range tools {
		result = append(result, convertMCPToolFromSQLc(tool))
	}
	
	return result, nil
}

func (r *MCPToolRepo) GetByServerInEnvironment(environmentID int64, serverName string) ([]*models.MCPTool, error) {
	params := queries.ListMCPToolsByServerInEnvironmentParams{
		EnvironmentID: environmentID,
		Name:          serverName,
	}
	tools, err := r.queries.ListMCPToolsByServerInEnvironment(context.Background(), params)
	if err != nil {
		return nil, err
	}
	
	var result []*models.MCPTool
	for _, tool := range tools {
		result = append(result, convertMCPToolFromSQLc(tool))
	}
	
	return result, nil
}

func (r *MCPToolRepo) DeleteByServerID(serverID int64) error {
	return r.DeleteByServerIDTx(nil, serverID)
}

func (r *MCPToolRepo) DeleteByServerIDTx(tx *sql.Tx, serverID int64) error {
	if tx != nil {
		txQueries := r.queries.WithTx(tx)
		return txQueries.DeleteMCPToolsByServer(context.Background(), serverID)
	} else {
		return r.queries.DeleteMCPToolsByServer(context.Background(), serverID)
	}
}

func (r *MCPToolRepo) GetAllWithDetails() ([]*models.MCPToolWithDetails, error) {
	// First check if there are any servers at all
	var serverCount int
	countQuery := `SELECT COUNT(*) FROM mcp_servers`
	if err := r.db.QueryRow(countQuery).Scan(&serverCount); err != nil {
		return nil, err
	}
	
	// If no servers exist, return empty slice
	if serverCount == 0 {
		return []*models.MCPToolWithDetails{}, nil
	}
	
	query := `SELECT t.id, t.mcp_server_id, t.name, t.description, t.input_schema, t.created_at,
			         s.name as server_name,
			         0 as config_id,
			         'server-' || s.name as config_name,
			         1 as config_version,
			         s.environment_id as environment_id,
			         e.name as environment_name
			  FROM mcp_tools t
			  JOIN mcp_servers s ON t.mcp_server_id = s.id
			  JOIN environments e ON s.environment_id = e.id
			  ORDER BY e.name, s.name, t.name`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var tools []*models.MCPToolWithDetails
	for rows.Next() {
		var tool models.MCPToolWithDetails
		var schemaStr sql.NullString // Use NullString to handle NULL values
		err := rows.Scan(
			&tool.ID, &tool.MCPServerID, &tool.Name, &tool.Description, &schemaStr, &tool.CreatedAt,
			&tool.ServerName, &tool.ConfigID, &tool.ConfigName, &tool.ConfigVersion, &tool.EnvironmentID, &tool.EnvironmentName,
		)
		if err != nil {
			return nil, err
		}
		
		// Convert NullString to json.RawMessage
		if schemaStr.Valid && schemaStr.String != "" {
			tool.Schema = json.RawMessage(schemaStr.String)
		} else {
			tool.Schema = json.RawMessage("null")
		}
		
		tools = append(tools, &tool)
	}
	
	return tools, rows.Err()
}