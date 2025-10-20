package services

import (
	"context"
	"testing"
	"time"

	"station/internal/db"
	"station/internal/db/repositories"
)

// TestNewSchedulerService tests scheduler creation
func TestNewSchedulerService(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	scheduler := NewSchedulerService(testDB, agentService)

	if scheduler == nil {
		t.Fatal("NewSchedulerService() returned nil")
	}

	if scheduler.cron == nil {
		t.Error("Scheduler cron should be initialized")
	}

	if scheduler.db == nil {
		t.Error("Scheduler database should be initialized")
	}

	if scheduler.agents == nil {
		t.Error("Scheduler agents map should be initialized")
	}

	if scheduler.agentService == nil {
		t.Error("Scheduler agent service should be initialized")
	}
}

// TestSchedulerStartStop tests scheduler lifecycle
func TestSchedulerStartStop(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	scheduler := NewSchedulerService(testDB, agentService)

	tests := []struct {
		name        string
		description string
	}{
		{
			name:        "Start scheduler",
			description: "Should start without errors",
		},
		{
			name:        "Stop scheduler",
			description: "Should stop gracefully",
		},
		{
			name:        "Restart scheduler",
			description: "Should handle start/stop cycles",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Start scheduler
			err := scheduler.Start()
			if err != nil {
				t.Errorf("Start() error = %v", err)
			}

			// Give scheduler time to initialize
			time.Sleep(100 * time.Millisecond)

			// Stop scheduler
			scheduler.Stop()

			// Give scheduler time to shutdown
			time.Sleep(100 * time.Millisecond)
		})
	}
}

// TestScheduleAgent tests agent scheduling
func TestScheduleAgent(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	scheduler := NewSchedulerService(testDB, agentService)

	// Start scheduler
	err = scheduler.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	// Create test environment
	env, err := repos.Environments.Create("test-scheduler-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	tests := []struct {
		name         string
		agentConfig  *AgentConfig
		cronSchedule string
		expectError  bool
		description  string
	}{
		{
			name: "Schedule with valid cron (every minute)",
			agentConfig: &AgentConfig{
				Name:          "test-scheduled-agent",
				Prompt:        "Test",
				MaxSteps:      5,
				EnvironmentID: env.ID,
				CreatedBy:     1,
			},
			cronSchedule: "0 * * * * *", // Every minute at :00 seconds
			expectError:  false,
			description:  "Should schedule agent with valid cron",
		},
		{
			name: "Schedule with invalid cron",
			agentConfig: &AgentConfig{
				Name:          "test-invalid-cron",
				Prompt:        "Test",
				MaxSteps:      5,
				EnvironmentID: env.ID,
				CreatedBy:     1,
			},
			cronSchedule: "invalid cron expression",
			expectError:  true,
			description:  "Should fail with invalid cron expression",
		},
		{
			name: "Schedule with empty cron",
			agentConfig: &AgentConfig{
				Name:          "test-empty-cron",
				Prompt:        "Test",
				MaxSteps:      5,
				EnvironmentID: env.ID,
				CreatedBy:     1,
			},
			cronSchedule: "",
			expectError:  true,
			description:  "Should fail with empty cron expression",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create agent
			agent, err := agentService.CreateAgent(context.Background(), tt.agentConfig)
			if err != nil {
				t.Fatalf("Failed to create agent: %v", err)
			}

			// Set cron schedule
			if tt.cronSchedule != "" {
				agent.CronSchedule = &tt.cronSchedule
			}

			// Schedule agent
			err = scheduler.ScheduleAgent(agent)

			if (err != nil) != tt.expectError {
				t.Errorf("ScheduleAgent() error = %v, expectError %v", err, tt.expectError)
			}

			if !tt.expectError {
				// Verify agent was scheduled
				if _, exists := scheduler.agents[agent.ID]; !exists {
					t.Error("Agent should be in scheduler.agents map")
				}
			}
		})
	}
}

