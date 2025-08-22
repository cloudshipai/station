package dotprompt

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	
	"station/internal/config"
	"station/internal/logging"
	"station/pkg/models"
	"station/pkg/schema"
	
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/google/dotprompt/go/dotprompt"
)

// ToolCallTracker monitors tool usage to prevent obsessive calling loops
type ToolCallTracker struct {
	TotalCalls          int
	ConsecutiveSameTool map[string]int
	LastToolUsed        string
	MaxToolCalls        int
	MaxConsecutive      int
	LogCallback         func(map[string]interface{})
}

// GenKitExecutor handles dotprompt-based agent execution using GenKit Generate
type GenKitExecutor struct{
	logCallback func(map[string]interface{})
}

// NewGenKitExecutor creates a new GenKit-based dotprompt executor
func NewGenKitExecutor() *GenKitExecutor {
	return &GenKitExecutor{}
}

// ExecuteAgentWithDotpromptTemplate is deprecated - use ExecuteAgentWithDatabaseConfig instead
// This method is kept for backward compatibility with existing tests only
func (e *GenKitExecutor) ExecuteAgentWithDotpromptTemplate(extractor *RuntimeExtraction, request *ExecutionRequest) (*ExecutionResponse, error) {
	// Redirect to the real implementation
	return nil, fmt.Errorf("ExecuteAgentWithDotpromptTemplate is deprecated - use ExecuteAgentWithDatabaseConfig instead")
}

// ExecuteAgentWithDotprompt executes an agent using hybrid approach: dotprompt direct + GenKit Generate


