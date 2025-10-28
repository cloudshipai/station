package repositories

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"station/internal/db"
)

func setupFileConfigRepoTest(t *testing.T) (*sql.DB, *FileMCPConfigRepo) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Run migrations
	if err := db.RunMigrations(database.Conn()); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := NewFileMCPConfigRepo(database.Conn())
	return database.Conn(), repo
}

func TestFileMCPConfigRepo_Create(t *testing.T) {
	db, repo := setupFileConfigRepoTest(t)
	defer func() { _ = db.Close() }()

	// Create test environment first
	envRepo := NewEnvironmentRepo(db)
	env, err := envRepo.Create("test-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create test environment: %v", err)
	}
	envID := env.ID

	record := &FileConfigRecord{
		EnvironmentID:            envID,
		ConfigName:               "test-config",
		TemplatePath:             "/test/templates/config.json",
		VariablesPath:            "/test/environments/dev/variables.yml",
		TemplateSpecificVarsPath: "/test/environments/dev/config.vars.yml",
		TemplateHash:             "template-hash-123",
		VariablesHash:            "vars-hash-456",
		TemplateVarsHash:         "template-vars-hash-789",
		Metadata:                 `{"created_by": "test"}`,
	}

	id, err := repo.Create(record)
	if err != nil {
		t.Fatalf("Failed to create file config record: %v", err)
	}

	if id <= 0 {
		t.Errorf("Expected positive ID, got %d", id)
	}

	// Verify record was created
	retrieved, err := repo.GetByID(id)
	if err != nil {
		t.Fatalf("Failed to retrieve created record: %v", err)
	}

	if retrieved.ConfigName != record.ConfigName {
		t.Errorf("Expected config name '%s', got '%s'", record.ConfigName, retrieved.ConfigName)
	}

	if retrieved.TemplatePath != record.TemplatePath {
		t.Errorf("Expected template path '%s', got '%s'", record.TemplatePath, retrieved.TemplatePath)
	}

	if retrieved.TemplateHash != record.TemplateHash {
		t.Errorf("Expected template hash '%s', got '%s'", record.TemplateHash, retrieved.TemplateHash)
	}
}

func TestFileMCPConfigRepo_GetByEnvironmentAndName(t *testing.T) {
	db, repo := setupFileConfigRepoTest(t)
	defer func() { _ = db.Close() }()

	// Create test environment
	envRepo := NewEnvironmentRepo(db)
	env, err := envRepo.Create("test-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create test environment: %v", err)
	}
	envID := env.ID

	// Create multiple configs
	config1 := &FileConfigRecord{
		EnvironmentID: envID,
		ConfigName:    "config1",
		TemplatePath:  "/test/config1.json",
		VariablesPath: "/test/vars.yml",
		TemplateHash:  "hash1",
		VariablesHash: "vars1",
		Metadata:      "{}",
	}

	config2 := &FileConfigRecord{
		EnvironmentID: envID,
		ConfigName:    "config2",
		TemplatePath:  "/test/config2.json",
		VariablesPath: "/test/vars.yml",
		TemplateHash:  "hash2",
		VariablesHash: "vars2",
		Metadata:      "{}",
	}

	id1, err := repo.Create(config1)
	if err != nil {
		t.Fatalf("Failed to create config1: %v", err)
	}

	id2, err := repo.Create(config2)
	if err != nil {
		t.Fatalf("Failed to create config2: %v", err)
	}

	// Test retrieval by environment and name
	retrieved1, err := repo.GetByEnvironmentAndName(envID, "config1")
	if err != nil {
		t.Fatalf("Failed to get config1: %v", err)
	}

	if retrieved1.ID != id1 {
		t.Errorf("Expected ID %d, got %d", id1, retrieved1.ID)
	}

	if retrieved1.ConfigName != "config1" {
		t.Errorf("Expected config name 'config1', got '%s'", retrieved1.ConfigName)
	}

	retrieved2, err := repo.GetByEnvironmentAndName(envID, "config2")
	if err != nil {
		t.Fatalf("Failed to get config2: %v", err)
	}

	if retrieved2.ID != id2 {
		t.Errorf("Expected ID %d, got %d", id2, retrieved2.ID)
	}

	// Test non-existent config
	_, err = repo.GetByEnvironmentAndName(envID, "non-existent")
	if err == nil {
		t.Errorf("Expected error for non-existent config")
	}
}

