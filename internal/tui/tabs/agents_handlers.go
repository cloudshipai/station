package tabs

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Handle key events in create form
func (m *AgentsModel) handleCreateFormKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd
	
	// Handle filter mode when in tools section first
	if m.focusedField == AgentFieldTools && m.isFiltering {
		switch msg.String() {
		case "esc":
			// Exit filter mode
			m.isFiltering = false
			m.toolsFilter = ""
			m.toolCursor = 0 // Reset cursor when exiting filter
			return m, nil
		case "backspace":
			// Remove last character from filter
			if len(m.toolsFilter) > 0 {
				m.toolsFilter = m.toolsFilter[:len(m.toolsFilter)-1]
				m.toolCursor = 0 // Reset cursor when filter changes
			}
			return m, nil
		default:
			// Add character to filter (only printable characters)
			if len(msg.String()) == 1 && msg.String() >= " " && msg.String() <= "~" {
				m.toolsFilter += msg.String()
				m.toolCursor = 0 // Reset cursor when filter changes
				return m, nil
			}
		}
	}
	
	switch msg.String() {
	case "esc":
		// Auto-save if form has content, then go back to list
		nameValue := m.nameInput.Value()
		descValue := m.descInput.Value()
		
		if nameValue != "" || descValue != "" {
			// Save the agent before exiting
			cmd := m.createAgent()
			m.SetViewMode("list")
			m.GoBack()
			m.resetCreateForm()
			return m, cmd
		}
		
		// No content to save, just cancel and go back
		m.SetViewMode("list")
		m.GoBack()
		m.resetCreateForm()
		return m, nil
		
	case "tab":
		// Cycle through form fields
		m.cycleFocusedField()
		return m, nil
		
	case "ctrl+s":
		// Save agent
		return m, m.createAgent()
		
	case "up", "k":
		if m.focusedField == AgentFieldEnvironments {
			// Navigate environment selection up
			if len(m.environments) > 0 && m.envCursor > 0 {
				m.envCursor--
				// Update scroll offset if cursor goes above visible area
				if m.envCursor < m.envOffset {
					m.envOffset = m.envCursor
				}
			}
			return m, nil
		} else if m.focusedField == AgentFieldTools {
			// Navigate tool selection up (work with filtered tools)
			filteredTools := m.getFilteredTools()
			if len(filteredTools) > 0 && m.toolCursor > 0 {
				m.toolCursor--
				// Update scroll offset if cursor goes above visible area
				if m.toolCursor < m.toolsOffset {
					m.toolsOffset = m.toolCursor
				}
			}
			return m, nil
		}
		
	case "down", "j":
		if m.focusedField == AgentFieldEnvironments {
			// Navigate environment selection down
			if len(m.environments) > 0 && m.envCursor < len(m.environments)-1 {
				m.envCursor++
				// Update scroll offset if cursor goes below visible area
				maxShow := 3  // Must match the display maxShow
				if m.envCursor >= m.envOffset + maxShow {
					m.envOffset = m.envCursor - maxShow + 1
				}
			}
			return m, nil
		} else if m.focusedField == AgentFieldTools {
			// Navigate tool selection down (work with filtered tools)
			filteredTools := m.getFilteredTools()
			if len(filteredTools) > 0 && m.toolCursor < len(filteredTools)-1 {
				m.toolCursor++
				// Update scroll offset if cursor goes below visible area
				maxShow := 4  // Must match the display maxShow
				if m.toolCursor >= m.toolsOffset + maxShow {
					m.toolsOffset = m.toolCursor - maxShow + 1
				}
			}
			return m, nil
		}
		
	case " ", "enter":
		if m.focusedField == AgentFieldEnvironments {
			// Toggle environment selection
			if len(m.environments) > 0 && m.envCursor < len(m.environments) {
				envID := m.environments[m.envCursor].ID
				
				// Check if environment is already selected
				found := false
				for i, selectedID := range m.selectedEnvIDs {
					if selectedID == envID {
						// Remove from selection
						m.selectedEnvIDs = append(m.selectedEnvIDs[:i], m.selectedEnvIDs[i+1:]...)
						found = true
						break
					}
				}
				
				// If not found, add to selection
				if !found {
					m.selectedEnvIDs = append(m.selectedEnvIDs, envID)
				}
			}
			return m, nil
		} else if m.focusedField == AgentFieldScheduleEnabled {
			// Toggle schedule enabled
			m.scheduleEnabled = !m.scheduleEnabled
			// Clear schedule input if disabling
			if !m.scheduleEnabled {
				m.scheduleInput.SetValue("")
			}
			return m, nil
		} else if m.focusedField == AgentFieldTools {
			// Toggle tool selection (work with filtered tools)
			filteredTools := m.getFilteredTools()
			if len(filteredTools) > 0 && m.toolCursor < len(filteredTools) {
				toolID := filteredTools[m.toolCursor].ID
				
				// Check if tool is already selected
				found := false
				for i, selectedID := range m.selectedToolIDs {
					if selectedID == toolID {
						// Remove from selection
						m.selectedToolIDs = append(m.selectedToolIDs[:i], m.selectedToolIDs[i+1:]...)
						found = true
						break
					}
				}
				
				// If not found, add to selection
				if !found {
					m.selectedToolIDs = append(m.selectedToolIDs, toolID)
				}
			}
			return m, nil
		}
		
	case "/":
		if m.focusedField == AgentFieldTools && !m.isFiltering {
			// Enter filter mode
			m.isFiltering = true
			m.toolsFilter = ""
			return m, nil
		}
	}
	
	// Update focused input component
	var cmd tea.Cmd
	switch m.focusedField {
	case AgentFieldName:
		m.nameInput, cmd = m.nameInput.Update(msg)
		cmds = append(cmds, cmd)
	case AgentFieldDesc:
		m.descInput, cmd = m.descInput.Update(msg)
		cmds = append(cmds, cmd)
	case AgentFieldPrompt:
		m.promptArea, cmd = m.promptArea.Update(msg)
		cmds = append(cmds, cmd)
	case AgentFieldSchedule:
		m.scheduleInput, cmd = m.scheduleInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	
	return m, tea.Batch(cmds...)
}