func (e *GenKitExecutor) ExecuteAgentWithDotprompt(agent models.Agent, agentTools []*models.AgentToolWithDetails, genkitApp *genkit.Genkit, mcpTools []ai.ToolRef, task string) (*ExecutionResponse, error) {
	startTime := time.Now()
	
	// Add panic recovery to catch any silent panics
	defer func() {
		if r := recover(); r != nil {
			log.Printf("🚨 PANIC RECOVERED in ExecuteAgentWithDotprompt: %v", r)
		}
	}()
	
	logging.Debug("Starting unified dotprompt execution for agent %s", agent.Name)
	log.Printf("🔥 DEBUG-FLOW: Function entry - agent ID %d, name %s", agent.ID, agent.Name)
	log.Printf("🔥 DEBUG-FLOW: Input parameters - agentTools: %d, mcpTools: %d, task length: %d", len(agentTools), len(mcpTools), len(task))
	
	// 1. Use agent prompt directly if it contains multi-role syntax
	var dotpromptContent string
	log.Printf("🔥 DEBUG-FLOW: Step 1 - Checking agent prompt format")
	if e.isDotpromptContent(agent.Prompt) {
		// Agent already has dotprompt format (either frontmatter or multi-role)
		dotpromptContent = agent.Prompt
		logging.Debug("Using agent prompt directly, length: %d", len(dotpromptContent))
		log.Printf("🔥 DEBUG-FLOW: Step 1 - Using agent prompt directly")
	} else {
		// Legacy agent, build frontmatter
		log.Printf("🔥 DEBUG-FLOW: Step 1 - Building dotprompt from legacy agent")
		dotpromptContent = e.buildDotpromptFromAgent(agent, agentTools, "default")
		logging.Debug("Built dotprompt content, length: %d", len(dotpromptContent))
		log.Printf("🔥 DEBUG-FLOW: Step 1 - Built dotprompt content successfully")
	}
	
	// 2. Use dotprompt library directly for multi-role rendering (bypasses GenKit constraint)
	logging.Debug("DEBUG-FLOW: Step 2 - Creating dotprompt instance")
	dp := dotprompt.NewDotprompt(nil)
	logging.Debug("DEBUG-FLOW: Step 2 - Compiling dotprompt content (length: %d)", len(dotpromptContent))
	promptFunc, err := dp.Compile(dotpromptContent, nil)
	if err != nil {
		logging.Debug("DEBUG-FLOW: Step 2 - FAILED to compile dotprompt: %v", err)
		return &ExecutionResponse{
			Success:   false,
			Response:  "",
			Duration:  time.Since(startTime),
			Error:     fmt.Sprintf("failed to compile dotprompt: %v", err),
		}, nil
	}
	
	logging.Debug("Dotprompt compiled successfully")
	logging.Debug("DEBUG-FLOW: Step 2 - Dotprompt compiled successfully")
	
	// 3. Render the prompt with merged input data (default + custom schema)
	logging.Debug("DEBUG-FLOW: Step 3 - Creating schema helper")
	schemaHelper := schema.NewExportHelper()
	
	// For now, only use userInput. Custom input data can be added via call_agent variables parameter
	logging.Debug("DEBUG-FLOW: Step 3 - Getting merged input data")
	inputData, err := schemaHelper.GetMergedInputData(&agent, task, nil)
	if err != nil {
		logging.Debug("Schema helper failed: %v, using basic userInput", err)
		logging.Debug("DEBUG-FLOW: Step 3 - Schema helper failed, using fallback")
		// Fallback to basic userInput on schema error
		inputData = map[string]interface{}{
			"userInput": task,
		}
	}
	logging.Debug("Input data prepared with %d fields", len(inputData))
	logging.Debug("DEBUG-FLOW: Step 3 - Input data prepared successfully")
	
	data := &dotprompt.DataArgument{
		Input: inputData,
	}
	
	logging.Debug("DEBUG-FLOW: Step 3 - Rendering prompt with data")
	renderedPrompt, err := promptFunc(data, nil)
	if err != nil {
		logging.Debug("DEBUG-FLOW: Step 3 - FAILED to render dotprompt: %v", err)
		return &ExecutionResponse{
			Success:   false,
			Response:  "",
			Duration:  time.Since(startTime),
			Error:     fmt.Sprintf("failed to render dotprompt: %v", err),
		}, nil
	}
	
	logging.Debug("Rendered %d messages from dotprompt", len(renderedPrompt.Messages))
	logging.Debug("DEBUG-FLOW: Step 3 - Rendered prompt successfully with %d messages", len(renderedPrompt.Messages))
	
	// 4. Convert dotprompt messages to GenKit messages
	logging.Debug("DEBUG-FLOW: Step 4 - Converting dotprompt messages to GenKit format")
	genkitMessages, err := e.convertDotpromptToGenkitMessages(renderedPrompt.Messages)
	if err != nil {
		logging.Debug("DEBUG-FLOW: Step 4 - FAILED to convert messages: %v", err)
		return &ExecutionResponse{
			Success:   false,
			Response:  "",
			Duration:  time.Since(startTime),
			Error:     fmt.Sprintf("failed to convert messages: %v", err),
		}, nil
	}
	logging.Debug("Converted to %d GenKit messages", len(genkitMessages))
	logging.Debug("DEBUG-FLOW: Step 4 - Message conversion successful")
	
	// 5. Get model name with proper provider prefix (same logic as agent_execution_engine.go)
	logging.Debug("DEBUG-FLOW: Step 5 - Getting model configuration")
	baseModelName := renderedPrompt.Model
	if baseModelName == "" {
		baseModelName = "gemini-1.5-flash" // fallback
	}
	logging.Debug("DEBUG-FLOW: Step 5 - Base model: %s", baseModelName)
	
	// Load configuration 
	logging.Debug("DEBUG-FLOW: Step 5 - Loading config")
	cfg, err := config.Load()
	if err != nil {
		logging.Debug("Failed to load config: %v, using defaults", err)
		logging.Debug("DEBUG-FLOW: Step 5 - Config load failed, using defaults")
		cfg = &config.Config{
			AIProvider: "openai",
			AIModel:    baseModelName,
		}
	}
	
	// Override model from config if available
	if cfg.AIModel != "" {
		baseModelName = cfg.AIModel
	}
	logging.Debug("DEBUG-FLOW: Step 5 - Config loaded - provider: %s, model: %s", cfg.AIProvider, cfg.AIModel)
	
	// Use the same provider resolution logic as agent_execution_engine.go:237-256
	var modelName string
	switch strings.ToLower(cfg.AIProvider) {
	case "gemini", "googlegenai":
		modelName = fmt.Sprintf("googleai/%s", baseModelName)
	case "openai":
		modelName = fmt.Sprintf("openai/%s", baseModelName)
	default:
		modelName = fmt.Sprintf("%s/%s", cfg.AIProvider, baseModelName)
	}
	
	logging.Debug("Using model %s (provider: %s, config: %s)", modelName, cfg.AIProvider, baseModelName)
	logging.Debug("DEBUG-FLOW: Step 5 - Final model name: %s", modelName)
	
	// 6. Extract tool names for GenKit (merge frontmatter tools + MCP tools)
	logging.Debug("DEBUG-FLOW: Step 6 - Extracting tool names from frontmatter")
	var toolNames []string
	for _, tool := range renderedPrompt.Tools {
		toolNames = append(toolNames, tool)
	}
	logging.Debug("DEBUG-FLOW: Step 6 - Found %d frontmatter tools", len(toolNames))
	
	logging.Debug("Using model %s with %d messages and %d MCP tools", modelName, len(genkitMessages), len(mcpTools))
	logging.Debug("DEBUG-FLOW: Step 6 - Ready for execution setup")
	
	// 7. Execute with GenKit's Generate for full multi-turn and tool support
	// Add timeout to prevent infinite hanging - 10 minutes for complex analysis tasks
	logging.Debug("DEBUG-FLOW: Step 7 - Creating execution context with 10min timeout")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	
	// Build generate options (match traditional approach exactly)
	logging.Debug("DEBUG-FLOW: Step 7 - Building generate options")
	var generateOpts []ai.GenerateOption
	generateOpts = append(generateOpts, ai.WithModelName(modelName))  // Use same as traditional
	generateOpts = append(generateOpts, ai.WithMessages(genkitMessages...))
	maxToolCalls := 25  // Allow 25 tool calls, then force final response
	// Set GenKit maxTurns higher than our custom limit to let our logic handle it
	generateOpts = append(generateOpts, ai.WithMaxTurns(30)) // Higher than our 25 to prevent GenKit interference
	logging.Debug("DEBUG-FLOW: Step 7 - Basic options set (model: %s, messages: %d, maxTurns: 30)", modelName, len(genkitMessages))
	
	// Add tool call limits to prevent obsessive tool calling
	maxToolCallsPerConversation := 15
	maxConsecutiveSameTool := 3
	
	// Add MCP tools if available (same as traditional)
	generateOpts = append(generateOpts, ai.WithTools(mcpTools...))
	
	// Check if we're approaching tool call limits and log warning
	messageCount := len(genkitMessages)
	toolCallsRemaining := maxToolCalls - messageCount  // Approximation based on message count
	
	if e.logCallback != nil && toolCallsRemaining <= 5 {
		e.logCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "warning",
			"message":   fmt.Sprintf("APPROACHING TOOL CALL LIMIT: %d messages, approximately %d tool calls remaining (max: %d)", 
				messageCount, toolCallsRemaining, maxToolCalls),
			"details": map[string]interface{}{
				"current_messages": messageCount,
				"max_tool_calls": maxToolCalls,
				"calls_remaining": toolCallsRemaining,
				"risk_level": func() string {
					if toolCallsRemaining <= 2 { return "CRITICAL" }
					if toolCallsRemaining <= 5 { return "HIGH" }
					return "MEDIUM"
				}(),
			},
		})
	}
	
	// Add logging before model execution with specific tool names for clarity
	if e.logCallback != nil {
		toolNames := make([]string, 0, len(mcpTools))
		for _, tool := range mcpTools {
			if namedTool, ok := tool.(interface{ Name() string }); ok {
				toolNames = append(toolNames, namedTool.Name())
			}
		}
		if len(toolNames) > 5 {
			toolNames = append(toolNames[:5], fmt.Sprintf("...and %d more", len(toolNames)-5))
		}
		
		e.logCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "info",
			"message":   fmt.Sprintf("Agent '%s' starting conversation with %d tools available", agent.Name, len(mcpTools)),
			"details": map[string]interface{}{
				"model":          modelName,
				"available_tools": toolNames,
				"max_turns":      40,
				"conversation_length": len(genkitMessages),
				"task_preview":   func() string { if len(task) > 80 { return task[:80] + "..." }; return task }(),
			},
		})
	}
	
	// Add detailed logging around the GenKit Generate call
	generateStartTime := time.Now()
	if e.logCallback != nil {
		e.logCallback(map[string]interface{}{
			"timestamp": generateStartTime.Format(time.RFC3339),
			"level":     "debug",
			"message":   "Starting AI model conversation",
			"details": map[string]interface{}{
				"context_timeout": "10_minutes",
				"genkit_app":      fmt.Sprintf("%T", genkitApp),
				"options_count":   len(generateOpts),
			},
		})
	}
	
	// Create tool call tracker to prevent obsessive loops
	logging.Debug("DEBUG-FLOW: Step 7 - Creating tool call tracker")
	toolCallTracker := &ToolCallTracker{
		TotalCalls:          0,
		ConsecutiveSameTool: make(map[string]int),
		LastToolUsed:        "",
		MaxToolCalls:        maxToolCallsPerConversation,
		MaxConsecutive:      maxConsecutiveSameTool,
		LogCallback:         e.logCallback,
	}
	logging.Debug("DEBUG-FLOW: Step 7 - Tool call tracker created (maxCalls: %d, maxConsecutive: %d)", maxToolCallsPerConversation, maxConsecutiveSameTool)
	
	// Use custom generate with proper turn limiting and final response capability  
	logging.Debug("DEBUG-FLOW: Step 7 - CALLING generateWithCustomTurnLimit - THIS IS THE CRITICAL CALL")
	logging.Debug("DEBUG-FLOW: Step 7 - Parameters: ctx=%v, genkitApp=%v, optsCount=%d, maxToolCalls=%d, model=%s", 
		ctx != nil, genkitApp != nil, len(generateOpts), maxToolCalls, modelName)
	response, err := e.generateWithCustomTurnLimit(ctx, genkitApp, generateOpts, toolCallTracker, maxToolCalls, modelName)
	logging.Debug("DEBUG-FLOW: Step 7 - generateWithCustomTurnLimit RETURNED - response=%v, error=%v", response != nil, err)
	generateDuration := time.Since(generateStartTime)
	
	// Log immediately after Generate call (success or failure)
	if e.logCallback != nil {
		if err != nil {
			// Analyze error type for better debugging
			errorMessage := err.Error()
			var failureType string
			var actionable_solution string
			
			if strings.Contains(errorMessage, "context") && strings.Contains(errorMessage, "deadline") {
				failureType = "TIMEOUT"
				actionable_solution = "API call timed out - check network or reduce context size"
			} else if strings.Contains(errorMessage, "token") || strings.Contains(errorMessage, "length") {
				failureType = "CONTEXT_LIMIT"
				actionable_solution = "Hit context window limit - reduce conversation history or message size"
			} else if strings.Contains(errorMessage, "turn") || strings.Contains(errorMessage, "max") {
				failureType = "TURN_LIMIT"
				actionable_solution = "Hit maximum turn limit - conversation exceeded 25 exchanges"
			} else if strings.Contains(errorMessage, "rate") || strings.Contains(errorMessage, "quota") {
				failureType = "RATE_LIMIT"
				actionable_solution = "API rate limit reached - wait before retrying"
			} else {
				failureType = "UNKNOWN_ERROR"
				actionable_solution = "Check error details and API connectivity"
			}
			
			e.logCallback(map[string]interface{}{
				"timestamp": time.Now().Format(time.RFC3339),
				"level":     "error",
				"message":   fmt.Sprintf("AI conversation FAILED (%s) after %v", failureType, generateDuration),
				"details": map[string]interface{}{
					"failure_type":       failureType,
					"error_message":      errorMessage,
					"duration":           generateDuration.String(),
					"model":              modelName,
					"messages_in_conversation": messageCount,
					"actionable_solution": actionable_solution,
				},
			})
		} else {
			e.logCallback(map[string]interface{}{
				"timestamp": time.Now().Format(time.RFC3339),
				"level":     "info",
				"message":   "AI model conversation completed",
				"details": map[string]interface{}{
					"duration_seconds": generateDuration.Seconds(),
					"response_nil":     response == nil,
				},
			})
		}
	}
	if err != nil {
		// Log the error
		if e.logCallback != nil {
			e.logCallback(map[string]interface{}{
				"timestamp": time.Now().Format(time.RFC3339),
				"level":     "error",
				"message":   "AI model execution failed",
				"details": map[string]interface{}{
					"error": err.Error(),
					"model": modelName,
				},
			})
		}
		
		return &ExecutionResponse{
			Success:   false,
			Response:  "",
			Duration:  time.Since(startTime),
			Error:     fmt.Sprintf("Generate failed: %v", err),
		}, nil
	}
	
	if response == nil {
		return &ExecutionResponse{
			Success:   false,
			Response:  "",
			Duration:  time.Since(startTime),
			Error:     "Generate returned nil response",
		}, nil
	}
	
	responseText := response.Text()
	
	// Log model response completion
	if e.logCallback != nil {
		e.logCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "info",
			"message":   "AI model response received",
			"details": map[string]interface{}{
				"response_length": len(responseText),
				"has_request":     response.Request != nil,
				"has_messages":    response.Request != nil && response.Request.Messages != nil,
			},
		})
	}
	
	// Extract tool calls and execution steps with detailed debugging
	var allToolCalls []interface{}
	var executionSteps []interface{}
	stepCounter := 1
	
	logging.Debug("=== TOOL CALLS & EXECUTION STEPS EXTRACTION ===")
	
	// Check if any tool calls were made in the response
	if response.Request != nil && response.Request.Messages != nil {
		logging.Debug("Processing %d messages from response.Request", len(response.Request.Messages))
		
		for msgIdx, msg := range response.Request.Messages {
			logging.Debug("  Message[%d]: Role=%s, ContentParts=%d", msgIdx, msg.Role, len(msg.Content))
			
			// Extract tool requests, responses, and model thoughts
			var modelThoughts []string
			var toolRequestsInMessage []map[string]interface{}
			
			for partIdx, part := range msg.Content {
				logging.Debug("    Part[%d]: IsToolRequest=%t, IsText=%t, IsToolResponse=%t", 
					partIdx, part.IsToolRequest(), part.IsText(), part.IsToolResponse())
				
				if part.IsToolRequest() && part.ToolRequest != nil {
					logging.Debug("      ToolRequest: Name=%s, Ref=%s", part.ToolRequest.Name, part.ToolRequest.Ref)
					logging.Debug("      ToolRequest Input: %+v", part.ToolRequest.Input)
					
					// Add tool call to array
					toolCall := map[string]interface{}{
						"step":           stepCounter,
						"tool_name":      part.ToolRequest.Name,
						"tool_input":     part.ToolRequest.Input,
						"tool_call_id":   part.ToolRequest.Ref,
						"message_role":   string(msg.Role),
					}
					allToolCalls = append(allToolCalls, toolCall)
					toolRequestsInMessage = append(toolRequestsInMessage, toolCall)
					
					// Log tool call in real-time
					if e.logCallback != nil {
						e.logCallback(map[string]interface{}{
							"timestamp": time.Now().Format(time.RFC3339),
							"level":     "info",
							"message":   "Tool executed",
							"details": map[string]interface{}{
								"tool_name":    part.ToolRequest.Name,
								"step":         stepCounter,
								"tool_call_id": part.ToolRequest.Ref,
								"input":        part.ToolRequest.Input,
							},
						})
					}
					
					// Add execution step
					executionStep := map[string]interface{}{
						"step":      stepCounter,
						"type":      "tool_call",
						"tool_name": part.ToolRequest.Name,
						"input":     part.ToolRequest.Input,
						"timestamp": time.Now().Format(time.RFC3339),
					}
					executionSteps = append(executionSteps, executionStep)
					stepCounter++
					
				} else if part.IsText() && part.Text != "" {
					logging.Debug("      Text content: %q", part.Text)
					// Capture model's intermediate thoughts/reasoning
					modelThoughts = append(modelThoughts, part.Text)
					
				} else if part.IsToolResponse() && part.ToolResponse != nil {
					logging.Debug("      ToolResponse: Name=%s", part.ToolResponse.Name)
					logging.Debug("      ToolResponse Output: %+v", part.ToolResponse.Output)
					
					// Add tool response as execution step
					executionStep := map[string]interface{}{
						"step":      stepCounter,
						"type":      "tool_response", 
						"tool_name": part.ToolResponse.Name,
						"output":    part.ToolResponse.Output,
						"timestamp": time.Now().Format(time.RFC3339),
					}
					executionSteps = append(executionSteps, executionStep)
					
					// Log tool response in real-time
					if e.logCallback != nil {
						e.logCallback(map[string]interface{}{
							"timestamp": time.Now().Format(time.RFC3339),
							"level":     "info",
							"message":   "Tool response received",
							"details": map[string]interface{}{
								"tool_name": part.ToolResponse.Name,
								"step":      stepCounter,
								"output_length": len(fmt.Sprintf("%v", part.ToolResponse.Output)),
							},
						})
					}
					
					stepCounter++
				}
			}
			
			// If there were model thoughts alongside tool requests, add them to tool calls
			if len(modelThoughts) > 0 && len(toolRequestsInMessage) > 0 {
				logging.Debug("    Adding model thoughts to %d tool requests", len(toolRequestsInMessage))
				for j := range toolRequestsInMessage {
					if j < len(allToolCalls) {
						allToolCalls[len(allToolCalls)-len(toolRequestsInMessage)+j].(map[string]interface{})["model_thoughts"] = strings.Join(modelThoughts, " ")
					}
				}
			} else if len(modelThoughts) > 0 {
				logging.Debug("    Found model thoughts without tool requests: %d thoughts", len(modelThoughts))
				// Add model thoughts as execution step even if no tool calls
				executionStep := map[string]interface{}{
					"step":      stepCounter,
					"type":      "model_reasoning",
					"content":   strings.Join(modelThoughts, " "),
					"timestamp": time.Now().Format(time.RFC3339),
				}
				executionSteps = append(executionSteps, executionStep)
				stepCounter++
			}
		}
	} else {
		logging.Debug("No messages found in response.Request")
	}
	
	// Note: Some AI providers put tool calls in different response fields,
	// but this GenKit version uses response.Request.Messages
	
	logging.Debug("EXTRACTION SUMMARY: %d tool calls, %d execution steps", len(allToolCalls), len(executionSteps))
	
	// Log detailed summary of extracted data
	if len(allToolCalls) > 0 {
		logging.Debug("Tool calls extracted:")
		for i, toolCall := range allToolCalls {
			if tc, ok := toolCall.(map[string]interface{}); ok {
				logging.Debug("  [%d] %s (step %v)", i, tc["tool_name"], tc["step"])
			}
		}
	}
	
	if len(executionSteps) > 0 {
		logging.Debug("Execution steps extracted:")
		for i, step := range executionSteps {
			if s, ok := step.(map[string]interface{}); ok {
				logging.Debug("  [%d] %s (step %v)", i, s["type"], s["step"])
			}
		}
	}
	
	// Convert to JSONArray format
	var toolCallsArray *models.JSONArray
	var executionStepsArray *models.JSONArray
	
	if len(allToolCalls) > 0 {
		jsonArray := models.JSONArray(allToolCalls)
		toolCallsArray = &jsonArray
	}
	
	if len(executionSteps) > 0 {
		jsonArray := models.JSONArray(executionSteps)
		executionStepsArray = &jsonArray
	}
	
	// Debug: Dissect the GenKit response object to understand its structure
	tokenUsage := make(map[string]interface{})
	if response != nil {
		logging.Debug("GenKit Response Object Structure Analysis:")
		logging.Debug("  response != nil: true")
		logging.Debug("  response.Text(): %q", response.Text())
		
		// Check Usage field
		if response.Usage != nil {
			logging.Debug("  response.Usage != nil: true")
			logging.Debug("  response.Usage.InputTokens: %d", response.Usage.InputTokens)
			logging.Debug("  response.Usage.OutputTokens: %d", response.Usage.OutputTokens)
			tokenUsage["input_tokens"] = response.Usage.InputTokens
			tokenUsage["output_tokens"] = response.Usage.OutputTokens
			tokenUsage["total_tokens"] = response.Usage.InputTokens + response.Usage.OutputTokens
		} else {
			logging.Debug("  response.Usage: nil")
		}
		
		// Check Request field (sometimes token usage is here)
		if response.Request != nil {
			logging.Debug("  response.Request != nil: true")
			if response.Request.Messages != nil {
				logging.Debug("  response.Request.Messages: %d messages", len(response.Request.Messages))
			}
		} else {
			logging.Debug("  response.Request: nil")
		}
		
		// Check if there are other response fields we should examine
		// (Candidates field not available in this GenKit version)
		
		// Try to access other potential fields using reflection-like approach
		logging.Debug("  Full response type: %T", response)
		
		// Log token usage summary
		if len(tokenUsage) > 0 {
			logging.Debug("  Extracted token usage: %+v", tokenUsage)
		} else {
			logging.Debug("  No token usage extracted")
		}
	} else {
		logging.Debug("GenKit response is nil")
	}

	return &ExecutionResponse{
		Success:        true,
		Response:       responseText, // Use the responseText variable we created
		ToolCalls:      toolCallsArray,
		ExecutionSteps: executionStepsArray,
		Duration:       time.Since(startTime),
		ModelName:      modelName,
		StepsUsed:      len(executionSteps), // Actual number of execution steps
		ToolsUsed:      len(allToolCalls),   // Actual number of tools used
		TokenUsage:     tokenUsage,          // Add extracted token usage
		Error:          "",
	}, nil
}

