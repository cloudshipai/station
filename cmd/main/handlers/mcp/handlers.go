package mcp

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"station/internal/db/repositories"
	"station/internal/theme"
	"station/pkg/models"
)

// MCPHandler handles MCP-related commands
type MCPHandler struct {
	themeManager *theme.ThemeManager
}

// NewMCPHandler creates a new MCP handler instance
func NewMCPHandler(themeManager *theme.ThemeManager) *MCPHandler {
	return &MCPHandler{themeManager: themeManager}
}

// RunMCPList implements the "station mcp list" command
func (h *MCPHandler) RunMCPList(cmd *cobra.Command, args []string) error {
	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("📋 MCP Configurations")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")

	// Determine if we're in local mode
	isLocal := endpoint == "" && viper.GetBool("local_mode")
	
	if isLocal {
		fmt.Println(styles.Info.Render("🏠 Listing local configurations"))
		return h.listMCPConfigsLocal(environment)
	} else if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Listing configurations from: " + endpoint))
		return h.listMCPConfigsRemote(environment, endpoint)
	} else {
		return fmt.Errorf("no endpoint specified and local_mode is false in config. Use --endpoint flag or enable local_mode in config")
	}
}

// RunMCPTools implements the "station mcp tools" command
func (h *MCPHandler) RunMCPTools(cmd *cobra.Command, args []string) error {
	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🔧 MCP Tools")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")
	filter, _ := cmd.Flags().GetString("filter")
	
	// If environment is not explicitly set via flag and we have args, use the first arg as environment
	if environment == "default" && len(args) > 0 {
		environment = args[0]
	}

	// Determine if we're in local mode
	isLocal := endpoint == "" && viper.GetBool("local_mode")
	
	if isLocal {
		fmt.Println(styles.Info.Render("🏠 Listing local tools"))
		return h.listMCPToolsLocal(environment, filter)
	} else if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Listing tools from: " + endpoint))
		return h.listMCPToolsRemote(environment, filter, endpoint)
	} else {
		return fmt.Errorf("no endpoint specified and local_mode is false in config. Use --endpoint flag or enable local_mode in config")
	}
}

// RunMCPDelete implements the "station mcp delete" command
func (h *MCPHandler) RunMCPDelete(cmd *cobra.Command, args []string) error {
	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🗑️ Delete MCP Configuration")
	fmt.Println(banner)

	configID := args[0]
	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")
	confirm, _ := cmd.Flags().GetBool("confirm")

	// Determine if we're in local mode
	isLocal := endpoint == "" && viper.GetBool("local_mode")
	
	if isLocal {
		fmt.Println(styles.Info.Render("🏠 Deleting from local database"))
		return h.deleteMCPConfigLocal(configID, environment, confirm)
	} else if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Deleting from: " + endpoint))
		return h.deleteMCPConfigRemote(configID, environment, endpoint, confirm)
	} else {
		return fmt.Errorf("no endpoint specified and local_mode is false in config. Use --endpoint flag or enable local_mode in config")
	}
}

// RunMCPSync implements the "station mcp sync" command
func (h *MCPHandler) RunMCPSync(cmd *cobra.Command, args []string) error {
	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🔄 MCP Configuration Sync")
	fmt.Println(banner)

	environment := args[0]
	endpoint, _ := cmd.Flags().GetString("endpoint")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Determine if we're in local mode
	isLocal := endpoint == "" && viper.GetBool("local_mode")
	
	if isLocal {
		fmt.Println(styles.Info.Render("🏠 Syncing local configurations"))
		return h.syncMCPConfigsLocal(environment, dryRun)
	} else if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Syncing with: " + endpoint))
		return fmt.Errorf("remote sync not yet implemented")
	} else {
		return fmt.Errorf("no endpoint specified and local_mode is false in config. Use --endpoint flag or enable local_mode in config")
	}
}

// RunMCPStatus implements the "station mcp status" command  
func (h *MCPHandler) RunMCPStatus(cmd *cobra.Command, args []string) error {
	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("📊 MCP Configuration Status")
	fmt.Println(banner)

	environment, _ := cmd.Flags().GetString("environment")
	endpoint, _ := cmd.Flags().GetString("endpoint")

	// Determine if we're in local mode
	isLocal := endpoint == "" && viper.GetBool("local_mode")
	
	if isLocal {
		fmt.Println(styles.Info.Render("🏠 Checking local configurations"))
		return h.statusMCPConfigsLocal(environment)
	} else if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Checking status at: " + endpoint))
		return fmt.Errorf("remote status not yet implemented")
	} else {
		return fmt.Errorf("no endpoint specified and local_mode is false in config. Use --endpoint flag or enable local_mode in config")
	}
}

// AddServerToConfig adds a single server to an existing MCP configuration (public method)
func (h *MCPHandler) AddServerToConfig(configID, serverName, command string, args []string, envVars map[string]string, environment, endpoint string) (string, error) {
	return h.addServerToConfig(configID, serverName, command, args, envVars, environment, endpoint)
}

