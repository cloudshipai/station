package repositories

import (
	"database/sql"
	"path/filepath"
	"station/pkg/models"
	"testing"

	_ "modernc.org/sqlite"
)

// setupAgentTestDB creates a test database using production schema
func setupAgentTestDB(t *testing.T) *sql.DB {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Enable foreign key constraints (SQLite requires this explicitly)
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatalf("Failed to enable foreign key constraints: %v", err)
	}

	// Use the complete production schema from schema.sql
	// This ensures test schema always matches production schema
	schema := `
	-- Users table
	CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		public_key TEXT NOT NULL,
		is_admin BOOLEAN NOT NULL DEFAULT FALSE,
		api_key TEXT UNIQUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Environments table for isolated agent environments
	CREATE TABLE environments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		description TEXT,
		created_by INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- File-based MCP configurations
	CREATE TABLE file_mcp_configs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		environment_id INTEGER NOT NULL,
		config_name TEXT NOT NULL,
		template_path TEXT NOT NULL,
		variables_path TEXT,
		template_specific_vars_path TEXT,
		last_loaded_at TIMESTAMP,
		template_hash TEXT,
		variables_hash TEXT,
		template_vars_hash TEXT,
		metadata TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
		UNIQUE(environment_id, config_name)
	);

	-- MCP servers
	CREATE TABLE mcp_servers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		command TEXT NOT NULL,
		args TEXT,
		env TEXT,
		working_dir TEXT,
		timeout_seconds INTEGER DEFAULT 30,
		auto_restart BOOLEAN DEFAULT true,
		environment_id INTEGER NOT NULL,
		file_config_id INTEGER REFERENCES file_mcp_configs(id) ON DELETE CASCADE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
		UNIQUE(name, environment_id)
	);

	-- Tools discovered from MCP servers
	CREATE TABLE mcp_tools (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		mcp_server_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		input_schema TEXT,
		file_config_id INTEGER REFERENCES file_mcp_configs(id) ON DELETE SET NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (mcp_server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE,
		UNIQUE(name, mcp_server_id)
	);

	-- AI Agents (model_id FK removed for tests - not needed)
	CREATE TABLE agents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL,
		prompt TEXT NOT NULL,
		max_steps INTEGER NOT NULL DEFAULT 5,
		environment_id INTEGER NOT NULL,
		created_by INTEGER NOT NULL,
		model_id INTEGER,
		input_schema TEXT DEFAULT NULL,
		output_schema TEXT DEFAULT NULL,
		output_schema_preset TEXT DEFAULT NULL,
		app TEXT,
		app_subtype TEXT CHECK (app_subtype IS NULL OR app_subtype IN ('investigations', 'opportunities', 'projections', 'inventory', 'events')),
		cron_schedule TEXT DEFAULT NULL,
		is_scheduled BOOLEAN DEFAULT FALSE,
		last_scheduled_run DATETIME DEFAULT NULL,
		next_scheduled_run DATETIME DEFAULT NULL,
		schedule_enabled BOOLEAN DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (environment_id) REFERENCES environments (id),
		FOREIGN KEY (created_by) REFERENCES users (id)
	);

	-- Agent-Tool relationships (many-to-many)
	CREATE TABLE agent_tools (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id INTEGER NOT NULL,
		tool_id INTEGER NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
		FOREIGN KEY (tool_id) REFERENCES mcp_tools(id) ON DELETE CASCADE,
		UNIQUE(agent_id, tool_id)
	);`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	// Create console user for tests
	_, err = db.Exec("INSERT INTO users (username, public_key) VALUES (?, ?)", "console", "test-key")
	if err != nil {
		t.Fatalf("Failed to create console user: %v", err)
	}

	return db
}