// ExecuteAgentWithDotpromptAndLogging executes an agent with progressive logging callbacks
func (e *GenKitExecutor) ExecuteAgentWithDotpromptAndLogging(agent models.Agent, agentTools []*models.AgentToolWithDetails, genkitApp *genkit.Genkit, mcpTools []ai.ToolRef, task string, logCallback func(map[string]interface{})) (*ExecutionResponse, error) {
	// Store the callback for use during execution
	e.logCallback = logCallback
	
	// Add detailed logging at key execution points
	if logCallback != nil {
		toolNames := make([]string, 0, len(mcpTools))
		for _, tool := range mcpTools {
			if namedTool, ok := tool.(interface{ Name() string }); ok {
				toolNames = append(toolNames, namedTool.Name())
			}
		}
		if len(toolNames) > 3 {
			toolNames = append(toolNames[:3], fmt.Sprintf("...%d more", len(toolNames)-3))
		}
		
		logCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "info", 
			"message":   fmt.Sprintf("Initializing agent '%s' with system prompt (%d chars) and %s", 
				agent.Name, len(agent.Prompt), func() string {
					if len(toolNames) == 0 {
						return "no tools"
					}
					return fmt.Sprintf("%d tools: [%s]", len(toolNames), strings.Join(toolNames, ", "))
				}()),
			"details": map[string]interface{}{
				"agent_id":     agent.ID,
				"agent_name":   agent.Name,
				"system_prompt_length": len(agent.Prompt),
				"tool_names":   toolNames,
				"task_preview": func() string { if len(task) > 60 { return task[:60] + "..." }; return task }(),
			},
		})
	}
	
	// Execute the normal dotprompt method
	log.Printf("🔥 ANDLOGGING: About to call ExecuteAgentWithDotprompt")
	result, err := e.ExecuteAgentWithDotprompt(agent, agentTools, genkitApp, mcpTools, task)
	log.Printf("🔥 ANDLOGGING: ExecuteAgentWithDotprompt returned - result=%v, err=%v", result != nil, err)
	
	if err != nil {
		if logCallback != nil {
			logCallback(map[string]interface{}{
				"timestamp": time.Now().Format(time.RFC3339),
				"level":     "error",
				"message":   "Dotprompt execution failed",
				"details": map[string]interface{}{
					"error": err.Error(),
				},
			})
		}
		return result, err
	}
	
	// Log successful completion with details
	if logCallback != nil {
		logCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "info",
			"message":   "Agent execution completed",
			"details": map[string]interface{}{
				"success": result.Success,
				"duration_seconds": result.Duration.Seconds(),
				"response_length": len(result.Response),
				"tools_used": result.ToolsUsed,
				"steps_taken": result.StepsUsed,
			},
		})
	}
	
	return result, nil
}

