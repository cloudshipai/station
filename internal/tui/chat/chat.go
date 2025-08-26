package chat

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/tui/chat/components"
	"station/internal/tui/dialogs"
	"station/internal/tui/layout"
	"station/internal/tui/theme"
)

// Message represents a conversation message
type Message struct {
	ID        string
	Role      string // "user", "agent", "system"
	Content   string
	AgentName string
	Timestamp time.Time
	Thinking  string // Reasoning content for agents
	ToolCalls []ToolCall
}

// ToolCall represents an agent tool execution
type ToolCall struct {
	ID       string
	Tool     string
	Input    string
	Output   string
	Status   string // "running", "completed", "error"
	Duration time.Duration
}

// SessionInfo represents the current agent session
type SessionInfo struct {
	ID          string
	AgentID     int64
	AgentName   string
	ModelName   string
	Created     time.Time
	TokenUsage  int
	TokenLimit  int
	Cost        float64
	MessageCount int
}

// Model represents the main chat TUI model
type Model struct {
	// Core components
	width  int
	height int
	
	// Database and services
	db             db.Database
	repos          *repositories.Repositories
	agentService   services.AgentServiceInterface
	agentClient    *AgentClient
	
	// UI components
	messages components.MessagesComponent
	editor   components.EditorComponent
	
	// State
	session      *ChatSession
	chatMessages []components.Message
	loading      bool
	error        string
	
	// Chat state
	activeSession bool
	agentsBusy    bool
	currentInput  string
	
	// Agent data
	availableAgents []Agent
	
	// UI dialogs
	agentDialog *dialogs.AgentSelectionDialog
	
	// Theme
	theme *theme.Theme
}

// NewModel creates a new chat TUI model
func NewModel(database db.Database, repos *repositories.Repositories, executionQueue interface{}, agentService services.AgentServiceInterface) *Model {
	return &Model{
		db:             database,
		repos:          repos,
		agentService:   agentService,
		agentClient:    NewAgentClient(database, repos, agentService),
		chatMessages:   make([]components.Message, 0),
		theme:          theme.DefaultTheme(),
		activeSession:  false,
		agentsBusy:     false,
	}
}

// Init initializes the chat model
func (m *Model) Init() tea.Cmd {
	// Initialize components
	m.messages = components.NewMessagesComponent(m.width, m.height-5) // Reserve space for editor
	m.editor = components.NewEditorComponent(m.width)
	
	var cmds []tea.Cmd
	cmds = append(cmds, m.messages.Init())
	cmds = append(cmds, m.editor.Init())
	
	// Load available agents
	cmds = append(cmds, m.loadAgents())
	
	return tea.Batch(cmds...)
}

// Update handles messages and updates the model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		// Update component sizes
		m.messages.SetSize(m.width, m.height-5) // Reserve space for editor and status
		m.editor.SetWidth(m.width)
		
		// Forward to components
		_, cmd := m.messages.Update(msg)
		cmds = append(cmds, cmd)
		_, cmd = m.editor.Update(msg)
		cmds = append(cmds, cmd)
		
	case tea.KeyMsg:
		// Handle global shortcuts
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "ctrl+n":
			return m, m.showAgentSelection()
		case "ctrl+l":
			return m, m.showAgentSelection()
		default:
			// Forward to editor if not handled
			updated, cmd := m.editor.Update(msg)
			m.editor = updated.(components.EditorComponent)
			cmds = append(cmds, cmd)
		}
		
	case components.MessageSubmitMsg:
		// User submitted a message
		return m, m.handleUserMessage(msg.Content)
		
	case AgentResponseMsg:
		// Received response from agent
		return m, m.handleAgentResponse(msg)
		
	case SessionStartedMsg:
		// New session started with an agent
		m.session = msg.Session
		m.activeSession = true
		m.chatMessages = make([]components.Message, 0)
		
		// Add system message
		systemMsg := components.Message{
			ID:        generateID(),
			Role:      "system", 
			Content:   fmt.Sprintf("Started new conversation with %s", msg.Session.AgentName),
			Timestamp: time.Now(),
		}
		m.chatMessages = append(m.chatMessages, systemMsg)
		
		// Update messages component
		_, cmd := m.messages.SetMessages(m.chatMessages)
		cmds = append(cmds, cmd)
		
	case dialogs.AgentSelectedMsg:
		// User selected an agent
		return m, m.startSessionWithAgent(msg.Agent)
		
	case AgentListMsg:
		// Received list of agents
		m.availableAgents = msg.Agents
		if m.agentDialog != nil {
			m.agentDialog.UpdateAgents(msg.Agents)
		}
		
	case error:
		m.error = msg.Error()
		m.loading = false
	}
	
	// Update components
	updated, cmd := m.messages.Update(msg)
	m.messages = updated.(components.MessagesComponent)
	cmds = append(cmds, cmd)
	
	// Update dialogs
	if m.agentDialog != nil {
		updated, cmd := m.agentDialog.Update(msg)
		m.agentDialog = updated.(*dialogs.AgentSelectionDialog)
		cmds = append(cmds, cmd)
	}
	
	return m, tea.Batch(cmds...)
}

