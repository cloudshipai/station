package sandbox

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestE2BSandbox_Create(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/sandboxes" {
			if r.Header.Get("X-API-KEY") != "test-api-key" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			var req map[string]interface{}
			json.NewDecoder(r.Body).Decode(&req)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"sandboxID":       "sb-test-123",
				"templateID":      req["templateID"],
				"domain":          "e2b.dev",
				"envdAccessToken": "test-access-token",
				"envdVersion":     "0.1.0",
				"startedAt":       time.Now().Format(time.RFC3339),
				"endAt":           time.Now().Add(5 * time.Minute).Format(time.RFC3339),
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	sb, err := NewE2BSandboxWithAPIURL(Config{
		Mode:    ModeE2B,
		Image:   "base",
		Timeout: 5 * time.Minute,
	}, E2BConfig{
		APIKey:     "test-api-key",
		TemplateID: "base",
	}, server.URL)
	if err != nil {
		t.Fatalf("NewE2BSandbox failed: %v", err)
	}

	ctx := context.Background()
	if err := sb.Create(ctx); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if sb.ID() != "sb-test-123" {
		t.Errorf("expected sandbox ID 'sb-test-123', got %q", sb.ID())
	}
}

func TestE2BSandbox_Exec(t *testing.T) {
	var executedCmd string

	envdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/commands" {
			var req map[string]interface{}
			json.NewDecoder(r.Body).Decode(&req)
			executedCmd = req["cmd"].(string)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"exitCode": 0,
				"stdout":   "hello world\n",
				"stderr":   "",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer envdServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/sandboxes" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"sandboxID":       "sb-test-123",
				"templateID":      "base",
				"domain":          "e2b.dev",
				"envdAccessToken": "test-token",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer apiServer.Close()

	sb, err := NewE2BSandboxWithAPIURL(Config{Mode: ModeE2B}, E2BConfig{
		APIKey: "test-key",
	}, apiServer.URL)
	if err != nil {
		t.Fatalf("NewE2BSandbox failed: %v", err)
	}

	sb.client.SetEnvdURL(envdServer.URL)
	sb.created = true
	sb.sandboxID = "sb-test-123"

	ctx := context.Background()
	result, err := sb.Exec(ctx, "echo", "hello", "world")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	if result.Stdout != "hello world\n" {
		t.Errorf("expected stdout 'hello world\\n', got %q", result.Stdout)
	}

	if !strings.Contains(executedCmd, "echo hello world") {
		t.Errorf("command not properly constructed: %q", executedCmd)
	}
}

