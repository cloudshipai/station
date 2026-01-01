package coding

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"station/internal/config"
)

func TestOpenCode_Level1_SingleFileCreation(t *testing.T) {
	if os.Getenv("OPENCODE_E2E") != "true" {
		t.Skip("Set OPENCODE_E2E=true to run")
	}

	backend := newTestBackend()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	workspaceDir := t.TempDir()
	initTestRepo(t, workspaceDir)

	session, err := backend.CreateSession(ctx, SessionOptions{
		WorkspacePath: workspaceDir,
		Title:         "Level1 Test",
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	defer backend.CloseSession(ctx, session.ID)

	result, err := backend.Execute(ctx, session.ID, Task{
		Instruction: "Create calculator.py with add, subtract, multiply, divide functions",
		Timeout:     2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Success {
		t.Fatalf("Task failed: %s", result.Error)
	}

	assertFileExists(t, workspaceDir, "calculator.py")
	t.Logf("Level1 PASSED: %v, tools=%d", result.Trace.Duration, len(result.Trace.ToolCalls))
}

func TestOpenCode_Level2_MultiFileProject(t *testing.T) {
	if os.Getenv("OPENCODE_E2E") != "true" {
		t.Skip("Set OPENCODE_E2E=true to run")
	}

	backend := newTestBackend()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	workspaceDir := t.TempDir()
	initTestRepo(t, workspaceDir)

	session, err := backend.CreateSession(ctx, SessionOptions{
		WorkspacePath: workspaceDir,
		Title:         "Level2 Test",
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	defer backend.CloseSession(ctx, session.ID)

	result, err := backend.Execute(ctx, session.ID, Task{
		Instruction: `Create a Python CLI tool called "taskman" with these files:
1. taskman/main.py - CLI entry point with argparse (add, list, done commands)
2. taskman/tasks.py - Task class and TaskManager with JSON file persistence
3. taskman/__init__.py - empty
4. taskman/README.md - usage documentation`,
		Timeout: 3 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Success {
		t.Fatalf("Task failed: %s", result.Error)
	}

	assertFileExists(t, workspaceDir, "taskman/main.py")
	assertFileExists(t, workspaceDir, "taskman/tasks.py")
	assertFileExists(t, workspaceDir, "taskman/__init__.py")
	t.Logf("Level2 PASSED: %v, tools=%d", result.Trace.Duration, len(result.Trace.ToolCalls))
}

func TestOpenCode_Level3_ModifyExistingCode(t *testing.T) {
	if os.Getenv("OPENCODE_E2E") != "true" {
		t.Skip("Set OPENCODE_E2E=true to run")
	}

	backend := newTestBackend()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	workspaceDir := t.TempDir()
	initTestRepo(t, workspaceDir)

	calcContent := `def add(a: float, b: float) -> float:
    return a + b

def subtract(a: float, b: float) -> float:
    return a - b

def multiply(a: float, b: float) -> float:
    return a * b

def divide(a: float, b: float) -> float:
    if b == 0:
        raise ValueError("Cannot divide by zero")
    return a / b
`
	os.WriteFile(filepath.Join(workspaceDir, "calculator.py"), []byte(calcContent), 0644)

	session, err := backend.CreateSession(ctx, SessionOptions{
		WorkspacePath: workspaceDir,
		Title:         "Level3 Test",
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	defer backend.CloseSession(ctx, session.ID)

	result, err := backend.Execute(ctx, session.ID, Task{
		Instruction: "Read calculator.py and add a power(base, exponent) function that follows the existing style",
		Timeout:     2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Success {
		t.Fatalf("Task failed: %s", result.Error)
	}

	content, _ := os.ReadFile(filepath.Join(workspaceDir, "calculator.py"))
	if !containsString(string(content), "power") {
		t.Error("power function not added to calculator.py")
	}
	t.Logf("Level3 PASSED: %v, tools=%d", result.Trace.Duration, len(result.Trace.ToolCalls))
}

func TestOpenCode_DirectoryAutoCreation(t *testing.T) {
	if os.Getenv("OPENCODE_E2E") != "true" {
		t.Skip("Set OPENCODE_E2E=true to run")
	}

	backend := newTestBackend()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	nonExistentDir := fmt.Sprintf("/tmp/opencode-autocreate-%d", time.Now().UnixNano())
	defer os.RemoveAll(nonExistentDir)

	session, err := backend.CreateSession(ctx, SessionOptions{
		WorkspacePath: nonExistentDir,
		Title:         "AutoCreate Dir Test",
	})
	if err != nil {
		t.Fatalf("CreateSession should succeed (directory param now optional): %v", err)
	}
	defer backend.CloseSession(ctx, session.ID)

	result, execErr := backend.Execute(ctx, session.ID, Task{
		Instruction: "Create test.txt with 'hello world'",
		Timeout:     1 * time.Minute,
	})

	if execErr != nil {
		t.Fatalf("Execute should succeed - OpenCode creates dirs as needed: %v", execErr)
	}
	if !result.Success {
		t.Fatalf("Task should succeed: %s", result.Error)
	}

	testFile := nonExistentDir + "/test.txt"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Errorf("File should exist: %s", testFile)
	} else {
		t.Logf("AutoCreate PASSED: directory and file created successfully")
	}
}

func TestOpenCode_SequentialTasks(t *testing.T) {
	if os.Getenv("OPENCODE_E2E") != "true" {
		t.Skip("Set OPENCODE_E2E=true to run")
	}

	backend := newTestBackend()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	workspaceDir := t.TempDir()
	initTestRepo(t, workspaceDir)

	session, err := backend.CreateSession(ctx, SessionOptions{
		WorkspacePath: workspaceDir,
		Title:         "Sequential Test",
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	defer backend.CloseSession(ctx, session.ID)

	tasks := []struct {
		instruction  string
		expectedFile string
	}{
		{"Create main.py with FastAPI app and one GET / endpoint", "main.py"},
		{"Create models.py with a Pydantic Todo model", "models.py"},
		{"Create requirements.txt with fastapi and uvicorn", "requirements.txt"},
	}

	for i, task := range tasks {
		start := time.Now()
		result, err := backend.Execute(ctx, session.ID, Task{
			Instruction: task.instruction,
			Timeout:     2 * time.Minute,
		})
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Task %d failed: %v", i, err)
		}
		if !result.Success {
			t.Fatalf("Task %d unsuccessful: %s", i, result.Error)
		}

		assertFileExists(t, workspaceDir, task.expectedFile)
		t.Logf("Task %d: %s created in %v", i, task.expectedFile, duration)
	}
}

func TestOpenCode_ConcurrentSessions(t *testing.T) {
	if os.Getenv("OPENCODE_E2E") != "true" {
		t.Skip("Set OPENCODE_E2E=true to run")
	}

	backend := newTestBackend()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := backend.Ping(ctx); err != nil {
		t.Fatalf("OpenCode unhealthy: %v", err)
	}

	const numSessions = 3
	var wg sync.WaitGroup
	var successes, failures int32

	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(sessionNum int) {
			defer wg.Done()

			workspaceDir, _ := os.MkdirTemp("", fmt.Sprintf("concurrent-%d-*", sessionNum))
			defer os.RemoveAll(workspaceDir)
			initRepoInDir(workspaceDir)

			session, err := backend.CreateSession(ctx, SessionOptions{
				WorkspacePath: workspaceDir,
				Title:         fmt.Sprintf("Concurrent %d", sessionNum),
			})
			if err != nil {
				t.Logf("Session %d: create failed: %v", sessionNum, err)
				atomic.AddInt32(&failures, 1)
				return
			}
			defer backend.CloseSession(ctx, session.ID)

			result, err := backend.Execute(ctx, session.ID, Task{
				Instruction: fmt.Sprintf("Create file%d.py with a hello() function", sessionNum),
				Timeout:     2 * time.Minute,
			})

			if err != nil || !result.Success {
				t.Logf("Session %d: execute failed: err=%v success=%v", sessionNum, err, result != nil && result.Success)
				atomic.AddInt32(&failures, 1)
				return
			}

			if !fileExists(filepath.Join(workspaceDir, fmt.Sprintf("file%d.py", sessionNum))) {
				t.Logf("Session %d: file not created", sessionNum)
				atomic.AddInt32(&failures, 1)
				return
			}

			atomic.AddInt32(&successes, 1)
			t.Logf("Session %d: OK", sessionNum)
		}(i)

		time.Sleep(500 * time.Millisecond)
	}

	wg.Wait()
	t.Logf("Concurrent sessions: %d/%d succeeded", successes, numSessions)

	if failures > 0 {
		t.Errorf("%d sessions failed", failures)
	}
}

func TestOpenCode_RecoveryAfterCrash(t *testing.T) {
	if os.Getenv("OPENCODE_E2E") != "true" {
		t.Skip("Set OPENCODE_E2E=true to run")
	}

	backend := newTestBackend()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	workspaceDir := t.TempDir()
	initTestRepo(t, workspaceDir)

	session, err := backend.CreateSession(ctx, SessionOptions{
		WorkspacePath: workspaceDir,
		Title:         "Recovery Test",
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	defer backend.CloseSession(ctx, session.ID)

	result1, err := backend.Execute(ctx, session.ID, Task{
		Instruction: "Create test1.txt with 'first'",
		Timeout:     2 * time.Minute,
	})
	if err != nil || !result1.Success {
		t.Fatalf("First task failed: %v", err)
	}
	assertFileExists(t, workspaceDir, "test1.txt")
	t.Log("First task succeeded")

	result2, _ := backend.Execute(ctx, session.ID, Task{
		Instruction: "Try to read /nonexistent/path/file.txt",
		Timeout:     1 * time.Minute,
	})
	t.Logf("Problematic task: success=%v", result2 != nil && result2.Success)

	result3, err := backend.Execute(ctx, session.ID, Task{
		Instruction: "Create test2.txt with 'recovered'",
		Timeout:     2 * time.Minute,
	})
	if err != nil || !result3.Success {
		t.Fatalf("Recovery task failed: %v", err)
	}
	assertFileExists(t, workspaceDir, "test2.txt")
	t.Log("Recovery task succeeded - OpenCode recovered from error")
}

func newTestBackend() *OpenCodeBackend {
	url := os.Getenv("OPENCODE_URL")
	if url == "" {
		url = "http://localhost:4096"
	}

	return NewOpenCodeBackend(config.CodingConfig{
		Backend: "opencode",
		OpenCode: config.CodingOpenCodeConfig{
			URL: url,
		},
		MaxAttempts:    3,
		TaskTimeoutMin: 5,
	})
}

func initTestRepo(t *testing.T, dir string) {
	t.Helper()
	initRepoInDir(dir)
}

func initRepoInDir(dir string) {
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = dir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = dir
	cmd.Run()

	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644)

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = dir
	cmd.Run()
}

func assertFileExists(t *testing.T, dir, filename string) {
	t.Helper()
	path := filepath.Join(dir, filename)
	if _, err := os.Stat(path); err != nil {
		t.Errorf("File %s not found: %v", filename, err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func containsString(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 &&
		(haystack == needle || len(haystack) > len(needle) &&
			(haystack[:len(needle)] == needle ||
				haystack[len(haystack)-len(needle):] == needle ||
				findSubstring(haystack, needle)))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
