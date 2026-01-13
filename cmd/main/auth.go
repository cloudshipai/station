package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"station/internal/config"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage CloudShip authentication",
	Long: `Manage authentication with CloudShip for accessing bundles and other resources.

The API key is stored in your Station config file and used to authenticate
with CloudShip's API for downloading bundles, viewing your organization's
resources, and other platform features.

Note: This is different from the registration_key which is used for Station
remote management. The API key is for user-level authentication.`,
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with CloudShip",
	Long: `Authenticate with CloudShip using your API key.

You can get your API key from the CloudShip dashboard at:
https://app.cloudshipai.com/webapp/settings/

The API key will be stored in your Station config file.

Examples:
  # Interactive login (prompts for API key)
  stn auth login

  # Login with API key flag
  stn auth login --api-key cs_live_xxxxxxxxxxxx

  # Login using environment variable
  CLOUDSHIP_API_KEY=cs_live_xxxxxxxxxxxx stn auth login`,
	RunE: runAuthLogin,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove CloudShip authentication",
	Long: `Remove your CloudShip API key from the Station config.

This will clear the stored API key. You will need to login again
to access CloudShip resources like bundles.`,
	RunE: runAuthLogout,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	Long: `Show your current CloudShip authentication status.

Displays whether you are authenticated and shows information about
your CloudShip account if authenticated.`,
	RunE: runAuthStatus,
}

func init() {
	authLoginCmd.Flags().String("api-key", "", "CloudShip API key")
	
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
	
	rootCmd.AddCommand(authCmd)
}

// CloudShipUserResponse represents the user info from CloudShip API
type CloudShipUserResponse struct {
	Email        string `json:"email"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Organization struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"organization"`
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	// Try to get API key from flag, env var, or prompt
	apiKey, _ := cmd.Flags().GetString("api-key")
	
	if apiKey == "" {
		apiKey = os.Getenv("CLOUDSHIP_API_KEY")
	}
	
	if apiKey == "" {
		// Prompt for API key
		fmt.Print("Enter your CloudShip API key: ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		apiKey = strings.TrimSpace(input)
	}
	
	if apiKey == "" {
		return fmt.Errorf("API key is required. Get yours from https://app.cloudshipai.com/webapp/settings/")
	}
	
	// Validate the API key by calling CloudShip API
	fmt.Println("Validating API key...")
	
	userInfo, err := validateAPIKey(apiKey)
	if err != nil {
		return fmt.Errorf("invalid API key: %w", err)
	}
	
	// Save to config
	if err := saveAPIKey(apiKey); err != nil {
		return fmt.Errorf("failed to save API key: %w", err)
	}
	
	fmt.Println()
	fmt.Println("✅ Successfully authenticated with CloudShip!")
	fmt.Printf("   Email: %s\n", userInfo.Email)
	if userInfo.Organization.Name != "" {
		fmt.Printf("   Organization: %s\n", userInfo.Organization.Name)
	}
	fmt.Println()
	fmt.Println("You can now download bundles from CloudShip:")
	fmt.Println("   stn bundle install <bundle-id> <environment-name>")
	
	return nil
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	// Clear API key from config
	if err := clearAPIKey(); err != nil {
		return fmt.Errorf("failed to clear API key: %w", err)
	}
	
	fmt.Println("✅ Logged out from CloudShip")
	fmt.Println("   API key has been removed from your config.")
	
	return nil
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	
	// Check for API key in config or env
	apiKey := cfg.CloudShip.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("CLOUDSHIP_API_KEY")
	}
	
	if apiKey == "" {
		fmt.Println("❌ Not authenticated with CloudShip")
		fmt.Println()
		fmt.Println("To authenticate, run:")
		fmt.Println("   stn auth login")
		return nil
	}
	
	// Validate and get user info
	userInfo, err := validateAPIKey(apiKey)
	if err != nil {
		fmt.Println("⚠️  API key is invalid or expired")
		fmt.Printf("   Error: %v\n", err)
		fmt.Println()
		fmt.Println("To re-authenticate, run:")
		fmt.Println("   stn auth login")
		return nil
	}
	
	fmt.Println("✅ Authenticated with CloudShip")
	fmt.Printf("   Email: %s\n", userInfo.Email)
	if userInfo.FirstName != "" || userInfo.LastName != "" {
		fmt.Printf("   Name: %s %s\n", userInfo.FirstName, userInfo.LastName)
	}
	if userInfo.Organization.Name != "" {
		fmt.Printf("   Organization: %s\n", userInfo.Organization.Name)
	}
	
	// Show where the key is stored
	if cfg.CloudShip.APIKey != "" {
		fmt.Println("   Key stored in: config file")
	} else {
		fmt.Println("   Key source: CLOUDSHIP_API_KEY environment variable")
	}
	
	return nil
}

