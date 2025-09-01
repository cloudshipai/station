// Package genkit provides Station's GenKit-native implementation.
// This eliminates duplicate agent loops and provides a clean API surface 
// for AI agent execution with Station's enhancements.
package genkit

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"sync"
	"time"

	stationcontext "station/pkg/genkit/context"
	"station/pkg/genkit/tracking"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/logger"
	"github.com/firebase/genkit/go/genkit"
)

// ToolCallCollector accumulates tool calls during GenKit execution
type ToolCallCollector struct {
	ToolCalls []ToolCallRecord `json:"tool_calls"`
	mutex     sync.RWMutex
}

// ToolCallRecord represents a single tool call with input and output
type ToolCallRecord struct {
	Name      string                 `json:"name"`
	Input     map[string]interface{} `json:"input"`
	Output    interface{}           `json:"output"`
	Timestamp time.Time             `json:"timestamp"`
	Duration  time.Duration         `json:"duration"`
}

// AddToolCall adds a tool call record to the collector
func (tcc *ToolCallCollector) AddToolCall(record ToolCallRecord) {
	tcc.mutex.Lock()
	defer tcc.mutex.Unlock()
	tcc.ToolCalls = append(tcc.ToolCalls, record)
}

// GetToolCalls returns a copy of all collected tool calls
func (tcc *ToolCallCollector) GetToolCalls() []ToolCallRecord {
	tcc.mutex.RLock()
	defer tcc.mutex.RUnlock()
	result := make([]ToolCallRecord, len(tcc.ToolCalls))
	copy(result, tcc.ToolCalls)
	return result
}

// StationConfig configures Station's GenKit enhancements
type StationConfig struct {
	// Context management settings
	ContextThreshold   float64 // Token usage threshold (0.0-1.0) to trigger context protection
	MaxContextTokens   int     // Maximum context window size
	
	// Progressive tracking settings  
	EnableProgressiveTracking bool                                    // Enable real-time execution logging
	LogCallback              func(map[string]interface{})           // Callback for progress logs
	
	// Tool enhancement settings
	EnableToolWrapping       bool                                    // Enable MCP tool enhancement
	MaxToolOutputSize        int                                     // Maximum tool output size before truncation
	
	// Turn limiting (uses GenKit's native WithMaxTurns)
	MaxTurns                 int                                     // Maximum conversation turns (default: 25)
}


// DefaultStationConfig returns sensible defaults for Station GenKit integration
func DefaultStationConfig() *StationConfig {
	return &StationConfig{
		ContextThreshold:          0.90, // Trigger context protection at 90% usage
		MaxContextTokens:          128000, // Common context window size
		EnableProgressiveTracking: true,
		EnableToolWrapping:        true, 
		MaxToolOutputSize:         10000, // 10KB max tool output
		MaxTurns:                  25,    // Reasonable turn limit
	}
}

