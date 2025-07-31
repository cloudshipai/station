package tabs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"

	"station/internal/tui/styles"
	"station/pkg/models"
)

// AgentItem implements list.Item interface for bubbles list component
type AgentItem struct {
	agent models.Agent
}

// Required by list.Item interface
func (i AgentItem) FilterValue() string {
	return i.agent.Name
}

// Required by list.DefaultItem interface
func (i AgentItem) Title() string {
	status := "●"
	statusStyle := styles.SuccessStyle
	
	// TODO: Get actual agent status from database
	// For now, show all as active
	
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		statusStyle.Render(status+" "),
		styles.BaseStyle.Render(i.agent.Name),
	)
}

func (i AgentItem) Description() string {
	desc := i.agent.Description
	if len(desc) > 40 {
		desc = desc[:40] + "..."
	}
	
	createdAt := i.agent.CreatedAt.Format("Jan 2")
	steps := fmt.Sprintf("max %d steps", i.agent.MaxSteps)
	
	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	return mutedStyle.Render(desc + " • " + createdAt + " • " + steps)
}

// Update list items from agents data
func (m *AgentsModel) updateListItems() {
	items := make([]list.Item, len(m.agents))
	for i, agent := range m.agents {
		items[i] = AgentItem{agent: agent}
	}
	m.list.SetItems(items)
}

// Render tools selection section with scrolling, filtering, and environment info
func (m AgentsModel) renderToolsSelection(label string) string {
	var toolsList []string
	toolsList = append(toolsList, label)
	toolsList = append(toolsList, "")
	
	// Show filter input if in filtering mode
	if m.isFiltering {
		filterLabel := lipgloss.NewStyle().Foreground(styles.Primary).Render("Filter: /" + m.toolsFilter)
		toolsList = append(toolsList, filterLabel)
		toolsList = append(toolsList, "")
	}
	
	if len(m.availableTools) == 0 {
		toolsList = append(toolsList, styles.BaseStyle.Render("No tools available"))
	} else {
		// Filter tools based on search text
		filteredTools := m.getFilteredTools()
		
		if len(filteredTools) == 0 {
			toolsList = append(toolsList, styles.BaseStyle.Render("No tools match filter"))
		} else {
			// Calculate visible range with scrolling
			maxShow := 4  // Reduced to fit in left column properly
			start := m.toolsOffset
			end := start + maxShow
			if end > len(filteredTools) {
				end = len(filteredTools)
			}
			
			// Show scroll indicator at top if needed
			if start > 0 {
				toolsList = append(toolsList, styles.BaseStyle.Render("↑ ... more tools above"))
			}
			
			// Show visible tools
			for i := start; i < end; i++ {
				tool := filteredTools[i]
				
				// Check if tool is selected
				selected := false
				for _, selectedID := range m.selectedToolIDs {
					if selectedID == tool.ID {
						selected = true
						break
					}
				}
				
				prefix := "☐ "
				if selected {
					prefix = "☑ "
				}
				
				// Enhanced display with MCP server, environment, and config info
				serverInfo := fmt.Sprintf("server: %s", tool.ServerName)
				envInfo := fmt.Sprintf("env: %s", tool.EnvironmentName)
				configInfo := fmt.Sprintf("config: %s v%d", tool.ConfigName, tool.ConfigVersion)
				
				toolLine := fmt.Sprintf("%s%s", prefix, tool.Name)
				
				// Highlight current cursor position when in tools field
				// m.toolCursor is the absolute index in filteredTools, i is also absolute index
				isCursorHere := m.focusedField == AgentFieldTools && m.toolCursor == i
				
				if isCursorHere {
					// Full highlighting for selected item - apply style to entire lines with single arrow
					line1 := fmt.Sprintf("▶ %s - %s", toolLine, serverInfo)
					line2 := fmt.Sprintf("     %s | %s", envInfo, configInfo)
					toolsList = append(toolsList, styles.ListItemSelectedStyle.Width(48).Render(line1))
					toolsList = append(toolsList, styles.ListItemSelectedStyle.Width(48).Render(line2))
				} else {
					// Normal display with muted colors for metadata
					line1 := fmt.Sprintf("  %s - %s", toolLine, serverInfo)
					line2 := fmt.Sprintf("     %s | %s", styles.BaseStyle.Foreground(styles.TextMuted).Render(envInfo), styles.BaseStyle.Foreground(styles.TextMuted).Render(configInfo))
					toolsList = append(toolsList, styles.BaseStyle.Render(line1))
					toolsList = append(toolsList, styles.BaseStyle.Render(line2))
				}
			}
			
			// Show scroll indicator at bottom if needed
			if end < len(filteredTools) {
				remaining := len(filteredTools) - end
				toolsList = append(toolsList, styles.BaseStyle.Render(fmt.Sprintf("↓ ... %d more tools below", remaining)))
			}
		}
	}
	
	// Add filter help text
	if !m.isFiltering {
		filterHelp := styles.HelpStyle.Render("Press / to filter tools")
		toolsList = append(toolsList, "")
		toolsList = append(toolsList, filterHelp)
	} else {
		filterHelp := styles.HelpStyle.Render("Press ESC to clear filter")
		toolsList = append(toolsList, "")
		toolsList = append(toolsList, filterHelp)
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, toolsList...)
}

