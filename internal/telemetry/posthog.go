package telemetry

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/posthog/posthog-go"
)

type TelemetryService struct {
	client    posthog.Client
	enabled   bool
	userID    string
	machineID string
}

// NewTelemetryService creates a new telemetry service with PostHog
func NewTelemetryService(enabled bool) *TelemetryService {
	if !enabled {
		return &TelemetryService{enabled: false}
	}

	// PostHog configuration with corrected API key
	client, err := posthog.NewWithConfig(
		"phc_3h5yqMKKJsnxofsqFxCEoUFmn3vbm2UFXDDKuhdai9f",
		posthog.Config{
			Endpoint: "https://us.i.posthog.com",
			// Send events more frequently for better responsiveness
			Interval: time.Second * 5,
			BatchSize: 1, // Send events immediately for testing
		},
	)
	if err != nil {
		log.Printf("Failed to initialize PostHog client: %v", err)
		return &TelemetryService{enabled: false}
	}

	service := &TelemetryService{
		client:    client,
		enabled:   true,
		userID:    generateAnonymousUserID(),
		machineID: generateMachineID(),
	}

	// Send initial installation/boot event
	service.TrackEvent("stn_boot", map[string]interface{}{
		"os":           runtime.GOOS,
		"arch":         runtime.GOARCH,
		"go_version":   runtime.Version(),
		"machine_id":   service.machineID,
		"timestamp":    time.Now().UTC(),
	})

	return service
}

// generateAnonymousUserID creates a consistent anonymous user ID
func generateAnonymousUserID() string {
	// Use hostname + system info for consistent anonymous ID
	hostname, _ := os.Hostname()
	data := fmt.Sprintf("%s-%s-%s", hostname, runtime.GOOS, runtime.GOARCH)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("anon_%x", hash[:8])
}

// generateMachineID creates a consistent machine ID for grouping
func generateMachineID() string {
	// Use hostname as base for machine identification
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	hash := sha256.Sum256([]byte(hostname))
	return fmt.Sprintf("machine_%x", hash[:6])
}

// TrackEvent sends an event to PostHog
func (t *TelemetryService) TrackEvent(eventName string, properties map[string]interface{}) {
	if !t.enabled || t.client == nil {
		return
	}

	// Add standard properties
	if properties == nil {
		properties = make(map[string]interface{})
	}
	
	properties["machine_id"] = t.machineID
	properties["os"] = runtime.GOOS
	properties["arch"] = runtime.GOARCH
	properties["timestamp"] = time.Now().UTC()

	// Disable person profile processing for anonymity
	properties["$process_person_profile"] = false

	err := t.client.Enqueue(posthog.Capture{
		DistinctId: t.userID,
		Event:      eventName,
		Properties: properties,
	})

	if err != nil {
		log.Printf("Failed to track event %s: %v", eventName, err)
	}
}

// TrackAgentCreated tracks agent creation events
func (t *TelemetryService) TrackAgentCreated(agentID int64, environmentID int64, toolCount int) {
	t.TrackEvent("stn_agent_created", map[string]interface{}{
		"agent_id":      agentID,
		"environment_id": environmentID,
		"tool_count":    toolCount,
	})
}

// TrackAgentExecuted tracks agent execution events
func (t *TelemetryService) TrackAgentExecuted(agentID int64, executionTimeMs int64, success bool, stepCount int) {
	t.TrackEvent("stn_agent_executed", map[string]interface{}{
		"agent_id":         agentID,
		"execution_time_ms": executionTimeMs,
		"success":          success,
		"step_count":       stepCount,
	})
}

// TrackCLICommand tracks CLI command usage
func (t *TelemetryService) TrackCLICommand(command string, subcommand string, success bool, durationMs int64) {
	t.TrackEvent("stn_cli_command", map[string]interface{}{
		"command":     command,
		"subcommand":  subcommand,
		"success":     success,
		"duration_ms": durationMs,
	})
}

