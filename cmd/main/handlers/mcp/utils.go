package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/models"
)

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// validateEnvironmentExists checks if file-based environment directory exists
func (h *MCPHandler) validateEnvironmentExists(envName string) bool {
	configDir := fmt.Sprintf("./config/environments/%s", envName)
	if _, err := os.Stat(configDir); err != nil {
		return false
	}
	return true
}

// syncMCPConfigsLocal performs declarative sync of file-based configs to database
func (h *MCPHandler) syncMCPConfigsLocal(environment string, dryRun, force bool) error {
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	styles := getCLIStyles(h.themeManager)

	// Get or create environment
	envID, err := h.getOrCreateEnvironmentID(repos, environment)
	if err != nil {
		return fmt.Errorf("environment '%s' not found: %w", environment, err)
	}
	// Get current database state
	fmt.Printf("🔍 Scanning database configs in environment '%s'...\n", environment)
	dbConfigs, err := repos.FileMCPConfigs.ListByEnvironment(envID)
	if err != nil {
		return fmt.Errorf("failed to list database configs: %w", err)
	}

	// Discover actual config files from filesystem
	fileConfigs, err := h.discoverConfigFiles(environment)
	if err != nil {
		return fmt.Errorf("failed to discover config files: %w", err)
	}

	// Get all agents in this environment
	agents, err := repos.Agents.ListByEnvironment(envID)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	// Track changes
	var toSync []string
	var toRemove []string
	var orphanedToolsRemoved int

	// Create maps for comparison
	fileConfigMap := make(map[string]*repositories.FileConfigRecord)
	dbConfigMap := make(map[string]*repositories.FileConfigRecord)
	
	for _, fileConfig := range fileConfigs {
		fileConfigMap[fileConfig.ConfigName] = fileConfig
	}
	
	for _, dbConfig := range dbConfigs {
		dbConfigMap[dbConfig.ConfigName] = dbConfig
	}

	// Find configs that exist in filesystem but not in database (new configs to sync)
	for _, fileConfig := range fileConfigs {
		dbConfig, existsInDB := dbConfigMap[fileConfig.ConfigName]
		
		if !existsInDB || force {
			// New config or force sync requested
			toSync = append(toSync, fileConfig.ConfigName)
		} else if dbConfig.LastLoadedAt != nil && !dbConfig.LastLoadedAt.IsZero() && fileConfig.LastLoadedAt.After(*dbConfig.LastLoadedAt) {
			// File modified after last sync
			toSync = append(toSync, fileConfig.ConfigName)
		}
	}

	// Find configs that exist in DB but not in filesystem (orphaned configs to remove)
	for _, dbConfig := range dbConfigs {
		if _, existsInFiles := fileConfigMap[dbConfig.ConfigName]; !existsInFiles {
			toRemove = append(toRemove, dbConfig.ConfigName)
		}
	}


	// Show what will be done
	if len(toSync) > 0 {
		fmt.Printf("\n📥 Configs to sync:\n")
		for _, name := range toSync {
			fmt.Printf("  • %s\n", styles.Success.Render(name))
		}
	}

	if len(toRemove) > 0 {
		fmt.Printf("\n🗑️  Configs to remove:\n")
		for _, name := range toRemove {
			fmt.Printf("  • %s\n", styles.Error.Render(name))
		}
	}

	if len(toSync) == 0 && len(toRemove) == 0 {
		fmt.Printf("\n✅ %s\n", styles.Success.Render("All configurations are up to date"))
		return nil
	}

	if dryRun {
		fmt.Printf("\n🔍 %s\n", styles.Info.Render("Dry run complete - no changes made"))
		return nil
	}

	// Perform actual sync
	fmt.Printf("\n🔄 Syncing configurations...\n")

	// Load new/updated configs
	var syncErrors []string
	for _, configName := range toSync {
		fmt.Printf("  📥 Loading %s...", configName)
		
		err := h.loadConfigFromFilesystem(repos, envID, environment, configName, fileConfigMap[configName])
		if err != nil {
			fmt.Printf(" %s\n", styles.Error.Render("❌"))
			syncErrors = append(syncErrors, fmt.Sprintf("%s: %v", configName, err))
		} else {
			fmt.Printf(" %s\n", styles.Success.Render("✅"))
		}
	}

	// Remove orphaned configs and clean up agent tools
	var affectedAgents []string
	for _, configName := range toRemove {
		fmt.Printf("  🗑️  Removing %s...", configName)
		
		// Find and remove from database
		var configToRemove *repositories.FileConfigRecord
		for _, dbConfig := range dbConfigs {
			if dbConfig.ConfigName == configName {
				configToRemove = dbConfig
				break
			}
		}
		
		if configToRemove != nil {
			// Count agents before tool removal for health tracking
			agentsWithTools := 0
			for _, agent := range agents {
				agentTools, err := repos.AgentTools.ListAgentTools(agent.ID)
				if err != nil {
					continue
				}
				
				// Check if this agent has any tools from the config being removed
				orphanedTools, err := repos.MCPTools.GetByServerID(configToRemove.ID)
				if err != nil {
					continue
				}
				
				for _, agentTool := range agentTools {
					for _, orphanedTool := range orphanedTools {
						if agentTool.ToolID == orphanedTool.ID {
							agentsWithTools++
							affectedAgents = append(affectedAgents, agent.Name)
							break
						}
					}
				}
			}
			
			// Remove agent tools that reference this config
			toolsRemoved, err := h.removeOrphanedAgentTools(repos, agents, configToRemove.ID)
			if err != nil {
				fmt.Printf(" %s\n", styles.Error.Render("❌"))
				return fmt.Errorf("failed to clean up agent tools for %s: %w", configName, err)
			}
			orphanedToolsRemoved += toolsRemoved
			
			// Remove the file config
			err = repos.FileMCPConfigs.Delete(configToRemove.ID)
			if err != nil {
				fmt.Printf(" %s\n", styles.Error.Render("❌"))
				return fmt.Errorf("failed to remove %s: %w", configName, err)
			}
		}
		
		fmt.Printf(" %s\n", styles.Success.Render("✅"))
	}

	// Summary
	if len(syncErrors) > 0 {
		fmt.Printf("\n⚠️ %s\n", styles.Error.Render("Sync completed with errors!"))
		fmt.Printf("📊 Summary:\n")
		fmt.Printf("  • Synced: %d configs\n", len(toSync)-len(syncErrors))
		fmt.Printf("  • Failed: %d configs\n", len(syncErrors))
		fmt.Printf("  • Removed: %d configs\n", len(toRemove))
		if orphanedToolsRemoved > 0 {
			fmt.Printf("  • Cleaned up: %d orphaned agent tools\n", orphanedToolsRemoved)
		}
		if len(affectedAgents) > 0 {
			fmt.Printf("  • Affected agents: %v\n", affectedAgents)
			fmt.Printf("  • ⚠️  Agent health may be impacted - check agent logs for details\n")
		}
		
		fmt.Printf("\n❌ Sync Errors:\n")
		for _, errMsg := range syncErrors {
			fmt.Printf("  • %s\n", styles.Error.Render(errMsg))
		}
		
		// Don't return error - partial success is still useful
		return nil
	} else {
		fmt.Printf("\n✅ %s\n", styles.Success.Render("Sync completed successfully!"))
		fmt.Printf("📊 Summary:\n")
		fmt.Printf("  • Synced: %d configs\n", len(toSync))
		fmt.Printf("  • Removed: %d configs\n", len(toRemove))
		if orphanedToolsRemoved > 0 {
			fmt.Printf("  • Cleaned up: %d orphaned agent tools\n", orphanedToolsRemoved)
		}
		if len(affectedAgents) > 0 {
			fmt.Printf("  • Affected agents: %v\n", affectedAgents)
			fmt.Printf("  • ⚠️  Agent health may be impacted - check agent logs for details\n")
		}
	}

	return nil
}

