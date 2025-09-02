package services

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"station/pkg/models"
)

// TestAgentExecutionScenarios tests specific execution scenarios and edge cases
func TestAgentExecutionScenarios(t *testing.T) {
	// Skip if OPENAI_API_KEY not set
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		t.Skip("OPENAI_API_KEY not set - skipping scenario tests")
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

	t.Run("ConversationalAgent", func(t *testing.T) {
		testConversationalAgent(t, agentService, env)
	})

	t.Run("TechnicalAgent", func(t *testing.T) {
		testTechnicalAgent(t, agentService, env)
	})

	t.Run("AnalyticalAgent", func(t *testing.T) {
		testAnalyticalAgent(t, agentService, env)
	})

	t.Run("CreativeAgent", func(t *testing.T) {
		testCreativeAgent(t, agentService, env)
	})

	t.Run("ConstrainedAgent", func(t *testing.T) {
		testConstrainedAgent(t, agentService, env)
	})

	t.Run("VariableSubstitution", func(t *testing.T) {
		testVariableSubstitution(t, agentService, env)
	})
}

// testConversationalAgent tests natural conversation scenarios
func testConversationalAgent(t *testing.T, agentService *AgentService, env *models.Environment) {
	ctx := context.Background()

	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Conversational Agent",
		Description:   "Agent specialized in natural conversation",
		Prompt: `You are a friendly, conversational AI assistant. You:
- Engage in natural, flowing conversation
- Ask follow-up questions when appropriate  
- Show empathy and understanding
- Maintain a warm, helpful tone
- Adapt your communication style to the user`,
		MaxSteps:      8,
		CreatedBy:     1,
		ModelProvider: "openai",
		ModelID:       "gpt-4o-mini",
	}

	agent, err := agentService.CreateAgent(ctx, config)
	require.NoError(t, err, "Failed to create conversational agent")

	// Test conversational scenarios
	scenarios := []struct {
		name     string
		task     string
		expectIn string
	}{
		{
			name:     "GreetingResponse",
			task:     "Hi there! How are you doing today?",
			expectIn: "hello|hi|good|fine|great|thanks",
		},
		{
			name:     "PersonalQuestion",
			task:     "What's your favorite hobby?",
			expectIn: "i|assistant|ai|help|enjoy|like",
		},
		{
			name:     "EmotionalSupport",
			task:     "I'm feeling a bit stressed about work. Any advice?",
			expectIn: "stress|understand|help|suggest|take|break|relax",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Logf("💬 Testing conversational scenario: %s", scenario.name)
			t.Logf("📝 Task: %s", scenario.task)

			result, err := agentService.ExecuteAgent(ctx, agent.ID, scenario.task, nil)

			assert.NoError(t, err, "Conversational execution should succeed")
			assert.NotNil(t, result, "Result should not be nil")
			assert.NotEmpty(t, result.Content, "Result should have content")

			content := strings.ToLower(result.Content)
			hasExpected := false
			expectedWords := strings.Split(scenario.expectIn, "|")
			for _, word := range expectedWords {
				if strings.Contains(content, word) {
					hasExpected = true
					break
				}
			}

			assert.True(t, hasExpected, "Response should contain expected conversational elements")
			t.Logf("💬 Response: %s", result.Content)
		})
	}
}

