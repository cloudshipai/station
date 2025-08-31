package tools

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	stationctx "station/pkg/context"

	"github.com/firebase/genkit/go/ai"
)

// Executor implements the ToolExecutor interface with context protection
type Executor struct {
	Config          ToolExecutorConfig `json:"config"`
	ContextManager  ContextManager     `json:"-"`
	OutputProcessor ToolOutputProcessor `json:"-"`
	LogCallback     LogCallback        `json:"-"`
	
	// Execution tracking
	metrics         *ToolMetrics        `json:"-"`
	executionHistory []ExecutionResult  `json:"-"`
	metricsMutex    sync.RWMutex        `json:"-"`
}

// NewExecutor creates a new tool executor with context protection
func NewExecutor(config ToolExecutorConfig, contextManager ContextManager, logCallback LogCallback) *Executor {
	// Set sensible defaults
	if config.MaxOutputTokens == 0 {
		config.MaxOutputTokens = 2000 // Default 2K tokens per tool output
	}
	if config.TruncationStrategy == "" {
		config.TruncationStrategy = TruncationStrategyIntelligent
	}
	if config.ContextBuffer == 0 {
		config.ContextBuffer = 1000 // Reserve 1K tokens for response generation
	}
	if config.MaxConcurrentTools == 0 {
		config.MaxConcurrentTools = 3 // Default to 3 concurrent tool executions
	}
	if config.ToolTimeout == 0 {
		config.ToolTimeout = 30 * time.Second // Default 30s timeout
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 2 // Default 2 retries
	}

	executor := &Executor{
		Config:          config,
		ContextManager:  contextManager,
		LogCallback:     logCallback,
		metrics: &ToolMetrics{
			ToolTypeBreakdown: make(map[string]int),
			LastExecutionTime: time.Now(),
		},
		executionHistory: make([]ExecutionResult, 0),
	}

	// Use default output processor if none provided
	if executor.OutputProcessor == nil {
		executor.OutputProcessor = NewDefaultOutputProcessor()
	}

	return executor
}

// ExecuteTool executes a single tool with comprehensive context protection
func (e *Executor) ExecuteTool(ctx context.Context, toolCall *ai.ToolRequest, execCtx *ExecutionContext) (*ExecutionResult, error) {
	startTime := time.Now()
	executionID := e.generateExecutionID()

	if e.LogCallback != nil {
		e.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "info",
			"message":   "Tool execution starting",
			"details": map[string]interface{}{
				"tool_name":      toolCall.Name,
				"execution_id":   executionID,
				"turn_number":    execCtx.TurnNumber,
				"remaining_tokens": execCtx.RemainingTokens,
				"safe_action_limit": execCtx.SafeActionLimit,
			},
		})
	}

	// Pre-execution context protection checks
	if e.Config.EnableContextProtection {
		canExecute, reason := e.CanExecuteTool(toolCall, execCtx)
		if !canExecute {
			result := &ExecutionResult{
				ToolName:    toolCall.Name,
				Success:     false,
				Output:      "",
				Error:       fmt.Sprintf("Context protection prevented execution: %s", reason),
				Duration:    time.Since(startTime),
				ExecutionID: executionID,
				Timestamp:   startTime,
			}
			e.recordExecution(result)
			return result, nil // Don't return error - this is expected behavior
		}
	}

	// Create execution context with timeout
	toolCtx, cancel := context.WithTimeout(ctx, e.Config.ToolTimeout)
	defer cancel()

	// Execute the tool with error handling
	var output string
	var executeErr error

	// The actual tool execution would happen here - for now this is a placeholder
	// In the real implementation, this would integrate with the MCP client or GenKit tool registry
	output, executeErr = e.executeToolInternal(toolCtx, toolCall)

	duration := time.Since(startTime)
	success := executeErr == nil

	result := &ExecutionResult{
		ToolName:    toolCall.Name,
		Success:     success,
		Duration:    duration,
		ExecutionID: executionID,
		Timestamp:   startTime,
	}

	if executeErr != nil {
		result.Error = executeErr.Error()
		result.Output = ""
	} else {
		// Process output with context-aware truncation
		processedOutput, processErr := e.processToolOutput(output, toolCall.Name, execCtx)
		if processErr != nil {
			result.Error = fmt.Sprintf("Output processing failed: %v", processErr)
			result.Success = false
		} else {
			result.Output = processedOutput.Content
			result.TokensUsed = processedOutput.TokenCount
			result.Truncated = processedOutput.Truncated
			result.OriginalLength = processedOutput.OriginalLength
		}
	}

	// Update context manager with token usage
	if result.Success && result.TokensUsed > 0 && e.ContextManager != nil {
		e.ContextManager.TrackTokenUsage(stationctx.TokenUsage{
			OutputTokens: result.TokensUsed,
			TotalTokens:  result.TokensUsed,
		})
	}

	// Record execution for metrics and logging
	e.recordExecution(result)

	if e.LogCallback != nil {
		logLevel := "info"
		if !result.Success {
			logLevel = "error"
		} else if result.Truncated {
			logLevel = "warning"
		}

		e.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     logLevel,
			"message":   "Tool execution completed",
			"details": map[string]interface{}{
				"tool_name":       result.ToolName,
				"execution_id":    result.ExecutionID,
				"success":         result.Success,
				"duration_ms":     result.Duration.Milliseconds(),
				"tokens_used":     result.TokensUsed,
				"output_length":   len(result.Output),
				"truncated":       result.Truncated,
				"original_length": result.OriginalLength,
				"error":          result.Error,
			},
		})
	}

	return result, nil
}

