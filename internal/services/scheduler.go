package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"station/internal/db"
	"station/internal/db/queries"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// SchedulerService manages cron-based agent scheduling
type SchedulerService struct {
	cron         *cron.Cron
	db           db.Database
	repos        *repositories.Repositories
	agents       map[int64]cron.EntryID // Track scheduled agents
	agentService AgentServiceInterface  // Direct agent execution (replaces queue)
}

// NewSchedulerService creates a new scheduler service
func NewSchedulerService(database db.Database, repos *repositories.Repositories, agentService AgentServiceInterface) *SchedulerService {
	// Create cron with seconds precision and logging
	c := cron.New(cron.WithSeconds(), cron.WithLogger(cron.VerbosePrintfLogger(log.New(log.Writer(), "CRON: ", log.LstdFlags))))

	return &SchedulerService{
		cron:         c,
		db:           database,
		repos:        repos,
		agents:       make(map[int64]cron.EntryID),
		agentService: agentService,
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

// Stop stops the cron scheduler with timeout
func (s *SchedulerService) Stop() {
	log.Println("Stopping agent scheduler service...")

	// Create context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Stop scheduler in goroutine
	done := make(chan struct{})
	go func() {
		s.cron.Stop()
		close(done)
	}()

	// Wait for shutdown or timeout
	select {
	case <-done:
		log.Println("Agent scheduler service stopped gracefully")
	case <-ctx.Done():
		log.Println("Agent scheduler service stop timeout - forcing close")
	}

	// Clear agent tracking
	s.agents = make(map[int64]cron.EntryID)
}

// ScheduleAgent adds or updates a scheduled agent
func (s *SchedulerService) ScheduleAgent(agent *models.Agent) error {
	if agent.CronSchedule == nil || *agent.CronSchedule == "" {
		return fmt.Errorf("agent %d has no cron schedule", agent.ID)
	}

	// Remove existing schedule if present
	s.UnscheduleAgent(agent.ID)

	// Parse and validate cron expression (using 6-field format with seconds)
	schedule := *agent.CronSchedule
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	if _, err := parser.Parse(schedule); err != nil {
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
	dbQueries := queries.New(s.db.Conn())

	agents, err := dbQueries.ListScheduledAgents(ctx)
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
			IsScheduled:     agent.IsScheduled.Bool,
			ScheduleEnabled: agent.ScheduleEnabled.Bool,
		}

		if agent.CronSchedule.Valid && agent.ScheduleEnabled.Bool {
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
	// Parse cron expression to calculate next run time (using 6-field format with seconds)
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(cronExpr)
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

// executeScheduledAgent executes a scheduled agent via the execution queue
func (s *SchedulerService) executeScheduledAgent(agentID int64) {
	log.Printf("Executing scheduled agent %d", agentID)

	ctx := context.Background()
	dbQueries := queries.New(s.db.Conn())

	// Get agent details
	agent, err := dbQueries.GetAgentBySchedule(ctx, agentID)
	if err != nil {
		log.Printf("Error: failed to get scheduled agent %d: %v", agentID, err)
		return
	}

	// Update last run time
	now := time.Now()
	nextRun := sql.NullTime{Valid: false}

	// Calculate next run time if schedule is still valid
	if agent.CronSchedule.Valid {
		parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		if schedule, err := parser.Parse(agent.CronSchedule.String); err == nil {
			nextRun = sql.NullTime{Time: schedule.Next(now), Valid: true}
		}
	}

	// Update schedule times in database
	if err := s.updateScheduleTimes(ctx, agentID, now, nextRun.Time); err != nil {
		log.Printf("Warning: failed to update schedule times for agent %d: %v", agentID, err)
	}

	// For scheduled execution, use a simple task description
	// The actual prompt template will be loaded from the .prompt file
	// and schedule_variables will be passed as input variables
	task := "Scheduled execution"
	if agent.Description != "" {
		task = "Scheduled execution: " + agent.Description
	}

	// Parse schedule variables if available
	var scheduleVars map[string]interface{}
	if agent.ScheduleVariables.Valid && agent.ScheduleVariables.String != "" {
		if err := json.Unmarshal([]byte(agent.ScheduleVariables.String), &scheduleVars); err != nil {
			log.Printf("Warning: failed to parse schedule_variables for agent %d: %v", agent.ID, err)
		}
	}

	log.Printf("🔔 Scheduled execution triggered for agent %d (%s) with task: %s", agent.ID, agent.Name, task)

	// Merge schedule variables with metadata
	execVars := map[string]interface{}{
		"source":        "cron_scheduler",
		"cron_schedule": agent.CronSchedule.String,
		"scheduled_at":  now,
	}

	// Add schedule variables to execution context
	if scheduleVars != nil {
		for k, v := range scheduleVars {
			execVars[k] = v
		}
		log.Printf("Executing scheduled agent with variables: %v", scheduleVars)
	}

	// Execute agent in a goroutine to avoid blocking the scheduler
	go func() {
		execCtx := context.Background()

		log.Printf("DEBUG: About to create run record, s.repos is nil: %v", s.repos == nil)

		// Create run record first (using user ID 1 for scheduled/system runs)
		run, err := s.repos.AgentRuns.Create(
			execCtx,
			agent.ID,
			1, // Default console user for scheduled runs
			task,
			"",       // finalResponse empty initially
			0,        // stepsTaken
			nil, nil, // toolCalls, executionSteps
			"running",
			nil, // completedAt
		)

		if err != nil {
			log.Printf("❌ Failed to create run record for scheduled agent %d (%s): %v", agent.ID, agent.Name, err)
			return
		}

		log.Printf("Created run record %d for scheduled agent %d", run.ID, agent.ID)

		// Execute agent with run ID
		result, err := s.agentService.ExecuteAgentWithRunID(execCtx, agent.ID, task, run.ID, execVars)
		if err != nil {
			log.Printf("❌ Scheduled execution failed for agent %d (%s): %v", agent.ID, agent.Name, err)
			// Update run status to failed with error message
			errorMsg := err.Error()
			_ = s.repos.AgentRuns.UpdateCompletionWithMetadata(
				execCtx, run.ID, "", 0, nil, nil, "failed", &now,
				nil, nil, nil, nil, nil, nil, &errorMsg,
			)
		} else {
			log.Printf("✅ Scheduled execution completed for agent %d (%s). Result: %s", agent.ID, agent.Name, result.Content)

			// Extract metadata from result.Extra
			var stepsTaken int64
			var toolsUsed *int64
			var inputTokens, outputTokens, totalTokens *int64
			var duration *float64
			var modelName *string

			if result.Extra != nil {
				if steps, ok := result.Extra["steps_taken"].(int64); ok {
					stepsTaken = steps
				}
				if tools, ok := result.Extra["tools_used"].(int); ok {
					toolsUsedVal := int64(tools)
					toolsUsed = &toolsUsedVal
				}

				// Extract token usage
				if tokenUsage, ok := result.Extra["token_usage"].(map[string]interface{}); ok {
					if input, ok := tokenUsage["input_tokens"].(int64); ok {
						inputTokens = &input
					}
					if output, ok := tokenUsage["output_tokens"].(int64); ok {
						outputTokens = &output
					}
					if total, ok := tokenUsage["total_tokens"].(int64); ok {
						totalTokens = &total
					}
				}

				// Extract duration
				if dur, ok := result.Extra["duration"].(float64); ok {
					duration = &dur
				}

				// Extract model name
				if model, ok := result.Extra["model_name"].(string); ok {
					modelName = &model
				}
			}

			// Update run with completion details including token usage
			completedAt := time.Now()
			updateErr := s.repos.AgentRuns.UpdateCompletionWithMetadata(
				execCtx,
				run.ID,
				result.Content,
				stepsTaken,
				nil, nil, // toolCalls, executionSteps - not available in Message format
				"completed",
				&completedAt,
				inputTokens, outputTokens, totalTokens,
				duration, modelName,
				toolsUsed,
				nil, // error
			)
			if updateErr != nil {
				log.Printf("Warning: Failed to update run completion for run %d: %v", run.ID, updateErr)
			} else {
				log.Printf("✅ Updated run %d with completion status", run.ID)
			}
		}
	}()

	log.Printf("Started execution for scheduled agent %d (%s)", agent.ID, agent.Name)
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