// Handle key events in detail view
func (m *AgentsModel) handleDetailViewKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		// Scroll up in details view (simple manual scrolling)
		if m.detailsScrollOffset > 0 {
			m.detailsScrollOffset--
		}
		return m, nil
		
	case "down", "j":
		// Scroll down in details view (simple manual scrolling)
		m.detailsScrollOffset++
		return m, nil
		
	case "pgup", "b":
		// Scroll up one page (half screen)
		maxHeight := m.height - 4
		if maxHeight < 10 {
			maxHeight = 10
		}
		pageSize := maxHeight / 2
		m.detailsScrollOffset -= pageSize
		if m.detailsScrollOffset < 0 {
			m.detailsScrollOffset = 0
		}
		return m, nil
		
	case "pgdown", "f":
		// Scroll down one page (half screen)
		maxHeight := m.height - 4
		if maxHeight < 10 {
			maxHeight = 10
		}
		pageSize := maxHeight / 2
		m.detailsScrollOffset += pageSize
		return m, nil
		
	case "g":
		// Go to top
		m.detailsScrollOffset = 0
		return m, nil
		
	case "G":
		// Go to bottom (will be clamped in render function)
		m.detailsScrollOffset = 9999
		return m, nil
		
	case "left", "h":
		// Navigate left through action buttons
		if m.actionButtonIndex > 0 {
			m.actionButtonIndex--
		}
		return m, nil
		
	case "right", "l":
		// Navigate right through action buttons
		if m.actionButtonIndex < 2 {  // 0=Run, 1=Edit, 2=Delete
			m.actionButtonIndex++
		}
		return m, nil
		
	case "enter", " ":
		// Execute selected action
		if m.selectedAgent == nil {
			return m, nil
		}
		
		switch m.actionButtonIndex {
		case 0: // Run Agent
			return m, m.runAgent(*m.selectedAgent)
		case 1: // Edit
			// Switch to edit mode - populate form with current agent data
			m.PushNavigation("Edit Agent")
			m.SetViewMode("edit")
			m.populateEditForm()
			m.nameInput.Focus()
			// Refresh available tools to ensure we have the latest data
			return m, m.loadTools()
		case 2: // Delete
			return m, m.deleteAgent(m.selectedAgent.ID)
		}
		return m, nil
		
	case "r":
		// Quick run with 'r' key
		if m.selectedAgent != nil {
			return m, m.runAgent(*m.selectedAgent)
		}
		return m, nil
		
	case "d":
		// Quick delete with 'd' key
		if m.selectedAgent != nil {
			return m, m.deleteAgent(m.selectedAgent.ID)
		}
		return m, nil
		
	case "esc":
		// Go back to list view
		if m.CanGoBack() {
			m.GoBack()
			m.selectedAgent = nil
			m.actionButtonIndex = 0
		}
		return m, nil
	}
	
	return m, nil
}