// TrackError tracks error events
func (t *TelemetryService) TrackError(errorType string, errorMessage string, context map[string]interface{}) {
	properties := map[string]interface{}{
		"error_type":    errorType,
		"error_message": errorMessage,
	}
	
	// Merge context properties
	for k, v := range context {
		properties[k] = v
	}
	
	t.TrackEvent("stn_error_occurred", properties)
}

// TrackMCPServerLoaded tracks MCP server loading
func (t *TelemetryService) TrackMCPServerLoaded(serverName string, toolCount int, success bool) {
	t.TrackEvent("stn_mcp_server_loaded", map[string]interface{}{
		"server_name": serverName,
		"tool_count":  toolCount,
		"success":     success,
	})
}

// TrackEnvironmentCreated tracks environment creation
func (t *TelemetryService) TrackEnvironmentCreated(environmentID int64) {
	t.TrackEvent("stn_environment_created", map[string]interface{}{
		"environment_id": environmentID,
	})
}

// TrackTemplateInstalled tracks template bundle installations
func (t *TelemetryService) TrackTemplateInstalled(templateName string, environment string, success bool, durationMs int64) {
	t.TrackEvent("stn_template_installed", map[string]interface{}{
		"template_name": templateName,
		"environment":   environment,
		"success":       success,
		"duration_ms":   durationMs,
	})
}

// TrackAPIRequest tracks API endpoint usage
func (t *TelemetryService) TrackAPIRequest(endpoint string, method string, statusCode int, durationMs int64) {
	t.TrackEvent("stn_api_request", map[string]interface{}{
		"endpoint":    endpoint,
		"method":      method,
		"status_code": statusCode,
		"duration_ms": durationMs,
	})
}

// TrackUIEvent tracks UI interactions
func (t *TelemetryService) TrackUIEvent(eventType string, component string, action string) {
	t.TrackEvent("stn_ui_event", map[string]interface{}{
		"event_type": eventType,
		"component":  component,
		"action":     action,
	})
}

// TrackStdioModeStarted tracks stdio mode usage
func (t *TelemetryService) TrackStdioModeStarted(withAPIServer bool) {
	t.TrackEvent("stn_stdio_mode_started", map[string]interface{}{
		"with_api_server": withAPIServer,
	})
}

// TrackServerModeStarted tracks server mode usage
func (t *TelemetryService) TrackServerModeStarted(apiPort int, mcpPort int, sshPort int) {
	t.TrackEvent("stn_server_mode_started", map[string]interface{}{
		"api_port": apiPort,
		"mcp_port": mcpPort,
		"ssh_port": sshPort,
	})
}

// Flush forces any queued events to be sent immediately
func (t *TelemetryService) Flush() {
	// PostHog Go client doesn't have Flush() method
	// Events are sent based on Interval and BatchSize configuration
	if t.enabled && t.client != nil {
		// Add a small delay to allow batched events to be processed
		time.Sleep(time.Millisecond * 100)
	}
}

// Close gracefully shuts down the telemetry service
func (t *TelemetryService) Close() {
	if t.enabled && t.client != nil {
		// Add delay to allow final events to be sent
		time.Sleep(time.Millisecond * 200)
		t.client.Close()
	}
}

// IsEnabled returns whether telemetry is enabled
func (t *TelemetryService) IsEnabled() bool {
	return t.enabled
}

// GetUserID returns the anonymous user ID
func (t *TelemetryService) GetUserID() string {
	return t.userID
}

// SetEnabled allows runtime enabling/disabling of telemetry
func (t *TelemetryService) SetEnabled(enabled bool) {
	t.enabled = enabled
	if !enabled && t.client != nil {
		// Send opt-out event before disabling
		t.TrackEvent("stn_telemetry_disabled", map[string]interface{}{
			"reason": "user_opt_out",
		})
	}
}