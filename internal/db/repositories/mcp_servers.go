package repositories

import (
	"database/sql"
	"encoding/json"
	"station/pkg/models"
)

type MCPServerRepo struct {
	db *sql.DB
}

func NewMCPServerRepo(db *sql.DB) *MCPServerRepo {
	return &MCPServerRepo{db: db}
}

func (r *MCPServerRepo) Create(server *models.MCPServer) (int64, error) {
	// Marshal env map to JSON
	envJSON, err := json.Marshal(server.Env)
	if err != nil {
		envJSON = []byte("{}")
	}
	
	// Marshal args to JSON
	argsJSON, err := json.Marshal(server.Args)
	if err != nil {
		argsJSON = []byte("[]")
	}
	
	query := `INSERT INTO mcp_servers (mcp_config_id, name, command, args, env) 
			  VALUES (?, ?, ?, ?, ?) 
			  RETURNING id`
	
	var id int64
	err = r.db.QueryRow(query, server.MCPConfigID, server.Name, server.Command, string(argsJSON), string(envJSON)).Scan(&id)
	if err != nil {
		return 0, err
	}
	
	return id, nil
}

func (r *MCPServerRepo) GetByID(id int64) (*models.MCPServer, error) {
	query := `SELECT id, mcp_config_id, name, command, args, env, created_at FROM mcp_servers WHERE id = ?`
	
	var server models.MCPServer
	var argsJSON, envJSON string
	err := r.db.QueryRow(query, id).Scan(
		&server.ID, &server.MCPConfigID, &server.Name, &server.Command, &argsJSON, &envJSON, &server.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	// Unmarshal JSON fields
	if err := json.Unmarshal([]byte(argsJSON), &server.Args); err != nil {
		server.Args = []string{}
	}
	if err := json.Unmarshal([]byte(envJSON), &server.Env); err != nil {
		server.Env = map[string]string{}
	}
	
	return &server, nil
}

func (r *MCPServerRepo) GetByConfigID(configID int64) ([]*models.MCPServer, error) {
	query := `SELECT id, mcp_config_id, name, command, args, env, created_at 
			  FROM mcp_servers WHERE mcp_config_id = ? ORDER BY name`
	
	rows, err := r.db.Query(query, configID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var servers []*models.MCPServer
	for rows.Next() {
		var server models.MCPServer
		var argsJSON, envJSON string
		err := rows.Scan(&server.ID, &server.MCPConfigID, &server.Name, &server.Command, &argsJSON, &envJSON, &server.CreatedAt)
		if err != nil {
			return nil, err
		}
		
		// Unmarshal JSON fields
		if err := json.Unmarshal([]byte(argsJSON), &server.Args); err != nil {
			server.Args = []string{}
		}
		if err := json.Unmarshal([]byte(envJSON), &server.Env); err != nil {
			server.Env = map[string]string{}
		}
		
		servers = append(servers, &server)
	}
	
	return servers, rows.Err()
}

func (r *MCPServerRepo) GetByEnvironmentID(environmentID int64) ([]*models.MCPServer, error) {
	query := `SELECT s.id, s.mcp_config_id, s.name, s.command, s.args, s.env, s.created_at 
			  FROM mcp_servers s
			  JOIN mcp_configs c ON s.mcp_config_id = c.id
			  WHERE c.environment_id = ? AND c.version = (
				  SELECT MAX(version) FROM mcp_configs WHERE environment_id = ?
			  )
			  ORDER BY s.name`
	
	rows, err := r.db.Query(query, environmentID, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var servers []*models.MCPServer
	for rows.Next() {
		var server models.MCPServer
		var argsJSON, envJSON string
		err := rows.Scan(&server.ID, &server.MCPConfigID, &server.Name, &server.Command, &argsJSON, &envJSON, &server.CreatedAt)
		if err != nil {
			return nil, err
		}
		
		// Unmarshal JSON fields
		if err := json.Unmarshal([]byte(argsJSON), &server.Args); err != nil {
			server.Args = []string{}
		}
		if err := json.Unmarshal([]byte(envJSON), &server.Env); err != nil {
			server.Env = map[string]string{}
		}
		
		servers = append(servers, &server)
	}
	
	return servers, rows.Err()
}

func (r *MCPServerRepo) DeleteByConfigID(configID int64) error {
	return r.DeleteByConfigIDTx(nil, configID)
}

func (r *MCPServerRepo) DeleteByConfigIDTx(tx *sql.Tx, configID int64) error {
	query := `DELETE FROM mcp_servers WHERE mcp_config_id = ?`
	
	// Use transaction if provided, otherwise use regular db connection
	if tx != nil {
		_, err := tx.Exec(query, configID)
		return err
	} else {
		_, err := r.db.Exec(query, configID)
		return err
	}
}