// ExecuteAgentWithDatabaseConfig executes an agent using the dotprompt + genkit approach
func (e *GenKitExecutor) ExecuteAgentWithDatabaseConfig(agent models.Agent, agentTools []*models.AgentToolWithDetails, genkitApp *genkit.Genkit, mcpTools []ai.ToolRef, task string) (*ExecutionResponse, error) {
	// Execute using the dotprompt + genkit approach
	result, err := e.ExecuteAgentWithDotprompt(agent, agentTools, genkitApp, mcpTools, task)
	if err != nil {
		return &ExecutionResponse{
			Success:  false,
			Response: "",
			Duration: 0,
			Error:    fmt.Sprintf("❌ Dotprompt + GenKit execution failed: %v\n\nThis agent requires the new dotprompt + genkit execution system. Please check that:\n- Agent configuration is valid\n- GenKit provider is properly initialized\n- All required tools are available", err),
		}, nil
	}
	
	// Success! Return clean response without execution engine prefix
	return result, nil
}

// ExecuteAgentWithDatabaseConfigAndLogging executes an agent with progressive logging callback
func (e *GenKitExecutor) ExecuteAgentWithDatabaseConfigAndLogging(agent models.Agent, agentTools []*models.AgentToolWithDetails, genkitApp *genkit.Genkit, mcpTools []ai.ToolRef, task string, logCallback func(map[string]interface{})) (*ExecutionResponse, error) {
	log.Printf("🔥 CONFIG-AND-LOGGING: Function entry - agent %s", agent.Name)
	// Store the logging callback for use during execution
	e.logCallback = logCallback
	
	// Add logging for LLM execution start
	if logCallback != nil {
		toolNames := make([]string, 0, len(mcpTools))
		for _, tool := range mcpTools {
			if namedTool, ok := tool.(interface{ Name() string }); ok {
				toolNames = append(toolNames, namedTool.Name())
			}
		}
		if len(toolNames) > 4 {
			toolNames = append(toolNames[:4], "...")
		}
		
		logCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "info",
			"message":   fmt.Sprintf("Agent '%s' ready to execute task with %d tools: %s", 
				agent.Name, len(mcpTools), strings.Join(toolNames, ", ")),
			"details": map[string]interface{}{
				"agent_id":      agent.ID,
				"agent_name":    agent.Name,
				"task_length":   len(task),
				"tool_names":    toolNames,
				"execution_mode": "dotprompt_genkit",
			},
		})
	}
	
	// Execute using the dotprompt + genkit approach with logging
	log.Printf("🔥 WRAPPER: About to call ExecuteAgentWithDotpromptAndLogging")
	result, err := e.ExecuteAgentWithDotpromptAndLogging(agent, agentTools, genkitApp, mcpTools, task, logCallback)
	log.Printf("🔥 WRAPPER: ExecuteAgentWithDotpromptAndLogging returned - result=%v, err=%v", result != nil, err)
	if err != nil {
		// Log the error
		if logCallback != nil {
			logCallback(map[string]interface{}{
				"timestamp": time.Now().Format(time.RFC3339),
				"level":     "error",
				"message":   "Dotprompt execution failed",
				"details": map[string]interface{}{
					"error": err.Error(),
				},
			})
		}
		
		return &ExecutionResponse{
			Success:  false,
			Response: "",
			Duration: 0,
			Error:    fmt.Sprintf("❌ Dotprompt + GenKit execution failed: %v\n\nThis agent requires the new dotprompt + genkit execution system. Please check that:\n- Agent configuration is valid\n- GenKit provider is properly initialized\n- All required tools are available", err),
		}, nil
	}
	
	// Log successful completion
	if logCallback != nil {
		logCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "info",
			"message":   "Agent execution completed successfully",
			"details": map[string]interface{}{
				"success":      result.Success,
				"duration":     result.Duration,
				"response_len": len(result.Response),
			},
		})
	}
	
	// Success! Return clean response
	return result, nil
}

