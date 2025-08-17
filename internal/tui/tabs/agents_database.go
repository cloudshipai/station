package tabs

import (
	"fmt"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"station/pkg/models"
)

// Load agents from database
func (m AgentsModel) loadAgents() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Load real agents from database
		agents, err := m.repos.Agents.List()
		if err != nil {
			return AgentsErrorMsg{Err: fmt.Errorf("failed to load agents: %w", err)}
		}
		
		// Convert from []*models.Agent to []models.Agent
		var convertedAgents []models.Agent
		for _, agent := range agents {
			convertedAgents = append(convertedAgents, *agent)
		}
		return AgentsLoadedMsg{Agents: convertedAgents}
	})
}

// Load environments from database
func (m AgentsModel) loadEnvironments() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		envs, err := m.repos.Environments.List()
		if err != nil {
			// Return empty slice on error
			return EnvironmentsLoadedMsg{Environments: []models.Environment{}}
		}
		
		// Convert from []*models.Environment to []models.Environment
		var environments []models.Environment
		for _, env := range envs {
			environments = append(environments, *env)
		}
		
		return AgentsEnvironmentsLoadedMsg{Environments: environments}
	})
}

// Load available tools from database
func (m AgentsModel) loadTools() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Get all tools with details (includes environment and config names)
		toolsWithDetails, err := m.repos.MCPTools.GetAllWithDetails()
		if err != nil {
			return AgentsErrorMsg{Err: fmt.Errorf("failed to load tools: %w", err)}
		}
		
		// Convert from []*models.MCPToolWithDetails to []models.MCPToolWithDetails
		var mcpTools []models.MCPToolWithDetails
		for _, tool := range toolsWithDetails {
			mcpTools = append(mcpTools, *tool)
		}
		
		return AgentsToolsLoadedMsg{Tools: mcpTools}
	})
}

// Load tools assigned to a specific agent
func (m AgentsModel) loadAgentTools(agentID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		tools, err := m.repos.AgentTools.ListAgentTools(agentID)
		if err != nil {
			return AgentsErrorMsg{Err: fmt.Errorf("failed to load agent tools: %w", err)}
		}
		
		// Convert from []*models.AgentToolWithDetails to []models.AgentToolWithDetails
		var agentTools []models.AgentToolWithDetails
		for _, tool := range tools {
			agentTools = append(agentTools, *tool)
		}
		
		return AgentToolsLoadedMsg{AgentID: agentID, Tools: agentTools}
	})
}

// Delete agent command
func (m AgentsModel) deleteAgent(agentID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Delete agent from database
		if err := m.repos.Agents.Delete(agentID); err != nil {
			return AgentsErrorMsg{Err: fmt.Errorf("failed to delete agent: %w", err)}
		}
		return AgentDeletedMsg{AgentID: agentID}
	})
}

