package services

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"station/internal/db/repositories"
	"station/internal/workflows"
	"station/pkg/models"
)

type WorkflowSchedulerService struct {
	repos           *repositories.Repositories
	workflowService *WorkflowService
	parser          cron.Parser
	ticker          *time.Ticker
	stopCh          chan struct{}
	mu              sync.Mutex
	running         bool
}

func NewWorkflowSchedulerService(repos *repositories.Repositories, workflowService *WorkflowService) *WorkflowSchedulerService {
	return &WorkflowSchedulerService{
		repos:           repos,
		workflowService: workflowService,
		parser:          cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
		stopCh:          make(chan struct{}),
	}
}

func (s *WorkflowSchedulerService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.ticker = time.NewTicker(1 * time.Minute)
	s.mu.Unlock()

	log.Println("[WorkflowScheduler] Started - checking for due workflows every minute")

	go func() {
		s.checkAndTrigger(ctx)

		for {
			select {
			case <-s.ticker.C:
				s.checkAndTrigger(ctx)
			case <-s.stopCh:
				log.Println("[WorkflowScheduler] Stopped")
				return
			case <-ctx.Done():
				log.Println("[WorkflowScheduler] Context cancelled, stopping")
				return
			}
		}
	}()

	return nil
}

func (s *WorkflowSchedulerService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	if s.ticker != nil {
		s.ticker.Stop()
	}
	close(s.stopCh)
}

func (s *WorkflowSchedulerService) checkAndTrigger(ctx context.Context) {
	now := time.Now()

	schedules, err := s.repos.WorkflowSchedules.ListDue(ctx, now)
	if err != nil {
		log.Printf("[WorkflowScheduler] Error listing due schedules: %v", err)
		return
	}

	if len(schedules) == 0 {
		return
	}

	log.Printf("[WorkflowScheduler] Found %d due schedule(s)", len(schedules))

	for _, schedule := range schedules {
		s.triggerSchedule(ctx, schedule, now)
	}
}

func (s *WorkflowSchedulerService) triggerSchedule(ctx context.Context, schedule *models.WorkflowSchedule, now time.Time) {
	log.Printf("[WorkflowScheduler] Triggering workflow %s (version %d)", schedule.WorkflowID, schedule.WorkflowVersion)

	_, _, err := s.workflowService.StartRun(ctx, StartWorkflowRunRequest{
		WorkflowID: schedule.WorkflowID,
		Version:    schedule.WorkflowVersion,
		Input:      schedule.Input,
	})
	if err != nil {
		log.Printf("[WorkflowScheduler] Error starting workflow run for %s: %v", schedule.WorkflowID, err)
		return
	}

	nextRunAt, err := s.calculateNextRun(schedule.CronExpression, schedule.Timezone, now)
	if err != nil {
		log.Printf("[WorkflowScheduler] Error calculating next run for %s: %v", schedule.WorkflowID, err)
		return
	}

	if err := s.repos.WorkflowSchedules.UpdateLastRun(ctx, schedule.ID, now, nextRunAt); err != nil {
		log.Printf("[WorkflowScheduler] Error updating schedule %d: %v", schedule.ID, err)
	}

	log.Printf("[WorkflowScheduler] Workflow %s triggered, next run at %s", schedule.WorkflowID, nextRunAt.Format(time.RFC3339))
}

func (s *WorkflowSchedulerService) calculateNextRun(cronExpr, timezone string, from time.Time) (time.Time, error) {
	loc := time.UTC
	if timezone != "" && timezone != "UTC" {
		var err error
		loc, err = time.LoadLocation(timezone)
		if err != nil {
			log.Printf("[WorkflowScheduler] Invalid timezone %s, using UTC: %v", timezone, err)
			loc = time.UTC
		}
	}

	schedule, err := s.parser.Parse(cronExpr)
	if err != nil {
		return time.Time{}, err
	}

	return schedule.Next(from.In(loc)).UTC(), nil
}

func (s *WorkflowSchedulerService) RegisterWorkflowSchedule(ctx context.Context, def *workflows.Definition, version int64) error {
	cronState := s.findCronState(def)
	if cronState == nil {
		if err := s.repos.WorkflowSchedules.DeleteByWorkflowID(ctx, def.ID); err != nil {
			log.Printf("[WorkflowScheduler] Error deleting schedule for %s: %v", def.ID, err)
		}
		return nil
	}

	enabled := true
	if cronState.Enabled != nil {
		enabled = *cronState.Enabled
	}

	timezone := cronState.Timezone
	if timezone == "" {
		timezone = "UTC"
	}

	var input json.RawMessage
	if cronState.Input != nil {
		inputBytes, err := json.Marshal(cronState.Input)
		if err == nil {
			input = inputBytes
		}
	}

	nextRunAt, err := s.calculateNextRun(cronState.Cron, timezone, time.Now())
	if err != nil {
		return err
	}

	existing, _ := s.repos.WorkflowSchedules.Get(ctx, def.ID, version)
	if existing != nil {
		_ = s.repos.WorkflowSchedules.Delete(ctx, def.ID, version)
	}

	_, err = s.repos.WorkflowSchedules.Create(ctx, repositories.CreateWorkflowScheduleParams{
		WorkflowID:      def.ID,
		WorkflowVersion: version,
		CronExpression:  cronState.Cron,
		Timezone:        timezone,
		Enabled:         enabled,
		Input:           input,
		NextRunAt:       &nextRunAt,
	})

	if err != nil {
		return err
	}

	log.Printf("[WorkflowScheduler] Registered schedule for workflow %s: %s (next run: %s)",
		def.ID, cronState.Cron, nextRunAt.Format(time.RFC3339))

	return nil
}

func (s *WorkflowSchedulerService) findCronState(def *workflows.Definition) *workflows.StateSpec {
	if def.Start == "" {
		return nil
	}

	for i := range def.States {
		state := &def.States[i]
		if state.StableID() == def.Start && state.Type == "cron" {
			return state
		}
	}

	return nil
}

func (s *WorkflowSchedulerService) UnregisterWorkflowSchedule(ctx context.Context, workflowID string) error {
	return s.repos.WorkflowSchedules.DeleteByWorkflowID(ctx, workflowID)
}

func (s *WorkflowSchedulerService) ListSchedules(ctx context.Context) ([]*models.WorkflowSchedule, error) {
	return s.repos.WorkflowSchedules.ListEnabled(ctx)
}

func (s *WorkflowSchedulerService) SetScheduleEnabled(ctx context.Context, workflowID string, version int64, enabled bool) error {
	return s.repos.WorkflowSchedules.SetEnabled(ctx, workflowID, version, enabled)
}