// StationGenerate is Station's primary AI generation function.
// It leverages GenKit's native agent loop with Station's enhancements.
// This is the single entry point for all Station AI functionality.
func StationGenerate(ctx context.Context, genkitApp *genkit.Genkit, config *StationConfig, opts ...ai.GenerateOption) (*ai.ModelResponse, error) {
	
	if config == nil {
		config = DefaultStationConfig()
	}
	
	// Create shared tool call collector for this execution
	toolCallCollector := &ToolCallCollector{
		ToolCalls: make([]ToolCallRecord, 0),
	}
	
	// Add collector to context so middleware can access it
	ctx = context.WithValue(ctx, "tool_call_collector", toolCallCollector)
	
	// Create Station's enhancement middleware
	stationMiddleware := createStationMiddleware(config)
	
	// Add Station middleware to the generation options
	enhancedOpts := append(opts, ai.WithMiddleware(stationMiddleware))
	
	// Debug logging to trace tool handling
	if config.EnableProgressiveTracking && config.LogCallback != nil {
		toolCount := 0
		toolNames := []string{}
		// Extract tool information from options for debugging
		for _, opt := range opts {
			// We can't directly inspect ai.GenerateOption but we can log that tools are present
			_ = opt
			// This is a simplified approach - tools will be shown when middleware processes the request
		}
		config.LogCallback(map[string]interface{}{
			"event":   "station_generate_tools_setup",
			"level":   "debug",
			"message": fmt.Sprintf("🔧 STATION-GENERATE: Processing generation request with %d options", len(opts)),
			"details": map[string]interface{}{
				"total_options": len(opts),
				"tool_count": toolCount,
				"tool_names": toolNames,
			},
		})
	}
	
	// Set max turns if not already specified
	if config.MaxTurns > 0 {
		hasMaxTurns := false
		// Check if MaxTurns is already set in opts
		for _, opt := range opts {
			// This is a simplified check - in practice we'd need to inspect the option
			_ = opt // Placeholder for MaxTurns detection
		}
		if !hasMaxTurns {
			enhancedOpts = append(enhancedOpts, ai.WithMaxTurns(config.MaxTurns))
		}
	}
	
	// Log the enhanced generation start
	if config.EnableProgressiveTracking && config.LogCallback != nil {
		config.LogCallback(map[string]interface{}{
			"event":       "station_generate_start",
			"level":       "info", 
			"message":     "Starting Station-enhanced GenKit generation",
			"config": map[string]interface{}{
				"context_threshold":    config.ContextThreshold,
				"max_turns":           config.MaxTurns,
				"progressive_tracking": config.EnableProgressiveTracking,
				"tool_wrapping":       config.EnableToolWrapping,
			},
		})
	}
	
	// Debug: Log just before calling GenKit's native generate
	log.Printf("🔧 STATION-GENERATE: About to call genkit.Generate with %d total options", len(enhancedOpts))

	// Set up log capture for MCP tool calls
	var logCapture *LogCapture
	if config.EnableProgressiveTracking {
		// Create and start log capture system to intercept GenKit MCP tool execution logs
		logCapture = NewLogCapture()
		logCapture.StartCapture()
		
		// Set both slog and GenKit logger to debug level temporarily to capture GenKit MCP tool logs
		originalSlogLevel := slog.SetLogLoggerLevel(slog.LevelDebug)
		originalGenkitLevel := logger.GetLevel()
		logger.SetLevel(slog.LevelDebug)
		
		defer func() {
			slog.SetLogLoggerLevel(originalSlogLevel)
			logger.SetLevel(originalGenkitLevel)
		}()
		
		log.Printf("🔧 STATION-GENERATE: Enabled log capture and DEBUG logging for MCP tool visibility")
	}
	
	// Initialize variables to collect tool call data from conversation analysis
	var conversationToolCalls []map[string]interface{}
	
	// Use GenKit's native Generate with Station enhancements
	response, err := genkit.Generate(ctx, genkitApp, enhancedOpts...)
	
	// Stop log capture and extract tool calls from GenKit execution logs
	var logCapturedToolCalls []ToolCallRecord
	if logCapture != nil {
		logCapturedToolCalls = logCapture.StopCapture()
		log.Printf("🔧 STATION-GENERATE: Log capture extracted %d tool calls from GenKit execution", len(logCapturedToolCalls))
		
		// Merge log-captured tool calls with existing collector
		if toolCallCollector != nil {
			for _, logCall := range logCapturedToolCalls {
				toolCallCollector.AddToolCall(logCall)
				log.Printf("🔧 STATION-GENERATE: Added log-captured tool call: %s (duration: %v)", logCall.Name, logCall.Duration)
			}
		}
	}
	
	// Debug: Log after GenKit generate call and inspect final usage
	if err != nil {
		log.Printf("🔧 STATION-GENERATE: genkit.Generate returned error: %v", err)
	} else {
		log.Printf("🔧 STATION-GENERATE: genkit.Generate completed successfully")
		
		// Debug: Inspect the final response structure for usage data and tool calls
		if response != nil {
			if response.Usage != nil {
				log.Printf("🔧 STATION-GENERATE: Final response has Usage - Input: %d, Output: %d, Total: %d", 
					response.Usage.InputTokens, response.Usage.OutputTokens, response.Usage.TotalTokens)
			} else {
				log.Printf("🔧 STATION-GENERATE: Final response has NO Usage field (nil)")
			}
			
			// Extract tool calls from the response message history and conversation context
			toolCallsFound := 0
			toolResponsesFound := 0
			// Use the properly scoped variables
			conversationToolCalls = make([]map[string]interface{}, 0)
			
			// Check response Message for tool calls
			if response.Message != nil && response.Message.Content != nil {
				for i, part := range response.Message.Content {
					if part.IsToolRequest() {
						toolCallsFound++
						log.Printf("🔧 STATION-GENERATE: Found ToolRequest[%d]: %s with input: %+v", 
							i, part.ToolRequest.Name, part.ToolRequest.Input)
					} else if part.IsToolResponse() {
						toolResponsesFound++
						log.Printf("🔧 STATION-GENERATE: Found ToolResponse[%d]: %s with output length: %d", 
							i, part.ToolResponse.Name, func() int {
								if str, ok := part.ToolResponse.Output.(string); ok {
									return len(str)
								}
								return 0
							}())
					}
				}
			}
			
			// Check if there's conversation history in the request (GenKit might return full conversation)
			log.Printf("🔧 STATION-GENERATE: Checking for conversation history in response...")
			if response.Request != nil && response.Request.Messages != nil {
				log.Printf("🔧 STATION-GENERATE: Response contains request with %d conversation messages", len(response.Request.Messages))
				for i, msg := range response.Request.Messages {
					if msg.Content != nil {
						for j, part := range msg.Content {
							if part.IsToolRequest() {
								toolCallsFound++
								toolCallData := map[string]interface{}{
									"tool_name": part.ToolRequest.Name,
									"input": part.ToolRequest.Input,
									"ref": part.ToolRequest.Ref,
								}
								conversationToolCalls = append(conversationToolCalls, toolCallData)
								log.Printf("🔧 STATION-GENERATE: Found conversation ToolRequest[%d,%d]: %s with input %v", i, j, part.ToolRequest.Name, part.ToolRequest.Input)
							} else if part.IsToolResponse() {
								toolResponsesFound++
								// Find the corresponding tool call and update it with output
								for k := range conversationToolCalls {
									if conversationToolCalls[k]["ref"] == part.ToolResponse.Ref {
										conversationToolCalls[k]["output"] = part.ToolResponse.Output
										break
									}
								}
								log.Printf("🔧 STATION-GENERATE: Found conversation ToolResponse[%d,%d]: %s with output %v", i, j, part.ToolResponse.Name, part.ToolResponse.Output)
							}
						}
					}
				}
			} else {
				log.Printf("🔧 STATION-GENERATE: No conversation history found in response.Request")
			}
			
			// Log other response fields for debugging
			log.Printf("🔧 STATION-GENERATE: Final response - FinishReason: %s, Content parts: %d, ToolRequests: %d, ToolResponses: %d", 
				response.FinishReason, func() int {
					if response.Message != nil && response.Message.Content != nil {
						return len(response.Message.Content)
					}
					return 0
				}(), toolCallsFound, toolResponsesFound)
			
			// Log extracted tool call data for debugging
			if len(conversationToolCalls) > 0 {
				log.Printf("🔧 STATION-GENERATE: Extracted %d tool calls with complete data", len(conversationToolCalls))
				for i, toolCall := range conversationToolCalls {
					log.Printf("🔧 STATION-GENERATE: ToolCall[%d]: %s -> %v", i, toolCall["tool_name"], toolCall["output"])
				}
			}
		} else {
			log.Printf("🔧 STATION-GENERATE: Final response is nil")
		}
	}
	
	// Log completion
	if config.EnableProgressiveTracking && config.LogCallback != nil {
		
		// Include usage data in completion log if available
		usageData := map[string]interface{}{}
		if response != nil && response.Usage != nil {
			usageData["input_tokens"] = response.Usage.InputTokens
			usageData["output_tokens"] = response.Usage.OutputTokens
			usageData["total_tokens"] = response.Usage.TotalTokens
		}
		
		// Extract tool call information from the collector and response analysis
		toolCallsInfo := map[string]interface{}{}
		if toolCallCollector != nil {
			toolCalls := toolCallCollector.GetToolCalls()
			toolCallsInfo["tool_calls"] = toolCalls
			toolCallsInfo["tools_used"] = len(toolCalls)
			
			log.Printf("🔧 STATION-GENERATE: Captured %d tool calls from execution", len(toolCalls))
			for i, call := range toolCalls {
				log.Printf("🔧 STATION-GENERATE: Tool[%d]: %s (duration: %v)", i, call.Name, call.Duration)
			}
		}
		
		// Add tool calls extracted from conversation history (from response analysis above)
		if response != nil {
			if len(conversationToolCalls) > 0 {
				toolCallsInfo["conversation_tool_calls"] = conversationToolCalls
				toolCallsInfo["conversation_tools_used"] = len(conversationToolCalls)
				log.Printf("🔧 STATION-GENERATE: Including %d tool calls from conversation history", len(conversationToolCalls))
			}
		}
		
		logCallbackData := map[string]interface{}{
			"timestamp":   time.Now().Format(time.RFC3339),
			"level":       "info",
			"message":     "Station GenKit execution completed successfully",
			"details": map[string]interface{}{
				"success":          err == nil,
				"duration_seconds": 0, // Duration will be calculated elsewhere
				"response_length":  func() int { if response != nil { return len(response.Text()) } else { return 0 } }(),
				"model_name":      "station-genkit", // Model name will be set elsewhere
				"error":           func() string { if err != nil { return err.Error() } else { return "" } }(),
				"usage":           usageData,
				"tool_calls":      toolCallsInfo,
			},
		}
		
		log.Printf("🔧 STATION-GENERATE: Sending LogCallback with keys: %v", func() []string {
			keys := make([]string, 0, len(logCallbackData))
			for k := range logCallbackData {
				keys = append(keys, k)
			}
			return keys
		}())
		log.Printf("🔧 STATION-GENERATE: LogCallback tool_calls data: %+v", toolCallsInfo)
		
		config.LogCallback(logCallbackData)
	}
	
	// Enhance the response with captured tool call information
	if response != nil && toolCallCollector != nil {
		toolCalls := toolCallCollector.GetToolCalls()
		if len(toolCalls) > 0 {
			// Store tool calls in response custom data for later retrieval
			if response.Custom == nil {
				response.Custom = make(map[string]interface{})
			}
			if customMap, ok := response.Custom.(map[string]interface{}); ok {
				customMap["station_tool_calls"] = toolCalls
				customMap["station_tools_used"] = len(toolCalls)
			}
			
			log.Printf("🔧 STATION-GENERATE: Enhanced response with %d tool calls", len(toolCalls))
		}
	}
	
	return response, err
}