// Create agent command
func (m AgentsModel) createAgent() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Validate inputs
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			return AgentsErrorMsg{Err: fmt.Errorf("agent name is required")}
		}
		if len(name) > 100 {
			return AgentsErrorMsg{Err: fmt.Errorf("agent name too long (max 100 characters)")}
		}
		
		description := strings.TrimSpace(m.descInput.Value())
		if description == "" {
			description = "No description provided"
		}
		if len(description) > 500 {
			return AgentsErrorMsg{Err: fmt.Errorf("description too long (max 500 characters)")}
		}
		
		prompt := strings.TrimSpace(m.promptArea.Value())
		if prompt == "" {
			prompt = "You are a helpful AI assistant."
		}
		if len(prompt) > 5000 {
			return AgentsErrorMsg{Err: fmt.Errorf("system prompt too long (max 5000 characters)")}
		}
		
		// Validate at least one environment is selected
		if len(m.selectedEnvIDs) == 0 {
			return AgentsErrorMsg{Err: fmt.Errorf("at least one environment must be selected")}
		}
		
		// Validate all selected environments exist
		var validEnvIDs []int64
		for _, selectedID := range m.selectedEnvIDs {
			for _, env := range m.environments {
				if env.ID == selectedID {
					validEnvIDs = append(validEnvIDs, selectedID)
					break
				}
			}
		}
		if len(validEnvIDs) == 0 {
			return AgentsErrorMsg{Err: fmt.Errorf("no valid environments selected")}
		}
		
		// Validate and prepare schedule fields
		var cronSchedule *string
		if m.scheduleEnabled {
			schedule := strings.TrimSpace(m.scheduleInput.Value())
			if schedule != "" {
				// TODO: Add cron expression validation here if needed
				cronSchedule = &schedule
			}
		}
		
		// Create agent in database (use first selected environment as primary)
		primaryEnvID := validEnvIDs[0]
		agent, err := m.repos.Agents.Create(name, description, prompt, 10, primaryEnvID, 1, nil, cronSchedule, m.scheduleEnabled) // Default to user ID 1
		if err != nil {
			return AgentsErrorMsg{Err: fmt.Errorf("failed to create agent: %w", err)}
		}
		
		// Associate selected tools with agent using cross-environment tool assignment
		var failedTools []string  // Track failed tool names instead of IDs
		log.Printf("DEBUG: Associating %d selected tools with new agent %d", len(m.selectedToolIDs), agent.ID)
		log.Printf("DEBUG: Available tools count: %d", len(m.availableTools))
		
		for i, toolID := range m.selectedToolIDs {
			log.Printf("DEBUG: Processing tool %d/%d with ID %d", i+1, len(m.selectedToolIDs), toolID)
			
			// Find the tool details by ID to get tool name and environment ID
			var toolName string
			var envID int64
			found := false
			
			for _, tool := range m.availableTools {
				if tool.ID == toolID {
					toolName = tool.Name
					envID = tool.EnvironmentID
					found = true
					log.Printf("DEBUG: Found tool '%s' (ID:%d) in environment %d", toolName, toolID, envID)
					break
				}
			}
			
			if !found {
				log.Printf("DEBUG: Tool ID %d not found in available tools list", toolID)
				failedTools = append(failedTools, fmt.Sprintf("ID:%d", toolID))
				continue
			}
			
			// Add the tool assignment (environment-specific)
			log.Printf("DEBUG: Adding tool assignment: agent=%d, tool='%s', environment=%d", agent.ID, toolName, envID)
			
			// Find tool by name in environment
			tool, err := m.repos.MCPTools.FindByNameInEnvironment(envID, toolName)
			if err != nil {
				log.Printf("DEBUG: Tool '%s' not found in environment %d: %v", toolName, envID, err)
				failedTools = append(failedTools, toolName)
				continue
			}
			
			// Add tool to agent using tool ID
			if _, err := m.repos.AgentTools.AddAgentTool(agent.ID, tool.ID); err != nil {
				log.Printf("DEBUG: Failed to add tool assignment: %v", err)
				failedTools = append(failedTools, toolName)
			} else {
				log.Printf("DEBUG: Successfully added tool assignment: agent=%d, tool='%s' (ID=%d), env=%d", agent.ID, toolName, tool.ID, envID)
			}
		}
		
		// Show warning if some tools failed to associate
		if len(failedTools) > 0 {
			return AgentsErrorMsg{Err: fmt.Errorf("agent created but %d tools failed to associate", len(failedTools))}
		}
		
		return AgentCreatedMsg{Agent: *agent}
	})
}

