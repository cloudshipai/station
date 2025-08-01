package main

import (
	"log"
	"time"

	"github.com/posthog/posthog-go"
)

func main() {
	log.Println("🔥 Sending direct PostHog verification event...")
	
	// Use exact same config as Station
	client, err := posthog.NewWithConfig(
		"phc_mEeFH3zxHHot6dGC5ZfQPPBjm2rApGpVZwpKYPYwZD",
		posthog.Config{
			Endpoint: "https://us.i.posthog.com",
		},
	)
	if err != nil {
		log.Fatalf("Failed to create PostHog client: %v", err)
	}
	defer client.Close()

	// Send a very obvious test event
	err = client.Enqueue(posthog.Capture{
		DistinctId: "test-verification-user",
		Event:      "posthog_integration_test",
		Properties: map[string]interface{}{
			"test_message":             "This is a verification test from Station",
			"timestamp":                time.Now().UTC(),
			"integration_status":       "testing",
			"$process_person_profile":  false, // Anonymous tracking
		},
	})

	if err != nil {
		log.Fatalf("Failed to send event: %v", err)
	}

	log.Println("✅ Verification event sent successfully!")
	log.Println("⏳ Waiting 3 seconds for delivery...")
	time.Sleep(3 * time.Second)
	log.Println("🎯 Check your PostHog dashboard for 'posthog_integration_test' event")
}