// TestAgentTransactionRollback_CreateAgent tests that agent creation + tool assignment
// rolls back atomically when tool assignment fails
func TestAgentTransactionRollback_CreateAgent(t *testing.T) {
	db := setupAgentTestDB(t)
	defer func() { _ = db.Close() }()

	repos := New(&mockDB{conn: db})

	// Setup: Create environment, file config, MCP server, and 2 tools
	env, err := repos.Environments.Create("test-env", strPtr("Test env"), 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	fileConfigID, err := repos.FileMCPConfigs.Create(&FileConfigRecord{
		EnvironmentID: env.ID,
		ConfigName:    "test-config",
		TemplatePath:  "/test/template.json",
		TemplateHash:  "template-hash",
		VariablesHash: "var-hash",
	})
	if err != nil {
		t.Fatalf("Failed to create file config: %v", err)
	}

	serverID, err := repos.MCPServers.Create(&models.MCPServer{
		Name:          "test-server",
		FileConfigID:  &fileConfigID,
		Command:       "test-cmd",
		Args:          []string{},
		Env:           map[string]string{},
		EnvironmentID: env.ID,
	})
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	tool1ID, err := repos.MCPTools.Create(&models.MCPTool{
		Name:        "tool1",
		MCPServerID: serverID,
		Description: "Tool 1",
		Schema:      []byte("{}"),
	})
	if err != nil {
		t.Fatalf("Failed to create tool1: %v", err)
	}

	// Test: Try to create agent with tools, but make second tool assignment fail
	// by using an invalid tool ID
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Create agent within transaction
	agent, err := repos.Agents.CreateTx(
		tx, "test-agent", "Test agent", "Test prompt", 5, env.ID, 1,
		nil, nil, false, nil, nil, "", "",
	)
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to create agent: %v", err)
	}

	// First tool assignment should succeed
	_, err = repos.AgentTools.AddAgentToolTx(tx, agent.ID, tool1ID)
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to add tool1: %v", err)
	}

	// Second tool assignment with invalid ID should fail
	invalidToolID := int64(99999)
	_, err = repos.AgentTools.AddAgentToolTx(tx, agent.ID, invalidToolID)
	if err == nil {
		tx.Commit()
		t.Fatal("Expected error when adding invalid tool, got nil")
	}

	// Transaction should rollback (happens automatically when we don't commit)
	tx.Rollback()

	// Verify: Agent should NOT exist in database due to rollback
	_, err = repos.Agents.GetByID(agent.ID)
	if err == nil {
		t.Error("Expected agent to NOT exist after rollback, but it does")
	}
	if err != sql.ErrNoRows {
		t.Errorf("Expected sql.ErrNoRows, got: %v", err)
	}

	// Verify: No agent_tools entries should exist
	rows, err := db.Query("SELECT COUNT(*) FROM agent_tools WHERE agent_id = ?", agent.ID)
	if err != nil {
		t.Fatalf("Failed to query agent_tools: %v", err)
	}
	defer rows.Close()

	var count int
	if rows.Next() {
		rows.Scan(&count)
	}

	if count != 0 {
		t.Errorf("Expected 0 agent_tools entries after rollback, got %d", count)
	}
}

// TestAgentTransactionCommit_CreateAgent tests successful atomic agent creation
func TestAgentTransactionCommit_CreateAgent(t *testing.T) {
	db := setupAgentTestDB(t)
	defer func() { _ = db.Close() }()

	repos := New(&mockDB{conn: db})

	// Setup: Create environment, file config, MCP server, and 2 tools
	env, err := repos.Environments.Create("test-env", strPtr("Test env"), 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	fileConfigID, err := repos.FileMCPConfigs.Create(&FileConfigRecord{
		EnvironmentID: env.ID,
		ConfigName:    "test-config",
		TemplatePath:  "/test/template.json",
		TemplateHash:  "template-hash",
		VariablesHash: "var-hash",
	})
	if err != nil {
		t.Fatalf("Failed to create file config: %v", err)
	}

	serverID, err := repos.MCPServers.Create(&models.MCPServer{
		Name:          "test-server",
		FileConfigID:  &fileConfigID,
		Command:       "test-cmd",
		Args:          []string{},
		Env:           map[string]string{},
		EnvironmentID: env.ID,
	})
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	tool1ID, err := repos.MCPTools.Create(&models.MCPTool{
		Name:        "tool1",
		MCPServerID: serverID,
		Description: "Tool 1",
		Schema:      []byte("{}"),
	})
	if err != nil {
		t.Fatalf("Failed to create tool1: %v", err)
	}

	tool2ID, err := repos.MCPTools.Create(&models.MCPTool{
		Name:        "tool2",
		MCPServerID: serverID,
		Description: "Tool 2",
		Schema:      []byte("{}"),
	})
	if err != nil {
		t.Fatalf("Failed to create tool2: %v", err)
	}

	// Test: Create agent with both tools successfully
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	agent, err := repos.Agents.CreateTx(
		tx, "test-agent", "Test agent", "Test prompt", 5, env.ID, 1,
		nil, nil, false, nil, nil, "", "",
	)
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to create agent: %v", err)
	}

	_, err = repos.AgentTools.AddAgentToolTx(tx, agent.ID, tool1ID)
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to add tool1: %v", err)
	}

	_, err = repos.AgentTools.AddAgentToolTx(tx, agent.ID, tool2ID)
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to add tool2: %v", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify: Agent exists
	savedAgent, err := repos.Agents.GetByID(agent.ID)
	if err != nil {
		t.Fatalf("Expected agent to exist after commit: %v", err)
	}
	if savedAgent.Name != "test-agent" {
		t.Errorf("Expected agent name 'test-agent', got '%s'", savedAgent.Name)
	}

	// Verify: Both tools are assigned
	tools, err := repos.AgentTools.ListAgentTools(agent.ID)
	if err != nil {
		t.Fatalf("Failed to list agent tools: %v", err)
	}

	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}

	// Verify tool names
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.ToolName] = true
	}

	if !toolNames["tool1"] || !toolNames["tool2"] {
		t.Errorf("Expected tools 'tool1' and 'tool2', got: %v", toolNames)
	}
}

