package harness_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"station/internal/config"
	"station/internal/genkit/anthropic_oauth"
	"station/internal/logging"
	"station/pkg/harness"
	harness_memory "station/pkg/harness/memory"
	"station/pkg/harness/sandbox"
	"station/pkg/harness/tools"
	"station/pkg/harness/workspace"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/openai/openai-go/option"
)

func TestAgenticExecutor_E2E_RealLLM(t *testing.T) {
	if os.Getenv("HARNESS_E2E_TEST") != "1" {
		t.Skip("Skipping e2e test. Set HARNESS_E2E_TEST=1 to run with real LLM")
	}

	ctx := context.Background()

	configDir := config.GetStationConfigDir()
	configPath := configDir + "/config.yaml"
	t.Logf("Looking for config in: %s", configDir)

	if _, err := os.Stat(configPath); err != nil {
		t.Logf("Config file not found at %s: %v", configPath, err)
	} else {
		t.Logf("Config file exists at %s", configPath)
	}

	if err := config.InitViper(""); err != nil {
		t.Fatalf("Failed to init viper: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	t.Logf("Loaded config - AIProvider: '%s', AIModel: '%s', AIAuthType: '%s', OAuthToken length: %d",
		cfg.AIProvider, cfg.AIModel, cfg.AIAuthType, len(cfg.AIOAuthToken))

	genkitApp, err := initGenkit(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize GenKit: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "harness-e2e-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ws := workspace.NewHostWorkspace(tmpDir)
	if err := ws.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	toolRegistry := tools.NewToolRegistry(genkitApp, tmpDir)
	if err := toolRegistry.RegisterBuiltinTools(); err != nil {
		t.Fatalf("Failed to register tools: %v", err)
	}

	harnessConfig := harness.DefaultHarnessConfig()
	agentConfig := &harness.AgentHarnessConfig{
		MaxSteps:          10,
		DoomLoopThreshold: 3,
		Timeout:           2 * time.Minute,
	}

	modelName := formatModelName(cfg.AIProvider, cfg.AIModel)

	executor := harness.NewAgenticExecutor(
		genkitApp,
		harnessConfig,
		agentConfig,
		harness.WithWorkspace(ws),
		harness.WithModelName(modelName),
	)

	task := "Create a file called 'hello.txt' with the content 'Hello from the agentic harness!' then read it back and confirm the content."

	t.Logf("Executing task: %s", task)
	startTime := time.Now()

	result, err := executor.Execute(ctx, "e2e-test", task, toolRegistry.All())

	t.Logf("Execution completed in %v", time.Since(startTime))
	t.Logf("Result: success=%v, steps=%d, tokens=%d, finish_reason=%s",
		result.Success, result.TotalSteps, result.TotalTokens, result.FinishReason)

	if err != nil {
		t.Logf("Execution error: %v", err)
	}

	if result.Response != "" {
		t.Logf("Response:\n%s", result.Response)
	}

	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.Error)
	}

	helloPath := tmpDir + "/hello.txt"
	if _, err := os.Stat(helloPath); os.IsNotExist(err) {
		t.Error("Expected hello.txt to be created")
	} else {
		content, _ := os.ReadFile(helloPath)
		t.Logf("Created file content: %s", string(content))
	}
}

