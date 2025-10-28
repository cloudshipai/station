package repositories

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create the test tables
	schema := `
	CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		public_key TEXT NOT NULL,
		is_admin BOOLEAN NOT NULL DEFAULT FALSE,
		api_key TEXT UNIQUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE environments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		description TEXT,
		created_by INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (created_by) REFERENCES users (id)
	);`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	// Create console user for tests
	_, err = db.Exec("INSERT INTO users (username, public_key) VALUES (?, ?)", "console", "test-key")
	if err != nil {
		t.Fatalf("Failed to create console user: %v", err)
	}

	return db
}

func TestEnvironmentRepo_Create(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := New(&mockDB{conn: db})
	repo := repos.Environments

	// Get console user for created_by field
	consoleUser, err := repos.Users.GetByUsername("console")
	if err != nil {
		t.Fatalf("Failed to get console user: %v", err)
	}

	name := "test-env"
	description := "Test environment"

	env, err := repo.Create(name, &description, consoleUser.ID)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	if env.ID == 0 {
		t.Error("Expected environment ID to be set")
	}
	if env.Name != name {
		t.Errorf("Expected name '%s', got '%s'", name, env.Name)
	}
	if env.Description == nil || *env.Description != description {
		t.Errorf("Expected description '%s', got %v", description, env.Description)
	}
	if env.CreatedAt.IsZero() {
		t.Error("Expected created_at to be set")
	}
	if env.UpdatedAt.IsZero() {
		t.Error("Expected updated_at to be set")
	}
}

func TestEnvironmentRepo_Create_WithoutDescription(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := New(&mockDB{conn: db})
	repo := repos.Environments

	// Get console user for created_by field
	consoleUser, err := repos.Users.GetByUsername("console")
	if err != nil {
		t.Fatalf("Failed to get console user: %v", err)
	}

	name := "test-env-no-desc"

	env, err := repo.Create(name, nil, consoleUser.ID)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	if env.Description != nil {
		t.Errorf("Expected description to be nil, got %v", env.Description)
	}
}

func TestEnvironmentRepo_GetByID(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := New(&mockDB{conn: db})
	repo := repos.Environments

	// Get console user for created_by field
	consoleUser, err := repos.Users.GetByUsername("console")
	if err != nil {
		t.Fatalf("Failed to get console user: %v", err)
	}

	// Create an environment first
	name := "test-env"
	description := "Test environment"
	created, err := repo.Create(name, &description, consoleUser.ID)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Retrieve it by ID
	retrieved, err := repo.GetByID(created.ID)
	if err != nil {
		t.Fatalf("Failed to get environment by ID: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("Expected ID %d, got %d", created.ID, retrieved.ID)
	}
	if retrieved.Name != created.Name {
		t.Errorf("Expected name '%s', got '%s'", created.Name, retrieved.Name)
	}
}

func TestEnvironmentRepo_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := New(&mockDB{conn: db})
	repo := repos.Environments

	_, err := repo.GetByID(999)
	if err != sql.ErrNoRows {
		t.Errorf("Expected sql.ErrNoRows, got %v", err)
	}
}

func TestEnvironmentRepo_GetByName(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := New(&mockDB{conn: db})
	repo := repos.Environments

	// Get console user for created_by field
	consoleUser, err := repos.Users.GetByUsername("console")
	if err != nil {
		t.Fatalf("Failed to get console user: %v", err)
	}

	// Create an environment first
	name := "test-env"
	description := "Test environment"
	created, err := repo.Create(name, &description, consoleUser.ID)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Retrieve it by name
	retrieved, err := repo.GetByName(name)
	if err != nil {
		t.Fatalf("Failed to get environment by name: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("Expected ID %d, got %d", created.ID, retrieved.ID)
	}
	if retrieved.Name != created.Name {
		t.Errorf("Expected name '%s', got '%s'", created.Name, retrieved.Name)
	}
}

func TestEnvironmentRepo_List(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := New(&mockDB{conn: db})
	repo := repos.Environments

	// Get console user for created_by field
	consoleUser, err := repos.Users.GetByUsername("console")
	if err != nil {
		t.Fatalf("Failed to get console user: %v", err)
	}

	// Create multiple environments
	envs := []struct {
		name string
		desc string
	}{
		{"env1", "Environment 1"},
		{"env2", "Environment 2"},
		{"env3", "Environment 3"},
	}

	for _, env := range envs {
		_, err := repo.Create(env.name, &env.desc, consoleUser.ID)
		if err != nil {
			t.Fatalf("Failed to create environment %s: %v", env.name, err)
		}
	}

	// List all environments
	list, err := repo.List()
	if err != nil {
		t.Fatalf("Failed to list environments: %v", err)
	}

	if len(list) != len(envs) {
		t.Errorf("Expected %d environments, got %d", len(envs), len(list))
	}

	// Check that they're sorted by name
	for i := 0; i < len(list)-1; i++ {
		if list[i].Name > list[i+1].Name {
			t.Error("Environments are not sorted by name")
			break
		}
	}
}

func TestEnvironmentRepo_Update(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := New(&mockDB{conn: db})
	repo := repos.Environments

	// Get console user for created_by field
	consoleUser, err := repos.Users.GetByUsername("console")
	if err != nil {
		t.Fatalf("Failed to get console user: %v", err)
	}

	// Create an environment first
	name := "test-env"
	description := "Test environment"
	created, err := repo.Create(name, &description, consoleUser.ID)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Update it
	newName := "updated-env"
	newDescription := "Updated environment"
	err = repo.Update(created.ID, newName, &newDescription)
	if err != nil {
		t.Fatalf("Failed to update environment: %v", err)
	}

	// Retrieve and verify the update
	updated, err := repo.GetByID(created.ID)
	if err != nil {
		t.Fatalf("Failed to get updated environment: %v", err)
	}

	if updated.Name != newName {
		t.Errorf("Expected name '%s', got '%s'", newName, updated.Name)
	}
	if updated.Description == nil || *updated.Description != newDescription {
		t.Errorf("Expected description '%s', got %v", newDescription, updated.Description)
	}
}

func TestEnvironmentRepo_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := New(&mockDB{conn: db})
	repo := repos.Environments

	// Get console user for created_by field
	consoleUser, err := repos.Users.GetByUsername("console")
	if err != nil {
		t.Fatalf("Failed to get console user: %v", err)
	}

	// Create an environment first
	name := "test-env"
	description := "Test environment"
	created, err := repo.Create(name, &description, consoleUser.ID)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Delete it
	err = repo.Delete(created.ID)
	if err != nil {
		t.Fatalf("Failed to delete environment: %v", err)
	}

	// Verify it's gone
	_, err = repo.GetByID(created.ID)
	if err != sql.ErrNoRows {
		t.Errorf("Expected sql.ErrNoRows after deletion, got %v", err)
	}
}

// mockDB is a simple wrapper to implement the DB interface for tests
type mockDB struct {
	conn *sql.DB
}

func (m *mockDB) Conn() *sql.DB {
	return m.conn
}

func (m *mockDB) Close() error {
	return m.conn.Close()
}

func (m *mockDB) Migrate() error {
	return nil
}

func (m *mockDB) MigrateFromDir(dir string) error {
	return nil
}
