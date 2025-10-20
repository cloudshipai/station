package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"station/internal/db/repositories"
)

// TestDebugLogToFile tests debug logging to file
func TestDebugLogToFile(t *testing.T) {
	// Use temporary directory for test
	tempHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", originalHome)

	tests := []struct {
		name        string
		message     string
		description string
	}{
		{
			name:        "Log simple message",
			message:     "Test debug message",
			description: "Should write message to log file",
		},
		{
			name:        "Log empty message",
			message:     "",
			description: "Should handle empty message",
		},
		{
			name:        "Log multiline message",
			message:     "Line 1\nLine 2\nLine 3",
			description: "Should write multiline message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call debugLogToFile
			debugLogToFile(tt.message)

			// Verify log file was created
			logPath := filepath.Join(tempHome, ".config", "station", "debug-mcp-sync.log")
			if _, err := os.Stat(logPath); os.IsNotExist(err) {
				t.Error("Log file was not created")
				return
			}

			// Read log file
			content, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}

			// Verify content contains message (if not empty)
			if tt.message != "" {
				contentStr := string(content)
				found := false
				for i := 0; i <= len(contentStr)-len(tt.message); i++ {
					if contentStr[i:i+len(tt.message)] == tt.message {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Log file should contain message %q, got:\n%s", tt.message, contentStr)
				}
			}

			// Verify timestamp format exists
			if len(content) > 0 && content[0] != '[' {
				t.Error("Log entry should start with timestamp in brackets")
			}

			t.Logf("Log file created at: %s", logPath)
		})
	}
}

// TestDebugLogToFileDirectoryCreation tests log directory creation
func TestDebugLogToFileDirectoryCreation(t *testing.T) {
	// Use temporary directory for test
	tempHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", originalHome)

	// Ensure .config/station directory doesn't exist
	configDir := filepath.Join(tempHome, ".config", "station")
	if _, err := os.Stat(configDir); !os.IsNotExist(err) {
		t.Fatal("Config directory should not exist initially")
	}

	// Log a message
	debugLogToFile("Test message")

	// Verify directory was created
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Error("debugLogToFile should create .config/station directory")
	}

	// Verify log file exists
	logPath := filepath.Join(configDir, "debug-mcp-sync.log")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("debugLogToFile should create debug log file")
	}
}

// TestDebugLogToFileAppend tests appending to existing log
func TestDebugLogToFileAppend(t *testing.T) {
	// Use temporary directory for test
	tempHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", originalHome)

	// Write first message
	debugLogToFile("First message")

	// Write second message
	debugLogToFile("Second message")

	// Read log file
	logPath := filepath.Join(tempHome, ".config", "station", "debug-mcp-sync.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)

	// Verify both messages exist
	hasFirst := false
	hasSecond := false
	for i := 0; i < len(contentStr); i++ {
		if i+len("First message") <= len(contentStr) && contentStr[i:i+len("First message")] == "First message" {
			hasFirst = true
		}
		if i+len("Second message") <= len(contentStr) && contentStr[i:i+len("Second message")] == "Second message" {
			hasSecond = true
		}
	}

	if !hasFirst {
		t.Error("Log should contain first message")
	}
	if !hasSecond {
		t.Error("Log should contain second message")
	}
}

// TestProcessFileConfigsParallelEmpty tests empty file configs
func TestProcessFileConfigsParallelEmpty(t *testing.T) {
	mcm := &MCPConnectionManager{
		serverPool: NewMCPServerPool(),
	}

	ctx := context.Background()
	fileConfigs := []*repositories.FileConfigRecord{}

	tools, clients := mcm.processFileConfigsParallel(ctx, fileConfigs)

	if tools != nil {
		t.Errorf("Expected nil tools for empty input, got %d tools", len(tools))
	}

	if clients != nil {
		t.Errorf("Expected nil clients for empty input, got %d clients", len(clients))
	}
}

// TestProcessServersParallelEmpty tests empty server data
func TestProcessServersParallelEmpty(t *testing.T) {
	mcm := &MCPConnectionManager{
		serverPool: NewMCPServerPool(),
	}

	ctx := context.Background()
	serversData := map[string]interface{}{}

	tools, clients := mcm.processServersParallel(ctx, serversData)

	if tools != nil {
		t.Errorf("Expected nil tools for empty input, got %d tools", len(tools))
	}

	if clients != nil {
		t.Errorf("Expected nil clients for empty input, got %d clients", len(clients))
	}
}

// TestGetEnvIntOrDefaultHelper tests environment variable parsing
func TestGetEnvIntOrDefaultHelper(t *testing.T) {
	tests := []struct {
		name        string
		envKey      string
		envValue    string
		defaultVal  int
		wantResult  int
		description string
	}{
		{
			name:        "Use environment variable",
			envKey:      "TEST_MCP_WORKERS",
			envValue:    "5",
			defaultVal:  2,
			wantResult:  5,
			description: "Should parse valid integer from env",
		},
		{
			name:        "Use default value",
			envKey:      "TEST_MCP_WORKERS_UNSET",
			envValue:    "",
			defaultVal:  3,
			wantResult:  3,
			description: "Should use default when env not set",
		},
		{
			name:        "Invalid environment value",
			envKey:      "TEST_MCP_WORKERS_INVALID",
			envValue:    "invalid",
			defaultVal:  4,
			wantResult:  4,
			description: "Should use default for invalid env value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable if provided
			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			}

			result := getEnvIntOrDefault(tt.envKey, tt.defaultVal)

			if result != tt.wantResult {
				t.Errorf("getEnvIntOrDefault(%s, %d) = %d, want %d",
					tt.envKey, tt.defaultVal, result, tt.wantResult)
			}
		})
	}
}

