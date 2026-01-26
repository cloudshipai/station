package e2b

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultAPIURL     = "https://api.e2b.dev"
	DefaultDomain     = "e2b.dev"
	EnvdPort          = 49983
	DefaultTimeoutSec = 300
)

type Client struct {
	apiKey        string
	apiURL        string
	domain        string
	httpClient    *http.Client
	sandboxID     string
	accessToken   string
	envdURL       string
	sandboxDomain string
}

type ClientConfig struct {
	APIKey  string
	APIURL  string
	Domain  string
	Timeout time.Duration
}

func NewClient(cfg ClientConfig) *Client {
	if cfg.APIURL == "" {
		cfg.APIURL = DefaultAPIURL
	}
	if cfg.Domain == "" {
		cfg.Domain = DefaultDomain
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	return &Client{
		apiKey: cfg.APIKey,
		apiURL: cfg.APIURL,
		domain: cfg.Domain,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

type CreateSandboxRequest struct {
	TemplateID string            `json:"templateID"`
	Timeout    int               `json:"timeout"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	EnvVars    map[string]string `json:"envVars,omitempty"`
}

type SandboxInfo struct {
	SandboxID          string `json:"sandboxID"`
	TemplateID         string `json:"templateID"`
	Domain             string `json:"domain"`
	EnvdAccessToken    string `json:"envdAccessToken"`
	TrafficAccessToken string `json:"trafficAccessToken,omitempty"`
	EnvdVersion        string `json:"envdVersion"`
	StartedAt          string `json:"startedAt"`
	EndAt              string `json:"endAt"`
}

func (c *Client) CreateSandbox(ctx context.Context, req CreateSandboxRequest) (*SandboxInfo, error) {
	if req.Timeout == 0 {
		req.Timeout = DefaultTimeoutSec
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/sandboxes", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-KEY", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create sandbox failed: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var info SandboxInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	c.sandboxID = info.SandboxID
	c.accessToken = info.EnvdAccessToken
	c.sandboxDomain = info.Domain
	if c.sandboxDomain == "" {
		c.sandboxDomain = c.domain
	}
	c.envdURL = fmt.Sprintf("https://%d-%s.%s", EnvdPort, info.SandboxID, c.sandboxDomain)

	return &info, nil
}

func (c *Client) Connect(sandboxID string, accessToken string, domain string) {
	c.sandboxID = sandboxID
	c.accessToken = accessToken
	if domain == "" {
		domain = c.domain
	}
	c.sandboxDomain = domain
	c.envdURL = fmt.Sprintf("https://%d-%s.%s", EnvdPort, sandboxID, domain)
}

func (c *Client) Kill(ctx context.Context) error {
	if c.sandboxID == "" {
		return nil
	}

	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", c.apiURL+"/sandboxes/"+c.sandboxID, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("X-API-KEY", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kill sandbox failed: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Error    string
}

func (c *Client) Exec(ctx context.Context, command string, args ...string) (*ExecResult, error) {
	fullCmd := command
	if len(args) > 0 {
		fullCmd = command + " " + strings.Join(args, " ")
	}

	reqBody := map[string]interface{}{
		"cmd":  fullCmd,
		"wait": true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.envdURL+"/commands", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Access-Token", c.accessToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("exec failed: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ExitCode int    `json:"exitCode"`
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
		Error    string `json:"error,omitempty"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &ExecResult{
		ExitCode: result.ExitCode,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		Error:    result.Error,
	}, nil
}

func (c *Client) ReadFile(ctx context.Context, path string) ([]byte, error) {
	reqURL := c.envdURL + "/files?" + url.Values{"path": {path}}.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-Access-Token", c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("read file failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

func (c *Client) WriteFile(ctx context.Context, path string, content []byte) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", path)
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}

	if _, err := part.Write(content); err != nil {
		return fmt.Errorf("write content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close writer: %w", err)
	}

	reqURL := c.envdURL + "/files?" + url.Values{"path": {path}}.Encode()

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, &buf)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Access-Token", c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("write file failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) DeleteFile(ctx context.Context, path string) error {
	reqURL := c.envdURL + "/files?" + url.Values{"path": {path}}.Encode()

	req, err := http.NewRequestWithContext(ctx, "DELETE", reqURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-Access-Token", c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete file failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) MakeDir(ctx context.Context, path string) error {
	return c.WriteFile(ctx, path+"/.keep", []byte{})
}

type FileInfo struct {
	Name    string
	Path    string
	Size    int64
	IsDir   bool
	ModTime time.Time
}

func (c *Client) ListDir(ctx context.Context, path string) ([]FileInfo, error) {
	result, err := c.Exec(ctx, "ls", "-la", "--time-style=+%s", path)
	if err != nil {
		return nil, err
	}

	if result.ExitCode != 0 {
		return nil, fmt.Errorf("ls failed: %s", result.Stderr)
	}

	var files []FileInfo
	lines := strings.Split(result.Stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}

		name := fields[len(fields)-1]
		if name == "." || name == ".." {
			continue
		}

		isDir := strings.HasPrefix(fields[0], "d")

		var size int64
		fmt.Sscanf(fields[4], "%d", &size)

		var modTime time.Time
		var timestamp int64
		if n, _ := fmt.Sscanf(fields[5], "%d", &timestamp); n == 1 {
			modTime = time.Unix(timestamp, 0)
		}

		files = append(files, FileInfo{
			Name:    name,
			Path:    path + "/" + name,
			Size:    size,
			IsDir:   isDir,
			ModTime: modTime,
		})
	}

	return files, nil
}

func (c *Client) FileExists(ctx context.Context, path string) (bool, error) {
	result, err := c.Exec(ctx, "test", "-e", path, "&&", "echo", "exists")
	if err != nil {
		return false, err
	}

	return strings.Contains(result.Stdout, "exists"), nil
}

func (c *Client) SandboxID() string {
	return c.sandboxID
}

func (c *Client) EnvdURL() string {
	return c.envdURL
}

func (c *Client) SetEnvdURL(url string) {
	c.envdURL = url
}

func (c *Client) SetAPIURL(url string) {
	c.apiURL = url
}
