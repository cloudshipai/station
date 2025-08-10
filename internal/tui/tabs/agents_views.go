package tabs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"station/internal/tui/components"
	"station/internal/tui/styles"
	"station/pkg/models"
)

// Render agents list view
func (m AgentsModel) renderAgentsList() string {
	// Navigation breadcrumbs
	breadcrumb := m.renderBreadcrumb()
	
	// Header with stats
	header := components.RenderSectionHeader(fmt.Sprintf("Agents (%d total)", len(m.agents)))
	
	// List component
	listView := m.list.View()
	
	// Help text
	helpText := styles.HelpStyle.Render("• enter: view details • n: new agent • d: delete • r: run agent")
	
	var sections []string
	if breadcrumb != "" {
		sections = append(sections, breadcrumb)
	}
	sections = append(sections, header, listView, "", helpText)
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Render breadcrumb navigation
func (m AgentsModel) renderBreadcrumb() string {
	if !m.CanGoBack() {
		return ""
	}
	
	breadcrumbText := m.GetBreadcrumb()
	backText := " (Press ESC to go back)"
	
	breadcrumbStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Padding(0, 1)
	
	backStyle := lipgloss.NewStyle().
		Foreground(styles.Secondary).
		Italic(true)
	
	content := breadcrumbStyle.Render(breadcrumbText) + backStyle.Render(backText)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		"",
	)
}

// Render agent details view
func (m *AgentsModel) renderAgentDetails() string {
	if m.selectedAgent == nil {
		return styles.ErrorStyle.Render("No agent selected")
	}
	
	agent := m.selectedAgent
	
	// Simple single-column layout
	var sections []string
	
	// Header
	primaryStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		styles.HeaderStyle.Render("Agent Details: "),
		primaryStyle.Render(agent.Name),
	)
	sections = append(sections, header)
	sections = append(sections, "")
	
	
	// Basic info
	info := m.renderAgentInfo(agent)
	sections = append(sections, info)
	
	// Agent environments (cross-environment access)
	agentEnvs := m.renderAgentEnvironments(agent)
	sections = append(sections, agentEnvs)
	
	// System prompt
	prompt := m.renderFullPrompt(agent)
	sections = append(sections, prompt)
	
	// Assigned tools
	assignedTools := m.renderAssignedTools()
	sections = append(sections, assignedTools)
	
	// Actions
	actions := m.renderAgentActions()
	sections = append(sections, actions)
	
	// Help instructions
	helpText := styles.HelpStyle.Render("• ↑/↓ or j/k: scroll • ←/→ or h/l: navigate buttons • enter: execute • r: run • d: delete • esc: back")
	backText := styles.HelpStyle.Render("Press ESC to go back to list")
	sections = append(sections, "")
	sections = append(sections, helpText)
	sections = append(sections, backText)
	
	// Simple vertical layout - return directly (viewport causes display issues)
	fullContent := lipgloss.JoinVertical(lipgloss.Left, sections...)
	
	// Manual scrolling implementation
	lines := strings.Split(fullContent, "\n")
	
	// Get terminal height and calculate available space for content
	maxHeight := m.height - 4 // Account for borders and header
	if maxHeight < 10 {
		maxHeight = 10 // Minimum height
	}
	
	// Apply scroll offset
	startLine := m.detailsScrollOffset
	endLine := startLine + maxHeight
	
	if startLine < 0 {
		startLine = 0
		m.detailsScrollOffset = 0
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine >= len(lines) {
		startLine = len(lines) - maxHeight
		if startLine < 0 {
			startLine = 0
		}
		m.detailsScrollOffset = startLine
	}
	
	// Get visible lines
	visibleLines := lines[startLine:endLine]
	return strings.Join(visibleLines, "\n")
}

// Render agent basic information
func (m AgentsModel) renderAgentInfo(agent *models.Agent) string {
	// Compact info display like in the list view
	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	
	// First line: Description
	descLine := agent.Description
	if descLine == "" {
		descLine = "No description"
	}
	
	// Second line: compact details with bullet separators
	createdAt := agent.CreatedAt.Format("Jan 2, 2006")
	detailsLine := lipgloss.JoinHorizontal(
		lipgloss.Left,
		mutedStyle.Render(fmt.Sprintf("ID: %d • ", agent.ID)),
		mutedStyle.Render(fmt.Sprintf("Env: %d • ", agent.EnvironmentID)),
		mutedStyle.Render(fmt.Sprintf("Max steps: %d • ", agent.MaxSteps)),
		mutedStyle.Render(fmt.Sprintf("Created: %s", createdAt)),
	)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		descLine,
		detailsLine,
		"", // Add some spacing
	)
}

