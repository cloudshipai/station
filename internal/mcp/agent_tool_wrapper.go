package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"station/internal/db/repositories"
	"station/internal/logging"
	"station/internal/services"
	"station/pkg/models"
)

// AgentToolWrapper wraps an agent as an MCP tool for GenKit integration
type AgentToolWrapper struct {
	agent       *models.Agent
	environment string
	repos       *repositories.Repositories
	parentRunID int64 // Track parent run for proper hierarchy
}

// NewAgentToolWrapper creates a new agent tool wrapper
func NewAgentToolWrapper(agent *models.Agent, environment string, repos *repositories.Repositories, parentRunID int64) *AgentToolWrapper {
	return &AgentToolWrapper{
		agent:       agent,
		environment: environment,
		repos:       repos,
		parentRunID: parentRunID,
	}
}

// Name returns the tool name for GenKit
func (w *AgentToolWrapper) Name() string {
	// Normalize agent name to match tool naming convention
	normalizedName := strings.ToLower(w.agent.Name)
	// Replace all special characters with underscores
	replacements := []string{" ", "-", ".", "!", "@", "#", "$", "%", "^", "&", "*", "(", ")", "+", "=", "[", "]", "{", "}", "|", "\\", ":", ";", "\"", "'", "<", ">", ",", "?", "/"}
	for _, char := range replacements {
		normalizedName = strings.ReplaceAll(normalizedName, char, "_")
	}
	// Remove multiple consecutive underscores
	for strings.Contains(normalizedName, "__") {
		normalizedName = strings.ReplaceAll(normalizedName, "__", "_")
	}
	// Trim leading/trailing underscores
	normalizedName = strings.Trim(normalizedName, "_")

	return fmt.Sprintf("__agent_%s", normalizedName)
}

// Description returns the tool description for GenKit
func (w *AgentToolWrapper) Description() string {
	return w.agent.Description
}

