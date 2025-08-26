package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/tui/types"
	"station/pkg/models"
)

// AgentClient handles communication with Station agents
type AgentClient struct {
	db             db.Database
	repos          *repositories.Repositories
	agentService   services.AgentServiceInterface
}

// NewAgentClient creates a new agent client
func NewAgentClient(database db.Database, repos *repositories.Repositories, agentService services.AgentServiceInterface) *AgentClient {
	return &AgentClient{
		db:             database,
		repos:          repos,
		agentService:   agentService,
	}
}

// Alias types from the shared types package
type Agent = types.Agent
type Tool = types.Tool

// ChatSession represents an active chat session
type ChatSession struct {
	ID        string
	AgentID   int64
	AgentName string
	Model     string
	Created   time.Time
	Messages  []ChatMessage
}

// ChatMessage represents a message in a chat session
type ChatMessage struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"` // "user", "assistant", "system"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Thinking  string    `json:"thinking,omitempty"`
	ToolCalls []ToolCallResult `json:"tool_calls,omitempty"`
}

// ToolCallResult represents the result of a tool execution
type ToolCallResult struct {
	ID       string        `json:"id"`
	Tool     string        `json:"tool"`
	Input    interface{}   `json:"input"`
	Output   interface{}   `json:"output"`
	Status   string        `json:"status"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
}

// ListAgents returns available agents
func (c *AgentClient) ListAgents(ctx context.Context) ([]Agent, error) {
	agents, err := c.repos.Agents.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	var result []Agent
	for _, agent := range agents {
		// Convert to TUI Agent type
		tuiAgent := Agent{
			ID:          int64(agent.ID),
			Name:        agent.Name,
			Description: agent.Description,
			Enabled:     true, // TODO: Check if models.Agent has Enabled field
			MaxSteps:    int(agent.MaxSteps),
			Tools:       []Tool{}, // TODO: Load tools if needed
		}
		
		result = append(result, tuiAgent)
	}

	return result, nil
}

// StartChatSession starts a new chat session with an agent
func (c *AgentClient) StartChatSession(ctx context.Context, agentID int64) (*ChatSession, error) {
	agent, err := c.repos.Agents.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	sessionID := fmt.Sprintf("chat-%d-%d", agentID, time.Now().Unix())
	
	session := &ChatSession{
		ID:        sessionID,
		AgentID:   agent.ID,
		AgentName: agent.Name,
		Created:   time.Now(),
		Messages:  make([]ChatMessage, 0),
	}

	return session, nil
}

// SendMessage sends a message to an agent and returns a tea.Cmd for handling the response
func (c *AgentClient) SendMessage(ctx context.Context, session *ChatSession, userMessage string) tea.Cmd {
	return func() tea.Msg {
		// Add user message to session
		userMsg := ChatMessage{
			ID:        fmt.Sprintf("user-%d", time.Now().UnixNano()),
			Role:      "user",
			Content:   userMessage,
			Timestamp: time.Now(),
		}
		session.Messages = append(session.Messages, userMsg)

		// Execute agent
		agent, err := c.repos.Agents.GetByID(session.AgentID)
		if err != nil {
			return AgentErrorMsg{
				SessionID: session.ID,
				Error:     fmt.Errorf("failed to get agent: %w", err),
			}
		}

		// Execute agent directly (no queue)
		metadata := map[string]interface{}{
			"session_id": session.ID,
			"chat_mode":  true,
		}
		
		result, err := c.agentService.ExecuteAgentWithRunID(ctx, agent.ID, userMessage, 0, metadata)
		if err != nil {
			return AgentErrorMsg{
				SessionID: session.ID,
				Error:     fmt.Errorf("failed to execute agent: %w", err),
			}
		}
		
		// Create agent response message directly
		agentMsg := ChatMessage{
			ID:        fmt.Sprintf("agent-%d", time.Now().UnixNano()),
			Role:      "assistant",
			Content:   result.Content,
			Timestamp: time.Now(),
		}
		session.Messages = append(session.Messages, agentMsg)
		
		return AgentResponseMsg{
			SessionID: session.ID,
			Message:   agentMsg,
			Session:   session,
		}
	}
}

// monitorExecution monitors an agent execution and returns results
func (c *AgentClient) monitorExecution(ctx context.Context, session *ChatSession, runID int64) tea.Msg {
	// Poll for execution completion
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.NewTimer(5 * time.Minute) // 5 minute timeout
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return AgentErrorMsg{
				SessionID: session.ID,
				Error:     ctx.Err(),
			}
		case <-timeout.C:
			return AgentErrorMsg{
				SessionID: session.ID,
				Error:     fmt.Errorf("agent execution timeout"),
			}
		case <-ticker.C:
			// Check execution status
			run, err := c.repos.AgentRuns.GetByID(context.Background(), runID)
			if err != nil {
				continue // Keep polling
			}

			switch run.Status {
			case "completed":
				// Parse the response and create agent message
				agentMsg := c.parseAgentResponse(run)
				session.Messages = append(session.Messages, agentMsg)
				
				return AgentResponseMsg{
					SessionID: session.ID,
					Message:   agentMsg,
					Session:   session,
				}
			case "failed", "error":
				errorMsg := "Agent execution failed"
				if run.FinalResponse != "" {
					errorMsg = run.FinalResponse
				}
				return AgentErrorMsg{
					SessionID: session.ID,
					Error:     fmt.Errorf("%s", errorMsg),
				}
			case "running", "queued":
				// Still running, continue polling
				continue
			default:
				// Unknown status, continue polling
				continue
			}
		}
	}
}

// parseAgentResponse parses the agent execution result into a chat message
func (c *AgentClient) parseAgentResponse(run *models.AgentRun) ChatMessage {
	timestamp := time.Now()
	if run.CompletedAt != nil {
		timestamp = *run.CompletedAt
	}
	
	msg := ChatMessage{
		ID:        fmt.Sprintf("agent-%d", time.Now().UnixNano()),
		Role:      "assistant",
		Timestamp: timestamp,
	}

	// Parse the output
	if run.FinalResponse != "" {
		var output map[string]interface{}
		if err := json.Unmarshal([]byte(run.FinalResponse), &output); err == nil {
			// Extract content and thinking from structured output
			if content, ok := output["content"].(string); ok {
				msg.Content = content
			}
			if thinking, ok := output["thinking"].(string); ok {
				msg.Thinking = thinking
			}
			if toolCallsData, ok := output["tool_calls"].([]interface{}); ok {
				msg.ToolCalls = c.parseToolCalls(toolCallsData)
			}
		} else {
			// Fallback to raw output
			msg.Content = run.FinalResponse
		}
	}

	// If no content was extracted, use a default message
	if msg.Content == "" && msg.Thinking == "" && len(msg.ToolCalls) == 0 {
		msg.Content = "Task completed successfully."
	}

	return msg
}

// parseToolCalls parses tool call results from execution metadata
func (c *AgentClient) parseToolCalls(toolCallsData []interface{}) []ToolCallResult {
	var toolCalls []ToolCallResult
	
	for _, tcData := range toolCallsData {
		if tcMap, ok := tcData.(map[string]interface{}); ok {
			tc := ToolCallResult{
				Status: "completed",
			}
			
			if id, ok := tcMap["id"].(string); ok {
				tc.ID = id
			}
			if tool, ok := tcMap["tool"].(string); ok {
				tc.Tool = tool
			}
			if input, ok := tcMap["input"]; ok {
				tc.Input = input
			}
			if output, ok := tcMap["output"]; ok {
				tc.Output = output
			}
			if status, ok := tcMap["status"].(string); ok {
				tc.Status = status
			}
			if duration, ok := tcMap["duration"].(float64); ok {
				tc.Duration = time.Duration(duration) * time.Millisecond
			}
			if errStr, ok := tcMap["error"].(string); ok {
				tc.Error = errStr
				if tc.Status == "" {
					tc.Status = "error"
				}
			}
			
			toolCalls = append(toolCalls, tc)
		}
	}
	
	return toolCalls
}

// Message types for agent communication
type AgentResponseMsg struct {
	SessionID string
	Message   ChatMessage
	Session   *ChatSession
}

type AgentErrorMsg struct {
	SessionID string
	Error     error
}

type AgentListMsg struct {
	Agents []Agent
}

type SessionStartedMsg struct {
	Session *ChatSession
}