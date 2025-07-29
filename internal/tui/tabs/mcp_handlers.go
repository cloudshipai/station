package tabs

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	
	"station/pkg/models"
)

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
		return m.handleKeyMsg(msg, cmds)
		
	case MCPConfigSavedMsg:
		return m.handleConfigSavedMsg(msg)
		
	case MCPConfigsLoadedMsg:
		return m.handleConfigsLoadedMsg(msg)
		
	case MCPConfigDeletedMsg:
		return m.handleConfigDeletedMsg(msg)
		
	case MCPConfigVersionsLoadedMsg:
		return m.handleConfigVersionsLoadedMsg(msg)
		
	case MCPVersionLoadedMsg:
		return m.handleVersionLoadedMsg(msg)
		
	case MCPToolDiscoveryCompletedMsg:
		return m.handleToolDiscoveryCompletedMsg(msg)
	}
	
	return m, tea.Batch(cmds...)
}

// Handle key messages
func (m *MCPModel) handleKeyMsg(msg tea.KeyMsg, cmds []tea.Cmd) (TabModel, tea.Cmd) {
	log.Printf("DEBUG: MCP received key: '%s', mode: %d, focusedField: %d", msg.String(), m.mode, m.focusedField)
	
	switch msg.String() {
	case "n":
		if m.mode == MCPModeList {
			m.mode = MCPModeEdit
			m.showEditor = true
			m.nameInput.Focus()
			m.nameInput.SetValue("")
			m.configEditor.SetValue("")
			// Store original values for change detection
			m.originalName = ""
			m.originalConfig = ""
			return m, nil
		}
		
	case "enter":
		return m.handleEnterKey(cmds)
		
	case "d":
		// Only handle delete in list mode, not in edit mode
		if m.mode == MCPModeList {
			return m.handleDeleteKey()
		}
		
	case "r":
		// Refresh/discover tools for selected config in list mode
		if m.mode == MCPModeList && len(m.configs) > 0 {
			return m.handleRefreshToolsFromList()
		}
		
	case "up", "k":
		// Let components handle up/down keys when config or versions field is focused
		if m.mode == MCPModeEdit && (m.focusedField == MCPFieldConfig || m.focusedField == MCPFieldVersions) {
			return m.handleFieldInput(msg, cmds)
		}
		return m.handleUpKey()
		
	case "down", "j":
		// Let components handle up/down keys when config or versions field is focused
		if m.mode == MCPModeEdit && (m.focusedField == MCPFieldConfig || m.focusedField == MCPFieldVersions) {
			return m.handleFieldInput(msg, cmds)
		}
		return m.handleDownKey()
		
	case "tab":
		return m.handleTabKey()
		
	case "esc":
		return m.handleEscKey()
		
	case "F1":
		// F1 key as a reliable save - should work from any field
		if m.mode == MCPModeEdit {
			return m, m.saveConfig()
		}
		
	case "ctrl+s":
		if m.mode == MCPModeEdit {
			return m, m.saveConfig()
		}
		
	case "ctrl+r":
		// Manual refresh/tool discovery
		if m.mode == MCPModeEdit && m.nameInput.Value() != "" {
			configName := m.nameInput.Value()
			log.Printf("DEBUG: Manual tool discovery triggered for config '%s'", configName)
			// Run tool discovery in background with timeout protection
			go func() {
				result, err := m.toolDiscoverySvc.ReplaceToolsWithTransaction(m.selectedEnvID, configName)
				if err != nil {
					log.Printf("ERROR: Manual tool discovery failed: %v", err)
				} else {
					log.Printf("DEBUG: Manual tool discovery completed - %d tools from %d/%d servers", 
						result.TotalTools, result.SuccessfulServers, result.TotalServers)
				}
			}()
			return m, tea.Printf("🔄 Tool discovery started in background...")
		}
	}
	
	// Handle input for components based on focused field
	if m.mode == MCPModeEdit {
		return m.handleFieldInput(msg, cmds)
	}
	
	return m, tea.Batch(cmds...)
}

