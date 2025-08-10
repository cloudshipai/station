package mcp

import (
	"fmt"
	"os"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// StatusService handles MCP configuration status reporting
type StatusService struct {
	repos *repositories.Repositories
}

// NewStatusService creates a new status service
func NewStatusService(repos *repositories.Repositories) *StatusService {
	return &StatusService{
		repos: repos,
	}
}

// EnvironmentStatus represents the status of MCP configurations in an environment
type EnvironmentStatus struct {
	Environment   *models.Environment
	Agents        []*AgentStatus
	FileConfigs   []*repositories.FileConfigRecord
	HasOrphaned   bool
	HasOutOfSync  bool
}

// AgentStatus represents the status of an agent and its MCP tools
type AgentStatus struct {
	Agent           *models.Agent
	ConfigNames     []string
	OrphanedTools   int
	Status          string
}

// GetEnvironmentStatuses returns status information for environments
func (s *StatusService) GetEnvironmentStatuses(environmentName string) ([]*EnvironmentStatus, error) {
	var environments []*models.Environment
	
	if environmentName == "default" || environmentName == "" {
		// Show all environments
		allEnvs, err := s.repos.Environments.List()
		if err != nil {
			return nil, fmt.Errorf("failed to list environments: %w", err)
		}
		environments = allEnvs
	} else {
		// Show specific environment
		env, err := s.repos.Environments.GetByName(environmentName)
		if err != nil {
			return nil, fmt.Errorf("environment '%s' not found", environmentName)
		}
		environments = []*models.Environment{env}
	}
	
	var statuses []*EnvironmentStatus
	
	for _, env := range environments {
		status, err := s.getEnvironmentStatus(env)
		if err != nil {
			continue // Skip environments with errors
		}
		statuses = append(statuses, status)
	}
	
	return statuses, nil
}

// getEnvironmentStatus gets status for a single environment
func (s *StatusService) getEnvironmentStatus(env *models.Environment) (*EnvironmentStatus, error) {
	// Get agents for this environment
	agents, err := s.repos.Agents.ListByEnvironment(env.ID)
	if err != nil {
		return nil, err
	}
	
	// Get file configs for this environment
	fileConfigs, err := s.repos.FileMCPConfigs.ListByEnvironment(env.ID)
	if err != nil {
		return nil, err
	}
	
	// Build agent statuses
	var agentStatuses []*AgentStatus
	var hasOrphaned, hasOutOfSync bool
	
	for _, agent := range agents {
		agentStatus, err := s.getAgentStatus(agent, fileConfigs)
		if err != nil {
			continue
		}
		
		agentStatuses = append(agentStatuses, agentStatus)
		
		if agentStatus.OrphanedTools > 0 {
			hasOrphaned = true
		}
		// TODO: Implement out-of-sync detection when file discovery is available
	}
	
	return &EnvironmentStatus{
		Environment: env,
		Agents:      agentStatuses,
		FileConfigs: fileConfigs,
		HasOrphaned: hasOrphaned,
		HasOutOfSync: hasOutOfSync,
	}, nil
}

// getAgentStatus gets status for a single agent
func (s *StatusService) getAgentStatus(agent *models.Agent, fileConfigs []*repositories.FileConfigRecord) (*AgentStatus, error) {
	// Get tools assigned to this agent
	agentTools, err := s.repos.AgentTools.ListAgentTools(agent.ID)
	if err != nil {
		return nil, err
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
	
	// Determine status
	var status string
	hasOutOfSync := false
	hasOrphaned := orphanedTools > 0
	
	// For now, assume all configs are in sync
	// TODO: Implement proper sync checking when file discovery is available
	
	if hasOrphaned && hasOutOfSync {
		status = "orphaned+sync"
	} else if hasOrphaned {
		status = "orphaned tools"
	} else if hasOutOfSync {
		status = "out of sync"
	} else if len(configList) == 0 {
		status = "no tools"
	} else {
		status = "synced"
	}
	
	return &AgentStatus{
		Agent:         agent,
		ConfigNames:   configList,
		OrphanedTools: orphanedTools,
		Status:        status,
	}, nil
}

// ValidateEnvironmentExists checks if file-based environment directory exists
func (s *StatusService) ValidateEnvironmentExists(envName string) bool {
	configDir := os.ExpandEnv("$HOME/.config/station")
	envDir := fmt.Sprintf("%s/environments/%s", configDir, envName)
	if _, err := os.Stat(envDir); err != nil {
		return false
	}
	return true
}

// TruncateString truncates a string to maxLen characters (utility function)
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}