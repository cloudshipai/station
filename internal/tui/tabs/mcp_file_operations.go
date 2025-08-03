package tabs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	
	"station/pkg/config"
	"station/pkg/models"
)

// Load file-based configurations for TUI display
func (m MCPModel) loadFileConfigs() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: Loading file-based configs for TUI")
		
		// Use shared services for file config operations
		
		// Load configurations from all environments
		configMap := make(map[string]*MCPConfigDisplay)
		
		// Get all environments first
		envs, err := m.repos.Environments.List()
		if err != nil {
			log.Printf("DEBUG: Failed to load environments: %v", err)
			return MCPConfigsLoadedMsg{Error: err}
		}
		
		// For each environment, load file-based configs
		for _, env := range envs {
			configs, err := m.fileConfigService.ListFileConfigs(context.Background(), env.ID)
			if err != nil {
				log.Printf("DEBUG: Failed to load file configs for env %d: %v", env.ID, err)
				continue
			}
			
			// Convert to display format
			for _, config := range configs {
				// Generate a simple version number based on file modification time
				version := int64(1) // Default version for file-based configs
				
				// Create display item
				displayConfig := &MCPConfigDisplay{
					ID:              int64(len(configMap) + 1), // Simple ID for display
					Name:            config.Name,
					Version:         version,
					Updated:         "file-based",
					Size:            "N/A", // Size not applicable for file-based configs
					ConfigJSON:      "", // Will be loaded when needed
					EnvironmentID:   env.ID,
					EnvironmentName: env.Name,
					ToolStatus:      ToolStatusUnknown, // Tool status will be determined separately
					ToolCount:       0,
				}
				
				// Get tool count for this config if available
				displayConfig.ToolCount = m.getFileConfigToolCount(env.ID, config.Name)
				if displayConfig.ToolCount > 0 {
					displayConfig.ToolStatus = ToolStatusSuccess
				} else {
					displayConfig.ToolStatus = ToolStatusUnknown
				}
				
				// Keep only latest version of each named config (for display purposes)
				if existing, exists := configMap[config.Name]; !exists || version > existing.Version {
					configMap[config.Name] = displayConfig
				}
			}
		}
		
		// Convert map to slice
		var displayConfigs []MCPConfigDisplay
		for _, config := range configMap {
			displayConfigs = append(displayConfigs, *config)
		}
		
		log.Printf("DEBUG: Loaded %d file-based configs for TUI display", len(displayConfigs))
		return MCPConfigsLoadedMsg{Configs: displayConfigs}
	})
}

// Save configuration using file-based system
func (m MCPModel) saveFileConfig() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: saveFileConfig() called")
		
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
		
		// Get config name
		configName := m.nameInput.Value()
		if configName == "" {
			configName = "Unnamed Configuration"
		}
		log.Printf("DEBUG: Config name: '%s'", configName)
		
		// Validate that the JSON contains the required structure
		var serversData map[string]interface{}
		var rawConfig map[string]interface{}
		if err := json.Unmarshal([]byte(configText), &rawConfig); err != nil {
			return MCPConfigSavedMsg{Error: fmt.Sprintf("Invalid JSON: %v", err)}
		}
		
		if mcpServers, ok := rawConfig["mcpServers"].(map[string]interface{}); ok {
			serversData = mcpServers
		} else if servers, ok := rawConfig["servers"].(map[string]interface{}); ok {
			serversData = servers
		} else {
			errorMsg := "Invalid config format: must contain 'mcpServers' or 'servers' field"
			log.Printf("DEBUG: Config structure validation failed: %s", errorMsg)
			return MCPConfigSavedMsg{Error: errorMsg}
		}
		
		// Validate that there's at least one server
		if len(serversData) == 0 {
			errorMsg := "Configuration must contain at least one server"
			log.Printf("DEBUG: Server count validation failed: %s", errorMsg)
			return MCPConfigSavedMsg{Error: errorMsg}
		}
		
		// Convert from UI format to file-based template
		template := &config.MCPTemplate{
			Name:      configName,
			Content:   configText,
			Variables: []config.TemplateVariable{}, // No variables for basic templates
			Metadata: config.TemplateMetadata{
				Description: fmt.Sprintf("Template for %s", configName),
				Version:     "1.0.0",
			},
		}
		
		// Create variables map (empty for now, could be enhanced later)
		variables := make(map[string]interface{})
		
		// Save using file config service
		err := m.fileConfigService.CreateOrUpdateTemplate(context.Background(), m.selectedEnvID, configName, template, variables)
		if err != nil {
			log.Printf("DEBUG: Failed to save file config: %v", err)
			return MCPConfigSavedMsg{Error: fmt.Sprintf("Failed to save config: %v", err)}
		}
		log.Printf("DEBUG: Saved file-based config successfully")
		
		// Start background tool discovery after save (non-blocking)
		log.Printf("DEBUG: Starting background tool discovery for config '%s'", configName)
		go func() {
			result, err := m.fileConfigService.DiscoverToolsForConfig(context.Background(), m.selectedEnvID, configName)
			if err != nil {
				log.Printf("ERROR: Background tool discovery failed for '%s': %v", configName, err)
			} else {
				log.Printf("DEBUG: Background tool discovery completed for '%s' - %d tools discovered", 
					configName, result.TotalTools)
			}
		}()

		// TODO: Trigger async MCP manager reinitialization to refresh available tools
		// Note: ReinitializeMCP method not available in AgentServiceInterface
		if m.genkitService != nil {
			log.Printf("DEBUG: Would reinitialize MCP manager after config save (not implemented in interface)")
		}
		
		// Create display item for response
		newConfig := MCPConfigDisplay{
			ID:      int64(len(m.configs) + 1), // Simple ID for display
			Name:    configName,
			Version: 1,
			Updated: time.Now().Format("Jan 2 15:04"),
			Size:    fmt.Sprintf("%.1fKB", float64(len(configText))/1024),
		}
		
		result := MCPConfigSavedMsg{
			Config:  newConfig,
			Message: fmt.Sprintf("Configuration '%s' saved successfully", configName),
		}
		log.Printf("DEBUG: Returning MCPConfigSavedMsg: %+v", result)
		return result
	})
}

