package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"station/pkg/bundle"
)

// HTTPRegistry implements BundleRegistry for HTTP-based registries
type HTTPRegistry struct {
	baseURL    string
	httpClient HTTPClient
	auth       map[string]string
	name       string
}

// HTTPClient defines the interface for HTTP operations (allows mocking)
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// NewHTTPRegistry creates a new HTTP registry
func NewHTTPRegistry(name, baseURL string, auth map[string]string) *HTTPRegistry {
	return &HTTPRegistry{
		name:       name,
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		auth:       auth,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SetHTTPClient allows setting a custom HTTP client (for testing)
func (r *HTTPRegistry) SetHTTPClient(client HTTPClient) {
	r.httpClient = client
}

// List returns all available bundles in the registry
func (r *HTTPRegistry) List(ctx context.Context, opts bundle.ListOptions) ([]bundle.BundleManifest, error) {
	// Build query parameters
	params := url.Values{}
	if opts.Registry != "" && opts.Registry != r.name {
		return []bundle.BundleManifest{}, nil // Wrong registry
	}
	if opts.Search != "" {
		params.Set("search", opts.Search)
	}
	if len(opts.Tags) > 0 {
		params.Set("tags", strings.Join(opts.Tags, ","))
	}

	// Make request
	endpoint := fmt.Sprintf("%s/bundles", r.baseURL)
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	r.addAuth(req)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	var response struct {
		Bundles []bundle.BundleManifest `json:"bundles"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Bundles, nil
}

// Get returns metadata for a specific bundle
func (r *HTTPRegistry) Get(ctx context.Context, name, version string) (*bundle.BundleManifest, error) {
	endpoint := fmt.Sprintf("%s/bundles/%s", r.baseURL, name)
	if version != "" {
		endpoint += "/" + version
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	r.addAuth(req)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("bundle not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	var manifest bundle.BundleManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &manifest, nil
}

// Download downloads a bundle and returns the archive data
func (r *HTTPRegistry) Download(ctx context.Context, name, version string) ([]byte, error) {
	endpoint := fmt.Sprintf("%s/bundles/%s/download", r.baseURL, name)
	if version != "" {
		endpoint += "?version=" + version
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	r.addAuth(req)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("bundle not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return data, nil
}

// GetVersions returns all available versions for a bundle
func (r *HTTPRegistry) GetVersions(ctx context.Context, name string) ([]string, error) {
	endpoint := fmt.Sprintf("%s/bundles/%s/versions", r.baseURL, name)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	r.addAuth(req)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("bundle not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	var response struct {
		Versions []string `json:"versions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Versions, nil
}

func (r *HTTPRegistry) addAuth(req *http.Request) {
	// Add authentication headers
	if apiKey, ok := r.auth["api_key"]; ok {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if token, ok := r.auth["token"]; ok {
		req.Header.Set("Authorization", "Token "+token)
	}
	if username, ok := r.auth["username"]; ok {
		if password, ok := r.auth["password"]; ok {
			req.SetBasicAuth(username, password)
		}
	}

	// Set content type and user agent
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "station-bundle-client/1.0")
}
