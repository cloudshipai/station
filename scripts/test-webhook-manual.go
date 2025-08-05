package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/models"
)

func main() {
	fmt.Println("🧪 Manual Webhook Test")
	
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	// Initialize database
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()
	
	// Initialize repositories
	repos := repositories.New(database)
	
	// Initialize webhook service
	webhookService := services.NewWebhookService(repos)
	
	// Create a mock agent and agent run for testing
	testAgent := &models.Agent{
		ID:          1,
		Name:        "Test Agent",
		Description: "Test agent for webhook functionality",
		SystemPrompt: "You are a test agent",
		MaxSteps:    5,
		Schedule:    "on-demand",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	testRun := &models.AgentRun{
		ID:          8, // Use the run ID from our actual test
		AgentID:     1,
		Status:      "completed",
		StartedAt:   time.Now().Add(-15 * time.Second),
		CompletedAt: &[]time.Time{time.Now()}[0],
		Result:      "Test webhook functionality by performing a simple directory scan and file analysis",
		StepsTaken:  5,
		Duration:    15000, // 15 seconds in milliseconds
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	fmt.Printf("📋 Testing webhook for Agent: %s (ID: %d)\n", testAgent.Name, testAgent.ID)
	fmt.Printf("📊 Agent Run: %d (Status: %s, Steps: %d)\n", testRun.ID, testRun.Status, testRun.StepsTaken)
	
	// Send webhook notification
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	fmt.Println("🚀 Sending webhook notification...")
	err = webhookService.NotifyAgentRunCompleted(ctx, testRun, testAgent)
	if err != nil {
		log.Fatalf("❌ Failed to send webhook notification: %v", err)
	}
	
	fmt.Println("✅ Webhook notification sent successfully!")
	
	// Wait a moment for the webhook to be delivered
	fmt.Println("⏳ Waiting 5 seconds for webhook delivery...")
	time.Sleep(5 * time.Second)
	
	// Check webhook deliveries
	fmt.Println("🔍 Checking webhook deliveries...")
	webhooks, err := repos.Webhooks.GetAll()
	if err != nil {
		log.Printf("Failed to get webhooks: %v", err)
		return
	}
	
	if len(webhooks) == 0 {
		fmt.Println("⚠️ No webhooks found")
		return
	}
	
	for _, webhook := range webhooks {
		fmt.Printf("📤 Webhook: %s (ID: %d, URL: %s)\n", webhook.Name, webhook.ID, webhook.URL)
		
		deliveries, err := repos.WebhookDeliveries.GetByWebhookID(webhook.ID)
		if err != nil {
			log.Printf("Failed to get deliveries for webhook %d: %v", webhook.ID, err)
			continue
		}
		
		fmt.Printf("📨 Found %d deliveries for webhook %d:\n", len(deliveries), webhook.ID)
		for _, delivery := range deliveries {
			fmt.Printf("  - Delivery %d: Status=%s, EventType=%s, Attempts=%d\n", 
				delivery.ID, delivery.Status, delivery.EventType, delivery.AttemptCount)
		}
	}
	
	fmt.Println("✅ Manual webhook test completed!")
}