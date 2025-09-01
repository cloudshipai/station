package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"station/internal/db/repositories"
	"station/internal/logging"
	"station/pkg/models"
	dotprompt "station/pkg/dotprompt"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/plugins/mcp"
	googledotprompt "github.com/google/dotprompt/go/dotprompt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ExecutionTracker tracks tool execution and builds user-friendly execution steps
type ExecutionTracker struct {
	runID         int64
	toolCalls     []interface{}
	executionSteps []interface{}
	activeTools   map[string]ToolExecution
	stepCounter   int
}

// ToolExecution represents an active tool execution
type ToolExecution struct {
	ExecutionID string
	ToolName    string
	StartTime   time.Time
	Parameters  map[string]interface{}
}

// ProcessLogEntry processes log entries from Station GenKit and builds execution steps
func (et *ExecutionTracker) ProcessLogEntry(logEntry map[string]interface{}, repos *repositories.Repositories) {
	message, ok := logEntry["message"].(string)
	if !ok {
		log.Printf("🔧 EXECUTION-TRACKER: Received log entry without message field: %+v", logEntry)
		return
	}
	
	details, hasDetails := logEntry["details"].(map[string]interface{})
	if !hasDetails && message != "Station GenKit execution completed successfully" {
		log.Printf("🔧 EXECUTION-TRACKER: Received message '%s' without details field", message)
		return  
	}
	
	if hasDetails {
		log.Printf("🔧 EXECUTION-TRACKER: Processing message: '%s' with details keys: %v", message, getDebugMapKeys(details))
	} else {
		log.Printf("🔧 EXECUTION-TRACKER: Processing message: '%s' with no details field", message)
	}
	
	switch message {
	case "Tool execution starting":
		log.Printf("🔧 EXECUTION-TRACKER: Handling tool start")
		et.handleToolStart(details)
		
	case "Tool execution completed":
		log.Printf("🔧 EXECUTION-TRACKER: Handling tool complete")
		et.handleToolComplete(details, repos)
		
	case "Enhanced generation completed":
		log.Printf("🔧 EXECUTION-TRACKER: Handling generation complete")
		et.handleGenerationComplete(details, repos)
	case "Station GenKit execution completed successfully":
		log.Printf("🔧 EXECUTION-TRACKER: Handling Station GenKit completion with tool calls")
		et.handleStationGenKitComplete(logEntry, repos)
	case "Station GenKit generation completed: success":
		log.Printf("🔧 EXECUTION-TRACKER: Handling Station GenKit generation completion (with tool_calls in details)")
		et.handleStationGenKitComplete(logEntry, repos)
	default:
		log.Printf("🔧 EXECUTION-TRACKER: Unknown message type: '%s'", message)
	}
}

// Helper function to get map keys for debugging
func getDebugMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (et *ExecutionTracker) handleToolStart(details map[string]interface{}) {
	executionID, ok1 := details["execution_id"].(string)
	toolName, ok2 := details["tool_name"].(string)
	
	if !ok1 || !ok2 {
		return
	}
	
	// Store active tool execution
	et.activeTools[executionID] = ToolExecution{
		ExecutionID: executionID,
		ToolName:    toolName,
		StartTime:   time.Now(),
		Parameters:  make(map[string]interface{}), // Could extract from details if available
	}
	
	// Create execution step for tool call start
	step := map[string]interface{}{
		"step":       et.stepCounter,
		"type":       "tool_call_start",
		"tool_name":  toolName,
		"execution_id": executionID,
		"content":    fmt.Sprintf("Calling %s", toolName),
		"timestamp":  time.Now().Format(time.RFC3339),
	}
	
	et.executionSteps = append(et.executionSteps, step)
	et.stepCounter++
}

