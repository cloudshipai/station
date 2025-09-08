package dotprompt

import (
	"context"
	"fmt"
	"log"
	"os"
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
	ToolFailures        int // Track number of tool failures
	HasToolFailures     bool // Track if any tool failures occurred
}

// GenKitExecutor handles dotprompt-based agent execution using GenKit Generate
type GenKitExecutor struct{
	logCallback func(map[string]interface{})
}

// NewGenKitExecutor creates a new GenKit-based dotprompt executor
func NewGenKitExecutor() *GenKitExecutor {
	return &GenKitExecutor{}
}


// ExecuteAgent executes an agent using dotprompt template system with GenKit Generate
func (e *GenKitExecutor) ExecuteAgent(agent models.Agent, agentTools []*models.AgentToolWithDetails, genkitApp *genkit.Genkit, mcpTools []ai.ToolRef, task string, logCallback func(map[string]interface{})) (*ExecutionResponse, error) {
	startTime := time.Now()
	
	// Store the logging callback for use during execution
	e.logCallback = logCallback
	
	// Add logging for execution start
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
	
	// 2. Parse frontmatter config (temperature configuration removed for gpt-5 compatibility)
	// Direct stderr write to bypass potential logging system hang
	fmt.Fprintf(os.Stderr, "🔥 DIRECT-STDERR: Step 2 - About to parse frontmatter config\n")
	log.Printf("🔥 DEBUG-FLOW: Step 2 - Parsing frontmatter config")
	logging.Debug("DEBUG-FLOW: Step 2 - Parsing frontmatter config")
	
	// 3. Use dotprompt library directly for multi-role rendering (bypasses GenKit constraint)
	log.Printf("🔥 DEBUG-FLOW: Step 3 - Creating dotprompt instance")
	logging.Debug("DEBUG-FLOW: Step 3 - Creating dotprompt instance")
	dp := dotprompt.NewDotprompt(nil)
	log.Printf("🔥 DEBUG-FLOW: Step 3 - Compiling dotprompt content (length: %d)", len(dotpromptContent))
	logging.Debug("DEBUG-FLOW: Step 3 - Compiling dotprompt content (length: %d)", len(dotpromptContent))
	promptFunc, err := dp.Compile(dotpromptContent, nil)
	if err != nil {
		logging.Debug("DEBUG-FLOW: Step 3 - FAILED to compile dotprompt: %v", err)
		return &ExecutionResponse{
			Success:   false,
			Response:  "",
			Duration:  time.Since(startTime),
			Error:     fmt.Sprintf("failed to compile dotprompt: %v", err),
		}, nil
	}
	
	logging.Debug("Dotprompt compiled successfully")
	logging.Debug("DEBUG-FLOW: Step 3 - Dotprompt compiled successfully")
	
	// 4. Render the prompt with merged input data (default + custom schema)
	logging.Debug("DEBUG-FLOW: Step 4 - Creating schema helper")
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
	
	// Temperature configuration removed for gpt-5 model compatibility
	// gpt-5 uses default temperature of 1 and doesn't accept temperature parameter
	
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

	// Log successful completion
	if logCallback != nil {
		logCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "info",
			"message":   "Agent execution completed successfully",
			"details": map[string]interface{}{
				"duration_seconds": time.Since(startTime).Seconds(),
				"response_length":  len(responseText),
				"tools_used":       len(allToolCalls),
				"steps_taken":      len(executionSteps),
				"token_usage":      tokenUsage,
			},
		})
	}

	// Determine overall success - fails if tool failures occurred
	overallSuccess := true
	if toolCallTracker != nil && toolCallTracker.HasToolFailures {
		overallSuccess = false
		if logCallback != nil {
			logCallback(map[string]interface{}{
				"timestamp": time.Now().Format(time.RFC3339),
				"level":     "warning",
				"message":   "Execution marked as failed due to tool failures",
				"details": map[string]interface{}{
					"tool_failures": toolCallTracker.ToolFailures,
				},
			})
		}
	}
	
	return &ExecutionResponse{
		Success:        overallSuccess,
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

// ============================================================================
// DEPRECATED METHODS FOR TEST COMPATIBILITY
// ============================================================================

// ExecuteAgentWithDotpromptTemplate is deprecated - kept for test compatibility
func (e *GenKitExecutor) ExecuteAgentWithDotpromptTemplate(extractor *RuntimeExtraction, request *ExecutionRequest) (*ExecutionResponse, error) {
	return nil, fmt.Errorf("ExecuteAgentWithDotpromptTemplate is deprecated - use ExecuteAgent instead")
}

// ExecuteAgentWithGenerate is deprecated - kept for test compatibility  
func (e *GenKitExecutor) ExecuteAgentWithGenerate(extractor *RuntimeExtraction, request *ExecutionRequest) (*ExecutionResponse, error) {
	return nil, fmt.Errorf("ExecuteAgentWithGenerate is deprecated - use ExecuteAgent instead")
}

// ============================================================================