// Update agent command
func (m AgentsModel) updateAgent() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if m.selectedAgent == nil {
			return AgentsErrorMsg{Err: fmt.Errorf("no agent selected for update")}
		}
		
		// Validate inputs (same as create)
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			return AgentsErrorMsg{Err: fmt.Errorf("agent name is required")}
		}
		if len(name) > 100 {
			return AgentsErrorMsg{Err: fmt.Errorf("agent name too long (max 100 characters)")}
		}
		
		description := strings.TrimSpace(m.descInput.Value())
		if description == "" {
			description = "No description provided"
		}
		if len(description) > 500 {
			return AgentsErrorMsg{Err: fmt.Errorf("description too long (max 500 characters)")}
		}
		
		prompt := strings.TrimSpace(m.promptArea.Value())
		if prompt == "" {
			prompt = "You are a helpful AI assistant."
		}
		if len(prompt) > 5000 {
			return AgentsErrorMsg{Err: fmt.Errorf("system prompt too long (max 5000 characters)")}
		}
		
		// Validate at least one environment is selected
		if len(m.selectedEnvIDs) == 0 {
			return AgentsErrorMsg{Err: fmt.Errorf("at least one environment must be selected")}
		}
		
		// Validate all selected environments exist
		var validEnvIDs []int64
		for _, selectedID := range m.selectedEnvIDs {
			for _, env := range m.environments {
				if env.ID == selectedID {
					validEnvIDs = append(validEnvIDs, selectedID)
					break
				}
			}
		}
		if len(validEnvIDs) == 0 {
			return AgentsErrorMsg{Err: fmt.Errorf("no valid environments selected")}
		}
		
		// Validate and prepare schedule fields
		var cronSchedule *string
		if m.scheduleEnabled {
			schedule := strings.TrimSpace(m.scheduleInput.Value())
			if schedule != "" {
				// TODO: Add cron expression validation here if needed
				cronSchedule = &schedule
			}
		}
		
		// Update agent in database
		if err := m.repos.Agents.Update(m.selectedAgent.ID, name, description, prompt, m.selectedAgent.MaxSteps, nil, cronSchedule, m.scheduleEnabled); err != nil {
			return AgentsErrorMsg{Err: fmt.Errorf("failed to update agent: %w", err)}
		}
		
		// Create updated agent model for return (use first selected environment as primary)
		primaryEnvID := validEnvIDs[0]
		updatedAgent := &models.Agent{
			ID:              m.selectedAgent.ID,
			Name:            name,
			Description:     description,
			Prompt:          prompt,
			MaxSteps:        m.selectedAgent.MaxSteps,
			EnvironmentID:   primaryEnvID,
			CreatedBy:       m.selectedAgent.CreatedBy,
			CronSchedule:    cronSchedule,
			IsScheduled:     cronSchedule != nil && *cronSchedule != "" && m.scheduleEnabled,
			ScheduleEnabled: m.scheduleEnabled,
			CreatedAt:       m.selectedAgent.CreatedAt,
		}
		
		// Clear existing tool associations
		if err := m.repos.AgentTools.Clear(m.selectedAgent.ID); err != nil {
			return AgentsErrorMsg{Err: fmt.Errorf("failed to clear agent tools: %w", err)}
		}
		
		// Associate selected tools with agent using cross-environment tool assignment
		var failedTools []string  // Track failed tool names instead of IDs
		log.Printf("DEBUG: Associating %d selected tools with agent %d", len(m.selectedToolIDs), m.selectedAgent.ID)
		log.Printf("DEBUG: Available tools count: %d", len(m.availableTools))
		
		for i, toolID := range m.selectedToolIDs {
			log.Printf("DEBUG: Processing tool %d/%d with ID %d", i+1, len(m.selectedToolIDs), toolID)
			
			// Find the tool details by ID to get tool name and environment ID
			var toolName string
			var envID int64
			found := false
			
			for _, tool := range m.availableTools {
				if tool.ID == toolID {
					toolName = tool.Name
					envID = tool.EnvironmentID
					found = true
					log.Printf("DEBUG: Found tool '%s' (ID:%d) in environment %d", toolName, toolID, envID)
					break
				}
			}
			
			if !found {
				log.Printf("DEBUG: Tool ID %d not found in available tools list", toolID)
				failedTools = append(failedTools, fmt.Sprintf("ID:%d", toolID))
				continue
			}
			
			// Add the tool assignment (environment-specific)
			log.Printf("DEBUG: Adding tool assignment: agent=%d, tool='%s', environment=%d", m.selectedAgent.ID, toolName, envID)
			
			// Find tool by name in environment
			tool, err := m.repos.MCPTools.FindByNameInEnvironment(envID, toolName)
			if err != nil {
				log.Printf("DEBUG: Tool '%s' not found in environment %d: %v", toolName, envID, err)
				failedTools = append(failedTools, toolName)
				continue
			}
			
			// Add tool to agent using tool ID
			if _, err := m.repos.AgentTools.AddAgentTool(m.selectedAgent.ID, tool.ID); err != nil {
				log.Printf("DEBUG: Failed to add tool assignment: %v", err)
				failedTools = append(failedTools, toolName)
			} else {
				log.Printf("DEBUG: Successfully added tool assignment: agent=%d, tool='%s' (ID=%d), env=%d", m.selectedAgent.ID, toolName, tool.ID, envID)
			}
		}
		
		// Show warning if some tools failed to associate
		if len(failedTools) > 0 {
			return AgentsErrorMsg{Err: fmt.Errorf("agent updated but %d tools failed to associate", len(failedTools))}
		}
		
		return AgentUpdatedMsg{Agent: *updatedAgent}
	})
}

// runAgent queues an agent for execution using the execution queue service
func (m *AgentsModel) runAgent(agent models.Agent) tea.Cmd {
	if m.executionQueue == nil {
		return tea.Sequence(
			tea.Printf("❌ ERROR: Execution queue service not available!"),
			tea.Printf("🔧 This indicates a configuration or initialization problem"),
		)
	}
	
	// Use a default task prompt for manual agent execution
	task := "Execute agent manually from TUI"
	if agent.Prompt != "" {
		task = agent.Prompt
	}
	
	// Create metadata to indicate this was a manual execution
	metadata := map[string]interface{}{
		"source":       "manual_tui",
		"triggered_at": time.Now(),
	}
	
	// For manual executions via SSH console, get the console user ID
	// TODO: Get actual user ID from session when authentication is implemented
	consoleUser, err := m.repos.Users.GetByUsername("console")
	if err != nil {
		return tea.Printf("❌ Could not find console user for execution tracking")
	}
	
	// Queue the execution
	runID, err := m.executionQueue.QueueExecution(agent.ID, consoleUser.ID, task, metadata)
	if err != nil {
		return tea.Printf("❌ Failed to queue agent execution: %v", err)
	}
	
	return tea.Sequence(
		tea.Printf("🚀 Agent '%s' has been queued for execution!", agent.Name),
		tea.Printf("📊 Run ID: %d - Check the Runs tab to monitor progress", runID),
		func() tea.Msg {
			return RunCreatedMsg{RunID: runID, AgentID: agent.ID}
		},
	)
}