// Handle Enter key press
func (m *MCPModel) handleEnterKey(cmds []tea.Cmd) (TabModel, tea.Cmd) {
	log.Printf("DEBUG: Enter key pressed - mode: %d, focusedField: %d", m.mode, m.focusedField)
	
	if m.mode == MCPModeList && len(m.configs) > 0 {
		// Edit selected config
		m.mode = MCPModeEdit
		m.showEditor = true
		m.nameInput.Focus()
		// Pre-populate with selected config data
		selected := m.configs[m.selectedIdx]
		m.nameInput.SetValue(selected.Name)
		m.currentConfigName = selected.Name
		
		// Store original values for change detection
		m.originalName = selected.Name
		if selected.ConfigJSON != "" {
			m.originalConfig = selected.ConfigJSON
		} else {
			m.originalConfig = ""
		}
		
		// Load all versions of this config
		cmd := m.loadConfigVersions(selected.Name, m.selectedEnvID)
		cmds = append(cmds, cmd)
		
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
		return m, tea.Batch(cmds...)
		
	} else if m.mode == MCPModeEdit && m.focusedField == MCPFieldVersions && len(m.configVersions) > 0 {
		// Load selected version into form
		selectedVersion := m.configVersions[m.selectedVersionIdx]
		return m, m.loadVersionIntoForm(selectedVersion.ID)
		
	} else if m.mode == MCPModeEdit && m.focusedField != MCPFieldConfig && m.focusedField != MCPFieldVersions {
		// Save when Enter is pressed in Name or Environment field
		return m.handleSaveConfig()
	}
	
	return m, tea.Batch(cmds...)
}

// Handle save configuration logic
func (m *MCPModel) handleSaveConfig() (TabModel, tea.Cmd) {
	log.Printf("DEBUG: Calling saveConfig() directly")
	
	// Do the save logic directly instead of using a command
	configText := m.configEditor.Value()
	log.Printf("DEBUG: configText length: %d", len(configText))
	
	if configText == "" {
		log.Printf("DEBUG: Config text is empty")
		return m, tea.Printf("Error: Configuration cannot be empty")
	}
	
	// Basic JSON validation
	var rawConfig map[string]interface{}
	if err := json.Unmarshal([]byte(configText), &rawConfig); err != nil {
		log.Printf("DEBUG: JSON validation failed: %v", err)
		// Set a persistent error state and show detailed feedback
		errorMsg := fmt.Sprintf("Invalid JSON: %v", err)
		m.SetError(errorMsg)
		// Return error to prevent saving
		return m, tea.Printf("❌ " + errorMsg)
	}
	
	// Validate that the JSON contains the required structure
	var serversData map[string]interface{}
	if mcpServers, ok := rawConfig["mcpServers"].(map[string]interface{}); ok {
		serversData = mcpServers
	} else if servers, ok := rawConfig["servers"].(map[string]interface{}); ok {
		serversData = servers
	} else {
		errorMsg := "Invalid config format: must contain 'mcpServers' or 'servers' field"
		log.Printf("DEBUG: Config structure validation failed: %s", errorMsg)
		m.SetError(errorMsg)
		return m, tea.Printf("❌ " + errorMsg)
	}
	
	// Validate that there's at least one server
	if len(serversData) == 0 {
		errorMsg := "Configuration must contain at least one server"
		log.Printf("DEBUG: Server count validation failed: %s", errorMsg)
		m.SetError(errorMsg)
		return m, tea.Printf("❌ " + errorMsg)
	}
	
	log.Printf("DEBUG: JSON validation passed")
	// Clear any previous error since JSON is now valid
	m.SetError("")
	
	// Save config to database
	configName := m.nameInput.Value()
	if configName == "" {
		configName = "Unnamed Configuration"
	}
	log.Printf("DEBUG: Config name: '%s'", configName)
	
	// Convert from UI format (mcpServers) to internal format (servers)
	var configData models.MCPConfigData
	configData.Servers = make(map[string]models.MCPServerConfig)
	
	// Convert servers to the expected format
	for serverName, serverConfigRaw := range serversData {
		serverConfigMap, ok := serverConfigRaw.(map[string]interface{})
		if !ok {
			continue
		}
		
		serverConfig := models.MCPServerConfig{}
		if command, ok := serverConfigMap["command"].(string); ok {
			serverConfig.Command = command
		}
		if argsRaw, ok := serverConfigMap["args"].([]interface{}); ok {
			for _, arg := range argsRaw {
				if argStr, ok := arg.(string); ok {
					serverConfig.Args = append(serverConfig.Args, argStr)
				}
			}
		}
		if envRaw, ok := serverConfigMap["env"].(map[string]interface{}); ok {
			serverConfig.Env = make(map[string]string)
			for k, v := range envRaw {
				if vStr, ok := v.(string); ok {
					serverConfig.Env[k] = vStr
				}
			}
		}
		
		configData.Servers[serverName] = serverConfig
	}
	
	log.Printf("DEBUG: Converted to internal format with %d servers", len(configData.Servers))
	
	// Add the config name to the data structure
	configData.Name = configName
	
	// Save config using service (with proper encryption)
	savedConfig, err := m.mcpConfigSvc.UploadConfig(m.selectedEnvID, &configData)
	if err != nil {
		log.Printf("DEBUG: Failed to save config: %v", err)
		return m, tea.Printf("Error: Failed to save config: %v", err)
	}
	log.Printf("DEBUG: Saved encrypted config to database: %+v", savedConfig)
	
	// Start background tool discovery after manual save (non-blocking)
	log.Printf("DEBUG: Starting background tool discovery for config '%s'", configName) 
	go func() {
		result, err := m.toolDiscoverySvc.ReplaceToolsWithTransaction(m.selectedEnvID, configName)
		if err != nil {
			log.Printf("ERROR: Background tool discovery failed for '%s': %v", configName, err)
		} else {
			log.Printf("DEBUG: Background tool discovery completed for '%s' - %d tools from %d/%d servers", 
				configName, result.TotalTools, result.SuccessfulServers, result.TotalServers)
		}
	}()
	
	// Stay in editor mode and show success toast
	log.Printf("DEBUG: Staying in editor mode, showing success toast")
	
	// Show success message but stay in editor
	return m, tea.Printf("✅ Configuration '%s' v%d saved successfully - discovering tools in background", configName, savedConfig.Version)
}

