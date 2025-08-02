package repositories

import (
	"database/sql"
	"time"
)

// FileConfigRecord represents a file-based MCP configuration record
type FileConfigRecord struct {
	ID                       int64     `db:"id"`
	EnvironmentID            int64     `db:"environment_id"`
	ConfigName               string    `db:"config_name"`
	TemplatePath             string    `db:"template_path"`
	VariablesPath            string    `db:"variables_path"`
	TemplateSpecificVarsPath string    `db:"template_specific_vars_path"`
	LastLoadedAt             *time.Time `db:"last_loaded_at"`
	TemplateHash             string    `db:"template_hash"`
	VariablesHash            string    `db:"variables_hash"`
	TemplateVarsHash         string    `db:"template_vars_hash"`
	Metadata                 string    `db:"metadata"`
	CreatedAt                time.Time `db:"created_at"`
	UpdatedAt                time.Time `db:"updated_at"`
}

// FileMCPConfigRepo manages file-based MCP configuration records
type FileMCPConfigRepo struct {
	db *sql.DB
}

// NewFileMCPConfigRepo creates a new file MCP config repository
func NewFileMCPConfigRepo(db *sql.DB) *FileMCPConfigRepo {
	return &FileMCPConfigRepo{db: db}
}

// Create creates a new file MCP config record
func (r *FileMCPConfigRepo) Create(record *FileConfigRecord) (int64, error) {
	query := `
		INSERT INTO file_mcp_configs (
			environment_id, config_name, template_path, variables_path,
			template_specific_vars_path, template_hash, variables_hash,
			template_vars_hash, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`
	
	var id int64
	err := r.db.QueryRow(
		query,
		record.EnvironmentID,
		record.ConfigName,
		record.TemplatePath,
		record.VariablesPath,
		record.TemplateSpecificVarsPath,
		record.TemplateHash,
		record.VariablesHash,
		record.TemplateVarsHash,
		record.Metadata,
	).Scan(&id)
	
	return id, err
}

