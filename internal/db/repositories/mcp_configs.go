package repositories

import (
	"database/sql"
	"station/pkg/models"
)

type MCPConfigRepo struct {
	db *sql.DB
}

func NewMCPConfigRepo(db *sql.DB) *MCPConfigRepo {
	return &MCPConfigRepo{db: db}
}

func (r *MCPConfigRepo) Create(environmentID int64, version int64, configJSON, encryptionKeyID string) (*models.MCPConfig, error) {
	query := `INSERT INTO mcp_configs (environment_id, version, config_json, encryption_key_id) 
			  VALUES (?, ?, ?, ?) 
			  RETURNING id, environment_id, version, config_json, encryption_key_id, created_at, updated_at`
	
	var config models.MCPConfig
	err := r.db.QueryRow(query, environmentID, version, configJSON, encryptionKeyID).Scan(
		&config.ID, &config.EnvironmentID, &config.Version, &config.ConfigJSON, 
		&config.EncryptionKeyID, &config.CreatedAt, &config.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &config, nil
}

// RotateEncryptionKey re-encrypts all configs with a new key
func (r *MCPConfigRepo) RotateEncryptionKey(oldKeyID, newKeyID string, reencryptFunc func([]byte, string, string) ([]byte, error)) error {
	// Get all configs using the old key
	query := `SELECT id, config_json FROM mcp_configs WHERE encryption_key_id = ?`
	rows, err := r.db.Query(query, oldKeyID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var configsToUpdate []struct {
		ID         int64
		ConfigJSON string
	}

	for rows.Next() {
		var config struct {
			ID         int64
			ConfigJSON string
		}
		if err := rows.Scan(&config.ID, &config.ConfigJSON); err != nil {
			return err
		}
		configsToUpdate = append(configsToUpdate, config)
	}

	// Re-encrypt each config
	updateQuery := `UPDATE mcp_configs SET config_json = ?, encryption_key_id = ? WHERE id = ?`
	for _, config := range configsToUpdate {
		newConfigJSON, err := reencryptFunc([]byte(config.ConfigJSON), oldKeyID, newKeyID)
		if err != nil {
			return err
		}

		_, err = r.db.Exec(updateQuery, string(newConfigJSON), newKeyID, config.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *MCPConfigRepo) GetByID(id int64) (*models.MCPConfig, error) {
	query := `SELECT id, environment_id, version, config_json, encryption_key_id, created_at, updated_at 
			  FROM mcp_configs WHERE id = ?`
	
	var config models.MCPConfig
	err := r.db.QueryRow(query, id).Scan(
		&config.ID, &config.EnvironmentID, &config.Version, &config.ConfigJSON,
		&config.EncryptionKeyID, &config.CreatedAt, &config.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &config, nil
}

func (r *MCPConfigRepo) GetLatest(environmentID int64) (*models.MCPConfig, error) {
	query := `SELECT id, environment_id, version, config_json, encryption_key_id, created_at, updated_at 
			  FROM mcp_configs 
			  WHERE environment_id = ? 
			  ORDER BY version DESC 
			  LIMIT 1`
	
	var config models.MCPConfig
	err := r.db.QueryRow(query, environmentID).Scan(
		&config.ID, &config.EnvironmentID, &config.Version, &config.ConfigJSON,
		&config.EncryptionKeyID, &config.CreatedAt, &config.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &config, nil
}

func (r *MCPConfigRepo) GetByVersion(environmentID, version int64) (*models.MCPConfig, error) {
	query := `SELECT id, environment_id, version, config_json, encryption_key_id, created_at, updated_at 
			  FROM mcp_configs 
			  WHERE environment_id = ? AND version = ?`
	
	var config models.MCPConfig
	err := r.db.QueryRow(query, environmentID, version).Scan(
		&config.ID, &config.EnvironmentID, &config.Version, &config.ConfigJSON,
		&config.EncryptionKeyID, &config.CreatedAt, &config.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &config, nil
}

func (r *MCPConfigRepo) ListByEnvironment(environmentID int64) ([]*models.MCPConfig, error) {
	query := `SELECT id, environment_id, version, config_json, encryption_key_id, created_at, updated_at 
			  FROM mcp_configs 
			  WHERE environment_id = ? 
			  ORDER BY version DESC`
	
	rows, err := r.db.Query(query, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var configs []*models.MCPConfig
	for rows.Next() {
		var config models.MCPConfig
		err := rows.Scan(&config.ID, &config.EnvironmentID, &config.Version, &config.ConfigJSON,
			&config.EncryptionKeyID, &config.CreatedAt, &config.UpdatedAt)
		if err != nil {
			return nil, err
		}
		configs = append(configs, &config)
	}
	
	return configs, rows.Err()
}

func (r *MCPConfigRepo) GetNextVersion(environmentID int64) (int64, error) {
	query := `SELECT COALESCE(MAX(version), 0) + 1 as next_version FROM mcp_configs WHERE environment_id = ?`
	
	var nextVersion int64
	err := r.db.QueryRow(query, environmentID).Scan(&nextVersion)
	if err != nil {
		return 0, err
	}
	
	return nextVersion, nil
}

func (r *MCPConfigRepo) Delete(id int64) error {
	query := `DELETE FROM mcp_configs WHERE id = ?`
	_, err := r.db.Exec(query, id)
	return err
}