// ExecuteToolBatch executes multiple tools with shared context management
func (e *Executor) ExecuteToolBatch(ctx context.Context, toolCalls []*ai.ToolRequest, execCtx *ExecutionContext) ([]*ExecutionResult, error) {
	if len(toolCalls) == 0 {
		return []*ExecutionResult{}, nil
	}

	// Limit concurrent executions
	maxConcurrent := e.Config.MaxConcurrentTools
	if len(toolCalls) < maxConcurrent {
		maxConcurrent = len(toolCalls)
	}

	if e.LogCallback != nil {
		e.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "info",
			"message":   "Batch tool execution starting",
			"details": map[string]interface{}{
				"tool_count":        len(toolCalls),
				"max_concurrent":    maxConcurrent,
				"remaining_tokens":  execCtx.RemainingTokens,
				"safe_action_limit": execCtx.SafeActionLimit,
			},
		})
	}

	// Use semaphore pattern for controlled concurrency
	semaphore := make(chan struct{}, maxConcurrent)
	results := make([]*ExecutionResult, len(toolCalls))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, toolCall := range toolCalls {
		wg.Add(1)
		go func(index int, tool *ai.ToolRequest) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Create isolated execution context for this tool
			toolExecCtx := *execCtx // Copy the struct
			
			// Execute tool with context protection
			result, err := e.ExecuteTool(ctx, tool, &toolExecCtx)
			if err != nil {
				result = &ExecutionResult{
					ToolName:    tool.Name,
					Success:     false,
					Error:       fmt.Sprintf("Execution failed: %v", err),
					ExecutionID: e.generateExecutionID(),
					Timestamp:   time.Now(),
				}
			}

			mu.Lock()
			results[index] = result
			// Update shared execution context with token usage for subsequent tools
			if result.Success && result.TokensUsed > 0 {
				execCtx.RemainingTokens -= result.TokensUsed
				if execCtx.RemainingTokens < 0 {
					execCtx.RemainingTokens = 0
				}
			}
			mu.Unlock()
		}(i, toolCall)
	}

	wg.Wait()

	// Count successful/failed executions
	successful := 0
	failed := 0
	totalTokens := 0
	
	for _, result := range results {
		if result.Success {
			successful++
			totalTokens += result.TokensUsed
		} else {
			failed++
		}
	}

	if e.LogCallback != nil {
		e.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "info",
			"message":   "Batch tool execution completed",
			"details": map[string]interface{}{
				"total_tools":    len(toolCalls),
				"successful":     successful,
				"failed":         failed,
				"total_tokens":   totalTokens,
				"remaining_tokens": execCtx.RemainingTokens,
			},
		})
	}

	return results, nil
}

