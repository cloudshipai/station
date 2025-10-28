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

// FindByNameInEnvironment finds a tool by name within a specific environment
func (r *MCPToolRepo) FindByNameInEnvironment(environmentID int64, toolName string) (*models.MCPTool, error) {
	params := queries.FindMCPToolByNameInEnvironmentParams{
		EnvironmentID: environmentID,
		Name:          toolName,
	}

	tool, err := r.queries.FindMCPToolByNameInEnvironment(context.Background(), params)
	if err != nil {
		return nil, err
	}

	return convertMCPToolFromSQLc(tool), nil
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
	// First check if there are any servers at all using SQLC
	serverCount, err := r.queries.GetMCPToolsWithServerCount(context.Background())
	if err != nil {
		return nil, err
	}

	// If no servers exist, return empty slice
	if serverCount == 0 {
		return []*models.MCPToolWithDetails{}, nil
	}

	// Use SQLC generated query and types directly
	rows, err := r.queries.GetMCPToolsWithDetails(context.Background())
	if err != nil {
		return nil, err
	}

	// Convert SQLC row type to domain model
	var tools []*models.MCPToolWithDetails
	for _, row := range rows {
		tool := &models.MCPToolWithDetails{
			MCPTool: models.MCPTool{
				ID:          row.ID,
				MCPServerID: row.McpServerID,
				Name:        row.Name,
				Description: row.Description.String,
			},
			ServerName:      row.ServerName,
			ConfigID:        row.ConfigID,
			ConfigName:      row.ConfigName.(string),
			ConfigVersion:   row.ConfigVersion,
			EnvironmentID:   row.EnvironmentID,
			EnvironmentName: row.EnvironmentName,
		}

		if row.CreatedAt.Valid {
			tool.CreatedAt = row.CreatedAt.Time
		}

		if row.InputSchema.Valid && row.InputSchema.String != "" {
			tool.Schema = json.RawMessage(row.InputSchema.String)
		} else {
			tool.Schema = json.RawMessage("null")
		}

		tools = append(tools, tool)
	}

	return tools, nil
}

// GetServerNameForTool looks up which MCP server hosts a specific tool
func (r *MCPToolRepo) GetServerNameForTool(toolName string) (string, error) {
	return r.queries.GetMCPServerNameByTool(context.Background(), toolName)
}