func (et *ExecutionTracker) handleToolComplete(details map[string]interface{}, repos *repositories.Repositories) {
	executionID, ok1 := details["execution_id"].(string)
	toolName, ok2 := details["tool_name"].(string)
	success, ok3 := details["success"].(bool)
	durationMs, ok4 := details["duration_ms"].(float64)
	outputLength, ok5 := details["output_length"].(float64)
	
	if !ok1 || !ok2 || !ok3 {
		return
	}
	
	// Get the active tool execution
	activeExec, exists := et.activeTools[executionID]
	if !exists {
		return
	}

	// Extract tool output response if available
	var outputResponse interface{}
	if outputData, hasOutput := details["output"]; hasOutput {
		outputResponse = outputData
	}
	
	// Create enhanced tool call record with actual input/output data
	toolCall := map[string]interface{}{
		"tool_name":    toolName,
		"execution_id": executionID,
		"success":      success,
		"duration_ms":  durationMs,
		"output_length": outputLength,
		"started_at":   activeExec.StartTime.Format(time.RFC3339),
		"completed_at": time.Now().Format(time.RFC3339),
	}

	// Add input parameters from the active tool execution
	if activeExec.Parameters != nil && len(activeExec.Parameters) > 0 {
		toolCall["input_parameters"] = activeExec.Parameters
	}

	// Add output response if available
	if outputResponse != nil {
		toolCall["output_response"] = outputResponse
	}
	
	if errorMsg, hasError := details["error"].(string); hasError && errorMsg != "" {
		toolCall["error"] = errorMsg
	}
	
	et.toolCalls = append(et.toolCalls, toolCall)
	
	// Create enhanced execution step for tool completion
	var content string
	if success {
		if ok4 && ok5 {
			content = fmt.Sprintf("%s completed successfully (%.0fms, %d chars output)", 
				toolName, durationMs, int(outputLength))
		} else {
			content = fmt.Sprintf("%s completed successfully", toolName)
		}
		
		// Add output summary to content if available
		if outputResponse != nil {
			switch v := outputResponse.(type) {
			case string:
				if len(v) > 100 {
					content += fmt.Sprintf(" - Response: %s...", v[:100])
				} else if len(v) > 0 {
					content += fmt.Sprintf(" - Response: %s", v)
				}
			case map[string]interface{}:
				content += fmt.Sprintf(" - Response: JSON object with %d fields", len(v))
			default:
				content += fmt.Sprintf(" - Response: %T", v)
			}
		}
	} else {
		content = fmt.Sprintf("%s failed", toolName)
		if errorMsg, hasError := details["error"].(string); hasError {
			content += fmt.Sprintf(": %s", errorMsg)
		}
	}
	
	step := map[string]interface{}{
		"step":         et.stepCounter,
		"type":         "tool_call_complete",
		"tool_name":    toolName,
		"execution_id": executionID,
		"content":      content,
		"timestamp":    time.Now().Format(time.RFC3339),
		"success":      success,
		"duration_ms":  durationMs,
	}
	
	// Add output response to execution step if available
	if outputResponse != nil {
		step["output_response"] = outputResponse
	}
	
	// Add input parameters to completion step as well for completeness
	if activeExec.Parameters != nil && len(activeExec.Parameters) > 0 {
		step["input_parameters"] = activeExec.Parameters
	}
	
	et.executionSteps = append(et.executionSteps, step)
	et.stepCounter++
	
	// Clean up active tool
	delete(et.activeTools, executionID)
}

func (et *ExecutionTracker) handleGenerationComplete(details map[string]interface{}, repos *repositories.Repositories) {
	// Add final generation step
	step := map[string]interface{}{
		"step":      et.stepCounter,
		"type":      "generation_complete", 
		"content":   "AI response generation completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	if duration, ok := details["duration"].(float64); ok {
		step["duration_seconds"] = duration / 1000.0 // Convert ms to seconds
	}
	
	et.executionSteps = append(et.executionSteps, step)
	et.stepCounter++
}

