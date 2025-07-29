package tabs

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/tui/components"
	"station/internal/tui/styles"
	"station/pkg/models"
)

// MCPModel represents the MCP servers configuration tab
type MCPModel struct {
	BaseTabModel
	
	// UI components
	configEditor textarea.Model
	nameInput    textinput.Model
	environmentList list.Model
	
	// Data
	envs         []*models.Environment
	repos        *repositories.Repositories
	
	// State
	mode         MCPMode
	configs      []MCPConfigDisplay
	selectedIdx  int
	showEditor   bool
	selectedEnvID int64
	focusedField MCPField
}

type MCPField int

const (
	MCPFieldName MCPField = iota
	MCPFieldEnvironment
	MCPFieldConfig
)

type MCPMode int

const (
	MCPModeList MCPMode = iota
	MCPModeEdit
)

type MCPConfigDisplay struct {
	ID          int64
	Name        string
	Version     int64
	Updated     string
	Size        string
	ConfigJSON  string // Store the actual JSON content
}

type MCPConfigSavedMsg struct {
	Config  MCPConfigDisplay
	Message string
	Error   string
}

// NewMCPModel creates a new MCP model
func NewMCPModel(database db.Database) *MCPModel {
	repos := repositories.New(database)
	
	// Create textarea for config editing - scrollable
	ta := textarea.New()
	ta.Placeholder = "Paste your MCP server configuration here (JSON format)..."
	ta.SetWidth(60) // Will be adjusted dynamically in renderEditor
	ta.SetHeight(5)  // Will be adjusted dynamically in renderEditor
	
	// Create text input for name
	ti := textinput.New()
	ti.Placeholder = "Configuration name"
	ti.Width = 30
	
	// Create environment list
	envList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 4)
	envList.SetShowHelp(false)
	envList.SetShowStatusBar(false)
	envList.SetShowTitle(false)
	envList.SetFilteringEnabled(false)
	
	m := &MCPModel{
		BaseTabModel:    NewBaseTabModel(database, "MCP Servers"),
		configEditor:    ta,
		nameInput:       ti,
		environmentList: envList,
		repos:           repos,
		mode:            MCPModeList,
		configs:         []MCPConfigDisplay{},
		selectedIdx:     0,
		showEditor:      false,
		selectedEnvID:   1, // Default to first environment
		focusedField:    MCPFieldName,
	}
	
	// Load environments
	m.loadEnvironments()
	
	return m
}

// Init initializes the MCP tab
func (m MCPModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadConfigs(),
		m.configEditor.Cursor.BlinkCmd(),
	)
}

