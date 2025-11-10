package toolcache

import (
	"context"
	"database/sql"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Create faker_tool_cache table
	_, err = db.Exec(`
		CREATE TABLE faker_tool_cache (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			faker_id TEXT NOT NULL,
			tool_name TEXT NOT NULL,
			tool_schema TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(faker_id, tool_name)
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	return db
}

func TestToolCache_SetAndGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db)
	ctx := context.Background()

	tools := []mcp.Tool{
		{
			Name:        "get_metrics",
			Description: "Get CloudWatch metrics",
		},
		{
			Name:        "list_alarms",
			Description: "List CloudWatch alarms",
		},
	}

	// Set tools
	err := cache.SetTools(ctx, "aws-cloudwatch-faker", tools)
	if err != nil {
		t.Fatalf("SetTools failed: %v", err)
	}

	// Get tools
	retrieved, err := cache.GetTools(ctx, "aws-cloudwatch-faker")
	if err != nil {
		t.Fatalf("GetTools failed: %v", err)
	}

	if len(retrieved) != len(tools) {
		t.Fatalf("expected %d tools, got %d", len(tools), len(retrieved))
	}

	if retrieved[0].Name != "get_metrics" {
		t.Errorf("expected first tool name 'get_metrics', got '%s'", retrieved[0].Name)
	}
}

func TestToolCache_HasTools(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db)
	ctx := context.Background()

	// Check before setting
	has, err := cache.HasTools(ctx, "test-faker")
	if err != nil {
		t.Fatalf("HasTools failed: %v", err)
	}
	if has {
		t.Error("expected no tools, but HasTools returned true")
	}

	// Set tools
	tools := []mcp.Tool{{Name: "test_tool", Description: "Test"}}
	err = cache.SetTools(ctx, "test-faker", tools)
	if err != nil {
		t.Fatalf("SetTools failed: %v", err)
	}

	// Check after setting
	has, err = cache.HasTools(ctx, "test-faker")
	if err != nil {
		t.Fatalf("HasTools failed: %v", err)
	}
	if !has {
		t.Error("expected tools, but HasTools returned false")
	}
}

func TestToolCache_ClearTools(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db)
	ctx := context.Background()

	// Set tools
	tools := []mcp.Tool{{Name: "test_tool", Description: "Test"}}
	err := cache.SetTools(ctx, "test-faker", tools)
	if err != nil {
		t.Fatalf("SetTools failed: %v", err)
	}

	// Clear tools
	err = cache.ClearTools(ctx, "test-faker")
	if err != nil {
		t.Fatalf("ClearTools failed: %v", err)
	}

	// Verify cleared
	has, err := cache.HasTools(ctx, "test-faker")
	if err != nil {
		t.Fatalf("HasTools failed: %v", err)
	}
	if has {
		t.Error("expected no tools after clear, but HasTools returned true")
	}
}

func TestToolCache_ReplaceTools(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db)
	ctx := context.Background()

	// Set initial tools
	tools1 := []mcp.Tool{
		{Name: "tool1", Description: "First"},
		{Name: "tool2", Description: "Second"},
	}
	err := cache.SetTools(ctx, "test-faker", tools1)
	if err != nil {
		t.Fatalf("SetTools failed: %v", err)
	}

	// Replace with different tools
	tools2 := []mcp.Tool{
		{Name: "tool3", Description: "Third"},
	}
	err = cache.SetTools(ctx, "test-faker", tools2)
	if err != nil {
		t.Fatalf("SetTools failed: %v", err)
	}

	// Verify replacement
	retrieved, err := cache.GetTools(ctx, "test-faker")
	if err != nil {
		t.Fatalf("GetTools failed: %v", err)
	}

	if len(retrieved) != 1 {
		t.Fatalf("expected 1 tool after replacement, got %d", len(retrieved))
	}

	if retrieved[0].Name != "tool3" {
		t.Errorf("expected tool name 'tool3', got '%s'", retrieved[0].Name)
	}
}
