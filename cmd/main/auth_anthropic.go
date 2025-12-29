package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"station/internal/config"
)

const anthropicClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"

var authAnthropicCmd = &cobra.Command{
	Use:   "anthropic",
	Short: "Manage Anthropic OAuth authentication (Claude Max/Pro)",
	Long: `Manage authentication with your Claude Max or Claude Pro subscription.

This uses Anthropic's OAuth flow to authenticate your Claude subscription,
allowing Station to use your subscription for AI operations instead of
requiring an API key.

Subcommands:
  login   - Authenticate with Anthropic
  status  - Show authentication status
  logout  - Remove stored tokens

Examples:
  stn auth anthropic login
  stn auth anthropic status
  stn auth anthropic logout`,
}

var authAnthropicLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Anthropic (Claude Max/Pro)",
	Long: `Authenticate with your Claude Max or Claude Pro subscription.

The flow will:
1. Open your browser to authenticate with Anthropic
2. You paste the authorization code back here
3. Station stores the tokens securely in your config

Examples:
  # Authenticate with Claude Max subscription
  stn auth anthropic login

  # Authenticate with Claude console (API billing)
  stn auth anthropic login --mode console`,
	RunE: runAuthAnthropic,
}

var authAnthropicStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Anthropic authentication status",
	RunE:  runAuthAnthropicStatus,
}

var authAnthropicLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove Anthropic authentication tokens",
	RunE:  runAuthAnthropicLogout,
}

func init() {
	authAnthropicLoginCmd.Flags().String("mode", "max", "Authentication mode: 'max' (Claude Max/Pro subscription) or 'console' (API billing)")
	authAnthropicCmd.AddCommand(authAnthropicLoginCmd)
	authAnthropicCmd.AddCommand(authAnthropicStatusCmd)
	authAnthropicCmd.AddCommand(authAnthropicLogoutCmd)
	authCmd.AddCommand(authAnthropicCmd)
}

func runAuthAnthropic(cmd *cobra.Command, args []string) error {
	mode, _ := cmd.Flags().GetString("mode")
	if mode != "max" && mode != "console" {
		return fmt.Errorf("invalid mode '%s': must be 'max' or 'console'", mode)
	}

	pkce, err := generatePKCE()
	if err != nil {
		return fmt.Errorf("failed to generate PKCE: %w", err)
	}

	authURL := buildAuthURL(mode, pkce.Challenge)

	fmt.Println("Opening browser to authenticate with Anthropic...")
	fmt.Println()

	if err := openBrowser(authURL); err != nil {
		fmt.Println("Could not open browser automatically.")
		fmt.Println()
	}

	fmt.Println("Please visit this URL if the browser didn't open:")
	fmt.Println()
	fmt.Printf("  %s\n", authURL)
	fmt.Println()
	fmt.Println("After authorizing, you'll see a code. Paste it here:")
	fmt.Print("> ")

	reader := bufio.NewReader(os.Stdin)
	codeInput, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read code: %w", err)
	}
	codeInput = strings.TrimSpace(codeInput)

	if codeInput == "" {
		return fmt.Errorf("authorization code is required")
	}

	fmt.Println()
	fmt.Println("Exchanging code for tokens...")

	tokens, err := exchangeCodeForTokens(codeInput, pkce.Verifier)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}

	if err := config.SaveOAuthTokens(tokens.AccessToken, tokens.RefreshToken, tokens.ExpiresAt); err != nil {
		return fmt.Errorf("failed to save tokens: %w", err)
	}

	fmt.Println()
	fmt.Println("✅ Successfully authenticated with Anthropic!")
	fmt.Println()

	if mode == "max" {
		fmt.Println("   You're using your Claude Max/Pro subscription.")
	} else {
		fmt.Println("   You're using Anthropic Console (API billing).")
	}

	fmt.Println("   Station will automatically refresh tokens as needed.")
	fmt.Println()
	fmt.Println("To use Anthropic as your AI provider, ensure your config has:")
	fmt.Println("   ai_provider: anthropic")
	fmt.Println("   ai_model: claude-sonnet-4-20250514")

	return nil
}

