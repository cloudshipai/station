// Package provider handles AI provider authentication (OAuth, API keys)
package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"station/internal/config"
)

// ProviderType identifies the AI provider
type ProviderType string

const (
	ProviderAnthropic ProviderType = "anthropic"
	ProviderOpenAI    ProviderType = "openai"
	ProviderGoogle    ProviderType = "google"
)

// AuthType identifies the authentication method
type AuthType string

const (
	AuthTypeAPIKey AuthType = "api_key"
	AuthTypeOAuth  AuthType = "oauth"
)

// ProviderCredentials stores authentication credentials for an AI provider
type ProviderCredentials struct {
	Provider     ProviderType `json:"provider"`
	AuthType     AuthType     `json:"auth_type"`
	APIKey       string       `json:"api_key,omitempty"`       // For API key auth or OAuth-derived key
	AccessToken  string       `json:"access_token,omitempty"`  // For OAuth (if we need refresh)
	RefreshToken string       `json:"refresh_token,omitempty"` // For OAuth refresh
	ExpiresAt    *time.Time   `json:"expires_at,omitempty"`    // Token expiration
	Email        string       `json:"email,omitempty"`         // User email from OAuth
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// ProviderAuthStore manages provider credentials storage
type ProviderAuthStore struct {
	Providers map[ProviderType]*ProviderCredentials `json:"providers"`
}

// authFilePath returns the path to the provider auth file
func authFilePath() (string, error) {
	configRoot := config.GetConfigRoot()
	return filepath.Join(configRoot, "provider_auth.json"), nil
}

// LoadProviderAuth loads the provider auth store from disk
func LoadProviderAuth() (*ProviderAuthStore, error) {
	filePath, err := authFilePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get auth file path: %w", err)
	}

	// If file doesn't exist, return empty store
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &ProviderAuthStore{
			Providers: make(map[ProviderType]*ProviderCredentials),
		}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read auth file: %w", err)
	}

	var store ProviderAuthStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse auth file: %w", err)
	}

	if store.Providers == nil {
		store.Providers = make(map[ProviderType]*ProviderCredentials)
	}

	return &store, nil
}

// Save persists the provider auth store to disk
func (s *ProviderAuthStore) Save() error {
	filePath, err := authFilePath()
	if err != nil {
		return fmt.Errorf("failed to get auth file path: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create auth directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal auth store: %w", err)
	}

	// Write with restrictive permissions (owner read/write only)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write auth file: %w", err)
	}

	return nil
}

// SetCredentials stores credentials for a provider
func (s *ProviderAuthStore) SetCredentials(creds *ProviderCredentials) error {
	creds.UpdatedAt = time.Now()
	if creds.CreatedAt.IsZero() {
		creds.CreatedAt = creds.UpdatedAt
	}
	s.Providers[creds.Provider] = creds
	return s.Save()
}

// GetCredentials retrieves credentials for a provider
func (s *ProviderAuthStore) GetCredentials(provider ProviderType) *ProviderCredentials {
	return s.Providers[provider]
}

// RemoveCredentials removes credentials for a provider
func (s *ProviderAuthStore) RemoveCredentials(provider ProviderType) error {
	delete(s.Providers, provider)
	return s.Save()
}

// HasCredentials checks if credentials exist for a provider
func (s *ProviderAuthStore) HasCredentials(provider ProviderType) bool {
	creds := s.Providers[provider]
	if creds == nil {
		return false
	}
	return creds.APIKey != "" || creds.AccessToken != ""
}

// GetAPIKey returns the API key for a provider (from direct API key auth)
func (s *ProviderAuthStore) GetAPIKey(provider ProviderType) string {
	creds := s.Providers[provider]
	if creds == nil {
		return ""
	}
	return creds.APIKey
}

// IsOAuth returns true if the provider uses OAuth authentication
func (s *ProviderAuthStore) IsOAuth(provider ProviderType) bool {
	creds := s.Providers[provider]
	return creds != nil && creds.AuthType == AuthTypeOAuth
}

// GetAccessToken returns the OAuth access token for a provider
func (s *ProviderAuthStore) GetAccessToken(provider ProviderType) string {
	creds := s.Providers[provider]
	if creds == nil {
		return ""
	}
	return creds.AccessToken
}

// IsExpired checks if OAuth credentials are expired
func (c *ProviderCredentials) IsExpired() bool {
	if c.ExpiresAt == nil {
		return false // No expiration set, assume valid
	}
	// Add 5 minute buffer before expiry
	return time.Now().Add(5 * time.Minute).After(*c.ExpiresAt)
}