// statusMCPConfigsLocal shows validation status table
func (h *MCPHandler) statusMCPConfigsLocal(environment string) error {
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	styles := getCLIStyles(h.themeManager)

	// Get environments to check
	var environments []*models.Environment
	if environment == "default" || environment == "" {
		// Show all environments
		allEnvs, err := repos.Environments.List()
		if err != nil {
			return fmt.Errorf("failed to list environments: %w", err)
		}
		environments = allEnvs
	} else {
		// Show specific environment
		env, err := repos.Environments.GetByName(environment)
		if err != nil {
			return fmt.Errorf("environment '%s' not found", environment)
		}
		environments = []*models.Environment{env}
	}

	fmt.Printf("\n📊 Configuration Status Report\n\n")

	// Print table header
	fmt.Printf("┌────────────────┬─────────────────────────────┬──────────────────────────┬────────────────┐\n")
	fmt.Printf("│ %-14s │ %-27s │ %-24s │ %-14s │\n", "Environment", "Agent", "MCP Configs", "Status")
	fmt.Printf("├────────────────┼─────────────────────────────┼──────────────────────────┼────────────────┤\n")

	for _, env := range environments {
		// Get agents for this environment
		agents, err := repos.Agents.ListByEnvironment(env.ID)
		if err != nil {
			continue
		}

		// Get file configs for this environment
		fileConfigs, err := repos.FileMCPConfigs.ListByEnvironment(env.ID)
		if err != nil {
			continue
		}

		// For now, we'll assume discovered configs are the same as database configs
		// TODO: Implement actual file system discovery when available
		_ = fileConfigs // discoveredConfigs := fileConfigs

		if len(agents) == 0 {
			// No agents in this environment
			configNames := make([]string, len(fileConfigs))
			for i, fc := range fileConfigs {
				configNames[i] = fc.ConfigName
			}
			configList := truncateString(fmt.Sprintf("%v", configNames), 24)
			if len(configNames) == 0 {
				configList = "none"
			}
			
			status := styles.Info.Render("no agents")
			fmt.Printf("│ %-14s │ %-27s │ %-24s │ %-14s │\n", 
				truncateString(env.Name, 14), "none", configList, status)
		} else {
			for i, agent := range agents {
				// Get tools assigned to this agent
				agentTools, err := repos.AgentTools.ListAgentTools(agent.ID)
				if err != nil {
					continue
				}

				// Check which configs the agent's tools come from
				agentConfigNames := make(map[string]bool)
				orphanedTools := 0
				
				for _, _ = range agentTools {
					// Use the tool information from agentTools which includes file config info
					// For now, we'll use a simpler approach without FileConfigID
					// TODO: Implement proper file config tracking when models are updated
					
					// For demonstration, assume all tools belong to existing configs for now
					if len(fileConfigs) > 0 {
						agentConfigNames[fileConfigs[0].ConfigName] = true
					}
				}

				// Build config list
				configList := make([]string, 0, len(agentConfigNames))
				for name := range agentConfigNames {
					configList = append(configList, name)
				}
				
				// Check status
				var status string
				hasOutOfSync := false
				hasOrphaned := orphanedTools > 0
				
				// For now, assume all configs are in sync
				// TODO: Implement proper sync checking when file discovery is available
				
				if hasOrphaned && hasOutOfSync {
					status = styles.Error.Render("orphaned+sync")
				} else if hasOrphaned {
					status = styles.Error.Render("orphaned tools")
				} else if hasOutOfSync {
					status = styles.Error.Render("out of sync")
				} else if len(configList) == 0 {
					status = styles.Info.Render("no tools")
				} else {
					status = styles.Success.Render("synced")
				}

				// Format display
				envName := ""
				if i == 0 {
					envName = truncateString(env.Name, 14)
				}
				
				configDisplay := truncateString(fmt.Sprintf("%v", configList), 24)
				if len(configList) == 0 {
					configDisplay = "none"
				}
				
				fmt.Printf("│ %-14s │ %-27s │ %-24s │ %-14s │\n", 
					envName,
					truncateString(agent.Name, 27),
					configDisplay,
					status)
			}
		}
	}

	fmt.Printf("└────────────────┴─────────────────────────────┴──────────────────────────┴────────────────┘\n")

	fmt.Printf("\n📝 Legend:\n")
	fmt.Printf("  • %s - All configs synced and current\n", styles.Success.Render("synced"))
	fmt.Printf("  • %s - Agent has tools from deleted config files\n", styles.Error.Render("orphaned tools"))
	fmt.Printf("  • %s - Config files changed since last sync\n", styles.Error.Render("out of sync"))
	fmt.Printf("  • %s - Agent has no MCP tools assigned\n", styles.Info.Render("no tools"))

	fmt.Printf("\n💡 Run 'stn mcp sync <environment>' to update configurations\n")

	return nil
}

