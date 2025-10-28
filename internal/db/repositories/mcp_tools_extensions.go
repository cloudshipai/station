package repositories

import (
	"context"
	"database/sql"
	"station/internal/db/queries"
	"station/pkg/models"
)

// Extension methods for MCPToolRepo to support file-based configs

// CreateWithFileConfig creates a new tool linked to a file config using SQLC
func (r *MCPToolRepo) CreateWithFileConfig(tool *models.MCPTool, fileConfigID int64) (int64, error) {
	params := queries.CreateMCPToolWithFileConfigParams{
		McpServerID:  tool.MCPServerID,
		Name:         tool.Name,
		Description:  sql.NullString{String: tool.Description, Valid: tool.Description != ""},
		InputSchema:  sql.NullString{String: string(tool.Schema), Valid: tool.Schema != nil},
		FileConfigID: sql.NullInt64{Int64: fileConfigID, Valid: true},
	}

	created, err := r.queries.CreateMCPToolWithFileConfig(context.Background(), params)
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

// CreateWithFileConfigTx creates a new tool linked to a file config within a transaction using SQLC
func (r *MCPToolRepo) CreateWithFileConfigTx(tx *sql.Tx, tool *models.MCPTool, fileConfigID int64) (int64, error) {
	params := queries.CreateMCPToolWithFileConfigParams{
		McpServerID:  tool.MCPServerID,
		Name:         tool.Name,
		Description:  sql.NullString{String: tool.Description, Valid: tool.Description != ""},
		InputSchema:  sql.NullString{String: string(tool.Schema), Valid: tool.Schema != nil},
		FileConfigID: sql.NullInt64{Int64: fileConfigID, Valid: true},
	}

	txQueries := r.queries.WithTx(tx)
	created, err := txQueries.CreateMCPToolWithFileConfig(context.Background(), params)
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

// GetByFileConfigID gets all tools for a specific file config using SQLC
func (r *MCPToolRepo) GetByFileConfigID(fileConfigID int64) ([]*models.MCPTool, error) {
	tools, err := r.queries.GetMCPToolsByFileConfigID(context.Background(), sql.NullInt64{Int64: fileConfigID, Valid: true})
	if err != nil {
		return nil, err
	}

	var result []*models.MCPTool
	for _, tool := range tools {
		result = append(result, convertMCPToolFromSQLc(tool))
	}

	return result, nil
}

// DeleteByFileConfigID deletes all tools for a specific file config using SQLC
func (r *MCPToolRepo) DeleteByFileConfigID(fileConfigID int64) error {
	return r.queries.DeleteMCPToolsByFileConfigID(context.Background(), sql.NullInt64{Int64: fileConfigID, Valid: true})
}

// DeleteByFileConfigIDTx deletes all tools for a specific file config within a transaction using SQLC
func (r *MCPToolRepo) DeleteByFileConfigIDTx(tx *sql.Tx, fileConfigID int64) error {
	txQueries := r.queries.WithTx(tx)
	return txQueries.DeleteMCPToolsByFileConfigID(context.Background(), sql.NullInt64{Int64: fileConfigID, Valid: true})
}

// GetToolsWithFileConfigInfo gets tools with their file config information using SQLC
func (r *MCPToolRepo) GetToolsWithFileConfigInfo(environmentID int64) ([]*models.MCPToolWithFileConfig, error) {
	// Use SQLC generated query and types directly
	rows, err := r.queries.GetMCPToolsWithFileConfigInfo(context.Background(), environmentID)
	if err != nil {
		return nil, err
	}

	// Convert SQLC row type to domain model
	var tools []*models.MCPToolWithFileConfig
	for _, row := range rows {
		tool := &models.MCPToolWithFileConfig{
			MCPTool: models.MCPTool{
				ID:          row.ID,
				MCPServerID: row.McpServerID,
				Name:        row.Name,
				Description: row.Description.String,
			},
			ServerName:   row.ServerName,
			ConfigName:   row.ConfigName.String,
			TemplatePath: row.TemplatePath.String,
		}

		if row.CreatedAt.Valid {
			tool.CreatedAt = row.CreatedAt.Time
		}

		if row.InputSchema.Valid {
			tool.Schema = []byte(row.InputSchema.String)
		}
		if row.FileConfigID.Valid {
			tool.FileConfigID = &row.FileConfigID.Int64
		}
		if row.LastLoadedAt.Valid {
			tool.LastLoaded = &row.LastLoadedAt.Time
		}

		tools = append(tools, tool)
	}

	return tools, nil
}

// GetOrphanedTools gets tools that reference non-existent file configs using SQLC
func (r *MCPToolRepo) GetOrphanedTools(environmentID int64) ([]*models.MCPTool, error) {
	tools, err := r.queries.GetOrphanedMCPTools(context.Background(), environmentID)
	if err != nil {
		return nil, err
	}

	var result []*models.MCPTool
	for _, tool := range tools {
		result = append(result, convertMCPToolFromSQLc(tool))
	}

	return result, nil
}

// UpdateFileConfigReference updates the file config reference for tools using SQLC
func (r *MCPToolRepo) UpdateFileConfigReference(toolID, fileConfigID int64) error {
	params := queries.UpdateMCPToolFileConfigReferenceParams{
		FileConfigID: sql.NullInt64{Int64: fileConfigID, Valid: true},
		ID:           toolID,
	}
	return r.queries.UpdateMCPToolFileConfigReference(context.Background(), params)
}

// ClearFileConfigReference clears the file config reference for tools using SQLC
func (r *MCPToolRepo) ClearFileConfigReference(toolID int64) error {
	return r.queries.ClearMCPToolFileConfigReference(context.Background(), toolID)
}
