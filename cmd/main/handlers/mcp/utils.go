package mcp

import (
	"fmt"

	"station/internal/db"
	"station/internal/db/repositories"
	mcpservice "station/internal/mcp"
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
	statusService := mcpservice.NewStatusService(nil)
	return statusService.ValidateEnvironmentExists(envName)
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
	
	// Create config syncer
	syncer := mcpservice.NewConfigSyncer(repos)
	
	fmt.Printf("🔍 Scanning database configs in environment '%s'...\n", environment)
	
	// Perform sync using the service
	options := mcpservice.SyncOptions{
		DryRun: dryRun,
		Force:  force,
	}
	
	result, err := syncer.Sync(environment, envID, options)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}
	
	// Display results
	if len(result.SyncedConfigs) > 0 {
		fmt.Printf("\n📥 Configs to sync:\n")
		for _, name := range result.SyncedConfigs {
			fmt.Printf("  • %s\n", styles.Success.Render(name))
		}
	}

	if len(result.RemovedConfigs) > 0 {
		fmt.Printf("\n🗑️  Configs to remove:\n")
		for _, name := range result.RemovedConfigs {
			fmt.Printf("  • %s\n", styles.Error.Render(name))
		}
	}

	if len(result.SyncedConfigs) == 0 && len(result.RemovedConfigs) == 0 {
		fmt.Printf("\n✅ %s\n", styles.Success.Render("All configurations are up to date"))
		return nil
	}

	if dryRun {
		fmt.Printf("\n🔍 %s\n", styles.Info.Render("Dry run complete - no changes made"))
		return nil
	}

	// Show sync progress
	fmt.Printf("\n🔄 Syncing configurations...\n")

	// Show individual config results
	for _, configName := range result.SyncedConfigs {
		// Check if this config had an error
		hasError := false
		for _, syncError := range result.SyncErrors {
			if syncError.ConfigName == configName {
				fmt.Printf("  📥 Loading %s... %s\n", configName, styles.Error.Render("❌"))
				hasError = true
				break
			}
		}
		if !hasError {
			fmt.Printf("  📥 Loading %s... %s\n", configName, styles.Success.Render("✅"))
		}
	}

	for _, configName := range result.RemovedConfigs {
		fmt.Printf("  🗑️  Removing %s... %s\n", configName, styles.Success.Render("✅"))
	}

	// Summary
	if len(result.SyncErrors) > 0 {
		fmt.Printf("\n⚠️ %s\n", styles.Error.Render("Sync completed with errors!"))
		fmt.Printf("📊 Summary:\n")
		fmt.Printf("  • Synced: %d configs\n", len(result.SyncedConfigs)-len(result.SyncErrors))
		fmt.Printf("  • Failed: %d configs\n", len(result.SyncErrors))
		fmt.Printf("  • Removed: %d configs\n", len(result.RemovedConfigs))
		if result.OrphanedToolsRemoved > 0 {
			fmt.Printf("  • Cleaned up: %d orphaned agent tools\n", result.OrphanedToolsRemoved)
		}
		if len(result.AffectedAgents) > 0 {
			fmt.Printf("  • Affected agents: %v\n", result.AffectedAgents)
			fmt.Printf("  • ⚠️  Agent health may be impacted - check agent logs for details\n")
		}
		
		fmt.Printf("\n❌ Sync Errors:\n")
		for _, syncError := range result.SyncErrors {
			fmt.Printf("  • %s: %s\n", syncError.ConfigName, styles.Error.Render(syncError.Error.Error()))
		}
		
		// Don't return error - partial success is still useful
		return nil
	} else {
		fmt.Printf("\n✅ %s\n", styles.Success.Render("Sync completed successfully!"))
		fmt.Printf("📊 Summary:\n")
		fmt.Printf("  • Synced: %d configs\n", len(result.SyncedConfigs))
		fmt.Printf("  • Removed: %d configs\n", len(result.RemovedConfigs))
		if result.OrphanedToolsRemoved > 0 {
			fmt.Printf("  • Cleaned up: %d orphaned agent tools\n", result.OrphanedToolsRemoved)
		}
		if len(result.AffectedAgents) > 0 {
			fmt.Printf("  • Affected agents: %v\n", result.AffectedAgents)
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
	
	// Create status service
	statusService := mcpservice.NewStatusService(repos)
	
	// Get environment statuses
	statuses, err := statusService.GetEnvironmentStatuses(environment)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	fmt.Printf("\n📊 Configuration Status Report\n\n")

	// Print table header
	fmt.Printf("┌────────────────┬─────────────────────────────┬──────────────────────────┬────────────────┐\n")
	fmt.Printf("│ %-14s │ %-27s │ %-24s │ %-14s │\n", "Environment", "Agent", "MCP Configs", "Status")
	fmt.Printf("├────────────────┼─────────────────────────────┼──────────────────────────┼────────────────┤\n")

	for _, envStatus := range statuses {
		if len(envStatus.Agents) == 0 {
			// No agents in this environment
			configNames := make([]string, len(envStatus.FileConfigs))
			for i, fc := range envStatus.FileConfigs {
				configNames[i] = fc.ConfigName
			}
			configList := mcpservice.TruncateString(fmt.Sprintf("%v", configNames), 24)
			if len(configNames) == 0 {
				configList = "none"
			}
			
			status := styles.Info.Render("no agents")
			fmt.Printf("│ %-14s │ %-27s │ %-24s │ %-14s │\n", 
				mcpservice.TruncateString(envStatus.Environment.Name, 14), "none", configList, status)
		} else {
			for i, agentStatus := range envStatus.Agents {
				// Format display
				envName := ""
				if i == 0 {
					envName = mcpservice.TruncateString(envStatus.Environment.Name, 14)
				}
				
				configDisplay := mcpservice.TruncateString(fmt.Sprintf("%v", agentStatus.ConfigNames), 24)
				if len(agentStatus.ConfigNames) == 0 {
					configDisplay = "none"
				}
				
				// Format status with styling
				var styledStatus string
				switch agentStatus.Status {
				case "synced":
					styledStatus = styles.Success.Render("synced")
				case "orphaned tools", "orphaned+sync", "out of sync":
					styledStatus = styles.Error.Render(agentStatus.Status)
				default:
					styledStatus = styles.Info.Render(agentStatus.Status)
				}
				
				fmt.Printf("│ %-14s │ %-27s │ %-24s │ %-14s │\n", 
					envName,
					mcpservice.TruncateString(agentStatus.Agent.Name, 27),
					configDisplay,
					styledStatus)
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