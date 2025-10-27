package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestDB represents a test database instance that implements Database interface
type TestDB struct {
	db *DB
}

// NewTest creates a new test database with migrations
func NewTest(tb testing.TB) (*TestDB, error) {
	tempDir := tb.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := New(dbPath)
	if err != nil {
		return nil, err
	}

	// Run migrations
	if err := RunMigrations(database.conn); err != nil {
		database.Close()
		return nil, err
	}

	return &TestDB{
		db: database,
	}, nil
}

// Conn returns the SQL connection (implements Database interface)
func (tdb *TestDB) Conn() *sql.DB {
	return tdb.db.conn
}

// Close closes the test database (implements Database interface)
func (tdb *TestDB) Close() error {
	return tdb.db.Close()
}

// Migrate runs migrations (implements Database interface)
func (tdb *TestDB) Migrate() error {
	return RunMigrations(tdb.db.conn)
}

// GetConnection returns the SQL connection (deprecated - use Conn() instead)
func (tdb *TestDB) GetConnection() *sql.DB {
	return tdb.db.conn
}
