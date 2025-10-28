package repositories

import (
	"context"
	"database/sql"
	"station/internal/db/queries"
	"time"
)

// FileConfigRecord represents a file-based MCP configuration record
type FileConfigRecord struct {
	ID                       int64      `db:"id"`
	EnvironmentID            int64      `db:"environment_id"`
	ConfigName               string     `db:"config_name"`
	TemplatePath             string     `db:"template_path"`
	VariablesPath            string     `db:"variables_path"`
	TemplateSpecificVarsPath string     `db:"template_specific_vars_path"`
	LastLoadedAt             *time.Time `db:"last_loaded_at"`
	TemplateHash             string     `db:"template_hash"`
	VariablesHash            string     `db:"variables_hash"`
	TemplateVarsHash         string     `db:"template_vars_hash"`
	Metadata                 string     `db:"metadata"`
	CreatedAt                time.Time  `db:"created_at"`
	UpdatedAt                time.Time  `db:"updated_at"`
}

// FileMCPConfigRepo manages file-based MCP configuration records using SQLC
type FileMCPConfigRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

// NewFileMCPConfigRepo creates a new file MCP config repository
func NewFileMCPConfigRepo(db *sql.DB) *FileMCPConfigRepo {
	return &FileMCPConfigRepo{
		db:      db,
		queries: queries.New(db),
	}
}

// convertFileMCPConfigFromSQLc converts SQLC FileMcpConfig to FileConfigRecord
func convertFileMCPConfigFromSQLc(config queries.FileMcpConfig) *FileConfigRecord {
	result := &FileConfigRecord{
		ID:                       config.ID,
		EnvironmentID:            config.EnvironmentID,
		ConfigName:               config.ConfigName,
		TemplatePath:             config.TemplatePath,
		VariablesPath:            config.VariablesPath.String,
		TemplateSpecificVarsPath: config.TemplateSpecificVarsPath.String,
		TemplateHash:             config.TemplateHash.String,
		VariablesHash:            config.VariablesHash.String,
		TemplateVarsHash:         config.TemplateVarsHash.String,
		Metadata:                 config.Metadata.String,
	}

	if config.CreatedAt.Valid {
		result.CreatedAt = config.CreatedAt.Time
	}
	if config.UpdatedAt.Valid {
		result.UpdatedAt = config.UpdatedAt.Time
	}
	if config.LastLoadedAt.Valid {
		result.LastLoadedAt = &config.LastLoadedAt.Time
	}

	return result
}

// convertFileMCPConfigToSQLc converts FileConfigRecord to SQLC CreateFileMCPConfigParams
func convertFileMCPConfigToSQLc(record *FileConfigRecord) queries.CreateFileMCPConfigParams {
	return queries.CreateFileMCPConfigParams{
		EnvironmentID:            record.EnvironmentID,
		ConfigName:               record.ConfigName,
		TemplatePath:             record.TemplatePath,
		VariablesPath:            sql.NullString{String: record.VariablesPath, Valid: record.VariablesPath != ""},
		TemplateSpecificVarsPath: sql.NullString{String: record.TemplateSpecificVarsPath, Valid: record.TemplateSpecificVarsPath != ""},
		TemplateHash:             sql.NullString{String: record.TemplateHash, Valid: record.TemplateHash != ""},
		VariablesHash:            sql.NullString{String: record.VariablesHash, Valid: record.VariablesHash != ""},
		TemplateVarsHash:         sql.NullString{String: record.TemplateVarsHash, Valid: record.TemplateVarsHash != ""},
		Metadata:                 sql.NullString{String: record.Metadata, Valid: record.Metadata != ""},
	}
}