// CanExecuteTool checks if a tool can be safely executed given current context
func (e *Executor) CanExecuteTool(toolCall *ai.ToolRequest, execCtx *ExecutionContext) (bool, string) {
	if !e.Config.EnableContextProtection || e.ContextManager == nil {
		return true, "Context protection disabled"
	}

	// Check if context is approaching limit
	if e.ContextManager.IsApproachingLimit() {
		return false, fmt.Sprintf("Context utilization %.1f%% exceeds safety threshold", 
			e.ContextManager.GetUtilizationPercent()*100)
	}

	// Estimate tool output tokens
	estimatedTokens, err := e.EstimateToolOutputTokens(toolCall)
	if err != nil {
		// If we can't estimate, use conservative default
		estimatedTokens = e.Config.MaxOutputTokens
	}

	// Check if we have enough tokens remaining
	canExecute, reason := e.ContextManager.CanExecuteAction(estimatedTokens)
	if !canExecute {
		return false, fmt.Sprintf("Insufficient context space: %s (estimated %d tokens)", reason, estimatedTokens)
	}

	// Check if we're within safe action limit
	safeLimit := e.ContextManager.GetSafeActionLimit()
	if estimatedTokens > safeLimit {
		return false, fmt.Sprintf("Tool output estimate %d tokens exceeds safe action limit %d tokens", 
			estimatedTokens, safeLimit)
	}

	return true, fmt.Sprintf("Tool can execute safely (estimated %d tokens, %d available)", 
		estimatedTokens, e.ContextManager.GetTokensRemaining())
}

// EstimateToolOutputTokens estimates the token usage of a tool execution
func (e *Executor) EstimateToolOutputTokens(toolCall *ai.ToolRequest) (int, error) {
	// Tool-specific estimation logic based on tool name and parameters
	toolName := strings.ToLower(toolCall.Name)
	
	// File reading tools - estimate based on file operations
	if strings.Contains(toolName, "read_file") || strings.Contains(toolName, "read_text") {
		// Estimate ~500 tokens for typical file reads, but use max output as ceiling
		return min(500, e.Config.MaxOutputTokens), nil
	}
	
	// Directory listing tools - usually smaller outputs
	if strings.Contains(toolName, "list_dir") || strings.Contains(toolName, "directory") {
		return min(200, e.Config.MaxOutputTokens), nil
	}
	
	// Search tools - variable, but typically moderate
	if strings.Contains(toolName, "search") || strings.Contains(toolName, "grep") {
		return min(800, e.Config.MaxOutputTokens), nil
	}
	
	// Code analysis tools - can be large
	if strings.Contains(toolName, "analyze") || strings.Contains(toolName, "lint") {
		return min(1500, e.Config.MaxOutputTokens), nil
	}
	
	// Write operations - usually small outputs (confirmations)
	if strings.Contains(toolName, "write") || strings.Contains(toolName, "create") {
		return min(50, e.Config.MaxOutputTokens), nil
	}
	
	// Default estimation - use 70% of max output tokens as conservative estimate
	return int(float64(e.Config.MaxOutputTokens) * 0.7), nil
}

// GetToolMetrics returns execution metrics for analysis
func (e *Executor) GetToolMetrics() *ToolMetrics {
	e.metricsMutex.RLock()
	defer e.metricsMutex.RUnlock()
	
	// Return a copy to avoid concurrent access issues
	metricsCopy := *e.metrics
	metricsCopy.ToolTypeBreakdown = make(map[string]int)
	for k, v := range e.metrics.ToolTypeBreakdown {
		metricsCopy.ToolTypeBreakdown[k] = v
	}
	
	return &metricsCopy
}

