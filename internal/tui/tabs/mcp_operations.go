package tabs

import (
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
)

// Load configurations using file-based system
func (m MCPModel) loadConfigs() tea.Cmd {
	// Use the new file-based operations
	return m.loadFileConfigs()
}

// Load environments from database
func (m *MCPModel) loadEnvironments() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		envs, err := m.repos.Environments.List()
		if err != nil {
			return MCPEnvironmentsLoadedMsg{Error: err}
		}
		
		return MCPEnvironmentsLoadedMsg{Environments: envs}
	})
}

// Save configuration using file-based system
func (m MCPModel) saveConfig() tea.Cmd {
	// Use the new file-based operations
	return m.saveFileConfig()
}

// Delete configuration using file-based system
func (m MCPModel) deleteConfig(configID int64) tea.Cmd {
	// For file-based configs, we need to get the config name from the display list
	// since configID is just a display ID, not a real database ID
	var configName string
	var environmentID int64
	
	// Find the config in our display list to get the name
	for _, config := range m.configs {
		if config.ID == configID {
			configName = config.Name
			environmentID = config.EnvironmentID
			break
		}
	}
	
	if configName == "" {
		return tea.Cmd(func() tea.Msg {
			return MCPConfigDeletedMsg{
				ConfigID: configID,
				Error:    fmt.Errorf("config not found"),
				Message:  "Configuration not found for deletion",
			}
		})
	}
	
	// Use the new file-based delete operation
	return m.deleteFileConfig(configName, environmentID)
}

// loadConfigVersions for file-based configs (simplified - typically only one version)
func (m MCPModel) loadConfigVersions(configName string, environmentID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: Loading versions for file config '%s' in environment %d", configName, environmentID)
		
		// For file-based configs, we typically just have one "version" - the current file
		// Create a single version entry
		var versions []MCPConfigDisplay
		
		// Find the config in our current list to get display info
		for _, config := range m.configs {
			if config.Name == configName && config.EnvironmentID == environmentID {
				versions = append(versions, config)
				break
			}
		}
		
		log.Printf("DEBUG: Found %d versions for file config '%s'", len(versions), configName)
		return MCPConfigVersionsLoadedMsg{Versions: versions}
	})
}

// loadVersionIntoForm loads file config content for editing
func (m MCPModel) loadVersionIntoForm(configID int64) tea.Cmd {
	// Find the config in our versions list to get name and environment
	var configName string
	var environmentID int64
	
	for _, version := range m.configVersions {
		if version.ID == configID {
			configName = version.Name
			environmentID = version.EnvironmentID
			break
		}
	}
	
	if configName == "" {
		return tea.Cmd(func() tea.Msg {
			log.Printf("DEBUG: Config ID %d not found in versions list", configID)
			return MCPVersionLoadedMsg{Error: fmt.Errorf("version not found")}
		})
	}
	
	// Use the new file-based content loading
	return m.loadFileConfigContent(configName, environmentID)
}

// getToolExtractionStatus checks the tool extraction status for file-based configs
func (m MCPModel) getToolExtractionStatus(configID int64) (ToolExtractionStatus, int) {
	// For file-based configs, the configID is just a display ID
	// We need to find the actual config info from our display list
	var configName string
	var environmentID int64
	
	for _, config := range m.configs {
		if config.ID == configID {
			configName = config.Name
			environmentID = config.EnvironmentID
			break
		}
	}
	
	if configName == "" {
		log.Printf("DEBUG: Failed to find config with display ID %d", configID)
		return ToolStatusUnknown, 0
	}
	
	// Get tool count using file config approach
	toolCount := m.getFileConfigToolCount(environmentID, configName)
	
	// Determine status based on tool count
	if toolCount == 0 {
		return ToolStatusUnknown, 0
	} else {
		return ToolStatusSuccess, toolCount
	}
}

// cleanupOrphanedAgentTools removes agent tool assignments that reference tools which no longer exist
// This is handled by the file config deletion process in mcp_file_operations.go
func (m MCPModel) cleanupOrphanedAgentTools(environmentID int64) error {
	// For file-based configs, orphaned tool cleanup is handled by the
	// deleteFileConfig function in mcp_file_operations.go
	log.Printf("DEBUG: Orphaned agent tools cleanup delegated to file config deletion process")
	return nil
}