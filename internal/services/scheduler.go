package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"station/internal/db"
	"station/internal/db/queries"
	"station/pkg/models"
)

// SchedulerService manages cron-based agent scheduling
type SchedulerService struct {
	cron   *cron.Cron
	db     db.Database
	agents map[int64]cron.EntryID // Track scheduled agents
}

// NewSchedulerService creates a new scheduler service
func NewSchedulerService(database db.Database) *SchedulerService {
	// Create cron with seconds precision and logging
	c := cron.New(cron.WithSeconds(), cron.WithLogger(cron.VerbosePrintfLogger(log.New(log.Writer(), "CRON: ", log.LstdFlags))))
	
	return &SchedulerService{
		cron:   c,
		db:     database,
		agents: make(map[int64]cron.EntryID),
	}
}

// Start starts the cron scheduler and loads existing scheduled agents
func (s *SchedulerService) Start() error {
	log.Println("Starting agent scheduler service...")
	
	// Load existing scheduled agents from database
	if err := s.loadScheduledAgents(); err != nil {
		return fmt.Errorf("failed to load scheduled agents: %w", err)
	}
	
	// Start the cron scheduler
	s.cron.Start()
	log.Println("Agent scheduler service started successfully")
	
	return nil
}

// Stop stops the cron scheduler
func (s *SchedulerService) Stop() {
	log.Println("Stopping agent scheduler service...")
	s.cron.Stop()
	log.Println("Agent scheduler service stopped")
}

// ScheduleAgent adds or updates a scheduled agent
func (s *SchedulerService) ScheduleAgent(agent *models.Agent) error {
	if agent.CronSchedule == nil || *agent.CronSchedule == "" {
		return fmt.Errorf("agent %d has no cron schedule", agent.ID)
	}

	// Remove existing schedule if present
	s.UnscheduleAgent(agent.ID)

	// Parse and validate cron expression
	schedule := *agent.CronSchedule
	if _, err := cron.ParseStandard(schedule); err != nil {
		return fmt.Errorf("invalid cron expression '%s': %w", schedule, err)
	}

	// Add new cron job
	entryID, err := s.cron.AddFunc(schedule, func() {
		s.executeScheduledAgent(agent.ID)
	})
	if err != nil {
		return fmt.Errorf("failed to schedule agent %d: %w", agent.ID, err)
	}

	// Track the scheduled agent
	s.agents[agent.ID] = entryID
	
	// Update next run time in database
	if err := s.updateNextRunTime(agent.ID, schedule); err != nil {
		log.Printf("Warning: failed to update next run time for agent %d: %v", agent.ID, err)
	}

	log.Printf("Scheduled agent %d (%s) with cron expression: %s", agent.ID, agent.Name, schedule)
	return nil
}

// UnscheduleAgent removes a scheduled agent
func (s *SchedulerService) UnscheduleAgent(agentID int64) {
	if entryID, exists := s.agents[agentID]; exists {
		s.cron.Remove(entryID)
		delete(s.agents, agentID)
		log.Printf("Unscheduled agent %d", agentID)
	}
}

// loadScheduledAgents loads all scheduled agents from the database
func (s *SchedulerService) loadScheduledAgents() error {
	ctx := context.Background()
	queries := queries.New(s.db.Conn())
	
	agents, err := queries.ListScheduledAgents(ctx)
	if err != nil {
		return fmt.Errorf("failed to query scheduled agents: %w", err)
	}

	log.Printf("Loading %d scheduled agents from database", len(agents))

	for _, agent := range agents {
		// Convert to models.Agent
		modelAgent := &models.Agent{
			ID:              agent.ID,
			Name:            agent.Name,
			Description:     agent.Description,
			CronSchedule:    &agent.CronSchedule.String,
			IsScheduled:     agent.IsScheduled,
			ScheduleEnabled: agent.ScheduleEnabled,
		}

		if agent.CronSchedule.Valid && agent.ScheduleEnabled {
			if err := s.ScheduleAgent(modelAgent); err != nil {
				log.Printf("Warning: failed to schedule agent %d (%s): %v", agent.ID, agent.Name, err)
				continue
			}
		}
	}

	return nil
}

// updateNextRunTime calculates and updates the next run time for an agent
func (s *SchedulerService) updateNextRunTime(agentID int64, cronExpr string) error {
	// Parse cron expression to calculate next run time
	schedule, err := cron.ParseStandard(cronExpr)
	if err != nil {
		return err
	}

	now := time.Now()
	nextRun := schedule.Next(now)
	
	ctx := context.Background()
	
	// TODO: Fix struct reference issue - implement direct SQL for now
	_, err = s.db.Conn().ExecContext(ctx, 
		"UPDATE agents SET next_scheduled_run = ? WHERE id = ?",
		nextRun, agentID)
	return err
}

// executeScheduledAgent executes a scheduled agent
func (s *SchedulerService) executeScheduledAgent(agentID int64) {
	log.Printf("Executing scheduled agent %d", agentID)
	
	ctx := context.Background()
	queries := queries.New(s.db.Conn())
	
	// Get agent details
	agent, err := queries.GetAgentBySchedule(ctx, agentID)
	if err != nil {
		log.Printf("Error: failed to get scheduled agent %d: %v", agentID, err)
		return
	}

	// Update last run time
	now := time.Now()
	nextRun := sql.NullTime{Valid: false}
	
	// Calculate next run time if schedule is still valid
	if agent.CronSchedule.Valid {
		if schedule, err := cron.ParseStandard(agent.CronSchedule.String); err == nil {
			nextRun = sql.NullTime{Time: schedule.Next(now), Valid: true}
		}
	}
	
	// TODO: Fix struct reference issue - implement direct SQL for now
	if err := s.updateScheduleTimes(ctx, agentID, now, nextRun.Time); err != nil {
		log.Printf("Warning: failed to update schedule times for agent %d: %v", agentID, err)
	}

	// TODO: Integrate with agent execution service
	// For now, just log that the agent would be executed
	log.Printf("Would execute agent %d (%s) with prompt: %.50s...", agent.ID, agent.Name, agent.Prompt)
	
	// In a real implementation, this would:
	// 1. Create an agent run record
	// 2. Execute the agent with appropriate context/input
	// 3. Handle the execution result
	// 4. Update the run status
}

// GetScheduledAgents returns all currently scheduled agents
func (s *SchedulerService) GetScheduledAgents() ([]int64, error) {
	var agentIDs []int64
	for agentID := range s.agents {
		agentIDs = append(agentIDs, agentID)
	}
	return agentIDs, nil
}

// IsAgentScheduled checks if an agent is currently scheduled
func (s *SchedulerService) IsAgentScheduled(agentID int64) bool {
	_, exists := s.agents[agentID]
	return exists
}

// updateScheduleTimes is a helper function to update schedule times
func (s *SchedulerService) updateScheduleTimes(ctx context.Context, agentID int64, lastRun time.Time, nextRun time.Time) error {
	_, err := s.db.Conn().ExecContext(ctx,
		"UPDATE agents SET last_scheduled_run = ?, next_scheduled_run = ? WHERE id = ?",
		lastRun, nextRun, agentID)
	return err
}