// handleStationGenKitComplete processes tool call data from Station GenKit completion
func (et *ExecutionTracker) handleStationGenKitComplete(logEntry map[string]interface{}, repos *repositories.Repositories) {
	log.Printf("🔧 EXECUTION-TRACKER: Processing Station GenKit completion logEntry: %+v", logEntry)
	
	// Debug: Check what keys are available in the top-level logEntry
	log.Printf("🔧 EXECUTION-TRACKER: Available logEntry keys: %v", getDebugMapKeys(logEntry))
	
	// Extract details from logEntry
	details, hasDetails := logEntry["details"].(map[string]interface{})
	if !hasDetails {
		log.Printf("🔧 EXECUTION-TRACKER: No details found in logEntry")
		return
	}
	
	// Extract tool call information from details field (station.go puts tool_calls in details)
	if toolCallsData, ok := details["tool_calls"].(map[string]interface{}); ok {
		log.Printf("🔧 EXECUTION-TRACKER: Found tool_calls data: %+v", toolCallsData)
		
		// Process conversation tool calls (from GenKit conversation history)
		if conversationToolCalls, ok := toolCallsData["conversation_tool_calls"].([]map[string]interface{}); ok {
			log.Printf("🔧 EXECUTION-TRACKER: Processing %d conversation tool calls", len(conversationToolCalls))
			
			for i, toolCall := range conversationToolCalls {
				log.Printf("🔧 EXECUTION-TRACKER: Processing tool call %d: %+v", i, toolCall)
				
				// Extract tool call data
				toolName, hasName := toolCall["tool_name"].(string)
				input, hasInput := toolCall["input"]
				output, hasOutput := toolCall["output"]
				
				if hasName {
					// Add tool call to the tracked tool calls
					toolCallEntry := map[string]interface{}{
						"tool_name": toolName,
						"input":     input,
						"output":    output,
						"timestamp": time.Now().Format(time.RFC3339),
					}
					
					et.toolCalls = append(et.toolCalls, toolCallEntry)
					log.Printf("🔧 EXECUTION-TRACKER: Added tool call: %s", toolName)
					
					// Add execution step for this tool call
					step := map[string]interface{}{
						"step":      et.stepCounter,
						"type":      "tool_call",
						"content":   fmt.Sprintf("Called %s", toolName),
						"timestamp": time.Now().Format(time.RFC3339),
						"tool_name": toolName,
					}
					
					if hasInput {
						step["input"] = input
					}
					if hasOutput {
						step["output"] = output
					}
					
					et.executionSteps = append(et.executionSteps, step)
					et.stepCounter++
					
					log.Printf("🔧 EXECUTION-TRACKER: Added execution step for tool: %s", toolName)
				}
			}
		}
	} else {
		log.Printf("🔧 EXECUTION-TRACKER: No tool_calls found in details")
	}
	
	// Add final completion step
	step := map[string]interface{}{
		"step":      et.stepCounter,
		"type":      "model_response",
		"content":   "Generation completed with tool calls processed",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	et.executionSteps = append(et.executionSteps, step)
	et.stepCounter++
	
	log.Printf("🔧 EXECUTION-TRACKER: Completed Station GenKit processing - %d tool calls, %d steps", len(et.toolCalls), len(et.executionSteps))
}

// GetExecutionData returns the collected tool calls and execution steps
func (et *ExecutionTracker) GetExecutionData() ([]interface{}, []interface{}) {
	return et.toolCalls, et.executionSteps
}

// AgentExecutionResult contains the result of an agent execution
type AgentExecutionResult struct {
	Success        bool                     `json:"success"`
	Response       string                   `json:"response"`
	ToolCalls      *models.JSONArray        `json:"tool_calls"`
	Steps          []interface{}            `json:"steps"`
	ExecutionSteps *models.JSONArray        `json:"execution_steps"` // For database storage
	Duration       time.Duration            `json:"duration"`
	TokenUsage     map[string]interface{}   `json:"token_usage,omitempty"`
	ModelName      string                   `json:"model_name"`
	StepsUsed      int                      `json:"steps_used"`
	StepsTaken     int64                    `json:"steps_taken"` // For database storage
	ToolsUsed      int                      `json:"tools_used"`
	Error          string                   `json:"error,omitempty"`
}

// AgentExecutionEngine handles the execution of agents using GenKit and MCP
type AgentExecutionEngine struct {
	repos              *repositories.Repositories
	agentService       AgentServiceInterface
	genkitProvider     *GenKitProvider
	mcpConnManager     *MCPConnectionManager
	telemetryManager   *TelemetryManager
	telemetryService   *TelemetryService // For creating spans
	activeMCPClients   []*mcp.GenkitMCPClient // Store active connections for cleanup after execution
}

// NewAgentExecutionEngine creates a new agent execution engine
func NewAgentExecutionEngine(repos *repositories.Repositories, agentService AgentServiceInterface) *AgentExecutionEngine {
	mcpConnManager := NewMCPConnectionManager(repos, nil)
	
	// Check environment variable for connection pooling
	if os.Getenv("STATION_MCP_POOLING") == "true" {
		mcpConnManager.EnableConnectionPooling()
		logging.Info("🏊 MCP connection pooling enabled via STATION_MCP_POOLING environment variable")
	}
	
	return &AgentExecutionEngine{
		repos:             repos,
		agentService:      agentService,
		genkitProvider:    NewGenKitProvider(),
		mcpConnManager:    mcpConnManager,
		telemetryManager:  NewTelemetryManager(),
	}
}

// ExecuteAgentViaStdioMCP executes an agent using self-bootstrapping stdio MCP architecture
func (aee *AgentExecutionEngine) ExecuteAgentViaStdioMCP(ctx context.Context, agent *models.Agent, task string, runID int64) (*AgentExecutionResult, error) {
	// Default to empty user variables for backward compatibility
	return aee.ExecuteAgentViaStdioMCPWithVariables(ctx, agent, task, runID, map[string]interface{}{})
}

// ExecuteAgentViaStdioMCPWithVariables executes an agent with user-defined variables for dotprompt rendering
func (aee *AgentExecutionEngine) ExecuteAgentViaStdioMCPWithVariables(ctx context.Context, agent *models.Agent, task string, runID int64, userVariables map[string]interface{}) (*AgentExecutionResult, error) {
	startTime := time.Now()
	logging.Info("Starting unified dotprompt execution for agent '%s'", agent.Name)

	// Create telemetry span if telemetry service is available
	var span trace.Span
	if aee.telemetryService != nil {
		ctx, span = aee.telemetryService.StartSpan(ctx, "agent_execution_engine.execute",
			trace.WithAttributes(
				attribute.String("agent.name", agent.Name),
				attribute.Int64("agent.id", agent.ID),
				attribute.Int64("run.id", runID),
				attribute.Int("user_variables.count", len(userVariables)),
			),
		)
		defer span.End()
	}

	// Log execution start
	err := aee.repos.AgentRuns.AppendDebugLog(ctx, runID, map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     "info",
		"message":   fmt.Sprintf("Starting execution for agent '%s'", agent.Name),
		"details": map[string]interface{}{
			"agent_id": agent.ID,
			"task":     task,
		},
	})
	if err != nil {
		logging.Debug("Failed to log execution start: %v", err)
	}

	// All agents now use unified dotprompt execution system
		
		// Note: MCP cleanup will happen after dotprompt execution completes
		// Do NOT defer cleanup here as it would disconnect connections while LLM is still using tools
		
		// Get agent tools for the new dotprompt system
		agentTools, err := aee.repos.AgentTools.ListAgentTools(agent.ID)
		if err != nil {
			if span != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to get agent tools")
			}
			return nil, fmt.Errorf("failed to get agent tools for dotprompt execution: %w", err)
		}
		
		if span != nil {
			span.SetAttributes(attribute.Int("agent.tools_count", len(agentTools)))
		}
		
		// Get GenKit app for dotprompt execution
		genkitApp, err := aee.genkitProvider.GetApp(ctx)
		if err != nil {
			if span != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to get genkit app")
			}
			return nil, fmt.Errorf("failed to get genkit app for dotprompt execution: %w", err)
		}
		
		// Update MCP connection manager with GenKit app (same as traditional)
		aee.mcpConnManager.genkitApp = genkitApp
		
		// Initialize server pool if pooling is enabled (same as traditional)
		if err := aee.mcpConnManager.InitializeServerPool(ctx); err != nil {
			logging.Info("Warning: Failed to initialize MCP server pool for dotprompt: %v", err)
		}
		
		// Load MCP tools for dotprompt execution (reuse the same logic as traditional execution)
		var mcpLoadSpan trace.Span
		if span != nil {
			ctx, mcpLoadSpan = aee.telemetryService.StartSpan(ctx, "mcp.load_tools",
				trace.WithAttributes(
					attribute.Int64("environment.id", agent.EnvironmentID),
				),
			)
			defer mcpLoadSpan.End()
		}
		
		allMCPTools, mcpClients, err := aee.mcpConnManager.GetEnvironmentMCPTools(ctx, agent.EnvironmentID)
		if err != nil {
			if mcpLoadSpan != nil {
				mcpLoadSpan.RecordError(err)
				mcpLoadSpan.SetStatus(codes.Error, "failed to load MCP tools")
			}
			if span != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to get environment MCP tools")
			}
			return nil, fmt.Errorf("failed to get environment MCP tools for dotprompt execution: %w", err)
		}
		
		if mcpLoadSpan != nil {
			mcpLoadSpan.SetAttributes(
				attribute.Int("mcp.tools_loaded", len(allMCPTools)),
				attribute.Int("mcp.clients_connected", len(mcpClients)),
			)
		}
		
		// Store clients for cleanup after execution
		aee.activeMCPClients = mcpClients
		
		// Filter to only include tools assigned to this agent (same filtering logic as traditional)
		logging.Debug("Filtering %d assigned tools from %d available MCP tools", len(agentTools), len(allMCPTools))
		var mcpTools []ai.ToolRef
		for _, assignedTool := range agentTools {
			for _, mcpTool := range allMCPTools {
				// Match by tool name - same method as traditional execution
				var toolName string
				if named, ok := mcpTool.(interface{ Name() string }); ok {
					toolName = named.Name()
				} else if stringer, ok := mcpTool.(interface{ String() string }); ok {
					toolName = stringer.String()
				} else {
					// Fallback: use the type name
					toolName = fmt.Sprintf("%T", mcpTool)
				}
				
				if toolName == assignedTool.ToolName {
					mcpTools = append(mcpTools, mcpTool)
					break
				}
			}
		}
		
		logging.Debug("Dotprompt execution using %d tools (filtered from %d available)", len(mcpTools), len(allMCPTools))
		log.Printf("🔥 MCP-SETUP: MCP tools loaded - %d tools available, %d filtered", len(allMCPTools), len(mcpTools))
		
		// Debug: Log filtered tool schemas to see what's being passed to the LLM
		for i, tool := range mcpTools {
			if namedTool, ok := tool.(interface{ Name() string }); ok {
				log.Printf("🔧 FILTERED-TOOL[%d]: %s - Type: %T", i, namedTool.Name(), tool)
				// Try to inspect the tool's actual structure if possible
				if descTool, ok := tool.(interface{ Description() string }); ok {
					log.Printf("🔧 FILTERED-TOOL[%d]: %s - Description: %s", i, namedTool.Name(), descTool.Description())
				}
			} else {
				log.Printf("🔧 FILTERED-TOOL[%d]: Type: %T", i, tool)
			}
		}
		
		// Add filtered tools count to span
		if span != nil {
			span.SetAttributes(attribute.Int("agent.filtered_tools_count", len(mcpTools)))
		}
		
		// Use our new dotprompt + genkit execution system with progressive logging
		log.Printf("🔥 MCP-SETUP: Creating dotprompt executor")
		
		// Wrap MCP tools with debug logging to capture exact inputs/outputs
		wrappedTools := aee.wrapToolsWithDebugLogging(mcpTools)
		
		executor := dotprompt.NewGenKitExecutor()
		
		// Enhanced execution tracking for tool calls and steps
		executionTracker := &ExecutionTracker{
			runID:         runID,
			toolCalls:     []interface{}{},
			executionSteps: []interface{}{},
			activeTools:   make(map[string]ToolExecution),
			stepCounter:   1,
		}
		
		// Create a logging callback for real-time progress updates and execution tracking
		logCallback := func(logEntry map[string]interface{}) {
			// Filter out application logic leakage before storing - only store user-relevant logs
			if aee.shouldShowInLiveExecution(logEntry) {
				// Store user-relevant logs in database for UI display
				err := aee.repos.AgentRuns.AppendDebugLog(ctx, runID, logEntry)
				if err != nil {
					logging.Debug("Failed to append debug log: %v", err)
				}
			}
			
			// Always process tool execution events for execution tracking (regardless of filtering)
			// ExecutionTracker needs these for database records and UI display
			executionTracker.ProcessLogEntry(logEntry, aee.repos)
		}
		
		// Set the logging callback on the OpenAI plugin for detailed API call logging
		aee.genkitProvider.SetOpenAILogCallback(logCallback)
		
		log.Printf("🔥 AGENT-ENGINE: About to call dotprompt executor - agent: %s", agent.Name)
		
		// Create execution span
		var execSpan trace.Span
		if span != nil {
			ctx, execSpan = aee.telemetryService.StartSpan(ctx, "dotprompt.execute",
				trace.WithAttributes(
					attribute.String("task.preview", func() string {
						if len(task) > 200 { return task[:200] + "..." }
						return task
					}()),
				),
			)
			defer execSpan.End()
		}
		
		// Use the new Station GenKit native integration
		log.Printf("🔥🔥🔥 EXECUTION ENGINE: About to call ExecuteAgentWithStationGenerate for agent %s", agent.Name)
		response, err := executor.ExecuteAgentWithStationGenerate(*agent, agentTools, genkitApp, wrappedTools, task, logCallback)
		log.Printf("🔥 AGENT-ENGINE: Dotprompt executor returned - response: %v, err: %v", response != nil, err)
		
		// Enhance response with execution tracker data
		if response != nil && executionTracker != nil {
			toolCalls, executionSteps := executionTracker.GetExecutionData()
			
			// Replace response data with tracked execution data
			if len(toolCalls) > 0 {
				toolCallsArray := models.JSONArray(toolCalls)
				response.ToolCalls = &toolCallsArray
				response.ToolsUsed = len(toolCalls)
			}
			
			if len(executionSteps) > 0 {
				executionStepsArray := models.JSONArray(executionSteps)
				response.ExecutionSteps = &executionStepsArray
				response.StepsUsed = len(executionSteps)
			}
			
			log.Printf("🔥🔥🔥 EXECUTION ENGINE: Enhanced response with %d tool calls, %d execution steps", 
				len(toolCalls), len(executionSteps))
		}
		
		// Clean up MCP connections after execution is complete
		aee.mcpConnManager.CleanupConnections(aee.activeMCPClients)
		aee.activeMCPClients = nil
		
		if err != nil {
			// Record error in spans
			if execSpan != nil {
				execSpan.RecordError(err)
				execSpan.SetStatus(codes.Error, "dotprompt execution failed")
			}
			if span != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "agent execution failed")
			}
			
			// Log the execution failure for debugging
			if logCallback != nil {
				logCallback(map[string]interface{}{
					"timestamp": time.Now().Format(time.RFC3339),
					"level":     "error",
					"message":   "Agent execution failed",
					"details": map[string]interface{}{
						"error":    err.Error(),
						"duration": time.Since(startTime).String(),
					},
				})
			}
			return nil, fmt.Errorf("dotprompt execution failed: %w", err)
		}
		
		// Add success metrics to spans
		duration := time.Since(startTime)
		if execSpan != nil {
			execSpan.SetAttributes(
				attribute.Bool("execution.success", response.Success),
				attribute.String("execution.model", response.ModelName),
				attribute.Float64("execution.duration_seconds", duration.Seconds()),
				attribute.Int("execution.steps_used", response.StepsUsed),
				attribute.Int("execution.tools_used", response.ToolsUsed),
			)
		}
		if span != nil {
			span.SetAttributes(
				attribute.Bool("execution.success", response.Success),
				attribute.String("execution.model", response.ModelName),
				attribute.Float64("execution.duration_seconds", duration.Seconds()),
				attribute.Int("execution.steps_used", response.StepsUsed),
				attribute.Int("execution.tools_used", response.ToolsUsed),
			)
		}

		// Convert ExecutionResponse to AgentExecutionResult  
		return &AgentExecutionResult{
			Success:        response.Success,
			Response:       response.Response,
			Duration:       duration,
			ModelName:      response.ModelName,
			StepsUsed:      response.StepsUsed,
			StepsTaken:     int64(response.StepsUsed), // Map StepsUsed to StepsTaken for database
			ToolsUsed:      response.ToolsUsed,
			Error:          response.Error,
			TokenUsage:     response.TokenUsage,           // ✅ Pass through token usage from dotprompt
			ToolCalls:      response.ToolCalls,           // ✅ Pass through tool calls
			ExecutionSteps: response.ExecutionSteps,     // ✅ Pass through execution steps
		}, nil
}