func runAuthAnthropicStatus(cmd *cobra.Command, args []string) error {
	authType := viper.GetString("ai_auth_type")
	accessToken := viper.GetString("ai_oauth_token")
	expiresAt := viper.GetInt64("ai_oauth_expires_at")

	if authType != "oauth" || accessToken == "" {
		fmt.Println("❌ Not authenticated with Anthropic")
		fmt.Println()
		fmt.Println("To authenticate, run:")
		fmt.Println("   stn auth anthropic login")
		return nil
	}

	fmt.Println("✅ Authenticated with Anthropic")
	fmt.Println()

	now := time.Now().UnixMilli()
	if expiresAt > 0 {
		expiresTime := time.UnixMilli(expiresAt)
		if expiresAt < now {
			fmt.Printf("   Token expired: %s\n", expiresTime.Format(time.RFC3339))
			fmt.Println("   (will be refreshed on next use)")
		} else {
			remaining := time.Duration(expiresAt-now) * time.Millisecond
			fmt.Printf("   Token expires: %s (%s remaining)\n", expiresTime.Format(time.RFC3339), remaining.Round(time.Minute))
		}
	}

	fmt.Printf("   Token prefix: %s...\n", accessToken[:20])
	fmt.Println()
	fmt.Println("To use this authentication, ensure your config has:")
	fmt.Println("   ai_provider: anthropic")

	return nil
}

func runAuthAnthropicLogout(cmd *cobra.Command, args []string) error {
	viper.Set("ai_auth_type", "")
	viper.Set("ai_oauth_token", "")
	viper.Set("ai_oauth_refresh_token", "")
	viper.Set("ai_oauth_expires_at", 0)

	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = config.GetConfigRoot() + "/config.yaml"
	}

	if err := viper.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("failed to clear tokens: %w", err)
	}

	fmt.Println("✅ Logged out from Anthropic")
	fmt.Println("   OAuth tokens have been removed from your config.")

	return nil
}

type pkceParams struct {
	Verifier  string
	Challenge string
}

func generatePKCE() (*pkceParams, error) {
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	h := sha256.New()
	h.Write([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return &pkceParams{
		Verifier:  verifier,
		Challenge: challenge,
	}, nil
}

func buildAuthURL(mode, challenge string) string {
	var host string
	if mode == "console" {
		host = "console.anthropic.com"
	} else {
		host = "claude.ai"
	}

	params := []string{
		"code=true",
		"client_id=" + anthropicClientID,
		"response_type=code",
		"redirect_uri=https://console.anthropic.com/oauth/code/callback",
		"scope=org:create_api_key user:profile user:inference",
		"code_challenge=" + challenge,
		"code_challenge_method=S256",
	}

	return fmt.Sprintf("https://%s/oauth/authorize?%s", host, strings.Join(params, "&"))
}

type oauthTokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64
}

func exchangeCodeForTokens(codeInput, verifier string) (*oauthTokens, error) {
	parts := strings.Split(codeInput, "#")
	code := parts[0]
	state := ""
	if len(parts) > 1 {
		state = parts[1]
	}

	reqBody := map[string]string{
		"code":          code,
		"grant_type":    "authorization_code",
		"client_id":     anthropicClientID,
		"redirect_uri":  "https://console.anthropic.com/oauth/code/callback",
		"code_verifier": verifier,
	}
	if state != "" {
		reqBody["state"] = state
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://console.anthropic.com/v1/oauth/token", strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Description != "" {
			return nil, fmt.Errorf("%s: %s", errResp.Error, errResp.Description)
		}
		return nil, fmt.Errorf("token exchange failed with status %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &oauthTokens{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().UnixMilli() + (tokenResp.ExpiresIn * 1000),
	}, nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}
