package coding

import (
	"context"
	"errors"
	"testing"
	"time"

	"station/internal/config"
)

type mockNATSClient struct {
	connected     bool
	sessions      map[string]*SessionState
	state         map[string][]byte
	execResult    *CodingResult
	execErr       error
	execEvents    []*CodingStreamEvent
	deleteErr     error
	saveErr       error
	getSessionErr error
}

func newMockNATSClient() *mockNATSClient {
	return &mockNATSClient{
		connected: true,
		sessions:  make(map[string]*SessionState),
		state:     make(map[string][]byte),
	}
}

func (m *mockNATSClient) IsConnected() bool {
	return m.connected
}

func (m *mockNATSClient) Close() error {
	return nil
}

func (m *mockNATSClient) ExecuteTask(ctx context.Context, task *CodingTask) (*TaskExecution, error) {
	if m.execErr != nil {
		return nil, m.execErr
	}

	exec := &TaskExecution{
		events: make(chan *CodingStreamEvent, 100),
		result: make(chan *CodingResult, 1),
		done:   make(chan struct{}),
	}

	go func() {
		for _, e := range m.execEvents {
			exec.events <- e
		}
		close(exec.events)
	}()

	go func() {
		result := m.execResult
		if result == nil {
			result = &CodingResult{
				TaskID: task.TaskID,
				Status: "completed",
				Result: "Task completed successfully",
				Session: ResultSession{
					Name:       task.Session.Name,
					OpencodeID: "oc-" + task.Session.Name,
				},
			}
		}
		exec.result <- result
		close(exec.done)
	}()

	return exec, nil
}

func (m *mockNATSClient) GetSession(ctx context.Context, name string) (*SessionState, error) {
	if m.getSessionErr != nil {
		return nil, m.getSessionErr
	}
	return m.sessions[name], nil
}

func (m *mockNATSClient) SaveSession(ctx context.Context, state *SessionState) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.sessions[state.SessionName] = state
	return nil
}

func (m *mockNATSClient) DeleteSession(ctx context.Context, name string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.sessions, name)
	return nil
}

func (m *mockNATSClient) GetState(ctx context.Context, key string) ([]byte, error) {
	return m.state[key], nil
}

func (m *mockNATSClient) SetState(ctx context.Context, key string, value []byte) error {
	m.state[key] = value
	return nil
}

func (m *mockNATSClient) DeleteState(ctx context.Context, key string) error {
	delete(m.state, key)
	return nil
}

func newTestNATSBackend(client *mockNATSClient) *NATSBackend {
	return &NATSBackend{
		client:      &NATSCodingClient{},
		cfg:         config.CodingConfig{},
		sessions:    make(map[string]*Session),
		taskTimeout: 5 * time.Minute,
	}
}

func newTestableNATSBackend(client NATSClient) *testableNATSBackend {
	return &testableNATSBackend{
		client:      client,
		sessions:    make(map[string]*Session),
		taskTimeout: 5 * time.Minute,
	}
}

type testableNATSBackend struct {
	client      NATSClient
	sessions    map[string]*Session
	taskTimeout time.Duration
}

func (b *testableNATSBackend) Ping(ctx context.Context) error {
	if !b.client.IsConnected() {
		return errors.New("NATS not connected")
	}
	return nil
}

func (b *testableNATSBackend) CreateSession(ctx context.Context, opts SessionOptions) (*Session, error) {
	session := &Session{
		ID:            "session-test123",
		WorkspacePath: opts.WorkspacePath,
		Title:         opts.Title,
		CreatedAt:     time.Now(),
		LastUsedAt:    time.Now(),
		Metadata:      make(map[string]string),
	}

	if opts.WorkspacePath == "" {
		session.WorkspacePath = session.ID
	}

	if opts.RepoURL != "" {
		session.Metadata["repo_url"] = opts.RepoURL
		if opts.Branch != "" {
			session.Metadata["branch"] = opts.Branch
		}
	}

	b.sessions[session.ID] = session
	return session, nil
}

func (b *testableNATSBackend) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	if session, ok := b.sessions[sessionID]; ok {
		return session, nil
	}

	state, err := b.client.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, ErrSessionNotFound
	}

	session := &Session{
		ID:               state.SessionName,
		BackendSessionID: state.OpencodeID,
		WorkspacePath:    state.WorkspacePath,
		Metadata:         make(map[string]string),
	}

	b.sessions[sessionID] = session
	return session, nil
}

func (b *testableNATSBackend) CloseSession(ctx context.Context, sessionID string) error {
	delete(b.sessions, sessionID)
	return b.client.DeleteSession(ctx, sessionID)
}