// createStationMiddleware creates the middleware chain for Station enhancements
func createStationMiddleware(config *StationConfig) ai.ModelMiddleware {
	return func(next ai.ModelFunc) ai.ModelFunc {
		return func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
			// Debug: Log that middleware is being called
			log.Printf("🔧 STATION-MIDDLEWARE: Called with request containing %d messages, %d tools", len(req.Messages), len(req.Tools))
			// 1. Context Protection - check before model call
			if config.ContextThreshold > 0 {
				contextManager := stationcontext.NewManager(config.MaxContextTokens, config.ContextThreshold)
				if contextManager.WouldExceedThreshold(req) {
					// Apply intelligent truncation
					truncator := stationcontext.NewTruncator(config.MaxContextTokens)
					req = truncator.CompactRequest(req)
					
					// Log context protection action
					if config.EnableProgressiveTracking && config.LogCallback != nil {
						config.LogCallback(map[string]interface{}{
							"event":   "context_protection_applied",
							"level":   "info",
							"message": "Applied context compaction to prevent overflow",
							"details": map[string]interface{}{
								"threshold_exceeded": true,
								"compaction_applied": true,
							},
						})
					}
				}
			}
			
			// 2. Progressive Tracking - log request
			if config.EnableProgressiveTracking && config.LogCallback != nil {
				tracker := tracking.NewTracker(config.LogCallback)
				tracker.LogModelRequest(req)
				
				// Debug: Log tools in the request
				if len(req.Tools) > 0 {
					config.LogCallback(map[string]interface{}{
						"event":   "station_middleware_tools",
						"level":   "debug", 
						"message": fmt.Sprintf("🔧 STATION-MIDDLEWARE: Request has %d tools", len(req.Tools)),
						"details": map[string]interface{}{
							"tool_count": len(req.Tools),
							"tool_names": func() []string {
								names := make([]string, 0, len(req.Tools))
								for _, tool := range req.Tools {
									if tool != nil {
										names = append(names, tool.Name)
									}
								}
								return names
							}(),
						},
					})
				}
			}
			
			// 3. Enhanced Callback Logging - capture tool calls in streaming  
			if cb != nil {
				originalCb := cb
				cb = func(ctx context.Context, chunk *ai.ModelResponseChunk) error {
					// Extract tool call collector from context
					if collector, ok := ctx.Value("tool_call_collector").(*ToolCallCollector); ok && collector != nil {
						// Log streaming chunks and capture tool calls
						if chunk != nil {
							log.Printf("🔧 STATION-STREAM: Received chunk - Role: %v, Content parts: %d", chunk.Role, len(chunk.Content))
							for i, part := range chunk.Content {
								if part.IsToolResponse() {
									// Capture tool response
									log.Printf("🔧 STATION-STREAM: ToolResponse[%d]: %s", i, part.ToolResponse.Name)
									log.Printf("🔧 STATION-STREAM: ToolResponse[%d] Output: %+v", i, part.ToolResponse.Output)
									
									// Find matching tool call and update with response
									toolCalls := collector.GetToolCalls()
									for j := len(toolCalls) - 1; j >= 0; j-- {
										if toolCalls[j].Name == part.ToolResponse.Name && toolCalls[j].Output == nil {
											// Update the existing tool call with response
											collector.ToolCalls[j].Output = part.ToolResponse.Output
											collector.ToolCalls[j].Duration = time.Since(toolCalls[j].Timestamp)
											break
										}
									}
								} else if part.IsToolRequest() {
									// Capture new tool request
									log.Printf("🔧 STATION-STREAM: ToolRequest[%d]: %s", i, part.ToolRequest.Name)
									log.Printf("🔧 STATION-STREAM: ToolRequest[%d] Input: %+v", i, part.ToolRequest.Input)
									
									// Convert input to map[string]interface{} if possible
									var toolInput map[string]interface{}
									if inputMap, ok := part.ToolRequest.Input.(map[string]interface{}); ok {
										toolInput = inputMap
									} else {
										// Fallback: create a map with the raw input
										toolInput = map[string]interface{}{
											"raw_input": part.ToolRequest.Input,
										}
									}
									
									collector.AddToolCall(ToolCallRecord{
										Name:      part.ToolRequest.Name,
										Input:     toolInput,
										Output:    nil, // Will be filled when response comes
										Timestamp: time.Now(),
										Duration:  0,   // Will be calculated when response comes
									})
								} else if part.IsText() {
									log.Printf("🔧 STATION-STREAM: Text[%d]: %s", i, part.Text)
								}
							}
						}
					}
					return originalCb(ctx, chunk)
				}
			}
			
			// 4. Call the model (GenKit handles the agent loop)  
			slog.Debug("Station middleware calling GenKit model", "messages", len(req.Messages))
			
			response, err := next(ctx, req, cb)
			
			// 5. Progressive Tracking - log response  
			if config.EnableProgressiveTracking && config.LogCallback != nil {
				tracker := tracking.NewTracker(config.LogCallback)
				tracker.LogModelResponse(response, err)
			}
			
			return response, err
		}
	}
}