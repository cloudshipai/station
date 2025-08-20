package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"station/internal/tui/theme"
)

// Message represents a chat message
type Message struct {
	ID        string
	Role      string // "user", "agent", "system"
	Content   string
	AgentName string
	Timestamp time.Time
	Thinking  string
	ToolCalls []ToolCall
}

type ToolCall struct {
	ID       string
	Tool     string
	Input    string
	Output   string
	Status   string
	Duration time.Duration
}

// MessagesComponent handles message display
type MessagesComponent interface {
	tea.Model
	SetSize(width, height int)
	SetMessages(messages []Message) (tea.Model, tea.Cmd)
	ScrollToBottom()
	View() string
}

type messagesComponent struct {
	width    int
	height   int
	viewport viewport.Model
	messages []Message
	theme    *theme.Theme
}

// NewMessagesComponent creates a new messages component
func NewMessagesComponent(width, height int) MessagesComponent {
	vp := viewport.New(width, height)
	
	return &messagesComponent{
		width:    width,
		height:   height,
		viewport: vp,
		messages: make([]Message, 0),
		theme:    theme.DefaultTheme(),
	}
}

// Init initializes the messages component
func (m *messagesComponent) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *messagesComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = m.width
		m.viewport.Height = m.height
		return m, m.renderMessages()
		
	case tea.KeyMsg:
		// Handle scroll keys
		switch msg.String() {
		case "up", "k":
			m.viewport.LineUp(1)
		case "down", "j":
			m.viewport.LineDown(1)
		case "pgup":
			m.viewport.ViewUp()
		case "pgdown":
			m.viewport.ViewDown()
		case "home", "g":
			m.viewport.GotoTop()
		case "end", "G":
			m.viewport.GotoBottom()
		}
		
	case SetMessagesMsg:
		m.messages = msg.Messages
		return m, m.renderMessages()
	}
	
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the messages
func (m *messagesComponent) View() string {
	if len(m.messages) == 0 {
		// Show welcome message
		welcomeStyle := lipgloss.NewStyle().
			Foreground(m.theme.TextMuted).
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center)
			
		return welcomeStyle.Render("Start a conversation with an agent to see messages here.\nPress Ctrl+N to begin.")
	}
	
	return m.viewport.View()
}

// SetSize updates the component size
func (m *messagesComponent) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width
	m.viewport.Height = height
}

// SetMessages updates the messages and re-renders
func (m *messagesComponent) SetMessages(messages []Message) (tea.Model, tea.Cmd) {
	m.messages = messages
	return m, m.renderMessages()
}

// ScrollToBottom scrolls to the latest message
func (m *messagesComponent) ScrollToBottom() {
	m.viewport.GotoBottom()
}

// renderMessages renders all messages to the viewport
func (m *messagesComponent) renderMessages() tea.Cmd {
	return func() tea.Msg {
		var content []string
		
		for _, msg := range m.messages {
			content = append(content, m.renderMessage(msg))
			content = append(content, "") // Add spacing between messages
		}
		
		finalContent := strings.Join(content, "\n")
		m.viewport.SetContent(finalContent)
		m.viewport.GotoBottom() // Auto-scroll to latest message
		
		return nil
	}
}

// renderMessage renders a single message
func (m *messagesComponent) renderMessage(msg Message) string {
	switch msg.Role {
	case "user":
		return m.renderUserMessage(msg)
	case "agent":
		return m.renderAgentMessage(msg)
	case "system":
		return m.renderSystemMessage(msg)
	default:
		return ""
	}
}

// renderUserMessage renders a user message
func (m *messagesComponent) renderUserMessage(msg Message) string {
	// User message style - aligned right, different color
	contentStyle := lipgloss.NewStyle().
		Foreground(m.theme.Text).
		Background(m.theme.Primary).
		Padding(1, 2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Primary).
		MaxWidth(m.width - 20)
	
	timestampStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextMuted).
		Align(lipgloss.Right)
	
	content := contentStyle.Render(msg.Content)
	timestamp := timestampStyle.Render(msg.Timestamp.Format("15:04"))
	
	// Right-align the entire message block
	messageBlock := lipgloss.JoinVertical(lipgloss.Right, content, timestamp)
	
	return lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Right).
		Render(messageBlock)
}