// GetGenkitProvider returns the genkit provider for external access
func (aee *AgentExecutionEngine) GetGenkitProvider() *GenKitProvider {
	return aee.genkitProvider
}

// TestStdioMCPConnection tests the MCP connection for debugging
func (aee *AgentExecutionEngine) TestStdioMCPConnection(ctx context.Context) error {
	logging.Info("Testing stdio MCP connection...")

	genkitApp, err := aee.genkitProvider.GetApp(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize Genkit for MCP test: %w", err)
	}
	
	// Update MCP connection manager
	aee.mcpConnManager.genkitApp = genkitApp

	// Test getting tools from default environment (ID: 1)
	tools, clients, err := aee.mcpConnManager.GetEnvironmentMCPTools(ctx, 1)
	if err != nil {
		return fmt.Errorf("failed to get MCP tools: %w", err)
	}
	
	// Cleanup connections
	defer aee.mcpConnManager.CleanupConnections(clients)

	logging.Info("✅ MCP connection test successful - discovered %d tools", len(tools))
	
	for i, tool := range tools {
		if named, ok := tool.(interface{ Name() string }); ok {
			logging.Info("  Tool %d: %s", i+1, named.Name())
		} else {
			logging.Info("  Tool %d: %T (no Name method)", i+1, tool)
		}
	}

	return nil
}

