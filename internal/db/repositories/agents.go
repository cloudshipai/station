package repositories

import (
	"context"
	"database/sql"
	"station/internal/db/queries"
	"station/pkg/models"
)

type AgentRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewAgentRepo(db *sql.DB) *AgentRepo {
	return &AgentRepo{
		db:      db,
		queries: queries.New(db),
	}
}

// convertAgentFromSQLc converts sqlc Agent to models.Agent
func convertAgentFromSQLc(agent queries.Agent) *models.Agent {
	result := &models.Agent{
		ID:              agent.ID,
		Name:            agent.Name,
		Description:     agent.Description,
		Prompt:          agent.Prompt,
		MaxSteps:        agent.MaxSteps,
		EnvironmentID:   agent.EnvironmentID,
		CreatedBy:       agent.CreatedBy,
		IsScheduled:     agent.IsScheduled.Bool,
		ScheduleEnabled: agent.ScheduleEnabled.Bool,
	}

	if agent.InputSchema.Valid {
		result.InputSchema = &agent.InputSchema.String
	}

	if agent.OutputSchema.Valid {
		result.OutputSchema = &agent.OutputSchema.String
	}

	if agent.OutputSchemaPreset.Valid {
		result.OutputSchemaPreset = &agent.OutputSchemaPreset.String
	}

	if agent.App.Valid {
		result.App = agent.App.String
	}

	if agent.AppSubtype.Valid {
		result.AppType = agent.AppSubtype.String
	}

	if agent.CronSchedule.Valid {
		result.CronSchedule = &agent.CronSchedule.String
	}

	if agent.LastScheduledRun.Valid {
		result.LastScheduledRun = &agent.LastScheduledRun.Time
	}

	if agent.NextScheduledRun.Valid {
		result.NextScheduledRun = &agent.NextScheduledRun.Time
	}

	if agent.CreatedAt.Valid {
		result.CreatedAt = agent.CreatedAt.Time
	}

	if agent.UpdatedAt.Valid {
		result.UpdatedAt = agent.UpdatedAt.Time
	}

	return result
}

func (r *AgentRepo) Create(name, description, prompt string, maxSteps, environmentID, createdBy int64, inputSchema *string, cronSchedule *string, scheduleEnabled bool, outputSchema *string, outputSchemaPreset *string, app, appType string) (*models.Agent, error) {
	isScheduled := cronSchedule != nil && *cronSchedule != "" && scheduleEnabled

	params := queries.CreateAgentParams{
		Name:            name,
		Description:     description,
		Prompt:          prompt,
		MaxSteps:        maxSteps,
		EnvironmentID:   environmentID,
		CreatedBy:       createdBy,
		IsScheduled:     sql.NullBool{Bool: isScheduled, Valid: true},
		ScheduleEnabled: sql.NullBool{Bool: scheduleEnabled, Valid: true},
	}

	if inputSchema != nil {
		params.InputSchema = sql.NullString{String: *inputSchema, Valid: true}
	}

	if cronSchedule != nil {
		params.CronSchedule = sql.NullString{String: *cronSchedule, Valid: true}
	}

	if outputSchema != nil {
		params.OutputSchema = sql.NullString{String: *outputSchema, Valid: true}
	}

	if outputSchemaPreset != nil {
		params.OutputSchemaPreset = sql.NullString{String: *outputSchemaPreset, Valid: true}
	}

	if app != "" {
		params.App = sql.NullString{String: app, Valid: true}
	}

	if appType != "" {
		params.AppSubtype = sql.NullString{String: appType, Valid: true}
	}

	created, err := r.queries.CreateAgent(context.Background(), params)
	if err != nil {
		return nil, err
	}

	return convertAgentFromSQLc(created), nil
}

