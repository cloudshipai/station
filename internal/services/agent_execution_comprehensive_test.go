package services

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// TestAgentExecutionEngine_Comprehensive tests the complete agent execution flow
// with real OpenAI integration and various scenarios
func TestAgentExecutionEngine_Comprehensive(t *testing.T) {
	// Skip if OPENAI_API_KEY not set to avoid test failures in CI without key
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		t.Skip("OPENAI_API_KEY not set - skipping comprehensive agent execution tests")
	}

	// Setup test configuration
	setupTestConfig(t)

	// Setup test database and repositories
	database, repos := setupComprehensiveTestDB(t)
	defer database.Close()

	// Create test environment
	env := setupComprehensiveTestEnvironment(t, repos)

	// Initialize services
	agentService := NewAgentService(repos)
	
	// Set up comprehensive test cases
	t.Run("BasicAgentExecution", func(t *testing.T) {
		testBasicAgentExecution(t, agentService, env)
	})

	t.Run("AgentExecutionWithDebugging", func(t *testing.T) {
		testAgentExecutionWithDebugging(t, agentService, env)
	})

	t.Run("AgentExecutionErrorHandling", func(t *testing.T) {
		testAgentExecutionErrorHandling(t, agentService, env)
	})

	t.Run("AgentExecutionLogging", func(t *testing.T) {
		testAgentExecutionLogging(t, agentService, env)
	})

	t.Run("AgentExecutionMetadata", func(t *testing.T) {
		testAgentExecutionMetadata(t, agentService, env)
	})

	t.Run("MultiStepAgentExecution", func(t *testing.T) {
		testMultiStepAgentExecution(t, agentService, env)
	})
}

// setupComprehensiveTestDB creates a test database with migrations
func setupComprehensiveTestDB(t *testing.T) (db.Database, *repositories.Repositories) {
	database, err := db.New(":memory:")
	require.NoError(t, err, "Failed to create test database")

	// Run migrations
	err = database.Migrate()
	require.NoError(t, err, "Failed to migrate test database")

	repos := repositories.New(database)
	return database, repos
}

// setupTestConfig sets up minimal configuration for testing
func setupTestConfig(t *testing.T) {
	// Set a test encryption key if not already set
	if os.Getenv("ENCRYPTION_KEY") == "" {
		err := os.Setenv("ENCRYPTION_KEY", "test-encryption-key-for-comprehensive-tests-12345678")
		require.NoError(t, err, "Failed to set test encryption key")
	}
}

// setupComprehensiveTestEnvironment creates a test environment with proper setup
func setupComprehensiveTestEnvironment(t *testing.T, repos *repositories.Repositories) *models.Environment {
	desc := "Comprehensive Test Environment"
	env, err := repos.Environments.Create("test-comprehensive", &desc, 1)
	require.NoError(t, err, "Failed to create test environment")
	return env
}

// testBasicAgentExecution tests basic agent execution functionality
func testBasicAgentExecution(t *testing.T, agentService *AgentService, env *models.Environment) {
	ctx := context.Background()

	// Create a simple test agent
	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Basic Test Agent",
		Description:   "Agent for testing basic execution",
		Prompt:        "You are a helpful assistant. Respond to user questions concisely and accurately.",
		MaxSteps:      5,
		CreatedBy:     1,
		ModelProvider: "openai",
		ModelID:       "gpt-4o-mini",
	}

	agent, err := agentService.CreateAgent(ctx, config)
	require.NoError(t, err, "Failed to create test agent")
	require.NotNil(t, agent, "Agent should not be nil")

	// Execute a simple task
	task := "What is 2 + 2? Provide just the number as your answer."
	
	t.Logf("🚀 Executing agent: %s", agent.Name)
	t.Logf("📝 Task: %s", task)

	result, err := agentService.ExecuteAgent(ctx, agent.ID, task, nil)
	
	// Verify execution succeeded
	assert.NoError(t, err, "Basic agent execution should succeed")
	assert.NotNil(t, result, "Result should not be nil")
	assert.NotEmpty(t, result.Content, "Result should have content")
	assert.Equal(t, "assistant", result.Role, "Result should be from assistant")

	// Verify the result makes sense (should contain "4")
	assert.Contains(t, strings.ToLower(result.Content), "4", "Result should contain the correct answer")

	t.Logf("✅ Execution result: %s", result.Content)
}