// testTechnicalAgent tests technical problem-solving scenarios  
func testTechnicalAgent(t *testing.T, agentService *AgentService, env *models.Environment) {
	ctx := context.Background()

	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Technical Agent",
		Description:   "Agent specialized in technical problem solving",
		Prompt: `You are a technical expert assistant. You:
- Provide accurate, detailed technical explanations
- Use proper technical terminology
- Break down complex concepts step-by-step
- Offer practical solutions and code examples when appropriate
- Stay current with best practices and standards`,
		MaxSteps:      10,
		CreatedBy:     1,
		ModelProvider: "openai",
		ModelID:       "gpt-4o-mini",
	}

	agent, err := agentService.CreateAgent(ctx, config)
	require.NoError(t, err, "Failed to create technical agent")

	// Test technical scenarios
	scenarios := []struct {
		name          string
		task          string
		expectTechnical string
	}{
		{
			name:            "DatabaseOptimization",
			task:            "How can I optimize a slow SQL query that joins multiple tables?",
			expectTechnical: "index|join|query|optimization|performance|explain|database",
		},
		{
			name:            "APIDesign",
			task:            "What are the key principles for designing a RESTful API?",
			expectTechnical: "rest|api|http|endpoint|resource|method|status",
		},
		{
			name:            "SecurityBestPractices", 
			task:            "What security measures should I implement for a web application?",
			expectTechnical: "security|authentication|authorization|encryption|https|xss|csrf",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Logf("🔧 Testing technical scenario: %s", scenario.name)
			t.Logf("📝 Task: %s", scenario.task)

			result, err := agentService.ExecuteAgent(ctx, agent.ID, scenario.task, nil)

			assert.NoError(t, err, "Technical execution should succeed")
			assert.NotNil(t, result, "Result should not be nil")
			assert.NotEmpty(t, result.Content, "Result should have content")

			content := strings.ToLower(result.Content)
			hasTechnical := false
			technicalTerms := strings.Split(scenario.expectTechnical, "|")
			for _, term := range technicalTerms {
				if strings.Contains(content, term) {
					hasTechnical = true
					break
				}
			}

			assert.True(t, hasTechnical, "Response should contain technical terminology")
			assert.Greater(t, len(result.Content), 100, "Technical responses should be detailed")
			t.Logf("🔧 Technical response length: %d characters", len(result.Content))
		})
	}
}

// testAnalyticalAgent tests analytical and reasoning scenarios
func testAnalyticalAgent(t *testing.T, agentService *AgentService, env *models.Environment) {
	ctx := context.Background()

	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Analytical Agent",
		Description:   "Agent specialized in analysis and reasoning",
		Prompt: `You are an analytical thinking assistant. You:
- Break down complex problems systematically
- Consider multiple perspectives and variables
- Use logical reasoning and evidence-based conclusions
- Present structured analysis with clear methodology
- Identify assumptions, risks, and trade-offs`,
		MaxSteps:      12,
		CreatedBy:     1,
		ModelProvider: "openai", 
		ModelID:       "gpt-4o-mini",
	}

	agent, err := agentService.CreateAgent(ctx, config)
	require.NoError(t, err, "Failed to create analytical agent")

	// Test analytical task
	task := "Should a startup choose microservices or monolithic architecture? Analyze the factors."

	t.Logf("📊 Testing analytical reasoning")
	t.Logf("📝 Complex analysis task: %s", task)

	start := time.Now()
	result, err := agentService.ExecuteAgent(ctx, agent.ID, task, nil)
	duration := time.Since(start)

	assert.NoError(t, err, "Analytical execution should succeed")
	assert.NotNil(t, result, "Result should not be nil")
	assert.NotEmpty(t, result.Content, "Result should have content")

	content := strings.ToLower(result.Content)

	// Check for analytical structure
	analyticalIndicators := []string{
		"factor", "consider", "analysis", "advantage", "disadvantage",
		"trade-off", "depends", "complexity", "scalability", "decision",
	}
	
	foundIndicators := 0
	for _, indicator := range analyticalIndicators {
		if strings.Contains(content, indicator) {
			foundIndicators++
		}
	}

	assert.GreaterOrEqual(t, foundIndicators, 3, "Should demonstrate analytical thinking")
	assert.Greater(t, len(result.Content), 500, "Analytical responses should be comprehensive")

	t.Logf("📊 Analysis completed in: %v", duration)
	t.Logf("📊 Found %d analytical indicators", foundIndicators)
	t.Logf("📊 Response length: %d characters", len(result.Content))
}

