package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentSpecificAgentsMigration(t *testing.T) {
	// Create a test database with all migrations
	db, err := New(":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Run migrations to apply the new schema
	err = db.Migrate()
	require.NoError(t, err)

	// Test 1: Verify agent_environments table is dropped
	_, err = db.Conn().Exec("SELECT * FROM agent_environments LIMIT 1")
	assert.Error(t, err, "agent_environments table should not exist")

	// Test 2: Verify agent_tools table has simplified structure (tool_id instead of tool_name + environment_id)
	rows, err := db.Conn().Query("PRAGMA table_info(agent_tools)")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull int
		var defaultValue interface{}
		var pk int
		err = rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
		require.NoError(t, err)
		columns[name] = true
	}

	// Should have tool_id column
	assert.True(t, columns["tool_id"], "agent_tools should have tool_id column")
	// Should NOT have tool_name and environment_id columns
	assert.False(t, columns["tool_name"], "agent_tools should not have tool_name column")
	assert.False(t, columns["environment_id"], "agent_tools should not have environment_id column")

	// Test 3: Verify agents table still has environment_id (single environment per agent)
	rows, err = db.Conn().Query("PRAGMA table_info(agents)")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	agentColumns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull int
		var defaultValue interface{}
		var pk int
		err = rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
		require.NoError(t, err)
		agentColumns[name] = true
	}

	assert.True(t, agentColumns["environment_id"], "agents table should still have environment_id column")
}

func TestEnvironmentSpecificAgentDataIntegrity(t *testing.T) {
	// Create a test database with all migrations
	db, err := New(":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Run migrations
	err = db.Migrate()
	require.NoError(t, err)

	// Test data integrity by creating some test data

	// Create test user first (with required public_key field)
	_, err = db.Conn().Exec("INSERT INTO users (username, public_key) VALUES (?, ?)", "testuser", "test-public-key")
	require.NoError(t, err)

	// Create test environment
	_, err = db.Conn().Exec("INSERT INTO environments (name, description, created_by) VALUES (?, ?, ?)", "test-env", "Test Environment", 1)
	require.NoError(t, err)

	// Create test agent in the environment
	result, err := db.Conn().Exec("INSERT INTO agents (name, description, prompt, environment_id, created_by) VALUES (?, ?, ?, ?, ?)",
		"test-agent", "Test Agent", "You are a test agent", 1, 1)
	require.NoError(t, err)

	agentID, err := result.LastInsertId()
	require.NoError(t, err)

	// Create test MCP server in the same environment
	serverResult, err := db.Conn().Exec("INSERT INTO mcp_servers (name, command, environment_id) VALUES (?, ?, ?)",
		"test-server", "echo", 1)
	require.NoError(t, err)

	serverID, err := serverResult.LastInsertId()
	require.NoError(t, err)

	// Create test tool
	toolResult, err := db.Conn().Exec("INSERT INTO mcp_tools (mcp_server_id, name, description) VALUES (?, ?, ?)",
		serverID, "test-tool", "Test Tool")
	require.NoError(t, err)

	toolID, err := toolResult.LastInsertId()
	require.NoError(t, err)

	// Test: Assign tool to agent using the new simplified structure
	_, err = db.Conn().Exec("INSERT INTO agent_tools (agent_id, tool_id) VALUES (?, ?)", agentID, toolID)
	require.NoError(t, err)

	// Test: Query agent tools through the new join logic
	rows, err := db.Conn().Query(`
		SELECT t.name as tool_name, s.name as server_name, s.environment_id
		FROM agent_tools at
		JOIN mcp_tools t ON at.tool_id = t.id 
		JOIN mcp_servers s ON t.mcp_server_id = s.id
		JOIN agents a ON at.agent_id = a.id
		WHERE at.agent_id = ? AND s.environment_id = a.environment_id
	`, agentID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	toolCount := 0
	for rows.Next() {
		var toolName, serverName string
		var envID int
		err = rows.Scan(&toolName, &serverName, &envID)
		require.NoError(t, err)
		
		assert.Equal(t, "test-tool", toolName)
		assert.Equal(t, "test-server", serverName)
		assert.Equal(t, 1, envID)
		toolCount++
	}

	assert.Equal(t, 1, toolCount, "Should find exactly one tool for the agent")
}