func TestAgenticExecutor_E2E_MultiStep(t *testing.T) {
	if os.Getenv("HARNESS_E2E_TEST") != "1" {
		t.Skip("Skipping e2e test. Set HARNESS_E2E_TEST=1 to run with real LLM")
	}

	ctx := context.Background()

	if err := config.InitViper(""); err != nil {
		t.Fatalf("Failed to init viper: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	genkitApp, err := initGenkit(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize GenKit: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "harness-e2e-multi-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ws := workspace.NewHostWorkspace(tmpDir)
	if err := ws.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	toolRegistry := tools.NewToolRegistry(genkitApp, tmpDir)
	if err := toolRegistry.RegisterBuiltinTools(); err != nil {
		t.Fatalf("Failed to register tools: %v", err)
	}

	harnessConfig := harness.DefaultHarnessConfig()
	agentConfig := &harness.AgentHarnessConfig{
		MaxSteps:          15,
		DoomLoopThreshold: 3,
		Timeout:           3 * time.Minute,
	}

	modelName := formatModelName(cfg.AIProvider, cfg.AIModel)

	executor := harness.NewAgenticExecutor(
		genkitApp,
		harnessConfig,
		agentConfig,
		harness.WithWorkspace(ws),
		harness.WithModelName(modelName),
	)

	task := `Do the following steps:
1. Create a directory called 'project'
2. Inside 'project', create a file called 'config.json' with content: {"name": "test", "version": "1.0"}
3. List all files in the workspace to confirm
4. Read the config.json file and tell me what version it contains`

	t.Logf("Executing multi-step task")
	startTime := time.Now()

	result, err := executor.Execute(ctx, "e2e-multi-test", task, toolRegistry.All())

	t.Logf("Execution completed in %v", time.Since(startTime))
	t.Logf("Result: success=%v, steps=%d, tokens=%d, finish_reason=%s",
		result.Success, result.TotalSteps, result.TotalTokens, result.FinishReason)

	if err != nil {
		t.Logf("Execution error: %v", err)
	}

	if result.Response != "" {
		t.Logf("Response:\n%s", result.Response)
	}

	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.Error)
	}

	configPath := tmpDir + "/project/config.json"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected project/config.json to be created")
	}
}

