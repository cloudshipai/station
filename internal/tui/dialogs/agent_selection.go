package dialogs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"station/internal/tui/theme"
	"station/internal/tui/types"
)

// AgentSelectionDialog displays a list of available agents for selection
type AgentSelectionDialog struct {
	width   int
	height  int
	list    list.Model
	theme   *theme.Theme
	visible bool
}

// agentItem represents an agent in the list
type agentItem struct {
	agent types.Agent
}

func (i agentItem) FilterValue() string { return i.agent.Name }
func (i agentItem) Title() string       { return i.agent.Name }
func (i agentItem) Description() string {
	status := "Enabled"
	if !i.agent.Enabled {
		status = "Disabled"
	}
	return fmt.Sprintf("%s • %s • %d tools", i.agent.Description, status, len(i.agent.Tools))
}

// NewAgentSelectionDialog creates a new agent selection dialog
func NewAgentSelectionDialog(agents []types.Agent) *AgentSelectionDialog {
	items := make([]list.Item, len(agents))
	for i, agent := range agents {
		items[i] = agentItem{agent: agent}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select an Agent"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(theme.DefaultTheme().Primary).
		Bold(true).
		Padding(0, 0, 1, 2)

	return &AgentSelectionDialog{
		list:    l,
		theme:   theme.DefaultTheme(),
		visible: false,
	}
}

// Show displays the dialog
func (d *AgentSelectionDialog) Show() {
	d.visible = true
}

// Hide hides the dialog
func (d *AgentSelectionDialog) Hide() {
	d.visible = false
}

// IsVisible returns whether the dialog is visible
func (d *AgentSelectionDialog) IsVisible() bool {
	return d.visible
}

// SetSize sets the dialog dimensions
func (d *AgentSelectionDialog) SetSize(width, height int) {
	d.width = width
	d.height = height
	
	// Make dialog take up most of the screen but not full
	dialogWidth := width - 10
	dialogHeight := height - 6
	
	if dialogWidth < 40 {
		dialogWidth = 40
	}
	if dialogHeight < 10 {
		dialogHeight = 10
	}
	
	d.list.SetWidth(dialogWidth)
	d.list.SetHeight(dialogHeight)
}

// Init initializes the dialog
func (d *AgentSelectionDialog) Init() tea.Cmd {
	return nil
}

// Update handles input and updates the dialog
func (d *AgentSelectionDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !d.visible {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Agent selected
			if selectedItem, ok := d.list.SelectedItem().(agentItem); ok {
				d.Hide()
				return d, func() tea.Msg {
					return AgentSelectedMsg{Agent: selectedItem.agent}
				}
			}
		case "esc", "q":
			// Cancel selection
			d.Hide()
			return d, nil
		}
	case tea.WindowSizeMsg:
		d.SetSize(msg.Width, msg.Height)
	}

	var cmd tea.Cmd
	d.list, cmd = d.list.Update(msg)
	return d, cmd
}

// View renders the dialog
func (d *AgentSelectionDialog) View() string {
	if !d.visible {
		return ""
	}

	// Create the dialog content
	content := d.list.View()
	
	// Add instructions
	instructions := lipgloss.NewStyle().
		Foreground(d.theme.TextMuted).
		Padding(1, 2).
		Render("↑/↓: navigate • Enter: select • Esc: cancel")
	
	dialogContent := lipgloss.JoinVertical(lipgloss.Left, content, instructions)
	
	// Style the dialog box
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.theme.Primary).
		Background(d.theme.BackgroundElement).
		Padding(1).
		Width(d.width - 8).
		Height(d.height - 4)
	
	dialog := dialogStyle.Render(dialogContent)
	
	// Center the dialog on screen
	return lipgloss.Place(
		d.width,
		d.height,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
		lipgloss.WithWhitespaceBackground(d.theme.Background),
	)
}

// UpdateAgents updates the list of available agents
func (d *AgentSelectionDialog) UpdateAgents(agents []types.Agent) {
	items := make([]list.Item, len(agents))
	for i, agent := range agents {
		items[i] = agentItem{agent: agent}
	}
	d.list.SetItems(items)
}

// AgentSelectedMsg is sent when an agent is selected
type AgentSelectedMsg struct {
	Agent types.Agent
}