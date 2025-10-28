package db

import (
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	// Create a temporary database file
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if db.conn == nil {
		t.Error("Database connection should not be nil")
	}

	// Test that we can ping the database
	if err := db.conn.Ping(); err != nil {
		t.Errorf("Failed to ping database: %v", err)
	}
}

func TestNew_InvalidPath(t *testing.T) {
	// Try to create database in non-existent directory with invalid path
	_, err := New("/invalid/path/that/does/not/exist/test.db")
	if err == nil {
		t.Error("Expected error when creating database with invalid path")
	}
}

func TestRunMigrations(t *testing.T) {
	// Create a temporary database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Run embedded migrations
	if err := RunMigrations(db.conn); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Verify some expected tables were created from our embedded migrations
	expectedTables := []string{"users", "environments", "file_mcp_configs", "agents"}

	for _, tableName := range expectedTables {
		var name string
		err = db.conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&name)
		if err != nil {
			t.Fatalf("Failed to find expected table '%s': %v", tableName, err)
		}
		if name != tableName {
			t.Errorf("Expected table name '%s', got '%s'", tableName, name)
		}
	}
}

func TestClose(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	// Close the database
	if err := db.Close(); err != nil {
		t.Errorf("Failed to close database: %v", err)
	}

	// Verify that subsequent operations fail
	if err := db.conn.Ping(); err == nil {
		t.Error("Expected ping to fail after closing database")
	}
}
