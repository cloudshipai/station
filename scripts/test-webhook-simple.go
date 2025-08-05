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
	fmt.Println("🧪 Simple Webhook Test")
	
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
	
	// Get the actual agent from the database (ID 1)
	agent, err := repos.Agents.GetByID(1)
	if err != nil {
		log.Fatalf("Failed to get agent: %v", err)
	}
	
	fmt.Printf("📋 Found agent: %s (ID: %d)\n", agent.Name, agent.ID)
	
	// Create a test agent run
	testRun := &models.AgentRun{
		ID:            999, // Use a test ID
		AgentID:       agent.ID,
		UserID:        1,
		Task:          "Test webhook functionality",
		FinalResponse: "Webhook test completed successfully",
		StepsTaken:    5,
		Status:        "completed",
		StartedAt:     time.Now().Add(-15 * time.Second),
		CompletedAt:   &[]time.Time{time.Now()}[0],
	}
	
	fmt.Printf("📊 Test Agent Run: %d (Status: %s, Steps: %d)\n", testRun.ID, testRun.Status, testRun.StepsTaken)
	
	// Send webhook notification
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	fmt.Println("🚀 Sending webhook notification...")
	err = webhookService.NotifyAgentRunCompleted(ctx, testRun, agent)
	if err != nil {
		log.Fatalf("❌ Failed to send webhook notification: %v", err)
	}
	
	fmt.Println("✅ Webhook notification sent successfully!")
	fmt.Println("🔍 Check https://station-ntfy.fly.dev/testing for the notification!")
	
	fmt.Println("✅ Simple webhook test completed!")
}