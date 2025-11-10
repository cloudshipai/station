package toolcache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// Cache handles tool caching for standalone fakers
type Cache interface {
	// GetTools retrieves cached tools for a faker ID
	GetTools(ctx context.Context, fakerID string) ([]mcp.Tool, error)

	// SetTools caches tools for a faker ID (replaces existing)
	SetTools(ctx context.Context, fakerID string, tools []mcp.Tool) error

	// ClearTools removes all cached tools for a faker ID
	ClearTools(ctx context.Context, fakerID string) error

	// HasTools checks if tools are cached for a faker ID
	HasTools(ctx context.Context, fakerID string) (bool, error)
}

// cache implements the Cache interface
type cache struct {
	db *sql.DB
}

// NewCache creates a new tool cache
func NewCache(db *sql.DB) Cache {
	return &cache{db: db}
}

// GetTools retrieves cached tools for a faker ID (config hash)
func (c *cache) GetTools(ctx context.Context, fakerID string) ([]mcp.Tool, error) {
	// First try to find by config_hash (new deterministic key)
	query := `
		SELECT tool_name, tool_schema
		FROM faker_tool_cache
		WHERE config_hash = ?
		ORDER BY tool_name
	`

	rows, err := c.db.QueryContext(ctx, query, fakerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query cached tools: %w", err)
	}
	defer rows.Close()

	var tools []mcp.Tool
	for rows.Next() {
		var toolName, toolSchemaJSON string
		if err := rows.Scan(&toolName, &toolSchemaJSON); err != nil {
			return nil, fmt.Errorf("failed to scan tool row: %w", err)
		}

		var tool mcp.Tool
		if err := json.Unmarshal([]byte(toolSchemaJSON), &tool); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool schema for %s: %w", toolName, err)
		}

		tools = append(tools, tool)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tool rows: %w", err)
	}

	// If no tools found by config_hash, try legacy faker_id lookup
	if len(tools) == 0 {
		rows, err := c.db.QueryContext(ctx, `
			SELECT tool_name, tool_schema
			FROM faker_tool_cache
			WHERE faker_id = ? AND (config_hash IS NULL OR config_hash = '')
			ORDER BY tool_name
		`, fakerID)
		if err != nil {
			return nil, fmt.Errorf("failed to query legacy cached tools: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var toolName, toolSchemaJSON string
			if err := rows.Scan(&toolName, &toolSchemaJSON); err != nil {
				return nil, fmt.Errorf("failed to scan legacy tool row: %w", err)
			}

			var tool mcp.Tool
			if err := json.Unmarshal([]byte(toolSchemaJSON), &tool); err != nil {
				return nil, fmt.Errorf("failed to unmarshal legacy tool schema for %s: %w", toolName, err)
			}

			tools = append(tools, tool)
		}

		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating legacy tool rows: %w", err)
		}
	}

	return tools, nil
}

// SetTools caches tools for a faker ID (config hash) - replaces existing
func (c *cache) SetTools(ctx context.Context, fakerID string, tools []mcp.Tool) error {
	// Start transaction
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Clear existing tools for this config hash
	if _, err := tx.ExecContext(ctx, "DELETE FROM faker_tool_cache WHERE config_hash = ?", fakerID); err != nil {
		return fmt.Errorf("failed to clear existing tools: %w", err)
	}

	// Insert new tools with config_hash
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO faker_tool_cache (faker_id, config_hash, tool_name, tool_schema, created_at, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	for _, tool := range tools {
		toolSchemaJSON, err := json.Marshal(tool)
		if err != nil {
			return fmt.Errorf("failed to marshal tool schema for %s: %w", tool.Name, err)
		}

		// Store both faker_id (original user name) and config_hash (deterministic key)
		// faker_id is kept for human readability in database
		// config_hash is used as the actual lookup key
		if _, err := stmt.ExecContext(ctx, fakerID, fakerID, tool.Name, string(toolSchemaJSON)); err != nil {
			return fmt.Errorf("failed to insert tool %s: %w", tool.Name, err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ClearTools removes all cached tools for a faker ID (config hash)
func (c *cache) ClearTools(ctx context.Context, fakerID string) error {
	// Try clearing by config_hash first
	result, err := c.db.ExecContext(ctx, "DELETE FROM faker_tool_cache WHERE config_hash = ?", fakerID)
	if err != nil {
		return fmt.Errorf("failed to clear tools: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Try legacy faker_id lookup if no config_hash match
		result, err = c.db.ExecContext(ctx, "DELETE FROM faker_tool_cache WHERE faker_id = ?", fakerID)
		if err != nil {
			return fmt.Errorf("failed to clear legacy tools: %w", err)
		}
		rows, _ = result.RowsAffected()
		if rows == 0 {
			return fmt.Errorf("no cached tools found for faker ID: %s", fakerID)
		}
	}

	return nil
}

// HasTools checks if tools are cached for a faker ID (config hash)
func (c *cache) HasTools(ctx context.Context, fakerID string) (bool, error) {
	// Try config_hash first (new deterministic key)
	var count int
	err := c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM faker_tool_cache WHERE config_hash = ?", fakerID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check for cached tools: %w", err)
	}

	if count > 0 {
		return true, nil
	}

	// Fallback to legacy faker_id lookup
	err = c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM faker_tool_cache WHERE faker_id = ?", fakerID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check for legacy cached tools: %w", err)
	}
	return count > 0, nil
}