// Update handles messages
func (m *MCPModel) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd
	
	// Debug: Log important incoming messages (filter out spam)
	switch v := msg.(type) {
	case MCPConfigsLoadedMsg:
		log.Printf("DEBUG: Update received MCPConfigsLoadedMsg with %d configs", len(v.Configs))
	case MCPConfigSavedMsg:
		log.Printf("DEBUG: Update received MCPConfigSavedMsg")
	case MCPConfigDeletedMsg:
		log.Printf("DEBUG: Update received MCPConfigDeletedMsg for config %d", v.ConfigID)
	case tea.KeyMsg:
		// Don't log key messages to avoid spam
	default:
		// Filter out common spam messages
		msgType := fmt.Sprintf("%T", msg)
		if !strings.Contains(msgType, "spinner.TickMsg") && 
		   !strings.Contains(msgType, "cursor.BlinkMsg") {
			log.Printf("DEBUG: Update received message type: %T", msg)
		}
	}
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		m.configEditor.SetWidth(msg.Width - 10)
		
	case tea.KeyMsg:
		log.Printf("DEBUG: MCP received key: '%s', mode: %d, focusedField: %d", msg.String(), m.mode, m.focusedField)
		switch msg.String() {
		case "n":
			if m.mode == MCPModeList {
				m.mode = MCPModeEdit
				m.showEditor = true
				m.nameInput.Focus()
				m.nameInput.SetValue("")
				m.configEditor.SetValue("")
				return m, nil
			}
		case "enter":
			log.Printf("DEBUG: Enter key pressed - mode: %d, focusedField: %d", m.mode, m.focusedField)
			if m.mode == MCPModeList && len(m.configs) > 0 {
				// Edit selected config
				m.mode = MCPModeEdit
				m.showEditor = true
				m.nameInput.Focus()
				// Pre-populate with selected config data
				selected := m.configs[m.selectedIdx]
				m.nameInput.SetValue(selected.Name)
				
				// Load the actual saved configuration JSON
				if selected.ConfigJSON != "" {
					m.configEditor.SetValue(selected.ConfigJSON)
				} else {
					// Fallback for configs that don't have JSON stored
					placeholderConfig := `{
  "mcpServers": {
    "` + selected.Name + `": {
      "command": "your-command-here",
      "args": ["arg1", "arg2"]
    }
  }
}`
					m.configEditor.SetValue(placeholderConfig)
				}
				return m, nil
			} else if m.mode == MCPModeEdit && m.focusedField != MCPFieldConfig {
				// Save when Enter is pressed in Name or Environment field
				log.Printf("DEBUG: Calling saveConfig() directly")
				
				// Do the save logic directly instead of using a command
				configText := m.configEditor.Value()
				log.Printf("DEBUG: configText length: %d", len(configText))
				
				if configText == "" {
					log.Printf("DEBUG: Config text is empty")
					return m, tea.Printf("Error: Configuration cannot be empty")
				}
				
				// Basic JSON validation
				var configData models.MCPConfigData
				if err := json.Unmarshal([]byte(configText), &configData); err != nil {
					log.Printf("DEBUG: JSON validation failed: %v", err)
					return m, tea.Printf("Error: Invalid JSON: %v", err)
				}
				log.Printf("DEBUG: JSON validation passed")
				
				// Save config to database
				configName := m.nameInput.Value()
				if configName == "" {
					configName = "Unnamed Configuration"
				}
				log.Printf("DEBUG: Config name: '%s'", configName)
				
				// Add the name to the JSON config for identification
				var configMap map[string]interface{}
				if err := json.Unmarshal([]byte(configText), &configMap); err == nil {
					configMap["name"] = configName
					if modifiedJSON, err := json.MarshalIndent(configMap, "", "  "); err == nil {
						configText = string(modifiedJSON)
					}
				}
				
				// Get next version for the selected environment
				nextVersion, err := m.repos.MCPConfigs.GetNextVersion(m.selectedEnvID)
				if err != nil {
					log.Printf("DEBUG: Failed to get next version: %v", err)
					return m, tea.Printf("Error: Failed to get next version: %v", err)
				}
				log.Printf("DEBUG: Next version: %d", nextVersion)
				
				// Save to database (simplified - not using encryption for now)
				savedConfig, err := m.repos.MCPConfigs.Create(m.selectedEnvID, nextVersion, configText, "")
				if err != nil {
					log.Printf("DEBUG: Failed to save config: %v", err)
					return m, tea.Printf("Error: Failed to save config: %v", err)
				}
				log.Printf("DEBUG: Saved config to database: %+v", savedConfig)
				
				// Return to list mode
				log.Printf("DEBUG: Returning to list mode")
				m.mode = MCPModeList
				m.showEditor = false
				m.nameInput.Blur()
				m.configEditor.Blur()
				m.focusedField = MCPFieldName
				m.nameInput.SetValue("")
				m.configEditor.SetValue("")
				
				// Set loading state and reload configs to show success message
				m.SetLoading(true)
				return m, tea.Batch(
					m.loadConfigs(),
					tea.Printf("Configuration '%s' v%d saved successfully", configName, nextVersion),
				)
			}
		case "d":
			log.Printf("DEBUG: Received 'd' key - mode: %d, configs count: %d, selectedIdx: %d", m.mode, len(m.configs), m.selectedIdx)
			if m.mode == MCPModeList && len(m.configs) > 0 {
				log.Printf("DEBUG: Calling deleteConfig for config ID: %d", m.configs[m.selectedIdx].ID)
				// Delete selected config
				return m, m.deleteConfig(m.configs[m.selectedIdx].ID)
			} else {
				log.Printf("DEBUG: Delete conditions not met - mode: %d, configs: %d", m.mode, len(m.configs))
			}
		case "up", "k":
			log.Printf("DEBUG: Received 'k' or 'up' key, mode=%d, configs=%d", m.mode, len(m.configs))
			if m.mode == MCPModeList && len(m.configs) > 0 {
				m.selectedIdx = (m.selectedIdx - 1 + len(m.configs)) % len(m.configs)
				log.Printf("DEBUG: Moved selection up to index %d", m.selectedIdx)
				return m, nil
			}
		case "down", "j":
			log.Printf("DEBUG: Received 'j' or 'down' key, mode=%d, configs=%d", m.mode, len(m.configs))
			if m.mode == MCPModeList && len(m.configs) > 0 {
				m.selectedIdx = (m.selectedIdx + 1) % len(m.configs)
				log.Printf("DEBUG: Moved selection down to index %d", m.selectedIdx)
				return m, nil
			}
		case "tab":
			if m.mode == MCPModeEdit {
				// Cycle through fields
				switch m.focusedField {
				case MCPFieldName:
					m.nameInput.Blur()
					m.focusedField = MCPFieldEnvironment
				case MCPFieldEnvironment:
					m.focusedField = MCPFieldConfig
					m.configEditor.Focus()
				case MCPFieldConfig:
					m.configEditor.Blur()
					m.focusedField = MCPFieldName
					m.nameInput.Focus()
				}
				return m, nil
			}
		case "esc":
			if m.mode == MCPModeEdit {
				m.mode = MCPModeList
				m.showEditor = false
				m.nameInput.Blur()
				m.configEditor.Blur()
				m.focusedField = MCPFieldName
				return m, nil
			}
		case "F1":
			// F1 key as a reliable save - should work from any field
			if m.mode == MCPModeEdit {
				return m, m.saveConfig()
			}
		case "right":
			if m.mode == MCPModeEdit && m.showEditor {
				// Handled by the main tab case above
				return m, nil
			}
		case "ctrl+s":
			if m.mode == MCPModeEdit {
				return m, m.saveConfig()
			}
		case "s":
			// Also allow just 's' key as a backup for save
			if m.mode == MCPModeEdit {
				return m, m.saveConfig()
			}
		}
		
		// Handle input for components based on focused field
		if m.mode == MCPModeEdit {
			switch m.focusedField {
			case MCPFieldName:
				var cmd tea.Cmd
				m.nameInput, cmd = m.nameInput.Update(msg)
				cmds = append(cmds, cmd)
			case MCPFieldEnvironment:
				// Handle environment selection with up/down keys
				if msg.String() == "up" || msg.String() == "k" {
					if len(m.envs) > 0 {
						for i, env := range m.envs {
							if env.ID == m.selectedEnvID {
								prevIdx := (i - 1 + len(m.envs)) % len(m.envs)
								m.selectedEnvID = m.envs[prevIdx].ID
								break
							}
						}
					}
				} else if msg.String() == "down" || msg.String() == "j" {
					if len(m.envs) > 0 {
						for i, env := range m.envs {
							if env.ID == m.selectedEnvID {
								nextIdx := (i + 1) % len(m.envs)
								m.selectedEnvID = m.envs[nextIdx].ID
								break
							}
						}
					}
				}
			case MCPFieldConfig:
				// Handle save keys in config field before passing to textarea
				if msg.String() == "ctrl+s" || msg.String() == "ctrl+enter" {
					return m, m.saveConfig()
				}
				var cmd tea.Cmd
				m.configEditor, cmd = m.configEditor.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
		
	case MCPConfigSavedMsg:
		log.Printf("DEBUG: Received MCPConfigSavedMsg: %+v", msg)
		// Handle config save response
		if msg.Error != "" {
			// Show error message
			log.Printf("DEBUG: Error in save: %s", msg.Error)
			return m, tea.Printf("Error: %s", msg.Error)
		} else {
			// Config saved successfully - return to list mode
			log.Printf("DEBUG: Save successful, returning to list mode")
			m.mode = MCPModeList
			m.showEditor = false
			m.nameInput.Blur()
			m.configEditor.Blur()
			m.focusedField = MCPFieldName
			m.nameInput.SetValue("")
			m.configEditor.SetValue("")
			return m, tea.Printf(msg.Message)
		}
		
	case MCPConfigsLoadedMsg:
		log.Printf("DEBUG: Received MCPConfigsLoadedMsg with %d configs", len(msg.Configs))
		// Handle configs loaded response
		if msg.Error != nil {
			log.Printf("DEBUG: Error loading configs: %v", msg.Error)
			m.SetLoading(false)
			return m, tea.Printf("Error loading configs: %v", msg.Error)
		}
		
		// Update configs and stop loading
		m.configs = msg.Configs
		m.SetLoading(false)
		log.Printf("DEBUG: Configs loaded successfully, loading state cleared")
		return m, nil
		
	case MCPConfigDeletedMsg:
		log.Printf("DEBUG: Received MCPConfigDeletedMsg for config %d", msg.ConfigID)
		// Handle config delete response
		if msg.Error != nil {
			log.Printf("DEBUG: Error deleting config: %v", msg.Error)
			return m, tea.Printf("Error: %s", msg.Message)
		}
		
		// Remove from local list and adjust selection
		for i, config := range m.configs {
			if config.ID == msg.ConfigID {
				m.configs = append(m.configs[:i], m.configs[i+1:]...)
				if m.selectedIdx >= len(m.configs) && len(m.configs) > 0 {
					m.selectedIdx = len(m.configs) - 1
				}
				break
			}
		}
		
		log.Printf("DEBUG: Config deleted successfully, %d configs remaining", len(m.configs))
		return m, tea.Printf(msg.Message)
	}
	
	// Update components when editor is shown - handled by focused field logic above
	// This section is now handled in the switch statement based on focusedField
	
	return m, tea.Batch(cmds...)
}

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

