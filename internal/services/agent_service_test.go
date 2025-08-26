package services

import (
	"context"
	"testing"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// setupTestDB creates an in-memory SQLite database for testing  
func setupTestDB(t *testing.T) (db.Database, *repositories.Repositories) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Run migrations
	if err := database.Migrate(); err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	repos := repositories.New(database)
	return database, repos
}

// setupTestEnvironment creates a test environment
func setupTestEnvironment(t *testing.T, repos *repositories.Repositories) *models.Environment {
	desc := "Test Environment"
	env, err := repos.Environments.Create("test-env", &desc, 1)
	if err != nil {
		t.Fatalf("Failed to create test environment: %v", err)
	}
	return env
}

func TestAgentService_CreateAgent(t *testing.T) {
	database, repos := setupTestDB(t)
	defer database.Close()

	env := setupTestEnvironment(t, repos)
	service := NewAgentService(repos)

	tests := []struct {
		name    string
		config  *AgentConfig
		wantErr bool
	}{
		{
			name: "valid agent creation",
			config: &AgentConfig{
				EnvironmentID: env.ID,
				Name:          "Test Agent",
				Description:   "A test agent",
				Prompt:        "You are a helpful test agent.",
				MaxSteps:      25,
				CreatedBy:     1,
				AssignedTools: []string{"tool1", "tool2"},
			},
			wantErr: false,
		},
		{
			name: "empty name should fail",
			config: &AgentConfig{
				EnvironmentID: env.ID,
				Name:          "",
				Description:   "A test agent",
				Prompt:        "You are a helpful test agent.",
				MaxSteps:      25,
				CreatedBy:     1,
			},
			wantErr: true,
		},
		{
			name: "invalid environment should fail",
			config: &AgentConfig{
				EnvironmentID: 999999, // Non-existent environment
				Name:          "Test Agent",
				Description:   "A test agent",
				Prompt:        "You are a helpful test agent.",
				MaxSteps:      25,
				CreatedBy:     1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			agent, err := service.CreateAgent(ctx, tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateAgent() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("CreateAgent() unexpected error: %v", err)
			}

			if agent == nil {
				t.Fatal("CreateAgent() returned nil agent")
			}

			// Verify agent properties
			if agent.Name != tt.config.Name {
				t.Errorf("Expected name %q, got %q", tt.config.Name, agent.Name)
			}
			if agent.Description != tt.config.Description {
				t.Errorf("Expected description %q, got %q", tt.config.Description, agent.Description)
			}
			if agent.EnvironmentID != tt.config.EnvironmentID {
				t.Errorf("Expected environment_id %d, got %d", tt.config.EnvironmentID, agent.EnvironmentID)
			}
		})
	}
}

func TestAgentService_GetAgent(t *testing.T) {
	database, repos := setupTestDB(t)
	defer database.Close()

	env := setupTestEnvironment(t, repos)
	service := NewAgentService(repos)
	ctx := context.Background()

	// Create test agent
	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Test Agent",
		Description:   "A test agent",
		Prompt:        "You are a helpful test agent.",
		MaxSteps:      25,
		CreatedBy:     1,
	}
	
	createdAgent, err := service.CreateAgent(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	// Test getting the agent
	retrievedAgent, err := service.GetAgent(ctx, createdAgent.ID)
	if err != nil {
		t.Fatalf("GetAgent() unexpected error: %v", err)
	}

	if retrievedAgent.ID != createdAgent.ID {
		t.Errorf("Expected ID %d, got %d", createdAgent.ID, retrievedAgent.ID)
	}
	if retrievedAgent.Name != createdAgent.Name {
		t.Errorf("Expected name %q, got %q", createdAgent.Name, retrievedAgent.Name)
	}

	// Test non-existent agent
	_, err = service.GetAgent(ctx, 999999)
	if err == nil {
		t.Error("Expected error for non-existent agent, got nil")
	}
}

