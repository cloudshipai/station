package repositories

import (
	"context"
	"database/sql"
	"station/internal/db/queries"
	"station/pkg/models"
)

type MCPConfigRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewMCPConfigRepo(db *sql.DB) *MCPConfigRepo {
	return &MCPConfigRepo{
		db:      db,
		queries: queries.New(db),
	}
}

// convertMCPConfigFromSQLc converts sqlc McpConfig to models.MCPConfig
func convertMCPConfigFromSQLc(config queries.McpConfig) *models.MCPConfig {
	result := &models.MCPConfig{
		ID:            config.ID,
		EnvironmentID: config.EnvironmentID,
		ConfigName:    config.ConfigName,
		Version:       config.Version,
		ConfigJSON:    config.ConfigJson,
	}
	
	if config.EncryptionKeyID.Valid {
		result.EncryptionKeyID = config.EncryptionKeyID.String
	}
	
	if config.CreatedAt.Valid {
		result.CreatedAt = config.CreatedAt.Time
	}
	
	if config.UpdatedAt.Valid {
		result.UpdatedAt = config.UpdatedAt.Time
	}
	
	return result
}

// convertMCPConfigToCreateParams converts parameters to sqlc CreateMCPConfigParams
func convertMCPConfigToCreateParams(environmentID int64, version int64, configJSON, encryptionKeyID string) queries.CreateMCPConfigParams {
	return queries.CreateMCPConfigParams{
		EnvironmentID:   environmentID,
		Version:         version,
		ConfigJson:      configJSON,
		EncryptionKeyID: sql.NullString{String: encryptionKeyID, Valid: encryptionKeyID != ""},
	}
}