// Handle delete key
func (m *MCPModel) handleDeleteKey() (TabModel, tea.Cmd) {
	log.Printf("DEBUG: Deleting config in list mode - configs count: %d, selectedIdx: %d", len(m.configs), m.selectedIdx)
	if len(m.configs) > 0 {
		log.Printf("DEBUG: Calling deleteConfig for config ID: %d", m.configs[m.selectedIdx].ID)
		// Delete selected config
		return m, m.deleteConfig(m.configs[m.selectedIdx].ID)
	} else {
		log.Printf("DEBUG: No configs to delete")
	}
	return m, nil
}

// Handle refresh tools from list mode
func (m *MCPModel) handleRefreshToolsFromList() (TabModel, tea.Cmd) {
	selected := m.configs[m.selectedIdx]
	configName := selected.Name
	environmentID := selected.EnvironmentID
	
	log.Printf("DEBUG: Starting tool discovery for config '%s' in environment %d from list mode", configName, environmentID)
	
	return m, m.discoverToolsForConfig(configName, environmentID)
}

// Command to discover tools for a config
func (m MCPModel) discoverToolsForConfig(configName string, environmentID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: Running tool discovery for config '%s'", configName)
		
		result, err := m.toolDiscoverySvc.ReplaceToolsWithTransaction(environmentID, configName)
		if err != nil {
			log.Printf("ERROR: Tool discovery failed for '%s': %v", configName, err)
			return MCPToolDiscoveryCompletedMsg{
				ConfigName: configName,
				Success:    false,
				Error:      err,
			}
		}
		
		log.Printf("DEBUG: Tool discovery completed for '%s' - %d tools from %d/%d servers", 
			configName, result.TotalTools, result.SuccessfulServers, result.TotalServers)
		
		return MCPToolDiscoveryCompletedMsg{
			ConfigName: configName,
			Success:    true,
			ToolCount:  result.TotalTools,
		}
	})
}

// Handle up key
func (m *MCPModel) handleUpKey() (TabModel, tea.Cmd) {
	log.Printf("DEBUG: Received 'k' or 'up' key, mode=%d, configs=%d", m.mode, len(m.configs))
	if m.mode == MCPModeList && len(m.configs) > 0 {
		m.selectedIdx = (m.selectedIdx - 1 + len(m.configs)) % len(m.configs)
		log.Printf("DEBUG: Moved selection up to index %d", m.selectedIdx)
		return m, nil
	}
	return m, nil
}

// Handle down key
func (m *MCPModel) handleDownKey() (TabModel, tea.Cmd) {
	log.Printf("DEBUG: Received 'j' or 'down' key, mode=%d, configs=%d", m.mode, len(m.configs))
	if m.mode == MCPModeList && len(m.configs) > 0 {
		m.selectedIdx = (m.selectedIdx + 1) % len(m.configs)
		log.Printf("DEBUG: Moved selection down to index %d", m.selectedIdx)
		return m, nil
	}
	return m, nil
}