// RefreshData reloads MCP configs from database
func (m *MCPModel) RefreshData() tea.Cmd {
	m.SetLoading(true)
	return m.loadConfigs()
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
		configLine := fmt.Sprintf("%s (v%d) - Updated %s - Size %s", 
			config.Name, config.Version, config.Updated, config.Size)
		configItems = append(configItems, prefix+configLine)
	}
	
	configList := strings.Join(configItems, "\n")
	
	helpText := styles.HelpStyle.Render("• ↑/↓/j/k: navigate • n: new config • enter: edit • d: delete selected")
	
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
	helpText := styles.HelpStyle.Render("tab: switch • enter: save (from name/env field) • esc: cancel")
	sections = append(sections, "")
	sections = append(sections, helpText)
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// MCPConfigsLoadedMsg represents loaded configs
type MCPConfigsLoadedMsg struct {
	Configs []MCPConfigDisplay
	Error   error
}

// Load configurations from database
func (m MCPModel) loadConfigs() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: Loading configs from database")
		
		// Load all configs from all environments and group them by name
		configMap := make(map[string]*MCPConfigDisplay)
		
		// Get all environments first
		envs, err := m.repos.Environments.List()
		if err != nil {
			log.Printf("DEBUG: Failed to load environments: %v", err)
			return MCPConfigsLoadedMsg{Error: err}
		}
		
		// For each environment, load its configs
		for _, env := range envs {
			configs, err := m.repos.MCPConfigs.ListByEnvironment(env.ID)
			if err != nil {
				log.Printf("DEBUG: Failed to load configs for env %d: %v", env.ID, err)
				continue
			}
			
			// Convert to display format - keep only the latest version of each config
			for _, config := range configs {
				// Extract name from config JSON (simplified - would need proper parsing)
				configName := fmt.Sprintf("config-%d", config.ID)
				
				// Try to extract name from JSON if possible
				var configData map[string]interface{}
				if err := json.Unmarshal([]byte(config.ConfigJSON), &configData); err == nil {
					if name, ok := configData["name"].(string); ok && name != "" {
						configName = name
					}
				}
				
				// Create display item
				displayConfig := &MCPConfigDisplay{
					ID:         config.ID,
					Name:       configName,
					Version:    config.Version,
					Updated:    config.UpdatedAt.Format("Jan 2 15:04"),
					Size:       fmt.Sprintf("%.1fKB", float64(len(config.ConfigJSON))/1024),
					ConfigJSON: config.ConfigJSON,
				}
				
				// Keep only the latest version of each named config
				if existing, exists := configMap[configName]; !exists || config.Version > existing.Version {
					configMap[configName] = displayConfig
				}
			}
		}
		
		// Convert map to slice
		var displayConfigs []MCPConfigDisplay
		for _, config := range configMap {
			displayConfigs = append(displayConfigs, *config)
		}
		
		log.Printf("DEBUG: Loaded %d configs from database, returning MCPConfigsLoadedMsg", len(displayConfigs))
		return MCPConfigsLoadedMsg{Configs: displayConfigs}
	})
}

