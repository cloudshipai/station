package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	
	"station/internal/db"
	"station/internal/services"
	"station/internal/tui/chat"
)

// NewChatProgram creates a new chat-based TUI program for Station
func NewChatProgram(database db.Database, executionQueue *services.ExecutionQueueService, agentService services.AgentServiceInterface) *tea.Program {
	model := chat.NewModel(database, executionQueue, agentService)
	
	return tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
}

// NewChatModel creates a new chat-based TUI model for SSH sessions
func NewChatModel(database db.Database, executionQueue *services.ExecutionQueueService, agentService services.AgentServiceInterface) tea.Model {
	return chat.NewModel(database, executionQueue, agentService)
}