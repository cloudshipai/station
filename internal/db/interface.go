package db

import "database/sql"

// Database interface for dependency injection and testing
type Database interface {
	Conn() *sql.DB
	Close() error
	Migrate() error
}

// Ensure DB implements Database interface
var _ Database = (*DB)(nil)