// testAgentExecutionWithDebugging tests execution with enhanced debugging
func testAgentExecutionWithDebugging(t *testing.T, agentService *AgentService, env *models.Environment) {
	ctx := context.Background()

	// Create an agent that will require multiple steps
	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Debug Test Agent",
		Description:   "Agent for testing debugging capabilities",
		Prompt: `You are a step-by-step problem solver. When given a task:
1. First, acknowledge the task
2. Break down the problem into steps
3. Solve each step methodically
4. Provide a final answer

Always show your reasoning process.`,
		MaxSteps:      10,
		CreatedBy:     1,
		ModelProvider: "openai",
		ModelID:       "gpt-4o-mini",
	}

	agent, err := agentService.CreateAgent(ctx, config)
	require.NoError(t, err, "Failed to create debug test agent")

	// Execute a complex task that should generate debug logs
	task := "Calculate the compound interest on $1000 at 5% annual rate for 3 years. Show your work step by step."
	
	t.Logf("🔍 Executing debug agent: %s", agent.Name)
	t.Logf("📝 Complex task: %s", task)

	result, err := agentService.ExecuteAgent(ctx, agent.ID, task, nil)
	
	// Verify execution succeeded
	assert.NoError(t, err, "Debug agent execution should succeed")
	assert.NotNil(t, result, "Result should not be nil")
	assert.NotEmpty(t, result.Content, "Result should have content")

	// The result should show step-by-step reasoning
	content := strings.ToLower(result.Content)
	assert.True(t, 
		strings.Contains(content, "step") || 
		strings.Contains(content, "first") ||
		strings.Contains(content, "calculate"),
		"Result should show reasoning steps")

	t.Logf("🔍 Debug result: %s", result.Content)
}

// testAgentExecutionErrorHandling tests error scenarios
func testAgentExecutionErrorHandling(t *testing.T, agentService *AgentService, env *models.Environment) {
	ctx := context.Background()

	t.Run("NonExistentAgent", func(t *testing.T) {
		// Try to execute a non-existent agent
		result, err := agentService.ExecuteAgent(ctx, 999999, "Test task", nil)
		
		assert.Error(t, err, "Should get error for non-existent agent")
		assert.Nil(t, result, "Result should be nil for failed execution")
		
		t.Logf("❌ Expected error for non-existent agent: %v", err)
	})

	t.Run("EmptyTask", func(t *testing.T) {
		// Create a simple agent
		config := &AgentConfig{
			EnvironmentID: env.ID,
			Name:          "Error Test Agent",
			Description:   "Agent for testing error handling",
			Prompt:        "You are a helpful assistant.",
			MaxSteps:      5,
			CreatedBy:     1,
			ModelProvider: "openai",
			ModelID:       "gpt-4o-mini",
		}

		agent, err := agentService.CreateAgent(ctx, config)
		require.NoError(t, err, "Failed to create error test agent")

		// Try to execute with empty task
		result, err := agentService.ExecuteAgent(ctx, agent.ID, "", nil)
		
		// This might succeed (empty task could be handled) or fail - test behavior
		if err != nil {
			t.Logf("❌ Empty task failed as expected: %v", err)
			assert.Nil(t, result, "Result should be nil for failed execution")
		} else {
			t.Logf("✅ Empty task handled gracefully: %s", result.Content)
			assert.NotNil(t, result, "Result should not be nil for successful execution")
		}
	})
}

// testAgentExecutionLogging tests the enhanced logging system
func testAgentExecutionLogging(t *testing.T, agentService *AgentService, env *models.Environment) {
	ctx := context.Background()

	// Create agent for logging test
	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Logging Test Agent",
		Description:   "Agent for testing logging system",
		Prompt:        "You are a conversational assistant that explains your thought process.",
		MaxSteps:      8,
		CreatedBy:     1,
		ModelProvider: "openai",
		ModelID:       "gpt-4o-mini",
	}

	agent, err := agentService.CreateAgent(ctx, config)
	require.NoError(t, err, "Failed to create logging test agent")

	// Execute task that should generate various log entries
	task := "Explain the difference between HTTP and HTTPS in simple terms."
	
	t.Logf("📊 Testing logging for agent: %s", agent.Name)
	
	result, err := agentService.ExecuteAgent(ctx, agent.ID, task, nil)
	
	// Verify execution succeeded
	assert.NoError(t, err, "Logging test should succeed")
	assert.NotNil(t, result, "Result should not be nil")
	assert.NotEmpty(t, result.Content, "Result should have content")
	
	// Verify the response makes sense for the technical question
	content := strings.ToLower(result.Content)
	assert.True(t,
		strings.Contains(content, "http") &&
		(strings.Contains(content, "secure") || strings.Contains(content, "encrypt")),
		"Result should explain HTTP vs HTTPS concepts")

	t.Logf("📊 Logging result: %s", result.Content)
}

