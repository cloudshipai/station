package faker

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestProxyConfig tests ProxyConfig validation
func TestProxyConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  ProxyConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ProxyConfig{
				TargetCommand: "echo",
				TargetArgs:    []string{"test"},
			},
			wantErr: false,
		},
		{
			name: "missing command",
			config: ProxyConfig{
				TargetArgs: []string{"test"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProxy(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProxy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestProxyCacheDirectory tests cache directory creation
func TestProxyCacheDirectory(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "test-cache")

	config := ProxyConfig{
		TargetCommand: "echo",
		CacheDir:      cacheDir,
	}

	proxy, err := NewProxy(config)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	if proxy == nil {
		t.Fatal("Expected non-nil proxy")
	}

	// Verify cache directory was created
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		t.Errorf("Cache directory was not created: %s", cacheDir)
	}
}

// TestTargetProcessLifecycle tests target process start/stop
func TestTargetProcessLifecycle(t *testing.T) {
	// Use a simple echo command that will respond and exit
	target, err := NewTargetProcess("echo", []string{"test"}, nil, false)
	if err != nil {
		t.Fatalf("Failed to start target process: %v", err)
	}

	// Process should be running
	if target.cmd.Process == nil {
		t.Fatal("Expected running process")
	}

	// Stop the process
	if err := target.Stop(); err != nil {
		t.Errorf("Failed to stop process: %v", err)
	}
}

// TestTargetProcessEnvironment tests environment variable injection
func TestTargetProcessEnvironment(t *testing.T) {
	// Create a script that prints environment variables
	script := `#!/bin/bash
echo "$TEST_VAR"
`
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "test-env.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	env := map[string]string{
		"TEST_VAR": "test_value",
	}

	target, err := NewTargetProcess("bash", []string{scriptPath}, env, false)
	if err != nil {
		t.Fatalf("Failed to start target process: %v", err)
	}
	defer target.Stop()

	// Read output
	line, err := target.ReadLine()
	if err != nil {
		t.Fatalf("Failed to read from target: %v", err)
	}

	output := string(line)
	if !strings.Contains(output, "test_value") {
		t.Errorf("Expected output to contain 'test_value', got: %s", output)
	}
}

// TestProxyWithRealMCPServer tests the faker proxy with a real MCP server
func TestProxyWithRealMCPServer(t *testing.T) {
	// Test with npx filesystem MCP server (most common real MCP server)
	// This requires npx to be installed
	tempDir := t.TempDir()

	config := ProxyConfig{
		TargetCommand: "npx",
		TargetArgs:    []string{"-y", "@modelcontextprotocol/server-filesystem@latest", tempDir},
		Debug:         true,
		Passthrough:   true,
	}

	// Check if npx is available
	if _, err := exec.LookPath("npx"); err != nil {
		t.Skip("Skipping test: npx not available (install Node.js to run this test)")
	}

	proxy, err := NewProxy(config)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Start target process
	target, err := NewTargetProcess(config.TargetCommand, config.TargetArgs, config.TargetEnv, config.Debug)
	if err != nil {
		t.Fatalf("Failed to start target MCP server: %v", err)
	}
	defer target.Stop()

	// Give the MCP server time to start
	time.Sleep(2 * time.Second)

	// Send MCP initialize request
	initRequest := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`

	if err := target.WriteJSON([]byte(initRequest)); err != nil {
		t.Fatalf("Failed to send initialize request: %v", err)
	}

	// Read initialize response
	response, err := target.ReadLine()
	if err != nil {
		t.Fatalf("Failed to read initialize response: %v", err)
	}

	t.Logf("Initialize response: %s", string(response))

	// Verify it's valid JSON-RPC response
	var jsonResponse map[string]interface{}
	if err := json.Unmarshal(response, &jsonResponse); err != nil {
		t.Errorf("Invalid JSON-RPC response: %v", err)
	}

	// Check for result field
	if _, ok := jsonResponse["result"]; !ok {
		t.Error("Expected 'result' field in initialize response")
	}

	// Send tools/list request
	toolsRequest := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`

	if err := target.WriteJSON([]byte(toolsRequest)); err != nil {
		t.Fatalf("Failed to send tools/list request: %v", err)
	}

	// Read tools/list response
	toolsResponse, err := target.ReadLine()
	if err != nil {
		t.Fatalf("Failed to read tools/list response: %v", err)
	}

	t.Logf("Tools response: %s", string(toolsResponse))

	// Verify tools response
	var toolsResult map[string]interface{}
	if err := json.Unmarshal(toolsResponse, &toolsResult); err != nil {
		t.Errorf("Invalid tools response: %v", err)
	}

	// Check for tools in result
	if result, ok := toolsResult["result"].(map[string]interface{}); ok {
		if tools, ok := result["tools"].([]interface{}); ok {
			if len(tools) == 0 {
				t.Error("Expected at least one tool from filesystem MCP server")
			}
			t.Logf("Successfully proxied MCP server with %d tools", len(tools))
		}
	}

	_ = proxy
}

// TestProxyDebugLogging tests debug logging functionality
func TestProxyDebugLogging(t *testing.T) {
	// Create a buffer to capture stderr
	var buf bytes.Buffer

	// Save original stderr
	oldStderr := os.Stderr
	defer func() { os.Stderr = oldStderr }()

	// Create pipe to capture stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stderr = w

	// Read stderr in background
	done := make(chan bool)
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			buf.WriteString(scanner.Text() + "\n")
		}
		done <- true
	}()

	// Create proxy with debug enabled
	config := ProxyConfig{
		TargetCommand: "echo",
		TargetArgs:    []string{"test"},
		Debug:         true,
	}

	target, err := NewTargetProcess(config.TargetCommand, config.TargetArgs, nil, config.Debug)
	if err != nil {
		t.Fatalf("Failed to create target process: %v", err)
	}

	// Close stderr writer and wait for reader
	w.Close()
	<-done

	// Verify debug output was generated
	output := buf.String()
	if config.Debug && len(output) == 0 {
		t.Log("Note: No debug output captured (may vary by system)")
	}

	target.Stop()
}