// TestWorkerPoolSizing tests worker pool size calculation
func TestWorkerPoolSizing(t *testing.T) {
	tests := []struct {
		name          string
		numConfigs    int
		envWorkers    string
		expectedMax   int
		description   string
	}{
		{
			name:        "More configs than workers",
			numConfigs:  10,
			envWorkers:  "2",
			expectedMax: 2,
			description: "Should use max workers when configs > workers",
		},
		{
			name:        "Fewer configs than workers",
			numConfigs:  3,
			envWorkers:  "5",
			expectedMax: 3,
			description: "Should limit to config count when configs < workers",
		},
		{
			name:        "Equal configs and workers",
			numConfigs:  4,
			envWorkers:  "4",
			expectedMax: 4,
			description: "Should use all workers when equal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envWorkers != "" {
				os.Setenv("STATION_MCP_CONFIG_WORKERS", tt.envWorkers)
				defer os.Unsetenv("STATION_MCP_CONFIG_WORKERS")
			}

			maxWorkers := getEnvIntOrDefault("STATION_MCP_CONFIG_WORKERS", 2)
			if tt.numConfigs < maxWorkers {
				maxWorkers = tt.numConfigs
			}

			if maxWorkers != tt.expectedMax {
				t.Errorf("Worker pool size = %d, want %d", maxWorkers, tt.expectedMax)
			}

			t.Logf("Worker pool: %d workers for %d configs", maxWorkers, tt.numConfigs)
		})
	}
}

// TestParallelProcessingChannelSize tests channel sizing
func TestParallelProcessingChannelSize(t *testing.T) {
	tests := []struct {
		name         string
		numItems     int
		expectedSize int
		description  string
	}{
		{
			name:         "Small batch",
			numItems:     5,
			expectedSize: 5,
			description:  "Channel size should match item count",
		},
		{
			name:         "Large batch",
			numItems:     100,
			expectedSize: 100,
			description:  "Channel should accommodate all items",
		},
		{
			name:         "Single item",
			numItems:     1,
			expectedSize: 1,
			description:  "Should handle single item",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate channel creation
			configChan := make(chan int, tt.numItems)

			if cap(configChan) != tt.expectedSize {
				t.Errorf("Channel capacity = %d, want %d", cap(configChan), tt.expectedSize)
			}

			// Verify channel can hold all items without blocking
			for i := 0; i < tt.numItems; i++ {
				select {
				case configChan <- i:
					// Success
				default:
					t.Errorf("Channel blocked at item %d of %d", i, tt.numItems)
				}
			}

			close(configChan)
			t.Logf("Channel successfully held %d items", tt.numItems)
		})
	}
}

// TestDebugLogToFileInvalidHome tests error handling with invalid home directory
func TestDebugLogToFileInvalidHome(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Unset HOME to simulate error condition
	os.Unsetenv("HOME")

	// Should not panic even with invalid home
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("debugLogToFile panicked with invalid home: %v", r)
		}
	}()

	debugLogToFile("Test message with invalid home")
	// Success if no panic
}

// TestDebugLogToFilePermissionError tests handling of permission errors
func TestDebugLogToFilePermissionError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	// Use temporary directory for test
	tempHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", originalHome)

	// Create .config directory with no write permissions
	configDir := filepath.Join(tempHome, ".config")
	os.MkdirAll(configDir, 0444) // Read-only
	defer os.Chmod(configDir, 0755)

	// Should not panic even with permission error
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("debugLogToFile panicked with permission error: %v", r)
		}
	}()

	debugLogToFile("Test message with permission error")
	// Success if no panic
}

// Benchmark tests
func BenchmarkDebugLogToFile(b *testing.B) {
	tempHome := b.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", originalHome)

	message := "Benchmark test message with some content"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		debugLogToFile(message)
	}
}

func BenchmarkGetEnvIntOrDefault(b *testing.B) {
	os.Setenv("BENCH_TEST_VAR", "10")
	defer os.Unsetenv("BENCH_TEST_VAR")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getEnvIntOrDefault("BENCH_TEST_VAR", 5)
	}
}

func BenchmarkChannelOperations(b *testing.B) {
	ch := make(chan int, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		select {
		case ch <- i:
		default:
		}
	}
}