// Handle tab key
func (m *MCPModel) handleTabKey() (TabModel, tea.Cmd) {
	if m.mode == MCPModeEdit {
		// Cycle through fields
		switch m.focusedField {
		case MCPFieldName:
			m.nameInput.Blur()
			m.focusedField = MCPFieldEnvironment
		case MCPFieldEnvironment:
			m.focusedField = MCPFieldVersions
		case MCPFieldVersions:
			m.focusedField = MCPFieldConfig
			m.configEditor.Focus()
		case MCPFieldConfig:
			m.configEditor.Blur()
			m.focusedField = MCPFieldName
			m.nameInput.Focus()
		}
		return m, nil
	}
	return m, nil
}

// Handle escape key
func (m *MCPModel) handleEscKey() (TabModel, tea.Cmd) {
	if m.mode == MCPModeEdit {
		// Check if content has actually changed before auto-saving
		configText := m.configEditor.Value()
		configName := m.nameInput.Value()
		
		// Only auto-save if there's content AND it has changed from original
		hasChanges := (configName != m.originalName) || (configText != m.originalConfig)
		
		if configText != "" && configName != "" && hasChanges {
			log.Printf("DEBUG: Changes detected - auto-saving on ESC (name: '%s' -> '%s', config changed: %v)", 
				m.originalName, configName, configText != m.originalConfig)
			
			// Basic JSON validation before auto-save
			var rawConfig map[string]interface{}
			if err := json.Unmarshal([]byte(configText), &rawConfig); err == nil {
				// Validate structure before auto-save
				var serversData map[string]interface{}
				if mcpServers, ok := rawConfig["mcpServers"].(map[string]interface{}); ok {
					serversData = mcpServers
				} else if servers, ok := rawConfig["servers"].(map[string]interface{}); ok {
					serversData = servers
				} else {
					log.Printf("DEBUG: Invalid config format during auto-save, skipping")
					serversData = nil
				}
				
				if serversData != nil && len(serversData) > 0 {
					log.Printf("DEBUG: Auto-saving config on ESC")
					
					// Convert from UI format to internal format and save
					var configData models.MCPConfigData
					configData.Servers = make(map[string]models.MCPServerConfig)
					// Convert servers to the expected format
					for serverName, serverConfigRaw := range serversData {
						serverConfigMap, ok := serverConfigRaw.(map[string]interface{})
						if !ok {
							continue
						}
						
						serverConfig := models.MCPServerConfig{}
						if command, ok := serverConfigMap["command"].(string); ok {
							serverConfig.Command = command
						}
						if argsRaw, ok := serverConfigMap["args"].([]interface{}); ok {
							for _, arg := range argsRaw {
								if argStr, ok := arg.(string); ok {
									serverConfig.Args = append(serverConfig.Args, argStr)
								}
							}
						}
						if envRaw, ok := serverConfigMap["env"].(map[string]interface{}); ok {
							serverConfig.Env = make(map[string]string)
							for k, v := range envRaw {
								if vStr, ok := v.(string); ok {
									serverConfig.Env[k] = vStr
								}
							}
						}
						
						configData.Servers[serverName] = serverConfig
					}
					
					// Add the config name to the data structure
					configData.Name = configName
					
					// Save config using service (with proper encryption)
					savedConfig, err := m.mcpConfigSvc.UploadConfig(m.selectedEnvID, &configData)
					if err != nil {
						log.Printf("DEBUG: Auto-save failed: %v", err)
					} else {
						log.Printf("DEBUG: Auto-saved config '%s' v%d on ESC", configName, savedConfig.Version)
						
						// Skip tool replacement on auto-save to avoid UI freeze
						// Tools will be discovered when user manually saves or refreshes
						log.Printf("DEBUG: Skipping tool replacement on auto-save to prevent UI freeze")
					}
				}
			}
		} else if configText != "" && configName != "" {
			log.Printf("DEBUG: No changes detected - skipping auto-save on ESC (name unchanged: %v, config unchanged: %v)", 
				configName == m.originalName, configText == m.originalConfig)
		}
		
		// Exit to list mode
		m.mode = MCPModeList
		m.showEditor = false
		m.nameInput.Blur()
		m.configEditor.Blur()
		m.focusedField = MCPFieldName
		// Reload configs when exiting editor to show any newly saved configs
		return m, m.loadConfigs()
	}
	return m, nil
}

