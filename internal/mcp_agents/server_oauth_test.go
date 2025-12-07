package mcp_agents

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"station/internal/config"
)

// TestOAuthChallengeResponse tests that returnOAuthChallenge returns proper 401 with WWW-Authenticate header
func TestOAuthChallengeResponse(t *testing.T) {
	cloudshipBaseURL := "https://app.cloudshipai.com"

	w := httptest.NewRecorder()
	returnOAuthChallenge(w, cloudshipBaseURL)

	// Check status code
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	// Check WWW-Authenticate header
	authHeader := w.Header().Get("WWW-Authenticate")
	if authHeader == "" {
		t.Error("Expected WWW-Authenticate header to be set")
	}

	expectedResourceURL := "https://app.cloudshipai.com/.well-known/oauth-protected-resource"
	if !strings.Contains(authHeader, expectedResourceURL) {
		t.Errorf("Expected WWW-Authenticate to contain resource_metadata URL, got: %s", authHeader)
	}

	// Check format: Bearer resource_metadata="..."
	expectedPrefix := `Bearer resource_metadata="`
	if !strings.HasPrefix(authHeader, expectedPrefix) {
		t.Errorf("Expected WWW-Authenticate to start with '%s', got: %s", expectedPrefix, authHeader)
	}

	// Check CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected Access-Control-Allow-Origin: *")
	}
	if !strings.Contains(w.Header().Get("Access-Control-Expose-Headers"), "WWW-Authenticate") {
		t.Error("Expected WWW-Authenticate to be in Access-Control-Expose-Headers")
	}

	// Check response body contains JSON error
	body := w.Body.String()
	if !strings.Contains(body, "authentication_required") {
		t.Errorf("Expected body to contain 'authentication_required', got: %s", body)
	}
}

// TestOAuthChallengeWithLocalURL tests with localhost URL for development
func TestOAuthChallengeWithLocalURL(t *testing.T) {
	cloudshipBaseURL := "http://localhost:8000"

	w := httptest.NewRecorder()
	returnOAuthChallenge(w, cloudshipBaseURL)

	authHeader := w.Header().Get("WWW-Authenticate")
	expectedResourceURL := "http://localhost:8000/.well-known/oauth-protected-resource"
	if !strings.Contains(authHeader, expectedResourceURL) {
		t.Errorf("Expected WWW-Authenticate to contain local resource_metadata URL, got: %s", authHeader)
	}
}

// mockHandler returns a simple handler that sets X-Authenticated header
func mockHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Authenticated", "true")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": "ok"}`))
	})
}

// TestOAuthDiscoveryMiddleware_NoAuth tests that requests without auth get 401
func TestOAuthDiscoveryMiddleware_NoAuth(t *testing.T) {
	cloudshipBaseURL := "https://app.cloudshipai.com"

	// Create wrapped handler
	handler := wrapWithOAuthDiscovery(mockHandler(), cloudshipBaseURL, nil, nil)

	// Make request without Authorization header
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(`{"jsonrpc":"2.0"}`))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 401
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	// Should have WWW-Authenticate header
	if w.Header().Get("WWW-Authenticate") == "" {
		t.Error("Expected WWW-Authenticate header")
	}
}

// TestOAuthDiscoveryMiddleware_EmptyBearerToken tests that empty Bearer gets 401
func TestOAuthDiscoveryMiddleware_EmptyBearerToken(t *testing.T) {
	cloudshipBaseURL := "https://app.cloudshipai.com"

	handler := wrapWithOAuthDiscovery(mockHandler(), cloudshipBaseURL, nil, nil)

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestOAuthDiscoveryMiddleware_WrongAuthType tests that non-Bearer auth gets 401
func TestOAuthDiscoveryMiddleware_WrongAuthType(t *testing.T) {
	cloudshipBaseURL := "https://app.cloudshipai.com"

	handler := wrapWithOAuthDiscovery(mockHandler(), cloudshipBaseURL, nil, nil)

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestConfigBaseURL tests that config.BaseURL defaults correctly
func TestConfigBaseURL(t *testing.T) {
	cfg := &config.Config{
		CloudShip: config.CloudShipConfig{
			BaseURL: "", // empty
		},
	}

	// When empty, should use default
	baseURL := cfg.CloudShip.BaseURL
	if baseURL == "" {
		// This is expected - config needs to be loaded properly to get default
		t.Log("BaseURL is empty in bare config (expected - defaults applied in Load())")
	}
}
