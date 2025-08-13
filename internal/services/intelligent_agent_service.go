package services

import (
	"context"

	"station/internal/db/repositories"
	"station/pkg/models"
)

// IntelligentAgentService provides a unified interface to agent planning and execution
// This maintains backward compatibility while using the new modular architecture
type IntelligentAgentService struct {
	planGenerator   *AgentPlanGenerator
	executionEngine *AgentExecutionEngine
}

// NewIntelligentAgentService creates a new intelligent agent service
func NewIntelligentAgentService(repos *repositories.Repositories, agentService AgentServiceInterface) *IntelligentAgentService {
	return &IntelligentAgentService{
		planGenerator:   NewAgentPlanGenerator(repos, agentService),
		executionEngine: NewAgentExecutionEngine(repos, agentService),
	}
}

// CreateIntelligentAgent creates a new agent using AI-powered planning
func (ias *IntelligentAgentService) CreateIntelligentAgent(ctx context.Context, req AgentCreationRequest) (*models.Agent, error) {
	return ias.planGenerator.CreateIntelligentAgent(ctx, req)
}

// ExecuteAgentViaStdioMCP executes an agent using the MCP architecture with optional user variables
func (ias *IntelligentAgentService) ExecuteAgentViaStdioMCP(ctx context.Context, agent *models.Agent, task string, runID int64, userVariables ...map[string]interface{}) (*AgentExecutionResult, error) {
	// Handle optional userVariables parameter for backward compatibility
	var variables map[string]interface{}
	if len(userVariables) > 0 {
		variables = userVariables[0]
	}
	if variables == nil {
		variables = make(map[string]interface{})
	}
	
	return ias.executionEngine.ExecuteAgentViaStdioMCPWithVariables(ctx, agent, task, runID, variables)
}

// TestStdioMCPConnection tests the MCP connection for debugging
func (ias *IntelligentAgentService) TestStdioMCPConnection(ctx context.Context) error {
	return ias.executionEngine.TestStdioMCPConnection(ctx)
}