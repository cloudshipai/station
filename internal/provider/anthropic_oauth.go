package provider

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	anthropicClientID    = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	anthropicAuthURL     = "https://claude.ai/oauth/authorize"
	anthropicTokenURL    = "https://console.anthropic.com/v1/oauth/token"
	anthropicRedirectURI = "http://localhost:%d/callback"
	anthropicScopes      = "org:create_api_key user:profile user:inference"
	callbackPortStart    = 54545
)

type AnthropicOAuth struct {
	codeVerifier string
	state        string
	callbackPort int
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Email        string `json:"email"`
	TokenType    string `json:"token_type"`
}

func NewAnthropicOAuth() *AnthropicOAuth {
	return &AnthropicOAuth{}
}

func (a *AnthropicOAuth) generatePKCE() error {
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return fmt.Errorf("failed to generate verifier: %w", err)
	}
	a.codeVerifier = base64.RawURLEncoding.EncodeToString(verifierBytes)

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return fmt.Errorf("failed to generate state: %w", err)
	}
	a.state = fmt.Sprintf("%x", stateBytes)

	return nil
}

func (a *AnthropicOAuth) codeChallenge() string {
	hash := sha256.Sum256([]byte(a.codeVerifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func (a *AnthropicOAuth) findAvailablePort() error {
	for port := callbackPortStart; port < callbackPortStart+100; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			continue
		}
		ln.Close()
		a.callbackPort = port
		return nil
	}
	return fmt.Errorf("no available port found in range %d-%d", callbackPortStart, callbackPortStart+99)
}

func (a *AnthropicOAuth) buildAuthURL() string {
	redirectURI := fmt.Sprintf(anthropicRedirectURI, a.callbackPort)

	params := url.Values{
		"client_id":             {anthropicClientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {anthropicScopes},
		"code_challenge":        {a.codeChallenge()},
		"code_challenge_method": {"S256"},
		"response_type":         {"code"},
		"state":                 {a.state},
	}

	return anthropicAuthURL + "?" + params.Encode()
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

func (a *AnthropicOAuth) Login(ctx context.Context) (*ProviderCredentials, error) {
	if err := a.generatePKCE(); err != nil {
		return nil, fmt.Errorf("failed to generate PKCE: %w", err)
	}

	if err := a.findAvailablePort(); err != nil {
		return nil, fmt.Errorf("failed to find available port: %w", err)
	}

	authURL := a.buildAuthURL()
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", a.callbackPort),
	}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		errorParam := r.URL.Query().Get("error")

		if errorParam != "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Authentication failed: %s", errorParam)
			errChan <- fmt.Errorf("OAuth error: %s", errorParam)
			return
		}

		if state != a.state {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "State mismatch - possible CSRF attack")
			errChan <- fmt.Errorf("state mismatch")
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Authentication Successful</title></head>
<body style="font-family: system-ui; text-align: center; padding: 50px; background: #1a1a2e; color: #eee;">
<h1 style="color: #00d4aa;">Authentication Successful</h1>
<p>You can close this window and return to the terminal.</p>
<script>setTimeout(() => window.close(), 2000);</script>
</body>
</html>`)

		codeChan <- code
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("callback server error: %w", err)
		}
	}()

	fmt.Printf("\nOpening browser for authentication...\n")
	fmt.Printf("If the browser doesn't open, visit:\n%s\n\n", authURL)

	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Failed to open browser automatically: %v\n", err)
	}

	fmt.Println("Waiting for authentication...")

	var authCode string
	select {
	case code := <-codeChan:
		authCode = code
	case err := <-errChan:
		server.Shutdown(ctx)
		return nil, err
	case <-ctx.Done():
		server.Shutdown(ctx)
		return nil, ctx.Err()
	case <-time.After(5 * time.Minute):
		server.Shutdown(ctx)
		return nil, fmt.Errorf("authentication timeout")
	}

	server.Shutdown(ctx)

	fmt.Println("Exchanging code for tokens...")
	tokens, err := a.exchangeCodeForTokens(authCode)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)

	return &ProviderCredentials{
		Provider:     ProviderAnthropic,
		AuthType:     AuthTypeOAuth,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    &expiresAt,
		Email:        tokens.Email,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}, nil
}

func (a *AnthropicOAuth) exchangeCodeForTokens(code string) (*tokenResponse, error) {
	redirectURI := fmt.Sprintf(anthropicRedirectURI, a.callbackPort)

	body := map[string]string{
		"code":          code,
		"state":         a.state,
		"grant_type":    "authorization_code",
		"client_id":     anthropicClientID,
		"redirect_uri":  redirectURI,
		"code_verifier": a.codeVerifier,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", anthropicTokenURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed with status %d", resp.StatusCode)
	}

	var tokens tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokens, nil
}