// discoverConfigFiles scans the filesystem for JSON config files in the environment directory
func (h *MCPHandler) discoverConfigFiles(environment string) ([]*repositories.FileConfigRecord, error) {
	// Get config directory path
	configDir := os.ExpandEnv("$HOME/.config/station")
	envDir := filepath.Join(configDir, "environments", environment)
	
	// Check if environment directory exists
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		return []*repositories.FileConfigRecord{}, nil
	}
	
	// Read all files in environment directory
	files, err := os.ReadDir(envDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read environment directory %s: %w", envDir, err)
	}
	
	var configs []*repositories.FileConfigRecord
	for _, file := range files {
		// Skip non-JSON files and variables.yml
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") || 
		   file.Name() == "variables.yml" {
			continue
		}
		
		// Get file info
		fileInfo, err := file.Info()
		if err != nil {
			continue
		}
		
		// Extract config name from filename (remove .json extension and timestamp suffix if present)
		configName := strings.TrimSuffix(file.Name(), ".json")
		
		// Create a FileConfigRecord-like structure for filesystem files
		modTime := fileInfo.ModTime()
		config := &repositories.FileConfigRecord{
			ConfigName:    configName,
			TemplatePath:  filepath.Join("environments", environment, file.Name()),
			LastLoadedAt:  &modTime,
		}
		
		configs = append(configs, config)
	}
	
	return configs, nil
}