// extractToolNames extracts tool names from agent tools for display
func (e *GenKitExecutor) extractToolNames(agentTools []*models.AgentToolWithDetails) []string {
	var toolNames []string
	for _, tool := range agentTools {
		// AgentToolWithDetails has ToolName field from the join
		toolNames = append(toolNames, tool.ToolName)
	}
	return toolNames
}

// isDotpromptContent checks if the prompt contains dotprompt frontmatter or multi-role syntax
func (e *GenKitExecutor) isDotpromptContent(prompt string) bool {
	trimmed := strings.TrimSpace(prompt)
	
	// Check for YAML frontmatter markers
	hasFrontmatter := strings.HasPrefix(trimmed, "---") && 
		   strings.Contains(prompt, "\n---\n")
		   
	// Check for multi-role dotprompt syntax
	hasMultiRole := strings.Contains(prompt, "{{role \"") || strings.Contains(prompt, "{{role '")
	
	return hasFrontmatter || hasMultiRole
}

// getPromptSource returns a description of the prompt source type
func (e *GenKitExecutor) getPromptSource(prompt string) string {
	if e.isDotpromptContent(prompt) {
		return "dotprompt (frontmatter + template)"
	}
	return "simple text prompt"
}

// RenderDotpromptContent renders a dotprompt template with the given variables using the new multi-role system
func (e *GenKitExecutor) RenderDotpromptContent(dotpromptContent, task, agentName string) (string, error) {
	// 1. Create renderer
	renderer := NewRenderer()
	
	// 2. Prepare render context with automatic variables
	context := &RenderContext{
		UserInput:   task,
		AgentName:   agentName,
		Environment: "default", // TODO: get from agent config
		UserVariables: make(map[string]interface{}),
	}
	
	// 3. Render the template using our multi-role system
	parsed, err := renderer.Render(dotpromptContent, context)
	if err != nil {
		return "", fmt.Errorf("failed to render dotprompt: %w", err)
	}
	
	// 4. Convert to Genkit-compatible format
	renderedText, err := renderer.RenderToGenkit(parsed)
	if err != nil {
		return "", fmt.Errorf("failed to convert to Genkit format: %w", err)
	}
	
	return renderedText, nil
}

// getActiveModelFromConfig gets the model from Station config (without dotprompt fallback)
func (e *GenKitExecutor) getActiveModelFromConfig() string {
	// Try to load Station config
	stationConfig, err := config.Load()
	if err == nil && stationConfig.AIModel != "" {
		return stationConfig.AIModel
	}
	
	// Fallback if Station config not available
	return "gemini-1.5-flash"
}

// ExecuteAgentWithGenerate provides OpenAI compatibility - deprecated in favor of ExecuteAgentWithDatabaseConfig
// This method is kept for backward compatibility with existing tests only
func (e *GenKitExecutor) ExecuteAgentWithGenerate(extractor *RuntimeExtraction, request *ExecutionRequest) (*ExecutionResponse, error) {
	return nil, fmt.Errorf("ExecuteAgentWithGenerate is deprecated - use ExecuteAgentWithDatabaseConfig for real execution")
}

// renderTemplate performs basic template variable substitution
func (e *GenKitExecutor) renderTemplate(template string, variables map[string]interface{}) (string, error) {
	rendered := template
	
	// Basic variable substitution
	for key, value := range variables {
		placeholder := "{{" + key + "}}"
		valueStr := fmt.Sprintf("%v", value)
		rendered = strings.ReplaceAll(rendered, placeholder, valueStr)
	}
	
	return rendered, nil
}

// getActiveModel determines the active model using Station config priority
func (e *GenKitExecutor) getActiveModel(dpConfig *DotpromptConfig) string {
	// Try to load Station config
	stationConfig, err := config.Load()
	if err == nil && stationConfig.AIModel != "" {
		// Station config takes priority
		return stationConfig.AIModel
	}
	
	// Fallback to dotprompt config
	if dpConfig.Model != "" {
		return dpConfig.Model
	}
	
	// Ultimate fallback
	return "gemini-1.5-flash"
}