// testCreativeAgent tests creative and generative scenarios
func testCreativeAgent(t *testing.T, agentService *AgentService, env *models.Environment) {
	ctx := context.Background()

	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Creative Agent",
		Description:   "Agent specialized in creative tasks",
		Prompt: `You are a creative assistant. You:
- Generate original, imaginative ideas
- Think outside the box and explore unique perspectives
- Use vivid, engaging language and storytelling
- Combine concepts in novel ways
- Inspire and spark creativity in others`,
		MaxSteps:      8,
		CreatedBy:     1,
		ModelProvider: "openai",
		ModelID:       "gpt-4o-mini",
	}

	agent, err := agentService.CreateAgent(ctx, config)
	require.NoError(t, err, "Failed to create creative agent")

	// Test creative task
	task := "Write a short story about an AI that discovers it has the ability to dream."

	t.Logf("🎨 Testing creative generation")
	t.Logf("📝 Creative task: %s", task)

	result, err := agentService.ExecuteAgent(ctx, agent.ID, task, nil)

	assert.NoError(t, err, "Creative execution should succeed")
	assert.NotNil(t, result, "Result should not be nil")  
	assert.NotEmpty(t, result.Content, "Result should have content")

	content := strings.ToLower(result.Content)

	// Check for creative elements
	creativeElements := []string{
		"dream", "ai", "discover", "realize", "imagine", "wonder", 
		"thought", "mind", "consciousness", "experience",
	}

	foundElements := 0
	for _, element := range creativeElements {
		if strings.Contains(content, element) {
			foundElements++
		}
	}

	assert.GreaterOrEqual(t, foundElements, 3, "Should demonstrate creative storytelling")
	assert.Greater(t, len(result.Content), 200, "Creative responses should have substance")

	t.Logf("🎨 Found %d creative elements", foundElements)
	t.Logf("🎨 Story length: %d characters", len(result.Content))
}

// testConstrainedAgent tests agents with specific constraints
func testConstrainedAgent(t *testing.T, agentService *AgentService, env *models.Environment) {
	ctx := context.Background()

	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Constrained Agent",
		Description:   "Agent with specific response constraints",
		Prompt: `You are a concise assistant with strict constraints:
- Always respond in exactly 3 sentences
- Never use more than 50 words total
- Start each sentence with a different letter
- Be helpful despite the constraints`,
		MaxSteps:      3, // Limited steps for constrained responses
		CreatedBy:     1,
		ModelProvider: "openai",
		ModelID:       "gpt-4o-mini",
	}

	agent, err := agentService.CreateAgent(ctx, config)
	require.NoError(t, err, "Failed to create constrained agent")

	// Test constrained response
	task := "Explain what machine learning is."

	t.Logf("⚖️  Testing constrained response")
	t.Logf("📝 Task: %s", task)

	result, err := agentService.ExecuteAgent(ctx, agent.ID, task, nil)

	assert.NoError(t, err, "Constrained execution should succeed")
	assert.NotNil(t, result, "Result should not be nil")
	assert.NotEmpty(t, result.Content, "Result should have content")

	// Count sentences (approximate)
	sentences := strings.Split(strings.TrimSpace(result.Content), ".")
	actualSentences := 0
	for _, sentence := range sentences {
		if strings.TrimSpace(sentence) != "" {
			actualSentences++
		}
	}

	// Count words
	words := strings.Fields(result.Content)
	wordCount := len(words)

	// Should be close to constraints (allow some flexibility)
	assert.LessOrEqual(t, wordCount, 60, "Should respect word limit constraint (with some flexibility)")
	assert.Contains(t, strings.ToLower(result.Content), "machine learning", "Should address the topic")

	t.Logf("⚖️  Constraint compliance - Sentences: %d, Words: %d", actualSentences, wordCount)
	t.Logf("⚖️  Response: %s", result.Content)
}

