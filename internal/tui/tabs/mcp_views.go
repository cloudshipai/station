package tabs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	
	"station/internal/tui/components"
	"station/internal/tui/styles"
)

// View renders the MCP tab
func (m MCPModel) View() string {
	if m.IsLoading() {
		return components.RenderLoadingIndicator(0)
	}
	
	if m.showEditor {
		return m.renderEditor()
	}
	
	return m.renderConfigList()
}

// Render configuration list
func (m MCPModel) renderConfigList() string {
	// Show breadcrumb if available (though main view doesn't need it)
	breadcrumb := ""
	if !m.IsMainView() {
		breadcrumb = "MCP Servers › Configuration List"
	}
	
	header := components.RenderSectionHeader("MCP Server Configurations")
	
	// Config list
	var configItems []string
	configItems = append(configItems, styles.HeaderStyle.Render("Available Configurations:"))
	configItems = append(configItems, "")
	
	for i, config := range m.configs {
		prefix := "  "
		if i == m.selectedIdx {
			prefix = "▶ "
		}
		
		// Create status indicator
		statusIndicator := m.getStatusIndicator(config.ToolStatus)
		toolInfo := ""
		if config.ToolCount > 0 {
			toolInfo = fmt.Sprintf(" (%d tools)", config.ToolCount)
		}
		
		configLine := fmt.Sprintf("%s%s %s [%s] (v%d) - Updated %s - Size %s%s", 
			prefix, statusIndicator, config.Name, config.EnvironmentName, 
			config.Version, config.Updated, config.Size, toolInfo)
		configItems = append(configItems, configLine)
	}
	
	configList := strings.Join(configItems, "\n")
	
	helpText := styles.HelpStyle.Render("• ↑/↓/j/k: navigate • n: new config • enter: edit • d: delete selected • r: refresh tools\n• Status: 🟢 success • 🔴 failed • 🟡 partial • ⚪ unknown")
	
	var sections []string
	if breadcrumb != "" {
		sections = append(sections, breadcrumb, "")
	}
	sections = append(sections, header, "", configList, "", "", helpText)
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Render configuration editor - compact like dashboard
func (m MCPModel) renderEditor() string {
	var sections []string
	
	// Header
	header := components.RenderSectionHeader("MCP Configuration Editor")
	sections = append(sections, header)
	
	// Show error if there is one
	if m.GetError() != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Background(lipgloss.Color("#331111")).
			Padding(1).
			Margin(1).
			Bold(true)
		sections = append(sections, errorStyle.Render("❌ "+m.GetError()))
		sections = append(sections, "")
	}
	
	// Compact form layout - everything inline like dashboard stats  
	nameLabel := "Name:"
	envLabel := "Environment:"
	if m.focusedField == MCPFieldName {
		nameLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Name:")
	}
	if m.focusedField == MCPFieldEnvironment {
		envLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Environment:")
	}
	
	// Compact name and environment section
	nameSection := lipgloss.JoinHorizontal(lipgloss.Top, 
		nameLabel, " ", m.nameInput.View())
	
	// Environment selection - simple inline
	envName := "default"
	if len(m.envs) > 0 {
		for _, env := range m.envs {
			if env.ID == m.selectedEnvID {
				envName = env.Name
				break
			}
		}
	}
	envSection := lipgloss.JoinHorizontal(lipgloss.Top,
		envLabel, " ", envName)
	
	sections = append(sections, nameSection)
	sections = append(sections, envSection)
	sections = append(sections, "")
	
	// Version history section - only show if we have versions loaded
	if len(m.configVersions) > 0 {
		versionsLabel := "Versions:"
		if m.focusedField == MCPFieldVersions {
			versionsLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Versions:")
		}
		
		sections = append(sections, versionsLabel)
		
		// Show compact version list
		var versionItems []string
		for i, version := range m.configVersions {
			prefix := "  "
			if i == m.selectedVersionIdx {
				if m.focusedField == MCPFieldVersions {
					prefix = "▶ "
				} else {
					prefix = "• "
				}
			}
			versionLine := fmt.Sprintf("v%d - %s (%s)", version.Version, version.Updated, version.Size)
			versionItems = append(versionItems, prefix+versionLine)
		}
		
		// Limit to max 4 lines to save space, scroll if needed
		maxLines := 4
		startIdx := 0
		if len(versionItems) > maxLines && m.selectedVersionIdx >= maxLines/2 {
			startIdx = m.selectedVersionIdx - maxLines/2
			if startIdx+maxLines > len(versionItems) {
				startIdx = len(versionItems) - maxLines
			}
		}
		
		endIdx := startIdx + maxLines
		if endIdx > len(versionItems) {
			endIdx = len(versionItems)
		}
		
		for i := startIdx; i < endIdx; i++ {
			sections = append(sections, versionItems[i])
		}
		
		if len(versionItems) > maxLines {
			scrollInfo := fmt.Sprintf("    (%d more versions - use ↑/↓ when focused on versions)", len(versionItems)-maxLines)
			sections = append(sections, styles.HelpStyle.Render(scrollInfo))
		}
		
		sections = append(sections, "")
	}
	
	// Config editor - simple and compact
	configLabel := "Configuration:"
	if m.focusedField == MCPFieldConfig {
		configLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Configuration:")
	}
	
	// Set reasonable size for config editor
	m.configEditor.SetWidth(m.width - 4)
	m.configEditor.SetHeight(m.height - 10) // Simple calculation like dashboard
	
	sections = append(sections, configLabel)
	sections = append(sections, m.configEditor.View())
	
	// Simple help text
	helpText := styles.HelpStyle.Render("tab: switch • enter: save/load version • ctrl+s: save • ctrl+r: refresh tools • esc: auto-save & exit")
	sections = append(sections, "")
	sections = append(sections, helpText)
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// getStatusIndicator returns a colored circle indicating tool extraction status
func (m MCPModel) getStatusIndicator(status ToolExtractionStatus) string {
	switch status {
	case ToolStatusSuccess:
		// Green circle
		return lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("●")
	case ToolStatusFailed:
		// Red circle
		return lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("●")
	case ToolStatusPartial:
		// Yellow circle
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("●")
	case ToolStatusUnknown:
		// Gray circle
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("●")
	default:
		// Default gray circle
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("●")
	}
}