// View renders the chat interface
func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}
	
	// Create main layout
	var sections []string
	
	// Header with session info
	sections = append(sections, m.renderHeader())
	
	// Messages area
	sections = append(sections, m.messages.View())
	
	// Editor area
	sections = append(sections, m.editor.View())
	
	// Status bar
	sections = append(sections, m.renderStatusBar())
	
	mainView := lipgloss.JoinVertical(lipgloss.Left, sections...)
	
	// Add overlay dialogs if any are visible
	if m.agentDialog != nil && m.agentDialog.IsVisible() {
		mainView = layout.RenderOverlay(
			m.width, 
			m.height, 
			mainView, 
			m.agentDialog.View(),
			true, // with backdrop
			m.theme.Background,
		)
	}
	
	return mainView
}

// renderHeader renders the session header
func (m *Model) renderHeader() string {
	if !m.activeSession || m.session == nil {
		// Show welcome header
		style := lipgloss.NewStyle().
			Foreground(m.theme.Primary).
			Bold(true).
			Width(m.width).
			Align(lipgloss.Center).
			Padding(1)
		return style.Render("Station Agent Chat - Press Ctrl+N to start a conversation")
	}
	
	// Show session info
	leftStyle := lipgloss.NewStyle().
		Foreground(m.theme.Text).
		Bold(true)
	
	rightStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextMuted)
	
	left := leftStyle.Render(fmt.Sprintf("🤖 %s", m.session.AgentName))
	messageCount := len(m.chatMessages)
	right := rightStyle.Render(fmt.Sprintf("%d messages • %s", messageCount, m.session.Model))
	
	headerStyle := lipgloss.NewStyle().
		Width(m.width).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(m.theme.Border).
		Padding(0, 1)
	
	// Calculate spacing
	availableWidth := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if availableWidth < 0 {
		availableWidth = 0
	}
	spacer := strings.Repeat(" ", availableWidth)
	
	return headerStyle.Render(left + spacer + right)
}

// renderStatusBar renders the bottom status bar
func (m *Model) renderStatusBar() string {
	status := "Ready"
	if m.loading {
		status = "Thinking..."
	} else if m.agentsBusy {
		status = "Agent working..."
	} else if m.error != "" {
		status = "Error: " + m.error
	}
	
	leftStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextMuted)
	
	rightStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextMuted)
	
	left := leftStyle.Render(status)
	right := rightStyle.Render("Ctrl+N new • Ctrl+L agents • Ctrl+C quit")
	
	statusStyle := lipgloss.NewStyle().
		Width(m.width).
		Background(m.theme.BackgroundPanel).
		Foreground(m.theme.TextMuted).
		Padding(0, 1)
	
	// Calculate spacing
	availableWidth := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if availableWidth < 0 {
		availableWidth = 0
	}
	spacer := strings.Repeat(" ", availableWidth)
	
	return statusStyle.Render(left + spacer + right)
}

// Message types for communication
type MessageSubmitMsg struct {
	Content string
}

// Helper functions
func (m *Model) loadAgents() tea.Cmd {
	return func() tea.Msg {
		agents, err := m.agentClient.ListAgents(context.Background())
		if err != nil {
			slog.Error("Failed to load agents", "error", err)
			return err
		}
		return AgentListMsg{Agents: agents}
	}
}

func (m *Model) showAgentSelection() tea.Cmd {
	if len(m.availableAgents) == 0 {
		// Load agents first
		return m.loadAgents()
	}
	
	// Show agent selection dialog
	if m.agentDialog == nil {
		m.agentDialog = dialogs.NewAgentSelectionDialog(m.availableAgents)
		m.agentDialog.SetSize(m.width, m.height)
	}
	
	m.agentDialog.Show()
	return nil
}

func (m *Model) startSessionWithAgent(agent Agent) tea.Cmd {
	return func() tea.Msg {
		session, err := m.agentClient.StartChatSession(context.Background(), agent.ID)
		if err != nil {
			slog.Error("Failed to start chat session", "error", err)
			return err
		}
		return SessionStartedMsg{Session: session}
	}
}

func (m *Model) handleUserMessage(content string) tea.Cmd {
	if !m.activeSession || m.session == nil {
		return nil
	}
	
	// Add user message to conversation
	userMsg := components.Message{
		ID:        generateID(),
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	}
	m.chatMessages = append(m.chatMessages, userMsg)
	
	// Update messages display
	m.messages.SetMessages(m.chatMessages)
	
	// Clear editor
	m.editor.Clear()
	
	// Set busy state
	m.agentsBusy = true
	
	// Send to agent
	return m.agentClient.SendMessage(context.Background(), m.session, content)
}

func (m *Model) handleAgentResponse(msg AgentResponseMsg) tea.Cmd {
	m.agentsBusy = false
	
	// Convert agent message to component message
	agentMsg := components.Message{
		ID:        msg.Message.ID,
		Role:      msg.Message.Role,
		Content:   msg.Message.Content,
		AgentName: m.session.AgentName,
		Timestamp: msg.Message.Timestamp,
		Thinking:  msg.Message.Thinking,
	}
	
	// Convert tool calls
	for _, tc := range msg.Message.ToolCalls {
		agentMsg.ToolCalls = append(agentMsg.ToolCalls, components.ToolCall{
			ID:       tc.ID,
			Tool:     tc.Tool,
			Input:    fmt.Sprintf("%v", tc.Input),
			Output:   fmt.Sprintf("%v", tc.Output),
			Status:   tc.Status,
			Duration: tc.Duration,
		})
	}
	
	m.chatMessages = append(m.chatMessages, agentMsg)
	
	// Update messages component
	_, cmd := m.messages.SetMessages(m.chatMessages)
	return cmd
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}