// testVariableSubstitution tests user variable substitution in agent execution
func testVariableSubstitution(t *testing.T, agentService *AgentService, env *models.Environment) {
	ctx := context.Background()

	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Variable Substitution Agent",
		Description:   "Agent that processes user variables",
		Prompt: `You are a personalized assistant. Use the provided user variables to customize your response.
Always acknowledge the user by name if provided and reference their context when relevant.`,
		MaxSteps:      5,
		CreatedBy:     1,
		ModelProvider: "openai",
		ModelID:       "gpt-4o-mini",
	}

	agent, err := agentService.CreateAgent(ctx, config)
	require.NoError(t, err, "Failed to create variable substitution agent")

	// Test with user variables
	userVariables := map[string]interface{}{
		"user_name":    "Alice",
		"user_role":    "Software Engineer",
		"company":      "TechCorp",
		"project_name": "CloudStation",
		"difficulty":   "intermediate",
	}

	task := "Help me understand best practices for code reviews in my current project."

	t.Logf("🏷️  Testing variable substitution")
	t.Logf("📝 Task: %s", task)
	t.Logf("📋 Variables: %+v", userVariables)

	result, err := agentService.ExecuteAgent(ctx, agent.ID, task, userVariables)

	assert.NoError(t, err, "Variable substitution execution should succeed")
	assert.NotNil(t, result, "Result should not be nil")
	assert.NotEmpty(t, result.Content, "Result should have content")

	// Check if variables were processed (in metadata)
	if result.Extra != nil {
		if userVars, ok := result.Extra["user_variables"]; ok {
			t.Logf("🏷️  Variables processed: %+v", userVars)
		}
	}

	t.Logf("🏷️  Response: %s", result.Content)
}

// TestAgentExecutionDebugScenarios tests specific debugging scenarios
func TestAgentExecutionDebugScenarios(t *testing.T) {
	// Skip if OPENAI_API_KEY not set
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		t.Skip("OPENAI_API_KEY not set - skipping debug scenario tests")
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

	t.Run("ExecutionWithDetailedLogging", func(t *testing.T) {
		testExecutionWithDetailedLogging(t, agentService, env)
	})

	t.Run("PerformanceMetrics", func(t *testing.T) {
		testPerformanceMetrics(t, agentService, env)
	})

	t.Run("TokenUsageTracking", func(t *testing.T) {
		testTokenUsageTracking(t, agentService, env)
	})
}

// testExecutionWithDetailedLogging tests detailed execution logging
func testExecutionWithDetailedLogging(t *testing.T, agentService *AgentService, env *models.Environment) {
	ctx := context.Background()

	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Debug Logging Agent",
		Description:   "Agent for testing detailed execution logging",
		Prompt: `You are a step-by-step problem solver. Always:
1. Acknowledge the request
2. Break down the approach
3. Work through each step
4. Provide a clear conclusion`,
		MaxSteps:      8,
		CreatedBy:     1,
		ModelProvider: "openai",
		ModelID:       "gpt-4o-mini",
	}

	agent, err := agentService.CreateAgent(ctx, config)
	require.NoError(t, err, "Failed to create debug logging agent")

	task := "Calculate the factorial of 5 and explain each step."

	t.Logf("🐛 Testing detailed execution logging")
	t.Logf("📝 Task: %s", task)

	// Capture start time for performance measurement
	start := time.Now()

	result, err := agentService.ExecuteAgent(ctx, agent.ID, task, nil)

	duration := time.Since(start)

	assert.NoError(t, err, "Debug logging execution should succeed")
	assert.NotNil(t, result, "Result should not be nil")
	assert.NotEmpty(t, result.Content, "Result should have content")

	// Verify step-by-step response
	content := strings.ToLower(result.Content)
	assert.Contains(t, content, "factorial", "Should mention factorial")
	assert.Contains(t, content, "5", "Should reference the number 5")

	// Check for step-by-step indicators
	stepIndicators := []string{"step", "first", "then", "next", "finally", "1", "2", "3"}
	foundSteps := 0
	for _, indicator := range stepIndicators {
		if strings.Contains(content, indicator) {
			foundSteps++
		}
	}

	assert.GreaterOrEqual(t, foundSteps, 2, "Should show step-by-step reasoning")

	t.Logf("🐛 Execution duration: %v", duration)
	t.Logf("🐛 Step indicators found: %d", foundSteps)
	t.Logf("🐛 Response length: %d characters", len(result.Content))
}

