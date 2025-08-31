// Package dotprompt provides clean interfaces for agent execution
package dotprompt

import (
	"station/pkg/models"
	"github.com/firebase/genkit/go/ai" 
	"github.com/firebase/genkit/go/genkit"
)

// ExecutorInterface defines the clean interface for agent execution
type ExecutorInterface interface {
	// ExecuteAgent executes an agent using Station's GenKit integration
	ExecuteAgent(agent models.Agent, agentTools []*models.AgentToolWithDetails, genkitApp *genkit.Genkit, mcpTools []ai.ToolRef, task string, logCallback func(map[string]interface{})) (*ExecutionResponse, error)
}

// ProcessorInterface defines the interface for dotprompt template processing  
type ProcessorInterface interface {
	// IsDotpromptContent checks if content is dotprompt format
	IsDotpromptContent(prompt string) bool
	
	// RenderContent renders dotprompt content with task and agent context
	RenderContent(dotpromptContent, task, agentName string) (string, error)
	
	// BuildFromAgent builds dotprompt content from agent configuration
	BuildFromAgent(agent models.Agent, agentTools []*models.AgentToolWithDetails, environment string) string
}

// ToolManagerInterface defines the interface for tool call management
type ToolManagerInterface interface {
	// ShouldForceCompletion determines if conversation should be completed due to tool limits
	ShouldForceCompletion(messages []*ai.Message, tracker *ToolCallTracker) (bool, string)
	
	// IsInformationGatheringTool checks if a tool is for information gathering
	IsInformationGatheringTool(toolName string) bool
}

// The GenKitExecutor implements ExecutorInterface and delegates to modular components
// This provides a clean API surface while maintaining backward compatibility