func TestAgentService_ListAgentsByEnvironment(t *testing.T) {
	database, repos := setupTestDB(t)
	defer database.Close()

	env1 := setupTestEnvironment(t, repos)
	desc2 := "Test Environment 2"
	env2, err := repos.Environments.Create("test-env-2", &desc2, 1)
	if err != nil {
		t.Fatalf("Failed to create second test environment: %v", err)
	}

	service := NewAgentService(repos)
	ctx := context.Background()

	// Create agents in different environments
	agent1Config := &AgentConfig{
		EnvironmentID: env1.ID,
		Name:          "Agent 1",
		Description:   "First test agent",
		Prompt:        "You are agent 1.",
		MaxSteps:      25,
		CreatedBy:     1,
	}
	agent2Config := &AgentConfig{
		EnvironmentID: env1.ID,
		Name:          "Agent 2", 
		Description:   "Second test agent",
		Prompt:        "You are agent 2.",
		MaxSteps:      25,
		CreatedBy:     1,
	}
	agent3Config := &AgentConfig{
		EnvironmentID: env2.ID,
		Name:          "Agent 3",
		Description:   "Third test agent",
		Prompt:        "You are agent 3.",
		MaxSteps:      25,
		CreatedBy:     1,
	}

	_, err = service.CreateAgent(ctx, agent1Config)
	if err != nil {
		t.Fatalf("Failed to create agent 1: %v", err)
	}
	_, err = service.CreateAgent(ctx, agent2Config)
	if err != nil {
		t.Fatalf("Failed to create agent 2: %v", err)
	}
	_, err = service.CreateAgent(ctx, agent3Config)
	if err != nil {
		t.Fatalf("Failed to create agent 3: %v", err)
	}

	// Test listing agents by environment
	env1Agents, err := service.ListAgentsByEnvironment(ctx, env1.ID)
	if err != nil {
		t.Fatalf("ListAgentsByEnvironment() unexpected error: %v", err)
	}
	if len(env1Agents) != 2 {
		t.Errorf("Expected 2 agents in env1, got %d", len(env1Agents))
	}

	env2Agents, err := service.ListAgentsByEnvironment(ctx, env2.ID)
	if err != nil {
		t.Fatalf("ListAgentsByEnvironment() unexpected error: %v", err)
	}
	if len(env2Agents) != 1 {
		t.Errorf("Expected 1 agent in env2, got %d", len(env2Agents))
	}
}

func TestAgentService_UpdateAgent(t *testing.T) {
	database, repos := setupTestDB(t)
	defer database.Close()

	env := setupTestEnvironment(t, repos)
	service := NewAgentService(repos)
	ctx := context.Background()

	// Create test agent
	createConfig := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Original Name",
		Description:   "Original Description",
		Prompt:        "Original prompt.",
		MaxSteps:      25,
		CreatedBy:     1,
	}
	
	createdAgent, err := service.CreateAgent(ctx, createConfig)
	if err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	// Update the agent
	updateConfig := &AgentConfig{
		Name:        "Updated Name",
		Description: "Updated Description", 
		Prompt:      "Updated prompt.",
		MaxSteps:    50,
	}
	
	updatedAgent, err := service.UpdateAgent(ctx, createdAgent.ID, updateConfig)
	if err != nil {
		t.Fatalf("UpdateAgent() unexpected error: %v", err)
	}

	// Verify updates
	if updatedAgent.Name != updateConfig.Name {
		t.Errorf("Expected name %q, got %q", updateConfig.Name, updatedAgent.Name)
	}
	if updatedAgent.Description != updateConfig.Description {
		t.Errorf("Expected description %q, got %q", updateConfig.Description, updatedAgent.Description)
	}
	if updatedAgent.MaxSteps != updateConfig.MaxSteps {
		t.Errorf("Expected max_steps %d, got %d", updateConfig.MaxSteps, updatedAgent.MaxSteps)
	}
}

func TestAgentService_DeleteAgent(t *testing.T) {
	database, repos := setupTestDB(t)
	defer database.Close()

	env := setupTestEnvironment(t, repos)
	service := NewAgentService(repos)
	ctx := context.Background()

	// Create test agent
	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Agent To Delete",
		Description:   "This agent will be deleted",
		Prompt:        "You are about to be deleted.",
		MaxSteps:      25,
		CreatedBy:     1,
	}
	
	createdAgent, err := service.CreateAgent(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	// Verify agent exists
	_, err = service.GetAgent(ctx, createdAgent.ID)
	if err != nil {
		t.Fatalf("Agent should exist before deletion: %v", err)
	}

	// Delete the agent
	err = service.DeleteAgent(ctx, createdAgent.ID)
	if err != nil {
		t.Fatalf("DeleteAgent() unexpected error: %v", err)
	}

	// Verify agent no longer exists
	_, err = service.GetAgent(ctx, createdAgent.ID)
	if err == nil {
		t.Error("Agent should not exist after deletion")
	}

	// Test deleting non-existent agent
	err = service.DeleteAgent(ctx, 999999)
	if err == nil {
		t.Error("Expected error when deleting non-existent agent")
	}
}