// RenderAgentPromptWithDotprompt renders agent prompt with dotprompt if it contains frontmatter
func (aee *AgentExecutionEngine) RenderAgentPromptWithDotprompt(agentPrompt string, userVariables map[string]interface{}) (string, error) {
	// Check if this is a dotprompt with YAML frontmatter
	if !aee.isDotpromptContent(agentPrompt) {
		// Not a dotprompt, return as-is
		return agentPrompt, nil
	}

	logging.Debug("Agent prompt is dotprompt format, rendering with %d variables", len(userVariables))
	
	// Do inline dotprompt rendering to avoid import cycle
	renderedPrompt, err := aee.renderDotpromptInline(agentPrompt, userVariables)
	if err != nil {
		return "", fmt.Errorf("failed to render dotprompt: %w", err)
	}

	logging.Debug("Dotprompt rendering successful, result length: %d characters", len(renderedPrompt))
	return renderedPrompt, nil
}

// isDotpromptContent checks if the prompt contains dotprompt frontmatter or multi-role syntax
func (aee *AgentExecutionEngine) isDotpromptContent(prompt string) bool {
	trimmed := strings.TrimSpace(prompt)
	
	// Check for YAML frontmatter markers
	hasFrontmatter := strings.HasPrefix(trimmed, "---") && 
		   strings.Contains(prompt, "\n---\n")
		   
	// Check for multi-role dotprompt syntax
	hasMultiRole := strings.Contains(prompt, "{{role \"") || strings.Contains(prompt, "{{role '")
	
	return hasFrontmatter || hasMultiRole
}

