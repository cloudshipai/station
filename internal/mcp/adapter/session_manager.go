package adapter

import (
	"fmt"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
)

// SessionManager manages agent sessions and their tool access
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[int64]*AgentSession // agent_id -> session
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[int64]*AgentSession),
	}
}

// CreateAgentSession creates a new agent session
func (sm *SessionManager) CreateAgentSession(agentID int64, selectedTools []string, environment string) *AgentSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	session := &AgentSession{
		AgentID:       agentID,
		SelectedTools: selectedTools,
		Environment:   environment,
	}
	
	sm.sessions[agentID] = session
	return session
}

// GetAgentSession returns the session for a specific agent
func (sm *SessionManager) GetAgentSession(agentID int64) (*AgentSession, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	session, exists := sm.sessions[agentID]
	if !exists {
		return nil, fmt.Errorf("no session found for agent %d", agentID)
	}
	
	return session, nil
}

// UpdateAgentTools updates the tool selection for an agent
func (sm *SessionManager) UpdateAgentTools(agentID int64, selectedTools []string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	session, exists := sm.sessions[agentID]
	if !exists {
		return fmt.Errorf("no session found for agent %d", agentID)
	}
	
	session.SelectedTools = selectedTools
	return nil
}

// FilterToolsForAgent filters available tools based on agent's session
func (sm *SessionManager) FilterToolsForAgent(agentID int64, allTools []mcp.Tool) ([]mcp.Tool, error) {
	session, err := sm.GetAgentSession(agentID)
	if err != nil {
		return nil, err
	}
	
	// Create a map of selected tools for quick lookup
	selectedToolsMap := make(map[string]bool)
	for _, toolName := range session.SelectedTools {
		selectedToolsMap[toolName] = true
	}
	
	// Filter tools based on agent's selection
	var filteredTools []mcp.Tool
	for _, tool := range allTools {
		if selectedToolsMap[tool.Name] {
			filteredTools = append(filteredTools, tool)
		}
	}
	
	return filteredTools, nil
}

// IsToolAllowedForAgent checks if an agent can access a specific tool
func (sm *SessionManager) IsToolAllowedForAgent(agentID int64, toolName string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	session, exists := sm.sessions[agentID]
	if !exists {
		return false
	}
	
	// Check if tool is in agent's selected tools
	for _, selectedTool := range session.SelectedTools {
		if selectedTool == toolName {
			return true
		}
	}
	
	return false
}

// RemoveAgentSession removes a session for an agent
func (sm *SessionManager) RemoveAgentSession(agentID int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	delete(sm.sessions, agentID)
}

// GetAllSessions returns all active sessions
func (sm *SessionManager) GetAllSessions() map[int64]*AgentSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	sessions := make(map[int64]*AgentSession)
	for agentID, session := range sm.sessions {
		sessionCopy := *session // Copy the session
		sessions[agentID] = &sessionCopy
	}
	
	return sessions
}

// GetSessionCount returns the number of active sessions
func (sm *SessionManager) GetSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	return len(sm.sessions)
}

// HasAgentSession checks if an agent has an active session
func (sm *SessionManager) HasAgentSession(agentID int64) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	_, exists := sm.sessions[agentID]
	return exists
}

// GetAgentTools returns the tools available to a specific agent
func (sm *SessionManager) GetAgentTools(agentID int64) ([]string, error) {
	session, err := sm.GetAgentSession(agentID)
	if err != nil {
		return nil, err
	}
	
	// Return a copy to avoid modification
	tools := make([]string, len(session.SelectedTools))
	copy(tools, session.SelectedTools)
	
	return tools, nil
}

// GetAgentEnvironment returns the environment for a specific agent
func (sm *SessionManager) GetAgentEnvironment(agentID int64) (string, error) {
	session, err := sm.GetAgentSession(agentID)
	if err != nil {
		return "", err
	}
	
	return session.Environment, nil
}