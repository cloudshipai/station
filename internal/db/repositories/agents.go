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

	if agent.ScheduleVariables.Valid {
		result.ScheduleVariables = &agent.ScheduleVariables.String
	}

	// CloudShip Memory Integration
	if agent.MemoryTopicKey.Valid {
		result.MemoryTopicKey = &agent.MemoryTopicKey.String
	}

	if agent.MemoryMaxTokens.Valid {
		tokens := int(agent.MemoryMaxTokens.Int64)
		result.MemoryMaxTokens = &tokens
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

func (r *AgentRepo) GetByNameGlobal(name string) (*models.Agent, error) {
	agent, err := r.queries.GetAgentByNameGlobal(context.Background(), name)
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

	result := make([]*models.Agent, len(agents))
	for i, agent := range agents {
		result[i] = convertAgentFromSQLc(agent)
	}
	return result, nil
}

// GetByEnvironment gets agents for an environment with context support
func (r *AgentRepo) GetByEnvironment(ctx context.Context, environmentID int64) ([]models.Agent, error) {
	agents, err := r.queries.ListAgentsByEnvironment(ctx, environmentID)
	if err != nil {
		return nil, err
	}

	// Return as []models.Agent (not pointers) for compatibility
	result := make([]models.Agent, len(agents))
	for i, agent := range agents {
		converted := convertAgentFromSQLc(agent)
		result[i] = *converted
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

func (r *AgentRepo) Update(id int64, name, description, prompt string, maxSteps int64, inputSchema *string, cronSchedule *string, scheduleEnabled bool, scheduleVariables *string, outputSchema *string, outputSchemaPreset *string, app, appType string) error {
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

	if scheduleVariables != nil {
		params.ScheduleVariables = sql.NullString{String: *scheduleVariables, Valid: true}
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

// CreateWithMemory creates agent with memory integration support
func (r *AgentRepo) CreateWithMemory(name, description, prompt string, maxSteps, environmentID, createdBy int64, inputSchema *string, cronSchedule *string, scheduleEnabled bool, outputSchema *string, outputSchemaPreset *string, app, appType string, memoryTopicKey *string, memoryMaxTokens *int) (*models.Agent, error) {
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

	// Memory integration fields
	if memoryTopicKey != nil && *memoryTopicKey != "" {
		params.MemoryTopicKey = sql.NullString{String: *memoryTopicKey, Valid: true}
	}

	if memoryMaxTokens != nil {
		params.MemoryMaxTokens = sql.NullInt64{Int64: int64(*memoryMaxTokens), Valid: true}
	}

	created, err := r.queries.CreateAgent(context.Background(), params)
	if err != nil {
		return nil, err
	}

	return convertAgentFromSQLc(created), nil
}

// UpdateWithMemory updates agent with memory integration support
func (r *AgentRepo) UpdateWithMemory(id int64, name, description, prompt string, maxSteps int64, inputSchema *string, cronSchedule *string, scheduleEnabled bool, scheduleVariables *string, outputSchema *string, outputSchemaPreset *string, app, appType string, memoryTopicKey *string, memoryMaxTokens *int) error {
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

	if scheduleVariables != nil {
		params.ScheduleVariables = sql.NullString{String: *scheduleVariables, Valid: true}
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

	// Memory integration fields
	if memoryTopicKey != nil {
		if *memoryTopicKey != "" {
			params.MemoryTopicKey = sql.NullString{String: *memoryTopicKey, Valid: true}
		} else {
			// Explicitly clear memory topic if empty string passed
			params.MemoryTopicKey = sql.NullString{String: "", Valid: false}
		}
	}

	if memoryMaxTokens != nil {
		params.MemoryMaxTokens = sql.NullInt64{Int64: int64(*memoryMaxTokens), Valid: true}
	}

	return r.queries.UpdateAgent(context.Background(), params)
}

// UpdateMemoryConfig updates only the memory configuration for an agent
func (r *AgentRepo) UpdateMemoryConfig(id int64, memoryTopicKey *string, memoryMaxTokens *int) error {
	// First get the existing agent to preserve other fields
	agent, err := r.GetByID(id)
	if err != nil {
		return err
	}

	return r.UpdateWithMemory(
		id,
		agent.Name,
		agent.Description,
		agent.Prompt,
		agent.MaxSteps,
		agent.InputSchema,
		agent.CronSchedule,
		agent.ScheduleEnabled,
		agent.ScheduleVariables,
		agent.OutputSchema,
		agent.OutputSchemaPreset,
		agent.App,
		agent.AppType,
		memoryTopicKey,
		memoryMaxTokens,
	)
}

// CreateTxWithMemory creates agent with memory support within a transaction
func (r *AgentRepo) CreateTxWithMemory(tx *sql.Tx, name, description, prompt string, maxSteps, environmentID, createdBy int64, inputSchema *string, cronSchedule *string, scheduleEnabled bool, outputSchema *string, outputSchemaPreset *string, app, appType string, memoryTopicKey *string, memoryMaxTokens *int) (*models.Agent, error) {
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

	// Memory integration fields
	if memoryTopicKey != nil && *memoryTopicKey != "" {
		params.MemoryTopicKey = sql.NullString{String: *memoryTopicKey, Valid: true}
	}

	if memoryMaxTokens != nil {
		params.MemoryMaxTokens = sql.NullInt64{Int64: int64(*memoryMaxTokens), Valid: true}
	}

	txQueries := r.queries.WithTx(tx)
	created, err := txQueries.CreateAgent(context.Background(), params)
	if err != nil {
		return nil, err
	}

	return convertAgentFromSQLc(created), nil
}

// UpdateTxWithMemory updates agent with memory support within a transaction
func (r *AgentRepo) UpdateTxWithMemory(tx *sql.Tx, id int64, name, description, prompt string, maxSteps int64, inputSchema *string, cronSchedule *string, scheduleEnabled bool, outputSchema *string, outputSchemaPreset *string, app, appType string, memoryTopicKey *string, memoryMaxTokens *int) error {
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

	// Memory integration fields
	if memoryTopicKey != nil {
		if *memoryTopicKey != "" {
			params.MemoryTopicKey = sql.NullString{String: *memoryTopicKey, Valid: true}
		} else {
			params.MemoryTopicKey = sql.NullString{String: "", Valid: false}
		}
	}

	if memoryMaxTokens != nil {
		params.MemoryMaxTokens = sql.NullInt64{Int64: int64(*memoryMaxTokens), Valid: true}
	}

	txQueries := r.queries.WithTx(tx)
	return txQueries.UpdateAgent(context.Background(), params)
}