// Get filtered tools based on selected environment and current filter text
func (m AgentsModel) getFilteredTools() []models.MCPToolWithDetails {
	// First filter by selected environments (show tools from all selected environments)
	var envFilteredTools []models.MCPToolWithDetails
	for _, tool := range m.availableTools {
		// Check if tool's environment is in the selected environments list
		for _, selectedEnvID := range m.selectedEnvIDs {
			if tool.EnvironmentID == selectedEnvID {
				envFilteredTools = append(envFilteredTools, tool)
				break
			}
		}
	}
	
	// Then filter by search text if any
	if m.toolsFilter == "" {
		return envFilteredTools
	}
	
	var filtered []models.MCPToolWithDetails
	filterLower := strings.ToLower(m.toolsFilter)
	
	for _, tool := range envFilteredTools {
		// Search in tool name, description, environment name, and config name
		if strings.Contains(strings.ToLower(tool.Name), filterLower) ||
		   strings.Contains(strings.ToLower(tool.Description), filterLower) ||
		   strings.Contains(strings.ToLower(tool.EnvironmentName), filterLower) ||
		   strings.Contains(strings.ToLower(tool.ConfigName), filterLower) {
			filtered = append(filtered, tool)
		}
	}
	
	return filtered
}

// renderEnvironmentSelection renders the multi-environment selection interface
func (m AgentsModel) renderEnvironmentSelection() string {
	envLabel := "Environments:"
	if m.focusedField == AgentFieldEnvironments {
		envLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Environments:")
	}
	
	var envList []string
	envList = append(envList, envLabel)
	envList = append(envList, "")
	
	if len(m.environments) == 0 {
		envList = append(envList, styles.BaseStyle.Render("No environments available"))
	} else {
		// Show environments with checkboxes
		maxShow := 3  // Show up to 3 environments in the form
		start := m.envOffset
		end := start + maxShow
		if end > len(m.environments) {
			end = len(m.environments)
		}
		
		// Show scroll indicator at top if needed
		if start > 0 {
			envList = append(envList, styles.BaseStyle.Render("↑ ... more environments above"))
		}
		
		// Show visible environments
		for i := start; i < end; i++ {
			env := m.environments[i]
			
			// Check if environment is selected
			selected := false
			for _, selectedID := range m.selectedEnvIDs {
				if selectedID == env.ID {
					selected = true
					break
				}
			}
			
			prefix := "☐ "
			if selected {
				prefix = "☑ "
			}
			
			envLine := fmt.Sprintf("%s%s", prefix, env.Name)
			
			// Highlight current cursor position when in environments field
			isCursorHere := m.focusedField == AgentFieldEnvironments && m.envCursor == i
			
			if isCursorHere {
				// Full highlighting for selected item
				envList = append(envList, styles.ListItemSelectedStyle.Width(48).Render("▶ "+envLine))
			} else {
				// Normal display
				envList = append(envList, styles.BaseStyle.Render("  "+envLine))
			}
		}
		
		// Show scroll indicator at bottom if needed
		if end < len(m.environments) {
			remaining := len(m.environments) - end
			envList = append(envList, styles.BaseStyle.Render(fmt.Sprintf("↓ ... %d more environments below", remaining)))
		}
	}
	
	// Add help text
	if m.focusedField == AgentFieldEnvironments {
		helpText := styles.HelpStyle.Render("Press space to toggle selection")
		envList = append(envList, "")
		envList = append(envList, helpText)
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, envList...)
}