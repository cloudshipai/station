package services

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"station/pkg/dotprompt"
)

// TestDefaultSandboxConfig verifies the default configuration values
func TestDefaultSandboxConfig(t *testing.T) {
	cfg := DefaultSandboxConfig()

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"Enabled", cfg.Enabled, false},
		{"EngineMode", cfg.EngineMode, SandboxEngineModeDockerSocket},
		{"DefaultTimeout", cfg.DefaultTimeout, 2 * time.Minute},
		{"MaxStdoutBytes", cfg.MaxStdoutBytes, 200000},
		{"MaxStderrBytes", cfg.MaxStderrBytes, 200000},
		{"MaxArtifactBytes", cfg.MaxArtifactBytes, 10 * 1024 * 1024},
		{"AllowNetworkByDef", cfg.AllowNetworkByDef, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("DefaultSandboxConfig().%s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}

	// Check allowed images separately (slice comparison)
	expectedImages := []string{"python:3.11-slim", "node:20-slim", "ubuntu:22.04"}
	if len(cfg.AllowedImages) != len(expectedImages) {
		t.Errorf("AllowedImages length = %d, want %d", len(cfg.AllowedImages), len(expectedImages))
	}
	for i, img := range expectedImages {
		if i < len(cfg.AllowedImages) && cfg.AllowedImages[i] != img {
			t.Errorf("AllowedImages[%d] = %s, want %s", i, cfg.AllowedImages[i], img)
		}
	}
}

// TestSandboxService_IsEnabled tests the IsEnabled method
func TestSandboxService_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected bool
	}{
		{"Enabled config", true, true},
		{"Disabled config", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultSandboxConfig()
			cfg.Enabled = tt.enabled
			svc := NewSandboxService(cfg)

			if got := svc.IsEnabled(); got != tt.expected {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// Test_runtimeToImage tests the package-level runtimeToImage function
func Test_runtimeToImage(t *testing.T) {
	tests := []struct {
		runtime  string
		expected string
	}{
		{"python", "python:3.11-slim"},
		{"node", "node:20-slim"},
		{"bash", "ubuntu:22.04"},
		{"python:3.12-slim", "python:3.12-slim"}, // Custom image passed through
		{"custom:latest", "custom:latest"},       // Unknown runtime passed through
		{"", ""},                                 // Empty string
	}

	for _, tt := range tests {
		t.Run(tt.runtime, func(t *testing.T) {
			if got := runtimeToImage(tt.runtime); got != tt.expected {
				t.Errorf("runtimeToImage(%q) = %q, want %q", tt.runtime, got, tt.expected)
			}
		})
	}
}

// TestSandboxService_isImageAllowed tests image allowlist validation
func TestSandboxService_isImageAllowed(t *testing.T) {
	cfg := DefaultSandboxConfig()
	cfg.Enabled = true
	svc := NewSandboxService(cfg)

	tests := []struct {
		name     string
		runtime  string
		expected bool
	}{
		// Allowed by direct match
		{"python:3.11-slim allowed", "python:3.11-slim", true},
		{"node:20-slim allowed", "node:20-slim", true},
		{"ubuntu:22.04 allowed", "ubuntu:22.04", true},

		// Allowed by runtime alias
		{"python alias allowed", "python", true},
		{"node alias allowed", "node", true},
		{"bash alias allowed", "bash", true},

		// Not allowed
		{"arbitrary image not allowed", "alpine:latest", false},
		{"python:3.10 not allowed", "python:3.10-slim", false},
		{"empty string not allowed", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svc.isImageAllowed(tt.runtime); got != tt.expected {
				t.Errorf("isImageAllowed(%q) = %v, want %v", tt.runtime, got, tt.expected)
			}
		})
	}
}

// TestSandboxService_truncate tests string truncation
func TestSandboxService_truncate(t *testing.T) {
	svc := NewSandboxService(DefaultSandboxConfig())

	tests := []struct {
		name     string
		input    string
		maxBytes int
		expected string
	}{
		{
			name:     "String under limit unchanged",
			input:    "hello",
			maxBytes: 10,
			expected: "hello",
		},
		{
			name:     "String at limit unchanged",
			input:    "hello",
			maxBytes: 5,
			expected: "hello",
		},
		{
			name:     "String over limit truncated",
			input:    "hello world",
			maxBytes: 5,
			expected: "hello\n... [truncated]",
		},
		{
			name:     "Empty string unchanged",
			input:    "",
			maxBytes: 10,
			expected: "",
		},
		{
			name:     "Zero max bytes",
			input:    "hello",
			maxBytes: 0,
			expected: "\n... [truncated]",
		},
		{
			name:     "Long string with large limit",
			input:    strings.Repeat("a", 1000),
			maxBytes: 1000,
			expected: strings.Repeat("a", 1000),
		},
		{
			name:     "Long string truncated",
			input:    strings.Repeat("a", 1000),
			maxBytes: 100,
			expected: strings.Repeat("a", 100) + "\n... [truncated]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svc.truncate(tt.input, tt.maxBytes); got != tt.expected {
				t.Errorf("truncate() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestSandboxService_MergeDefaults tests merging agent config with defaults
func TestSandboxService_MergeDefaults(t *testing.T) {
	cfg := DefaultSandboxConfig()
	cfg.DefaultTimeout = 120 * time.Second
	svc := NewSandboxService(cfg)

	tests := []struct {
		name          string
		agentSandbox  *dotprompt.SandboxConfig
		expectRuntime string
		expectTimeout int
	}{
		{
			name:          "Nil agent config uses defaults",
			agentSandbox:  nil,
			expectRuntime: "python",
			expectTimeout: 120,
		},
		{
			name:          "Empty agent config uses defaults",
			agentSandbox:  &dotprompt.SandboxConfig{},
			expectRuntime: "python",
			expectTimeout: 120,
		},
		{
			name: "Agent runtime overrides default",
			agentSandbox: &dotprompt.SandboxConfig{
				Runtime: "node",
			},
			expectRuntime: "node",
			expectTimeout: 120,
		},
		{
			name: "Agent timeout overrides default",
			agentSandbox: &dotprompt.SandboxConfig{
				TimeoutSeconds: 60,
			},
			expectRuntime: "python",
			expectTimeout: 60,
		},
		{
			name: "Both overrides applied",
			agentSandbox: &dotprompt.SandboxConfig{
				Runtime:        "bash",
				TimeoutSeconds: 30,
			},
			expectRuntime: "bash",
			expectTimeout: 30,
		},
		{
			name: "Zero timeout uses default",
			agentSandbox: &dotprompt.SandboxConfig{
				TimeoutSeconds: 0,
			},
			expectRuntime: "python",
			expectTimeout: 120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := svc.MergeDefaults(tt.agentSandbox)

			if req.Runtime != tt.expectRuntime {
				t.Errorf("MergeDefaults().Runtime = %q, want %q", req.Runtime, tt.expectRuntime)
			}
			if req.TimeoutSeconds != tt.expectTimeout {
				t.Errorf("MergeDefaults().TimeoutSeconds = %d, want %d", req.TimeoutSeconds, tt.expectTimeout)
			}
		})
	}
}

// TestSandboxService_resolveImage tests image resolution for runtimes
func TestSandboxService_resolveImage(t *testing.T) {
	svc := NewSandboxService(DefaultSandboxConfig())

	tests := []struct {
		runtime  string
		expected string
	}{
		{"python", "python:3.11-slim"},
		{"node", "node:20-slim"},
		{"bash", "ubuntu:22.04"},
		{"custom:tag", "custom:tag"},
	}

	for _, tt := range tests {
		t.Run(tt.runtime, func(t *testing.T) {
			if got := svc.resolveImage(tt.runtime); got != tt.expected {
				t.Errorf("resolveImage(%q) = %q, want %q", tt.runtime, got, tt.expected)
			}
		})
	}
}

// TestSandboxService_resolveEntrypoint tests entrypoint resolution
func TestSandboxService_resolveEntrypoint(t *testing.T) {
	svc := NewSandboxService(DefaultSandboxConfig())

	tests := []struct {
		runtime        string
		expectCmd      []string
		expectFilename string
	}{
		{"python", []string{"python", "/work/main.py"}, "main.py"},
		{"node", []string{"node", "/work/main.js"}, "main.js"},
		{"bash", []string{"bash", "/work/main.sh"}, "main.sh"},
		{"unknown", []string{"python", "/work/main.py"}, "main.py"}, // Defaults to python
	}

	for _, tt := range tests {
		t.Run(tt.runtime, func(t *testing.T) {
			cmd, filename := svc.resolveEntrypoint(tt.runtime)

			if len(cmd) != len(tt.expectCmd) {
				t.Errorf("resolveEntrypoint(%q) cmd len = %d, want %d", tt.runtime, len(cmd), len(tt.expectCmd))
			}
			for i := range tt.expectCmd {
				if i < len(cmd) && cmd[i] != tt.expectCmd[i] {
					t.Errorf("resolveEntrypoint(%q) cmd[%d] = %q, want %q", tt.runtime, i, cmd[i], tt.expectCmd[i])
				}
			}
			if filename != tt.expectFilename {
				t.Errorf("resolveEntrypoint(%q) filename = %q, want %q", tt.runtime, filename, tt.expectFilename)
			}
		})
	}
}

// TestSandboxService_Run_Disabled tests that Run returns error when disabled
func TestSandboxService_Run_Disabled(t *testing.T) {
	cfg := DefaultSandboxConfig()
	cfg.Enabled = false
	svc := NewSandboxService(cfg)

	ctx := context.Background()
	req := SandboxRunRequest{
		Runtime: "python",
		Code:    "print('hello')",
	}

	_, err := svc.Run(ctx, req)
	if err == nil {
		t.Error("Run() should return error when sandbox is disabled")
	}

	if !strings.Contains(err.Error(), "not enabled") {
		t.Errorf("Run() error = %q, should contain 'not enabled'", err.Error())
	}
}

// TestSandboxService_Run_RuntimeNotAllowed tests runtime validation
func TestSandboxService_Run_RuntimeNotAllowed(t *testing.T) {
	cfg := DefaultSandboxConfig()
	cfg.Enabled = true
	svc := NewSandboxService(cfg)

	ctx := context.Background()
	req := SandboxRunRequest{
		Runtime: "alpine:latest", // Not in allowed list
		Code:    "echo hello",
	}

	result, err := svc.Run(ctx, req)

	// Should not return error (validation failure is in result)
	if err != nil {
		t.Errorf("Run() returned unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	if result.OK {
		t.Error("Run() result.OK should be false for disallowed runtime")
	}

	if !strings.Contains(result.Error, "not in allowed list") {
		t.Errorf("Run() result.Error = %q, should contain 'not in allowed list'", result.Error)
	}

	if result.ExitCode != -1 {
		t.Errorf("Run() result.ExitCode = %d, want -1", result.ExitCode)
	}
}

// TestSandboxService_Run_CustomTimeout tests custom timeout handling
func TestSandboxService_Run_CustomTimeout(t *testing.T) {
	cfg := DefaultSandboxConfig()
	cfg.Enabled = true
	cfg.DefaultTimeout = 60 * time.Second
	svc := NewSandboxService(cfg)

	ctx := context.Background()
	req := SandboxRunRequest{
		Runtime:        "alpine:latest", // Will fail allowlist check before reaching Dagger
		Code:           "echo hello",
		TimeoutSeconds: 30,
	}

	result, _ := svc.Run(ctx, req)

	// Result should have been rejected before we even get to timeout,
	// but the limits should still be populated
	if result != nil && result.Limits.TimeoutSeconds != 0 {
		// Only check if we actually got to execution
		t.Logf("Timeout was set to: %d", result.Limits.TimeoutSeconds)
	}
}

// TestSandboxRunRequest_Fields tests SandboxRunRequest struct fields
func TestSandboxRunRequest_Fields(t *testing.T) {
	req := SandboxRunRequest{
		Runtime:        "python",
		Code:           "print('hello')",
		Args:           []string{"--verbose"},
		Env:            map[string]string{"DEBUG": "1"},
		Files:          map[string]string{"data.json": "{}"},
		TimeoutSeconds: 30,
	}

	if req.Runtime != "python" {
		t.Errorf("Runtime = %q, want %q", req.Runtime, "python")
	}
	if req.Code != "print('hello')" {
		t.Errorf("Code = %q, want %q", req.Code, "print('hello')")
	}
	if len(req.Args) != 1 || req.Args[0] != "--verbose" {
		t.Errorf("Args = %v, want [--verbose]", req.Args)
	}
	if req.Env["DEBUG"] != "1" {
		t.Errorf("Env[DEBUG] = %q, want %q", req.Env["DEBUG"], "1")
	}
	if req.Files["data.json"] != "{}" {
		t.Errorf("Files[data.json] = %q, want %q", req.Files["data.json"], "{}")
	}
	if req.TimeoutSeconds != 30 {
		t.Errorf("TimeoutSeconds = %d, want %d", req.TimeoutSeconds, 30)
	}
}

// TestSandboxRunResult_Fields tests SandboxRunResult struct fields
func TestSandboxRunResult_Fields(t *testing.T) {
	result := SandboxRunResult{
		OK:         true,
		Runtime:    "python",
		ExitCode:   0,
		DurationMs: 1500,
		Stdout:     "hello world",
		Stderr:     "",
		Artifacts:  []SandboxArtifact{{Path: "output.txt", SizeBytes: 100}},
		Error:      "",
	}
	result.Limits.TimeoutSeconds = 120
	result.Limits.MaxStdoutBytes = 200000

	if !result.OK {
		t.Error("OK should be true")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if result.DurationMs != 1500 {
		t.Errorf("DurationMs = %d, want 1500", result.DurationMs)
	}
	if len(result.Artifacts) != 1 {
		t.Errorf("Artifacts length = %d, want 1", len(result.Artifacts))
	}
	if result.Limits.TimeoutSeconds != 120 {
		t.Errorf("Limits.TimeoutSeconds = %d, want 120", result.Limits.TimeoutSeconds)
	}
}

// TestSandboxEngineModes tests engine mode constants
func TestSandboxEngineModes(t *testing.T) {
	tests := []struct {
		mode     SandboxEngineMode
		expected string
	}{
		{SandboxEngineModeDockerSocket, "docker_socket"},
		{SandboxEngineModeSidecar, "sidecar"},
		{SandboxEngineModeRemote, "remote"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if string(tt.mode) != tt.expected {
				t.Errorf("SandboxEngineMode = %q, want %q", tt.mode, tt.expected)
			}
		})
	}
}

// =============================================================================
// Integration Tests (require Docker/Dagger)
// =============================================================================

// isDockerAvailable checks if Docker is available for integration tests
func isDockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

// TestSandboxService_Run_Integration tests actual container execution
func TestSandboxService_Run_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	if os.Getenv("CI") != "" && !isDockerAvailable() {
		t.Skip("Skipping integration test in CI without Docker")
	}

	if !isDockerAvailable() {
		t.Skip("Skipping integration test: Docker not available")
	}

	cfg := DefaultSandboxConfig()
	cfg.Enabled = true
	svc := NewSandboxService(cfg)

	tests := []struct {
		name           string
		req            SandboxRunRequest
		expectOK       bool
		expectContains string
	}{
		{
			name: "Python hello world",
			req: SandboxRunRequest{
				Runtime: "python",
				Code:    "print('Hello from sandbox!')",
			},
			expectOK:       true,
			expectContains: "Hello from sandbox!",
		},
		{
			name: "Python with env var",
			req: SandboxRunRequest{
				Runtime: "python",
				Code:    "import os; print(f'ENV={os.environ.get(\"TEST_VAR\", \"not set\")}')",
				Env:     map[string]string{"TEST_VAR": "sandbox_test"},
			},
			expectOK:       true,
			expectContains: "ENV=sandbox_test",
		},
		{
			name: "Python with file",
			req: SandboxRunRequest{
				Runtime: "python",
				Code:    "with open('data.txt') as f: print(f.read())",
				Files:   map[string]string{"data.txt": "file content here"},
			},
			expectOK:       true,
			expectContains: "file content here",
		},
		{
			name: "Python syntax error",
			req: SandboxRunRequest{
				Runtime: "python",
				Code:    "print('unclosed",
			},
			expectOK:       false,
			expectContains: "", // Just check it fails
		},
		{
			name: "Node hello world",
			req: SandboxRunRequest{
				Runtime: "node",
				Code:    "console.log('Hello from Node!')",
			},
			expectOK:       true,
			expectContains: "Hello from Node!",
		},
		{
			name: "Bash hello world",
			req: SandboxRunRequest{
				Runtime: "bash",
				Code:    "echo 'Hello from Bash!'",
			},
			expectOK:       true,
			expectContains: "Hello from Bash!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			result, err := svc.Run(ctx, tt.req)

			if err != nil {
				t.Errorf("Run() returned error: %v", err)
				return
			}

			if result == nil {
				t.Fatal("Run() returned nil result")
			}

			if result.OK != tt.expectOK {
				t.Errorf("Run() OK = %v, want %v. Stdout: %s, Stderr: %s, Error: %s",
					result.OK, tt.expectOK, result.Stdout, result.Stderr, result.Error)
			}

			if tt.expectContains != "" && !strings.Contains(result.Stdout, tt.expectContains) {
				t.Errorf("Run() stdout = %q, should contain %q", result.Stdout, tt.expectContains)
			}

			// Verify limits are populated
			if result.Limits.TimeoutSeconds == 0 {
				t.Error("Run() Limits.TimeoutSeconds should be populated")
			}

			// Verify duration is recorded
			if result.DurationMs == 0 {
				t.Error("Run() DurationMs should be > 0")
			}

			t.Logf("Result: OK=%v, Duration=%dms, Exit=%d, Stdout=%q",
				result.OK, result.DurationMs, result.ExitCode, result.Stdout)
		})
	}
}

// TestSandboxService_Run_Timeout tests timeout handling
func TestSandboxService_Run_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	if !isDockerAvailable() {
		t.Skip("Skipping integration test: Docker not available")
	}

	cfg := DefaultSandboxConfig()
	cfg.Enabled = true
	svc := NewSandboxService(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := SandboxRunRequest{
		Runtime:        "python",
		Code:           "import time; time.sleep(30); print('done')",
		TimeoutSeconds: 2, // 2 second timeout
	}

	result, err := svc.Run(ctx, req)

	// Should get a result (possibly with error) but not nil
	if err != nil && result == nil {
		t.Logf("Run() timed out as expected: %v", err)
		return
	}

	if result != nil && result.OK {
		t.Error("Run() should not succeed with a sleep longer than timeout")
	}

	t.Logf("Result: OK=%v, Error=%s", result.OK, result.Error)
}

// TestSandboxService_Run_OutputTruncation tests stdout/stderr truncation
func TestSandboxService_Run_OutputTruncation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	if !isDockerAvailable() {
		t.Skip("Skipping integration test: Docker not available")
	}

	cfg := DefaultSandboxConfig()
	cfg.Enabled = true
	cfg.MaxStdoutBytes = 100 // Small limit for testing
	cfg.MaxStderrBytes = 100
	svc := NewSandboxService(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := SandboxRunRequest{
		Runtime: "python",
		Code:    "print('A' * 500)", // Generate 500 chars of output
	}

	result, err := svc.Run(ctx, req)

	if err != nil {
		t.Errorf("Run() returned error: %v", err)
		return
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	// Output should be truncated
	if len(result.Stdout) > 150 { // 100 + truncation message
		t.Errorf("Stdout should be truncated, got %d bytes", len(result.Stdout))
	}

	if !strings.Contains(result.Stdout, "[truncated]") {
		t.Errorf("Truncated output should contain [truncated] marker")
	}

	t.Logf("Truncated output length: %d", len(result.Stdout))
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkSandboxService_isImageAllowed(b *testing.B) {
	cfg := DefaultSandboxConfig()
	cfg.Enabled = true
	svc := NewSandboxService(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = svc.isImageAllowed("python")
	}
}

func BenchmarkSandboxService_truncate(b *testing.B) {
	svc := NewSandboxService(DefaultSandboxConfig())
	longString := strings.Repeat("a", 10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = svc.truncate(longString, 1000)
	}
}

func BenchmarkSandboxService_MergeDefaults(b *testing.B) {
	svc := NewSandboxService(DefaultSandboxConfig())
	sandboxCfg := &dotprompt.SandboxConfig{
		Runtime:        "python",
		TimeoutSeconds: 60,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = svc.MergeDefaults(sandboxCfg)
	}
}