// Handle key events in edit form (same as create form but saves updates)
func (m *AgentsModel) handleEditFormKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd
	
	// Handle filter mode when in tools section first
	if m.focusedField == AgentFieldTools && m.isFiltering {
		switch msg.String() {
		case "esc":
			// Exit filter mode
			m.isFiltering = false
			m.toolsFilter = ""
			m.toolCursor = 0 // Reset cursor when exiting filter
			return m, nil
		case "backspace":
			// Remove last character from filter
			if len(m.toolsFilter) > 0 {
				m.toolsFilter = m.toolsFilter[:len(m.toolsFilter)-1]
				m.toolCursor = 0 // Reset cursor when filter changes
			}
			return m, nil
		default:
			// Add character to filter (only printable characters)
			if len(msg.String()) == 1 && msg.String() >= " " && msg.String() <= "~" {
				m.toolsFilter += msg.String()
				m.toolCursor = 0 // Reset cursor when filter changes
				return m, nil
			}
		}
	}
	
	switch msg.String() {
	case "esc":
		// Auto-save changes if form has been modified, then go back to detail view
		nameValue := m.nameInput.Value()
		descValue := m.descInput.Value()
		
		if nameValue != "" || descValue != "" {
			// Save the updated agent before exiting 
			cmd := m.updateAgent()
			m.SetViewMode("detail")
			m.GoBack()
			return m, cmd
		}
		
		// No changes to save, just go back
		m.SetViewMode("detail")
		m.GoBack()
		return m, nil
		
	case "tab":
		// Cycle through form fields
		m.cycleFocusedField()
		return m, nil
		
	case "ctrl+s":
		// Save agent updates
		return m, m.updateAgent()
		
	case "up", "k":
		if m.focusedField == AgentFieldEnvironments {
			// Navigate environment selection up
			if len(m.environments) > 0 && m.envCursor > 0 {
				m.envCursor--
				// Update scroll offset if cursor goes above visible area
				if m.envCursor < m.envOffset {
					m.envOffset = m.envCursor
				}
			}
			return m, nil
		} else if m.focusedField == AgentFieldTools {
			// Navigate tool selection up (work with filtered tools)
			filteredTools := m.getFilteredTools()
			if len(filteredTools) > 0 && m.toolCursor > 0 {
				m.toolCursor--
				// Update scroll offset if cursor goes above visible area
				if m.toolCursor < m.toolsOffset {
					m.toolsOffset = m.toolCursor
				}
			}
			return m, nil
		}
		
	case "down", "j":
		if m.focusedField == AgentFieldEnvironments {
			// Navigate environment selection down
			if len(m.environments) > 0 && m.envCursor < len(m.environments)-1 {
				m.envCursor++
				// Update scroll offset if cursor goes below visible area
				maxShow := 3  // Must match the display maxShow
				if m.envCursor >= m.envOffset + maxShow {
					m.envOffset = m.envCursor - maxShow + 1
				}
			}
			return m, nil
		} else if m.focusedField == AgentFieldTools {
			// Navigate tool selection down (work with filtered tools)
			filteredTools := m.getFilteredTools()
			if len(filteredTools) > 0 && m.toolCursor < len(filteredTools)-1 {
				m.toolCursor++
				// Update scroll offset if cursor goes below visible area
				maxShow := 4  // Must match the display maxShow
				if m.toolCursor >= m.toolsOffset + maxShow {
					m.toolsOffset = m.toolCursor - maxShow + 1
				}
			}
			return m, nil
		}
		
	case " ", "enter":
		if m.focusedField == AgentFieldEnvironments {
			// Toggle environment selection
			if len(m.environments) > 0 && m.envCursor < len(m.environments) {
				envID := m.environments[m.envCursor].ID
				
				// Check if environment is already selected
				found := false
				for i, selectedID := range m.selectedEnvIDs {
					if selectedID == envID {
						// Remove from selection
						m.selectedEnvIDs = append(m.selectedEnvIDs[:i], m.selectedEnvIDs[i+1:]...)
						found = true
						break
					}
				}
				
				// If not found, add to selection
				if !found {
					m.selectedEnvIDs = append(m.selectedEnvIDs, envID)
				}
			}
			return m, nil
		} else if m.focusedField == AgentFieldScheduleEnabled {
			// Toggle schedule enabled
			m.scheduleEnabled = !m.scheduleEnabled
			// Clear schedule input if disabling
			if !m.scheduleEnabled {
				m.scheduleInput.SetValue("")
			}
			return m, nil
		} else if m.focusedField == AgentFieldTools {
			// Toggle tool selection (work with filtered tools)
			filteredTools := m.getFilteredTools()
			if len(filteredTools) > 0 && m.toolCursor < len(filteredTools) {
				toolID := filteredTools[m.toolCursor].ID
				
				// Check if tool is already selected
				found := false
				for i, selectedID := range m.selectedToolIDs {
					if selectedID == toolID {
						// Remove from selection
						m.selectedToolIDs = append(m.selectedToolIDs[:i], m.selectedToolIDs[i+1:]...)
						found = true
						break
					}
				}
				
				// If not found, add to selection
				if !found {
					m.selectedToolIDs = append(m.selectedToolIDs, toolID)
				}
			}
			return m, nil
		}
		
	case "/":
		if m.focusedField == AgentFieldTools && !m.isFiltering {
			// Enter filter mode
			m.isFiltering = true
			m.toolsFilter = ""
			return m, nil
		}
	}
	
	// Update focused input component
	var cmd tea.Cmd
	switch m.focusedField {
	case AgentFieldName:
		m.nameInput, cmd = m.nameInput.Update(msg)
		cmds = append(cmds, cmd)
	case AgentFieldDesc:
		m.descInput, cmd = m.descInput.Update(msg)
		cmds = append(cmds, cmd)
	case AgentFieldPrompt:
		m.promptArea, cmd = m.promptArea.Update(msg)
		cmds = append(cmds, cmd)
	case AgentFieldSchedule:
		m.scheduleInput, cmd = m.scheduleInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	
	return m, tea.Batch(cmds...)
}

