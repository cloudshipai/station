package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func dockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

func TestDockerSandbox_Create(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}

	tmpDir := t.TempDir()
	cfg := Config{
		Mode:          ModeDocker,
		Image:         "alpine:latest",
		WorkspacePath: filepath.Join(tmpDir, "workspace"),
		Timeout:       5 * time.Minute,
	}

	sb, err := NewDockerSandbox(cfg)
	if err != nil {
		t.Fatalf("NewDockerSandbox failed: %v", err)
	}

	ctx := context.Background()
	if err := sb.Create(ctx); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer sb.Destroy(ctx)

	if !sb.created {
		t.Error("sandbox should be marked as created")
	}
}

func TestDockerSandbox_Exec(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}

	cfg := Config{
		Mode:    ModeDocker,
		Image:   "alpine:latest",
		Timeout: 5 * time.Minute,
	}

	sb, _ := NewDockerSandbox(cfg)
	ctx := context.Background()
	defer sb.Destroy(ctx)

	result, err := sb.Exec(ctx, "echo", "hello from docker")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	if result.Stdout != "hello from docker\n" {
		t.Errorf("expected stdout 'hello from docker\\n', got %q", result.Stdout)
	}
}

func TestDockerSandbox_CopyOut_AutoCreatesContainer(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}

	tmpDir := t.TempDir()
	cfg := Config{
		Mode:          ModeDocker,
		Image:         "alpine:latest",
		WorkspacePath: filepath.Join(tmpDir, "workspace"),
		Timeout:       5 * time.Minute,
	}

	sb, _ := NewDockerSandbox(cfg)
	ctx := context.Background()
	defer sb.Destroy(ctx)

	if err := sb.WriteFile(ctx, "test.txt", []byte("test content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	destFile := filepath.Join(tmpDir, "dest.txt")
	if err := sb.CopyOut(ctx, "test.txt", destFile); err != nil {
		t.Fatalf("CopyOut failed: %v", err)
	}

	content, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("failed to read dest file: %v", err)
	}
	if string(content) != "test content" {
		t.Errorf("content mismatch: got %q", content)
	}
}

func TestDockerSandbox_RegistryAuth_Config(t *testing.T) {
	cfg := Config{
		Mode:  ModeDocker,
		Image: "private-registry.example.com/myimage:latest",
		RegistryAuth: &RegistryAuthConfig{
			Username:      "testuser",
			Password:      "testpass",
			ServerAddress: "private-registry.example.com",
		},
	}

	sb, err := NewDockerSandbox(cfg)
	if err != nil {
		t.Fatalf("NewDockerSandbox failed: %v", err)
	}

	if sb.config.RegistryAuth == nil {
		t.Fatal("RegistryAuth should be set")
	}

	if sb.config.RegistryAuth.Username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", sb.config.RegistryAuth.Username)
	}

	if sb.config.RegistryAuth.ServerAddress != "private-registry.example.com" {
		t.Errorf("expected server 'private-registry.example.com', got %q", sb.config.RegistryAuth.ServerAddress)
	}
}

func TestDockerSandbox_RegistryAuth_DockerConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configContent := `{"auths":{"https://index.docker.io/v1/":{"auth":"dGVzdDp0ZXN0"}}}`
	os.WriteFile(configPath, []byte(configContent), 0600)

	cfg := Config{
		Mode:  ModeDocker,
		Image: "alpine:latest",
		RegistryAuth: &RegistryAuthConfig{
			DockerConfigPath: configPath,
		},
	}

	sb, err := NewDockerSandbox(cfg)
	if err != nil {
		t.Fatalf("NewDockerSandbox failed: %v", err)
	}

	if sb.config.RegistryAuth.DockerConfigPath != configPath {
		t.Errorf("expected DockerConfigPath %q, got %q", configPath, sb.config.RegistryAuth.DockerConfigPath)
	}
}

func TestDockerSandbox_RegistryAuth_IdentityToken(t *testing.T) {
	cfg := Config{
		Mode:  ModeDocker,
		Image: "123456789.dkr.ecr.us-east-1.amazonaws.com/myapp:latest",
		RegistryAuth: &RegistryAuthConfig{
			IdentityToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			ServerAddress: "123456789.dkr.ecr.us-east-1.amazonaws.com",
		},
	}

	sb, err := NewDockerSandbox(cfg)
	if err != nil {
		t.Fatalf("NewDockerSandbox failed: %v", err)
	}

	if sb.config.RegistryAuth.IdentityToken == "" {
		t.Error("IdentityToken should be set")
	}
}

func TestDockerSandbox_pullImage_NoAuth(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}

	cfg := Config{
		Mode:    ModeDocker,
		Image:   "alpine:latest",
		Timeout: 5 * time.Minute,
	}

	sb, _ := NewDockerSandbox(cfg)
	ctx := context.Background()

	err := sb.pullImage(ctx)
	if err != nil {
		t.Fatalf("pullImage failed: %v", err)
	}
}

func TestDockerSandbox_Destroy(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}

	cfg := Config{
		Mode:    ModeDocker,
		Image:   "alpine:latest",
		Timeout: 5 * time.Minute,
	}

	sb, _ := NewDockerSandbox(cfg)
	ctx := context.Background()

	if err := sb.Create(ctx); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := sb.Destroy(ctx); err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}

	if sb.created {
		t.Error("sandbox should be marked as not created after destroy")
	}
}

func TestDockerSandbox_FileOperations(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}

	tmpDir := t.TempDir()
	cfg := Config{
		Mode:          ModeDocker,
		Image:         "alpine:latest",
		WorkspacePath: filepath.Join(tmpDir, "workspace"),
		Timeout:       5 * time.Minute,
	}

	sb, _ := NewDockerSandbox(cfg)
	ctx := context.Background()
	defer sb.Destroy(ctx)

	content := []byte("docker file test")
	if err := sb.WriteFile(ctx, "test.txt", content, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	exists, err := sb.FileExists(ctx, "test.txt")
	if err != nil {
		t.Fatalf("FileExists failed: %v", err)
	}
	if !exists {
		t.Error("file should exist after write")
	}

	readContent, err := sb.ReadFile(ctx, "test.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(readContent) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", readContent, content)
	}

	if err := sb.DeleteFile(ctx, "test.txt"); err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	exists, _ = sb.FileExists(ctx, "test.txt")
	if exists {
		t.Error("file should not exist after delete")
	}
}