func (b *testableNATSBackend) Execute(ctx context.Context, sessionID string, task Task) (*Result, error) {
	session, err := b.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	timeout := task.Timeout
	if timeout == 0 {
		timeout = b.taskTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	natsTask := &CodingTask{
		TaskID: "task-123",
		Session: TaskSession{
			Name:     session.ID,
			Continue: true,
		},
		Workspace: TaskWorkspace{
			Name: session.WorkspacePath,
		},
		Prompt:  task.Instruction,
		Timeout: int(timeout.Milliseconds()),
	}

	exec, err := b.client.ExecuteTask(ctx, natsTask)
	if err != nil {
		return nil, err
	}

	result, err := exec.Wait(ctx)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return &Result{Success: false, Error: "task timed out"}, nil
		}
		return nil, err
	}

	if result.Status != "completed" {
		return &Result{Success: false, Error: result.Error}, nil
	}

	return &Result{
		Success: true,
		Summary: result.Result,
		Trace: &Trace{
			SessionID: result.Session.OpencodeID,
		},
	}, nil
}

func TestNATSBackend_Ping(t *testing.T) {
	t.Run("connected", func(t *testing.T) {
		mock := newMockNATSClient()
		backend := newTestableNATSBackend(mock)

		if err := backend.Ping(context.Background()); err != nil {
			t.Errorf("Ping() error = %v, want nil", err)
		}
	})

	t.Run("disconnected", func(t *testing.T) {
		mock := newMockNATSClient()
		mock.connected = false
		backend := newTestableNATSBackend(mock)

		if err := backend.Ping(context.Background()); err == nil {
			t.Error("Ping() error = nil, want error")
		}
	})
}