// Render full system prompt for right column in details view
func (m AgentsModel) renderFullPrompt(agent *models.Agent) string {
	prompt := agent.Prompt
	if prompt == "" {
		prompt = "No system prompt configured"
	}
	
	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.HeaderStyle.Render("System Prompt:"),
		mutedStyle.Render(prompt),
		"", // Add spacing after prompt
	)
	
	return content
}

// Render assigned tools section
func (m AgentsModel) renderAssignedTools() string {
	var toolsList []string
	
	toolsList = append(toolsList, styles.HeaderStyle.Render("Assigned Tools:"))
	toolsList = append(toolsList, "")
	
	// Filter out orphaned tool assignments (tools that no longer exist in MCP configs)
	var validTools []models.AgentToolWithDetails
	var orphanedCount int
	
	for _, assignedTool := range m.assignedTools {
		// Check if this assigned tool still exists in available tools
		toolExists := false
		for _, availableTool := range m.availableTools {
			if availableTool.Name == assignedTool.ToolName && availableTool.EnvironmentID == assignedTool.EnvironmentID {
				toolExists = true
				break
			}
		}
		
		if toolExists {
			validTools = append(validTools, assignedTool)
		} else {
			orphanedCount++
		}
	}
	
	if len(validTools) == 0 {
		if orphanedCount > 0 {
			toolsList = append(toolsList, styles.BaseStyle.Render(fmt.Sprintf("No valid tools assigned (%d orphaned tools hidden)", orphanedCount)))
		} else {
			toolsList = append(toolsList, styles.BaseStyle.Render("No tools assigned"))
		}
	} else {
		for _, tool := range validTools {
			// Get environment name from tool's environment info
			envInfo := fmt.Sprintf("(Environment: %s)", tool.EnvironmentName)
			
			toolLine := fmt.Sprintf("• %s", tool.ToolName)
			if len(toolLine) > 50 {
				toolLine = toolLine[:50] + "..."
			}
			
			mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
			content := lipgloss.JoinVertical(
				lipgloss.Left,
				styles.BaseStyle.Render(toolLine),
				mutedStyle.Render("  "+envInfo),
			)
			toolsList = append(toolsList, content)
		}
		
		// Show warning about orphaned tools if any
		if orphanedCount > 0 {
			toolsList = append(toolsList, "")
			warningMsg := fmt.Sprintf("⚠️  %d orphaned tool assignment(s) hidden (tools no longer exist)", orphanedCount)
			toolsList = append(toolsList, styles.BaseStyle.Foreground(styles.Secondary).Render(warningMsg))
		}
	}
	
	content := lipgloss.JoinVertical(lipgloss.Left, toolsList...)
	
	return content
}

// Render agent action buttons
func (m AgentsModel) renderAgentActions() string {
	// Style buttons based on selection
	var runBtn, editBtn, deleteBtn string
	
	if m.actionButtonIndex == 0 {
		runBtn = styles.ButtonActiveStyle.Render("▶ Run Agent")
	} else {
		runBtn = styles.ButtonStyle.Render("Run Agent")
	}
	
	if m.actionButtonIndex == 1 {
		editBtn = styles.ButtonActiveStyle.Render("▶ Edit")
	} else {
		editBtn = styles.ButtonStyle.Render("Edit")
	}
	
	if m.actionButtonIndex == 2 {
		deleteBtn = styles.ButtonActiveStyle.Render("▶ Delete")
	} else {
		deleteBtn = styles.ErrorStyle.Render("Delete")
	}
	
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		runBtn, "  ",
		editBtn, "  ",
		deleteBtn,
	)
}