// GetByID retrieves a file config record by ID
func (r *FileMCPConfigRepo) GetByID(id int64) (*FileConfigRecord, error) {
	query := `
		SELECT id, environment_id, config_name, template_path, variables_path,
			   template_specific_vars_path, last_loaded_at, template_hash,
			   variables_hash, template_vars_hash, metadata, created_at, updated_at
		FROM file_mcp_configs
		WHERE id = ?
	`
	
	record := &FileConfigRecord{}
	err := r.db.QueryRow(query, id).Scan(
		&record.ID,
		&record.EnvironmentID,
		&record.ConfigName,
		&record.TemplatePath,
		&record.VariablesPath,
		&record.TemplateSpecificVarsPath,
		&record.LastLoadedAt,
		&record.TemplateHash,
		&record.VariablesHash,
		&record.TemplateVarsHash,
		&record.Metadata,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	return record, nil
}

// GetByEnvironmentAndName retrieves a file config by environment and name
func (r *FileMCPConfigRepo) GetByEnvironmentAndName(environmentID int64, configName string) (*FileConfigRecord, error) {
	query := `
		SELECT id, environment_id, config_name, template_path, variables_path,
			   template_specific_vars_path, last_loaded_at, template_hash,
			   variables_hash, template_vars_hash, metadata, created_at, updated_at
		FROM file_mcp_configs
		WHERE environment_id = ? AND config_name = ?
	`
	
	record := &FileConfigRecord{}
	err := r.db.QueryRow(query, environmentID, configName).Scan(
		&record.ID,
		&record.EnvironmentID,
		&record.ConfigName,
		&record.TemplatePath,
		&record.VariablesPath,
		&record.TemplateSpecificVarsPath,
		&record.LastLoadedAt,
		&record.TemplateHash,
		&record.VariablesHash,
		&record.TemplateVarsHash,
		&record.Metadata,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	return record, nil
}

// ListByEnvironment lists all file configs for an environment
func (r *FileMCPConfigRepo) ListByEnvironment(environmentID int64) ([]*FileConfigRecord, error) {
	query := `
		SELECT id, environment_id, config_name, template_path, variables_path,
			   template_specific_vars_path, last_loaded_at, template_hash,
			   variables_hash, template_vars_hash, metadata, created_at, updated_at
		FROM file_mcp_configs
		WHERE environment_id = ?
		ORDER BY config_name
	`
	
	rows, err := r.db.Query(query, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var records []*FileConfigRecord
	for rows.Next() {
		record := &FileConfigRecord{}
		err := rows.Scan(
			&record.ID,
			&record.EnvironmentID,
			&record.ConfigName,
			&record.TemplatePath,
			&record.VariablesPath,
			&record.TemplateSpecificVarsPath,
			&record.LastLoadedAt,
			&record.TemplateHash,
			&record.VariablesHash,
			&record.TemplateVarsHash,
			&record.Metadata,
			&record.CreatedAt,
			&record.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	
	return records, rows.Err()
}

// UpdateHashes updates the template and variables hashes
func (r *FileMCPConfigRepo) UpdateHashes(id int64, templateHash, variablesHash, templateVarsHash string) error {
	query := `
		UPDATE file_mcp_configs
		SET template_hash = ?, variables_hash = ?, template_vars_hash = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	
	_, err := r.db.Exec(query, templateHash, variablesHash, templateVarsHash, id)
	return err
}

// UpdateLastLoadedAt updates the last loaded timestamp
func (r *FileMCPConfigRepo) UpdateLastLoadedAt(id int64) error {
	query := `
		UPDATE file_mcp_configs
		SET last_loaded_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	
	_, err := r.db.Exec(query, id)
	return err
}

// UpdateMetadata updates the metadata JSON
func (r *FileMCPConfigRepo) UpdateMetadata(id int64, metadata string) error {
	query := `
		UPDATE file_mcp_configs
		SET metadata = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	
	_, err := r.db.Exec(query, metadata, id)
	return err
}

// Upsert creates or updates a file config record
func (r *FileMCPConfigRepo) Upsert(record *FileConfigRecord) (int64, error) {
	// Try to get existing record
	existing, err := r.GetByEnvironmentAndName(record.EnvironmentID, record.ConfigName)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	
	if existing != nil {
		// Update existing record
		err := r.UpdateHashes(existing.ID, record.TemplateHash, record.VariablesHash, record.TemplateVarsHash)
		if err != nil {
			return 0, err
		}
		return existing.ID, nil
	}
	
	// Create new record
	return r.Create(record)
}

// Delete deletes a file config record
func (r *FileMCPConfigRepo) Delete(id int64) error {
	query := `DELETE FROM file_mcp_configs WHERE id = ?`
	_, err := r.db.Exec(query, id)
	return err
}

// DeleteByEnvironmentAndName deletes a file config by environment and name
func (r *FileMCPConfigRepo) DeleteByEnvironmentAndName(environmentID int64, configName string) error {
	query := `DELETE FROM file_mcp_configs WHERE environment_id = ? AND config_name = ?`
	_, err := r.db.Exec(query, environmentID, configName)
	return err
}

// GetStaleConfigs returns configs that need to be re-rendered based on hash changes
func (r *FileMCPConfigRepo) GetStaleConfigs(environmentID int64) ([]*FileConfigRecord, error) {
	// This would compare file hashes with database hashes to find configs that changed
	// For now, return empty list - implementation would need filesystem integration
	return []*FileConfigRecord{}, nil
}

// CreateTx creates a new file config record within a transaction
func (r *FileMCPConfigRepo) CreateTx(tx *sql.Tx, record *FileConfigRecord) (int64, error) {
	query := `
		INSERT INTO file_mcp_configs (
			environment_id, config_name, template_path, variables_path,
			template_specific_vars_path, template_hash, variables_hash,
			template_vars_hash, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`
	
	var id int64
	err := tx.QueryRow(
		query,
		record.EnvironmentID,
		record.ConfigName,
		record.TemplatePath,
		record.VariablesPath,
		record.TemplateSpecificVarsPath,
		record.TemplateHash,
		record.VariablesHash,
		record.TemplateVarsHash,
		record.Metadata,
	).Scan(&id)
	
	return id, err
}

// UpdateHashesTx updates hashes within a transaction
func (r *FileMCPConfigRepo) UpdateHashesTx(tx *sql.Tx, id int64, templateHash, variablesHash, templateVarsHash string) error {
	query := `
		UPDATE file_mcp_configs
		SET template_hash = ?, variables_hash = ?, template_vars_hash = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	
	_, err := tx.Exec(query, templateHash, variablesHash, templateVarsHash, id)
	return err
}

// GetConfigsForChangeDetection gets configs with their hashes for change detection
func (r *FileMCPConfigRepo) GetConfigsForChangeDetection(environmentID int64) (map[string]*FileConfigRecord, error) {
	query := `
		SELECT id, environment_id, config_name, template_path, variables_path,
			   template_hash, variables_hash, template_vars_hash, last_loaded_at
		FROM file_mcp_configs
		WHERE environment_id = ?
	`
	
	rows, err := r.db.Query(query, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	configs := make(map[string]*FileConfigRecord)
	for rows.Next() {
		record := &FileConfigRecord{}
		err := rows.Scan(
			&record.ID,
			&record.EnvironmentID,
			&record.ConfigName,
			&record.TemplatePath,
			&record.VariablesPath,
			&record.TemplateHash,
			&record.VariablesHash,
			&record.TemplateVarsHash,
			&record.LastLoadedAt,
		)
		if err != nil {
			return nil, err
		}
		configs[record.ConfigName] = record
	}
	
	return configs, rows.Err()
}

// HasChanges checks if template or variables have changed since last load
func (r *FileMCPConfigRepo) HasChanges(environmentID int64, configName, templateHash, variablesHash, templateVarsHash string) (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM file_mcp_configs
		WHERE environment_id = ? AND config_name = ?
		  AND template_hash = ? AND variables_hash = ? AND template_vars_hash = ?
	`
	
	var count int
	err := r.db.QueryRow(query, environmentID, configName, templateHash, variablesHash, templateVarsHash).Scan(&count)
	if err != nil {
		return false, err
	}
	
	// If count is 0, it means the hashes don't match (have changes)
	return count == 0, nil
}