func TestFileMCPConfigRepo_ListByEnvironment(t *testing.T) {
	db, repo := setupFileConfigRepoTest(t)
	defer func() { _ = db.Close() }()

	// Create test environments
	envRepo := NewEnvironmentRepo(db)
	env1, err := envRepo.Create("env1", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create env1: %v", err)
	}
	env1ID := env1.ID

	env2, err := envRepo.Create("env2", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create env2: %v", err)
	}
	env2ID := env2.ID

	// Create configs in different environments
	configs := []*FileConfigRecord{
		{
			EnvironmentID: env1ID,
			ConfigName:    "config1-env1",
			TemplatePath:  "/test/config1.json",
			VariablesPath: "/test/vars.yml",
			TemplateHash:  "hash1",
			VariablesHash: "vars1",
			Metadata:      "{}",
		},
		{
			EnvironmentID: env1ID,
			ConfigName:    "config2-env1",
			TemplatePath:  "/test/config2.json",
			VariablesPath: "/test/vars.yml",
			TemplateHash:  "hash2",
			VariablesHash: "vars2",
			Metadata:      "{}",
		},
		{
			EnvironmentID: env2ID,
			ConfigName:    "config1-env2",
			TemplatePath:  "/test/config1.json",
			VariablesPath: "/test/vars.yml",
			TemplateHash:  "hash3",
			VariablesHash: "vars3",
			Metadata:      "{}",
		},
	}

	for _, config := range configs {
		_, err := repo.Create(config)
		if err != nil {
			t.Fatalf("Failed to create config %s: %v", config.ConfigName, err)
		}
	}

	// Test listing configs for env1
	env1Configs, err := repo.ListByEnvironment(env1ID)
	if err != nil {
		t.Fatalf("Failed to list env1 configs: %v", err)
	}

	if len(env1Configs) != 2 {
		t.Errorf("Expected 2 configs for env1, got %d", len(env1Configs))
	}

	configNames := make(map[string]bool)
	for _, config := range env1Configs {
		configNames[config.ConfigName] = true
		if config.EnvironmentID != env1ID {
			t.Errorf("Expected environment ID %d, got %d", env1ID, config.EnvironmentID)
		}
	}

	if !configNames["config1-env1"] || !configNames["config2-env1"] {
		t.Errorf("Missing expected config names for env1. Got: %v", configNames)
	}

	// Test listing configs for env2
	env2Configs, err := repo.ListByEnvironment(env2ID)
	if err != nil {
		t.Fatalf("Failed to list env2 configs: %v", err)
	}

	if len(env2Configs) != 1 {
		t.Errorf("Expected 1 config for env2, got %d", len(env2Configs))
	}

	if env2Configs[0].ConfigName != "config1-env2" {
		t.Errorf("Expected config name 'config1-env2', got '%s'", env2Configs[0].ConfigName)
	}
}

