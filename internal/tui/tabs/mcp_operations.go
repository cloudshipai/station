package tabs

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	
	"station/pkg/models"
)

// Load configurations from database
func (m MCPModel) loadConfigs() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: Loading configs from database")
		
		// Use shared services for decryption
		
		// Load all configs from all environments and group them by name
		configMap := make(map[string]*MCPConfigDisplay)
		
		// Get all environments first
		envs, err := m.repos.Environments.List()
		if err != nil {
			log.Printf("DEBUG: Failed to load environments: %v", err)
			return MCPConfigsLoadedMsg{Error: err}
		}
		
		// For each environment, load only the latest configs
		for _, env := range envs {
			configs, err := m.repos.MCPConfigs.GetLatestConfigs(env.ID)
			if err != nil {
				log.Printf("DEBUG: Failed to load latest configs for env %d: %v", env.ID, err)
				continue
			}
			
			// Convert to display format - these are already the latest versions
			for _, config := range configs {
				// Decrypt the config to extract name and JSON for display
				var configJSON string
				var configName string = fmt.Sprintf("config-%d", config.ID) // Default fallback
				
				if config.EncryptionKeyID == "" {
					// Config is not encrypted - parse to extract name and format for UI
					var rawConfig map[string]interface{}
					if err := json.Unmarshal([]byte(config.ConfigJSON), &rawConfig); err == nil {
						// Extract name if present
						if name, ok := rawConfig["name"].(string); ok && name != "" {
							configName = name
						}
						
						// Convert to UI format if needed
						if _, ok := rawConfig["mcpServers"]; ok {
							// Already in UI format
							configJSON = config.ConfigJSON
						} else if servers, ok := rawConfig["servers"]; ok {
							// Convert from internal format to UI format
							uiFormat := map[string]interface{}{
								"mcpServers": servers,
							}
							if uiJSON, err := json.MarshalIndent(uiFormat, "", "  "); err == nil {
								configJSON = string(uiJSON)
							} else {
								configJSON = config.ConfigJSON // fallback
							}
						} else {
							configJSON = config.ConfigJSON // fallback
						}
					} else {
						configJSON = config.ConfigJSON // fallback on parse error
					}
				} else {
					// Config is encrypted, decrypt it
					decryptedData, err := m.mcpConfigSvc.DecryptConfigWithKeyID(config.ConfigJSON, config.EncryptionKeyID)
					if err != nil {
						log.Printf("DEBUG: Failed to decrypt config %d: %v", config.ID, err)
						configName = fmt.Sprintf("config-%d (encrypted)", config.ID)
						configJSON = "{\"error\": \"failed to decrypt\"}"
					} else {
						// Use the name from the decrypted data if available
						if decryptedData.Name != "" {
							configName = decryptedData.Name
						}
						
						// Convert from internal format (servers) to UI format (mcpServers)
						uiFormat := map[string]interface{}{
							"mcpServers": decryptedData.Servers,
						}
						
						if decryptedJSON, err := json.MarshalIndent(uiFormat, "", "  "); err == nil {
							configJSON = string(decryptedJSON)
						} else {
							log.Printf("DEBUG: Failed to serialize decrypted config %d: %v", config.ID, err)  
							configJSON = "{\"error\": \"failed to serialize\"}"
						}
						
						// For encrypted configs, use a generated name since we don't store name separately
						// In the future, we could enhance this to extract name from decrypted data
					}
				}
				
				// Check tool extraction status for this config
				toolStatus, toolCount := m.getToolExtractionStatus(config.ID)
				
				// Create display item
				displayConfig := &MCPConfigDisplay{
					ID:              config.ID,
					Name:            configName,
					Version:         config.Version,
					Updated:         config.UpdatedAt.Format("Jan 2 15:04"),
					Size:            fmt.Sprintf("%.1fKB", float64(len(configJSON))/1024),
					ConfigJSON:      configJSON, // Use the decrypted JSON for display
					EnvironmentID:   config.EnvironmentID,
					EnvironmentName: env.Name,
					ToolStatus:      toolStatus,
					ToolCount:       toolCount,
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

// Delete configuration with cascade deletion of servers and tools
func (m MCPModel) deleteConfig(configID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: Deleting MCP config %d with cascade deletion", configID)
		
		// First, get the config to extract its name and environment
		targetConfig, err := m.repos.MCPConfigs.GetByID(configID)
		if err != nil {
			log.Printf("DEBUG: Failed to get config %d: %v", configID, err)
			return MCPConfigDeletedMsg{
				ConfigID: configID,
				Error:    err,
				Message:  fmt.Sprintf("Configuration not found: %v", err),
			}
		}
		
		// Use the database config_name column directly
		configName := targetConfig.ConfigName
		if configName == "" {
			configName = fmt.Sprintf("config-%d", configID)
		}
		log.Printf("DEBUG: Config name from database: '%s', will delete all versions", configName)
		
		// Get all configs with the same name in the same environment
		allConfigs, err := m.repos.MCPConfigs.ListByEnvironment(targetConfig.EnvironmentID)
		if err != nil {
			log.Printf("DEBUG: Failed to get configs for environment %d: %v", targetConfig.EnvironmentID, err)
			return MCPConfigDeletedMsg{
				ConfigID: configID,
				Error:    err,
				Message:  fmt.Sprintf("Failed to get configs for deletion: %v", err),
			}
		}
		
		// Find all configs with the same name using database column
		var configsToDelete []*models.MCPConfig
		for _, config := range allConfigs {
			otherConfigName := config.ConfigName
			if otherConfigName == "" {
				otherConfigName = fmt.Sprintf("config-%d", config.ID)
			}
			
			if otherConfigName == configName {
				configsToDelete = append(configsToDelete, config)
			}
		}
		
		log.Printf("DEBUG: Found %d config versions to delete with name '%s'", len(configsToDelete), configName)
		
		// Delete each config version with cascade deletion
		for _, config := range configsToDelete {
			log.Printf("DEBUG: Deleting config version %d (ID: %d)", config.Version, config.ID)
			
			// Get all servers for this config to cascade delete tools
			servers, err := m.repos.MCPServers.GetByConfigID(config.ID)
			if err != nil {
				log.Printf("DEBUG: Failed to get servers for config %d: %v", config.ID, err)
				return MCPConfigDeletedMsg{
					ConfigID: configID,
					Error:    err,
					Message:  fmt.Sprintf("Failed to get servers for config %d: %v", config.ID, err),
				}
			}
			
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
			if err := m.repos.MCPServers.DeleteByConfigID(config.ID); err != nil {
				log.Printf("DEBUG: Failed to delete servers for config %d: %v", config.ID, err)
				return MCPConfigDeletedMsg{
					ConfigID: configID,
					Error:    err,
					Message:  fmt.Sprintf("Failed to delete servers for config %d: %v", config.ID, err),
				}
			}
			
			// Delete the config itself
			if err := m.repos.MCPConfigs.Delete(config.ID); err != nil {
				log.Printf("DEBUG: Failed to delete config %d: %v", config.ID, err)
				return MCPConfigDeletedMsg{
					ConfigID: configID,
					Error:    err,
					Message:  fmt.Sprintf("Failed to delete configuration %d: %v", config.ID, err),
				}
			}
			
			log.Printf("DEBUG: Successfully deleted config version %d (ID: %d)", config.Version, config.ID)
		}
		
		log.Printf("DEBUG: Successfully deleted all %d versions of config '%s'", len(configsToDelete), configName)
		return MCPConfigDeletedMsg{
			ConfigID: configID,
			Message:  fmt.Sprintf("Configuration '%s' (all %d versions) deleted successfully", configName, len(configsToDelete)),
		}
	})
}

// loadConfigVersions loads all versions of a specific config name
func (m MCPModel) loadConfigVersions(configName string, environmentID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: Loading all versions for config '%s' in environment %d", configName, environmentID)
		
		// Get all configs for this environment
		allConfigs, err := m.repos.MCPConfigs.ListByEnvironment(environmentID)
		if err != nil {
			log.Printf("DEBUG: Failed to load configs for environment %d: %v", environmentID, err)
			return MCPConfigVersionsLoadedMsg{Error: err}
		}
		
		var versions []MCPConfigDisplay
		
		// Filter configs by name and convert to display format
		for _, config := range allConfigs {
			// Extract config name for comparison
			var currentConfigName string = fmt.Sprintf("config-%d", config.ID)
			
			if config.EncryptionKeyID == "" {
				// Config is not encrypted - parse to extract name
				var rawConfig map[string]interface{}
				if err := json.Unmarshal([]byte(config.ConfigJSON), &rawConfig); err == nil {
					if name, ok := rawConfig["name"].(string); ok && name != "" {
						currentConfigName = name
					}
				}
			} else {
				// Config is encrypted, decrypt to get name
				decryptedData, err := m.mcpConfigSvc.DecryptConfigWithKeyID(config.ConfigJSON, config.EncryptionKeyID)
				if err != nil {
					log.Printf("DEBUG: Failed to decrypt config %d for name extraction: %v", config.ID, err)
					currentConfigName = fmt.Sprintf("config-%d", config.ID)
				} else if decryptedData.Name != "" {
					currentConfigName = decryptedData.Name
				}
			}
			
			// Only include configs with matching names
			if currentConfigName == configName {
				// Decrypt for display
				var configJSON string
				if config.EncryptionKeyID == "" {
					// Not encrypted - convert to UI format if needed
					var rawConfig map[string]interface{}
					if err := json.Unmarshal([]byte(config.ConfigJSON), &rawConfig); err == nil {
						if _, ok := rawConfig["mcpServers"]; ok {
							configJSON = config.ConfigJSON
						} else if servers, ok := rawConfig["servers"]; ok {
							uiFormat := map[string]interface{}{"mcpServers": servers}
							if uiJSON, err := json.MarshalIndent(uiFormat, "", "  "); err == nil {
								configJSON = string(uiJSON)
							} else {
								configJSON = config.ConfigJSON
							}
						} else {
							configJSON = config.ConfigJSON
						}
					} else {
						configJSON = config.ConfigJSON
					}
				} else {
					// Encrypted - decrypt and convert to UI format
					decryptedData, err := m.mcpConfigSvc.DecryptConfigWithKeyID(config.ConfigJSON, config.EncryptionKeyID)
					if err != nil {
						log.Printf("DEBUG: Failed to decrypt config %d: %v", config.ID, err)
						configJSON = "{\"error\": \"failed to decrypt\"}"
					} else {
						uiFormat := map[string]interface{}{"mcpServers": decryptedData.Servers}
						if decryptedJSON, err := json.MarshalIndent(uiFormat, "", "  "); err == nil {
							configJSON = string(decryptedJSON)
						} else {
							configJSON = "{\"error\": \"failed to serialize\"}"
						}
					}
				}
				
				// Check tool extraction status for this version
				toolStatus, toolCount := m.getToolExtractionStatus(config.ID)
				
				displayConfig := MCPConfigDisplay{
					ID:              config.ID,
					Name:            currentConfigName,
					Version:         config.Version,
					Updated:         config.UpdatedAt.Format("Jan 2 15:04"),
					Size:            fmt.Sprintf("%.1fKB", float64(len(configJSON))/1024),
					ConfigJSON:      configJSON,
					EnvironmentID:   config.EnvironmentID,
					EnvironmentName: "Environment", // Could be enhanced to get actual env name
					ToolStatus:      toolStatus,
					ToolCount:       toolCount,
				}
				versions = append(versions, displayConfig)
			}
		}
		
		log.Printf("DEBUG: Found %d versions for config '%s'", len(versions), configName)
		return MCPConfigVersionsLoadedMsg{Versions: versions}
	})
}