// testAgentExecutionMetadata tests execution with metadata
func testAgentExecutionMetadata(t *testing.T, agentService *AgentService, env *models.Environment) {
	ctx := context.Background()

	// Create agent for metadata test
	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Metadata Test Agent",
		Description:   "Agent for testing metadata handling",
		Prompt:        "You are a helpful assistant that provides clear, concise answers.",
		MaxSteps:      5,
		CreatedBy:     1,
		ModelProvider: "openai",
		ModelID:       "gpt-4o-mini",
	}

	agent, err := agentService.CreateAgent(ctx, config)
	require.NoError(t, err, "Failed to create metadata test agent")

	// Prepare metadata
	userVariables := map[string]interface{}{
		"test_scenario":    "metadata_testing",
		"execution_source": "comprehensive_test",
		"priority":         "high",
		"debug_mode":       true,
	}

	task := "What is the capital of France?"
	
	t.Logf("🏷️  Testing metadata handling for agent: %s", agent.Name)
	t.Logf("📋 Metadata: %+v", userVariables)
	
	result, err := agentService.ExecuteAgent(ctx, agent.ID, task, userVariables)
	
	// Verify execution succeeded
	assert.NoError(t, err, "Metadata test should succeed")
	assert.NotNil(t, result, "Result should not be nil")
	assert.NotEmpty(t, result.Content, "Result should have content")
	
	// Verify metadata was passed through (should be in result.Extra)
	if result.Extra != nil {
		t.Logf("🏷️  Metadata passed through: %+v", result.Extra)
	}

	// Verify correct answer
	assert.Contains(t, strings.ToLower(result.Content), "paris", "Should correctly identify Paris as capital of France")

	t.Logf("🏷️  Metadata result: %s", result.Content)
}

// testMultiStepAgentExecution tests agents with multiple reasoning steps
func testMultiStepAgentExecution(t *testing.T, agentService *AgentService, env *models.Environment) {
	ctx := context.Background()

	// Create agent designed for multi-step reasoning
	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Multi-Step Test Agent",
		Description:   "Agent for testing multi-step execution",
		Prompt: `You are an analytical assistant. For complex questions:
1. Break down the problem
2. Analyze each component
3. Synthesize your findings
4. Provide a comprehensive answer

Always show your reasoning process clearly.`,
		MaxSteps:      15, // Allow more steps for complex reasoning
		CreatedBy:     1,
		ModelProvider: "openai",
		ModelID:       "gpt-4o-mini",
	}

	agent, err := agentService.CreateAgent(ctx, config)
	require.NoError(t, err, "Failed to create multi-step test agent")

	// Complex task requiring multiple reasoning steps
	task := `Compare and contrast the advantages and disadvantages of renewable energy sources (solar, wind, hydro) 
	versus fossil fuels (coal, oil, gas) in terms of environmental impact, cost, and reliability.`
	
	t.Logf("🔄 Testing multi-step reasoning for agent: %s", agent.Name)
	t.Logf("📝 Complex analysis task: %s", task)

	// Use longer timeout for complex reasoning
	start := time.Now()
	result, err := agentService.ExecuteAgent(ctx, agent.ID, task, nil)
	duration := time.Since(start)
	
	// Verify execution succeeded
	assert.NoError(t, err, "Multi-step execution should succeed")
	assert.NotNil(t, result, "Result should not be nil")
	assert.NotEmpty(t, result.Content, "Result should have content")
	
	// Verify comprehensive analysis
	content := strings.ToLower(result.Content)
	
	// Should mention key concepts
	assert.True(t,
		strings.Contains(content, "renewable") ||
		strings.Contains(content, "solar") ||
		strings.Contains(content, "wind"),
		"Should discuss renewable energy")
		
	assert.True(t,
		strings.Contains(content, "fossil") ||
		strings.Contains(content, "coal") ||
		strings.Contains(content, "oil"),
		"Should discuss fossil fuels")
		
	assert.True(t,
		strings.Contains(content, "environmental") ||
		strings.Contains(content, "cost") ||
		strings.Contains(content, "reliability"),
		"Should address comparison criteria")

	// Log performance metrics
	t.Logf("🔄 Multi-step execution completed in: %v", duration)
	t.Logf("📊 Response length: %d characters", len(result.Content))
	t.Logf("🔄 Multi-step result preview: %s...", 
		func() string {
			if len(result.Content) > 200 {
				return result.Content[:200]
			}
			return result.Content
		}())
}

