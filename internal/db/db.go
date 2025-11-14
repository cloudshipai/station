package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func New(databaseURL string) (*DB, error) {
	// Detect database type from URL scheme
	var conn *sql.DB
	var err error
	isLibSQL := strings.HasPrefix(databaseURL, "libsql://") || strings.HasPrefix(databaseURL, "http://") || strings.HasPrefix(databaseURL, "https://")

	if isLibSQL {
		// Turso/libsql connection - expects URL format: libsql://host?authToken=token
		// or can be constructed from separate URL and token
		conn, err = sql.Open("libsql", databaseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to open libsql database: %w", err)
		}

		// Configure connection pool for cloud database
		conn.SetMaxOpenConns(25) // Higher limit for remote connections
		conn.SetMaxIdleConns(10)
		conn.SetConnMaxLifetime(5 * time.Minute)

		// Test connection
		if err := conn.Ping(); err != nil {
			return nil, fmt.Errorf("failed to connect to libsql database: %w", err)
		}

		return &DB{conn: conn}, nil
	}

	// Local SQLite file connection
	// Ensure the directory exists before creating the database
	dbDir := filepath.Dir(databaseURL)
	if dbDir != "." && dbDir != "" {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory %s: %w", dbDir, err)
		}
	}

	// Retry connection with exponential backoff for concurrent access
	maxRetries := 5
	baseDelay := 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		conn, err = sql.Open("sqlite", databaseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to open database: %w", err)
		}

		// Configure connection pool for concurrency
		conn.SetMaxOpenConns(10) // Allow up to 10 concurrent connections
		conn.SetMaxIdleConns(5)  // Keep 5 connections in idle pool

		// Try to ping with retry logic
		if err := conn.Ping(); err != nil {
			if attempt == maxRetries-1 {
				return nil, fmt.Errorf("failed to ping database after %d attempts: %w", maxRetries, err)
			}

			conn.Close()                                         // Close failed connection
			delay := baseDelay * time.Duration(1<<uint(attempt)) // Exponential backoff
			time.Sleep(delay)
			continue
		}

		// Connection successful
		break
	}

	// Enable foreign key constraints for SQLite
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign key constraints: %w", err)
	}

	// Enable WAL mode for better concurrency (allows multiple readers + 1 writer)
	if _, err := conn.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Set busy timeout to 30 seconds (wait for locked database)
	if _, err := conn.Exec("PRAGMA busy_timeout = 30000"); err != nil {
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	// Enable optimized settings for concurrent access
	if _, err := conn.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		return nil, fmt.Errorf("failed to set synchronous mode: %w", err)
	}

	if _, err := conn.Exec("PRAGMA cache_size = -64000"); err != nil { // 64MB cache
		return nil, fmt.Errorf("failed to set cache size: %w", err)
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
