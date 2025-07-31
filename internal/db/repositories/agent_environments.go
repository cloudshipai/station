package repositories

import (
	"database/sql"
	"station/pkg/models"
)

type AgentEnvironmentRepo struct {
	db *sql.DB
}

func NewAgentEnvironmentRepo(db *sql.DB) *AgentEnvironmentRepo {
	return &AgentEnvironmentRepo{db: db}
}

// Add creates a new agent-environment relationship
func (r *AgentEnvironmentRepo) Add(agentID, environmentID int64) (*models.AgentEnvironment, error) {
	query := `INSERT INTO agent_environments (agent_id, environment_id) 
			  VALUES (?, ?) 
			  RETURNING id, agent_id, environment_id, created_at`
	
	var agentEnv models.AgentEnvironment
	err := r.db.QueryRow(query, agentID, environmentID).Scan(
		&agentEnv.ID, &agentEnv.AgentID, &agentEnv.EnvironmentID, &agentEnv.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &agentEnv, nil
}

// Remove removes an agent-environment relationship
func (r *AgentEnvironmentRepo) Remove(agentID, environmentID int64) error {
	query := `DELETE FROM agent_environments WHERE agent_id = ? AND environment_id = ?`
	_, err := r.db.Exec(query, agentID, environmentID)
	return err
}

// ListByAgent returns all environments that an agent has access to
func (r *AgentEnvironmentRepo) ListByAgent(agentID int64) ([]*models.AgentEnvironmentWithDetails, error) {
	query := `SELECT ae.id, ae.agent_id, ae.environment_id, ae.created_at,
					 e.name as environment_name, e.description as environment_description
			  FROM agent_environments ae
			  JOIN environments e ON ae.environment_id = e.id
			  WHERE ae.agent_id = ?
			  ORDER BY e.name`
	
	rows, err := r.db.Query(query, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var agentEnvironments []*models.AgentEnvironmentWithDetails
	for rows.Next() {
		var ae models.AgentEnvironmentWithDetails
		err := rows.Scan(&ae.ID, &ae.AgentID, &ae.EnvironmentID, &ae.CreatedAt,
			&ae.EnvironmentName, &ae.EnvironmentDescription)
		if err != nil {
			return nil, err
		}
		agentEnvironments = append(agentEnvironments, &ae)
	}
	
	return agentEnvironments, rows.Err()
}

// ListByEnvironment returns all agents that have access to an environment
func (r *AgentEnvironmentRepo) ListByEnvironment(environmentID int64) ([]*models.AgentEnvironment, error) {
	query := `SELECT id, agent_id, environment_id, created_at
			  FROM agent_environments
			  WHERE environment_id = ?
			  ORDER BY agent_id`
	
	rows, err := r.db.Query(query, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var agentEnvironments []*models.AgentEnvironment
	for rows.Next() {
		var ae models.AgentEnvironment
		err := rows.Scan(&ae.ID, &ae.AgentID, &ae.EnvironmentID, &ae.CreatedAt)
		if err != nil {
			return nil, err
		}
		agentEnvironments = append(agentEnvironments, &ae)
	}
	
	return agentEnvironments, rows.Err()
}

// Clear removes all environment relationships for an agent
func (r *AgentEnvironmentRepo) Clear(agentID int64) error {
	query := `DELETE FROM agent_environments WHERE agent_id = ?`
	_, err := r.db.Exec(query, agentID)
	return err
}

// SetEnvironments replaces all environment relationships for an agent
func (r *AgentEnvironmentRepo) SetEnvironments(agentID int64, environmentIDs []int64) error {
	// Start transaction
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear existing relationships
	_, err = tx.Exec(`DELETE FROM agent_environments WHERE agent_id = ?`, agentID)
	if err != nil {
		return err
	}

	// Add new relationships
	for _, environmentID := range environmentIDs {
		_, err = tx.Exec(`INSERT INTO agent_environments (agent_id, environment_id) VALUES (?, ?)`, 
			agentID, environmentID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// HasAccess checks if an agent has access to a specific environment
func (r *AgentEnvironmentRepo) HasAccess(agentID, environmentID int64) (bool, error) {
	query := `SELECT COUNT(*) FROM agent_environments WHERE agent_id = ? AND environment_id = ?`
	
	var count int
	err := r.db.QueryRow(query, agentID, environmentID).Scan(&count)
	if err != nil {
		return false, err
	}
	
	return count > 0, nil
}