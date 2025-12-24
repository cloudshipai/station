package repositories

import (
	"database/sql"
	"station/internal/db"
)

type Repositories struct {
	Environments      *EnvironmentRepo
	Users             *UserRepo
	FileMCPConfigs    *FileMCPConfigRepo
	MCPServers        *MCPServerRepo
	MCPTools          *MCPToolRepo
	ModelProviders    *ModelProviderRepository
	Models            *ModelRepository
	Agents            *AgentRepo
	AgentTools        *AgentToolRepo
	AgentRuns         *AgentRunRepo
	AgentAgents       *AgentAgentRepo
	Settings          *SettingsRepo
	Reports           *ReportRepo
	BenchmarkMetrics  *BenchmarkMetricsRepo
	BenchmarkTasks    *BenchmarkTasksRepo
	Workflows         *WorkflowRepo
	WorkflowRuns      *WorkflowRunRepo
	WorkflowRunSteps  *WorkflowRunStepRepo
	WorkflowRunEvents *WorkflowRunEventRepo
	WorkflowApprovals *WorkflowApprovalRepo
	WorkflowSchedules *WorkflowScheduleRepo
	db                db.Database
}

func New(database db.Database) *Repositories {
	conn := database.Conn()

	return &Repositories{
		Environments:      NewEnvironmentRepo(conn),
		Users:             NewUserRepo(conn),
		FileMCPConfigs:    NewFileMCPConfigRepo(conn),
		MCPServers:        NewMCPServerRepo(conn),
		MCPTools:          NewMCPToolRepo(conn),
		ModelProviders:    NewModelProviderRepository(conn),
		Models:            NewModelRepository(conn),
		Agents:            NewAgentRepo(conn),
		AgentTools:        NewAgentToolRepo(conn),
		AgentAgents:       NewAgentAgentRepo(conn),
		AgentRuns:         NewAgentRunRepo(conn),
		Settings:          NewSettingsRepo(conn),
		Reports:           NewReportRepo(conn),
		BenchmarkMetrics:  NewBenchmarkMetricsRepo(conn),
		BenchmarkTasks:    NewBenchmarkTasksRepo(conn),
		Workflows:         NewWorkflowRepo(conn),
		WorkflowRuns:      NewWorkflowRunRepo(conn),
		WorkflowRunSteps:  NewWorkflowRunStepRepo(conn),
		WorkflowRunEvents: NewWorkflowRunEventRepo(conn),
		WorkflowApprovals: NewWorkflowApprovalRepo(conn),
		WorkflowSchedules: NewWorkflowScheduleRepo(conn),
		db:                database,
	}
}

// BeginTx starts a database transaction
func (r *Repositories) BeginTx() (*sql.Tx, error) {
	return r.db.Conn().Begin()
}