// renderAgentMessage renders an agent message
func (m *messagesComponent) renderAgentMessage(msg Message) string {
	var blocks []string
	
	// Agent name and timestamp header
	headerStyle := lipgloss.NewStyle().
		Foreground(m.theme.Secondary).
		Bold(true)
	
	timestampStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextMuted)
	
	agentName := msg.AgentName
	if agentName == "" {
		agentName = "Agent"
	}
	
	header := headerStyle.Render("🤖 " + agentName) + "  " + 
			 timestampStyle.Render(msg.Timestamp.Format("15:04"))
	blocks = append(blocks, header)
	
	// Show thinking content if present
	if msg.Thinking != "" {
		thinkingStyle := lipgloss.NewStyle().
			Foreground(m.theme.TextMuted).
			Background(m.theme.BackgroundPanel).
			Padding(1, 2).
			BorderLeft(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(m.theme.Secondary).
			MaxWidth(m.width - 10).
			Italic(true)
		
		thinkingBlock := thinkingStyle.Render("💭 " + msg.Thinking)
		blocks = append(blocks, thinkingBlock)
	}
	
	// Main content
	contentStyle := lipgloss.NewStyle().
		Foreground(m.theme.Text).
		Background(m.theme.BackgroundElement).
		Padding(1, 2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Border).
		MaxWidth(m.width - 10)
	
	content := contentStyle.Render(msg.Content)
	blocks = append(blocks, content)
	
	// Tool calls if present
	for _, toolCall := range msg.ToolCalls {
		toolBlock := m.renderToolCall(toolCall)
		blocks = append(blocks, toolBlock)
	}
	
	return strings.Join(blocks, "\n")
}

// renderSystemMessage renders a system message
func (m *messagesComponent) renderSystemMessage(msg Message) string {
	style := lipgloss.NewStyle().
		Foreground(m.theme.TextMuted).
		Background(m.theme.BackgroundPanel).
		Padding(0, 2).
		Width(m.width).
		Align(lipgloss.Center).
		BorderStyle(lipgloss.HiddenBorder())
	
	return style.Render("• " + msg.Content + " •")
}

// renderToolCall renders a tool execution
func (m *messagesComponent) renderToolCall(toolCall ToolCall) string {
	var statusIcon string
	var statusColor lipgloss.Color
	
	switch toolCall.Status {
	case "running":
		statusIcon = "🔄"
		statusColor = m.theme.Secondary
	case "completed":
		statusIcon = "✅"
		statusColor = m.theme.Success
	case "error":
		statusIcon = "❌"
		statusColor = m.theme.Error
	default:
		statusIcon = "⏳"
		statusColor = m.theme.TextMuted
	}
	
	headerStyle := lipgloss.NewStyle().
		Foreground(statusColor).
		Bold(true)
	
	detailStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextMuted)
	
	contentStyle := lipgloss.NewStyle().
		Foreground(m.theme.Text).
		Background(m.theme.BackgroundPanel).
		Padding(1, 2).
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(statusColor).
		MaxWidth(m.width - 15)
	
	header := headerStyle.Render(fmt.Sprintf("%s %s", statusIcon, toolCall.Tool))
	if toolCall.Duration > 0 {
		header += "  " + detailStyle.Render(fmt.Sprintf("(%v)", toolCall.Duration))
	}
	
	var content []string
	
	if toolCall.Input != "" {
		content = append(content, "Input: "+toolCall.Input)
	}
	
	if toolCall.Output != "" {
		content = append(content, "Output: "+toolCall.Output)
	}
	
	if len(content) == 0 {
		content = append(content, "Executing...")
	}
	
	toolContent := contentStyle.Render(strings.Join(content, "\n"))
	
	return header + "\n" + toolContent
}

// SetMessagesMsg is sent to update the messages
type SetMessagesMsg struct {
	Messages []Message
}