// isModelSupported checks if a model is supported (for testing)
func (e *GenKitExecutor) isModelSupported(dpConfig *DotpromptConfig) bool {
	supportedModels := map[string]bool{
		"gemini-2.0-flash-exp": true,
		"gpt-4":                true,
		"gpt-3.5-turbo":        true,
	}
	
	return supportedModels[dpConfig.Model]
}

// buildDotpromptFromAgent constructs complete dotprompt content from database agent data
// This reuses the same logic as our export functions
func (e *GenKitExecutor) buildDotpromptFromAgent(agent models.Agent, agentTools []*models.AgentToolWithDetails, environment string) string {
	// Check if agent prompt already contains dotprompt frontmatter
	if strings.HasPrefix(strings.TrimSpace(agent.Prompt), "---") {
		// Agent prompt is already a complete dotprompt, use as-is
		return agent.Prompt
	}

	// Agent prompt is simple text, wrap it with frontmatter
	var content strings.Builder

	// Get configured model from Station config, fallback to default
	modelName := "gemini-1.5-flash" // default fallback
	if cfg, _ := config.Load(); cfg != nil && cfg.AIModel != "" {
		modelName = cfg.AIModel
	}

	// YAML frontmatter with multi-role support (same as export logic)
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("model: \"%s\"\n", modelName))
	content.WriteString("config:\n")
	content.WriteString("  temperature: 0.3\n")
	content.WriteString("  max_tokens: 2000\n")
	// NOTE: Removed maxTurns from config - we handle turn limiting manually
	
	// Input schema with merged custom and default variables
	schemaHelper := schema.NewExportHelper()
	inputSchemaSection, err := schemaHelper.GenerateInputSchemaSection(&agent)
	if err != nil {
		// Fallback to default if custom schema is invalid
		content.WriteString("input:\n")
		content.WriteString("  schema:\n")
		content.WriteString("    userInput: string\n")
	} else {
		content.WriteString(inputSchemaSection)
	}
	
	content.WriteString("metadata:\n")
	content.WriteString(fmt.Sprintf("  name: \"%s\"\n", agent.Name))
	if agent.Description != "" {
		content.WriteString(fmt.Sprintf("  description: \"%s\"\n", agent.Description))
	}
	content.WriteString("  version: \"1.0.0\"\n")

	// Tools section
	if len(agentTools) > 0 {
		content.WriteString("tools:\n")
		for _, tool := range agentTools {
			content.WriteString(fmt.Sprintf("  - \"%s\"\n", tool.ToolName))
		}
	}

	// Station metadata
	content.WriteString("station:\n")
	content.WriteString("  execution_metadata:\n")
	if agent.MaxSteps > 0 {
		content.WriteString(fmt.Sprintf("    max_steps: %d\n", agent.MaxSteps))
	}
	content.WriteString(fmt.Sprintf("    environment: \"%s\"\n", environment))
	content.WriteString(fmt.Sprintf("    agent_id: %d\n", agent.ID))
	content.WriteString("---\n\n")

	// Multi-role template using dotprompt {{role}} directives
	content.WriteString("{{role \"system\"}}\n")
	content.WriteString(agent.Prompt)
	content.WriteString("\n\n{{role \"user\"}}\n")
	content.WriteString("{{userInput}}")
	content.WriteString("\n")

	return content.String()
}


// extractPromptContent extracts just the prompt content from a dotprompt file (removes frontmatter)
func (e *GenKitExecutor) extractPromptContent(dotpromptContent string) (string, error) {
	// Split by frontmatter markers
	parts := strings.Split(dotpromptContent, "---")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid dotprompt format: missing frontmatter markers")
	}
	
	// Return everything after the second "---"
	content := strings.Join(parts[2:], "---")
	return strings.TrimSpace(content), nil
}

// isMultiRolePrompt checks if a prompt already contains role directives
func (e *GenKitExecutor) isMultiRolePrompt(prompt string) bool {
	return strings.Contains(prompt, "{{role \"") || strings.Contains(prompt, "{{role '")
}

// convertDotpromptToGenkitMessages converts dotprompt messages to GenKit messages
func (e *GenKitExecutor) convertDotpromptToGenkitMessages(dotpromptMessages []dotprompt.Message) ([]*ai.Message, error) {
	var genkitMessages []*ai.Message
	
	for _, dpMsg := range dotpromptMessages {
		// Convert role
		var role ai.Role
		switch dpMsg.Role {
		case dotprompt.RoleUser:
			role = ai.RoleUser
		case dotprompt.RoleModel:
			role = ai.RoleModel
		case dotprompt.RoleSystem:
			role = ai.RoleSystem
		case dotprompt.RoleTool:
			role = ai.RoleTool
		default:
			role = ai.RoleUser // fallback
		}
		
		// Convert content parts
		var parts []*ai.Part
		for _, dpPart := range dpMsg.Content {
			switch part := dpPart.(type) {
			case *dotprompt.TextPart:
				if part.Text != "" {
					parts = append(parts, ai.NewTextPart(part.Text))
				}
			case *dotprompt.MediaPart:
				// Convert media part if needed
				if part.Media.URL != "" {
					parts = append(parts, ai.NewMediaPart(part.Media.ContentType, part.Media.URL))
				}
			}
		}
		
		// Skip empty messages
		if len(parts) == 0 {
			continue
		}
		
		// Create GenKit message
		genkitMsg := &ai.Message{
			Role:    role,
			Content: parts,
		}
		
		genkitMessages = append(genkitMessages, genkitMsg)
	}
	
	return genkitMessages, nil
}

