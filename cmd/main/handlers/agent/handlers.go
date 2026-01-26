package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"station/internal/config"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"station/cmd/main/handlers/common"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/telemetry"
	"station/internal/theme"
	"station/pkg/models"
)

// AgentHandler handles agent-related CLI commands
type AgentHandler struct {
	themeManager     *theme.ThemeManager
	telemetryService *telemetry.TelemetryService
}

func NewAgentHandler(themeManager *theme.ThemeManager, telemetryService *telemetry.TelemetryService) *AgentHandler {
	return &AgentHandler{
		themeManager:     themeManager,
		telemetryService: telemetryService,
	}
}

// RunAgentList lists all agents
func (h *AgentHandler) RunAgentList(cmd *cobra.Command, args []string) error {
	startTime := time.Now()
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🤖 Agents")
	fmt.Println(banner)

	environment, _ := cmd.Flags().GetString("environment")
	err := h.listAgentsLocalWithFilter(environment)

	// Track telemetry
	if h.telemetryService != nil {
		h.telemetryService.TrackCLICommand("agent", "list", err == nil, time.Since(startTime).Milliseconds())
	}

	return err
}

// RunAgentShow shows details of a specific agent
func (h *AgentHandler) RunAgentShow(cmd *cobra.Command, args []string) error {
	environment, _ := cmd.Flags().GetString("environment")

	if len(args) != 1 {
		return fmt.Errorf("usage: stn agent show <agent_name>")
	}

	agentName := args[0]
	return h.showAgentLocalByName(agentName, environment)
}

// showAgentLocalByName shows details of an agent by name and environment
func (h *AgentHandler) showAgentLocalByName(agentName, environment string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load station config: %v", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer func() { _ = database.Close() }()

	repos := repositories.New(database)
	agentService := services.NewAgentService(repos)

	// Find agent by name and environment
	agents, err := agentService.ListAgentsByEnvironment(context.Background(), 0)
	if err != nil {
		return fmt.Errorf("failed to list agents: %v", err)
	}

	var targetAgent *models.Agent
	var environmentName string

	for _, agent := range agents {
		if agent.Name == agentName {
			env, err := repos.Environments.GetByID(agent.EnvironmentID)
			if err != nil {
				continue
			}

			// Filter by environment if specified
			if environment != "" && env.Name != environment {
				continue
			}

			targetAgent = agent
			environmentName = env.Name
			break
		}
	}

	if targetAgent == nil {
		if environment != "" {
			return fmt.Errorf("agent '%s' not found in environment '%s'", agentName, environment)
		}
		return fmt.Errorf("agent '%s' not found", agentName)
	}

	// Get agent tools
	tools, err := repos.AgentTools.ListAgentTools(targetAgent.ID)
	if err != nil {
		return fmt.Errorf("failed to get agent tools: %v", err)
	}

	// Display agent details
	fmt.Printf("📋 Agent Details\n")
	fmt.Printf("Name: %s\n", targetAgent.Name)
	fmt.Printf("Description: %s\n", targetAgent.Description)
	fmt.Printf("Environment: %s\n", environmentName)
	fmt.Printf("Max Steps: %d\n", targetAgent.MaxSteps)
	fmt.Printf("Created: %s\n", targetAgent.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated: %s\n", targetAgent.UpdatedAt.Format("2006-01-02 15:04:05"))

	if len(tools) > 0 {
		fmt.Printf("\n🛠 Tools (%d):\n", len(tools))
		for _, tool := range tools {
			fmt.Printf("  • %s\n", tool.ToolName)
			if tool.ToolDescription != "" {
				fmt.Printf("    %s\n", tool.ToolDescription)
			}
		}
	}

	fmt.Printf("\n📝 Prompt:\n%s\n", targetAgent.Prompt)
	return nil
}

// RunAgentDelete deletes an agent
func (h *AgentHandler) RunAgentDelete(cmd *cobra.Command, args []string) error {
	environment, _ := cmd.Flags().GetString("environment")

	if len(args) != 1 {
		return fmt.Errorf("usage: stn agent delete <agent_name>")
	}

	agentName := args[0]
	return h.deleteAgentLocalByName(agentName, environment)
}

// deleteAgentLocalByName deletes an agent by name and environment
func (h *AgentHandler) deleteAgentLocalByName(agentName, environment string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load station config: %v", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer func() { _ = database.Close() }()

	repos := repositories.New(database)
	agentService := services.NewAgentService(repos)

	// Find agent by name and environment
	agents, err := agentService.ListAgentsByEnvironment(context.Background(), 0)
	if err != nil {
		return fmt.Errorf("failed to list agents: %v", err)
	}

	var targetAgent *models.Agent
	for _, agent := range agents {
		if agent.Name == agentName {
			// Get environment name if filtering by environment
			if environment != "" {
				env, err := repos.Environments.GetByID(agent.EnvironmentID)
				if err != nil {
					continue
				}
				if env.Name != environment {
					continue
				}
			}
			targetAgent = agent
			break
		}
	}

	if targetAgent == nil {
		return fmt.Errorf("agent '%s' not found", agentName)
	}

	// Confirm deletion
	fmt.Printf("⚠ Are you sure you want to delete agent '%s'? [y/N]: ", targetAgent.Name)

	var response string
	fmt.Scanln(&response)

	if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
		fmt.Printf("❌ Deletion cancelled\n")
		return nil
	}

	// Delete the agent
	err = agentService.DeleteAgent(context.Background(), targetAgent.ID)
	if err != nil {
		// Track failed deletion
		if h.telemetryService != nil {
			h.telemetryService.TrackError("agent_deletion_failed", err.Error(), map[string]interface{}{
				"agent_name": targetAgent.Name,
				"agent_id":   targetAgent.ID,
			})
		}
		return fmt.Errorf("failed to delete agent: %v", err)
	}

	// Track successful deletion
	if h.telemetryService != nil {
		h.telemetryService.TrackEvent("stn_agent_deleted", map[string]interface{}{
			"agent_id":       targetAgent.ID,
			"environment_id": targetAgent.EnvironmentID,
		})
	}

	fmt.Printf("✅ Agent '%s' deleted successfully\n", targetAgent.Name)
	return nil
}

