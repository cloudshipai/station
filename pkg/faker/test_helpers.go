package faker

import (
	"database/sql"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB creates an in-memory SQLite database with the required schema for testing
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Enable foreign key constraints (required for CASCADE DELETE to work)
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	// Create tables
	schema := `
	CREATE TABLE faker_sessions (
		id TEXT PRIMARY KEY,
		instruction TEXT,
		created_at DATETIME,
		updated_at DATETIME
	);

	CREATE TABLE faker_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT,
		tool_name TEXT,
		arguments TEXT,
		response TEXT,
		operation_type TEXT,
		timestamp DATETIME,
		FOREIGN KEY(session_id) REFERENCES faker_sessions(id) ON DELETE CASCADE
	);
	`

	_, err = db.Exec(schema)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return db
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