// renderDotpromptInline renders dotprompt content inline to avoid import cycles
func (aee *AgentExecutionEngine) renderDotpromptInline(dotpromptContent string, userVariables map[string]interface{}) (string, error) {
	// 1. Create dotprompt instance
	dp := googledotprompt.NewDotprompt(nil) // Use default options
	
	// 2. Prepare data for rendering with user-defined variables only
	data := &googledotprompt.DataArgument{
		Input:   userVariables, // User-defined variables like {{my_folder}}, {{my_var}}
		Context: map[string]any{}, // Keep context empty unless needed
	}
	
	// 3. Render the template  
	rendered, err := dp.Render(dotpromptContent, data, nil)
	if err != nil {
		return "", fmt.Errorf("failed to render dotprompt: %w", err)
	}
	
	// 4. Convert messages to text (extract just the content, no role prefixes)
	var renderedText strings.Builder
	for i, msg := range rendered.Messages {
		if i > 0 {
			renderedText.WriteString("\n\n")
		}
		// Don't include role prefix - just the content
		for _, part := range msg.Content {
			if textPart, ok := part.(*googledotprompt.TextPart); ok {
				renderedText.WriteString(textPart.Text)
			}
		}
	}
	
	return renderedText.String(), nil
}

// AgentSchema represents the input/output schema for an agent
type AgentSchema struct {
	AgentID      int64                  `json:"agent_id"`
	AgentName    string                 `json:"agent_name"`
	HasSchema    bool                   `json:"has_schema"`
	InputSchema  map[string]interface{} `json:"input_schema,omitempty"`
	OutputSchema map[string]interface{} `json:"output_schema,omitempty"`
	Variables    []string               `json:"variables,omitempty"` // Available template variables
}

