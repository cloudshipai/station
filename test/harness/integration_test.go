//go:build integration

package harness_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"station/internal/config"
	"station/pkg/harness"
	"station/pkg/harness/workspace"
)

func TestIntegration_HarnessConfigLoading(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Harness.Workspace.Path == "" {
		t.Error("Expected harness workspace path to be set")
	}

	if cfg.Harness.Workspace.Mode == "" {
		t.Error("Expected harness workspace mode to be set")
	}

	t.Logf("Harness config loaded: workspace=%s, mode=%s",
		cfg.Harness.Workspace.Path, cfg.Harness.Workspace.Mode)
}

func TestIntegration_WorkspaceCreation(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	configDir := config.GetStationConfigDir()
	workspacePath := filepath.Join(configDir, cfg.Harness.Workspace.Path)

	ws, err := workspace.NewHostWorkspace(workspacePath, nil)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	testDir := filepath.Join("test-integration", time.Now().Format("20060102-150405"))
	if err := ws.MkdirAll(testDir); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	testFile := filepath.Join(testDir, "test.txt")
	content := []byte("Integration test content")
	if err := ws.WriteFile(testFile, content); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	readContent, err := ws.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	if string(readContent) != string(content) {
		t.Errorf("Content mismatch: got %q, want %q", readContent, content)
	}

	if err := ws.Remove(testDir); err != nil {
		t.Logf("Warning: Failed to cleanup test directory: %v", err)
	}

	t.Logf("Workspace integration test passed: %s", workspacePath)
}

func TestIntegration_ExecutorWithRealLLM(t *testing.T) {
	if os.Getenv("INTEGRATION_LLM_TESTS") != "true" {
		t.Skip("Skipping LLM test: set INTEGRATION_LLM_TESTS=true to enable")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.AIAPIKey == "" {
		t.Skip("Skipping: no AI API key configured")
	}

	configDir := config.GetStationConfigDir()
	workspacePath := filepath.Join(configDir, cfg.Harness.Workspace.Path)

	ws, err := workspace.NewHostWorkspace(workspacePath, nil)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	harnessCfg := harness.DefaultHarnessConfig()
	agentCfg := harness.DefaultAgentHarnessConfig()
	agentCfg.MaxSteps = 5
	agentCfg.Timeout = 2 * time.Minute

	exec := harness.NewExecutor(harnessCfg, agentCfg, ws)

	ctx, cancel := context.WithTimeout(context.Background(), agentCfg.Timeout)
	defer cancel()

	result, err := exec.Execute(ctx, "What is 2+2? Respond with just the number.")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	t.Logf("Execution completed: steps=%d, status=%s", result.Steps, result.Status)
	t.Logf("Response: %s", result.Response)
}

func TestIntegration_GitWorkflow(t *testing.T) {
	if os.Getenv("INTEGRATION_GIT_TESTS") != "true" {
		t.Skip("Skipping git test: set INTEGRATION_GIT_TESTS=true to enable")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	configDir := config.GetStationConfigDir()
	workspacePath := filepath.Join(configDir, cfg.Harness.Workspace.Path)

	if _, err := os.Stat(filepath.Join(workspacePath, ".git")); os.IsNotExist(err) {
		t.Skipf("Skipping: workspace is not a git repository: %s", workspacePath)
	}

	t.Logf("Git workflow test would run against: %s", workspacePath)
}