// loadVersionIntoForm loads a specific version into the form for editing
func (m MCPModel) loadVersionIntoForm(configID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: Loading config ID %d into form", configID)
		
		// Find the config in our versions list
		for _, version := range m.configVersions {
			if version.ID == configID {
				log.Printf("DEBUG: Found version v%d, loading into form", version.Version)
				return MCPVersionLoadedMsg{
					ConfigJSON: version.ConfigJSON,
					ConfigName: version.Name,
				}
			}
		}
		
		log.Printf("DEBUG: Config ID %d not found in versions list", configID)
		return MCPVersionLoadedMsg{Error: fmt.Errorf("version not found")}
	})
}

// getToolExtractionStatus checks the tool extraction status for a given config
func (m MCPModel) getToolExtractionStatus(configID int64) (ToolExtractionStatus, int) {
	// Get all servers for this config
	servers, err := m.repos.MCPServers.GetByConfigID(configID)
	if err != nil {
		log.Printf("DEBUG: Failed to get servers for config %d: %v", configID, err)
		return ToolStatusUnknown, 0
	}
	
	if len(servers) == 0 {
		// No servers means no tool extraction attempted
		return ToolStatusUnknown, 0
	}
	
	totalTools := 0
	serverCount := len(servers)
	serversWithTools := 0
	
	// Check each server for tools
	for _, server := range servers {
		tools, err := m.repos.MCPTools.GetByServerID(server.ID)
		if err != nil {
			log.Printf("DEBUG: Failed to get tools for server %d: %v", server.ID, err)
			continue
		}
		
		toolCount := len(tools)
		totalTools += toolCount
		
		if toolCount > 0 {
			serversWithTools++
		}
	}
	
	// Determine status based on results
	if totalTools == 0 {
		// No tools extracted from any server
		return ToolStatusFailed, 0
	} else if serversWithTools == serverCount {
		// All servers have tools
		return ToolStatusSuccess, totalTools
	} else {
		// Some servers have tools, some don't
		return ToolStatusPartial, totalTools
	}
}