// Delete file-based configuration
func (m MCPModel) deleteFileConfig(configName string, environmentID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: Deleting file-based config '%s' in environment %d", configName, environmentID)
		
		// For file-based configs, we need to determine what to delete
		// This could involve removing template files, clearing tools, etc.
		
		// Get the file config record first to get file config ID
		fileConfig, err := m.repos.FileMCPConfigs.GetByEnvironmentAndName(environmentID, configName)
		if err != nil {
			log.Printf("DEBUG: Failed to get file config record for '%s': %v", configName, err)
			return MCPConfigDeletedMsg{
				ConfigID: 0, // No specific ID for file configs
				Error:    err,
				Message:  fmt.Sprintf("Configuration '%s' not found: %v", configName, err),
			}
		}
		
		// Clear existing tools for this file config
		if err := m.repos.MCPTools.DeleteByFileConfigID(fileConfig.ID); err != nil {
			log.Printf("DEBUG: Failed to delete tools for file config '%s': %v", configName, err)
		} else {
			log.Printf("DEBUG: Deleted tools for file config '%s'", configName)
		}
		
		// Delete file config record from database
		if err := m.repos.FileMCPConfigs.Delete(fileConfig.ID); err != nil {
			log.Printf("DEBUG: Failed to delete file config record '%s': %v", configName, err)
		} else {
			log.Printf("DEBUG: Deleted file config record for '%s'", configName)
		}
		
		// Note: We don't delete the actual template files as they might be used by other environments
		// or be part of a GitOps workflow
		
		// TODO: Trigger async MCP manager reinitialization to remove cached tools
		// Note: ReinitializeMCP method not available in AgentServiceInterface
		if m.genkitService != nil {
			log.Printf("DEBUG: Would reinitialize MCP manager after config deletion (not implemented in interface)")
		}
		
		return MCPConfigDeletedMsg{
			ConfigID: fileConfig.ID,
			Message:  fmt.Sprintf("Configuration '%s' deleted successfully", configName),
		}
	})
}

// Load file config content for editing
func (m MCPModel) loadFileConfigContent(configName string, environmentID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: Loading file config content for '%s' in environment %d", configName, environmentID)
		
		// Load and render the config using file service
		renderedConfig, err := m.fileConfigService.LoadAndRenderConfig(context.Background(), environmentID, configName)
		if err != nil {
			log.Printf("DEBUG: Failed to load file config content: %v", err)
			return MCPVersionLoadedMsg{Error: err}
		}
		
		// Convert rendered config back to JSON for editing
		configJSON, err := json.MarshalIndent(map[string]interface{}{
			"mcpServers": renderedConfig.Servers,
		}, "", "  ")
		if err != nil {
			log.Printf("DEBUG: Failed to marshal config to JSON: %v", err)
			return MCPVersionLoadedMsg{Error: err}
		}
		
		return MCPVersionLoadedMsg{
			ConfigJSON: string(configJSON),
			ConfigName: configName,
		}
	})
}

// Discover tools for a file-based config
func (m MCPModel) discoverFileConfigTools(configName string, environmentID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: Running tool discovery for file config '%s'", configName)
		
		result, err := m.fileConfigService.DiscoverToolsForConfig(context.Background(), environmentID, configName)
		if err != nil {
			log.Printf("ERROR: Tool discovery failed for '%s': %v", configName, err)
			return MCPToolDiscoveryCompletedMsg{
				ConfigName: configName,
				Success:    false,
				Error:      err,
			}
		}
		
		log.Printf("DEBUG: Tool discovery completed for '%s' - %d tools discovered", 
			configName, result.TotalTools)
		
		return MCPToolDiscoveryCompletedMsg{
			ConfigName: configName,
			Success:    true,
			ToolCount:  result.TotalTools,
		}
	})
}

// Get tool count for a file config
func (m MCPModel) getFileConfigToolCount(environmentID int64, configName string) int {
	// Get the file config record
	fileConfig, err := m.repos.FileMCPConfigs.GetByEnvironmentAndName(environmentID, configName)
	if err != nil {
		log.Printf("DEBUG: Failed to get file config record for '%s': %v", configName, err)
		return 0
	}
	
	// Get tools linked to this file config
	tools, err := m.repos.MCPTools.GetByFileConfigID(fileConfig.ID)
	if err != nil {
		log.Printf("DEBUG: Failed to get tools for file config '%s': %v", configName, err)
		return 0
	}
	
	return len(tools)
}

// Helper function to determine config type (for future use)
func (m MCPModel) getConfigType(config MCPConfigDisplay) string {
	// File-based configs can be identified by their source
	return "file-based"
}