// Render create agent form with two-column layout
func (m AgentsModel) renderCreateAgentForm() string {
	// Header
	header := components.RenderSectionHeader("Create New Agent")
	
	// LEFT COLUMN - Basic fields and tools
	var leftColumn []string
	
	// Name field
	nameLabel := "Name:"
	if m.focusedField == AgentFieldName {
		nameLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Name:")
	}
	nameSection := lipgloss.JoinVertical(lipgloss.Left, nameLabel, m.nameInput.View())
	leftColumn = append(leftColumn, nameSection)
	leftColumn = append(leftColumn, "")
	
	// Description field
	descLabel := "Description:"
	if m.focusedField == AgentFieldDesc {
		descLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Description:")
	}
	descSection := lipgloss.JoinVertical(lipgloss.Left, descLabel, m.descInput.View())
	leftColumn = append(leftColumn, descSection)
	leftColumn = append(leftColumn, "")
	
	// Environment selection (multi-select)
	envSection := m.renderEnvironmentSelection()
	leftColumn = append(leftColumn, envSection)
	leftColumn = append(leftColumn, "")
	
	// Schedule enabled checkbox
	scheduleEnabledLabel := "Schedule Enabled:"
	if m.focusedField == AgentFieldScheduleEnabled {
		scheduleEnabledLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Schedule Enabled:")
	}
	scheduleEnabledValue := "☐ No"
	if m.scheduleEnabled {
		scheduleEnabledValue = "☑ Yes"
	}
	scheduleEnabledSection := lipgloss.JoinVertical(lipgloss.Left, scheduleEnabledLabel, styles.BaseStyle.Render("▶ "+scheduleEnabledValue))
	leftColumn = append(leftColumn, scheduleEnabledSection)
	leftColumn = append(leftColumn, "")
	
	// Cron schedule input (only show if scheduling is enabled)
	if m.scheduleEnabled {
		scheduleLabel := "Cron Schedule:"
		if m.focusedField == AgentFieldSchedule {
			scheduleLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Cron Schedule:")
		}
		scheduleSection := lipgloss.JoinVertical(lipgloss.Left, scheduleLabel, m.scheduleInput.View())
		leftColumn = append(leftColumn, scheduleSection)
		leftColumn = append(leftColumn, "")
	}
	
	// Tools selection
	toolsLabel := "Available Tools:"
	if m.focusedField == AgentFieldTools {
		toolsLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Available Tools:")
	}
	toolsSection := m.renderToolsSelection(toolsLabel)
	leftColumn = append(leftColumn, toolsSection)
	
	// RIGHT COLUMN - System prompt (takes up majority of space)
	var rightColumn []string
	
	// System prompt
	promptLabel := "System Prompt:"
	if m.focusedField == AgentFieldPrompt {
		promptLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("System Prompt:")
	}
	promptSection := lipgloss.JoinVertical(lipgloss.Left, promptLabel, m.promptArea.View())
	rightColumn = append(rightColumn, promptSection)
	
	// Combine columns
	leftColumnContent := lipgloss.JoinVertical(lipgloss.Left, leftColumn...)
	rightColumnContent := lipgloss.JoinVertical(lipgloss.Left, rightColumn...)
	
	// Style columns with appropriate widths
	leftColumnStyled := lipgloss.NewStyle().
		Width(50).  // Left column: ~50 chars
		Render(leftColumnContent)
	
	rightColumnStyled := lipgloss.NewStyle().
		Width(85).  // Right column: ~85 chars (majority of space)
		Render(rightColumnContent)
	
	// Join columns horizontally
	columnsContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftColumnStyled,
		"  ", // Small gap between columns
		rightColumnStyled,
	)
	
	// Help text
	helpText := styles.HelpStyle.Render("• tab: next field • ↑/↓: navigate • space: select tools • ctrl+s: save • esc: auto-save & exit")
	
	// Combine all sections
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		columnsContent,
		"",
		helpText,
	)
}

