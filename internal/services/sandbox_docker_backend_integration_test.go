//go:build integration

package services

import (
	"context"
	"strings"
	"testing"

	"station/pkg/dotprompt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerBackend_Integration(t *testing.T) {
	backend, err := NewDockerBackend()
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	t.Run("CreateSession_Python", func(t *testing.T) {
		key := NewAgentSessionKey("test-python-1")
		session, err := backend.CreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)
		require.NotNil(t, session)
		require.NotEmpty(t, session.ContainerID)
		defer backend.DestroySession(ctx, session.ContainerID)

		result, err := backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "python --version",
			TimeoutSeconds: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, result.Stdout+result.Stderr, "Python")
	})

	t.Run("CreateSession_Node", func(t *testing.T) {
		key := NewAgentSessionKey("test-node-1")
		session, err := backend.CreateSession(ctx, key, SessionConfig{
			Runtime: "node",
		})
		require.NoError(t, err)
		defer backend.DestroySession(ctx, session.ContainerID)

		result, err := backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "node --version",
			TimeoutSeconds: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, result.Stdout, "v")
	})

	t.Run("Exec_PythonCode", func(t *testing.T) {
		key := NewAgentSessionKey("test-exec-1")
		session, err := backend.CreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)
		defer backend.DestroySession(ctx, session.ContainerID)

		result, err := backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "python -c \"print(2 + 2)\"",
			TimeoutSeconds: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, strings.TrimSpace(result.Stdout), "4")
	})

	t.Run("Exec_Timeout", func(t *testing.T) {
		key := NewAgentSessionKey("test-timeout-1")
		session, err := backend.CreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)
		defer backend.DestroySession(ctx, session.ContainerID)

		result, err := backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "sleep 10",
			TimeoutSeconds: 1,
		})
		require.NoError(t, err)
		assert.True(t, result.TimedOut)
	})

	t.Run("WriteFile_ReadFile", func(t *testing.T) {
		key := NewAgentSessionKey("test-file-1")
		session, err := backend.CreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)
		defer backend.DestroySession(ctx, session.ContainerID)

		content := []byte("print('hello from file')")
		err = backend.WriteFile(ctx, session.ContainerID, "test.py", content)
		require.NoError(t, err)

		readContent, err := backend.ReadFile(ctx, session.ContainerID, "test.py")
		require.NoError(t, err)
		assert.Equal(t, content, readContent)

		result, err := backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "python test.py",
			TimeoutSeconds: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, result.Stdout, "hello from file")
	})

	t.Run("WriteFile_NestedDirectory", func(t *testing.T) {
		key := NewAgentSessionKey("test-nested-1")
		session, err := backend.CreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)
		defer backend.DestroySession(ctx, session.ContainerID)

		content := []byte("nested content")
		err = backend.WriteFile(ctx, session.ContainerID, "src/lib/utils.txt", content)
		require.NoError(t, err)

		readContent, err := backend.ReadFile(ctx, session.ContainerID, "src/lib/utils.txt")
		require.NoError(t, err)
		assert.Equal(t, content, readContent)
	})

	t.Run("ListFiles", func(t *testing.T) {
		key := NewAgentSessionKey("test-list-1")
		session, err := backend.CreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)
		defer backend.DestroySession(ctx, session.ContainerID)

		backend.WriteFile(ctx, session.ContainerID, "file1.py", []byte("# file1"))
		backend.WriteFile(ctx, session.ContainerID, "file2.py", []byte("# file2"))
		backend.WriteFile(ctx, session.ContainerID, "subdir/file3.py", []byte("# file3"))

		entries, err := backend.ListFiles(ctx, session.ContainerID, ".", false)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(entries), 2)

		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name
		}
		assert.Contains(t, names, "file1.py")
		assert.Contains(t, names, "file2.py")
	})

	t.Run("ListFiles_Recursive", func(t *testing.T) {
		key := NewAgentSessionKey("test-list-recursive-1")
		session, err := backend.CreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)
		defer backend.DestroySession(ctx, session.ContainerID)

		backend.WriteFile(ctx, session.ContainerID, "main.py", []byte("# main"))
		backend.WriteFile(ctx, session.ContainerID, "lib/helper.py", []byte("# helper"))

		entries, err := backend.ListFiles(ctx, session.ContainerID, ".", true)
		require.NoError(t, err)

		var foundHelper bool
		for _, e := range entries {
			if strings.Contains(e.Name, "helper") || strings.Contains(e.Path, "helper") {
				foundHelper = true
				break
			}
		}
		assert.True(t, foundHelper, "Should find helper.py in recursive listing")
	})

	t.Run("DeleteFile", func(t *testing.T) {
		key := NewAgentSessionKey("test-delete-1")
		session, err := backend.CreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)
		defer backend.DestroySession(ctx, session.ContainerID)

		backend.WriteFile(ctx, session.ContainerID, "todelete.txt", []byte("delete me"))

		err = backend.DeleteFile(ctx, session.ContainerID, "todelete.txt", false)
		require.NoError(t, err)

		_, err = backend.ReadFile(ctx, session.ContainerID, "todelete.txt")
		assert.Error(t, err)
	})

	t.Run("DeleteFile_Recursive", func(t *testing.T) {
		key := NewAgentSessionKey("test-delete-recursive-1")
		session, err := backend.CreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)
		defer backend.DestroySession(ctx, session.ContainerID)

		backend.WriteFile(ctx, session.ContainerID, "mydir/file1.txt", []byte("1"))
		backend.WriteFile(ctx, session.ContainerID, "mydir/file2.txt", []byte("2"))

		err = backend.DeleteFile(ctx, session.ContainerID, "mydir", true)
		require.NoError(t, err)

		_, err = backend.ReadFile(ctx, session.ContainerID, "mydir/file1.txt")
		assert.Error(t, err)
	})
}