func validateAPIKey(apiKey string) (*CloudShipUserResponse, error) {
	// Get API URL from config or use default
	apiURL := viper.GetString("cloudship.api_url")
	if apiURL == "" {
		apiURL = "https://app.cloudshipai.com"
	}
	
	// Validate by trying to list bundles - this endpoint is known to work
	// We'll use this as a proxy for "is this key valid"
	url := fmt.Sprintf("%s/api/public/bundles/", strings.TrimSuffix(apiURL, "/"))
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Use Bearer token for user authentication
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CloudShip: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, fmt.Errorf("invalid or expired API key")
	}
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("CloudShip API error (status %d)", resp.StatusCode)
	}
	
	// Parse the bundles response to extract organization info if available
	var bundlesResp struct {
		Results []struct {
			Organization string `json:"organization"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&bundlesResp); err != nil {
		// Even if we can't parse, the key is valid since we got 200
		return &CloudShipUserResponse{
			Email: "authenticated",
		}, nil
	}
	
	// Build user info from what we have
	userInfo := &CloudShipUserResponse{
		Email: "authenticated",
	}
	
	if len(bundlesResp.Results) > 0 && bundlesResp.Results[0].Organization != "" {
		userInfo.Organization.Name = bundlesResp.Results[0].Organization
	}
	
	return userInfo, nil
}

func saveAPIKey(apiKey string) error {
	// Update viper config
	viper.Set("cloudship.api_key", apiKey)
	viper.Set("cloudship.enabled", true)
	
	// Write to config file
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		// Use default config location
		configRoot := config.GetConfigRoot()
		configFile = configRoot + "/config.yaml"
	}
	
	return viper.WriteConfigAs(configFile)
}

func clearAPIKey() error {
	// Clear API key from viper
	viper.Set("cloudship.api_key", "")
	
	// Write to config file
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configRoot := config.GetConfigRoot()
		configFile = configRoot + "/config.yaml"
	}
	
	return viper.WriteConfigAs(configFile)
}

// GetCloudShipAPIKey returns the CloudShip API key from config or environment
// GetCloudShipAuthHeader returns the header name and value for CloudShip authentication
// Priority: 1) APIKey from env, 2) APIKey from config (stn auth login), 3) RegistrationKey (CI/CD fallback)
func GetCloudShipAuthHeader() (headerName string, headerValue string, err error) {
	// Try environment variable first (Bearer token)
	if apiKey := os.Getenv("CLOUDSHIP_API_KEY"); apiKey != "" {
		return "Authorization", "Bearer " + apiKey, nil
	}
	
	// Try config
	cfg, err := config.Load()
	if err != nil {
		return "", "", fmt.Errorf("failed to load config: %w", err)
	}
	
	// Try user auth from config (from stn auth login) - Bearer token
	if cfg.CloudShip.APIKey != "" {
		return "Authorization", "Bearer " + cfg.CloudShip.APIKey, nil
	}
	
	// Fall back to registration key (CI/CD)
	if cfg.CloudShip.RegistrationKey != "" {
		return "X-Registration-Key", cfg.CloudShip.RegistrationKey, nil
	}
	
	return "", "", fmt.Errorf("not authenticated. Run 'stn auth login' first")
}

// GetCloudShipAPIKey returns the API key for backward compatibility
// Deprecated: Use GetCloudShipAuthHeader instead
func GetCloudShipAPIKey() (string, error) {
	_, value, err := GetCloudShipAuthHeader()
	if err != nil {
		return "", err
	}
	// Strip "Bearer " prefix if present
	if strings.HasPrefix(value, "Bearer ") {
		return strings.TrimPrefix(value, "Bearer "), nil
	}
	return value, nil
}

// GetCloudShipAPIURL returns the CloudShip API URL from config or default
func GetCloudShipAPIURL() string {
	apiURL := viper.GetString("cloudship.api_url")
	if apiURL == "" {
		apiURL = "https://api.cloudshipai.com"
	}
	return strings.TrimSuffix(apiURL, "/")
}

// CloudShipAuthResult contains the result of CloudShip authentication
type CloudShipAuthResult struct {
	APIKey       string
	Email        string
	Organization string
}

// HasCloudShipAuth checks if CloudShip authentication is already configured
func HasCloudShipAuth() bool {
	// Check environment variables
	if os.Getenv("STN_CLOUDSHIP_KEY") != "" || os.Getenv("CLOUDSHIPAI_REGISTRATION_KEY") != "" {
		return true
	}
	if os.Getenv("CLOUDSHIP_API_KEY") != "" {
		return true
	}
	// Check viper config
	if viper.GetString("cloudship.api_key") != "" {
		return true
	}
	if viper.GetString("cloudship.registration_key") != "" {
		return true
	}
	return false
}

// RunCloudShipAuthFlow runs the CloudShip authentication flow and returns the result
// This is designed to be called from stn init when cloudshipai provider is selected
func RunCloudShipAuthFlow() (*CloudShipAuthResult, error) {
	fmt.Println()
	fmt.Println("🌩️  CloudShip AI Authentication")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("To use CloudShip AI, you need an API key.")
	fmt.Println("Get your API key from: https://app.cloudshipai.com/webapp/settings/")
	fmt.Println()

	// Prompt for API key
	fmt.Print("Enter your CloudShip API key: ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}
	apiKey := strings.TrimSpace(input)

	if apiKey == "" {
		return nil, fmt.Errorf("API key is required for CloudShip AI provider")
	}

	// Validate the API key
	fmt.Println("Validating API key...")

	userInfo, err := validateAPIKey(apiKey)
	if err != nil {
		return nil, fmt.Errorf("invalid API key: %w", err)
	}

	// Save to config
	viper.Set("cloudship.api_key", apiKey)
	viper.Set("cloudship.enabled", true)

	fmt.Println()
	fmt.Println("✅ Successfully authenticated with CloudShip!")
	if userInfo.Email != "" && userInfo.Email != "authenticated" {
		fmt.Printf("   Email: %s\n", userInfo.Email)
	}
	if userInfo.Organization.Name != "" {
		fmt.Printf("   Organization: %s\n", userInfo.Organization.Name)
	}
	fmt.Println()

	return &CloudShipAuthResult{
		APIKey:       apiKey,
		Email:        userInfo.Email,
		Organization: userInfo.Organization.Name,
	}, nil
}
