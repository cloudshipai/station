package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"station/internal/logging"

	"github.com/firebase/genkit/go/ai"
)

// ResilientMCPTool wraps an MCP tool with retry and reconnection logic
type ResilientMCPTool struct {
	originalTool ai.ToolRef
	serverName   string
	mcpManager   *MCPConnectionManager
	maxRetries   int
}

// WrapToolsWithResilience wraps MCP tools with resilience logic
func WrapToolsWithResilience(tools []ai.ToolRef, mcpManager *MCPConnectionManager) []ai.ToolRef {
	wrappedTools := make([]ai.ToolRef, len(tools))
	
	for i, tool := range tools {
		// Extract server name from tool name (tools are prefixed with server name)
		serverName := extractServerNameFromTool(tool)
		
		wrappedTools[i] = &ResilientMCPTool{
			originalTool: tool,
			serverName:   serverName,
			mcpManager:   mcpManager,
			maxRetries:   3,
		}
	}
	
	return wrappedTools
}

// extractServerNameFromTool extracts the MCP server name from a tool
func extractServerNameFromTool(tool ai.ToolRef) string {
	// This is a simplified approach - in practice you'd need to track
	// which tools came from which servers during discovery
	if toolWithName, ok := tool.(interface{ Name() string }); ok {
		name := toolWithName.Name()
		// GitHub MCP tools typically have specific patterns
		if strings.Contains(name, "github") || strings.Contains(name, "repository") || strings.Contains(name, "issue") {
			return "github"
		}
		if strings.Contains(name, "file") || strings.Contains(name, "directory") {
			return "filesystem"
		}
	}
	return "unknown"
}

// Name implements the ai.Tool interface
func (rmt *ResilientMCPTool) Name() string {
	if named, ok := rmt.originalTool.(interface{ Name() string }); ok {
		return named.Name()
	}
	return "unknown_tool"
}

// Description implements the ai.Tool interface  
func (rmt *ResilientMCPTool) Description() string {
	if described, ok := rmt.originalTool.(interface{ Description() string }); ok {
		return described.Description()
	}
	return "Tool with resilient transport"
}

// Execute implements the ai.Tool interface with retry logic
func (rmt *ResilientMCPTool) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	var lastErr error
	
	logging.Debug("ResilientMCPTool.Execute called for tool: %s", rmt.Name())
	
	for attempt := 1; attempt <= rmt.maxRetries; attempt++ {
		// Try executing the original tool
		result, err := rmt.executeOriginalTool(ctx, input)
		
		if err == nil {
			// Success - return the result
			if attempt > 1 {
				logging.Info("🎉 Tool %s succeeded on retry attempt %d", rmt.Name(), attempt)
			}
			return result, nil
		}
		
		lastErr = err
		
		// Check if this is a business logic error that should be handled gracefully
		if isExpectedBusinessError(err) {
			logging.Debug("Tool %s encountered expected business error: %v", rmt.Name(), err)
			// Return success with error information instead of failing
			return map[string]interface{}{
				"success": false,
				"error":   err.Error(),
				"type":    "business_error",
				"message": fmt.Sprintf("Tool %s completed with expected error: %v", rmt.Name(), err),
			}, nil
		}
		
		// Check if this is a transport error that we should retry
		if !isTransportError(err) {
			// Not a transport error, don't retry
			logging.Debug("Tool %s failed with non-transport error (no retry): %v", rmt.Name(), err)
			break
		}
		
		// Transport error - log and potentially retry
		logging.Info("🔄 Tool %s failed on attempt %d with transport error: %v", rmt.Name(), attempt, err)
		
		if attempt < rmt.maxRetries {
			// Wait before retrying with exponential backoff
			backoffDuration := time.Duration(attempt) * time.Second
			logging.Info("⏳ Retrying tool %s in %v (attempt %d/%d)...", rmt.Name(), backoffDuration, attempt+1, rmt.maxRetries)
			
			select {
			case <-time.After(backoffDuration):
				// Continue to next attempt
				logging.Debug("Retry delay completed, attempting tool %s again", rmt.Name())
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	
	// All attempts failed
	logging.Info("❌ Tool %s failed after %d attempts with final error: %v", rmt.Name(), rmt.maxRetries, lastErr)
	return nil, fmt.Errorf("tool %s failed after %d attempts: %w", rmt.Name(), rmt.maxRetries, lastErr)
}

// executeOriginalTool executes the wrapped tool
func (rmt *ResilientMCPTool) executeOriginalTool(ctx context.Context, input interface{}) (interface{}, error) {
	// Create a timeout context for the tool execution
	toolCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	// Execute the original tool
	if executor, ok := rmt.originalTool.(interface{ Execute(context.Context, interface{}) (interface{}, error) }); ok {
		return executor.Execute(toolCtx, input)
	}
	
	return nil, fmt.Errorf("tool %s does not implement Execute method", rmt.Name())
}

// isExpectedBusinessError determines if an error is an expected business logic error
// that should be handled gracefully rather than causing complete failure
func isExpectedBusinessError(err error) bool {
	if err == nil {
		return false
	}
	
	errorStr := strings.ToLower(err.Error())
	expectedErrors := []string{
		"git repository is empty",
		"repository is empty",
		"no commits found",
		"not found",
		"404",
		"forbidden",
		"403",
		"rate limit",
		"access denied",
		"permission denied",
		"file not found",
		"directory not found",
		"does not exist",
	}
	
	for _, expectedErr := range expectedErrors {
		if strings.Contains(errorStr, expectedErr) {
			return true
		}
	}
	
	return false
}

// isTransportError determines if an error is related to transport/connection issues
func isTransportError(err error) bool {
	if err == nil {
		return false
	}
	
	errorStr := strings.ToLower(err.Error())
	transportErrors := []string{
		"transport error",
		"file already closed",
		"broken pipe",
		"connection refused",
		"connection reset",
		"write: broken pipe",
		"read: connection reset",
		"no such file or directory",
		"deadline exceeded",
		"context deadline exceeded",
		"connection timed out",
	}
	
	for _, transportErr := range transportErrors {
		if strings.Contains(errorStr, transportErr) {
			return true
		}
	}
	
	return false
}