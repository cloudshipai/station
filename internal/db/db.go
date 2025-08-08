package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func New(databaseURL string) (*DB, error) {
	// Ensure the directory exists before creating the database
	dbDir := filepath.Dir(databaseURL)
	if dbDir != "." && dbDir != "" {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory %s: %w", dbDir, err)
		}
	}

	conn, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Enable foreign key constraints for SQLite
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign key constraints: %w", err)
	}

	return &DB{conn: conn}, nil
}

func (db *DB) Close() error {
	// Set connection limits for faster shutdown
	db.conn.SetMaxOpenConns(0)
	db.conn.SetMaxIdleConns(0)
	db.conn.SetConnMaxLifetime(0)
	
	return db.conn.Close()
}

func (db *DB) Conn() *sql.DB {
	return db.conn
}

// Migrate runs embedded migrations
func (db *DB) Migrate() error {
	return RunMigrations(db.conn)
}