// executeToolInternal handles the actual tool execution (placeholder for MCP integration)
func (e *Executor) executeToolInternal(ctx context.Context, toolCall *ai.ToolRequest) (string, error) {
	// This is a placeholder - in the real implementation, this would:
	// 1. Look up the tool in the MCP client registry
	// 2. Execute the tool with the provided parameters
	// 3. Handle any errors or timeouts
	// 4. Return the raw output
	
	// For testing purposes, simulate different tool behaviors
	toolName := strings.ToLower(toolCall.Name)
	
	switch {
	case strings.Contains(toolName, "read_file"):
		return "File contents would be here...", nil
	case strings.Contains(toolName, "list_dir"):
		return "file1.txt\nfile2.txt\nsubdir/", nil
	case strings.Contains(toolName, "search"):
		return "Search results: 3 matches found", nil
	default:
		return fmt.Sprintf("Tool %s executed successfully", toolCall.Name), nil
	}
}

// processToolOutput processes raw tool output with context-aware truncation
func (e *Executor) processToolOutput(output string, toolName string, execCtx *ExecutionContext) (*ProcessedOutput, error) {
	maxTokens := e.Config.MaxOutputTokens
	
	// Adjust max tokens based on remaining context
	if e.Config.EnableContextProtection && e.ContextManager != nil {
		safeLimit := e.ContextManager.GetSafeActionLimit()
		if safeLimit < maxTokens {
			maxTokens = safeLimit
		}
	}
	
	return e.OutputProcessor.ProcessOutput(output, toolName, maxTokens, e.Config.TruncationStrategy)
}

// recordExecution records execution results for metrics and history
func (e *Executor) recordExecution(result *ExecutionResult) {
	e.metricsMutex.Lock()
	defer e.metricsMutex.Unlock()
	
	// Update metrics
	e.metrics.TotalExecutions++
	if result.Success {
		e.metrics.SuccessfulExecutions++
	} else {
		e.metrics.FailedExecutions++
	}
	
	if result.Truncated {
		e.metrics.TruncatedExecutions++
		e.metrics.TokensSaved += (result.OriginalLength - len(result.Output)) / 4 // Rough token estimate
	}
	
	// Update tool type breakdown
	toolType := e.categorizeToolType(result.ToolName)
	e.metrics.ToolTypeBreakdown[toolType]++
	
	// Update averages
	if e.metrics.TotalExecutions > 0 {
		totalTokens := float64(e.metrics.AverageTokenUsage)*float64(e.metrics.TotalExecutions-1) + float64(result.TokensUsed)
		e.metrics.AverageTokenUsage = totalTokens / float64(e.metrics.TotalExecutions)
		
		totalDuration := e.metrics.AverageDuration*time.Duration(e.metrics.TotalExecutions-1) + result.Duration
		e.metrics.AverageDuration = totalDuration / time.Duration(e.metrics.TotalExecutions)
	}
	
	e.metrics.LastExecutionTime = result.Timestamp
	
	// Store in execution history (keep last 100 executions)
	e.executionHistory = append(e.executionHistory, *result)
	if len(e.executionHistory) > 100 {
		e.executionHistory = e.executionHistory[1:]
	}
}

// categorizeToolType categorizes tools by their primary function
func (e *Executor) categorizeToolType(toolName string) string {
	toolName = strings.ToLower(toolName)
	
	switch {
	case strings.Contains(toolName, "read") || strings.Contains(toolName, "get"):
		return "read"
	case strings.Contains(toolName, "write") || strings.Contains(toolName, "create") || strings.Contains(toolName, "update"):
		return "write"
	case strings.Contains(toolName, "list") || strings.Contains(toolName, "directory"):
		return "list"
	case strings.Contains(toolName, "search") || strings.Contains(toolName, "find"):
		return "search"
	case strings.Contains(toolName, "analyze") || strings.Contains(toolName, "check"):
		return "analyze"
	case strings.Contains(toolName, "execute") || strings.Contains(toolName, "run"):
		return "execute"
	default:
		return "other"
	}
}

// generateExecutionID generates a unique execution ID for tracking
func (e *Executor) generateExecutionID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}