func (r *MCPConfigRepo) Create(environmentID int64, configName string, version int64, configJSON, encryptionKeyID string) (*models.MCPConfig, error) {
	// Since sqlc doesn't include config_name in the create params yet, we'll use a direct query
	// TODO: Update this to use sqlc once the generated code includes config_name
	query := `INSERT INTO mcp_configs (environment_id, config_name, version, config_json, encryption_key_id) 
			  VALUES (?, ?, ?, ?, ?) 
			  RETURNING id, environment_id, config_name, version, config_json, encryption_key_id, created_at, updated_at`
	
	var sqlcConfig queries.McpConfig
	err := r.db.QueryRow(query, environmentID, configName, version, configJSON, encryptionKeyID).Scan(
		&sqlcConfig.ID, &sqlcConfig.EnvironmentID, &sqlcConfig.ConfigName, &sqlcConfig.Version, &sqlcConfig.ConfigJson, 
		&sqlcConfig.EncryptionKeyID, &sqlcConfig.CreatedAt, &sqlcConfig.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return convertMCPConfigFromSQLc(sqlcConfig), nil
}

// RotateEncryptionKey re-encrypts all configs with a new key
func (r *MCPConfigRepo) RotateEncryptionKey(oldKeyID, newKeyID string, reencryptFunc func([]byte, string, string) ([]byte, error)) error {
	// This method uses custom queries as sqlc doesn't have the exact queries we need
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

	// Re-encrypt each config using custom update query
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
	config, err := r.queries.GetMCPConfig(context.Background(), id)
	if err != nil {
		return nil, err
	}
	
	return convertMCPConfigFromSQLc(config), nil
}

// GetLatest returns the latest version of any named config in the environment
// This is used for backward compatibility
func (r *MCPConfigRepo) GetLatest(environmentID int64) (*models.MCPConfig, error) {
	config, err := r.queries.GetLatestMCPConfig(context.Background(), environmentID)
	if err != nil {
		return nil, err
	}
	
	return convertMCPConfigFromSQLc(config), nil
}

// GetLatestByName returns the latest version of a specific named config
func (r *MCPConfigRepo) GetLatestByName(environmentID int64, configName string) (*models.MCPConfig, error) {
	// Using custom query for now as sqlc doesn't have this specific query yet
	query := `SELECT id, environment_id, config_name, version, config_json, encryption_key_id, created_at, updated_at 
			  FROM mcp_configs 
			  WHERE environment_id = ? AND config_name = ? 
			  ORDER BY version DESC 
			  LIMIT 1`
	
	var sqlcConfig queries.McpConfig
	err := r.db.QueryRow(query, environmentID, configName).Scan(
		&sqlcConfig.ID, &sqlcConfig.EnvironmentID, &sqlcConfig.ConfigName, &sqlcConfig.Version, &sqlcConfig.ConfigJson,
		&sqlcConfig.EncryptionKeyID, &sqlcConfig.CreatedAt, &sqlcConfig.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return convertMCPConfigFromSQLc(sqlcConfig), nil
}

// GetLatestConfigs returns the latest version of each named config in the environment
func (r *MCPConfigRepo) GetLatestConfigs(environmentID int64) ([]*models.MCPConfig, error) {
	// Using custom query for now as sqlc doesn't have this specific query yet
	query := `SELECT id, environment_id, config_name, version, config_json, encryption_key_id, created_at, updated_at 
			  FROM mcp_configs c1
			  WHERE environment_id = ? 
			  AND version = (
				  SELECT MAX(version) 
				  FROM mcp_configs c2 
				  WHERE c2.environment_id = c1.environment_id 
				  AND c2.config_name = c1.config_name
			  )
			  ORDER BY config_name, version DESC`
	
	rows, err := r.db.Query(query, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var configs []*models.MCPConfig
	for rows.Next() {
		var sqlcConfig queries.McpConfig
		err := rows.Scan(&sqlcConfig.ID, &sqlcConfig.EnvironmentID, &sqlcConfig.ConfigName, &sqlcConfig.Version, &sqlcConfig.ConfigJson,
			&sqlcConfig.EncryptionKeyID, &sqlcConfig.CreatedAt, &sqlcConfig.UpdatedAt)
		if err != nil {
			return nil, err
		}
		configs = append(configs, convertMCPConfigFromSQLc(sqlcConfig))
	}
	
	return configs, rows.Err()
}

// GetAllLatestConfigs returns the latest version of each named config across ALL environments
// This is used for cross-environment MCP initialization
func (r *MCPConfigRepo) GetAllLatestConfigs() ([]*models.MCPConfig, error) {
	// Using custom query for now as sqlc doesn't have this specific query yet
	query := `SELECT id, environment_id, config_name, version, config_json, encryption_key_id, created_at, updated_at 
			  FROM mcp_configs c1
			  WHERE version = (
				  SELECT MAX(version) 
				  FROM mcp_configs c2 
				  WHERE c2.environment_id = c1.environment_id 
				  AND c2.config_name = c1.config_name
			  )
			  ORDER BY environment_id, config_name, version DESC`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var configs []*models.MCPConfig
	for rows.Next() {
		var sqlcConfig queries.McpConfig
		err := rows.Scan(&sqlcConfig.ID, &sqlcConfig.EnvironmentID, &sqlcConfig.ConfigName, &sqlcConfig.Version, &sqlcConfig.ConfigJson,
			&sqlcConfig.EncryptionKeyID, &sqlcConfig.CreatedAt, &sqlcConfig.UpdatedAt)
		if err != nil {
			return nil, err
		}
		configs = append(configs, convertMCPConfigFromSQLc(sqlcConfig))
	}
	
	return configs, rows.Err()
}

func (r *MCPConfigRepo) GetByVersion(environmentID int64, configName string, version int64) (*models.MCPConfig, error) {
	// Using custom query as sqlc GetMCPConfigByVersion doesn't include config_name in WHERE clause
	query := `SELECT id, environment_id, config_name, version, config_json, encryption_key_id, created_at, updated_at 
			  FROM mcp_configs 
			  WHERE environment_id = ? AND config_name = ? AND version = ?`
	
	var sqlcConfig queries.McpConfig
	err := r.db.QueryRow(query, environmentID, configName, version).Scan(
		&sqlcConfig.ID, &sqlcConfig.EnvironmentID, &sqlcConfig.ConfigName, &sqlcConfig.Version, &sqlcConfig.ConfigJson,
		&sqlcConfig.EncryptionKeyID, &sqlcConfig.CreatedAt, &sqlcConfig.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return convertMCPConfigFromSQLc(sqlcConfig), nil
}

func (r *MCPConfigRepo) ListByEnvironment(environmentID int64) ([]*models.MCPConfig, error) {
	sqlcConfigs, err := r.queries.ListMCPConfigsByEnvironment(context.Background(), environmentID)
	if err != nil {
		return nil, err
	}
	
	var configs []*models.MCPConfig
	for _, sqlcConfig := range sqlcConfigs {
		configs = append(configs, convertMCPConfigFromSQLc(sqlcConfig))
	}
	
	return configs, nil
}

// ListByConfigName returns all versions of a specific named config
func (r *MCPConfigRepo) ListByConfigName(environmentID int64, configName string) ([]*models.MCPConfig, error) {
	// Using custom query for now as sqlc doesn't have this specific query yet
	query := `SELECT id, environment_id, config_name, version, config_json, encryption_key_id, created_at, updated_at 
			  FROM mcp_configs 
			  WHERE environment_id = ? AND config_name = ? 
			  ORDER BY version DESC`
	
	rows, err := r.db.Query(query, environmentID, configName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var configs []*models.MCPConfig
	for rows.Next() {
		var sqlcConfig queries.McpConfig
		err := rows.Scan(&sqlcConfig.ID, &sqlcConfig.EnvironmentID, &sqlcConfig.ConfigName, &sqlcConfig.Version, &sqlcConfig.ConfigJson,
			&sqlcConfig.EncryptionKeyID, &sqlcConfig.CreatedAt, &sqlcConfig.UpdatedAt)
		if err != nil {
			return nil, err
		}
		configs = append(configs, convertMCPConfigFromSQLc(sqlcConfig))
	}
	
	return configs, rows.Err()
}

func (r *MCPConfigRepo) GetNextVersion(environmentID int64, configName string) (int64, error) {
	// There's a sqlc query for this, but it doesn't include configName, so using custom query
	query := `SELECT COALESCE(MAX(version), 0) + 1 as next_version FROM mcp_configs WHERE environment_id = ? AND config_name = ?`
	
	var nextVersion int64
	err := r.db.QueryRow(query, environmentID, configName).Scan(&nextVersion)
	if err != nil {
		return 0, err
	}
	
	return nextVersion, nil
}

func (r *MCPConfigRepo) Delete(id int64) error {
	// Using custom exec for now as sqlc doesn't generate this exact method yet
	query := `DELETE FROM mcp_configs WHERE id = ?`
	_, err := r.db.Exec(query, id)
	return err
}

// DeleteTx deletes a config within a transaction
func (r *MCPConfigRepo) DeleteTx(tx *sql.Tx, id int64) error {
	// Using transaction with custom query
	query := `DELETE FROM mcp_configs WHERE id = ?`
	_, err := tx.Exec(query, id)
	return err
}

// UpdateEncryption updates the encryption details for a specific config
func (r *MCPConfigRepo) UpdateEncryption(id int64, configJSON, encryptionKeyID string) error {
	// Using custom update query for now as sqlc doesn't have this specific update method yet
	query := `UPDATE mcp_configs SET config_json = ?, encryption_key_id = ? WHERE id = ?`
	_, err := r.db.Exec(query, configJSON, encryptionKeyID, id)
	return err
}

// ListAll returns all MCP configs across all environments (used for key rotation)
func (r *MCPConfigRepo) ListAll() ([]*models.MCPConfig, error) {
	// Using custom query for now as sqlc doesn't have this specific query yet
	query := `SELECT id, environment_id, config_name, version, config_json, encryption_key_id, created_at, updated_at 
			  FROM mcp_configs 
			  ORDER BY environment_id, config_name, version DESC`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var configs []*models.MCPConfig
	for rows.Next() {
		var sqlcConfig queries.McpConfig
		err := rows.Scan(&sqlcConfig.ID, &sqlcConfig.EnvironmentID, &sqlcConfig.ConfigName, &sqlcConfig.Version, &sqlcConfig.ConfigJson,
			&sqlcConfig.EncryptionKeyID, &sqlcConfig.CreatedAt, &sqlcConfig.UpdatedAt)
		if err != nil {
			return nil, err
		}
		configs = append(configs, convertMCPConfigFromSQLc(sqlcConfig))
	}
	
	return configs, rows.Err()
}