// TestAgentTransactionRollback_UpdateAgent tests that agent update + tool sync
// rolls back atomically when tool operation fails
func TestAgentTransactionRollback_UpdateAgent(t *testing.T) {
	db := setupAgentTestDB(t)
	defer func() { _ = db.Close() }()

	repos := New(&mockDB{conn: db})

	// Setup: Create environment, agent with tool1
	env, err := repos.Environments.Create("test-env", strPtr("Test env"), 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	fileConfigID, err := repos.FileMCPConfigs.Create(&FileConfigRecord{
		EnvironmentID: env.ID,
		ConfigName:    "test-config",
		TemplatePath:  "/test/template.json",
		TemplateHash:  "template-hash",
		VariablesHash: "var-hash",
	})
	if err != nil {
		t.Fatalf("Failed to create file config: %v", err)
	}

	serverID, err := repos.MCPServers.Create(&models.MCPServer{
		Name:          "test-server",
		FileConfigID:  &fileConfigID,
		Command:       "test-cmd",
		Args:          []string{},
		Env:           map[string]string{},
		EnvironmentID: env.ID,
	})
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	tool1ID, err := repos.MCPTools.Create(&models.MCPTool{
		Name:        "tool1",
		MCPServerID: serverID,
		Description: "Tool 1",
		Schema:      []byte("{}"),
	})
	if err != nil {
		t.Fatalf("Failed to create tool1: %v", err)
	}

	// Create agent with tool1
	agent, err := repos.Agents.Create(
		"test-agent", "Original description", "Original prompt", 5, env.ID, 1,
		nil, nil, false, nil, nil, "", "",
	)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	_, err = repos.AgentTools.AddAgentTool(agent.ID, tool1ID)
	if err != nil {
		t.Fatalf("Failed to add tool1 to agent: %v", err)
	}

	// Test: Try to update agent and assign invalid tool
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Update agent metadata
	err = repos.Agents.UpdateTx(
		tx, agent.ID, "test-agent", "Updated description", "Updated prompt", 10,
		nil, nil, false, nil, nil, "", "",
	)
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to update agent: %v", err)
	}

	// Try to add invalid tool (should fail)
	invalidToolID := int64(99999)
	_, err = repos.AgentTools.AddAgentToolTx(tx, agent.ID, invalidToolID)
	if err == nil {
		tx.Commit()
		t.Fatal("Expected error when adding invalid tool, got nil")
	}

	// Transaction should rollback
	tx.Rollback()

	// Verify: Agent metadata should NOT be updated
	savedAgent, err := repos.Agents.GetByID(agent.ID)
	if err != nil {
		t.Fatalf("Failed to get agent: %v", err)
	}

	if savedAgent.Description != "Original description" {
		t.Errorf("Expected description 'Original description', got '%s' (update should have rolled back)", savedAgent.Description)
	}

	if savedAgent.Prompt != "Original prompt" {
		t.Errorf("Expected prompt 'Original prompt', got '%s' (update should have rolled back)", savedAgent.Prompt)
	}

	if savedAgent.MaxSteps != 5 {
		t.Errorf("Expected max_steps 5, got %d (update should have rolled back)", savedAgent.MaxSteps)
	}

	// Verify: Original tool assignment still intact
	tools, err := repos.AgentTools.ListAgentTools(agent.ID)
	if err != nil {
		t.Fatalf("Failed to list agent tools: %v", err)
	}

	if len(tools) != 1 {
		t.Errorf("Expected 1 tool (original), got %d", len(tools))
	}

	if len(tools) > 0 && tools[0].ToolName != "tool1" {
		t.Errorf("Expected tool 'tool1', got '%s'", tools[0].ToolName)
	}
}

// Helper function for string pointers
func strPtr(s string) *string {
	return &s
}
