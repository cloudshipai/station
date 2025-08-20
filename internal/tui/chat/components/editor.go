package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"station/internal/tui/theme"
)

// EditorComponent handles user input
type EditorComponent interface {
	tea.Model
	SetWidth(width int)
	Clear()
	Focus() tea.Cmd
	Blur()
	Value() string
	SetValue(string)
	View() string
}

type editorComponent struct {
	width    int
	textarea textarea.Model
	theme    *theme.Theme
	focused  bool
}

// NewEditorComponent creates a new editor component
func NewEditorComponent(width int) EditorComponent {
	ta := textarea.New()
	ta.Placeholder = "Type your message here... (Enter to send, Shift+Enter for new line)"
	ta.SetWidth(width - 4) // Account for borders and padding
	ta.SetHeight(3)        // Multi-line editor
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false
	ta.CharLimit = 2000 // Reasonable message limit
	
	return &editorComponent{
		width:    width,
		textarea: ta,
		theme:    theme.DefaultTheme(),
		focused:  false,
	}
}

// Init initializes the editor component
func (e *editorComponent) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles messages
func (e *editorComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		e.width = msg.Width
		e.textarea.SetWidth(e.width - 4)
		
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Check if Shift is held for new line
			if len(msg.Alt) > 0 || strings.Contains(msg.String(), "shift") {
				// Shift+Enter - add new line
				e.textarea, cmd = e.textarea.Update(msg)
				return e, cmd
			}
			
			// Regular Enter - submit message
			content := strings.TrimSpace(e.textarea.Value())
			if content != "" {
				// Clear the editor
				e.textarea.Reset()
				
				// Send submit message
				return e, func() tea.Msg {
					return MessageSubmitMsg{Content: content}
				}
			}
			return e, nil
			
		case "tab":
			// Handle autocomplete in the future
			return e, nil
			
		case "ctrl+a":
			// Select all text
			e.textarea.SelectAll()
			return e, nil
			
		case "ctrl+u":
			// Clear line
			e.Clear()
			return e, nil
		}
	}
	
	e.textarea, cmd = e.textarea.Update(msg)
	return e, cmd
}

// View renders the editor
func (e *editorComponent) View() string {
	// Create editor container with border
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(e.theme.Border).
		Width(e.width).
		Padding(1)
		
	if e.focused {
		borderStyle = borderStyle.BorderForeground(e.theme.Primary)
	}
	
	// Add prompt indicator
	promptStyle := lipgloss.NewStyle().
		Foreground(e.theme.Primary).
		Bold(true)
	
	prompt := promptStyle.Render("> ")
	
	// Combine prompt and textarea
	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		prompt,
		e.textarea.View(),
	)
	
	return borderStyle.Render(content)
}

// SetWidth updates the editor width
func (e *editorComponent) SetWidth(width int) {
	e.width = width
	e.textarea.SetWidth(width - 6) // Account for borders, padding, and prompt
}

// Clear clears the editor content
func (e *editorComponent) Clear() {
	e.textarea.Reset()
}

// Focus focuses the editor
func (e *editorComponent) Focus() tea.Cmd {
	e.focused = true
	return e.textarea.Focus()
}

// Blur blurs the editor
func (e *editorComponent) Blur() {
	e.focused = false
	e.textarea.Blur()
}

// Value returns the current editor value
func (e *editorComponent) Value() string {
	return e.textarea.Value()
}

// SetValue sets the editor value
func (e *editorComponent) SetValue(value string) {
	e.textarea.SetValue(value)
}

// MessageSubmitMsg is sent when the user submits a message
type MessageSubmitMsg struct {
	Content string
}