// GetAgentSchema extracts schema information from agent's dotprompt content using GenKit's parser
func (aee *AgentExecutionEngine) GetAgentSchema(agent *models.Agent) (*AgentSchema, error) {
	schema := &AgentSchema{
		AgentID:   agent.ID,
		AgentName: agent.Name,
		HasSchema: false,
		Variables: []string{},
	}
	
	if !aee.isDotpromptContent(agent.Prompt) {
		// Simple text prompt - no schema
		return schema, nil
	}
	
	// Use GenKit's dotprompt parser to properly parse the template
	parsedPrompt, err := googledotprompt.ParseDocument(agent.Prompt)
	if err != nil {
		return schema, fmt.Errorf("failed to parse dotprompt document: %w", err)
	}
	
	schema.HasSchema = true
	
	// Extract input schema from parsed metadata
	if parsedPrompt.Input.Schema != nil {
		// Schema is of type 'any', so we need to properly handle it
		if schemaMap, ok := parsedPrompt.Input.Schema.(map[string]interface{}); ok {
			schema.InputSchema = schemaMap
			
			// Extract variable names from the input schema
			for varName := range schemaMap {
				schema.Variables = append(schema.Variables, varName)
			}
		} else {
			// Store the raw schema even if it's not a map
			if schemaAny, ok := parsedPrompt.Input.Schema.(interface{}); ok {
				// Try to convert to map[string]interface{} for JSON serialization
				schema.InputSchema = map[string]interface{}{"schema": schemaAny}
			}
		}
	}
	
	// Extract output schema from parsed metadata
	if parsedPrompt.Output.Schema != nil {
		if schemaMap, ok := parsedPrompt.Output.Schema.(map[string]interface{}); ok {
			schema.OutputSchema = schemaMap
		} else {
			// Store the raw schema even if it's not a map
			if schemaAny, ok := parsedPrompt.Output.Schema.(interface{}); ok {
				schema.OutputSchema = map[string]interface{}{"schema": schemaAny}
			}
		}
	}
	
	// Also extract variables from template content as fallback
	if len(schema.Variables) == 0 {
		variables := aee.extractTemplateVariables(agent.Prompt)
		schema.Variables = variables
	}
	
	return schema, nil
}

// extractTemplateVariables finds all {{variable}} patterns in the template content as fallback
func (aee *AgentExecutionEngine) extractTemplateVariables(dotpromptContent string) []string {
	// Extract template content (after frontmatter)
	parts := strings.SplitN(strings.TrimSpace(dotpromptContent), "\n---\n", 2)
	templateContent := parts[len(parts)-1] // Use last part (template content)
	
	// Find all {{variable}} patterns
	var variables []string
	variableMap := make(map[string]bool) // Use map to deduplicate
	
	// Simple regex to find {{variable}} patterns
	start := 0
	for {
		openIndex := strings.Index(templateContent[start:], "{{")
		if openIndex == -1 {
			break
		}
		openIndex += start
		
		closeIndex := strings.Index(templateContent[openIndex:], "}}")
		if closeIndex == -1 {
			break
		}
		closeIndex += openIndex
		
		// Extract variable name
		varContent := strings.TrimSpace(templateContent[openIndex+2 : closeIndex])
		
		// Handle simple variable names (no complex handlebars logic)
		if varContent != "" && !strings.Contains(varContent, " ") && !strings.Contains(varContent, "#") {
			variableMap[varContent] = true
		}
		
		start = closeIndex + 2
	}
	
	// Convert map to slice
	for variable := range variableMap {
		variables = append(variables, variable)
	}
	
	return variables
}

