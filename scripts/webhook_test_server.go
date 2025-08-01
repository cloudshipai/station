package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// WebhookPayload represents the structure of webhook payloads from Station
type WebhookPayload struct {
	Event     string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	Agent     interface{} `json:"agent"`
	Run       interface{} `json:"run"`
	Settings  interface{} `json:"settings,omitempty"`
}

func main() {
	port := "8888"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}

	secret := os.Getenv("WEBHOOK_SECRET")
	if secret != "" {
		log.Printf("🔐 Webhook signature verification enabled with secret")
	} else {
		log.Printf("⚠️  No WEBHOOK_SECRET set - signatures will not be verified")
	}

	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		handleWebhook(w, r, secret)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("🪝 Starting webhook test server on port %s", port)
	log.Printf("📡 Webhook endpoint: http://localhost:%s/webhook", port)
	log.Printf("💡 To test with signature verification, set WEBHOOK_SECRET environment variable")
	log.Printf("Press Ctrl+C to stop")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func handleWebhook(w http.ResponseWriter, r *http.Request, secret string) {
	// Log request details
	log.Printf("📥 Received %s request to %s", r.Method, r.URL.Path)
	log.Printf("🔍 Headers:")
	for name, values := range r.Header {
		for _, value := range values {
			log.Printf("   %s: %s", name, value)
		}
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("❌ Failed to read request body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	log.Printf("📦 Payload size: %d bytes", len(body))

	// Verify signature if secret is provided
	if secret != "" {
		signature := r.Header.Get("X-Station-Signature")
		if signature == "" {
			log.Printf("❌ Missing X-Station-Signature header")
			http.Error(w, "Missing signature", http.StatusUnauthorized)
			return
		}

		expectedSignature := generateSignature(body, secret)
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			log.Printf("❌ Invalid signature. Expected: %s, Got: %s", expectedSignature, signature)
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}

		log.Printf("✅ Signature verified successfully")
	}

	// Parse JSON payload
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("❌ Failed to parse JSON payload: %v", err)
		log.Printf("📄 Raw payload: %s", string(body))
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Log parsed payload
	log.Printf("🎯 Event: %s", payload.Event)
	log.Printf("⏰ Timestamp: %s", payload.Timestamp.Format(time.RFC3339))
	
	// Pretty print the payload
	prettyPayload, err := json.MarshalIndent(payload, "", "  ")
	if err == nil {
		log.Printf("📋 Parsed payload:")
		for _, line := range strings.Split(string(prettyPayload), "\n") {
			log.Printf("   %s", line)
		}
	}

	// Simulate processing time (optional)
	// time.Sleep(100 * time.Millisecond)

	// Return success response
	response := map[string]interface{}{
		"status":    "success",
		"message":   "Webhook processed successfully",
		"event":     payload.Event,
		"timestamp": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("❌ Failed to encode response: %v", err)
	}

	log.Printf("✅ Webhook processed successfully")
	log.Printf("" + strings.Repeat("-", 50))
}

// generateSignature generates HMAC-SHA256 signature for webhook payload
func generateSignature(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}