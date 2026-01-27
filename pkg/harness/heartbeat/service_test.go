package heartbeat

import (
	"context"
	"testing"
	"time"

	"station/pkg/harness/prompt"
)

func TestIsHeartbeatOK(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     bool
	}{
		{"exact match", "HEARTBEAT_OK", true},
		{"with whitespace", "  HEARTBEAT_OK  ", true},
		{"lowercase", "heartbeat_ok", true},
		{"with prefix", "Status: HEARTBEAT_OK", true},
		{"with suffix", "HEARTBEAT_OK - all good", true},
		{"empty", "", false},
		{"other response", "There are 3 pending tasks", false},
		{"similar but not", "HEARTBEAT_NOT_OK", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHeartbeatOK(tt.response); got != tt.want {
				t.Errorf("IsHeartbeatOK(%q) = %v, want %v", tt.response, got, tt.want)
			}
		})
	}
}

func TestIsNoReply(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     bool
	}{
		{"exact match", "NO_REPLY", true},
		{"with whitespace", "  NO_REPLY  ", true},
		{"lowercase", "no_reply", true},
		{"with context", "Memory flushed. NO_REPLY", true},
		{"empty", "", false},
		{"other response", "Please respond", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNoReply(tt.response); got != tt.want {
				t.Errorf("IsNoReply(%q) = %v, want %v", tt.response, got, tt.want)
			}
		})
	}
}

func TestShouldNotify(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     bool
	}{
		{"heartbeat ok", "HEARTBEAT_OK", false},
		{"no reply", "NO_REPLY", false},
		{"empty", "", false},
		{"whitespace only", "   ", false},
		{"notification needed", "3 pending tasks require attention", true},
		{"alert", "ALERT: Service down", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldNotify(tt.response); got != tt.want {
				t.Errorf("ShouldNotify(%q) = %v, want %v", tt.response, got, tt.want)
			}
		})
	}
}

func TestServiceStartStop(t *testing.T) {
	config := &prompt.HeartbeatConfig{
		Enabled: true,
		Every:   "1s",
	}

	svc := NewService(config, "/tmp/test")

	// Start should succeed
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !svc.IsRunning() {
		t.Error("IsRunning() = false after Start()")
	}

	// Starting again should be idempotent
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Second Start() error = %v", err)
	}

	// Stop
	svc.Stop()

	if svc.IsRunning() {
		t.Error("IsRunning() = true after Stop()")
	}
}

func TestServiceDisabled(t *testing.T) {
	// Nil config
	svc1 := NewService(nil, "/tmp/test")
	ctx := context.Background()
	if err := svc1.Start(ctx); err != nil {
		t.Errorf("Start() with nil config error = %v", err)
	}
	if svc1.IsRunning() {
		t.Error("Service running with nil config")
	}

	// Disabled config
	config := &prompt.HeartbeatConfig{
		Enabled: false,
		Every:   "1m",
	}
	svc2 := NewService(config, "/tmp/test")
	if err := svc2.Start(ctx); err != nil {
		t.Errorf("Start() with disabled config error = %v", err)
	}
	if svc2.IsRunning() {
		t.Error("Service running with disabled config")
	}
}

func TestTriggerNow(t *testing.T) {
	config := &prompt.HeartbeatConfig{
		Enabled: true,
		Every:   "1h", // Long interval
	}

	svc := NewService(config, "/tmp/test")

	// Set a simple check function
	checkCalled := false
	svc.SetCheckFunction(func(ctx context.Context, prompt string) (string, error) {
		checkCalled = true
		return "HEARTBEAT_OK", nil
	})

	ctx := context.Background()
	response, err := svc.TriggerNow(ctx)
	if err != nil {
		t.Fatalf("TriggerNow() error = %v", err)
	}

	if !checkCalled {
		t.Error("Check function was not called")
	}

	if response != "HEARTBEAT_OK" {
		t.Errorf("TriggerNow() response = %q, want %q", response, "HEARTBEAT_OK")
	}

	// LastCheck should be updated
	if svc.LastCheck().IsZero() {
		t.Error("LastCheck() is zero after TriggerNow()")
	}
}

func TestActiveHours(t *testing.T) {
	// Create a service with active hours
	config := &prompt.HeartbeatConfig{
		Enabled: true,
		Every:   "1m",
		ActiveHours: &prompt.ActiveHoursConfig{
			Start:    "00:00",
			End:      "23:59",
			Timezone: "UTC",
		},
	}

	svc := NewService(config, "/tmp/test")

	// Should always be within active hours with this config
	if !svc.isWithinActiveHours() {
		t.Error("isWithinActiveHours() = false with 00:00-23:59")
	}

	// Test with narrow window (may or may not be active depending on time)
	config2 := &prompt.HeartbeatConfig{
		Enabled: true,
		Every:   "1m",
		ActiveHours: &prompt.ActiveHoursConfig{
			Start:    "03:00",
			End:      "03:01",
			Timezone: "UTC",
		},
	}

	svc2 := NewService(config2, "/tmp/test")
	now := time.Now().UTC()
	currentHour := now.Hour()

	// If current hour is 3, it might be active; otherwise definitely not
	if currentHour != 3 && svc2.isWithinActiveHours() {
		t.Errorf("isWithinActiveHours() = true outside 03:00-03:01 window (current hour: %d)", currentHour)
	}
}

func TestConfigHelpers(t *testing.T) {
	// Test nil config
	if IsEnabled(nil) {
		t.Error("IsEnabled(nil) = true")
	}
	if got := GetInterval(nil); got != 30*time.Minute {
		t.Errorf("GetInterval(nil) = %v, want 30m", got)
	}
	if got := GetSessionMode(nil); got != "main" {
		t.Errorf("GetSessionMode(nil) = %q, want %q", got, "main")
	}

	// Test with config
	cfg := &prompt.HeartbeatConfig{
		Enabled: true,
		Every:   "5m",
		Session: "isolated",
	}

	if !IsEnabled(cfg) {
		t.Error("IsEnabled() = false for enabled config")
	}
	if got := GetInterval(cfg); got != 5*time.Minute {
		t.Errorf("GetInterval() = %v, want 5m", got)
	}
	if got := GetSessionMode(cfg); got != "isolated" {
		t.Errorf("GetSessionMode() = %q, want %q", got, "isolated")
	}
	if !IsIsolatedSession(cfg) {
		t.Error("IsIsolatedSession() = false for isolated config")
	}

	// Test invalid duration
	cfg2 := &prompt.HeartbeatConfig{
		Every: "invalid",
	}
	if got := GetInterval(cfg2); got != 30*time.Minute {
		t.Errorf("GetInterval() with invalid = %v, want 30m", got)
	}
}