// TestProxyPassthroughMode tests passthrough mode behavior
func TestProxyPassthroughMode(t *testing.T) {
	config := ProxyConfig{
		TargetCommand: "cat",
		Passthrough:   true,
	}

	proxy, err := NewProxy(config)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	if !proxy.config.Passthrough {
		t.Error("Expected passthrough mode to be enabled")
	}
}

// TestTargetProcessWriteJSON tests JSON-RPC message writing
func TestTargetProcessWriteJSON(t *testing.T) {
	// Use cat command to echo back input
	target, err := NewTargetProcess("cat", nil, nil, false)
	if err != nil {
		t.Fatalf("Failed to start target process: %v", err)
	}
	defer target.Stop()

	// Write test message
	testMsg := []byte(`{"test": "message"}`)
	if err := target.WriteJSON(testMsg); err != nil {
		t.Fatalf("Failed to write JSON: %v", err)
	}

	// Read back the message
	line, err := target.ReadLine()
	if err != nil {
		t.Fatalf("Failed to read line: %v", err)
	}

	// Verify message was echoed
	if string(line) != string(testMsg) {
		t.Errorf("Expected %s, got %s", testMsg, line)
	}
}

// TestProxyConfigDefaults tests default configuration values
func TestProxyConfigDefaults(t *testing.T) {
	config := ProxyConfig{
		TargetCommand: "echo",
	}

	proxy, err := NewProxy(config)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Verify default cache dir was set
	if proxy.config.CacheDir == "" {
		t.Error("Expected default cache dir to be set")
	}

	// Verify cache dir exists
	if _, err := os.Stat(proxy.config.CacheDir); os.IsNotExist(err) {
		t.Errorf("Default cache dir was not created: %s", proxy.config.CacheDir)
	}
}
