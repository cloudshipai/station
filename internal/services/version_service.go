package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"station/internal/version"
)

// GitHubRelease represents a GitHub release from the API
type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
}

// VersionInfo contains current and latest version information
type VersionInfo struct {
	CurrentVersion  string    `json:"current_version"`
	LatestVersion   string    `json:"latest_version"`
	UpdateAvailable bool      `json:"update_available"`
	ReleaseURL      string    `json:"release_url,omitempty"`
	ReleaseNotes    string    `json:"release_notes,omitempty"`
	PublishedAt     time.Time `json:"published_at,omitempty"`
	CheckedAt       time.Time `json:"checked_at"`
}

// UpdateResult contains the result of an update operation
type UpdateResult struct {
	Success         bool   `json:"success"`
	Message         string `json:"message"`
	PreviousVersion string `json:"previous_version,omitempty"`
	NewVersion      string `json:"new_version,omitempty"`
	Error           string `json:"error,omitempty"`
}

// VersionService handles version checking and updates
type VersionService struct {
	githubOwner string
	githubRepo  string
	httpClient  *http.Client

	// Cache for version info
	cache      *VersionInfo
	cacheMutex sync.RWMutex
	cacheTTL   time.Duration
}

// NewVersionService creates a new version service
func NewVersionService() *VersionService {
	return &VersionService{
		githubOwner: "cloudshipai",
		githubRepo:  "station",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		cacheTTL: 5 * time.Minute,
	}
}

// GetCurrentVersion returns the current version information
func (s *VersionService) GetCurrentVersion() map[string]interface{} {
	buildInfo := version.GetBuildInfo()
	return map[string]interface{}{
		"version":    buildInfo.Version,
		"build_time": buildInfo.BuildTime,
		"go_version": buildInfo.GoVersion,
		"go_arch":    buildInfo.GoArch,
		"go_os":      buildInfo.GoOS,
		"compiler":   buildInfo.Compiler,
		"is_dev":     version.IsDevBuild(),
	}
}

// CheckForUpdates checks GitHub for the latest release
func (s *VersionService) CheckForUpdates(ctx context.Context) (*VersionInfo, error) {
	// Check cache first
	s.cacheMutex.RLock()
	if s.cache != nil && time.Since(s.cache.CheckedAt) < s.cacheTTL {
		cached := *s.cache
		s.cacheMutex.RUnlock()
		return &cached, nil
	}
	s.cacheMutex.RUnlock()

	// Fetch latest release from GitHub
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", s.githubOwner, s.githubRepo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Station-Update-Checker")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// No releases found - return current version info
		return &VersionInfo{
			CurrentVersion:  version.GetVersion(),
			LatestVersion:   version.GetVersion(),
			UpdateAvailable: false,
			CheckedAt:       time.Now(),
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release: %w", err)
	}

	currentVersion := version.GetVersion()
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersionClean := strings.TrimPrefix(currentVersion, "v")

	info := &VersionInfo{
		CurrentVersion:  currentVersion,
		LatestVersion:   release.TagName,
		UpdateAvailable: s.isNewerVersion(currentVersionClean, latestVersion),
		ReleaseURL:      release.HTMLURL,
		ReleaseNotes:    release.Body,
		PublishedAt:     release.PublishedAt,
		CheckedAt:       time.Now(),
	}

	// Update cache
	s.cacheMutex.Lock()
	s.cache = info
	s.cacheMutex.Unlock()

	return info, nil
}

// isNewerVersion compares two semantic versions
// Returns true if latest is newer than current
func (s *VersionService) isNewerVersion(current, latest string) bool {
	// Handle dev builds - always show update available for dev
	if current == "dev" || current == "" {
		return true
	}

	// Simple version comparison for semver
	currentParts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")

	// Pad shorter version with zeros
	for len(currentParts) < 3 {
		currentParts = append(currentParts, "0")
	}
	for len(latestParts) < 3 {
		latestParts = append(latestParts, "0")
	}

	for i := 0; i < 3; i++ {
		// Extract numeric part (remove any suffix like -alpha, -beta, etc.)
		currentNum := extractNumeric(currentParts[i])
		latestNum := extractNumeric(latestParts[i])

		if latestNum > currentNum {
			return true
		}
		if latestNum < currentNum {
			return false
		}
	}

	return false
}

// extractNumeric extracts the numeric part from a version component
func extractNumeric(s string) int {
	// Remove any non-numeric suffix
	var num int
	fmt.Sscanf(s, "%d", &num)
	return num
}

// PerformUpdate runs the install script to update Station
func (s *VersionService) PerformUpdate(ctx context.Context) (*UpdateResult, error) {
	previousVersion := version.GetVersion()

	// Check platform compatibility
	if runtime.GOOS == "windows" {
		return &UpdateResult{
			Success: false,
			Message: "Automatic updates are not supported on Windows. Please download the latest release manually.",
			Error:   "windows_not_supported",
		}, nil
	}

	// The install script URL
	installScript := "https://raw.githubusercontent.com/cloudshipai/station/main/install.sh"

	// Create the command to download and run the install script
	// Using bash -c to pipe curl output to bash
	cmd := exec.CommandContext(ctx, "bash", "-c",
		fmt.Sprintf("curl -fsSL %s | bash", installScript))

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &UpdateResult{
			Success:         false,
			Message:         "Update failed",
			PreviousVersion: previousVersion,
			Error:           fmt.Sprintf("install script error: %v\nOutput: %s", err, string(output)),
		}, nil
	}

	// Clear the cache so next check shows correct version
	s.cacheMutex.Lock()
	s.cache = nil
	s.cacheMutex.Unlock()

	return &UpdateResult{
		Success:         true,
		Message:         "Update completed successfully. Please restart Station to use the new version.",
		PreviousVersion: previousVersion,
		NewVersion:      "latest",
	}, nil
}

// ClearCache clears the version cache
func (s *VersionService) ClearCache() {
	s.cacheMutex.Lock()
	s.cache = nil
	s.cacheMutex.Unlock()
}