// Create creates a new file MCP config record using SQLC
func (r *FileMCPConfigRepo) Create(record *FileConfigRecord) (int64, error) {
	params := convertFileMCPConfigToSQLc(record)
	created, err := r.queries.CreateFileMCPConfig(context.Background(), params)
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

// GetByID retrieves a file config record by ID using SQLC
func (r *FileMCPConfigRepo) GetByID(id int64) (*FileConfigRecord, error) {
	config, err := r.queries.GetFileMCPConfig(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return convertFileMCPConfigFromSQLc(config), nil
}

// GetByEnvironmentAndName retrieves a file config by environment and name using SQLC
func (r *FileMCPConfigRepo) GetByEnvironmentAndName(environmentID int64, configName string) (*FileConfigRecord, error) {
	params := queries.GetFileMCPConfigByEnvironmentAndNameParams{
		EnvironmentID: environmentID,
		ConfigName:    configName,
	}
	config, err := r.queries.GetFileMCPConfigByEnvironmentAndName(context.Background(), params)
	if err != nil {
		return nil, err
	}
	return convertFileMCPConfigFromSQLc(config), nil
}

// ListByEnvironment lists all file configs for an environment using SQLC
func (r *FileMCPConfigRepo) ListByEnvironment(environmentID int64) ([]*FileConfigRecord, error) {
	configs, err := r.queries.ListFileMCPConfigsByEnvironment(context.Background(), environmentID)
	if err != nil {
		return nil, err
	}

	var records []*FileConfigRecord
	for _, config := range configs {
		records = append(records, convertFileMCPConfigFromSQLc(config))
	}

	return records, nil
}

// UpdateHashes updates the template and variables hashes using SQLC
func (r *FileMCPConfigRepo) UpdateHashes(id int64, templateHash, variablesHash, templateVarsHash string) error {
	params := queries.UpdateFileMCPConfigHashesParams{
		TemplateHash:     sql.NullString{String: templateHash, Valid: templateHash != ""},
		VariablesHash:    sql.NullString{String: variablesHash, Valid: variablesHash != ""},
		TemplateVarsHash: sql.NullString{String: templateVarsHash, Valid: templateVarsHash != ""},
		ID:               id,
	}
	return r.queries.UpdateFileMCPConfigHashes(context.Background(), params)
}

// UpdateLastLoadedAt updates the last loaded timestamp using SQLC
func (r *FileMCPConfigRepo) UpdateLastLoadedAt(id int64) error {
	return r.queries.UpdateFileMCPConfigLastLoadedAt(context.Background(), id)
}

// UpdateMetadata updates the metadata JSON using SQLC
func (r *FileMCPConfigRepo) UpdateMetadata(id int64, metadata string) error {
	params := queries.UpdateFileMCPConfigMetadataParams{
		Metadata: sql.NullString{String: metadata, Valid: metadata != ""},
		ID:       id,
	}
	return r.queries.UpdateFileMCPConfigMetadata(context.Background(), params)
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

// Delete deletes a file config record using SQLC
func (r *FileMCPConfigRepo) Delete(id int64) error {
	return r.queries.DeleteFileMCPConfig(context.Background(), id)
}

// DeleteByEnvironmentAndName deletes a file config by environment and name using SQLC
func (r *FileMCPConfigRepo) DeleteByEnvironmentAndName(environmentID int64, configName string) error {
	params := queries.DeleteFileMCPConfigByEnvironmentAndNameParams{
		EnvironmentID: environmentID,
		ConfigName:    configName,
	}
	return r.queries.DeleteFileMCPConfigByEnvironmentAndName(context.Background(), params)
}

// GetStaleConfigs returns configs that need to be re-rendered based on hash changes
func (r *FileMCPConfigRepo) GetStaleConfigs(environmentID int64) ([]*FileConfigRecord, error) {
	// This would compare file hashes with database hashes to find configs that changed
	// For now, return empty list - implementation would need filesystem integration
	return []*FileConfigRecord{}, nil
}

// CreateTx creates a new file config record within a transaction using SQLC
func (r *FileMCPConfigRepo) CreateTx(tx *sql.Tx, record *FileConfigRecord) (int64, error) {
	params := convertFileMCPConfigToSQLc(record)
	txQueries := r.queries.WithTx(tx)
	created, err := txQueries.CreateFileMCPConfig(context.Background(), params)
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

// UpdateHashesTx updates hashes within a transaction using SQLC
func (r *FileMCPConfigRepo) UpdateHashesTx(tx *sql.Tx, id int64, templateHash, variablesHash, templateVarsHash string) error {
	params := queries.UpdateFileMCPConfigHashesParams{
		TemplateHash:     sql.NullString{String: templateHash, Valid: templateHash != ""},
		VariablesHash:    sql.NullString{String: variablesHash, Valid: variablesHash != ""},
		TemplateVarsHash: sql.NullString{String: templateVarsHash, Valid: templateVarsHash != ""},
		ID:               id,
	}
	txQueries := r.queries.WithTx(tx)
	return txQueries.UpdateFileMCPConfigHashes(context.Background(), params)
}

// GetConfigsForChangeDetection gets configs with their hashes for change detection using SQLC
func (r *FileMCPConfigRepo) GetConfigsForChangeDetection(environmentID int64) (map[string]*FileConfigRecord, error) {
	configs, err := r.queries.GetFileMCPConfigsForChangeDetection(context.Background(), environmentID)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*FileConfigRecord)
	for _, config := range configs {
		record := &FileConfigRecord{
			ID:               config.ID,
			EnvironmentID:    config.EnvironmentID,
			ConfigName:       config.ConfigName,
			TemplatePath:     config.TemplatePath,
			VariablesPath:    config.VariablesPath.String,
			TemplateHash:     config.TemplateHash.String,
			VariablesHash:    config.VariablesHash.String,
			TemplateVarsHash: config.TemplateVarsHash.String,
		}
		if config.LastLoadedAt.Valid {
			record.LastLoadedAt = &config.LastLoadedAt.Time
		}
		result[record.ConfigName] = record
	}

	return result, nil
}

// HasChanges checks if template or variables have changed since last load using SQLC
func (r *FileMCPConfigRepo) HasChanges(environmentID int64, configName, templateHash, variablesHash, templateVarsHash string) (bool, error) {
	params := queries.CheckFileMCPConfigChangesParams{
		EnvironmentID:    environmentID,
		ConfigName:       configName,
		TemplateHash:     sql.NullString{String: templateHash, Valid: templateHash != ""},
		VariablesHash:    sql.NullString{String: variablesHash, Valid: variablesHash != ""},
		TemplateVarsHash: sql.NullString{String: templateVarsHash, Valid: templateVarsHash != ""},
	}

	count, err := r.queries.CheckFileMCPConfigChanges(context.Background(), params)
	if err != nil {
		return false, err
	}

	// If count is 0, it means the hashes don't match (have changes)
	return count == 0, nil
}