// RunAgentCreate creates a new agent via CLI flags (mirrors MCP handleCreateAgent)
func (h *AgentHandler) RunAgentCreate(cmd *cobra.Command, args []string) error {
	startTime := time.Now()
	ctx := cmd.Context()
	agentName := args[0]

	prompt, _ := cmd.Flags().GetString("prompt")
	description, _ := cmd.Flags().GetString("description")
	if prompt == "" {
		return fmt.Errorf("--prompt is required")
	}
	if description == "" {
		return fmt.Errorf("--description is required")
	}

	environment, _ := cmd.Flags().GetString("environment")
	maxSteps, _ := cmd.Flags().GetInt("max-steps")
	toolsStr, _ := cmd.Flags().GetString("tools")
	outputSchema, _ := cmd.Flags().GetString("output-schema")
	outputSchemaPreset, _ := cmd.Flags().GetString("output-schema-preset")
	inputSchema, _ := cmd.Flags().GetString("input-schema")
	app, _ := cmd.Flags().GetString("app")
	appType, _ := cmd.Flags().GetString("app-type")
	memoryTopic, _ := cmd.Flags().GetString("memory-topic")
	memoryMaxTokens, _ := cmd.Flags().GetInt("memory-max-tokens")
	sandboxConfig, _ := cmd.Flags().GetString("sandbox")
	codingConfig, _ := cmd.Flags().GetString("coding")
	notifyEnabled, _ := cmd.Flags().GetBool("notify")
	harnessConfig, _ := cmd.Flags().GetString("harness-config")

	if sandboxConfig != "" {
		var sandboxMap map[string]interface{}
		if err := json.Unmarshal([]byte(sandboxConfig), &sandboxMap); err != nil {
			return fmt.Errorf("invalid --sandbox JSON: %v", err)
		}
	}
	if codingConfig != "" {
		var codingMap map[string]interface{}
		if err := json.Unmarshal([]byte(codingConfig), &codingMap); err != nil {
			return fmt.Errorf("invalid --coding JSON: %v", err)
		}
	}
	if harnessConfig != "" {
		var harnessMap map[string]interface{}
		if err := json.Unmarshal([]byte(harnessConfig), &harnessMap); err != nil {
			return fmt.Errorf("invalid --harness-config JSON: %v", err)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load station config: %v", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer func() { _ = database.Close() }()

	repos := repositories.New(database)
	agentService := services.NewAgentService(repos)
	exportService := services.NewAgentExportService(repos)

	env, err := repos.Environments.GetByName(environment)
	if err != nil {
		return fmt.Errorf("environment '%s' not found: %v", environment, err)
	}

	var toolNames []string
	if toolsStr != "" {
		toolNames = strings.Split(toolsStr, ",")
		for i, t := range toolNames {
			toolNames[i] = strings.TrimSpace(t)
		}
	}

	agentConfig := &services.AgentConfig{
		EnvironmentID: env.ID,
		Name:          agentName,
		Description:   description,
		Prompt:        prompt,
		MaxSteps:      int64(maxSteps),
		AssignedTools: toolNames,
		CreatedBy:     1,
		App:           app,
		AppType:       appType,
	}

	if outputSchema != "" {
		agentConfig.OutputSchema = &outputSchema
	}
	if outputSchemaPreset != "" {
		agentConfig.OutputSchemaPreset = &outputSchemaPreset
	}
	if inputSchema != "" {
		agentConfig.InputSchema = &inputSchema
	}
	if memoryTopic != "" {
		agentConfig.MemoryTopicKey = &memoryTopic
	}
	if memoryMaxTokens > 0 {
		agentConfig.MemoryMaxTokens = &memoryMaxTokens
	}

	createdAgent, err := agentService.CreateAgent(ctx, agentConfig)
	if err != nil {
		if h.telemetryService != nil {
			h.telemetryService.TrackError("agent_creation_failed", err.Error(), map[string]interface{}{
				"agent_name":  agentName,
				"environment": environment,
			})
		}
		return fmt.Errorf("failed to create agent: %v", err)
	}

	if sandboxConfig != "" || codingConfig != "" || harnessConfig != "" || notifyEnabled {
		if err := exportService.ExportAgentWithAllConfigs(createdAgent.ID, app, appType, sandboxConfig, codingConfig, harnessConfig, notifyEnabled); err != nil {
			fmt.Printf("⚠️  Agent created but export failed: %v\n", err)
		}
	}

	if h.telemetryService != nil {
		h.telemetryService.TrackCLICommand("agent", "create", true, time.Since(startTime).Milliseconds())
		h.telemetryService.TrackEvent("stn_agent_created", map[string]interface{}{
			"agent_id":       createdAgent.ID,
			"agent_name":     createdAgent.Name,
			"environment_id": createdAgent.EnvironmentID,
			"max_steps":      createdAgent.MaxSteps,
			"tools_count":    len(toolNames),
		})
	}

	fmt.Printf("✅ Agent '%s' created successfully\n", createdAgent.Name)
	fmt.Printf("   ID: %d\n", createdAgent.ID)
	fmt.Printf("   Environment: %s\n", environment)
	fmt.Printf("   Max Steps: %d\n", createdAgent.MaxSteps)
	if len(toolNames) > 0 {
		fmt.Printf("   Tools: %s\n", strings.Join(toolNames, ", "))
	}
	if memoryTopic != "" {
		fmt.Printf("   Memory Topic: %s\n", memoryTopic)
	}
	if notifyEnabled {
		fmt.Printf("   Notify: enabled\n")
	}
	fmt.Printf("\n💡 Run 'stn agent show %s' to see full details\n", agentName)
	fmt.Printf("💡 Run 'stn agent run %s \"<task>\"' to execute the agent\n", agentName)

	return nil
}

// RunAgentUpdate updates an existing agent via CLI flags (mirrors MCP handleUpdateAgent)
func (h *AgentHandler) RunAgentUpdate(cmd *cobra.Command, args []string) error {
	startTime := time.Now()
	ctx := cmd.Context()
	agentName := args[0]

	environment, _ := cmd.Flags().GetString("environment")
	prompt, _ := cmd.Flags().GetString("prompt")
	description, _ := cmd.Flags().GetString("description")
	maxSteps, _ := cmd.Flags().GetInt("max-steps")
	toolsStr, _ := cmd.Flags().GetString("tools")
	outputSchema, _ := cmd.Flags().GetString("output-schema")
	outputSchemaPreset, _ := cmd.Flags().GetString("output-schema-preset")
	app, _ := cmd.Flags().GetString("app")
	appType, _ := cmd.Flags().GetString("app-type")
	memoryTopic, _ := cmd.Flags().GetString("memory-topic")
	memoryMaxTokens, _ := cmd.Flags().GetInt("memory-max-tokens")
	sandboxConfig, _ := cmd.Flags().GetString("sandbox")
	codingConfig, _ := cmd.Flags().GetString("coding")
	notifyEnabled, _ := cmd.Flags().GetBool("notify")
	harnessConfig, _ := cmd.Flags().GetString("harness-config")

	if sandboxConfig != "" {
		var sandboxMap map[string]interface{}
		if err := json.Unmarshal([]byte(sandboxConfig), &sandboxMap); err != nil {
			return fmt.Errorf("invalid --sandbox JSON: %v", err)
		}
	}
	if codingConfig != "" {
		var codingMap map[string]interface{}
		if err := json.Unmarshal([]byte(codingConfig), &codingMap); err != nil {
			return fmt.Errorf("invalid --coding JSON: %v", err)
		}
	}
	if harnessConfig != "" {
		var harnessMap map[string]interface{}
		if err := json.Unmarshal([]byte(harnessConfig), &harnessMap); err != nil {
			return fmt.Errorf("invalid --harness-config JSON: %v", err)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load station config: %v", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer func() { _ = database.Close() }()

	repos := repositories.New(database)
	agentService := services.NewAgentService(repos)
	exportService := services.NewAgentExportService(repos)

	agents, err := agentService.ListAgentsByEnvironment(ctx, 0)
	if err != nil {
		return fmt.Errorf("failed to list agents: %v", err)
	}

	var targetAgent *models.Agent
	var targetEnvName string
	for _, agent := range agents {
		if agent.Name == agentName {
			env, err := repos.Environments.GetByID(agent.EnvironmentID)
			if err != nil {
				continue
			}

			if environment != "" && env.Name != environment {
				continue
			}

			targetAgent = agent
			targetEnvName = env.Name
			break
		}
	}

	if targetAgent == nil {
		if environment != "" {
			return fmt.Errorf("agent '%s' not found in environment '%s'", agentName, environment)
		}
		return fmt.Errorf("agent '%s' not found", agentName)
	}

	agentConfig := &services.AgentConfig{
		EnvironmentID: targetAgent.EnvironmentID,
		Name:          targetAgent.Name,
		Description:   targetAgent.Description,
		Prompt:        targetAgent.Prompt,
		MaxSteps:      targetAgent.MaxSteps,
		App:           targetAgent.App,
		AppType:       targetAgent.AppType,
	}
	if targetAgent.MemoryTopicKey != nil {
		agentConfig.MemoryTopicKey = targetAgent.MemoryTopicKey
	}
	if targetAgent.MemoryMaxTokens != nil {
		agentConfig.MemoryMaxTokens = targetAgent.MemoryMaxTokens
	}

	var updates []string
	needsExport := false

	if cmd.Flags().Changed("prompt") {
		agentConfig.Prompt = prompt
		updates = append(updates, "prompt")
	}
	if cmd.Flags().Changed("description") {
		agentConfig.Description = description
		updates = append(updates, "description")
	}
	if cmd.Flags().Changed("max-steps") {
		agentConfig.MaxSteps = int64(maxSteps)
		updates = append(updates, fmt.Sprintf("max-steps=%d", maxSteps))
	}
	if cmd.Flags().Changed("output-schema") {
		agentConfig.OutputSchema = &outputSchema
		updates = append(updates, "output-schema")
	}
	if cmd.Flags().Changed("output-schema-preset") {
		agentConfig.OutputSchemaPreset = &outputSchemaPreset
		updates = append(updates, "output-schema-preset")
	}
	if cmd.Flags().Changed("tools") {
		var toolNames []string
		if toolsStr != "" {
			toolNames = strings.Split(toolsStr, ",")
			for i, t := range toolNames {
				toolNames[i] = strings.TrimSpace(t)
			}
		}
		agentConfig.AssignedTools = toolNames
		updates = append(updates, fmt.Sprintf("tools=%d", len(toolNames)))
	}
	if cmd.Flags().Changed("app") {
		agentConfig.App = app
		updates = append(updates, "app")
		needsExport = true
	}
	if cmd.Flags().Changed("app-type") {
		agentConfig.AppType = appType
		updates = append(updates, "app-type")
		needsExport = true
	}
	if cmd.Flags().Changed("memory-topic") {
		agentConfig.MemoryTopicKey = &memoryTopic
		updates = append(updates, "memory-topic")
	}
	if cmd.Flags().Changed("memory-max-tokens") {
		agentConfig.MemoryMaxTokens = &memoryMaxTokens
		updates = append(updates, "memory-max-tokens")
	}
	if cmd.Flags().Changed("sandbox") {
		updates = append(updates, "sandbox")
		needsExport = true
	}
	if cmd.Flags().Changed("coding") {
		updates = append(updates, "coding")
		needsExport = true
	}
	if cmd.Flags().Changed("notify") {
		updates = append(updates, "notify")
		needsExport = true
	}
	if cmd.Flags().Changed("harness-config") {
		updates = append(updates, "harness-config")
		needsExport = true
	}

	if len(updates) == 0 {
		fmt.Printf("⚠️  No updates specified. Available flags:\n")
		fmt.Printf("   --prompt, --description, --max-steps, --tools, --output-schema\n")
		fmt.Printf("   --output-schema-preset, --app, --app-type\n")
		fmt.Printf("   --memory-topic, --memory-max-tokens\n")
		fmt.Printf("   --sandbox, --coding, --notify, --harness-config\n")
		return nil
	}

	updatedAgent, err := agentService.UpdateAgent(ctx, targetAgent.ID, agentConfig)
	if err != nil {
		if h.telemetryService != nil {
			h.telemetryService.TrackError("agent_update_failed", err.Error(), map[string]interface{}{
				"agent_name": agentName,
				"agent_id":   targetAgent.ID,
			})
		}
		return fmt.Errorf("failed to update agent: %v", err)
	}

	if needsExport {
		finalApp := agentConfig.App
		finalAppType := agentConfig.AppType
		if err := exportService.ExportAgentWithAllConfigs(updatedAgent.ID, finalApp, finalAppType, sandboxConfig, codingConfig, harnessConfig, notifyEnabled); err != nil {
			fmt.Printf("⚠️  Agent updated but export failed: %v\n", err)
		}
	}

	if h.telemetryService != nil {
		h.telemetryService.TrackCLICommand("agent", "update", true, time.Since(startTime).Milliseconds())
		h.telemetryService.TrackEvent("stn_agent_updated", map[string]interface{}{
			"agent_id":   updatedAgent.ID,
			"agent_name": updatedAgent.Name,
			"updates":    updates,
		})
	}

	fmt.Printf("✅ Agent '%s' updated successfully\n", updatedAgent.Name)
	fmt.Printf("   ID: %d\n", updatedAgent.ID)
	fmt.Printf("   Environment: %s\n", targetEnvName)
	fmt.Printf("   Updated: %s\n", strings.Join(updates, ", "))
	fmt.Printf("\n💡 Run 'stn agent show %s' to see full details\n", agentName)

	return nil
}
