package adapter

import (
	"fmt"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
)

// ToolRegistry manages the mapping of tools to their source servers
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]ToolMapping // tool_name -> ToolMapping
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]ToolMapping),
	}
}

// RegisterTool registers a tool from a source server
func (tr *ToolRegistry) RegisterTool(serverID string, tool mcp.Tool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	
	mapping := ToolMapping{
		ServerID:    serverID,
		ToolName:    tool.Name,
		Schema:      tool,
		Description: tool.Description,
	}
	
	tr.tools[tool.Name] = mapping
}

// RegisterTools registers multiple tools from a source server
func (tr *ToolRegistry) RegisterTools(serverID string, tools []mcp.Tool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	
	for _, tool := range tools {
		mapping := ToolMapping{
			ServerID:    serverID,
			ToolName:    tool.Name,
			Schema:      tool,
			Description: tool.Description,
		}
		tr.tools[tool.Name] = mapping
	}
}

// GetToolMapping returns the mapping for a specific tool
func (tr *ToolRegistry) GetToolMapping(toolName string) (ToolMapping, error) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	
	mapping, exists := tr.tools[toolName]
	if !exists {
		return ToolMapping{}, fmt.Errorf("tool %s not found in registry", toolName)
	}
	
	return mapping, nil
}

// GetAllTools returns all registered tools
func (tr *ToolRegistry) GetAllTools() []mcp.Tool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	
	tools := make([]mcp.Tool, 0, len(tr.tools))
	for _, mapping := range tr.tools {
		tools = append(tools, mapping.Schema)
	}
	
	return tools
}

// GetToolsForAgent returns tools filtered for a specific agent session
func (tr *ToolRegistry) GetToolsForAgent(session *AgentSession) []mcp.Tool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	
	// Create a map of selected tools for quick lookup
	selectedTools := make(map[string]bool)
	for _, toolName := range session.SelectedTools {
		selectedTools[toolName] = true
	}
	
	// Filter tools based on agent's selection
	tools := make([]mcp.Tool, 0, len(session.SelectedTools))
	for _, mapping := range tr.tools {
		if selectedTools[mapping.ToolName] {
			tools = append(tools, mapping.Schema)
		}
	}
	
	return tools
}

// HasTool checks if a tool is registered
func (tr *ToolRegistry) HasTool(toolName string) bool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	
	_, exists := tr.tools[toolName]
	return exists
}

// RemoveTool removes a tool from the registry
func (tr *ToolRegistry) RemoveTool(toolName string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	
	delete(tr.tools, toolName)
}

// RemoveToolsFromServer removes all tools from a specific server
func (tr *ToolRegistry) RemoveToolsFromServer(serverID string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	
	// Find and remove tools from the specified server
	for toolName, mapping := range tr.tools {
		if mapping.ServerID == serverID {
			delete(tr.tools, toolName)
		}
	}
}

// GetToolCount returns the total number of registered tools
func (tr *ToolRegistry) GetToolCount() int {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	
	return len(tr.tools)
}

// GetServerTools returns all tools from a specific server
func (tr *ToolRegistry) GetServerTools(serverID string) []mcp.Tool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	
	var tools []mcp.Tool
	for _, mapping := range tr.tools {
		if mapping.ServerID == serverID {
			tools = append(tools, mapping.Schema)
		}
	}
	
	return tools
}