func initGenkit(ctx context.Context, cfg *config.Config) (*genkit.Genkit, error) {
	os.Setenv("OTEL_SDK_DISABLED", "true")

	promptDir := "/tmp/harness-e2e-prompts"
	os.MkdirAll(promptDir, 0755)

	provider := strings.ToLower(cfg.AIProvider)

	if strings.HasPrefix(strings.ToLower(cfg.AIModel), "claude") {
		provider = "anthropic"
	} else if strings.HasPrefix(strings.ToLower(cfg.AIModel), "gemini") {
		provider = "gemini"
	} else if strings.HasPrefix(strings.ToLower(cfg.AIModel), "gpt") {
		provider = "openai"
	}

	logging.Info("Initializing GenKit for e2e test with provider: %s, model: %s, base_url: %s", provider, cfg.AIModel, cfg.AIBaseURL)

	switch provider {
	case "openai":
		oaiPlugin := &openai.OpenAI{APIKey: cfg.AIAPIKey}
		// Support OpenAI-compatible providers like Together AI, CloudShip, etc.
		if cfg.AIBaseURL != "" {
			oaiPlugin.Opts = append(oaiPlugin.Opts, option.WithBaseURL(cfg.AIBaseURL))
		}
		return genkit.Init(ctx,
			genkit.WithPlugins(oaiPlugin),
			genkit.WithPromptDir(promptDir)), nil

	case "gemini", "googlegenai":
		return genkit.Init(ctx,
			genkit.WithPlugins(&googlegenai.GoogleAI{}),
			genkit.WithPromptDir(promptDir)), nil

	case "anthropic":
		var plugin *anthropic_oauth.AnthropicOAuth
		if cfg.AIAuthType == "oauth" && cfg.AIOAuthToken != "" {
			plugin = &anthropic_oauth.AnthropicOAuth{OAuthToken: cfg.AIOAuthToken}
		} else if cfg.AIAPIKey != "" {
			plugin = &anthropic_oauth.AnthropicOAuth{APIKey: cfg.AIAPIKey}
		} else {
			return nil, fmt.Errorf("Anthropic requires OAuth token or API key")
		}
		return genkit.Init(ctx,
			genkit.WithPlugins(plugin),
			genkit.WithPromptDir(promptDir)), nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func formatModelName(provider, model string) string {
	provider = strings.ToLower(provider)

	if strings.HasPrefix(strings.ToLower(model), "claude") {
		provider = "anthropic"
	} else if strings.HasPrefix(strings.ToLower(model), "gemini") {
		provider = "googlegenai"
	} else if strings.HasPrefix(strings.ToLower(model), "gpt") {
		provider = "openai"
	}

	return provider + "/" + model
}

func TestAgenticExecutor_E2E_Compaction(t *testing.T) {
	if os.Getenv("HARNESS_E2E_TEST") != "1" {
		t.Skip("Skipping e2e test. Set HARNESS_E2E_TEST=1 to run with real LLM")
	}

	ctx := context.Background()

	if err := config.InitViper(""); err != nil {
		t.Fatalf("Failed to init viper: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	genkitApp, err := initGenkit(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize GenKit: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "harness-e2e-compaction-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ws := workspace.NewHostWorkspace(tmpDir)
	if err := ws.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	toolRegistry := tools.NewToolRegistry(genkitApp, tmpDir)
	if err := toolRegistry.RegisterBuiltinTools(); err != nil {
		t.Fatalf("Failed to register tools: %v", err)
	}

	harnessConfig := harness.DefaultHarnessConfig()
	harnessConfig.Compaction.Enabled = true
	harnessConfig.Compaction.Threshold = 0.3
	harnessConfig.Compaction.ProtectTokens = 200

	agentConfig := &harness.AgentHarnessConfig{
		MaxSteps:          10,
		DoomLoopThreshold: 5,
		Timeout:           3 * time.Minute,
	}

	modelName := formatModelName(cfg.AIProvider, cfg.AIModel)

	compactor := harness.NewCompactor(genkitApp, modelName, harnessConfig.Compaction, 1000)

	executor := harness.NewAgenticExecutor(
		genkitApp,
		harnessConfig,
		agentConfig,
		harness.WithWorkspace(ws),
		harness.WithModelName(modelName),
		harness.WithCompactor(compactor),
	)

	task := `Create two files: 'data.txt' with content 'Important data here' and 'config.json' with content '{"version": "1.0"}'. Then read both files back and summarize what they contain.`

	t.Logf("Executing compaction test task with context window=1000, threshold=0.3")
	startTime := time.Now()

	result, err := executor.Execute(ctx, "e2e-compaction-test", task, toolRegistry.All())

	t.Logf("Execution completed in %v", time.Since(startTime))
	t.Logf("Result: success=%v, steps=%d, tokens=%d, finish_reason=%s",
		result.Success, result.TotalSteps, result.TotalTokens, result.FinishReason)

	if err != nil {
		t.Logf("Execution error: %v", err)
	}

	if result.Response != "" {
		t.Logf("Response:\n%s", result.Response)
	}

	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.Error)
	}

	dataPath := tmpDir + "/data.txt"
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		t.Error("Expected data.txt to be created")
	}

	configPath := tmpDir + "/config.json"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected config.json to be created")
	}

	t.Logf("Compaction test completed - context window was limited to 1000 tokens with 0.3 threshold")
}