func (r *AgentRepo) GetByID(id int64) (*models.Agent, error) {
	agent, err := r.queries.GetAgent(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return convertAgentFromSQLc(agent), nil
}

func (r *AgentRepo) GetByName(name string) (*models.Agent, error) {
	agent, err := r.queries.GetAgentByName(context.Background(), name)
	if err != nil {
		return nil, err
	}
	return convertAgentFromSQLc(agent), nil
}

func (r *AgentRepo) GetByNameAndEnvironment(name string, environmentID int64) (*models.Agent, error) {
	params := queries.GetAgentByNameAndEnvironmentParams{
		Name:          name,
		EnvironmentID: environmentID,
	}
	agent, err := r.queries.GetAgentByNameAndEnvironment(context.Background(), params)
	if err != nil {
		return nil, err
	}
	return convertAgentFromSQLc(agent), nil
}

func (r *AgentRepo) List() ([]*models.Agent, error) {
	agents, err := r.queries.ListAgents(context.Background())
	if err != nil {
		return nil, err
	}

	var result []*models.Agent
	for _, agent := range agents {
		result = append(result, convertAgentFromSQLc(agent))
	}

	return result, nil
}

// GetAgentWithTools returns the GetAgentWithTools query rows directly for API use
func (r *AgentRepo) GetAgentWithTools(ctx context.Context, id int64) ([]queries.GetAgentWithToolsRow, error) {
	return r.queries.GetAgentWithTools(ctx, id)
}

func (r *AgentRepo) ListByEnvironment(environmentID int64) ([]*models.Agent, error) {
	agents, err := r.queries.ListAgentsByEnvironment(context.Background(), environmentID)
	if err != nil {
		return nil, err
	}

	var result []*models.Agent
	for _, agent := range agents {
		result = append(result, convertAgentFromSQLc(agent))
	}

	return result, nil
}

func (r *AgentRepo) ListByUser(userID int64) ([]*models.Agent, error) {
	agents, err := r.queries.ListAgentsByUser(context.Background(), userID)
	if err != nil {
		return nil, err
	}

	var result []*models.Agent
	for _, agent := range agents {
		result = append(result, convertAgentFromSQLc(agent))
	}

	return result, nil
}

func (r *AgentRepo) Update(id int64, name, description, prompt string, maxSteps int64, inputSchema *string, cronSchedule *string, scheduleEnabled bool, outputSchema *string, outputSchemaPreset *string, app, appType string) error {
	isScheduled := cronSchedule != nil && *cronSchedule != "" && scheduleEnabled

	params := queries.UpdateAgentParams{
		Name:            name,
		Description:     description,
		Prompt:          prompt,
		MaxSteps:        maxSteps,
		IsScheduled:     sql.NullBool{Bool: isScheduled, Valid: true},
		ScheduleEnabled: sql.NullBool{Bool: scheduleEnabled, Valid: true},
		ID:              id,
	}

	if inputSchema != nil {
		params.InputSchema = sql.NullString{String: *inputSchema, Valid: true}
	}

	if cronSchedule != nil {
		params.CronSchedule = sql.NullString{String: *cronSchedule, Valid: true}
	}

	if outputSchema != nil {
		params.OutputSchema = sql.NullString{String: *outputSchema, Valid: true}
	}

	if outputSchemaPreset != nil {
		params.OutputSchemaPreset = sql.NullString{String: *outputSchemaPreset, Valid: true}
	}

	if app != "" {
		params.App = sql.NullString{String: app, Valid: true}
	}

	if appType != "" {
		params.AppSubtype = sql.NullString{String: appType, Valid: true}
	}

	return r.queries.UpdateAgent(context.Background(), params)
}

func (r *AgentRepo) Delete(id int64) error {
	return r.queries.DeleteAgent(context.Background(), id)
}

func (r *AgentRepo) UpdatePrompt(id int64, prompt string) error {
	return r.queries.UpdateAgentPrompt(context.Background(), queries.UpdateAgentPromptParams{
		ID:     id,
		Prompt: prompt,
	})
}

// CreateTx creates agent within a transaction
func (r *AgentRepo) CreateTx(tx *sql.Tx, name, description, prompt string, maxSteps, environmentID, createdBy int64, inputSchema *string, cronSchedule *string, scheduleEnabled bool, outputSchema *string, outputSchemaPreset *string, app, appType string) (*models.Agent, error) {
	isScheduled := cronSchedule != nil && *cronSchedule != "" && scheduleEnabled

	params := queries.CreateAgentParams{
		Name:            name,
		Description:     description,
		Prompt:          prompt,
		MaxSteps:        maxSteps,
		EnvironmentID:   environmentID,
		CreatedBy:       createdBy,
		IsScheduled:     sql.NullBool{Bool: isScheduled, Valid: true},
		ScheduleEnabled: sql.NullBool{Bool: scheduleEnabled, Valid: true},
	}

	if inputSchema != nil {
		params.InputSchema = sql.NullString{String: *inputSchema, Valid: true}
	}

	if cronSchedule != nil {
		params.CronSchedule = sql.NullString{String: *cronSchedule, Valid: true}
	}

	if outputSchema != nil {
		params.OutputSchema = sql.NullString{String: *outputSchema, Valid: true}
	}

	if outputSchemaPreset != nil {
		params.OutputSchemaPreset = sql.NullString{String: *outputSchemaPreset, Valid: true}
	}

	if app != "" {
		params.App = sql.NullString{String: app, Valid: true}
	}

	if appType != "" {
		params.AppSubtype = sql.NullString{String: appType, Valid: true}
	}

	txQueries := r.queries.WithTx(tx)
	created, err := txQueries.CreateAgent(context.Background(), params)
	if err != nil {
		return nil, err
	}

	return convertAgentFromSQLc(created), nil
}

// UpdateTx updates agent within a transaction
func (r *AgentRepo) UpdateTx(tx *sql.Tx, id int64, name, description, prompt string, maxSteps int64, inputSchema *string, cronSchedule *string, scheduleEnabled bool, outputSchema *string, outputSchemaPreset *string, app, appType string) error {
	isScheduled := cronSchedule != nil && *cronSchedule != "" && scheduleEnabled

	params := queries.UpdateAgentParams{
		Name:            name,
		Description:     description,
		Prompt:          prompt,
		MaxSteps:        maxSteps,
		IsScheduled:     sql.NullBool{Bool: isScheduled, Valid: true},
		ScheduleEnabled: sql.NullBool{Bool: scheduleEnabled, Valid: true},
		ID:              id,
	}

	if inputSchema != nil {
		params.InputSchema = sql.NullString{String: *inputSchema, Valid: true}
	}

	if cronSchedule != nil {
		params.CronSchedule = sql.NullString{String: *cronSchedule, Valid: true}
	}

	if outputSchema != nil {
		params.OutputSchema = sql.NullString{String: *outputSchema, Valid: true}
	}

	if outputSchemaPreset != nil {
		params.OutputSchemaPreset = sql.NullString{String: *outputSchemaPreset, Valid: true}
	}

	if app != "" {
		params.App = sql.NullString{String: app, Valid: true}
	}

	if appType != "" {
		params.AppSubtype = sql.NullString{String: appType, Valid: true}
	}

	txQueries := r.queries.WithTx(tx)
	return txQueries.UpdateAgent(context.Background(), params)
}