// shouldShowInLiveExecution filters out GenKit framework noise from live execution logs
// while keeping user-relevant information visible
func (aee *AgentExecutionEngine) shouldShowInLiveExecution(logEntry map[string]interface{}) bool {
	message, ok := logEntry["message"].(string)
	if !ok {
		return false
	}
	
	// Framework noise to filter out from live logs
	frameworkNoise := []string{
		"Context usage updated",
		"Turn 1/25 completed",
		"Turn 2/25 completed", 
		"Turn 3/25 completed",
		"Turn 4/25 completed",
		"Turn 5/25 completed",
		"Batch tool execution starting",
		"Batch tool execution completed", 
		"Enhanced generation starting",
		"Enhanced generation completed",
		"Station GenKit generation completed: success",
		"Starting Station-enhanced GenKit generation",
		// Additional GenKit/Station internal noise
		"🔧 STATION-GENERATE: Processing generation request with 4 options",
		"🔧 STATION-MIDDLEWARE: Request has 4 tools",
		"Starting Station-enhanced GenKit generation",
		"Turn 0: Model responded",
		"Turn 1: Model responded",
	}
	
	// Filter out turn completion messages (Turn X/Y completed)
	if strings.Contains(message, "Turn ") && strings.Contains(message, " completed") {
		return false
	}
	
	// Filter out turn messages with patterns
	if strings.Contains(message, "Turn ") && (strings.Contains(message, "Sending request to model") || strings.Contains(message, "Model requested") || strings.Contains(message, "Model responded")) {
		return false
	}
	
	// Filter out debug messages starting with emojis (application logic)
	if strings.HasPrefix(message, "🔧 ") || strings.HasPrefix(message, "🔥 ") || strings.HasPrefix(message, "📊 ") || strings.HasPrefix(message, "⚡ ") {
		return false
	}
	
	// Filter out specific framework noise
	for _, noise := range frameworkNoise {
		if message == noise {
			return false
		}
	}
	
	// Keep user-relevant logs
	userRelevantMessages := []string{
		"Tool execution starting",
		"Tool execution completed", 
		"Starting execution for agent",
		"Starting Station GenKit execution for agent",
		"Station GenKit execution completed successfully",
	}
	
	for _, relevant := range userRelevantMessages {
		if message == relevant {
			return true
		}
	}
	
	// Default: show unknown messages (be conservative)
	return true
}

// isToolExecutionEvent checks if a log entry is a tool execution event
// that the ExecutionTracker needs to process
func (aee *AgentExecutionEngine) isToolExecutionEvent(logEntry map[string]interface{}) bool {
	message, ok := logEntry["message"].(string)
	if !ok {
		return false
	}
	
	toolMessages := []string{
		"Tool execution starting",
		"Tool execution completed",
		"Enhanced generation completed",
	}
	
	for _, toolMsg := range toolMessages {
		if message == toolMsg {
			return true
		}
	}
	
	return false
}

// wrapToolsWithDebugLogging wraps MCP tools to capture exact inputs and outputs
func (aee *AgentExecutionEngine) wrapToolsWithDebugLogging(tools []ai.ToolRef) []ai.ToolRef {
	wrappedTools := make([]ai.ToolRef, len(tools))
	
	for i, tool := range tools {
		wrappedTools[i] = aee.createToolWrapper(tool)
	}
	
	log.Printf("🔧 TOOL-WRAPPER: Wrapped %d tools with debug logging", len(wrappedTools))
	return wrappedTools
}

// createToolWrapper creates a debug wrapper for a single tool
func (aee *AgentExecutionEngine) createToolWrapper(originalTool ai.ToolRef) ai.ToolRef {
	// For now, we'll return the original tool but log that we would wrap it
	// Full wrapping requires implementing the ToolRef interface which is complex
	if namedTool, ok := originalTool.(interface{ Name() string }); ok {
		log.Printf("🔧 TOOL-WRAPPER: Would wrap tool: %s (type: %T)", namedTool.Name(), originalTool)
	}
	
	// Return original tool for now - actual wrapping would require significant work
	return originalTool
}