// getOrCreateEnvironmentID gets environment ID from database, creating if needed for file-based configs
func (h *MCPHandler) getOrCreateEnvironmentID(repos *repositories.Repositories, envName string) (int64, error) {
	// Try to get existing environment
	env, err := repos.Environments.GetByName(envName)
	if err == nil {
		return env.ID, nil
	}
	
	// Check if this is a file-based environment
	if h.validateEnvironmentExists(envName) {
		// Environment directory exists, create database record
		description := fmt.Sprintf("Auto-created environment for file-based config: %s", envName)
		env, err = repos.Environments.Create(envName, &description, 1) // Default user ID 1
		if err != nil {
			return 0, fmt.Errorf("failed to create environment: %w", err)
		}
		return env.ID, nil
	}
	
	// Environment doesn't exist
	return 0, fmt.Errorf("environment not found")
}

// removeOrphanedAgentTools removes agent tools that belong to a deleted file config
func (h *MCPHandler) removeOrphanedAgentTools(repos *repositories.Repositories, agents []*models.Agent, fileConfigID int64) (int, error) {
	removed := 0
	
	// Get the config name for logging
	var configName string
	dbConfigs, err := repos.FileMCPConfigs.ListByEnvironment(1) // Assuming default environment
	if err == nil {
		for _, config := range dbConfigs {
			if config.ID == fileConfigID {
				configName = config.ConfigName
				break
			}
		}
	}
	if configName == "" {
		configName = fmt.Sprintf("config_%d", fileConfigID)
	}
	
	// Get all MCP tools that belong to the deleted config
	orphanedTools, err := repos.MCPTools.GetByServerID(fileConfigID)
	if err != nil {
		return 0, fmt.Errorf("failed to get tools for config ID %d: %w", fileConfigID, err)
	}
	
	// Create a set of orphaned tool IDs and names for quick lookup and logging
	orphanedToolIDs := make(map[int64]bool)
	orphanedToolNames := make(map[int64]string)
	for _, tool := range orphanedTools {
		orphanedToolIDs[tool.ID] = true
		orphanedToolNames[tool.ID] = tool.Name
	}
	
	// Remove orphaned tools from all agents and log the changes
	for _, agent := range agents {
		agentTools, err := repos.AgentTools.ListAgentTools(agent.ID)
		if err != nil {
			continue
		}
		
		var removedToolNames []string
		
		// Remove tools that belong to the deleted config
		for _, agentTool := range agentTools {
			if orphanedToolIDs[agentTool.ToolID] {
				err = repos.AgentTools.RemoveAgentTool(agent.ID, agentTool.ToolID)
				if err != nil {
					return removed, fmt.Errorf("failed to remove tool %d from agent %s: %w", agentTool.ToolID, agent.Name, err)
				}
				
				toolName := orphanedToolNames[agentTool.ToolID]
				removedToolNames = append(removedToolNames, toolName)
				removed++
			}
		}
		
		// Create audit log entry for this agent if tools were removed
		if len(removedToolNames) > 0 {
			err := h.logAgentHealthEvent(repos, agent.ID, "tool_removed", "orphaned_config", 
				fmt.Sprintf("Removed tools %v from deleted config '%s'", removedToolNames, configName),
				h.determineImpactLevel(len(removedToolNames)))
			if err != nil {
				// Don't fail the entire operation for logging errors, just log the issue
				fmt.Printf("\n    ⚠️  Warning: Failed to log health event for agent %s: %v", agent.Name, err)
			}
		}
	}
	
	// Also delete the MCP tools and servers from the database
	if len(orphanedTools) > 0 {
		// Delete tools first
		err = repos.MCPTools.DeleteByServerID(fileConfigID)
		if err != nil {
			return removed, fmt.Errorf("failed to delete orphaned tools: %w", err)
		}
		
		// Delete the MCP server record
		err = repos.MCPServers.Delete(fileConfigID)
		if err != nil {
			return removed, fmt.Errorf("failed to delete orphaned server: %w", err)
		}
	}
	
	return removed, nil
}

// logAgentHealthEvent creates an audit log entry for agent health monitoring
func (h *MCPHandler) logAgentHealthEvent(repos *repositories.Repositories, agentID int64, eventType, eventReason, details, impact string) error {
	// For now, we'll just log to console since we don't have AgentAuditLog repository yet
	// In a full implementation, you would create the repository and save to database
	fmt.Printf("\n    📋 Agent Health Event: Agent %d - %s (%s) - %s - Impact: %s", 
		agentID, eventType, eventReason, details, impact)
	
	// TODO: Implement AgentAuditLog repository and save to database
	// auditLog := &models.AgentAuditLog{
	//     AgentID:     agentID,
	//     EventType:   eventType,
	//     EventReason: eventReason,
	//     Details:     details,
	//     Impact:      impact,
	//     CreatedAt:   time.Now(),
	// }
	// return repos.AgentAuditLog.Create(auditLog)
	
	return nil
}

// determineImpactLevel determines the impact level based on number of tools removed
func (h *MCPHandler) determineImpactLevel(toolsRemoved int) string {
	if toolsRemoved >= 5 {
		return "high"
	} else if toolsRemoved >= 2 {
		return "medium"
	}
	return "low"
}