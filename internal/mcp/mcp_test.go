package mcp

import (
	"database/sql"
	"path/filepath"
	"testing"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/crypto"
)

func setupTestMCPServer(t *testing.T) (*Server, *db.DB) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Run migrations to set up schema
	if err := db.RunMigrations(database.Conn()); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create key manager for encryption
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	keyManager := crypto.NewKeyManager(key)

	// Create repositories
	repos := repositories.New(&mockDB{conn: database.Conn()})

	// Create MCP config service
	mcpConfigSvc := services.NewMCPConfigService(repos, keyManager)

	// Create MCP server
	server := NewServer(database, mcpConfigSvc)

	return server, database
}

// TestNewServer tests creating a new MCP server
func TestNewServer(t *testing.T) {
	server, db := setupTestMCPServer(t)
	defer db.Close()

	if server == nil {
		t.Fatal("Expected server to be created")
	}

	if server.mcpServer == nil {
		t.Fatal("Expected MCP server to be initialized")
	}

	if server.httpServer == nil {
		t.Fatal("Expected HTTP server to be initialized")
	}

	if server.toolDiscoverySvc == nil {
		t.Fatal("Expected tool discovery service to be initialized")
	}
}

// TestToolDiscoveryService tests the tool discovery functionality
func TestToolDiscoveryService(t *testing.T) {
	server, db := setupTestMCPServer(t)
	defer db.Close()

	if server.toolDiscoverySvc == nil {
		t.Fatal("Expected tool discovery service to be initialized")
	}

	// Test that we can access the service
	tools := server.toolDiscoverySvc.GetDiscoveredTools("1")
	
	// Should be empty initially since no tools have been discovered
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools initially, got %d", len(tools))
	}
}

// TestNewToolDiscoveryService tests creating a new tool discovery service
func TestNewToolDiscoveryService(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer database.Close()

	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	keyManager := crypto.NewKeyManager(key)
	
	repos := repositories.New(&mockDB{conn: database.Conn()})
	mcpConfigSvc := services.NewMCPConfigService(repos, keyManager)

	toolSvc := NewToolDiscoveryService(&mockDB{conn: database.Conn()}, mcpConfigSvc)

	if toolSvc == nil {
		t.Fatal("Expected tool discovery service to be created")
	}

	if toolSvc.db == nil {
		t.Fatal("Expected database to be set")
	}

	if toolSvc.mcpConfigSvc == nil {
		t.Fatal("Expected MCP config service to be set")
	}

	if toolSvc.discoveredTools == nil {
		t.Fatal("Expected discovered tools map to be initialized")
	}
}

// mockDB implements the db.Database interface for testing
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