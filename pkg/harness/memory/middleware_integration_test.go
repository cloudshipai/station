package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryFlush_Integration(t *testing.T) {
	// Create a temp workspace
	tmpDir, err := os.MkdirTemp("", "memory-flush-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create middleware with workspace
	backend := &FSBackend{}
	sources := []string{filepath.Join(tmpDir, "MEMORY.md")}
	mw := NewMemoryMiddlewareWithWorkspace(backend, sources, tmpDir)

	// Initialize workspace memory
	if err := mw.InitializeWorkspaceMemory(); err != nil {
		t.Fatalf("InitializeWorkspaceMemory failed: %v", err)
	}

	// Verify MEMORY.md was created
	memoryFile := filepath.Join(tmpDir, "MEMORY.md")
	if _, err := os.Stat(memoryFile); os.IsNotExist(err) {
		t.Error("MEMORY.md was not created")
	}

	// Verify HEARTBEAT.md was created
	heartbeatFile := filepath.Join(tmpDir, "HEARTBEAT.md")
	if _, err := os.Stat(heartbeatFile); os.IsNotExist(err) {
		t.Error("HEARTBEAT.md was not created")
	}

	// Test memory flush
	summary := "### Test Session\n- Completed 5 steps\n- User requested help with debugging"
	if err := mw.FlushSessionMemory(summary); err != nil {
		t.Fatalf("FlushSessionMemory failed: %v", err)
	}

	// Verify daily log was created with content
	dailyPath := filepath.Join(tmpDir, mw.DailyLogPath())
	content, err := os.ReadFile(dailyPath)
	if err != nil {
		t.Fatalf("Failed to read daily log: %v", err)
	}

	if !strings.Contains(string(content), "Test Session") {
		t.Errorf("Daily log doesn't contain expected content. Got: %s", string(content))
	}

	if !strings.Contains(string(content), "Completed 5 steps") {
		t.Errorf("Daily log doesn't contain step count. Got: %s", string(content))
	}

	// Test multiple flushes append
	summary2 := "### Another Session\n- Did some analysis"
	if err := mw.FlushSessionMemory(summary2); err != nil {
		t.Fatalf("Second FlushSessionMemory failed: %v", err)
	}

	content, err = os.ReadFile(dailyPath)
	if err != nil {
		t.Fatalf("Failed to read daily log after second flush: %v", err)
	}

	if !strings.Contains(string(content), "Test Session") {
		t.Error("First flush content was overwritten")
	}
	if !strings.Contains(string(content), "Another Session") {
		t.Error("Second flush content not found")
	}
}

func TestMemoryFlush_EmptySummary(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "memory-empty-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	backend := &FSBackend{}
	mw := NewMemoryMiddlewareWithWorkspace(backend, nil, tmpDir)

	// Empty summary should not create file
	if err := mw.FlushSessionMemory(""); err != nil {
		t.Errorf("FlushSessionMemory with empty summary returned error: %v", err)
	}

	dailyPath := filepath.Join(tmpDir, mw.DailyLogPath())
	if _, err := os.Stat(dailyPath); !os.IsNotExist(err) {
		t.Error("Daily log should not exist for empty summary")
	}
}

func TestMemoryFlush_NoWorkspace(t *testing.T) {
	backend := &FSBackend{}
	mw := NewMemoryMiddleware(backend, nil) // No workspace

	// Should return nil (no error) when no workspace
	if err := mw.FlushSessionMemory("some content"); err != nil {
		t.Errorf("FlushSessionMemory without workspace returned error: %v", err)
	}
}

func TestWorkspaceMemoryInitialization(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "workspace-init-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	backend := &FSBackend{}
	mw := NewMemoryMiddlewareWithWorkspace(backend, nil, tmpDir)

	// Initialize workspace
	if err := mw.InitializeWorkspaceMemory(); err != nil {
		t.Fatalf("InitializeWorkspaceMemory failed: %v", err)
	}

	// Verify memory directory was created
	memoryDir := filepath.Join(tmpDir, "memory")
	if info, err := os.Stat(memoryDir); err != nil || !info.IsDir() {
		t.Error("Memory directory was not created")
	}

	// Verify MEMORY.md content
	memoryContent, err := os.ReadFile(filepath.Join(tmpDir, "MEMORY.md"))
	if err != nil {
		t.Fatalf("Failed to read MEMORY.md: %v", err)
	}
	if !strings.Contains(string(memoryContent), "Agent Memory") {
		t.Error("MEMORY.md doesn't have expected header")
	}

	// Verify HEARTBEAT.md content
	heartbeatContent, err := os.ReadFile(filepath.Join(tmpDir, "HEARTBEAT.md"))
	if err != nil {
		t.Fatalf("Failed to read HEARTBEAT.md: %v", err)
	}
	if !strings.Contains(string(heartbeatContent), "HEARTBEAT_OK") {
		t.Error("HEARTBEAT.md doesn't mention HEARTBEAT_OK token")
	}
	if !strings.Contains(string(heartbeatContent), "Heartbeat Checklist") {
		t.Error("HEARTBEAT.md doesn't have expected header")
	}
}