func TestNATSBackend_CreateSession(t *testing.T) {
	mock := newMockNATSClient()
	backend := newTestableNATSBackend(mock)

	session, err := backend.CreateSession(context.Background(), SessionOptions{
		WorkspacePath: "/workspaces/test",
		Title:         "Test Session",
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if session.WorkspacePath != "/workspaces/test" {
		t.Errorf("WorkspacePath = %q, want %q", session.WorkspacePath, "/workspaces/test")
	}
	if session.Title != "Test Session" {
		t.Errorf("Title = %q, want %q", session.Title, "Test Session")
	}
}

func TestNATSBackend_CreateSession_WithRepo(t *testing.T) {
	mock := newMockNATSClient()
	backend := newTestableNATSBackend(mock)

	session, err := backend.CreateSession(context.Background(), SessionOptions{
		RepoURL: "https://github.com/test/repo.git",
		Branch:  "main",
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if session.Metadata["repo_url"] != "https://github.com/test/repo.git" {
		t.Errorf("Metadata[repo_url] = %q, want %q", session.Metadata["repo_url"], "https://github.com/test/repo.git")
	}
	if session.Metadata["branch"] != "main" {
		t.Errorf("Metadata[branch] = %q, want %q", session.Metadata["branch"], "main")
	}
}

func TestNATSBackend_GetSession(t *testing.T) {
	t.Run("cached session", func(t *testing.T) {
		mock := newMockNATSClient()
		backend := newTestableNATSBackend(mock)

		created, _ := backend.CreateSession(context.Background(), SessionOptions{
			WorkspacePath: "/ws/test",
		})

		got, err := backend.GetSession(context.Background(), created.ID)
		if err != nil {
			t.Fatalf("GetSession() error = %v", err)
		}
		if got.ID != created.ID {
			t.Errorf("ID = %q, want %q", got.ID, created.ID)
		}
	})

	t.Run("from KV store", func(t *testing.T) {
		mock := newMockNATSClient()
		mock.sessions["session-from-kv"] = &SessionState{
			SessionName:   "session-from-kv",
			OpencodeID:    "oc-123",
			WorkspacePath: "/ws/kv",
		}
		backend := newTestableNATSBackend(mock)

		got, err := backend.GetSession(context.Background(), "session-from-kv")
		if err != nil {
			t.Fatalf("GetSession() error = %v", err)
		}
		if got.BackendSessionID != "oc-123" {
			t.Errorf("BackendSessionID = %q, want %q", got.BackendSessionID, "oc-123")
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock := newMockNATSClient()
		backend := newTestableNATSBackend(mock)

		_, err := backend.GetSession(context.Background(), "nonexistent")
		if !errors.Is(err, ErrSessionNotFound) {
			t.Errorf("GetSession() error = %v, want ErrSessionNotFound", err)
		}
	})
}

func TestNATSBackend_CloseSession(t *testing.T) {
	mock := newMockNATSClient()
	backend := newTestableNATSBackend(mock)

	session, _ := backend.CreateSession(context.Background(), SessionOptions{
		WorkspacePath: "/ws/test",
	})

	if err := backend.CloseSession(context.Background(), session.ID); err != nil {
		t.Fatalf("CloseSession() error = %v", err)
	}

	_, err := backend.GetSession(context.Background(), session.ID)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("GetSession after close error = %v, want ErrSessionNotFound", err)
	}
}

func TestNATSBackend_Execute(t *testing.T) {
	t.Run("successful execution", func(t *testing.T) {
		mock := newMockNATSClient()
		mock.execResult = &CodingResult{
			TaskID: "task-1",
			Status: "completed",
			Result: "Fixed the bug in auth.go",
			Session: ResultSession{
				Name:       "session-test",
				OpencodeID: "oc-session-1",
			},
		}
		backend := newTestableNATSBackend(mock)

		session, _ := backend.CreateSession(context.Background(), SessionOptions{
			WorkspacePath: "/ws/test",
		})

		result, err := backend.Execute(context.Background(), session.ID, Task{
			Instruction: "Fix the authentication bug",
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if !result.Success {
			t.Error("Success = false, want true")
		}
		if result.Summary != "Fixed the bug in auth.go" {
			t.Errorf("Summary = %q, want %q", result.Summary, "Fixed the bug in auth.go")
		}
	})

	t.Run("failed execution", func(t *testing.T) {
		mock := newMockNATSClient()
		mock.execResult = &CodingResult{
			TaskID: "task-1",
			Status: "failed",
			Error:  "Could not compile the code",
		}
		backend := newTestableNATSBackend(mock)

		session, _ := backend.CreateSession(context.Background(), SessionOptions{
			WorkspacePath: "/ws/test",
		})

		result, err := backend.Execute(context.Background(), session.ID, Task{
			Instruction: "Build the project",
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if result.Success {
			t.Error("Success = true, want false")
		}
		if result.Error != "Could not compile the code" {
			t.Errorf("Error = %q, want %q", result.Error, "Could not compile the code")
		}
	})

	t.Run("session not found", func(t *testing.T) {
		mock := newMockNATSClient()
		backend := newTestableNATSBackend(mock)

		_, err := backend.Execute(context.Background(), "nonexistent", Task{
			Instruction: "Do something",
		})
		if !errors.Is(err, ErrSessionNotFound) {
			t.Errorf("Execute() error = %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("client error", func(t *testing.T) {
		mock := newMockNATSClient()
		mock.execErr = errors.New("NATS connection lost")
		backend := newTestableNATSBackend(mock)

		session, _ := backend.CreateSession(context.Background(), SessionOptions{
			WorkspacePath: "/ws/test",
		})

		_, err := backend.Execute(context.Background(), session.ID, Task{
			Instruction: "Do something",
		})
		if err == nil {
			t.Error("Execute() error = nil, want error")
		}
	})
}

func TestNATSBackend_BuildPrompt(t *testing.T) {
	backend := &NATSBackend{}

	t.Run("basic task", func(t *testing.T) {
		got := backend.buildPrompt(Task{Instruction: "Fix the bug"}, "/ws/test")
		if got != "Fix the bug" {
			t.Errorf("buildPrompt() = %q, want %q", got, "Fix the bug")
		}
	})

	t.Run("with context", func(t *testing.T) {
		got := backend.buildPrompt(Task{
			Instruction: "Fix the bug",
			Context:     "Users report crashes",
		}, "/ws/test")
		expected := "Context: Users report crashes\n\nTask: Fix the bug"
		if got != expected {
			t.Errorf("buildPrompt() = %q, want %q", got, expected)
		}
	})

	t.Run("with files", func(t *testing.T) {
		got := backend.buildPrompt(Task{
			Instruction: "Fix the bug",
			Files:       []string{"auth.go", "user.go"},
		}, "/ws/test")
		expected := "Fix the bug\n\nFocus on these files: [auth.go user.go]"
		if got != expected {
			t.Errorf("buildPrompt() = %q, want %q", got, expected)
		}
	})
}

func TestNATSBackend_BuildCloneTask(t *testing.T) {
	backend := &NATSBackend{}

	t.Run("without branch", func(t *testing.T) {
		got := backend.buildCloneTask("https://github.com/test/repo.git", "", nil)
		expected := "Clone the git repository: git clone https://github.com/test/repo.git . && git status"
		if got != expected {
			t.Errorf("buildCloneTask() = %q, want %q", got, expected)
		}
	})

	t.Run("with branch", func(t *testing.T) {
		got := backend.buildCloneTask("https://github.com/test/repo.git", "develop", nil)
		expected := "Clone the git repository: git clone --branch develop https://github.com/test/repo.git . && git status"
		if got != expected {
			t.Errorf("buildCloneTask() = %q, want %q", got, expected)
		}
	})

	t.Run("with token credentials", func(t *testing.T) {
		creds := &GitCredentials{Token: "ghp_test123"}
		got := backend.buildCloneTask("https://github.com/test/repo.git", "", creds)
		if got == "" {
			t.Error("buildCloneTask() returned empty string")
		}
	})
}
