package heartbeat

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"station/pkg/harness/memory"
	"station/pkg/harness/prompt"
)

// Service manages periodic heartbeat checks for an agent session
type Service struct {
	config          *prompt.HeartbeatConfig
	memoryMW        *memory.MemoryMiddleware
	logger          *slog.Logger
	workspacePath   string
	heartbeatPrompt string

	// Runtime state
	mu        sync.Mutex
	running   bool
	stopCh    chan struct{}
	ticker    *time.Ticker
	lastCheck time.Time
	checkFn   HeartbeatCheckFunc
}

// HeartbeatCheckFunc is called when a heartbeat check is due.
// It receives the heartbeat prompt and should return the agent's response.
type HeartbeatCheckFunc func(ctx context.Context, prompt string) (string, error)

// NewService creates a new heartbeat service
func NewService(config *prompt.HeartbeatConfig, workspacePath string) *Service {
	return &Service{
		config:        config,
		workspacePath: workspacePath,
		logger:        slog.Default().With("component", "heartbeat_service"),
	}
}

// SetMemoryMiddleware sets the memory middleware for heartbeat file access
func (s *Service) SetMemoryMiddleware(mm *memory.MemoryMiddleware) {
	s.memoryMW = mm
}

// SetCheckFunction sets the function called to execute heartbeat checks
func (s *Service) SetCheckFunction(fn HeartbeatCheckFunc) {
	s.checkFn = fn
}

// Start begins the heartbeat scheduler
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	if s.config == nil || !s.config.Enabled {
		s.logger.Debug("heartbeat disabled, not starting")
		return nil
	}

	interval, err := time.ParseDuration(s.config.Every)
	if err != nil {
		interval = 30 * time.Minute // Default
	}

	s.ticker = time.NewTicker(interval)
	s.stopCh = make(chan struct{})
	s.running = true

	s.logger.Info("heartbeat service started",
		"interval", interval,
		"active_hours", s.formatActiveHours())

	go s.runLoop(ctx)

	return nil
}

// Stop stops the heartbeat scheduler
func (s *Service) Stop() {
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

	s.logger.Info("heartbeat service stopped")
}

// IsRunning returns whether the heartbeat service is active
func (s *Service) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// LastCheck returns the timestamp of the last heartbeat check
func (s *Service) LastCheck() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastCheck
}

// TriggerNow forces an immediate heartbeat check
func (s *Service) TriggerNow(ctx context.Context) (string, error) {
	return s.runCheck(ctx)
}

func (s *Service) runLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-s.ticker.C:
			if !s.isWithinActiveHours() {
				s.logger.Debug("outside active hours, skipping heartbeat")
				continue
			}

			response, err := s.runCheck(ctx)
			if err != nil {
				s.logger.Error("heartbeat check failed", "error", err)
				continue
			}

			// Log result
			if IsHeartbeatOK(response) {
				s.logger.Debug("heartbeat OK")
			} else {
				s.logger.Info("heartbeat notification", "response", truncate(response, 200))
				// Optionally send notification
				if s.config.Notify != nil && s.config.Notify.URL != "" {
					go s.sendNotification(context.Background(), response)
				}
			}
		}
	}
}

func (s *Service) runCheck(ctx context.Context) (string, error) {
	s.mu.Lock()
	s.lastCheck = time.Now()
	checkFn := s.checkFn
	s.mu.Unlock()

	if checkFn == nil {
		return "", fmt.Errorf("no heartbeat check function configured")
	}

	// Load heartbeat prompt from HEARTBEAT.md
	heartbeatPrompt, err := s.loadHeartbeatPrompt()
	if err != nil {
		return "", fmt.Errorf("failed to load heartbeat prompt: %w", err)
	}

	// Execute the heartbeat check
	response, err := checkFn(ctx, heartbeatPrompt)
	if err != nil {
		return "", err
	}

	return response, nil
}

func (s *Service) loadHeartbeatPrompt() (string, error) {
	if s.memoryMW == nil {
		return DefaultHeartbeatPrompt, nil
	}

	// Try to load from workspace HEARTBEAT.md
	contents, err := s.memoryMW.LoadMemory()
	if err != nil {
		return DefaultHeartbeatPrompt, nil
	}

	// Look for heartbeat content
	for source, content := range contents {
		if strings.HasSuffix(source, "HEARTBEAT.md") {
			return content, nil
		}
	}

	return DefaultHeartbeatPrompt, nil
}

func (s *Service) isWithinActiveHours() bool {
	if s.config == nil || s.config.ActiveHours == nil {
		return true // No restriction
	}

	ah := s.config.ActiveHours
	if ah.Start == "" || ah.End == "" {
		return true
	}

	// Get current time in configured timezone
	loc := time.Local
	if ah.Timezone != "" && ah.Timezone != "local" {
		if l, err := time.LoadLocation(ah.Timezone); err == nil {
			loc = l
		}
	}

	now := time.Now().In(loc)
	currentTime := now.Format("15:04")

	// Simple string comparison works for 24h format
	return currentTime >= ah.Start && currentTime <= ah.End
}

func (s *Service) formatActiveHours() string {
	if s.config == nil || s.config.ActiveHours == nil {
		return "always"
	}
	ah := s.config.ActiveHours
	if ah.Start == "" || ah.End == "" {
		return "always"
	}
	tz := ah.Timezone
	if tz == "" {
		tz = "local"
	}
	return fmt.Sprintf("%s-%s (%s)", ah.Start, ah.End, tz)
}

func (s *Service) sendNotification(ctx context.Context, message string) {
	if s.config == nil || s.config.Notify == nil || s.config.Notify.URL == "" {
		return
	}

	// TODO: Implement webhook notification
	// For now, just log
	s.logger.Info("would send notification",
		"channel", s.config.Notify.Channel,
		"url", s.config.Notify.URL,
		"message", truncate(message, 100))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// DefaultHeartbeatPrompt is used when no HEARTBEAT.md is found
const DefaultHeartbeatPrompt = `# Heartbeat Check

Review the current session state and any pending tasks:
1. Check for any alerts or issues that need attention
2. Review any pending notifications or reminders
3. Check workspace state for anything unusual

Rules:
- If nothing needs attention, reply with HEARTBEAT_OK
- If there are items to report, provide a concise summary
- Keep responses actionable and brief
`