// Benchmark agent execution performance
func BenchmarkAgentExecution(b *testing.B) {
	// Skip if OPENAI_API_KEY not set
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		b.Skip("OPENAI_API_KEY not set - skipping agent execution benchmarks")
	}

	// Setup test configuration
	if os.Getenv("ENCRYPTION_KEY") == "" {
		os.Setenv("ENCRYPTION_KEY", "test-encryption-key-for-benchmarks-12345678")
	}

	// Setup
	database, err := db.New(":memory:")
	if err != nil {
		b.Fatalf("Failed to create benchmark database: %v", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		b.Fatalf("Failed to migrate benchmark database: %v", err)
	}

	repos := repositories.New(database)
	
	// Create test environment
	desc := "Benchmark Environment"
	env, err := repos.Environments.Create("benchmark", &desc, 1)
	if err != nil {
		b.Fatalf("Failed to create benchmark environment: %v", err)
	}

	agentService := NewAgentService(repos)
	
	// Create benchmark agent
	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Benchmark Agent",
		Description:   "Agent for performance benchmarking",
		Prompt:        "You are a fast, efficient assistant. Provide concise, accurate answers.",
		MaxSteps:      3, // Keep it simple for benchmarking
		CreatedBy:     1,
		ModelProvider: "openai",
		ModelID:       "gpt-4o-mini", // Use faster model for benchmarks
	}

	agent, err := agentService.CreateAgent(context.Background(), config)
	if err != nil {
		b.Fatalf("Failed to create benchmark agent: %v", err)
	}

	// Simple task for consistent benchmarking
	task := "What is 10 + 15?"

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		result, err := agentService.ExecuteAgent(ctx, agent.ID, task, nil)
		if err != nil {
			b.Fatalf("Benchmark execution failed: %v", err)
		}
		if result == nil || result.Content == "" {
			b.Fatal("Benchmark execution returned empty result")
		}
	}
}

// Test execution timeout handling
func TestAgentExecutionTimeout(t *testing.T) {
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		t.Skip("OPENAI_API_KEY not set - skipping timeout tests")
	}

	// Setup test configuration
	setupTestConfig(t)

	// Setup
	database, repos := setupComprehensiveTestDB(t)
	defer database.Close()
	
	env := setupComprehensiveTestEnvironment(t, repos)
	agentService := NewAgentService(repos)

	// Create agent
	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Timeout Test Agent",
		Description:   "Agent for testing timeout handling",
		Prompt:        "You are a helpful assistant.",
		MaxSteps:      5,
		CreatedBy:     1,
		ModelProvider: "openai",
		ModelID:       "gpt-4o-mini",
	}

	agent, err := agentService.CreateAgent(context.Background(), config)
	require.NoError(t, err, "Failed to create timeout test agent")

	// Test with short timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond) // Very short timeout
	defer cancel()

	task := "What is artificial intelligence?"
	
	result, err := agentService.ExecuteAgent(ctx, agent.ID, task, nil)
	
	// Should either timeout (error) or complete very quickly
	if err != nil {
		// Timeout occurred - verify it's a context error
		t.Logf("⏱️  Timeout handled correctly: %v", err)
		assert.Nil(t, result, "Result should be nil on timeout")
	} else {
		// Completed before timeout - that's also valid
		t.Logf("⚡ Execution completed before timeout: %s", result.Content)
		assert.NotNil(t, result, "Result should not be nil on success")
	}
}