func TestSessionManager_Integration(t *testing.T) {
	backend, err := NewDockerBackend()
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	defer backend.Close()

	manager := NewSessionManager(backend)
	ctx := context.Background()
	defer manager.Close(ctx)

	t.Run("GetOrCreateSession_CreatesContainer", func(t *testing.T) {
		key := NewAgentSessionKey("integration-agent-1")
		session, err := manager.GetOrCreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)
		require.NotNil(t, session)
		assert.NotEmpty(t, session.ID)
		assert.NotEmpty(t, session.ContainerID)
		assert.Equal(t, SessionStatusReady, session.Status)

		defer manager.DestroySession(ctx, key)

		result, err := backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "echo hello",
			TimeoutSeconds: 5,
		})
		require.NoError(t, err)
		assert.Contains(t, result.Stdout, "hello")
	})

	t.Run("GetOrCreateSession_ReusesExisting", func(t *testing.T) {
		key := NewAgentSessionKey("integration-agent-2")
		session1, err := manager.GetOrCreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)
		defer manager.DestroySession(ctx, key)

		session2, err := manager.GetOrCreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)

		assert.Equal(t, session1.ID, session2.ID)
		assert.Equal(t, session1.ContainerID, session2.ContainerID)
	})

	t.Run("FilePersistence_AcrossCalls", func(t *testing.T) {
		key := NewAgentSessionKey("integration-agent-3")
		session, err := manager.GetOrCreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)
		defer manager.DestroySession(ctx, key)

		err = backend.WriteFile(ctx, session.ContainerID, "persistent.txt", []byte("persisted data"))
		require.NoError(t, err)

		session2, err := manager.GetOrCreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)

		content, err := backend.ReadFile(ctx, session2.ContainerID, "persistent.txt")
		require.NoError(t, err)
		assert.Equal(t, "persisted data", string(content))
	})

	t.Run("DestroySession_CleansUp", func(t *testing.T) {
		key := NewAgentSessionKey("integration-agent-4")
		session, err := manager.GetOrCreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)
		containerID := session.ContainerID

		err = manager.DestroySession(ctx, key)
		require.NoError(t, err)

		_, exists := manager.GetSession(key)
		assert.False(t, exists)

		_, err = backend.Exec(ctx, containerID, ExecRequest{
			Command:        "echo test",
			TimeoutSeconds: 5,
		})
		assert.Error(t, err)
	})

	t.Run("CleanupWorkflow_DestroysAllWorkflowSessions", func(t *testing.T) {
		workflowID := "workflow-cleanup-integration"
		key1 := NewWorkflowSessionKey(workflowID)

		session1, err := manager.GetOrCreateSession(ctx, key1, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)
		container1 := session1.ContainerID

		err = manager.CleanupWorkflow(ctx, workflowID)
		require.NoError(t, err)

		_, exists := manager.GetSession(key1)
		assert.False(t, exists)

		_, err = backend.Exec(ctx, container1, ExecRequest{
			Command:        "echo test",
			TimeoutSeconds: 5,
		})
		assert.Error(t, err)
	})
}

