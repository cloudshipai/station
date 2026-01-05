package cloudship

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"station/internal/config"
)

// Client is a CloudShip API client for bundle operations.
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
}

// NewClient creates a new CloudShip client.
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// GetAPIURL returns the CloudShip API URL from config or default.
func (c *Client) GetAPIURL() string {
	apiURL := c.cfg.CloudShip.APIURL
	if apiURL == "" {
		apiURL = c.cfg.CloudShip.BaseURL
	}
	if apiURL == "" {
		apiURL = "https://app.cloudshipai.com"
	}
	return strings.TrimSuffix(apiURL, "/")
}

// GetAuthHeader returns the header name and value for CloudShip authentication.
// Priority: 1) APIKey from config (stn auth login), 2) RegistrationKey (CI/CD, Docker)
func (c *Client) GetAuthHeader() (headerName string, headerValue string, err error) {
	// Try API key from config (from stn auth login) - Bearer token
	if c.cfg.CloudShip.APIKey != "" {
		return "Authorization", "Bearer " + c.cfg.CloudShip.APIKey, nil
	}

	// Fall back to registration key (CI/CD, Docker containers)
	if c.cfg.CloudShip.RegistrationKey != "" {
		return "X-Registration-Key", c.cfg.CloudShip.RegistrationKey, nil
	}

	return "", "", fmt.Errorf("no CloudShip authentication configured. Set STN_CLOUDSHIP_KEY environment variable")
}

// IsUUID checks if a string matches UUID format.
func IsUUID(s string) bool {
	uuidPattern := `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	matched, _ := regexp.MatchString(uuidPattern, s)
	return matched
}

// DownloadBundle downloads a bundle from CloudShip by ID and returns the path to the downloaded file.
// The caller is responsible for cleaning up the temp file.
func (c *Client) DownloadBundle(bundleID string) (string, error) {
	// Get auth header
	headerName, headerValue, err := c.GetAuthHeader()
	if err != nil {
		return "", err
	}

	// Build download URL (trailing slash required by CloudShip API)
	apiURL := c.GetAPIURL()
	downloadURL := fmt.Sprintf("%s/api/public/bundles/%s/download/", apiURL, bundleID)

	// Create request
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set(headerName, headerValue)

	// Make request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to connect to CloudShip: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return "", fmt.Errorf("unauthorized - check your CloudShip registration key or API key")
	}

	if resp.StatusCode == 404 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("bundle not found or no downloadable version: %s (API response: %s)", bundleID, string(body))
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("CloudShip API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", "station-bundle-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	// Copy response body to temp file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to download bundle: %w", err)
	}

	tmpFile.Close()
	return tmpFile.Name(), nil
}

type UploadResponse struct {
	BundleID     string `json:"bundle_id"`
	Filename     string `json:"filename"`
	Size         int64  `json:"size"`
	Organization string `json:"organization"`
	UploadedAt   string `json:"uploaded_at"`
	DownloadURL  string `json:"download_url"`
}

func (c *Client) UploadBundle(bundlePath string) (*UploadResponse, error) {
	headerName, headerValue, err := c.GetAuthHeader()
	if err != nil {
		return nil, err
	}

	file, err := os.Open(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open bundle: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("bundle", filepath.Base(bundlePath))
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}
	writer.Close()

	apiURL := c.GetAPIURL()
	uploadURL := fmt.Sprintf("%s/api/public/bundles/upload/", apiURL)

	req, err := http.NewRequest("POST", uploadURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set(headerName, headerValue)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("upload failed (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var uploadResp UploadResponse
	if err := json.Unmarshal(bodyBytes, &uploadResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &uploadResp, nil
}