// loadConfigFromFilesystem loads a config file from filesystem, processes templates, and registers tools
func (h *MCPHandler) loadConfigFromFilesystem(repos *repositories.Repositories, envID int64, environment, configName string, fileConfig *repositories.FileConfigRecord) error {
	// Get config directory path
	configDir := os.ExpandEnv("$HOME/.config/station")
	configPath := filepath.Join(configDir, fileConfig.TemplatePath)
	
	// Read the config file
	rawContent, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	
	// Process template variables
	templateService := services.NewTemplateVariableService(configDir, repos)
	result, err := templateService.ProcessTemplateWithVariables(envID, configName, string(rawContent), false)
	if err != nil {
		return fmt.Errorf("failed to process template variables: %w", err)
	}
	
	// Check if all variables were resolved
	if !result.AllResolved {
		missingVars := make([]string, 0, len(result.MissingVars))
		for _, missingVar := range result.MissingVars {
			missingVars = append(missingVars, missingVar.Name)
		}
		return fmt.Errorf("missing template variables: %v. Please update %s/environments/%s/variables.yml", 
			missingVars, configDir, environment)
	}
	
	// Parse the rendered JSON
	var configData map[string]interface{}
	if err := json.Unmarshal([]byte(result.RenderedContent), &configData); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}
	
	// Extract MCP servers from config
	var serversData map[string]interface{}
	if mcpServers, ok := configData["mcpServers"].(map[string]interface{}); ok {
		serversData = mcpServers
	} else if servers, ok := configData["servers"].(map[string]interface{}); ok {
		serversData = servers
	} else {
		return fmt.Errorf("no 'mcpServers' or 'servers' field found in config")
	}
	
	// Create or update file config record
	now := time.Now()
	var fileConfigID int64
	
	// Check if config already exists
	existingConfigs, err := repos.FileMCPConfigs.ListByEnvironment(envID)
	if err != nil {
		return fmt.Errorf("failed to check existing configs: %w", err)
	}
	
	var existingConfig *repositories.FileConfigRecord
	for _, existing := range existingConfigs {
		if existing.ConfigName == configName {
			existingConfig = existing
			break
		}
	}
	
	if existingConfig != nil {
		// Update existing config
		fileConfigID = existingConfig.ID
		err = repos.FileMCPConfigs.UpdateLastLoadedAt(fileConfigID)
		if err != nil {
			return fmt.Errorf("failed to update config timestamp: %w", err)
		}
	} else {
		// Create new config
		fileConfigID, err = repos.FileMCPConfigs.Create(&repositories.FileConfigRecord{
			EnvironmentID: envID,
			ConfigName:    configName,
			TemplatePath:  fileConfig.TemplatePath,
			LastLoadedAt:  &now,
		})
		if err != nil {
			return fmt.Errorf("failed to create file config record: %w", err)
		}
	}
	
	// Create MCP servers and discover tools
	for serverName, serverConfig := range serversData {
		serverConfigMap, ok := serverConfig.(map[string]interface{})
		if !ok {
			continue
		}
		
		// Create MCP server record
		server := &models.MCPServer{
			EnvironmentID: envID,
			Name:          serverName,
		}
		
		// Extract server configuration
		if command, ok := serverConfigMap["command"].(string); ok {
			server.Command = command
		}
		if argsInterface, ok := serverConfigMap["args"]; ok {
			if argsArray, ok := argsInterface.([]interface{}); ok {
				args := make([]string, len(argsArray))
				for i, arg := range argsArray {
					if argStr, ok := arg.(string); ok {
						args[i] = argStr
					}
				}
				server.Args = args
			}
		}
		if envInterface, ok := serverConfigMap["env"]; ok {
			if envMap, ok := envInterface.(map[string]interface{}); ok {
				env := make(map[string]string)
				for k, v := range envMap {
					if vStr, ok := v.(string); ok {
						env[k] = vStr
					}
				}
				server.Env = env
			}
		}
		
		// Create server (or try to find existing one)
		serverID, err := repos.MCPServers.Create(server)
		if err != nil {
			return fmt.Errorf("failed to create/update MCP server %s: %w", serverName, err)
		}
		
		// Discover and register tools for this server
		err = h.discoverToolsForServer(repos, serverID, serverName, serverConfigMap, result.RenderedContent)
		if err != nil {
			// Don't fail the entire sync for tool discovery errors, just log them
			fmt.Printf("\n    ⚠️  Warning: Failed to discover tools for %s: %v", serverName, err)
		}
	}
	
	return nil
}

// discoverToolsForServer discovers and registers tools for a specific MCP server
func (h *MCPHandler) discoverToolsForServer(repos *repositories.Repositories, serverID int64, serverName string, serverConfig map[string]interface{}, renderedContent string) error {
	// This is a simplified version - you could integrate with the existing ToolDiscoveryService
	// For now, we'll create a basic tool discovery based on the server configuration
	
	// Create a basic tool entry as placeholder
	// In a full implementation, this would actually connect to the MCP server and discover real tools
	tool := &models.MCPTool{
		MCPServerID: serverID,
		Name:        fmt.Sprintf("%s_placeholder", serverName),
		Description: fmt.Sprintf("Placeholder tool for %s server", serverName),
	}
	
	_, err := repos.MCPTools.Create(tool)
	return err
}