// testPerformanceMetrics tests performance measurement capabilities
func testPerformanceMetrics(t *testing.T, agentService *AgentService, env *models.Environment) {
	ctx := context.Background()

	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Performance Test Agent",
		Description:   "Agent for performance measurement testing",
		Prompt:        "You are an efficient assistant. Provide concise, accurate answers.",
		MaxSteps:      3, // Keep it simple for consistent performance testing
		CreatedBy:     1,
		ModelProvider: "openai",
		ModelID:       "gpt-4o-mini",
	}

	agent, err := agentService.CreateAgent(ctx, config)
	require.NoError(t, err, "Failed to create performance test agent")

	// Run multiple executions to measure performance consistency
	executions := 3
	durations := make([]time.Duration, executions)

	for i := 0; i < executions; i++ {
		task := "What is 15 * 7?"

		start := time.Now()
		result, err := agentService.ExecuteAgent(ctx, agent.ID, task, nil)
		durations[i] = time.Since(start)

		assert.NoError(t, err, "Performance test execution should succeed")
		assert.NotNil(t, result, "Result should not be nil")
		assert.Contains(t, result.Content, "105", "Should provide correct calculation")

		t.Logf("📊 Execution %d duration: %v", i+1, durations[i])
	}

	// Calculate average duration
	totalDuration := time.Duration(0)
	for _, d := range durations {
		totalDuration += d
	}
	avgDuration := totalDuration / time.Duration(executions)

	t.Logf("📊 Average execution duration: %v", avgDuration)
	t.Logf("📊 Performance consistency: executions completed successfully")

	// Performance should be reasonable (not too slow)
	assert.Less(t, avgDuration, 30*time.Second, "Average execution should be reasonably fast")
}

// testTokenUsageTracking tests token usage measurement
func testTokenUsageTracking(t *testing.T, agentService *AgentService, env *models.Environment) {
	ctx := context.Background()

	config := &AgentConfig{
		EnvironmentID: env.ID,
		Name:          "Token Tracking Agent",
		Description:   "Agent for token usage tracking",
		Prompt:        "You are a helpful assistant that provides detailed explanations.",
		MaxSteps:      5,
		CreatedBy:     1,
		ModelProvider: "openai",
		ModelID:       "gpt-4o-mini",
	}

	agent, err := agentService.CreateAgent(ctx, config)
	require.NoError(t, err, "Failed to create token tracking agent")

	task := "Explain the concept of recursion in programming with a simple example."

	t.Logf("🎫 Testing token usage tracking")
	t.Logf("📝 Task: %s", task)

	result, err := agentService.ExecuteAgent(ctx, agent.ID, task, nil)

	assert.NoError(t, err, "Token tracking execution should succeed")
	assert.NotNil(t, result, "Result should not be nil")
	assert.NotEmpty(t, result.Content, "Result should have content")

	// Check for token usage in metadata
	if result.Extra != nil {
		if tokenUsage, ok := result.Extra["token_usage"]; ok {
			t.Logf("🎫 Token usage data: %+v", tokenUsage)

			// Verify token usage structure
			if tokenMap, ok := tokenUsage.(map[string]interface{}); ok {
				if inputTokens, hasInput := tokenMap["input_tokens"]; hasInput {
					t.Logf("🎫 Input tokens: %v", inputTokens)
				}
				if outputTokens, hasOutput := tokenMap["output_tokens"]; hasOutput {
					t.Logf("🎫 Output tokens: %v", outputTokens)
				}
				if totalTokens, hasTotal := tokenMap["total_tokens"]; hasTotal {
					t.Logf("🎫 Total tokens: %v", totalTokens)
				}
			}
		}
	}

	// Verify response quality
	content := strings.ToLower(result.Content)
	assert.Contains(t, content, "recursion", "Should explain recursion")
	assert.True(t, 
		strings.Contains(content, "example") || strings.Contains(content, "function"),
		"Should include examples or function references")

	t.Logf("🎫 Response length: %d characters", len(result.Content))
}