func TestCodeModeTools_Integration(t *testing.T) {
	backend, err := NewDockerBackend()
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	defer backend.Close()

	manager := NewSessionManager(backend)
	ctx := context.Background()
	defer manager.Close(ctx)

	t.Run("FullWorkflow_CreateExecuteCleanup", func(t *testing.T) {
		key := NewAgentSessionKey("full-workflow-agent")

		session, err := manager.GetOrCreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)

		pythonCode := `
def fibonacci(n):
    if n <= 1:
        return n
    return fibonacci(n-1) + fibonacci(n-2)

for i in range(10):
    print(f"fib({i}) = {fibonacci(i)}")
`
		err = backend.WriteFile(ctx, session.ContainerID, "fib.py", []byte(pythonCode))
		require.NoError(t, err)

		result, err := backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "python fib.py",
			TimeoutSeconds: 30,
		})
		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, result.Stdout, "fib(9) = 34")

		err = manager.DestroySession(ctx, key)
		require.NoError(t, err)
	})

	t.Run("Linux_FullEnvironment", func(t *testing.T) {
		key := NewAgentSessionKey("linux-full-env")

		session, err := manager.GetOrCreateSession(ctx, key, SessionConfig{
			Runtime: "linux",
		})
		require.NoError(t, err)
		defer manager.DestroySession(ctx, key)

		result, err := backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "cat /etc/os-release | grep -E '^(NAME|VERSION)='",
			TimeoutSeconds: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, result.Stdout, "Ubuntu")
	})

	t.Run("Linux_InstallPackage", func(t *testing.T) {
		key := NewAgentSessionKey("linux-apt-install")

		session, err := manager.GetOrCreateSession(ctx, key, SessionConfig{
			Runtime: "linux",
		})
		require.NoError(t, err)
		defer manager.DestroySession(ctx, key)

		result, err := backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "apt-get update -qq && apt-get install -y -qq curl > /dev/null 2>&1 && curl --version | head -1",
			TimeoutSeconds: 120,
		})
		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, result.Stdout, "curl")
	})

	t.Run("Linux_BashScript", func(t *testing.T) {
		key := NewAgentSessionKey("linux-bash-script")

		session, err := manager.GetOrCreateSession(ctx, key, SessionConfig{
			Runtime: "bash",
		})
		require.NoError(t, err)
		defer manager.DestroySession(ctx, key)

		script := `#!/bin/bash
for i in 1 2 3; do
    echo "Count: $i"
done
echo "Total: 3"
`
		err = backend.WriteFile(ctx, session.ContainerID, "script.sh", []byte(script))
		require.NoError(t, err)

		result, err := backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "chmod +x script.sh && ./script.sh",
			TimeoutSeconds: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, result.Stdout, "Count: 1")
		assert.Contains(t, result.Stdout, "Total: 3")
	})

	t.Run("Linux_PipeCommands", func(t *testing.T) {
		key := NewAgentSessionKey("linux-pipe-cmds")

		session, err := manager.GetOrCreateSession(ctx, key, SessionConfig{
			Runtime: "linux",
		})
		require.NoError(t, err)
		defer manager.DestroySession(ctx, key)

		result, err := backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "echo -e 'apple\\nbanana\\ncherry\\napricot' | grep '^a' | wc -l",
			TimeoutSeconds: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, strings.TrimSpace(result.Stdout), "2")
	})

	t.Run("Linux_CompileAndRun_C", func(t *testing.T) {
		key := NewAgentSessionKey("linux-compile-c")

		session, err := manager.GetOrCreateSession(ctx, key, SessionConfig{
			Runtime: "linux",
		})
		require.NoError(t, err)
		defer manager.DestroySession(ctx, key)

		installResult, err := backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "apt-get update -qq && apt-get install -y -qq gcc > /dev/null 2>&1 && gcc --version | head -1",
			TimeoutSeconds: 120,
		})
		require.NoError(t, err)
		if installResult.ExitCode != 0 {
			t.Skip("gcc installation failed, skipping")
		}

		cCode := `#include <stdio.h>
int main() {
    printf("Hello from C!\\n");
    return 0;
}
`
		err = backend.WriteFile(ctx, session.ContainerID, "hello.c", []byte(cCode))
		require.NoError(t, err)

		result, err := backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "gcc -o hello hello.c && ./hello",
			TimeoutSeconds: 30,
		})
		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, result.Stdout, "Hello from C!")
	})

	t.Run("NodeJS_SimpleScript", func(t *testing.T) {
		key := NewAgentSessionKey("node-script-test")

		session, err := manager.GetOrCreateSession(ctx, key, SessionConfig{
			Runtime: "node",
		})
		require.NoError(t, err)
		defer manager.DestroySession(ctx, key)

		indexJS := `console.log("Hello from Node.js!");`
		err = backend.WriteFile(ctx, session.ContainerID, "index.js", []byte(indexJS))
		require.NoError(t, err)

		result, err := backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "node index.js",
			TimeoutSeconds: 30,
		})
		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, result.Stdout, "Hello from Node.js!")
	})

	t.Run("MultiFile_Project", func(t *testing.T) {
		key := NewAgentSessionKey("multifile-test")

		session, err := manager.GetOrCreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)
		defer manager.DestroySession(ctx, key)

		utilsCode := `
def add(a, b):
    return a + b

def multiply(a, b):
    return a * b
`
		err = backend.WriteFile(ctx, session.ContainerID, "utils.py", []byte(utilsCode))
		require.NoError(t, err)

		mainCode := `
from utils import add, multiply

result1 = add(5, 3)
result2 = multiply(4, 7)
print(f"5 + 3 = {result1}")
print(f"4 * 7 = {result2}")
`
		err = backend.WriteFile(ctx, session.ContainerID, "main.py", []byte(mainCode))
		require.NoError(t, err)

		result, err := backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "python main.py",
			TimeoutSeconds: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, result.Stdout, "5 + 3 = 8")
		assert.Contains(t, result.Stdout, "4 * 7 = 28")
	})

	t.Run("ErrorHandling_SyntaxError", func(t *testing.T) {
		key := NewAgentSessionKey("error-handling-test")

		session, err := manager.GetOrCreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)
		defer manager.DestroySession(ctx, key)

		badCode := `print("missing quote)`
		err = backend.WriteFile(ctx, session.ContainerID, "bad.py", []byte(badCode))
		require.NoError(t, err)

		result, err := backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "python bad.py",
			TimeoutSeconds: 10,
		})
		require.NoError(t, err)
		assert.NotEqual(t, 0, result.ExitCode)
		assert.Contains(t, result.Stderr, "SyntaxError")
	})

	t.Run("IterativeDevelopment_FixAndRerun", func(t *testing.T) {
		key := NewAgentSessionKey("iterative-dev-test")

		session, err := manager.GetOrCreateSession(ctx, key, SessionConfig{
			Runtime: "python",
		})
		require.NoError(t, err)
		defer manager.DestroySession(ctx, key)

		buggyCode := `print(undefined_variable)`
		err = backend.WriteFile(ctx, session.ContainerID, "app.py", []byte(buggyCode))
		require.NoError(t, err)

		result, err := backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "python app.py",
			TimeoutSeconds: 10,
		})
		require.NoError(t, err)
		assert.NotEqual(t, 0, result.ExitCode)
		assert.Contains(t, result.Stderr, "NameError")

		fixedCode := `message = "Hello, World!"\nprint(message)`
		err = backend.WriteFile(ctx, session.ContainerID, "app.py", []byte(fixedCode))
		require.NoError(t, err)

		result, err = backend.Exec(ctx, session.ContainerID, ExecRequest{
			Command:        "python app.py",
			TimeoutSeconds: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, result.Stdout, "Hello, World!")
	})
}