func TestAgenticExecutor_E2E_WorkflowSimulation(t *testing.T) {
	if os.Getenv("HARNESS_E2E_TEST") != "1" {
		t.Skip("Skipping e2e test. Set HARNESS_E2E_TEST=1 to run with real LLM")
	}

	ctx := context.Background()

	if err := config.InitViper(""); err != nil {
		t.Fatalf("Failed to init viper: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	genkitApp, err := initGenkit(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize GenKit: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "harness-e2e-workflow-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ws := workspace.NewHostWorkspace(tmpDir)
	if err := ws.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	toolRegistry := tools.NewToolRegistry(genkitApp, tmpDir)
	if err := toolRegistry.RegisterBuiltinTools(); err != nil {
		t.Fatalf("Failed to register tools: %v", err)
	}

	harnessConfig := harness.DefaultHarnessConfig()
	modelName := formatModelName(cfg.AIProvider, cfg.AIModel)

	t.Log("=== Simulating Multi-Agent Workflow with Harness ===")
	t.Log("Step 1: Setup Agent - Create project structure")

	setupConfig := &harness.AgentHarnessConfig{
		MaxSteps:          10,
		DoomLoopThreshold: 3,
		Timeout:           2 * time.Minute,
	}

	setupExecutor := harness.NewAgenticExecutor(
		genkitApp,
		harnessConfig,
		setupConfig,
		harness.WithWorkspace(ws),
		harness.WithModelName(modelName),
	)

	setupTask := "Create a directory called 'test-project' and inside it create a file called 'config.json' with content '{\"name\": \"test\", \"version\": \"1.0\"}'"

	setupResult, err := setupExecutor.Execute(ctx, "setup-agent", setupTask, toolRegistry.All())
	if err != nil {
		t.Fatalf("Setup agent failed: %v", err)
	}

	t.Logf("Setup agent: success=%v, steps=%d, tokens=%d", setupResult.Success, setupResult.TotalSteps, setupResult.TotalTokens)

	if !setupResult.Success {
		t.Fatalf("Setup agent should succeed: %s", setupResult.Error)
	}

	configPath := tmpDir + "/test-project/config.json"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Setup agent should have created test-project/config.json")
	}

	t.Log("Step 2: Verify Agent - Check created files")

	verifyConfig := &harness.AgentHarnessConfig{
		MaxSteps:          10,
		DoomLoopThreshold: 3,
		Timeout:           2 * time.Minute,
	}

	verifyExecutor := harness.NewAgenticExecutor(
		genkitApp,
		harnessConfig,
		verifyConfig,
		harness.WithWorkspace(ws),
		harness.WithModelName(modelName),
	)

	verifyTask := "List all files in the 'test-project' directory and read config.json. Report the version number found."

	verifyResult, err := verifyExecutor.Execute(ctx, "verify-agent", verifyTask, toolRegistry.All())
	if err != nil {
		t.Fatalf("Verify agent failed: %v", err)
	}

	t.Logf("Verify agent: success=%v, steps=%d, tokens=%d", verifyResult.Success, verifyResult.TotalSteps, verifyResult.TotalTokens)
	t.Logf("Verify response: %s", verifyResult.Response)

	if !verifyResult.Success {
		t.Fatalf("Verify agent should succeed: %s", verifyResult.Error)
	}

	if !strings.Contains(verifyResult.Response, "1.0") {
		t.Error("Verify agent should report version 1.0")
	}

	t.Log("=== Workflow Simulation Complete ===")
	t.Logf("Total steps: Setup=%d, Verify=%d", setupResult.TotalSteps, verifyResult.TotalSteps)
	t.Logf("Total tokens: Setup=%d, Verify=%d", setupResult.TotalTokens, verifyResult.TotalTokens)
}

func TestAgenticExecutor_E2E_DockerSandbox(t *testing.T) {
	if os.Getenv("HARNESS_E2E_TEST") != "1" {
		t.Skip("Skipping e2e test. Set HARNESS_E2E_TEST=1 to run with real LLM")
	}

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping Docker sandbox test")
	}

	ctx := context.Background()

	if err := config.InitViper(""); err != nil {
		t.Fatalf("Failed to init viper: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	genkitApp, err := initGenkit(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize GenKit: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "harness-e2e-docker-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Logf("Using workspace directory: %s", tmpDir)

	sbConfig := sandbox.Config{
		Mode:          sandbox.ModeDocker,
		Image:         "python:3.11-slim",
		WorkspacePath: tmpDir,
		Timeout:       5 * time.Minute,
		Resources: sandbox.ResourceConfig{
			CPU:    1,
			Memory: "512m",
			PIDs:   100,
		},
		Network: sandbox.NetworkConfig{
			Enabled: false,
		},
	}

	factory := sandbox.NewFactory(sandbox.DefaultConfig())
	sb, err := factory.Create(sbConfig)
	if err != nil {
		t.Fatalf("Failed to create Docker sandbox: %v", err)
	}
	defer func() {
		if err := sb.Destroy(ctx); err != nil {
			t.Logf("Warning: failed to destroy sandbox: %v", err)
		}
	}()

	if err := sb.Create(ctx); err != nil {
		t.Fatalf("Failed to initialize Docker sandbox: %v", err)
	}

	t.Logf("Docker sandbox created: id=%s, image=%s", sb.ID(), sbConfig.Image)

	ws := workspace.NewHostWorkspace(tmpDir)
	if err := ws.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	toolRegistry := tools.NewToolRegistryWithSandbox(genkitApp, tmpDir, sb)
	if err := toolRegistry.RegisterBuiltinTools(); err != nil {
		t.Fatalf("Failed to register tools: %v", err)
	}

	harnessConfig := harness.DefaultHarnessConfig()
	agentConfig := &harness.AgentHarnessConfig{
		MaxSteps:          15,
		DoomLoopThreshold: 3,
		Timeout:           3 * time.Minute,
	}

	modelName := formatModelName(cfg.AIProvider, cfg.AIModel)

	executor := harness.NewAgenticExecutor(
		genkitApp,
		harnessConfig,
		agentConfig,
		harness.WithWorkspace(ws),
		harness.WithModelName(modelName),
		harness.WithSandbox(sb),
	)

	task := `Create a Python script called 'hello.py' with this content:
print("Hello from Docker sandbox!")
import sys
print(f"Python version: {sys.version}")

Then run it using: python hello.py

Report the output.`

	t.Logf("Executing task in Docker sandbox...")
	startTime := time.Now()

	result, err := executor.Execute(ctx, "docker-e2e-test", task, toolRegistry.All())

	t.Logf("Execution completed in %v", time.Since(startTime))
	t.Logf("Result: success=%v, steps=%d, tokens=%d, finish_reason=%s",
		result.Success, result.TotalSteps, result.TotalTokens, result.FinishReason)

	if err != nil {
		t.Logf("Execution error: %v", err)
	}

	if result.Response != "" {
		t.Logf("Response:\n%s", result.Response)
	}

	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.Error)
	}

	scriptPath := tmpDir + "/hello.py"
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		t.Error("Expected hello.py to be created on host (via mounted volume)")
	} else {
		content, _ := os.ReadFile(scriptPath)
		t.Logf("Script content on host:\n%s", string(content))
	}

	if !strings.Contains(result.Response, "Hello from Docker sandbox") &&
		!strings.Contains(result.Response, "Python version") {
		t.Logf("Warning: Response may not contain expected output")
	}
}

