package anthropic_oauth_test

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"station/internal/genkit/anthropic_oauth"

	"github.com/firebase/genkit/go/genkit"
)

// TestMCPSubprocessAfterAnthropicInit tests if subprocess creation works after Anthropic OAuth init
func TestMCPSubprocessAfterAnthropicInit(t *testing.T) {
	token := os.Getenv("ANTHROPIC_OAUTH_TOKEN")
	if token == "" {
		t.Skip("ANTHROPIC_OAUTH_TOKEN not set")
	}

	ctx := context.Background()

	// Initialize Anthropic OAuth plugin
	oauthPlugin := &anthropic_oauth.AnthropicOAuth{
		OAuthToken: token,
	}

	t.Log("Initializing GenKit with Anthropic OAuth plugin...")
	genkitApp := genkit.Init(ctx, genkit.WithPlugins(oauthPlugin))
	if genkitApp == nil {
		t.Fatal("GenKit app is nil after init")
	}
	t.Log("GenKit initialized successfully")

	// Now try to spawn a subprocess
	t.Log("Attempting to spawn subprocess after Anthropic init...")

	// Use a simple command that should complete quickly
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "echo", "hello")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to spawn subprocess after Anthropic init: %v", err)
	}
	t.Logf("Subprocess output: %s", string(output))
	t.Log("Subprocess spawned successfully after Anthropic init")
}

// TestMCPSubprocessBeforeAnthropicInit tests if subprocess creation works before Anthropic OAuth init
func TestMCPSubprocessBeforeAnthropicInit(t *testing.T) {
	token := os.Getenv("ANTHROPIC_OAUTH_TOKEN")
	if token == "" {
		t.Skip("ANTHROPIC_OAUTH_TOKEN not set")
	}

	ctx := context.Background()

	// First spawn a subprocess
	t.Log("Spawning subprocess before Anthropic init...")
	cmd := exec.Command("echo", "hello")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to spawn subprocess before Anthropic init: %v", err)
	}
	t.Logf("Subprocess output: %s", string(output))

	// Now initialize Anthropic OAuth plugin
	oauthPlugin := &anthropic_oauth.AnthropicOAuth{
		OAuthToken: token,
	}

	t.Log("Initializing GenKit with Anthropic OAuth plugin...")
	genkitApp := genkit.Init(ctx, genkit.WithPlugins(oauthPlugin))
	if genkitApp == nil {
		t.Fatal("GenKit app is nil after init")
	}
	t.Log("GenKit initialized successfully")

	// Try another subprocess after init
	t.Log("Spawning second subprocess after Anthropic init...")
	cmd2 := exec.Command("echo", "world")
	output2, err := cmd2.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to spawn second subprocess after Anthropic init: %v", err)
	}
	t.Logf("Second subprocess output: %s", string(output2))
}
