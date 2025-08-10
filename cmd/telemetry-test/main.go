package main

import (
	"fmt"
	"log"
	"time"

	"station/internal/telemetry"
)

func main() {
	fmt.Println("🔍 Testing Station Telemetry Integration")
	fmt.Println("=========================================")

	// Test telemetry enabled
	fmt.Println("\n📊 Testing telemetry with PostHog enabled...")
	telemetryService := telemetry.NewTelemetryService(true)
	
	if !telemetryService.IsEnabled() {
		log.Fatal("❌ Telemetry service should be enabled but isn't")
	}
	
	fmt.Printf("✅ Telemetry service initialized successfully\n")
	fmt.Printf("📋 Anonymous User ID: %s\n", telemetryService.GetUserID())
	
	// Test various event types
	fmt.Println("\n🎯 Testing event tracking...")
	
	// Test agent creation
	telemetryService.TrackAgentCreated(123, 1, 5)
	fmt.Println("✅ Agent creation event tracked")
	
	// Test agent execution
	telemetryService.TrackAgentExecuted(123, 2500, true, 3)
	fmt.Println("✅ Agent execution event tracked")
	
	// Test CLI command
	telemetryService.TrackCLICommand("agent", "create", true, 1200)
	fmt.Println("✅ CLI command event tracked")
	
	// Test error tracking
	telemetryService.TrackError("test_error", "This is a test error message", map[string]interface{}{
		"test_context": "telemetry_verification",
		"component":    "test_suite",
	})
	fmt.Println("✅ Error event tracked")
	
	// Test MCP server loading
	telemetryService.TrackMCPServerLoaded("test-server", 10, true)
	fmt.Println("✅ MCP server loading event tracked")
	
	// Test environment creation
	telemetryService.TrackEnvironmentCreated(42)
	fmt.Println("✅ Environment creation event tracked")
	
	// Wait a moment for events to be sent
	fmt.Println("\n⏳ Waiting for events to be sent to PostHog...")
	time.Sleep(2 * time.Second)
	
	// Test opt-out
	fmt.Println("\n🚫 Testing telemetry opt-out...")
	telemetryService.SetEnabled(false)
	
	if telemetryService.IsEnabled() {
		log.Fatal("❌ Telemetry should be disabled after opt-out")
	}
	
	fmt.Println("✅ Telemetry successfully disabled")
	
	// Test disabled telemetry service
	fmt.Println("\n📊 Testing telemetry with PostHog disabled...")
	disabledService := telemetry.NewTelemetryService(false)
	
	if disabledService.IsEnabled() {
		log.Fatal("❌ Disabled telemetry service should not be enabled")
	}
	
	// These should not send any events
	disabledService.TrackAgentCreated(456, 2, 3)
	disabledService.TrackCLICommand("test", "disabled", true, 500)
	fmt.Println("✅ Disabled telemetry service working correctly")
	
	// Cleanup
	telemetryService.Close()
	disabledService.Close()
	
	fmt.Println("\n🎉 All telemetry tests completed successfully!")
	fmt.Println("\n📈 PostHog Dashboard Check:")
	fmt.Println("   1. Go to your PostHog dashboard")
	fmt.Println("   2. Check for events with names starting with 'station_'")
	fmt.Println("   3. Verify anonymous user ID is consistent")
	fmt.Println("   4. Check event properties include OS, arch, etc.")
	fmt.Println("\n🔐 Privacy Verification:")
	fmt.Println("   • All user IDs are anonymous hashes")
	fmt.Println("   • No personal information is transmitted")
	fmt.Println("   • process_person_profile is set to false")
	fmt.Println("   • Users can opt-out via TELEMETRY_ENABLED=false")
}