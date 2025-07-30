package repositories

import (
	"database/sql"
	"station/pkg/models"
)

type AgentRepo struct {
	db *sql.DB
}

func NewAgentRepo(db *sql.DB) *AgentRepo {
	return &AgentRepo{db: db}
}

func (r *AgentRepo) Create(name, description, prompt string, maxSteps, environmentID, createdBy int64, cronSchedule *string, scheduleEnabled bool) (*models.Agent, error) {
	query := `INSERT INTO agents (name, description, prompt, max_steps, environment_id, created_by, cron_schedule, is_scheduled, schedule_enabled) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) 
			  RETURNING id, name, description, prompt, max_steps, environment_id, created_by, cron_schedule, is_scheduled, schedule_enabled, created_at, updated_at`
	
	isScheduled := cronSchedule != nil && *cronSchedule != "" && scheduleEnabled
	
	var agent models.Agent
	err := r.db.QueryRow(query, name, description, prompt, maxSteps, environmentID, createdBy, cronSchedule, isScheduled, scheduleEnabled).Scan(
		&agent.ID, &agent.Name, &agent.Description, &agent.Prompt, &agent.MaxSteps,
		&agent.EnvironmentID, &agent.CreatedBy, &agent.CronSchedule, &agent.IsScheduled, &agent.ScheduleEnabled, &agent.CreatedAt, &agent.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &agent, nil
}

func (r *AgentRepo) GetByID(id int64) (*models.Agent, error) {
	query := `SELECT id, name, description, prompt, max_steps, environment_id, created_by, cron_schedule, is_scheduled, last_scheduled_run, next_scheduled_run, schedule_enabled, created_at, updated_at 
			  FROM agents WHERE id = ?`
	
	var agent models.Agent
	err := r.db.QueryRow(query, id).Scan(
		&agent.ID, &agent.Name, &agent.Description, &agent.Prompt, &agent.MaxSteps,
		&agent.EnvironmentID, &agent.CreatedBy, &agent.CronSchedule, &agent.IsScheduled, &agent.LastScheduledRun, &agent.NextScheduledRun, &agent.ScheduleEnabled, &agent.CreatedAt, &agent.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &agent, nil
}

func (r *AgentRepo) GetByName(name string) (*models.Agent, error) {
	query := `SELECT id, name, description, prompt, max_steps, environment_id, created_by, cron_schedule, is_scheduled, last_scheduled_run, next_scheduled_run, schedule_enabled, created_at, updated_at 
			  FROM agents WHERE name = ?`
	
	var agent models.Agent
	err := r.db.QueryRow(query, name).Scan(
		&agent.ID, &agent.Name, &agent.Description, &agent.Prompt, &agent.MaxSteps,
		&agent.EnvironmentID, &agent.CreatedBy, &agent.CronSchedule, &agent.IsScheduled, &agent.LastScheduledRun, &agent.NextScheduledRun, &agent.ScheduleEnabled, &agent.CreatedAt, &agent.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &agent, nil
}

func (r *AgentRepo) List() ([]*models.Agent, error) {
	query := `SELECT id, name, description, prompt, max_steps, environment_id, created_by, cron_schedule, is_scheduled, last_scheduled_run, next_scheduled_run, schedule_enabled, created_at, updated_at 
			  FROM agents ORDER BY name`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var agents []*models.Agent
	for rows.Next() {
		var agent models.Agent
		err := rows.Scan(&agent.ID, &agent.Name, &agent.Description, &agent.Prompt, &agent.MaxSteps,
			&agent.EnvironmentID, &agent.CreatedBy, &agent.CronSchedule, &agent.IsScheduled, &agent.LastScheduledRun, &agent.NextScheduledRun, &agent.ScheduleEnabled, &agent.CreatedAt, &agent.UpdatedAt)
		if err != nil {
			return nil, err
		}
		agents = append(agents, &agent)
	}
	
	return agents, rows.Err()
}

func (r *AgentRepo) ListByEnvironment(environmentID int64) ([]*models.Agent, error) {
	query := `SELECT id, name, description, prompt, max_steps, environment_id, created_by, cron_schedule, is_scheduled, last_scheduled_run, next_scheduled_run, schedule_enabled, created_at, updated_at 
			  FROM agents WHERE environment_id = ? ORDER BY name`
	
	rows, err := r.db.Query(query, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var agents []*models.Agent
	for rows.Next() {
		var agent models.Agent
		err := rows.Scan(&agent.ID, &agent.Name, &agent.Description, &agent.Prompt, &agent.MaxSteps,
			&agent.EnvironmentID, &agent.CreatedBy, &agent.CronSchedule, &agent.IsScheduled, &agent.LastScheduledRun, &agent.NextScheduledRun, &agent.ScheduleEnabled, &agent.CreatedAt, &agent.UpdatedAt)
		if err != nil {
			return nil, err
		}
		agents = append(agents, &agent)
	}
	
	return agents, rows.Err()
}

func (r *AgentRepo) ListByUser(userID int64) ([]*models.Agent, error) {
	query := `SELECT id, name, description, prompt, max_steps, environment_id, created_by, cron_schedule, is_scheduled, last_scheduled_run, next_scheduled_run, schedule_enabled, created_at, updated_at 
			  FROM agents WHERE created_by = ? ORDER BY name`
	
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var agents []*models.Agent
	for rows.Next() {
		var agent models.Agent
		err := rows.Scan(&agent.ID, &agent.Name, &agent.Description, &agent.Prompt, &agent.MaxSteps,
			&agent.EnvironmentID, &agent.CreatedBy, &agent.CronSchedule, &agent.IsScheduled, &agent.LastScheduledRun, &agent.NextScheduledRun, &agent.ScheduleEnabled, &agent.CreatedAt, &agent.UpdatedAt)
		if err != nil {
			return nil, err
		}
		agents = append(agents, &agent)
	}
	
	return agents, rows.Err()
}

func (r *AgentRepo) Update(id int64, name, description, prompt string, maxSteps int64, cronSchedule *string, scheduleEnabled bool) error {
	isScheduled := cronSchedule != nil && *cronSchedule != "" && scheduleEnabled
	query := `UPDATE agents SET name = ?, description = ?, prompt = ?, max_steps = ?, cron_schedule = ?, is_scheduled = ?, schedule_enabled = ? WHERE id = ?`
	_, err := r.db.Exec(query, name, description, prompt, maxSteps, cronSchedule, isScheduled, scheduleEnabled, id)
	return err
}

func (r *AgentRepo) Delete(id int64) error {
	query := `DELETE FROM agents WHERE id = ?`
	_, err := r.db.Exec(query, id)
	return err
}