// Load environments from database
func (m *MCPModel) loadEnvironments() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		envs, err := m.repos.Environments.List()
		if err != nil {
			return err
		}
		
		m.envs = envs
		
		// Convert to list items
		items := make([]list.Item, len(envs))
		for i, env := range envs {
			items[i] = EnvironmentItem{env: *env}
		}
		m.environmentList.SetItems(items)
		
		// Set default selection to first environment
		if len(envs) > 0 {
			m.selectedEnvID = envs[0].ID
		}
		
		return nil
	})
}

// Save configuration with tool discovery
func (m MCPModel) saveConfig() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: saveConfig() called")
		// Parse the config JSON to validate it
		configText := m.configEditor.Value()
		log.Printf("DEBUG: configText length: %d", len(configText))
		if configText == "" {
			log.Printf("DEBUG: Config text is empty, returning error")
			return MCPConfigSavedMsg{Error: "Configuration cannot be empty"}
		}
		
		// Basic JSON validation
		var configData models.MCPConfigData
		if err := json.Unmarshal([]byte(configText), &configData); err != nil {
			log.Printf("DEBUG: JSON validation failed: %v", err)
			return MCPConfigSavedMsg{Error: fmt.Sprintf("Invalid JSON: %v", err)}
		}
		log.Printf("DEBUG: JSON validation passed")
		
		// Save config to database (simplified for now)
		// In a real implementation, this would use the MCPConfigService
		configName := m.nameInput.Value()
		if configName == "" {
			configName = "Unnamed Configuration"
		}
		log.Printf("DEBUG: Config name: '%s'", configName)
		
		// Create new config display item
		newConfig := MCPConfigDisplay{
			ID:      int64(len(m.configs) + 1), // Simple ID for now
			Name:    configName,
			Version: 1,
			Updated: "just now",
			Size:    fmt.Sprintf("%.1fKB", float64(len(configText))/1024),
		}
		log.Printf("DEBUG: Created newConfig: %+v", newConfig)
		
		// Add to configs list
		m.configs = append(m.configs, newConfig)
		log.Printf("DEBUG: Added to configs list, new length: %d", len(m.configs))
		
		// TODO: Trigger tool discovery for this config/environment
		// This would call: toolDiscoveryService.DiscoverTools(m.selectedEnvID)
		
		result := MCPConfigSavedMsg{
			Config: newConfig,
			Message: fmt.Sprintf("Configuration '%s' saved successfully", configName),
		}
		log.Printf("DEBUG: Returning MCPConfigSavedMsg: %+v", result)
		return result
	})
}

