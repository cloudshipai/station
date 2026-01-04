package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type BrowserSyncService struct {
	apiBaseURL string
	uiBaseURL  string
	httpClient *http.Client
}

type BrowserSyncStatus struct {
	ID          string       `json:"id"`
	Status      string       `json:"status"`
	Environment string       `json:"environment"`
	Progress    SyncProgress `json:"progress"`
	Result      *SyncResult  `json:"result,omitempty"`
	Error       string       `json:"error,omitempty"`
}

type SyncProgress struct {
	CurrentStep   string `json:"current_step"`
	StepsTotal    int    `json:"steps_total"`
	StepsComplete int    `json:"steps_complete"`
	Message       string `json:"message"`
}

func NewBrowserSyncService(port int) *BrowserSyncService {
	baseURL := fmt.Sprintf("http://localhost:%d", port)
	return &BrowserSyncService{
		apiBaseURL: baseURL,
		uiBaseURL:  baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *BrowserSyncService) SyncWithBrowser(ctx context.Context, environment string) (*SyncResult, error) {
	syncID, err := s.startInteractiveSync(environment)
	if err != nil {
		return nil, fmt.Errorf("failed to start sync: %w", err)
	}

	fmt.Printf("\n🌐 Opening browser for variable configuration...\n")

	syncURL := fmt.Sprintf("%s/sync/%s?sync_id=%s", s.uiBaseURL, environment, syncID)
	if err := openBrowserURL(syncURL); err != nil {
		fmt.Printf("⚠️  Could not open browser automatically.\n")
	}
	fmt.Printf("\nIf browser didn't open, visit:\n  %s\n\n", syncURL)

	fmt.Printf("⏳ Waiting for variable input in browser (5 min timeout)...\n\n")

	return s.pollForCompletion(ctx, syncID, 5*time.Minute)
}

func (s *BrowserSyncService) startInteractiveSync(environment string) (string, error) {
	reqBody := fmt.Sprintf(`{"environment":"%s"}`, environment)
	resp, err := s.httpClient.Post(
		s.apiBaseURL+"/api/v1/sync/interactive",
		"application/json",
		strings.NewReader(reqBody),
	)
	if err != nil {
		return "", fmt.Errorf("failed to connect to Station server: %w\nMake sure Station is running: stn serve", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to start sync: server returned %d", resp.StatusCode)
	}

	var result struct {
		SyncID string `json:"sync_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return result.SyncID, nil
}

func (s *BrowserSyncService) pollForCompletion(ctx context.Context, syncID string, timeout time.Duration) (*SyncResult, error) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeoutCh := time.After(timeout)
	lastStatus := ""

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeoutCh:
			return nil, fmt.Errorf("timeout waiting for sync completion - no input received in browser")
		case <-ticker.C:
			status, err := s.getSyncStatus(syncID)
			if err != nil {
				// 404 means sync completed and was cleaned up by API - treat as success
				if strings.Contains(err.Error(), "404") {
					fmt.Printf("✅ Sync completed\n")
					return &SyncResult{}, nil
				}
				continue
			}

			if status.Status != lastStatus {
				switch status.Status {
				case "waiting_for_input":
					fmt.Printf("📝 Waiting for variable input in browser...\n")
				case "processing":
					fmt.Printf("⚙️  Processing variables and syncing templates...\n")
				case "running":
					if status.Progress.Message != "" {
						fmt.Printf("🔄 %s\n", status.Progress.Message)
					}
				}
				lastStatus = status.Status
			}

			switch status.Status {
			case "completed":
				fmt.Printf("\n✅ Variables configured via browser\n")
				if status.Result != nil {
					fmt.Printf("✅ Sync completed: %d agents, %d MCP servers connected\n",
						status.Result.AgentsSynced, status.Result.MCPServersConnected)
				}
				return status.Result, nil
			case "failed":
				return nil, fmt.Errorf("sync failed: %s", status.Error)
			case "cancelled":
				return nil, fmt.Errorf("sync cancelled by user")
			}
		}
	}
}

func (s *BrowserSyncService) getSyncStatus(syncID string) (*BrowserSyncStatus, error) {
	resp, err := s.httpClient.Get(s.apiBaseURL + "/api/v1/sync/status/" + syncID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("404: sync not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var status BrowserSyncStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

func openBrowserURL(url string) error {
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
