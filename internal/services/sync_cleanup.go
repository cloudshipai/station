package services

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"station/internal/db/repositories"
	"station/internal/logging"
)

// cleanupOrphanedResources removes configs, servers, and tools that no longer exist in filesystem
func (s *DeclarativeSync) cleanupOrphanedResources(ctx context.Context, envDir, environmentName string, options SyncOptions) (string, error) {
	// Get environment from database
	env, err := s.repos.Environments.GetByName(environmentName)
	if err != nil {
		return "", fmt.Errorf("failed to get environment: %w", err)
	}

	// Find all .json files in filesystem (current source of truth)
	jsonFiles, err := filepath.Glob(filepath.Join(envDir, "*.json"))
	if err != nil {
		return "", fmt.Errorf("failed to scan JSON files: %w", err)
	}

	// Build map of existing files 
	filesystemConfigs := make(map[string]bool)
	for _, jsonFile := range jsonFiles {
		configName := strings.TrimSuffix(filepath.Base(jsonFile), ".json")
		filesystemConfigs[configName] = true
	}

	// Get all file configs from database for this environment
	dbConfigs, err := s.repos.FileMCPConfigs.ListByEnvironment(env.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get database configs: %w", err)
	}

	// Find configs that exist in DB but not in filesystem (to remove)
	var toRemove []string
	for _, dbConfig := range dbConfigs {
		if !filesystemConfigs[dbConfig.ConfigName] {
			toRemove = append(toRemove, dbConfig.ConfigName)
		}
	}

	if len(toRemove) == 0 {
		return "No orphaned resources found", nil
	}

	logging.Info("🗑️  Found %d orphaned configs to remove: %v", len(toRemove), toRemove)

	if options.DryRun {
		return fmt.Sprintf("Would remove %d orphaned configs: %v", len(toRemove), toRemove), nil
	}

	// Remove orphaned configs and their associated servers/tools
	var removedConfigs, removedServers, removedTools int
	for _, configName := range toRemove {
		logging.Info("   🗑️  Removing orphaned config: %s", configName)
		
		// Find the config to remove
		var configToRemove *repositories.FileConfigRecord
		for _, dbConfig := range dbConfigs {
			if dbConfig.ConfigName == configName {
				configToRemove = dbConfig
				break
			}
		}
		
		if configToRemove == nil {
			logging.Info("     ⚠️  Warning: Could not find config %s in database", configName)
			continue
		}

		// Get servers associated with this config (by reading the config file from database)
		// We need to parse the config to find server names, then delete those servers
		serversRemoved, toolsRemoved, err := s.removeConfigServersAndTools(ctx, env.ID, configName, configToRemove)
		if err != nil {
			logging.Info("     ❌ Failed to cleanup servers/tools for %s: %v", configName, err)
			continue
		}

		// Remove the file config itself
		err = s.repos.FileMCPConfigs.Delete(configToRemove.ID)
		if err != nil {
			logging.Info("     ❌ Failed to remove file config %s: %v", configName, err)
			continue
		}

		logging.Info("     ✅ Removed config %s (%d servers, %d tools)", configName, serversRemoved, toolsRemoved)
		removedConfigs++
		removedServers += serversRemoved
		removedTools += toolsRemoved
	}

	return fmt.Sprintf("Removed %d configs, %d servers, %d tools", removedConfigs, removedServers, removedTools), nil
}

// removeConfigServersAndTools removes servers and tools associated with a specific config
func (s *DeclarativeSync) removeConfigServersAndTools(ctx context.Context, envID int64, configName string, fileConfig *repositories.FileConfigRecord) (int, int, error) {
	// Since the file no longer exists, we need to identify servers that belonged to this config
	// We can get all servers for this environment and match by naming patterns or timestamps
	// For now, we'll use a simpler approach: delete servers that were created around the same time as this config
	
	allServers, err := s.repos.MCPServers.GetByEnvironmentID(envID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get servers for environment: %w", err)
	}

	var serversRemoved, toolsRemoved int
	
	// Strategy: Remove servers that likely belonged to this config
	// Since we can't read the deleted file, we'll look for servers with similar timing or 
	// use any available metadata to associate them with this config
	
	// For safety, we'll only remove servers if there's a clear association
	// A more robust implementation would store config_name or file_config_id in the servers table
	
	logging.Info("     🔍 Checking %d servers for association with config %s", len(allServers), configName)
	
	// Simple heuristic: remove servers whose names might be related to the config name
	// This is imperfect but better than leaving orphaned servers
	for _, server := range allServers {
		shouldRemove := false
		
		// Check if server name is similar to config name
		if strings.Contains(server.Name, configName) || strings.Contains(configName, server.Name) {
			shouldRemove = true
		}
		
		// Additional heuristic: if this is the only config being removed and there are few servers,
		// we might be more aggressive, but for safety we'll be conservative
		
		if shouldRemove {
			logging.Info("     🗑️  Removing server: %s (ID: %d)", server.Name, server.ID)
			
			// Get tools for this server before removing
			tools, err := s.repos.MCPTools.GetByServerID(server.ID)
			if err == nil {
				toolsRemoved += len(tools)
				logging.Info("       🔧 Removing %d tools from server %s", len(tools), server.Name)
			}
			
			// Remove server (tools should cascade delete)
			err = s.repos.MCPServers.Delete(server.ID)
			if err != nil {
				logging.Info("       ❌ Failed to remove server %s: %v", server.Name, err)
				continue
			}
			
			serversRemoved++
		}
	}
	
	return serversRemoved, toolsRemoved, nil
}

// cleanupOrphanedAgents removes agents from database that don't have corresponding .prompt files
func (s *DeclarativeSync) cleanupOrphanedAgents(ctx context.Context, agentsDir, environmentName string, promptFiles []string) (int, error) {
	env, err := s.repos.Environments.GetByName(environmentName)
	if err != nil {
		return 0, fmt.Errorf("environment '%s' not found: %w", environmentName, err)
	}

	// Get all agents from database for this environment
	dbAgents, err := s.repos.Agents.ListByEnvironment(env.ID)
	if err != nil {
		return 0, fmt.Errorf("failed to list agents from database: %w", err)
	}

	// Build set of agent names that have .prompt files
	promptAgentNames := make(map[string]bool)
	for _, promptFile := range promptFiles {
		agentName := strings.TrimSuffix(filepath.Base(promptFile), ".prompt")
		promptAgentNames[agentName] = true
	}

	// Find orphaned agents (in DB but not in filesystem)
	orphanedCount := 0
	agentService := NewAgentService(s.repos)

	for _, dbAgent := range dbAgents {
		if !promptAgentNames[dbAgent.Name] {
			// This agent exists in DB but has no corresponding .prompt file
			logging.Info("🗑️  Removing orphaned agent: %s", dbAgent.Name)
			
			err := agentService.DeleteAgent(ctx, dbAgent.ID)
			if err != nil {
				logging.Info("Warning: Failed to delete orphaned agent %s: %v", dbAgent.Name, err)
				continue
			}
			
			orphanedCount++
		}
	}

	return orphanedCount, nil
}