// Call executes the agent as a tool with proper runID handling
func (w *AgentToolWrapper) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	logging.Debug("AgentToolWrapper.Call: Executing agent %s as tool with parent run ID %d", w.agent.Name, w.parentRunID)

	// Create proper execution with runID (same as regular agents)
	runID, err := w.createAgentRun(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent run: %w", err)
	}

	logging.Debug("AgentToolWrapper.Call: Created run ID %d for agent %s", runID, w.agent.Name)

	// Create execution engine with proper context
	engine := services.NewAgentExecutionEngine(w.repos, nil)

	// Convert structured input to task (respects agent's input schema)
	task, err := w.convertInputToTask(input)
	if err != nil {
		return nil, fmt.Errorf("input conversion failed: %w", err)
	}

	logging.Debug("AgentToolWrapper.Call: Converted input to task: %s", task)

	// Update the run with the task before execution
	updateErr := w.repos.AgentRuns.UpdateCompletionWithMetadata(
		ctx, runID, "", 0, nil, nil, "running", nil,
		nil, nil, nil, nil, nil, nil, nil,
	)
	if updateErr != nil {
		logging.Debug("Warning: Failed to update run %d with task: %v", runID, updateErr)
	}

	// Execute with same runID system as all other agents
	result, err := engine.ExecuteAgent(ctx, w.agent, task, runID)

	// Update run completion with result metadata (CRITICAL for child run persistence)
	completedAt := time.Now()
	if err != nil {
		// Update run as failed
		errorMsg := fmt.Sprintf("Child agent execution failed: %v", err)
		updateErr := w.repos.AgentRuns.UpdateCompletionWithMetadata(
			ctx, runID, errorMsg, 0, nil, nil, "failed", &completedAt,
			nil, nil, nil, nil, nil, nil, &errorMsg,
		)
		if updateErr != nil {
			logging.Debug("Warning: Failed to update failed child run %d: %v", runID, updateErr)
		}
		return nil, fmt.Errorf("agent execution failed: %w", err)
	}

	logging.Debug("AgentToolWrapper.Call: Agent execution completed successfully")
	logging.Debug("🔍 Child agent result - Response length: %d, ToolCalls: %v, ExecutionSteps: %v, StepsTaken: %d",
		len(result.Response),
		result.ToolCalls != nil && len(*result.ToolCalls) > 0,
		result.ExecutionSteps != nil && len(*result.ExecutionSteps) > 0,
		result.StepsTaken)

	if result.ToolCalls != nil {
		logging.Debug("🔍 Child agent ToolCalls count: %d", len(*result.ToolCalls))
	}
	if result.ExecutionSteps != nil {
		logging.Debug("🔍 Child agent ExecutionSteps count: %d", len(*result.ExecutionSteps))
	}

	// Extract token usage from result for database persistence
	var inputTokens, outputTokens, totalTokens *int64
	if result.TokenUsage != nil {
		if inputVal, ok := result.TokenUsage["inputTokens"].(int64); ok {
			inputTokens = &inputVal
		} else if inputVal, ok := result.TokenUsage["inputTokens"].(float64); ok {
			inputTokensInt := int64(inputVal)
			inputTokens = &inputTokensInt
		}

		if outputVal, ok := result.TokenUsage["outputTokens"].(int64); ok {
			outputTokens = &outputVal
		} else if outputVal, ok := result.TokenUsage["outputTokens"].(float64); ok {
			outputTokensInt := int64(outputVal)
			outputTokens = &outputTokensInt
		}

		if totalVal, ok := result.TokenUsage["totalTokens"].(int64); ok {
			totalTokens = &totalVal
		} else if totalVal, ok := result.TokenUsage["totalTokens"].(float64); ok {
			totalTokensInt := int64(totalVal)
			totalTokens = &totalTokensInt
		}
	}

	var toolsUsed *int64
	if result.ToolsUsed > 0 {
		toolsUsedVal := int64(result.ToolsUsed)
		toolsUsed = &toolsUsedVal
	}

	var durationSeconds *float64
	if result.Duration > 0 {
		durationSec := result.Duration.Seconds()
		durationSeconds = &durationSec
	}

	var modelName *string
	if result.ModelName != "" {
		modelName = &result.ModelName
	}

	// Determine status based on execution result
	status := "completed"
	var errorMsg *string
	if !result.Success {
		status = "failed"
		if result.Error != "" {
			errorMsg = &result.Error
		}
	}

	// Update database with complete child run metadata (CRITICAL!)
	logging.Debug("AgentToolWrapper.Call: Updating child run %d with completion metadata", runID)
	updateErr = w.repos.AgentRuns.UpdateCompletionWithMetadata(
		ctx,
		runID,
		result.Response,       // final_response
		result.StepsTaken,     // steps_taken
		result.ToolCalls,      // tool_calls
		result.ExecutionSteps, // execution_steps
		status,                // status
		&completedAt,          // completed_at
		inputTokens,           // input_tokens
		outputTokens,          // output_tokens
		totalTokens,           // total_tokens
		durationSeconds,       // duration_seconds
		modelName,             // model_name
		toolsUsed,             // tools_used
		errorMsg,              // error
	)
	if updateErr != nil {
		logging.Debug("Warning: Failed to update child run %d completion: %v", runID, updateErr)
	} else {
		logging.Debug("✅ AgentToolWrapper.Call: Successfully saved child run %d completion metadata", runID)
	}

	// Format response preserving structure
	return w.formatToolResponse(result), nil
}

// createAgentRun creates a run record same way as regular agent execution
func (w *AgentToolWrapper) createAgentRun(ctx context.Context) (int64, error) {
	// Get parent run ID from context for hierarchical tracking
	parentRunID := services.GetParentRunIDFromContext(ctx)

	if parentRunID != nil {
		logging.Debug("AgentToolWrapper.createAgentRun: Creating child run for agent %s with parent_run_id=%d", w.agent.Name, *parentRunID)
	} else {
		logging.Debug("AgentToolWrapper.createAgentRun: Creating root run for agent %s (no parent)", w.agent.Name)
	}

	// For now, use userID = 0 (system) since we don't have user context in tool calls
	// In the future, we might want to pass user context through the execution chain
	// Use CreateWithMetadata to properly set parent_run_id
	run, err := w.repos.AgentRuns.CreateWithMetadata(
		ctx,
		w.agent.ID,
		0,           // userID - system user for now
		"",          // task - will be updated during execution
		"",          // finalResponse - will be updated on completion
		0,           // stepsTaken - will be updated on completion
		nil,         // toolCalls - will be updated on completion
		nil,         // executionSteps - will be updated on completion
		"running",   // status
		nil,         // completedAt - will be set on completion
		nil,         // inputTokens - will be updated on completion
		nil,         // outputTokens - will be updated on completion
		nil,         // totalTokens - will be updated on completion
		nil,         // durationSeconds - will be updated on completion
		nil,         // modelName - will be updated on completion
		nil,         // toolsUsed - will be updated on completion
		parentRunID, // parentRunID - CRITICAL: links child to parent
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create agent run: %w", err)
	}

	logging.Debug("AgentToolWrapper.createAgentRun: Created run ID %d for agent %s (parent=%v)", run.ID, w.agent.Name, parentRunID)
	return run.ID, nil
}

