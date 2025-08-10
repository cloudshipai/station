package turbo_wizard

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Block Selection Handlers

func (m TurboWizardModel) updateBlockSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.currentBlock > 0 {
			m.currentBlock--
		}
	case "down", "j":
		if m.currentBlock < len(m.blocks)-1 {
			m.currentBlock++
		}
	case " ", "enter": // Toggle selection
		m.selectedBlocks[m.currentBlock] = !m.selectedBlocks[m.currentBlock]
	case "n": // Next step
		if m.hasSelectedBlocks() {
			m.state = StateConfiguringServer
			m.currentBlock = m.getNextSelectedBlock(0)
			m.startConfiguringCurrentBlock()
		}
	case "q", "esc":
		m.cancelled = true
		return m, tea.Quit
	}
	return m, nil
}

// Server Configuration Handlers

func (m TurboWizardModel) updateServerConfiguration(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y": // Accept current value and move to next
		m.configStep++
		if m.configStep >= m.getConfigStepCount() {
			// Configuration complete for this server
			m.configurations = append(m.configurations, *m.currentConfig)

			// Move to next selected block
			nextBlock := m.getNextSelectedBlock(m.currentBlock + 1)
			if nextBlock == -1 {
				// All blocks configured, go to environment selection
				m.state = StateSelectingEnvironment
			} else {
				m.currentBlock = nextBlock
				m.startConfiguringCurrentBlock()
			}
		}
	case "e": // Edit current field
		m.state = StateEditingField
		m.editingField = m.getCurrentFieldName()
		m.fieldValue = m.getCurrentFieldValue()
	case "b": // Go back
		if m.configStep > 0 {
			m.configStep--
		} else {
			// Go back to block selection
			m.state = StateShowingBlocks
		}
	case "q", "esc":
		m.cancelled = true
		return m, tea.Quit
	}
	return m, nil
}

// Field Editor Handlers

func (m TurboWizardModel) updateFieldEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter": // Save field
		m.saveCurrentField()
		m.state = StateConfiguringServer
	case "esc": // Cancel editing
		m.state = StateConfiguringServer
	case "backspace":
		if len(m.fieldValue) > 0 {
			m.fieldValue = m.fieldValue[:len(m.fieldValue)-1]
		}
	default:
		// Add character to field value
		if len(msg.String()) == 1 {
			m.fieldValue += msg.String()
		}
	}
	return m, nil
}

// Environment Selection Handlers

func (m TurboWizardModel) updateEnvironmentSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selectedEnv > 0 {
			m.selectedEnv--
		}
	case "down", "j":
		if m.selectedEnv < len(m.environments)-1 {
			m.selectedEnv++
		}
	case "enter": // Select environment and go to review
		m.state = StateReviewConfig
	case "b": // Go back
		if len(m.configurations) > 0 {
			// Go back to configuring last server
			m.configurations = m.configurations[:len(m.configurations)-1]
			m.state = StateConfiguringServer
			m.currentBlock = m.getLastSelectedBlock()
			m.startConfiguringCurrentBlock()
			m.configStep = m.getConfigStepCount() - 1
		} else {
			m.state = StateShowingBlocks
		}
	case "q", "esc":
		m.cancelled = true
		return m, tea.Quit
	}
	return m, nil
}

// Review Configuration Handlers

func (m TurboWizardModel) updateReviewConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter": // Accept and complete
		m.state = StateCompleted
		m.completed = true
		return m, tea.Quit
	case "b": // Go back to environment selection
		m.state = StateSelectingEnvironment
	case "q", "esc":
		m.cancelled = true
		return m, tea.Quit
	}
	return m, nil
}

// Helper methods for handlers

func (m *TurboWizardModel) hasSelectedBlocks() bool {
	for _, selected := range m.selectedBlocks {
		if selected {
			return true
		}
	}
	return false
}

func (m *TurboWizardModel) getCurrentFieldName() string {
	switch m.configStep {
	case 0:
		return "Server Name"
	case 1:
		switch m.currentConfig.Transport {
		case TransportSTDIO:
			return "Command"
		case TransportDocker:
			return "Docker Image"
		case TransportHTTP, TransportSSE:
			return "URL"
		}
	case 2:
		switch m.currentConfig.Transport {
		case TransportSTDIO:
			return "Arguments"
		case TransportDocker:
			return "Docker Mounts"
		case TransportHTTP, TransportSSE:
			return "Authentication"
		}
	case 3:
		return "Environment Variables"
	}
	return "Unknown Field"
}

func (m *TurboWizardModel) getCurrentFieldValue() string {
	switch m.configStep {
	case 0:
		return m.currentConfig.Name
	case 1:
		switch m.currentConfig.Transport {
		case TransportSTDIO, TransportDocker:
			return m.currentConfig.Command
		case TransportHTTP, TransportSSE:
			return m.currentConfig.URL
		}
	case 2:
		switch m.currentConfig.Transport {
		case TransportSTDIO:
			if len(m.currentConfig.Args) > 0 {
				return m.currentConfig.Args[0] // Simplified for demo
			}
		case TransportDocker:
			return "Docker mounts configuration" // Would be more complex in real implementation
		}
	}
	return ""
}

func (m *TurboWizardModel) saveCurrentField() {
	switch m.configStep {
	case 0:
		m.currentConfig.Name = m.fieldValue
	case 1:
		switch m.currentConfig.Transport {
		case TransportSTDIO, TransportDocker:
			m.currentConfig.Command = m.fieldValue
		case TransportHTTP, TransportSSE:
			m.currentConfig.URL = m.fieldValue
		}
	case 2:
		switch m.currentConfig.Transport {
		case TransportSTDIO:
			if m.fieldValue != "" {
				m.currentConfig.Args = []string{m.fieldValue} // Simplified
			}
		}
	}
}