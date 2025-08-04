package main

import (
	"fmt"
	"time"
	"station/internal/telemetry"
)

func main() {
	fmt.Println("🚀 Sending test events to PostHog for validation...")
	
	// Create telemetry service
	service := telemetry.NewTelemetryService(true)
	defer service.Close()
	
	// Send multiple test events
	for i := 1; i <= 5; i++ {
		eventName := fmt.Sprintf("test_event_%d", i)
		service.TrackEvent(eventName, map[string]interface{}{
			"test_number": i,
			"message":     fmt.Sprintf("This is test event number %d", i),
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		})
		fmt.Printf("✅ Sent event: %s\n", eventName)
		time.Sleep(time.Millisecond * 100) // Small delay between events
	}
	
	// Send some CLI command simulations
	service.TrackCLICommand("test", "validation", true, 1500)
	service.TrackCLICommand("mcp", "list", true, 800)
	service.TrackCLICommand("agent", "create", true, 2200)
	
	fmt.Printf("✅ Sent 3 CLI command events\n")
	
	// Send other event types
	service.TrackAgentCreated(999, 1, 10)
	service.TrackMCPServerLoaded("test-validation-server", 15, true)
	service.TrackEnvironmentCreated(42)
	
	fmt.Printf("✅ Sent 3 additional event types\n")
	
	fmt.Printf("\n🎯 Total events sent: 11\n")
	fmt.Printf("⏳ Waiting 3 seconds for events to be transmitted...\n")
	
	// Wait for events to be sent
	time.Sleep(time.Second * 3)
	
	fmt.Printf("🚀 All test events sent! Check your PostHog dashboard.\n")
	fmt.Printf("👀 Look for events starting with 'test_event_', 'cli_command', 'agent_created', etc.\n")
	fmt.Printf("🔍 User ID: %s\n", service.GetUserID())
}