// generateWithCustomTurnLimit implements custom turn limiting with final response capability
func (e *GenKitExecutor) generateWithCustomTurnLimit(ctx context.Context, genkitApp *genkit.Genkit, generateOpts []ai.GenerateOption, tracker *ToolCallTracker, maxToolCalls int, modelName string) (*ai.ModelResponse, error) {
	
	// Add detailed debug logging before GenKit Generate call
	if tracker.LogCallback != nil {
		tracker.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "debug",
			"message":   "ABOUT TO CALL genkit.Generate() - this is where conversations often hang",
			"details": map[string]interface{}{
				"model_name":    modelName,
				"genkit_app":    fmt.Sprintf("%T", genkitApp),
				"genkit_app_nil": genkitApp == nil,
				"context_done":  ctx.Err() != nil,
				"opts_count":    len(generateOpts),
				"max_tool_calls": maxToolCalls,
			},
		})
	}
	
	// First attempt: normal generation with tools
	logging.Debug("CRITICAL: About to call genkit.Generate with model=%s, opts=%d", modelName, len(generateOpts))
	
	generateStart := time.Now()
	response, err := genkit.Generate(ctx, genkitApp, generateOpts...)
	generateDuration := time.Since(generateStart)
	
	// Log immediately after GenKit call (before any other processing)
	if tracker.LogCallback != nil {
		tracker.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "debug",
			"message":   fmt.Sprintf("GenKit Generate call COMPLETED in %v", generateDuration),
			"details": map[string]interface{}{
				"duration":     generateDuration.String(),
				"error":        err != nil,
				"error_msg":    func() string { if err != nil { return err.Error() }; return "none" }(),
				"response_nil": response == nil,
				"quick_return": generateDuration < 100*time.Millisecond,
			},
		})
	}
	
	logging.Debug("CRITICAL: genkit.Generate returned after %v, err=%v, response_nil=%v", generateDuration, err != nil, response == nil)
	
	// If the first call succeeds, check message count to see if we need final response
	if err == nil && response != nil && response.Request != nil && response.Request.Messages != nil {
		msgCount := len(response.Request.Messages)
		
		// If we're at or past turn limit, force final response
		if msgCount >= 40 {
			if tracker.LogCallback != nil {
				tracker.LogCallback(map[string]interface{}{
					"timestamp": time.Now().Format(time.RFC3339),
					"level":     "warning",
					"message":   fmt.Sprintf("Turn limit reached (%d messages >= 40) - requesting final response without tools", msgCount),
					"details": map[string]interface{}{
						"message_count": msgCount,
						"max_turns": 40,
						"solution": "Requesting AI to provide final summary without additional tools",
					},
				})
			}
			
			// Extract conversation history and add final prompt
			currentMessages := response.Request.Messages
			summaryMessage := &ai.Message{
				Role: ai.RoleUser,
				Content: []*ai.Part{
					ai.NewTextPart("You have reached the conversation turn limit. Please provide a final response summarizing what you've learned and completed so far. Do not call any more tools."),
				},
			}
			currentMessages = append(currentMessages, summaryMessage)
			
			// Create final response options WITHOUT any tools
			var finalOpts []ai.GenerateOption
			finalOpts = append(finalOpts, ai.WithModelName(modelName))
			finalOpts = append(finalOpts, ai.WithMessages(currentMessages...))
			// Intentionally NOT adding tools - force text-only response
			
			// Attempt final generation WITHOUT tools
			finalResponse, finalErr := genkit.Generate(ctx, genkitApp, finalOpts...)
			
			if finalErr != nil {
				if tracker.LogCallback != nil {
					tracker.LogCallback(map[string]interface{}{
						"timestamp": time.Now().Format(time.RFC3339),
						"level":     "error",
						"message":   "Final response generation failed after hitting turn limit",
						"details": map[string]interface{}{
							"final_error": finalErr.Error(),
						},
					})
				}
				// Return the original successful response instead of failing completely
				return response, nil
			}
			
			// Success! Final response generated
			if tracker.LogCallback != nil {
				tracker.LogCallback(map[string]interface{}{
					"timestamp": time.Now().Format(time.RFC3339),
					"level":     "info",
					"message":   "Successfully generated final response after hitting turn limit",
					"details": map[string]interface{}{
						"response_length": len(finalResponse.Text()),
						"final_generation": true,
						"turn_limit_handled": true,
					},
				})
			}
			
			return finalResponse, nil
		}
	}
	
	// Check for API timeout errors and turn limit errors from the original call
	isAPITimeout := err != nil && (strings.Contains(strings.ToLower(err.Error()), "timeout") || 
									strings.Contains(strings.ToLower(err.Error()), "deadline exceeded") ||
									strings.Contains(strings.ToLower(err.Error()), "failed to create completion after"))
	isTurnLimitError := err != nil && strings.Contains(strings.ToLower(err.Error()), "turn")
	
	if isAPITimeout || isTurnLimitError {
		// This is an API timeout or turn limit error - we need to try a final response without tools
		errorType := "turn limit"
		if isAPITimeout {
			errorType = "API timeout"
		}
		
		if tracker.LogCallback != nil {
			tracker.LogCallback(map[string]interface{}{
				"timestamp": time.Now().Format(time.RFC3339),
				"level":     "warning",
				"message":   fmt.Sprintf("Hit %s - attempting final response without tools", errorType),
				"details": map[string]interface{}{
					"original_error": err.Error(),
					"error_type": errorType,
					"max_tool_calls": maxToolCalls,
					"solution": "Requesting AI to provide final summary without additional tools",
				},
			})
		}
		
		// Extract conversation history from the failed response
		var currentMessages []*ai.Message
		if response != nil && response.Request != nil && response.Request.Messages != nil {
			currentMessages = response.Request.Messages
		}
		
		// If we got a conversation history, try to generate a final response
		if len(currentMessages) > 0 {
			// Add a final user message requesting summary without tools
			var summaryPrompt string
			if isAPITimeout {
				summaryPrompt = "The API request timed out. Please provide a final response summarizing what you've learned and completed so far, including any analysis you performed and tools you used. Do not call any more tools."
			} else {
				summaryPrompt = "You have reached the tool usage limit. Please provide a final response summarizing what you've learned and completed so far. Do not call any more tools."
			}
			
			summaryMessage := &ai.Message{
				Role: ai.RoleUser,
				Content: []*ai.Part{
					ai.NewTextPart(summaryPrompt),
				},
			}
			currentMessages = append(currentMessages, summaryMessage)
			
			// Create final response options WITHOUT any tools
			var finalOpts []ai.GenerateOption
			finalOpts = append(finalOpts, ai.WithModelName(modelName))
			finalOpts = append(finalOpts, ai.WithMessages(currentMessages...))
			// Intentionally NOT adding tools - force text-only response
			
			// Attempt final generation WITHOUT tools
			finalResponse, finalErr := genkit.Generate(ctx, genkitApp, finalOpts...)
			
			if finalErr != nil {
				// Both attempts failed - return original error but with explanation
				if tracker.LogCallback != nil {
					tracker.LogCallback(map[string]interface{}{
						"timestamp": time.Now().Format(time.RFC3339),
						"level":     "error",
						"message":   "Both tool-enabled and final response generation failed",
						"details": map[string]interface{}{
							"original_error": err.Error(),
							"final_error": finalErr.Error(),
						},
					})
				}
				return nil, fmt.Errorf("conversation failed: %v (final attempt also failed: %v)", err, finalErr)
			}
			
			// Success! Final response generated
			if tracker.LogCallback != nil {
				successMessage := fmt.Sprintf("Successfully generated final response after hitting %s", errorType)
				tracker.LogCallback(map[string]interface{}{
					"timestamp": time.Now().Format(time.RFC3339),
					"level":     "info",
					"message":   successMessage,
					"details": map[string]interface{}{
						"response_length": len(finalResponse.Text()),
						"final_generation": true,
						"error_recovery": errorType,
						"response_preview": func() string {
							text := finalResponse.Text()
							if len(text) > 200 {
								return text[:200] + "..."
							}
							return text
						}(),
					},
				})
			}
			
			return finalResponse, nil
		}
		
		// Fallback: if we can't create a final response, return original error
		return nil, err
	}
	
	// No turn limit error - analyze the response for tool calling patterns  
	if err == nil && response != nil && response.Request != nil && response.Request.Messages != nil {
		violation := e.analyzeToolCallPatterns(response.Request.Messages, tracker)
		if violation != "" {
			// Log the pattern violation
			if tracker.LogCallback != nil {
				tracker.LogCallback(map[string]interface{}{
					"timestamp": time.Now().Format(time.RFC3339),
					"level":     "warning",
					"message":   "Tool calling pattern violation detected",
					"details": map[string]interface{}{
						"violation":     violation,
						"total_calls":   tracker.TotalCalls,
						"last_tool":     tracker.LastToolUsed,
						"consecutive":   tracker.ConsecutiveSameTool,
						"conversation_length": len(response.Request.Messages),
					},
				})
			}
		}
		
		// Check if we should force completion to prevent endless loops
		shouldComplete, reason := e.shouldForceCompletion(response.Request.Messages, tracker)
		if shouldComplete {
			if tracker.LogCallback != nil {
				tracker.LogCallback(map[string]interface{}{
					"timestamp": time.Now().Format(time.RFC3339),
					"level":     "warning",
					"message":   "Agent should complete task - enough information gathered",
					"details": map[string]interface{}{
						"completion_reason": reason,
						"total_calls":       tracker.TotalCalls,
						"conversation_length": len(response.Request.Messages),
						"recommendation":    "Agent should provide final response instead of calling more tools",
					},
				})
			}
		}
	}
	
	return response, err
}