// Cycle through form fields
func (m *AgentsModel) cycleFocusedField() {
	// Blur current field
	switch m.focusedField {
	case AgentFieldName:
		m.nameInput.Blur()
	case AgentFieldDesc:
		m.descInput.Blur()
	case AgentFieldPrompt:
		m.promptArea.Blur()
	case AgentFieldSchedule:
		m.scheduleInput.Blur()
	}
	
	// Move to next field (skip schedule field if scheduling is disabled)
	for {
		m.focusedField = (m.focusedField + 1) % 7
		// Skip schedule field if scheduling is disabled
		if m.focusedField == AgentFieldSchedule && !m.scheduleEnabled {
			continue
		}
		break
	}
	
	// Focus new field
	switch m.focusedField {
	case AgentFieldName:
		m.nameInput.Focus()
	case AgentFieldDesc:
		m.descInput.Focus()
	case AgentFieldPrompt:
		m.promptArea.Focus()
	case AgentFieldSchedule:
		m.scheduleInput.Focus()
	}
}

// Reset create form to initial state
func (m *AgentsModel) resetCreateForm() {
	m.nameInput.SetValue("")
	m.descInput.SetValue("")
	m.promptArea.SetValue("")
	m.scheduleInput.SetValue("")
	m.scheduleEnabled = false
	m.selectedToolIDs = []int64{}
	m.focusedField = AgentFieldName
	m.toolCursor = 0
	m.envCursor = 0
	m.envOffset = 0
	if len(m.environments) > 0 {
		m.selectedEnvIDs = []int64{m.environments[0].ID}
	} else {
		m.selectedEnvIDs = []int64{}
	}
}

// Populate edit form with current agent data
func (m *AgentsModel) populateEditForm() {
	if m.selectedAgent == nil {
		return
	}
	
	m.nameInput.SetValue(m.selectedAgent.Name)
	m.descInput.SetValue(m.selectedAgent.Description)
	m.promptArea.SetValue(m.selectedAgent.Prompt)
	
	// Set schedule fields
	if m.selectedAgent.CronSchedule != nil {
		m.scheduleInput.SetValue(*m.selectedAgent.CronSchedule)
	} else {
		m.scheduleInput.SetValue("")
	}
	m.scheduleEnabled = m.selectedAgent.ScheduleEnabled
	
	// Load agent's environments from database (for now, use primary environment as fallback)
	// TODO: Load actual agent environments using AgentEnvironmentRepo.ListByAgent()
	m.selectedEnvIDs = []int64{m.selectedAgent.EnvironmentID}
	validEnv := false
	for _, env := range m.environments {
		if env.ID == m.selectedAgent.EnvironmentID {
			validEnv = true
			break
		}
	}
	if !validEnv && len(m.environments) > 0 {
		// Agent's environment doesn't exist, default to first available environment
		m.selectedEnvIDs = []int64{m.environments[0].ID}
	}
	
	m.focusedField = AgentFieldName
	m.toolCursor = 0
	
	// Populate selected tools from assigned tools
	m.selectedToolIDs = []int64{}
	for _, assignedTool := range m.assignedTools {
		// Find the matching tool in the available tools list to get the correct ID
		for _, availableTool := range m.availableTools {
			if availableTool.Name == assignedTool.ToolName && availableTool.EnvironmentID == assignedTool.EnvironmentID {
				m.selectedToolIDs = append(m.selectedToolIDs, availableTool.ID)
				break
			}
		}
	}
}