func TestAgenticExecutor_E2E_MemoryFlush(t *testing.T) {
	if os.Getenv("HARNESS_E2E_TEST") != "1" {
		t.Skip("Skipping e2e test. Set HARNESS_E2E_TEST=1 to run with real LLM")
	}

	ctx := context.Background()

	if err := config.InitViper(""); err != nil {
		t.Fatalf("Failed to init viper: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	genkitApp, err := initGenkit(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize GenKit: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "harness-e2e-memflush-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ws := workspace.NewHostWorkspace(tmpDir)
	if err := ws.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	toolRegistry := tools.NewToolRegistry(genkitApp, tmpDir)
	if err := toolRegistry.RegisterBuiltinTools(); err != nil {
		t.Fatalf("Failed to register tools: %v", err)
	}

	// Create memory middleware with workspace
	memBackend := &memoryFSBackend{}
	memMW := harness_memory.NewMemoryMiddlewareWithWorkspace(memBackend, nil, tmpDir)
	if err := memMW.InitializeWorkspaceMemory(); err != nil {
		t.Fatalf("Failed to initialize workspace memory: %v", err)
	}

	harnessConfig := harness.DefaultHarnessConfig()
	agentConfig := &harness.AgentHarnessConfig{
		MaxSteps:           10,
		DoomLoopThreshold:  3,
		Timeout:            2 * time.Minute,
		MemoryFlushEnabled: true, // Enable memory flush
	}

	modelName := formatModelName(cfg.AIProvider, cfg.AIModel)

	executor := harness.NewAgenticExecutor(
		genkitApp,
		harnessConfig,
		agentConfig,
		harness.WithWorkspace(ws),
		harness.WithModelName(modelName),
		harness.WithMemoryMiddleware(memMW),
	)

	task := "Create a file called 'notes.txt' with a brief summary of today's work. Say that you analyzed a codebase and found 3 issues."

	t.Logf("Executing task with memory flush enabled")
	startTime := time.Now()

	result, err := executor.Execute(ctx, "memory-flush-test", task, toolRegistry.All())

	t.Logf("Execution completed in %v", time.Since(startTime))
	t.Logf("Result: success=%v, steps=%d, tokens=%d, finish_reason=%s",
		result.Success, result.TotalSteps, result.TotalTokens, result.FinishReason)

	if err != nil {
		t.Logf("Execution error: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.Error)
	}

	// Verify MEMORY.md was created
	memoryFile := tmpDir + "/MEMORY.md"
	if _, err := os.Stat(memoryFile); os.IsNotExist(err) {
		t.Error("Expected MEMORY.md to be created")
	} else {
		content, _ := os.ReadFile(memoryFile)
		t.Logf("MEMORY.md content:\n%s", string(content))
	}

	// Verify HEARTBEAT.md was created
	heartbeatFile := tmpDir + "/HEARTBEAT.md"
	if _, err := os.Stat(heartbeatFile); os.IsNotExist(err) {
		t.Error("Expected HEARTBEAT.md to be created")
	} else {
		content, _ := os.ReadFile(heartbeatFile)
		t.Logf("HEARTBEAT.md content:\n%s", string(content))
		if !strings.Contains(string(content), "HEARTBEAT_OK") {
			t.Error("HEARTBEAT.md should contain HEARTBEAT_OK token")
		}
	}

	// Verify daily memory log was created (if result was successful with enough content)
	memoryDir := tmpDir + "/memory"
	if info, err := os.Stat(memoryDir); err == nil && info.IsDir() {
		files, _ := os.ReadDir(memoryDir)
		t.Logf("Memory directory contents: %d files", len(files))
		for _, f := range files {
			content, _ := os.ReadFile(memoryDir + "/" + f.Name())
			t.Logf("Memory file %s:\n%s", f.Name(), string(content))
		}
	}

	t.Log("Memory flush test completed successfully")
}

// memoryFSBackend implements memory.Backend for testing
type memoryFSBackend struct{}

func (b *memoryFSBackend) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func TestAgenticExecutor_E2E_DockerWorkflowHandoff(t *testing.T) {
	if os.Getenv("HARNESS_E2E_TEST") != "1" {
		t.Skip("Skipping e2e test. Set HARNESS_E2E_TEST=1 to run with real LLM")
	}

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping Docker sandbox test")
	}

	ctx := context.Background()

	if err := config.InitViper(""); err != nil {
		t.Fatalf("Failed to init viper: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	genkitApp, err := initGenkit(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize GenKit: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "harness-e2e-docker-handoff-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Log("=== Docker Workflow Handoff Test ===")
	t.Logf("Shared workspace: %s", tmpDir)

	harnessConfig := harness.DefaultHarnessConfig()
	modelName := formatModelName(cfg.AIProvider, cfg.AIModel)

	t.Log("Step 1: Agent A creates files in Docker container")

	sbConfigA := sandbox.Config{
		Mode:          sandbox.ModeDocker,
		Image:         "python:3.11-slim",
		WorkspacePath: tmpDir,
		Timeout:       3 * time.Minute,
		Resources: sandbox.ResourceConfig{
			CPU:    1,
			Memory: "512m",
		},
	}

	factoryA := sandbox.NewFactory(sandbox.DefaultConfig())
	sbA, err := factoryA.Create(sbConfigA)
	if err != nil {
		t.Fatalf("Failed to create sandbox A: %v", err)
	}

	if err := sbA.Create(ctx); err != nil {
		t.Fatalf("Failed to initialize sandbox A: %v", err)
	}

	t.Logf("Sandbox A created: %s", sbA.ID())

	wsA := workspace.NewHostWorkspace(tmpDir)
	wsA.Initialize(ctx)

	toolRegistryA := tools.NewToolRegistryWithSandbox(genkitApp, tmpDir, sbA)
	toolRegistryA.RegisterBuiltinTools()

	executorA := harness.NewAgenticExecutor(
		genkitApp,
		harnessConfig,
		&harness.AgentHarnessConfig{MaxSteps: 10, DoomLoopThreshold: 3, Timeout: 2 * time.Minute},
		harness.WithWorkspace(wsA),
		harness.WithModelName(modelName),
		harness.WithSandbox(sbA),
	)

	taskA := "Create a file called 'data.json' with content: {\"created_by\": \"agent_a\", \"value\": 42}"

	resultA, err := executorA.Execute(ctx, "agent-a", taskA, toolRegistryA.All())
	if err != nil || !resultA.Success {
		t.Fatalf("Agent A failed: %v / %s", err, resultA.Error)
	}
	t.Logf("Agent A completed: steps=%d", resultA.TotalSteps)

	if err := sbA.Destroy(ctx); err != nil {
		t.Logf("Warning: failed to destroy sandbox A: %v", err)
	}
	t.Log("Sandbox A destroyed")

	dataPath := tmpDir + "/data.json"
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		t.Fatal("data.json should exist on host after Agent A")
	}
	contentAfterA, _ := os.ReadFile(dataPath)
	t.Logf("File content after Agent A: %s", string(contentAfterA))

	t.Log("Step 2: Agent B reads and modifies files in NEW Docker container")

	sbConfigB := sandbox.Config{
		Mode:          sandbox.ModeDocker,
		Image:         "python:3.11-slim",
		WorkspacePath: tmpDir,
		Timeout:       3 * time.Minute,
		Resources: sandbox.ResourceConfig{
			CPU:    1,
			Memory: "512m",
		},
	}

	factoryB := sandbox.NewFactory(sandbox.DefaultConfig())
	sbB, err := factoryB.Create(sbConfigB)
	if err != nil {
		t.Fatalf("Failed to create sandbox B: %v", err)
	}

	if err := sbB.Create(ctx); err != nil {
		t.Fatalf("Failed to initialize sandbox B: %v", err)
	}

	t.Logf("Sandbox B created: %s (different container!)", sbB.ID())

	wsB := workspace.NewHostWorkspace(tmpDir)
	wsB.Initialize(ctx)

	toolRegistryB := tools.NewToolRegistryWithSandbox(genkitApp, tmpDir, sbB)
	toolRegistryB.RegisterBuiltinTools()

	executorB := harness.NewAgenticExecutor(
		genkitApp,
		harnessConfig,
		&harness.AgentHarnessConfig{MaxSteps: 10, DoomLoopThreshold: 3, Timeout: 2 * time.Minute},
		harness.WithWorkspace(wsB),
		harness.WithModelName(modelName),
		harness.WithSandbox(sbB),
	)

	taskB := "Read the file 'data.json', report who created it and what value is stored."

	resultB, err := executorB.Execute(ctx, "agent-b", taskB, toolRegistryB.All())
	if err != nil || !resultB.Success {
		t.Fatalf("Agent B failed: %v / %s", err, resultB.Error)
	}
	t.Logf("Agent B completed: steps=%d", resultB.TotalSteps)
	t.Logf("Agent B response: %s", resultB.Response)

	if err := sbB.Destroy(ctx); err != nil {
		t.Logf("Warning: failed to destroy sandbox B: %v", err)
	}

	if !strings.Contains(resultB.Response, "agent_a") || !strings.Contains(resultB.Response, "42") {
		t.Error("Agent B should report data created by Agent A")
	}

	t.Log("=== Docker Workflow Handoff Test Complete ===")
	t.Log("Files created by Agent A were successfully read by Agent B in a DIFFERENT container")
	t.Log("This proves workspace volume mounting enables session persistence across container destroys")
}
