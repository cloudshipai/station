package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"station/internal/db/queries"
	"station/pkg/models"
)

type MCPServerRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewMCPServerRepo(db *sql.DB) *MCPServerRepo {
	return &MCPServerRepo{
		db:      db,
		queries: queries.New(db),
	}
}

// convertMCPServerFromSQLc converts sqlc McpServer to models.MCPServer
func convertMCPServerFromSQLc(server queries.McpServer) *models.MCPServer {
	result := &models.MCPServer{
		ID:            server.ID,
		Name:          server.Name,
		Command:       server.Command,
		EnvironmentID: server.EnvironmentID,
	}
	
	// Handle JSON fields
	if server.Args.Valid {
		if err := json.Unmarshal([]byte(server.Args.String), &result.Args); err != nil {
			result.Args = []string{}
		}
	} else {
		result.Args = []string{}
	}
	
	if server.Env.Valid {
		if err := json.Unmarshal([]byte(server.Env.String), &result.Env); err != nil {
			result.Env = map[string]string{}
		}
	} else {
		result.Env = map[string]string{}
	}
	
	if server.WorkingDir.Valid {
		result.WorkingDir = &server.WorkingDir.String
	}
	
	if server.TimeoutSeconds.Valid {
		result.TimeoutSeconds = &server.TimeoutSeconds.Int64
	}
	
	if server.AutoRestart.Valid {
		result.AutoRestart = &server.AutoRestart.Bool
	}
	
	if server.FileConfigID.Valid {
		result.FileConfigID = &server.FileConfigID.Int64
	}
	
	if server.CreatedAt.Valid {
		result.CreatedAt = server.CreatedAt.Time
	}
	
	return result
}

// convertMCPServerToSQLc converts models.MCPServer to sqlc CreateMCPServerParams
func convertMCPServerToSQLc(server *models.MCPServer) queries.CreateMCPServerParams {
	params := queries.CreateMCPServerParams{
		Name:          server.Name,
		Command:       server.Command,
		EnvironmentID: server.EnvironmentID,
	}
	
	// Set FileConfigID if available
	if server.FileConfigID != nil {
		params.FileConfigID = sql.NullInt64{Int64: *server.FileConfigID, Valid: true}
	}
	
	// Handle JSON fields
	if argsJSON, err := json.Marshal(server.Args); err == nil {
		params.Args = sql.NullString{String: string(argsJSON), Valid: true}
	}
	
	if envJSON, err := json.Marshal(server.Env); err == nil {
		params.Env = sql.NullString{String: string(envJSON), Valid: true}
	}
	
	if server.WorkingDir != nil {
		params.WorkingDir = sql.NullString{String: *server.WorkingDir, Valid: true}
	}
	
	if server.TimeoutSeconds != nil {
		params.TimeoutSeconds = sql.NullInt64{Int64: *server.TimeoutSeconds, Valid: true}
	}
	
	if server.AutoRestart != nil {
		params.AutoRestart = sql.NullBool{Bool: *server.AutoRestart, Valid: true}
	}
	
	return params
}

func (r *MCPServerRepo) Create(server *models.MCPServer) (int64, error) {
	params := convertMCPServerToSQLc(server)
	created, err := r.queries.CreateMCPServer(context.Background(), params)
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

func (r *MCPServerRepo) CreateTx(tx *sql.Tx, server *models.MCPServer) (int64, error) {
	params := convertMCPServerToSQLc(server)
	txQueries := r.queries.WithTx(tx)
	created, err := txQueries.CreateMCPServer(context.Background(), params)
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

func (r *MCPServerRepo) GetByID(id int64) (*models.MCPServer, error) {
	server, err := r.queries.GetMCPServer(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return convertMCPServerFromSQLc(server), nil
}

func (r *MCPServerRepo) GetByEnvironmentID(environmentID int64) ([]*models.MCPServer, error) {
	servers, err := r.queries.ListMCPServersByEnvironment(context.Background(), environmentID)
	if err != nil {
		return nil, err
	}
	
	var result []*models.MCPServer
	for _, server := range servers {
		result = append(result, convertMCPServerFromSQLc(server))
	}
	
	return result, nil
}

func (r *MCPServerRepo) Delete(id int64) error {
	return r.queries.DeleteMCPServer(context.Background(), id)
}

func (r *MCPServerRepo) DeleteByEnvironmentID(environmentID int64) error {
	return r.DeleteByEnvironmentIDTx(nil, environmentID)
}

func (r *MCPServerRepo) DeleteByEnvironmentIDTx(tx *sql.Tx, environmentID int64) error {
	if tx != nil {
		txQueries := r.queries.WithTx(tx)
		return txQueries.DeleteMCPServersByEnvironment(context.Background(), environmentID)
	} else {
		return r.queries.DeleteMCPServersByEnvironment(context.Background(), environmentID)
	}
}

// GetByNameAndEnvironment finds a server by name and environment ID
func (r *MCPServerRepo) GetByNameAndEnvironment(name string, environmentID int64) (*models.MCPServer, error) {
	params := queries.GetMCPServerByNameAndEnvironmentParams{
		Name:          name,
		EnvironmentID: environmentID,
	}
	
	server, err := r.queries.GetMCPServerByNameAndEnvironment(context.Background(), params)
	if err != nil {
		return nil, err
	}
	
	return convertMCPServerFromSQLc(server), nil
}

// Update updates an existing MCP server
func (r *MCPServerRepo) Update(server *models.MCPServer) error {
	// Convert the server to SQLC update params
	params := queries.UpdateMCPServerParams{
		ID:      server.ID,
		Name:    server.Name,
		Command: server.Command,
	}
	
	// Handle JSON fields
	if argsJSON, err := json.Marshal(server.Args); err == nil {
		params.Args = sql.NullString{String: string(argsJSON), Valid: true}
	}
	
	if envJSON, err := json.Marshal(server.Env); err == nil {
		params.Env = sql.NullString{String: string(envJSON), Valid: true}
	}
	
	if server.WorkingDir != nil {
		params.WorkingDir = sql.NullString{String: *server.WorkingDir, Valid: true}
	}
	
	if server.TimeoutSeconds != nil {
		params.TimeoutSeconds = sql.NullInt64{Int64: *server.TimeoutSeconds, Valid: true}
	}
	
	if server.AutoRestart != nil {
		params.AutoRestart = sql.NullBool{Bool: *server.AutoRestart, Valid: true}
	}
	
	if server.FileConfigID != nil {
		params.FileConfigID = sql.NullInt64{Int64: *server.FileConfigID, Valid: true}
	}
	
	_, err := r.queries.UpdateMCPServer(context.Background(), params)
	return err
}

// GetAll retrieves all MCP servers across all environments
func (r *MCPServerRepo) GetAll() ([]*models.MCPServer, error) {
	servers, err := r.queries.ListAllMCPServers(context.Background())
	if err != nil {
		return nil, err
	}

	var result []*models.MCPServer
	for _, server := range servers {
		result = append(result, convertMCPServerFromSQLc(server))
	}

	return result, nil
}