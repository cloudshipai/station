package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/mcp"
	"station/internal/services"
	"station/pkg/dotprompt"
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
	statusService := mcp.NewStatusService(nil)
	return statusService.ValidateEnvironmentExists(envName)
}

// syncMCPConfigsLocal performs declarative sync of file-based configs to database
func (h *MCPHandler) syncMCPConfigsLocal(environment string, dryRun bool) error {
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
	_, err = h.getOrCreateEnvironmentID(repos, environment)
	if err != nil {
		return fmt.Errorf("environment '%s' not found: %w", environment, err)
	}
	
	// Create declarative sync service
	declarativeSync := services.NewDeclarativeSync(repos, cfg)
	
	fmt.Printf("🔍 Scanning file-based configs in environment '%s'...\n", environment)
	
	// Perform sync using the declarative sync service
	options := services.SyncOptions{
		DryRun:      dryRun,
		Validate:    false,
		Interactive: true,
		Verbose:     false,
	}
	
	result, err := declarativeSync.SyncEnvironment(context.Background(), environment, options)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}
	
	// Note: Agent syncing is now handled by the DeclarativeSync service
	
	// Display simplified results for the new declarative sync
	if dryRun {
		fmt.Printf("\n🔍 %s\n", styles.Success.Render("Dry run complete - no changes made"))
		return nil
	}

	// Display results from the declarative sync
	fmt.Printf("\n📊 Sync Results:\n")
	fmt.Printf("  • MCP Servers: %d processed, %d connected\n", result.MCPServersProcessed, result.MCPServersConnected)
	fmt.Printf("  • Agents: %d processed, %d synced\n", result.AgentsProcessed, result.AgentsSynced)

	// Show failed servers if any
	failedServers := result.MCPServersProcessed - result.MCPServersConnected
	if failedServers > 0 {
		fmt.Printf("  • ❌ MCP Servers FAILED: %d (NOT saved to database)\n", failedServers)
		fmt.Printf("     ⚠️  These servers will NOT provide tools for agents\n")
	}

	if result.ValidationErrors > 0 {
		fmt.Printf("  • ⚠️  Validation Errors: %d\n", result.ValidationErrors)
		for _, errMsg := range result.ValidationMessages {
			fmt.Printf("    - %s\n", styles.Error.Render(errMsg))
		}
		
		if failedServers > 0 {
			fmt.Printf("\n❌ %s\n", styles.Error.Render("CRITICAL: Some MCP servers failed to sync!"))
			fmt.Printf("💡 Check server configurations and ensure MCP servers start correctly\n")
			fmt.Printf("💡 Agents using tools from failed servers will not work\n")
		} else {
			fmt.Printf("\n⚠️ %s\n", styles.Error.Render("Sync completed with validation errors!"))
		}
	} else {
		if failedServers > 0 {
			fmt.Printf("\n⚠️ %s\n", styles.Error.Render("Sync completed but some servers failed!"))
		} else {
			fmt.Printf("\n✅ %s\n", styles.Success.Render("Sync completed successfully!"))
		}
	}

	// Always show debug log location for troubleshooting
	homeDir, _ := os.UserHomeDir()
	debugLogPath := fmt.Sprintf("%s/.config/station/debug-mcp-sync.log", homeDir)
	if result.ValidationErrors > 0 || failedServers > 0 {
		fmt.Printf("\n🔍 Detailed debug logs available at: %s\n", debugLogPath)
		fmt.Printf("💡 Use 'tail -f %s' to monitor MCP sync issues\n", debugLogPath)
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
	statusService := mcp.NewStatusService(repos)
	
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
			configList := truncateString(fmt.Sprintf("%v", configNames), 24)
			if len(configNames) == 0 {
				configList = "none"
			}
			
			status := styles.Info.Render("no agents")
			fmt.Printf("│ %-14s │ %-27s │ %-24s │ %-14s │\n", 
				truncateString(envStatus.Environment.Name, 14), "none", configList, status)
		} else {
			for i, agentStatus := range envStatus.Agents {
				// Format display
				envName := ""
				if i == 0 {
					envName = truncateString(envStatus.Environment.Name, 14)
				}
				
				configDisplay := truncateString(fmt.Sprintf("%v", agentStatus.ConfigNames), 24)
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
					truncateString(agentStatus.Agent.Name, 27),
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

// syncAgentPromptFiles syncs .prompt files to the database
func (h *MCPHandler) syncAgentPromptFiles(repos *repositories.Repositories, environment string, envID int64, dryRun bool) (int, error) {
	// Get user home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return 0, fmt.Errorf("failed to get user home directory: %w", err)
	}
	
	// Construct agents directory path
	agentsDir := fmt.Sprintf("%s/.config/station/environments/%s/agents", homeDir, environment)
	
	// Check if agents directory exists
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		// No agents directory, nothing to sync
		return 0, nil
	}
	
	// Scan for .prompt files
	promptFiles, err := filepath.Glob(filepath.Join(agentsDir, "*.prompt"))
	if err != nil {
		return 0, fmt.Errorf("failed to scan for .prompt files: %w", err)
	}
	
	if len(promptFiles) == 0 {
		return 0, nil
	}
	
	fmt.Printf("🤖 Found %d .prompt files to sync...\n", len(promptFiles))
	
	var syncedCount int
	
	for _, promptFile := range promptFiles {
		// Extract agent name from filename
		filename := filepath.Base(promptFile)
		agentName := strings.TrimSuffix(filename, ".prompt")
		
		if dryRun {
			fmt.Printf("  [DRY RUN] Would sync agent: %s\n", agentName)
			syncedCount++
			continue
		}
		
		// Parse the .prompt file
		extractor, err := dotprompt.NewRuntimeExtraction(promptFile)
		if err != nil {
			fmt.Printf("  ❌ Failed to parse %s: %v\n", agentName, err)
			continue
		}
		
		config := extractor.GetConfig()
		
		// Validate agent name matches file
		if config.Metadata.Name != agentName {
			fmt.Printf("  ❌ Agent name mismatch in %s: expected '%s', got '%s'\n", 
				filename, agentName, config.Metadata.Name)
			continue
		}
		
		// Validate tool dependencies exist in MCP configs
		if err := h.validateAgentToolDependencies(repos, envID, &config, agentName); err != nil {
			fmt.Printf("  ❌ Validation failed for %s: %v\n", agentName, err)
			continue
		}
		
		// Sync agent to database - update prompt and tool assignments
		if err := h.syncAgentToDatabase(repos, envID, &config, agentName, extractor.GetTemplate()); err != nil {
			fmt.Printf("  ❌ Failed to sync %s to database: %v\n", agentName, err)
			continue
		}
		
		// Sync successful
		fmt.Printf("  ✅ Synced agent: %s\n", agentName)
		syncedCount++
	}
	
	return syncedCount, nil
}