func TestE2BSandbox_FileOperations(t *testing.T) {
	fileStore := make(map[string][]byte)

	envdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")

		switch {
		case r.Method == "GET" && r.URL.Path == "/files":
			if content, ok := fileStore[path]; ok {
				w.Write(content)
			} else {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("file not found"))
			}

		case r.Method == "POST" && r.URL.Path == "/files":
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			file, _, err := r.FormFile("file")
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer file.Close()
			content, _ := io.ReadAll(file)
			fileStore[path] = content
			w.WriteHeader(http.StatusCreated)

		case r.Method == "DELETE" && r.URL.Path == "/files":
			delete(fileStore, path)
			w.WriteHeader(http.StatusNoContent)

		case r.Method == "POST" && r.URL.Path == "/commands":
			var req map[string]interface{}
			json.NewDecoder(r.Body).Decode(&req)
			cmd := req["cmd"].(string)

			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(cmd, "test -e") {
				parts := strings.Split(cmd, " ")
				for i, p := range parts {
					if p == "-e" && i+1 < len(parts) {
						checkPath := strings.Split(parts[i+1], " ")[0]
						if _, ok := fileStore[checkPath]; ok {
							json.NewEncoder(w).Encode(map[string]interface{}{
								"exitCode": 0,
								"stdout":   "exists\n",
								"stderr":   "",
							})
							return
						}
					}
				}
				json.NewEncoder(w).Encode(map[string]interface{}{
					"exitCode": 1,
					"stdout":   "",
					"stderr":   "",
				})
				return
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"exitCode": 0,
				"stdout":   "",
				"stderr":   "",
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer envdServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer apiServer.Close()

	sb, _ := NewE2BSandboxWithAPIURL(Config{Mode: ModeE2B}, E2BConfig{APIKey: "test-key"}, apiServer.URL)
	sb.client.SetEnvdURL(envdServer.URL)
	sb.created = true
	sb.sandboxID = "sb-test-123"

	ctx := context.Background()

	content := []byte("test content")
	if err := sb.WriteFile(ctx, "/test.txt", content, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	readContent, err := sb.ReadFile(ctx, "/test.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(readContent) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", readContent, content)
	}

	exists, err := sb.FileExists(ctx, "/test.txt")
	if err != nil {
		t.Fatalf("FileExists failed: %v", err)
	}
	if !exists {
		t.Error("file should exist after write")
	}

	if err := sb.DeleteFile(ctx, "/test.txt"); err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	exists, err = sb.FileExists(ctx, "/test.txt")
	if err != nil {
		t.Fatalf("FileExists after delete failed: %v", err)
	}
	if exists {
		t.Error("file should not exist after delete")
	}
}

func TestE2BSandbox_Destroy(t *testing.T) {
	destroyed := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/sandboxes/") {
			destroyed = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	sb, _ := NewE2BSandboxWithAPIURL(Config{Mode: ModeE2B}, E2BConfig{APIKey: "test-key"}, server.URL)
	sb.created = true
	sb.sandboxID = "sb-test-123"
	sb.client.Connect("sb-test-123", "test-token", "e2b.dev")

	ctx := context.Background()
	if err := sb.Destroy(ctx); err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}

	if !destroyed {
		t.Error("sandbox should have been destroyed via API call")
	}

	if sb.created {
		t.Error("created flag should be false after destroy")
	}

	if sb.sandboxID != "" {
		t.Error("sandboxID should be empty after destroy")
	}
}

func TestE2BSandbox_RequiresAPIKey(t *testing.T) {
	os.Unsetenv("E2B_API_KEY")

	_, err := NewE2BSandbox(Config{Mode: ModeE2B}, E2BConfig{})
	if err == nil {
		t.Error("expected error when API key is missing")
	}

	if !strings.Contains(err.Error(), "E2B_API_KEY") {
		t.Errorf("error should mention E2B_API_KEY: %v", err)
	}
}

func TestE2BSandbox_Config(t *testing.T) {
	os.Setenv("E2B_API_KEY", "test-key")
	defer os.Unsetenv("E2B_API_KEY")

	cfg := Config{
		Mode:          ModeE2B,
		Image:         "python",
		Timeout:       10 * time.Minute,
		WorkspacePath: "/workspace",
	}

	sb, err := NewE2BSandbox(cfg, E2BConfig{})
	if err != nil {
		t.Fatalf("NewE2BSandbox failed: %v", err)
	}

	gotCfg := sb.Config()
	if gotCfg.Mode != ModeE2B {
		t.Errorf("expected mode %s, got %s", ModeE2B, gotCfg.Mode)
	}
	if gotCfg.Image != "python" {
		t.Errorf("expected image 'python', got %q", gotCfg.Image)
	}
	if gotCfg.WorkspacePath != "/workspace" {
		t.Errorf("expected workspace '/workspace', got %q", gotCfg.WorkspacePath)
	}
}

func TestFactory_CreateE2B(t *testing.T) {
	os.Setenv("E2B_API_KEY", "test-key")
	defer os.Unsetenv("E2B_API_KEY")

	factory := NewFactory(DefaultConfig())

	sb, err := factory.Create(Config{Mode: ModeE2B})
	if err != nil {
		t.Fatalf("Factory.Create failed: %v", err)
	}

	_, ok := sb.(*E2BSandbox)
	if !ok {
		t.Error("expected E2BSandbox type")
	}

	cfg := sb.Config()
	if cfg.Mode != ModeE2B {
		t.Errorf("expected mode %s, got %s", ModeE2B, cfg.Mode)
	}
}