// convertInputToTask converts structured input to task string based on agent's input schema
func (w *AgentToolWrapper) convertInputToTask(input map[string]interface{}) (string, error) {
	// If agent has explicit input schema, validate and format accordingly
	if w.agent.InputSchema != nil {
		return w.formatStructuredInput(input), nil
	}

	// Check for query parameter first
	if query, ok := input["query"].(string); ok && query != "" {
		return query, nil
	}

	// Build task from all input parameters
	var parts []string
	for key, value := range input {
		if str, ok := value.(string); ok {
			parts = append(parts, fmt.Sprintf("%s: %s", key, str))
		} else {
			// Handle complex types
			jsonBytes, err := json.Marshal(value)
			if err != nil {
				return "", fmt.Errorf("failed to marshal input value for key %s: %w", key, err)
			}
			parts = append(parts, fmt.Sprintf("%s: %s", key, string(jsonBytes)))
		}
	}

	if len(parts) == 0 {
		return "Execute agent task", nil
	}

	return strings.Join(parts, "\n"), nil
}

// formatStructuredInput formats input based on agent's input schema
func (w *AgentToolWrapper) formatStructuredInput(input map[string]interface{}) string {
	// Build task from structured input based on agent's schema
	var parts []string

	for key, value := range input {
		if str, ok := value.(string); ok {
			parts = append(parts, fmt.Sprintf("%s: %s", key, str))
		} else {
			// Handle complex types
			jsonBytes, _ := json.Marshal(value)
			parts = append(parts, fmt.Sprintf("%s: %s", key, string(jsonBytes)))
		}
	}

	if len(parts) == 0 {
		return "Execute agent task"
	}

	return strings.Join(parts, "\n")
}

// formatToolResponse formats the agent execution result for tool response
func (w *AgentToolWrapper) formatToolResponse(result *services.AgentExecutionResult) map[string]interface{} {
	response := map[string]interface{}{
		"response":   result.Response,
		"success":    result.Success,
		"tool_calls": result.ToolCalls,
		"duration":   result.Duration.Seconds(),
		"steps_used": result.StepsUsed,
	}

	// Include additional metadata if available
	if result.TokenUsage != nil {
		response["token_usage"] = result.TokenUsage
	}
	if result.Error != "" {
		response["error"] = result.Error
	}

	return response
}

// GetInputSchema returns the JSON schema for this agent tool
func (w *AgentToolWrapper) GetInputSchema() map[string]interface{} {
	extractor := &AgentSchemaExtractor{}
	return extractor.ExtractInputSchema(w.agent)
}

// AgentSchemaExtractor extracts JSON schema from agent's input definition
type AgentSchemaExtractor struct{}

// ExtractInputSchema extracts JSON schema from agent's input definition
func (e *AgentSchemaExtractor) ExtractInputSchema(agent *models.Agent) map[string]interface{} {
	// If agent has explicit input schema, convert it properly
	if agent.InputSchema != nil {
		return e.convertAgentInputSchema(*agent.InputSchema)
	}

	// Default schema for agents without explicit input
	return e.createDefaultInputSchema()
}

func (e *AgentSchemaExtractor) convertAgentInputSchema(agentSchema string) map[string]interface{} {
	// Parse the JSON schema string
	var schemaMap map[string]interface{}
	if err := json.Unmarshal([]byte(agentSchema), &schemaMap); err != nil {
		logging.Debug("AgentSchemaExtractor.convertAgentInputSchema: Failed to parse input schema, using default: %v", err)
		return e.createDefaultInputSchema()
	}

	return schemaMap
}

func (e *AgentSchemaExtractor) createDefaultInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "The task or query for the agent",
			},
		},
		"required": []string{"query"},
	}
}