func TestFileMCPConfigRepo_UpdateHashes(t *testing.T) {
	db, repo := setupFileConfigRepoTest(t)
	defer func() { _ = db.Close() }()

	// Create test environment
	envRepo := NewEnvironmentRepo(db)
	env, err := envRepo.Create("test-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create test environment: %v", err)
	}
	envID := env.ID

	// Create config
	config := &FileConfigRecord{
		EnvironmentID:    envID,
		ConfigName:       "test-config",
		TemplatePath:     "/test/config.json",
		VariablesPath:    "/test/vars.yml",
		TemplateHash:     "original-hash",
		VariablesHash:    "original-vars-hash",
		TemplateVarsHash: "original-template-vars-hash",
		Metadata:         "{}",
	}

	id, err := repo.Create(config)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Update hashes
	newTemplateHash := "new-template-hash"
	newVarsHash := "new-vars-hash"
	newTemplateVarsHash := "new-template-vars-hash"

	err = repo.UpdateHashes(id, newTemplateHash, newVarsHash, newTemplateVarsHash)
	if err != nil {
		t.Fatalf("Failed to update hashes: %v", err)
	}

	// Verify hashes were updated
	updated, err := repo.GetByID(id)
	if err != nil {
		t.Fatalf("Failed to get updated config: %v", err)
	}

	if updated.TemplateHash != newTemplateHash {
		t.Errorf("Expected template hash '%s', got '%s'", newTemplateHash, updated.TemplateHash)
	}

	if updated.VariablesHash != newVarsHash {
		t.Errorf("Expected variables hash '%s', got '%s'", newVarsHash, updated.VariablesHash)
	}

	if updated.TemplateVarsHash != newTemplateVarsHash {
		t.Errorf("Expected template vars hash '%s', got '%s'", newTemplateVarsHash, updated.TemplateVarsHash)
	}

	// Verify UpdatedAt was updated
	if updated.UpdatedAt.Before(updated.CreatedAt) {
		t.Errorf("Expected UpdatedAt to be after CreatedAt")
	}
}

func TestFileMCPConfigRepo_UpdateLastLoadedAt(t *testing.T) {
	db, repo := setupFileConfigRepoTest(t)
	defer func() { _ = db.Close() }()

	// Create test environment
	envRepo := NewEnvironmentRepo(db)
	env, err := envRepo.Create("test-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create test environment: %v", err)
	}
	envID := env.ID

	// Create config
	config := &FileConfigRecord{
		EnvironmentID: envID,
		ConfigName:    "test-config",
		TemplatePath:  "/test/config.json",
		VariablesPath: "/test/vars.yml",
		TemplateHash:  "hash",
		VariablesHash: "vars-hash",
		Metadata:      "{}",
	}

	id, err := repo.Create(config)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Verify LastLoadedAt is initially nil
	original, err := repo.GetByID(id)
	if err != nil {
		t.Fatalf("Failed to get original config: %v", err)
	}

	if original.LastLoadedAt != nil {
		t.Errorf("Expected LastLoadedAt to be nil initially")
	}

	// Update LastLoadedAt
	beforeUpdate := time.Now().Add(-1 * time.Second) // Add 1 second buffer for DB precision
	err = repo.UpdateLastLoadedAt(id)
	if err != nil {
		t.Fatalf("Failed to update last loaded at: %v", err)
	}
	afterUpdate := time.Now().Add(1 * time.Second) // Add 1 second buffer for DB precision

	// Verify LastLoadedAt was set
	updated, err := repo.GetByID(id)
	if err != nil {
		t.Fatalf("Failed to get updated config: %v", err)
	}

	if updated.LastLoadedAt == nil {
		t.Errorf("Expected LastLoadedAt to be set")
	} else {
		if updated.LastLoadedAt.Before(beforeUpdate) || updated.LastLoadedAt.After(afterUpdate) {
			t.Errorf("LastLoadedAt timestamp not in expected range: got %v, expected between %v and %v",
				updated.LastLoadedAt, beforeUpdate, afterUpdate)
		}
	}
}

