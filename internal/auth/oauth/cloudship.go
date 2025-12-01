// Package oauth provides CloudShip OAuth integration for Station MCP authentication.
//
// This package enables Station to authenticate MCP clients using CloudShip OAuth tokens.
// When OAuth is enabled, clients can authenticate by:
// 1. Obtaining an OAuth token from CloudShip (authorization_code flow)
// 2. Using the token as a Bearer token when calling Station MCP endpoints
// 3. Station validates the token by calling CloudShip's introspect endpoint
//
// Configuration (in config.yaml):
//
//	cloudship:
//	  oauth:
//	    enabled: true
//	    client_id: "your-client-id"
//	    auth_url: "https://app.cloudshipai.com/oauth/authorize/"
//	    token_url: "https://app.cloudshipai.com/oauth/token/"
//	    introspect_url: "https://app.cloudshipai.com/oauth/introspect/"
package oauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"station/internal/config"
)

// CloudShipOAuth handles OAuth token validation with CloudShip
type CloudShipOAuth struct {
	config *config.OAuthConfig
	client *http.Client
	// Token cache to avoid repeated introspection calls
	cache     map[string]*TokenInfo
	cacheMu   sync.RWMutex
	cacheTTL  time.Duration
}

// TokenInfo holds information about a validated OAuth token
type TokenInfo struct {
	Active    bool      `json:"active"`
	UserID    string    `json:"user_id,omitempty"`
	Username  string    `json:"username,omitempty"`
	Email     string    `json:"email,omitempty"`
	OrgID     string    `json:"org_id,omitempty"`
	Scope     string    `json:"scope,omitempty"`
	ClientID  string    `json:"client_id,omitempty"`
	ExpiresAt time.Time `json:"exp,omitempty"`
	// Cache metadata
	CachedAt  time.Time `json:"-"`
}

// IntrospectionResponse is the response from CloudShip's introspect endpoint
type IntrospectionResponse struct {
	Active    bool   `json:"active"`
	Scope     string `json:"scope,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	Username  string `json:"username,omitempty"`
	TokenType string `json:"token_type,omitempty"`
	Exp       int64  `json:"exp,omitempty"`
	Iat       int64  `json:"iat,omitempty"`
	Nbf       int64  `json:"nbf,omitempty"`
	Sub       string `json:"sub,omitempty"`
	Aud       string `json:"aud,omitempty"`
	Iss       string `json:"iss,omitempty"`
	// CloudShip-specific claims
	UserID string `json:"user_id,omitempty"`
	Email  string `json:"email,omitempty"`
	OrgID  string `json:"org_id,omitempty"`
}

// NewCloudShipOAuth creates a new CloudShip OAuth handler
func NewCloudShipOAuth(cfg *config.OAuthConfig) *CloudShipOAuth {
	return &CloudShipOAuth{
		config: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache:    make(map[string]*TokenInfo),
		cacheTTL: 5 * time.Minute, // Cache valid tokens for 5 minutes
	}
}

// ValidateToken validates an OAuth token by calling CloudShip's introspect endpoint
func (o *CloudShipOAuth) ValidateToken(token string) (*TokenInfo, error) {
	if token == "" {
		return nil, fmt.Errorf("empty token")
	}

	// Check cache first
	if info := o.getCachedToken(token); info != nil {
		return info, nil
	}

	// Call introspect endpoint
	info, err := o.introspect(token)
	if err != nil {
		return nil, fmt.Errorf("token introspection failed: %w", err)
	}

	if !info.Active {
		return nil, fmt.Errorf("token is not active")
	}

	// Cache the result
	o.cacheToken(token, info)

	return info, nil
}

// introspect calls CloudShip's introspect endpoint to validate a token
func (o *CloudShipOAuth) introspect(token string) (*TokenInfo, error) {
	// Build request body
	data := url.Values{}
	data.Set("token", token)

	req, err := http.NewRequest("POST", o.config.IntrospectURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	// For public clients, we don't need client credentials
	// For confidential clients, we would add Basic auth here
	if o.config.ClientID != "" {
		// Add client_id to the request for identification
		data.Set("client_id", o.config.ClientID)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("introspect request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("introspect returned status %d: %s", resp.StatusCode, string(body))
	}

	var introspectResp IntrospectionResponse
	if err := json.Unmarshal(body, &introspectResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to TokenInfo
	info := &TokenInfo{
		Active:   introspectResp.Active,
		UserID:   introspectResp.UserID,
		Username: introspectResp.Username,
		Email:    introspectResp.Email,
		OrgID:    introspectResp.OrgID,
		Scope:    introspectResp.Scope,
		ClientID: introspectResp.ClientID,
	}

	if introspectResp.Exp > 0 {
		info.ExpiresAt = time.Unix(introspectResp.Exp, 0)
	}

	// If UserID not in response, try sub claim
	if info.UserID == "" && introspectResp.Sub != "" {
		info.UserID = introspectResp.Sub
	}

	return info, nil
}

// getCachedToken retrieves a token from cache if still valid
func (o *CloudShipOAuth) getCachedToken(token string) *TokenInfo {
	o.cacheMu.RLock()
	defer o.cacheMu.RUnlock()

	info, ok := o.cache[token]
	if !ok {
		return nil
	}

	// Check if cache entry is still valid
	if time.Since(info.CachedAt) > o.cacheTTL {
		return nil
	}

	// Check if token itself is still valid
	if !info.ExpiresAt.IsZero() && time.Now().After(info.ExpiresAt) {
		return nil
	}

	return info
}

// cacheToken stores a validated token in cache
func (o *CloudShipOAuth) cacheToken(token string, info *TokenInfo) {
	o.cacheMu.Lock()
	defer o.cacheMu.Unlock()

	info.CachedAt = time.Now()
	o.cache[token] = info

	// Simple cache cleanup - remove expired entries
	for k, v := range o.cache {
		if time.Since(v.CachedAt) > o.cacheTTL {
			delete(o.cache, k)
		}
	}
}

// GetAuthorizationURL returns the URL to redirect users for OAuth authorization
func (o *CloudShipOAuth) GetAuthorizationURL(state, codeChallenge string) string {
	params := url.Values{}
	params.Set("client_id", o.config.ClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", o.config.RedirectURI)
	params.Set("scope", o.config.Scopes)
	params.Set("state", state)
	
	// PKCE support
	if codeChallenge != "" {
		params.Set("code_challenge", codeChallenge)
		params.Set("code_challenge_method", "S256")
	}

	return o.config.AuthURL + "?" + params.Encode()
}

// IsEnabled returns whether CloudShip OAuth is enabled
func (o *CloudShipOAuth) IsEnabled() bool {
	return o.config != nil && o.config.Enabled && o.config.ClientID != ""
}
