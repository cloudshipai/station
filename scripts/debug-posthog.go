package main

import (
	"fmt"
	"log"
	"time"

	"github.com/posthog/posthog-go"
)

func main() {
	fmt.Println("🔍 Debugging PostHog Connection...")
	
	// Enable debug logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	
	// Create PostHog client with debug configuration
	client, err := posthog.NewWithConfig(
		"phc_3h5yqMKKJsnxofsqFxCEoUFmn3vbm2UFXDDKuhdai9f",
		posthog.Config{
			Endpoint:  "https://us.i.posthog.com",
			Interval:  time.Second * 2,  // Very frequent
			BatchSize: 1,               // Send immediately
			Verbose:   true,            // Enable verbose logging
		},
	)
	
	if err != nil {
		log.Fatalf("❌ Failed to create PostHog client: %v", err)
	}
	
	fmt.Println("✅ PostHog client created successfully")
	
	// Send a simple test event
	fmt.Println("📤 Sending test event...")
	
	err = client.Enqueue(posthog.Capture{
		DistinctId: "debug-test-user",
		Event:      "debug_test_event",
		Properties: posthog.NewProperties().
			Set("test", "debug_connection").
			Set("timestamp", time.Now().UTC().Format(time.RFC3339)).
			Set("$process_person_profile", false),
	})
	
	if err != nil {
		log.Printf("❌ Failed to enqueue event: %v", err)
	} else {
		fmt.Println("✅ Event enqueued successfully")
	}
	
	// Wait for event to be sent
	fmt.Println("⏳ Waiting 10 seconds for event transmission...")
	time.Sleep(time.Second * 10)
	
	// Close client
	fmt.Println("🔒 Closing PostHog client...")
	client.Close()
	
	fmt.Println("🎯 Debug test complete!")
	fmt.Println("📋 Check PostHog dashboard for event: 'debug_test_event' from user 'debug-test-user'")
}