// validateAgentToolDependencies validates that all tools assigned to an agent exist in MCP configs
func (h *MCPHandler) validateAgentToolDependencies(repos *repositories.Repositories, envID int64, config *dotprompt.DotpromptConfig, agentName string) error {
	if len(config.Tools) == 0 {
		return nil // No tools to validate
	}
	
	// Get all available MCP tools for this environment
	mcpTools, err := repos.MCPTools.GetByEnvironmentID(envID)
	if err != nil {
		return fmt.Errorf("failed to get MCP tools: %w", err)
	}
	
	// Create a map of available tool names for quick lookup
	availableTools := make(map[string]bool)
	for _, tool := range mcpTools {
		availableTools[tool.Name] = true
	}
	
	// Check each agent tool against available tools
	var missingTools []string
	for _, toolName := range config.Tools {
		if !availableTools[toolName] {
			missingTools = append(missingTools, toolName)
		}
	}
	
	if len(missingTools) > 0 {
		return fmt.Errorf("agent '%s' references non-existent tools: %v", agentName, missingTools)
	}
	
	return nil
}

// syncAgentToDatabase updates the agent in the database with .prompt file content and tool assignments
func (h *MCPHandler) syncAgentToDatabase(repos *repositories.Repositories, envID int64, config *dotprompt.DotpromptConfig, agentName, promptTemplate string) error {
	// Try to find existing agent by name
	agent, err := repos.Agents.GetByName(agentName)
	
	var agentID int64
	var isUpdate bool
	
	if err != nil {
		// Agent doesn't exist, create it
		isUpdate = false
	} else {
		// Agent exists, verify it's in the correct environment
		if agent.EnvironmentID != envID {
			return fmt.Errorf("agent '%s' is in environment %d, expected %d", agentName, agent.EnvironmentID, envID)
		}
		agentID = agent.ID
		isUpdate = true
	}
	
	// Extract max_steps from .prompt file metadata
	maxSteps := int64(5) // Default
	if stationData, ok := config.CustomFields["station"].(map[interface{}]interface{}); ok {
		if execData, ok := stationData["execution_metadata"].(map[interface{}]interface{}); ok {
			if steps, ok := execData["max_steps"].(int); ok {
				maxSteps = int64(steps)
			}
		}
	}
	
	if isUpdate {
		// Update existing agent
		err = repos.Agents.Update(
			agentID,
			config.Metadata.Name,
			config.Metadata.Description,
			promptTemplate,
			maxSteps,
			nil, // input_schema - not set in MCP utils
			nil, // cron schedule - preserve existing
			agent.ScheduleEnabled,
		)
		if err != nil {
			return fmt.Errorf("failed to update agent: %w", err)
		}
	} else {
		// Create new agent
		newAgent, err := repos.Agents.Create(
			agentName,
			config.Metadata.Description,
			promptTemplate,
			maxSteps,
			envID,
			1, // Default user ID
			nil, // input_schema - not set in MCP utils
			nil, // No cron schedule initially
			false, // Schedule not enabled initially
		)
		if err != nil {
			return fmt.Errorf("failed to create agent: %w", err)
		}
		agentID = newAgent.ID
	}
	
	// Clear existing tool assignments
	err = repos.AgentTools.Clear(agentID)
	if err != nil {
		return fmt.Errorf("failed to clear existing tool assignments: %w", err)
	}
	
	// Assign new tools from .prompt file
	for _, toolName := range config.Tools {
		// Find tool by name in this environment
		tool, err := repos.MCPTools.FindByNameInEnvironment(envID, toolName)
		if err != nil {
			return fmt.Errorf("tool '%s' not found in environment: %w", toolName, err)
		}
		
		// Add tool assignment
		_, err = repos.AgentTools.AddAgentTool(agentID, tool.ID)
		if err != nil {
			return fmt.Errorf("failed to assign tool '%s': %w", toolName, err)
		}
	}
	
	return nil
}