// generateWithToolLimits wraps genkit.Generate with tool call monitoring and limits (deprecated)
func (e *GenKitExecutor) generateWithToolLimits(ctx context.Context, genkitApp *genkit.Genkit, generateOpts []ai.GenerateOption, tracker *ToolCallTracker) (*ai.ModelResponse, error) {
	// This method is deprecated in favor of generateWithCustomTurnLimit
	return e.generateWithCustomTurnLimit(ctx, genkitApp, generateOpts, tracker, 25, "gpt-4")
}

// analyzeToolCallPatterns examines conversation messages to detect problematic tool usage
func (e *GenKitExecutor) analyzeToolCallPatterns(messages []*ai.Message, tracker *ToolCallTracker) string {
	// Reset tracker state
	tracker.TotalCalls = 0
	tracker.ConsecutiveSameTool = make(map[string]int)
	tracker.LastToolUsed = ""
	
	var consecutiveCount int
	
	// Analyze all messages for tool calling patterns
	for i, msg := range messages {
		if msg.Content == nil {
			continue
		}
		
		// Count tool requests in this message
		toolsInMessage := 0
		var toolsUsed []string
		
		for _, part := range msg.Content {
			if part.IsToolRequest() && part.ToolRequest != nil {
				toolName := part.ToolRequest.Name
				toolsUsed = append(toolsUsed, toolName)
				toolsInMessage++
				tracker.TotalCalls++
				
				// Track consecutive usage
				if toolName == tracker.LastToolUsed {
					consecutiveCount++
				} else {
					consecutiveCount = 1
					tracker.LastToolUsed = toolName
				}
				
				tracker.ConsecutiveSameTool[toolName] = consecutiveCount
				
				// Check for violations
				if tracker.TotalCalls > tracker.MaxToolCalls {
					return fmt.Sprintf("Total tool calls (%d) exceeded limit (%d)", tracker.TotalCalls, tracker.MaxToolCalls)
				}
				
				if consecutiveCount > tracker.MaxConsecutive {
					return fmt.Sprintf("Consecutive calls to '%s' (%d) exceeded limit (%d)", toolName, consecutiveCount, tracker.MaxConsecutive)
				}
			}
		}
		
		// Detect mass tool calling in single message (warning sign)
		if toolsInMessage > 5 {
			if tracker.LogCallback != nil {
				tracker.LogCallback(map[string]interface{}{
					"timestamp": time.Now().Format(time.RFC3339),
					"level":     "warning", 
					"message":   fmt.Sprintf("Mass tool calling detected: %d tools in message %d", toolsInMessage, i),
					"details": map[string]interface{}{
						"tools_in_message": toolsInMessage,
						"message_index":    i,
						"tools_used":       toolsUsed,
					},
				})
			}
		}
	}
	
	// Look for escalating pattern (2 -> 12 -> more)
	if tracker.TotalCalls > 10 {
		if tracker.LogCallback != nil {
			tracker.LogCallback(map[string]interface{}{
				"timestamp": time.Now().Format(time.RFC3339),
				"level":     "warning",
				"message":   "High tool usage detected - potential obsessive calling pattern",
				"details": map[string]interface{}{
					"total_calls":         tracker.TotalCalls,
					"conversation_length": len(messages),
					"avg_tools_per_turn":  float64(tracker.TotalCalls) / float64(len(messages)),
				},
			})
		}
	}
	
	return "" // No violation detected
}

// shouldForceCompletion determines if conversation should be forced to complete
func (e *GenKitExecutor) shouldForceCompletion(messages []*ai.Message, tracker *ToolCallTracker) (bool, string) {
	messageCount := len(messages)
	
	// Force completion if approaching turn limit
	if messageCount >= 20 {
		return true, fmt.Sprintf("Approaching turn limit (%d messages)", messageCount)
	}
	
	// Force completion if too many tools used  
	if tracker.TotalCalls >= 12 {
		return true, fmt.Sprintf("Tool usage limit reached (%d calls)", tracker.TotalCalls)
	}
	
	// Detect repetitive information gathering pattern
	if messageCount > 8 {
		recentToolCalls := e.extractRecentToolCalls(messages, 5)
		if e.isRepetitivePattern(recentToolCalls) {
			return true, "Repetitive tool calling detected - likely sufficient information gathered"
		}
	}
	
	// Check for information gathering vs action taking balance
	if tracker.TotalCalls > 6 {
		infoGatheringTools := 0
		for _, msg := range messages {
			if msg.Content == nil {
				continue
			}
			for _, part := range msg.Content {
				if part.IsToolRequest() && part.ToolRequest != nil {
					toolName := part.ToolRequest.Name
					if e.isInformationGatheringTool(toolName) {
						infoGatheringTools++
					}
				}
			}
		}
		
		// If >80% of tools are information gathering, suggest completion
		if float64(infoGatheringTools)/float64(tracker.TotalCalls) > 0.8 {
			return true, fmt.Sprintf("High information gathering ratio (%d/%d) - suggest final response", infoGatheringTools, tracker.TotalCalls)
		}
	}
	
	return false, ""
}

// extractRecentToolCalls gets tool names from recent messages
func (e *GenKitExecutor) extractRecentToolCalls(messages []*ai.Message, lookback int) []string {
	var tools []string
	start := len(messages) - lookback
	if start < 0 {
		start = 0
	}
	
	for i := start; i < len(messages); i++ {
		msg := messages[i]
		if msg.Content == nil {
			continue
		}
		for _, part := range msg.Content {
			if part.IsToolRequest() && part.ToolRequest != nil {
				tools = append(tools, part.ToolRequest.Name)
			}
		}
	}
	
	return tools
}

// isRepetitivePattern detects if the same tools are being called repeatedly
func (e *GenKitExecutor) isRepetitivePattern(toolCalls []string) bool {
	if len(toolCalls) < 3 {
		return false
	}
	
	// Count occurrences
	counts := make(map[string]int)
	for _, tool := range toolCalls {
		counts[tool]++
	}
	
	// If any tool appears >50% of the time in recent calls, it's repetitive
	for _, count := range counts {
		if float64(count)/float64(len(toolCalls)) > 0.5 {
			return true
		}
	}
	
	return false
}

// isInformationGatheringTool determines if a tool is primarily for information gathering
func (e *GenKitExecutor) isInformationGatheringTool(toolName string) bool {
	infoTools := map[string]bool{
		"list_directory":      true,
		"directory_tree":      true,
		"read_text_file":      true,
		"get_file_info":       true,
		"search_files":        true,
		"list_files":          true,
		"read_file":           true,
		"stat_file":           true,
		"find_files":          true,
		"grep_files":          true,
		"get_system_info":     true,
		"check_status":        true,
		"list_processes":      true,
		"get_environment":     true,
	}
	
	return infoTools[toolName]
}