// Handle field input based on focused field
func (m *MCPModel) handleFieldInput(msg tea.KeyMsg, cmds []tea.Cmd) (TabModel, tea.Cmd) {
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
		
	case MCPFieldVersions:
		// Handle version selection with up/down keys
		if msg.String() == "up" || msg.String() == "k" {
			if len(m.configVersions) > 0 {
				m.selectedVersionIdx = (m.selectedVersionIdx - 1 + len(m.configVersions)) % len(m.configVersions)
			}
		} else if msg.String() == "down" || msg.String() == "j" {
			if len(m.configVersions) > 0 {
				m.selectedVersionIdx = (m.selectedVersionIdx + 1) % len(m.configVersions)
			}
		}
		
	case MCPFieldConfig:
		// Clear error when user starts typing
		if msg.Type == tea.KeyRunes || msg.String() == "backspace" || msg.String() == "delete" {
			m.SetError("")
		}
		// Handle save keys in config field before passing to textarea
		if msg.String() == "ctrl+s" || msg.String() == "ctrl+enter" {
			return m, m.saveConfig()
		}
		var cmd tea.Cmd
		m.configEditor, cmd = m.configEditor.Update(msg)
		cmds = append(cmds, cmd)
	}
	
	return m, tea.Batch(cmds...)
}

// Handle config saved message
func (m *MCPModel) handleConfigSavedMsg(msg MCPConfigSavedMsg) (TabModel, tea.Cmd) {
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
}

// Handle configs loaded message
func (m *MCPModel) handleConfigsLoadedMsg(msg MCPConfigsLoadedMsg) (TabModel, tea.Cmd) {
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
}

// Handle config deleted message
func (m *MCPModel) handleConfigDeletedMsg(msg MCPConfigDeletedMsg) (TabModel, tea.Cmd) {
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

// Handle config versions loaded message
func (m *MCPModel) handleConfigVersionsLoadedMsg(msg MCPConfigVersionsLoadedMsg) (TabModel, tea.Cmd) {
	log.Printf("DEBUG: Received MCPConfigVersionsLoadedMsg with %d versions", len(msg.Versions))
	if msg.Error != nil {
		log.Printf("DEBUG: Error loading config versions: %v", msg.Error)
		return m, tea.Printf("Error loading versions: %v", msg.Error)
	}
	
	// Update versions list and reset selection
	m.configVersions = msg.Versions
	m.selectedVersionIdx = 0
	
	// Convert to list items for the version list component
	versionItems := make([]list.Item, len(msg.Versions))
	for i, version := range msg.Versions {
		versionItems[i] = VersionItem{config: version}
	}
	m.versionList.SetItems(versionItems)
	
	log.Printf("DEBUG: Config versions loaded successfully")
	return m, nil
}

// Handle version loaded message  
func (m *MCPModel) handleVersionLoadedMsg(msg MCPVersionLoadedMsg) (TabModel, tea.Cmd) {
	log.Printf("DEBUG: Received MCPVersionLoadedMsg")
	if msg.Error != nil {
		log.Printf("DEBUG: Error loading version into form: %v", msg.Error)
		return m, tea.Printf("Error loading version: %v", msg.Error)
	}
	
	// Load the selected version into the form
	m.configEditor.SetValue(msg.ConfigJSON)
	// Don't change the name input since user might want to save as new version
	
	// Update original values for change detection
	m.originalConfig = msg.ConfigJSON
	// Keep the current name as original since user might want to create new version
	m.originalName = m.nameInput.Value()
	
	log.Printf("DEBUG: Version loaded into form successfully")
	return m, tea.Printf("✅ Version loaded - modify and save to create new version")
}

// Handle tool discovery completion message
func (m *MCPModel) handleToolDiscoveryCompletedMsg(msg MCPToolDiscoveryCompletedMsg) (TabModel, tea.Cmd) {
	log.Printf("DEBUG: Received MCPToolDiscoveryCompletedMsg for '%s', success: %v", msg.ConfigName, msg.Success)
	
	if msg.Error != nil {
		log.Printf("DEBUG: Tool discovery error: %v", msg.Error)
		return m, tea.Batch(
			tea.Printf("❌ Tool discovery failed for '%s': %v", msg.ConfigName, msg.Error),
			m.loadConfigs(), // Reload to update status indicators
		)
	}
	
	log.Printf("DEBUG: Tool discovery successful, reloading configs to update status indicators")
	return m, tea.Batch(
		tea.Printf("✅ Discovered %d tools for '%s'", msg.ToolCount, msg.ConfigName),
		m.loadConfigs(), // Reload to update status indicators
	)
}