func TestFileMCPConfigRepo_Upsert(t *testing.T) {
	db, repo := setupFileConfigRepoTest(t)
	defer func() { _ = db.Close() }()

	// Create test environment
	envRepo := NewEnvironmentRepo(db)
	env, err := envRepo.Create("test-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create test environment: %v", err)
	}
	envID := env.ID

	// Test insert (config doesn't exist)
	config := &FileConfigRecord{
		EnvironmentID:    envID,
		ConfigName:       "test-config",
		TemplatePath:     "/test/config.json",
		VariablesPath:    "/test/vars.yml",
		TemplateHash:     "hash1",
		VariablesHash:    "vars-hash1",
		TemplateVarsHash: "template-vars-hash1",
		Metadata:         "{}",
	}

	id1, err := repo.Upsert(config)
	if err != nil {
		t.Fatalf("Failed to upsert new config: %v", err)
	}

	if id1 <= 0 {
		t.Errorf("Expected positive ID for new config, got %d", id1)
	}

	// Test update (config exists)
	config.TemplateHash = "hash2"
	config.VariablesHash = "vars-hash2"
	config.TemplateVarsHash = "template-vars-hash2"

	id2, err := repo.Upsert(config)
	if err != nil {
		t.Fatalf("Failed to upsert existing config: %v", err)
	}

	if id2 != id1 {
		t.Errorf("Expected same ID for existing config, got %d != %d", id2, id1)
	}

	// Verify config was updated
	updated, err := repo.GetByID(id1)
	if err != nil {
		t.Fatalf("Failed to get updated config: %v", err)
	}

	if updated.TemplateHash != "hash2" {
		t.Errorf("Expected updated template hash 'hash2', got '%s'", updated.TemplateHash)
	}

	if updated.VariablesHash != "vars-hash2" {
		t.Errorf("Expected updated variables hash 'vars-hash2', got '%s'", updated.VariablesHash)
	}
}

func TestFileMCPConfigRepo_HasChanges(t *testing.T) {
	db, repo := setupFileConfigRepoTest(t)
	defer func() { _ = db.Close() }()

	// Create test environment
	envRepo := NewEnvironmentRepo(db)
	env, err := envRepo.Create("test-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create test environment: %v", err)
	}
	envID := env.ID

	// Create config
	config := &FileConfigRecord{
		EnvironmentID:    envID,
		ConfigName:       "test-config",
		TemplatePath:     "/test/config.json",
		VariablesPath:    "/test/vars.yml",
		TemplateHash:     "template-hash-123",
		VariablesHash:    "vars-hash-456",
		TemplateVarsHash: "template-vars-hash-789",
		Metadata:         "{}",
	}

	_, err = repo.Create(config)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Test no changes
	hasChanges, err := repo.HasChanges(envID, "test-config", "template-hash-123", "vars-hash-456", "template-vars-hash-789")
	if err != nil {
		t.Fatalf("Failed to check for changes: %v", err)
	}

	if hasChanges {
		t.Errorf("Expected no changes with matching hashes")
	}

	// Test template hash change
	hasChanges, err = repo.HasChanges(envID, "test-config", "new-template-hash", "vars-hash-456", "template-vars-hash-789")
	if err != nil {
		t.Fatalf("Failed to check for template changes: %v", err)
	}

	if !hasChanges {
		t.Errorf("Expected changes with different template hash")
	}

	// Test variables hash change
	hasChanges, err = repo.HasChanges(envID, "test-config", "template-hash-123", "new-vars-hash", "template-vars-hash-789")
	if err != nil {
		t.Fatalf("Failed to check for variables changes: %v", err)
	}

	if !hasChanges {
		t.Errorf("Expected changes with different variables hash")
	}

	// Test template vars hash change
	hasChanges, err = repo.HasChanges(envID, "test-config", "template-hash-123", "vars-hash-456", "new-template-vars-hash")
	if err != nil {
		t.Fatalf("Failed to check for template vars changes: %v", err)
	}

	if !hasChanges {
		t.Errorf("Expected changes with different template vars hash")
	}

	// Test non-existent config
	hasChanges, err = repo.HasChanges(envID, "non-existent", "hash", "vars", "template-vars")
	if err != nil {
		t.Fatalf("Failed to check changes for non-existent config: %v", err)
	}

	if !hasChanges {
		t.Errorf("Expected changes for non-existent config")
	}
}