// MCPConfigDeletedMsg represents a deleted config
type MCPConfigDeletedMsg struct {
	ConfigID int64
	Error    error
	Message  string
}

// Delete configuration with cascade deletion of servers and tools
func (m MCPModel) deleteConfig(configID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: Deleting MCP config %d with cascade deletion", configID)
		
		// Get all servers for this config to cascade delete tools
		servers, err := m.repos.MCPServers.GetByConfigID(configID)
		if err != nil {
			log.Printf("DEBUG: Failed to get servers for config %d: %v", configID, err)
			return MCPConfigDeletedMsg{
				ConfigID: configID,
				Error:    err,
				Message:  fmt.Sprintf("Failed to get servers for config: %v", err),
			}
		}
		
		log.Printf("DEBUG: Found %d servers to cascade delete", len(servers))
		
		// Delete tools for each server
		for _, server := range servers {
			if err := m.repos.MCPTools.DeleteByServerID(server.ID); err != nil {
				log.Printf("DEBUG: Failed to delete tools for server %d: %v", server.ID, err)
				return MCPConfigDeletedMsg{
					ConfigID: configID,
					Error:    err,
					Message:  fmt.Sprintf("Failed to delete tools for server %s: %v", server.Name, err),
				}
			}
			log.Printf("DEBUG: Deleted tools for server %s (ID: %d)", server.Name, server.ID)
		}
		
		// Delete the servers
		if err := m.repos.MCPServers.DeleteByConfigID(configID); err != nil {
			log.Printf("DEBUG: Failed to delete servers for config %d: %v", configID, err)
			return MCPConfigDeletedMsg{
				ConfigID: configID,
				Error:    err,
				Message:  fmt.Sprintf("Failed to delete servers: %v", err),
			}
		}
		
		log.Printf("DEBUG: Deleted %d servers for config %d", len(servers), configID)
		
		// Delete the config itself
		if err := m.repos.MCPConfigs.Delete(configID); err != nil {
			log.Printf("DEBUG: Failed to delete config %d: %v", configID, err)
			return MCPConfigDeletedMsg{
				ConfigID: configID,
				Error:    err,
				Message:  fmt.Sprintf("Failed to delete configuration: %v", err),
			}
		}
		
		log.Printf("DEBUG: Successfully deleted config %d with cascade deletion", configID)
		return MCPConfigDeletedMsg{
			ConfigID: configID,
			Message:  "Configuration and all associated tools deleted successfully",
		}
	})
}

// IsMainView returns true if in main list view
func (m MCPModel) IsMainView() bool {
	return m.mode == MCPModeList
}