func TestUnifiedSandboxFactory_Integration(t *testing.T) {
	backend, err := NewDockerBackend()
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	defer backend.Close()

	sessionManager := NewSessionManager(backend)
	ctx := context.Background()
	defer sessionManager.Close(ctx)

	computeService := NewSandboxService(DefaultSandboxConfig())

	factory := NewUnifiedSandboxFactory(computeService, sessionManager, backend, true)

	t.Run("CodeMode_ReturnsMultipleTools", func(t *testing.T) {
		cfg := &dotprompt.SandboxConfig{
			Mode:    "code",
			Runtime: "python",
		}
		execCtx := ExecutionContext{
			AgentRunID: "test-run-1",
			AgentName:  "test-agent",
		}

		tools := factory.GetSandboxTools(cfg, execCtx)
		assert.Len(t, tools, 7)

		toolNames := make([]string, len(tools))
		for i, tool := range tools {
			toolNames[i] = tool.Name()
		}
		assert.Contains(t, toolNames, "sandbox_open")
		assert.Contains(t, toolNames, "sandbox_exec")
		assert.Contains(t, toolNames, "sandbox_fs_write")
		assert.Contains(t, toolNames, "sandbox_fs_read")
		assert.Contains(t, toolNames, "sandbox_fs_list")
		assert.Contains(t, toolNames, "sandbox_fs_delete")
		assert.Contains(t, toolNames, "sandbox_close")
	})

	t.Run("ComputeMode_ReturnsSingleTool", func(t *testing.T) {
		cfg := &dotprompt.SandboxConfig{
			Mode:    "compute",
			Runtime: "python",
		}
		execCtx := ExecutionContext{
			AgentRunID: "test-run-2",
			AgentName:  "test-agent",
		}

		tools := factory.GetSandboxTools(cfg, execCtx)
		assert.Len(t, tools, 1)
		assert.Equal(t, "sandbox_run", tools[0].Name())
	})

	t.Run("IsCodeMode", func(t *testing.T) {
		codeCfg := &dotprompt.SandboxConfig{Mode: "code"}
		computeCfg := &dotprompt.SandboxConfig{Mode: "compute"}
		emptyCfg := &dotprompt.SandboxConfig{}

		assert.True(t, factory.IsCodeMode(codeCfg))
		assert.False(t, factory.IsCodeMode(computeCfg))
		assert.False(t, factory.IsCodeMode(emptyCfg))
	})
}