// Render edit agent form with two-column layout (same as create but with different header)
func (m AgentsModel) renderEditAgentForm() string {
	// Header
	header := components.RenderSectionHeader("Edit Agent")
	
	// LEFT COLUMN - Basic fields and tools
	var leftColumn []string
	
	// Name field
	nameLabel := "Name:"
	if m.focusedField == AgentFieldName {
		nameLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Name:")
	}
	nameSection := lipgloss.JoinVertical(lipgloss.Left, nameLabel, m.nameInput.View())
	leftColumn = append(leftColumn, nameSection)
	leftColumn = append(leftColumn, "")
	
	// Description field
	descLabel := "Description:"
	if m.focusedField == AgentFieldDesc {
		descLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Description:")
	}
	descSection := lipgloss.JoinVertical(lipgloss.Left, descLabel, m.descInput.View())
	leftColumn = append(leftColumn, descSection)
	leftColumn = append(leftColumn, "")
	
	// Environment selection (multi-select)
	envSection := m.renderEnvironmentSelection()
	leftColumn = append(leftColumn, envSection)
	leftColumn = append(leftColumn, "")
	
	// Schedule enabled checkbox
	scheduleEnabledLabel := "Schedule Enabled:"
	if m.focusedField == AgentFieldScheduleEnabled {
		scheduleEnabledLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Schedule Enabled:")
	}
	scheduleEnabledValue := "☐ No"
	if m.scheduleEnabled {
		scheduleEnabledValue = "☑ Yes"
	}
	scheduleEnabledSection := lipgloss.JoinVertical(lipgloss.Left, scheduleEnabledLabel, styles.BaseStyle.Render("▶ "+scheduleEnabledValue))
	leftColumn = append(leftColumn, scheduleEnabledSection)
	leftColumn = append(leftColumn, "")
	
	// Cron schedule input (only show if scheduling is enabled)
	if m.scheduleEnabled {
		scheduleLabel := "Cron Schedule:"
		if m.focusedField == AgentFieldSchedule {
			scheduleLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Cron Schedule:")
		}
		scheduleSection := lipgloss.JoinVertical(lipgloss.Left, scheduleLabel, m.scheduleInput.View())
		leftColumn = append(leftColumn, scheduleSection)
		leftColumn = append(leftColumn, "")
	}
	
	// Tools selection
	toolsLabel := "Available Tools:"
	if m.focusedField == AgentFieldTools {
		toolsLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Available Tools:")
	}
	toolsSection := m.renderToolsSelection(toolsLabel)
	leftColumn = append(leftColumn, toolsSection)
	
	// RIGHT COLUMN - System prompt (takes up majority of space)
	var rightColumn []string
	
	// System prompt
	promptLabel := "System Prompt:"
	if m.focusedField == AgentFieldPrompt {
		promptLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("System Prompt:")
	}
	promptSection := lipgloss.JoinVertical(lipgloss.Left, promptLabel, m.promptArea.View())
	rightColumn = append(rightColumn, promptSection)
	
	// Combine columns
	leftColumnContent := lipgloss.JoinVertical(lipgloss.Left, leftColumn...)
	rightColumnContent := lipgloss.JoinVertical(lipgloss.Left, rightColumn...)
	
	// Style columns with appropriate widths
	leftColumnStyled := lipgloss.NewStyle().
		Width(50).  // Left column: ~50 chars
		Render(leftColumnContent)
	
	rightColumnStyled := lipgloss.NewStyle().
		Width(85).  // Right column: ~85 chars (majority of space)
		Render(rightColumnContent)
	
	// Join columns horizontally
	columnsContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftColumnStyled,
		"  ", // Small gap between columns
		rightColumnStyled,
	)
	
	// Help text (different for edit)
	helpText := styles.HelpStyle.Render("• tab: next field • ↑/↓: navigate • space: select tools • ctrl+s: update • esc: auto-save & exit")
	
	// Combine all sections
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		columnsContent,
		"",
		helpText,
	)
}

// renderAgentEnvironments shows the environment this agent belongs to
func (m AgentsModel) renderAgentEnvironments(agent *models.Agent) string {
	var envList []string
	
	envList = append(envList, styles.HeaderStyle.Render("Environment:"))
	envList = append(envList, "")
	
	// Find the environment name for this agent
	envName := "Unknown"
	for _, env := range m.environments {
		if env.ID == agent.EnvironmentID {
			envName = env.Name
			break
		}
	}
	
	envList = append(envList, fmt.Sprintf("• %s (ID: %d)", envName, agent.EnvironmentID))
	envList = append(envList, "")
	
	// Create muted text style for help text
	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	envList = append(envList, mutedStyle.Render("Agents are environment-specific and can only"))
	envList = append(envList, mutedStyle.Render("access tools from their assigned environment."))
	
	return lipgloss.JoinVertical(lipgloss.Left, envList...)
}