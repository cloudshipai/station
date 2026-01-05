package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type BrowserConfigService struct {
	port        int
	apiBaseURL  string
	httpClient  *http.Client
	serverCmd   *exec.Cmd
	startedByUs bool
}

type BrowserConfigStatus struct {
	ID         string                 `json:"id"`
	Status     string                 `json:"status"`
	ConfigPath string                 `json:"config_path"`
	Values     map[string]interface{} `json:"values,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

func NewBrowserConfigService(port int) *BrowserConfigService {
	return &BrowserConfigService{
		port:       port,
		apiBaseURL: fmt.Sprintf("http://localhost:%d", port),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (s *BrowserConfigService) EditWithBrowser(ctx context.Context, configPath string) error {
	serverRunning := s.isServerRunning()

	if !serverRunning {
		fmt.Printf("🚀 Starting Station server on port %d...\n", s.port)
		if err := s.startServer(); err != nil {
			return fmt.Errorf("failed to start server: %w", err)
		}
		s.startedByUs = true
		defer s.stopServer()
	}

	sessionID, err := s.startConfigSession(configPath)
	if err != nil {
		return fmt.Errorf("failed to start config session: %w", err)
	}

	fmt.Printf("\n🌐 Opening browser for configuration editing...\n")

	configURL := fmt.Sprintf("%s/config/edit?session_id=%s", s.apiBaseURL, sessionID)
	if err := openBrowser(configURL); err != nil {
		fmt.Printf("⚠️  Could not open browser automatically.\n")
	}
	fmt.Printf("\nIf browser didn't open, visit:\n  %s\n\n", configURL)

	fmt.Printf("⏳ Waiting for configuration save in browser...\n")
	fmt.Printf("   (Press Ctrl+C to cancel)\n\n")

	return s.pollForCompletion(ctx, sessionID, 10*time.Minute)
}

func (s *BrowserConfigService) startServer() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not find executable: %w", err)
	}

	s.serverCmd = exec.Command(execPath, "serve")
	s.serverCmd.Stdout = nil
	s.serverCmd.Stderr = nil

	if err := s.serverCmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		if s.isServerRunning() {
			return nil
		}
	}

	s.serverCmd.Process.Kill()
	return fmt.Errorf("server failed to start within 15 seconds")
}

func (s *BrowserConfigService) stopServer() {
	if s.serverCmd != nil && s.serverCmd.Process != nil && s.startedByUs {
		fmt.Printf("\n🛑 Stopping Station server...\n")
		s.serverCmd.Process.Kill()
		s.serverCmd.Wait()
	}
}

func (s *BrowserConfigService) isServerRunning() bool {
	resp, err := s.httpClient.Get(s.apiBaseURL + "/api/v1/agents")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (s *BrowserConfigService) startConfigSession(configPath string) (string, error) {
	reqBody := fmt.Sprintf(`{"config_path":"%s"}`, configPath)
	resp, err := s.httpClient.Post(
		s.apiBaseURL+"/api/v1/config/session",
		"application/json",
		strings.NewReader(reqBody),
	)
	if err != nil {
		return "", fmt.Errorf("failed to connect to Station server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to start config session: server returned %d", resp.StatusCode)
	}

	var result struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return result.SessionID, nil
}

func (s *BrowserConfigService) pollForCompletion(ctx context.Context, sessionID string, timeout time.Duration) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeoutCh := time.After(timeout)
	lastStatus := ""

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeoutCh:
			return fmt.Errorf("timeout waiting for config save - no input received in browser")
		case <-ticker.C:
			status, err := s.getSessionStatus(sessionID)
			if err != nil {
				if strings.Contains(err.Error(), "404") {
					fmt.Printf("✅ Configuration saved\n")
					return nil
				}
				continue
			}

			if status.Status != lastStatus {
				switch status.Status {
				case "waiting":
					fmt.Printf("📝 Waiting for configuration input in browser...\n")
				case "saving":
					fmt.Printf("💾 Saving configuration...\n")
				}
				lastStatus = status.Status
			}

			switch status.Status {
			case "completed":
				fmt.Printf("\n✅ Configuration saved successfully\n")
				fmt.Printf("   Config file: %s\n", status.ConfigPath)
				return nil
			case "failed":
				return fmt.Errorf("config save failed: %s", status.Error)
			case "cancelled":
				return fmt.Errorf("configuration cancelled by user")
			}
		}
	}
}

func (s *BrowserConfigService) getSessionStatus(sessionID string) (*BrowserConfigStatus, error) {
	resp, err := s.httpClient.Get(s.apiBaseURL + "/api/v1/config/session/" + sessionID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("404: session not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var status BrowserConfigStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return exec.Command(cmd, args...).Start()
}