// TestUnscheduleAgent tests agent unscheduling
func TestUnscheduleAgent(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	scheduler := NewSchedulerService(testDB, agentService)

	// Start scheduler
	err = scheduler.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	// Create environment and agent
	env, err := repos.Environments.Create("test-unschedule-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	agent, err := agentService.CreateAgent(context.Background(), &AgentConfig{
		Name:          "test-unschedule-agent",
		Prompt:        "Test",
		MaxSteps:      5,
		EnvironmentID: env.ID,
		CreatedBy:     1,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	cronSchedule := "0 * * * * *"
	agent.CronSchedule = &cronSchedule

	// Schedule agent
	err = scheduler.ScheduleAgent(agent)
	if err != nil {
		t.Fatalf("Failed to schedule agent: %v", err)
	}

	// Verify scheduled
	if _, exists := scheduler.agents[agent.ID]; !exists {
		t.Error("Agent should be scheduled")
	}

	// Unschedule agent
	scheduler.UnscheduleAgent(agent.ID)

	// Verify unscheduled
	if _, exists := scheduler.agents[agent.ID]; exists {
		t.Error("Agent should be unscheduled")
	}

	// Unschedule non-existent agent (should not error)
	scheduler.UnscheduleAgent(99999)
}

// TestRescheduleAgent tests agent rescheduling
func TestRescheduleAgent(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	scheduler := NewSchedulerService(testDB, agentService)

	// Start scheduler
	err = scheduler.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	// Create environment and agent
	env, err := repos.Environments.Create("test-reschedule-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	agent, err := agentService.CreateAgent(context.Background(), &AgentConfig{
		Name:          "test-reschedule-agent",
		Prompt:        "Test",
		MaxSteps:      5,
		EnvironmentID: env.ID,
		CreatedBy:     1,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Schedule with first cron
	cron1 := "0 * * * * *"
	agent.CronSchedule = &cron1
	err = scheduler.ScheduleAgent(agent)
	if err != nil {
		t.Fatalf("Failed to schedule agent: %v", err)
	}

	firstEntryID := scheduler.agents[agent.ID]

	// Reschedule with different cron
	cron2 := "30 * * * * *"
	agent.CronSchedule = &cron2
	err = scheduler.ScheduleAgent(agent)
	if err != nil {
		t.Fatalf("Failed to reschedule agent: %v", err)
	}

	secondEntryID := scheduler.agents[agent.ID]

	// Entry IDs should be different after rescheduling
	if firstEntryID == secondEntryID {
		t.Error("Entry IDs should differ after rescheduling")
	}
}

// TestGetScheduledAgents tests retrieving scheduled agents
func TestGetScheduledAgents(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	scheduler := NewSchedulerService(testDB, agentService)

	// Start scheduler
	err = scheduler.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	// Initially should be empty
	agents, err := scheduler.GetScheduledAgents()
	if err != nil {
		t.Fatalf("GetScheduledAgents() error = %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("GetScheduledAgents() = %d, want 0", len(agents))
	}

	// Create and schedule an agent
	env, err := repos.Environments.Create("test-get-agents-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	agent, err := agentService.CreateAgent(context.Background(), &AgentConfig{
		Name:          "test-get-agent",
		Prompt:        "Test",
		MaxSteps:      5,
		EnvironmentID: env.ID,
		CreatedBy:     1,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	cronSchedule := "0 * * * * *"
	agent.CronSchedule = &cronSchedule
	err = scheduler.ScheduleAgent(agent)
	if err != nil {
		t.Fatalf("Failed to schedule agent: %v", err)
	}

	// Should now have 1 scheduled agent
	agents, err = scheduler.GetScheduledAgents()
	if err != nil {
		t.Fatalf("GetScheduledAgents() error = %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("GetScheduledAgents() = %d, want 1", len(agents))
	}

	if len(agents) > 0 && agents[0] != agent.ID {
		t.Errorf("Scheduled agent ID = %d, want %d", agents[0], agent.ID)
	}
}

// TestSchedulerWithNilSchedule tests handling of nil schedules
func TestSchedulerWithNilSchedule(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	scheduler := NewSchedulerService(testDB, agentService)

	// Start scheduler
	err = scheduler.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	// Create agent without schedule
	env, err := repos.Environments.Create("test-nil-schedule-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	agent, err := agentService.CreateAgent(context.Background(), &AgentConfig{
		Name:          "test-nil-schedule-agent",
		Prompt:        "Test",
		MaxSteps:      5,
		EnvironmentID: env.ID,
		CreatedBy:     1,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// agent.CronSchedule is nil by default
	err = scheduler.ScheduleAgent(agent)
	if err == nil {
		t.Error("ScheduleAgent() should fail with nil schedule")
	}
}

// TestCronExpressionValidation tests various cron expression formats
func TestCronExpressionValidation(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	scheduler := NewSchedulerService(testDB, agentService)

	err = scheduler.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	env, err := repos.Environments.Create("test-cron-validation-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	tests := []struct {
		name         string
		cronSchedule string
		expectError  bool
	}{
		{"Every minute", "0 * * * * *", false},
		{"Every hour", "0 0 * * * *", false},
		{"Daily at midnight", "0 0 0 * * *", false},
		{"Every 30 seconds", "*/30 * * * * *", false},
		{"Descriptor - @hourly", "@hourly", false},
		{"Descriptor - @daily", "@daily", false},
		{"Invalid - too few fields", "* * *", true},
		{"Invalid - bad syntax", "invalid", true},
		{"Invalid - out of range", "99 * * * * *", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := agentService.CreateAgent(context.Background(), &AgentConfig{
				Name:          "test-cron-" + tt.name,
				Prompt:        "Test",
				MaxSteps:      5,
				EnvironmentID: env.ID,
				CreatedBy:     1,
			})
			if err != nil {
				t.Fatalf("Failed to create agent: %v", err)
			}

			agent.CronSchedule = &tt.cronSchedule
			err = scheduler.ScheduleAgent(agent)

			if (err != nil) != tt.expectError {
				t.Errorf("ScheduleAgent() with cron '%s' error = %v, expectError %v",
					tt.cronSchedule, err, tt.expectError)
			}
		})
	}
}

// TestSchedulerGracefulShutdown tests graceful shutdown
func TestSchedulerGracefulShutdown(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	scheduler := NewSchedulerService(testDB, agentService)

	// Start and stop multiple times
	for i := 0; i < 3; i++ {
		err := scheduler.Start()
		if err != nil {
			t.Errorf("Start() iteration %d error = %v", i, err)
		}

		time.Sleep(50 * time.Millisecond)

		scheduler.Stop()

		time.Sleep(50 * time.Millisecond)
	}

	// Verify agents map is cleared
	if len(scheduler.agents) != 0 {
		t.Error("Agents map should be cleared after stop")
	}
}

// Benchmark tests
func BenchmarkScheduleAgent(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	scheduler := NewSchedulerService(testDB, agentService)

	scheduler.Start()
	defer scheduler.Stop()

	env, _ := repos.Environments.Create("bench-env", nil, 1)
	agent, _ := agentService.CreateAgent(context.Background(), &AgentConfig{
		Name:          "bench-agent",
		Prompt:        "Test",
		MaxSteps:      5,
		EnvironmentID: env.ID,
		CreatedBy:     1,
	})

	cronSchedule := "0 * * * * *"
	agent.CronSchedule = &cronSchedule

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.ScheduleAgent(agent)
	}
}

func BenchmarkUnscheduleAgent(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	scheduler := NewSchedulerService(testDB, agentService)

	scheduler.Start()
	defer scheduler.Stop()

	env, _ := repos.Environments.Create("bench-env", nil, 1)
	agent, _ := agentService.CreateAgent(context.Background(), &AgentConfig{
		Name:          "bench-agent",
		Prompt:        "Test",
		MaxSteps:      5,
		EnvironmentID: env.ID,
		CreatedBy:     1,
	})

	cronSchedule := "0 * * * * *"
	agent.CronSchedule = &cronSchedule

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.ScheduleAgent(agent)
		scheduler.UnscheduleAgent(agent.ID)
	}
}
