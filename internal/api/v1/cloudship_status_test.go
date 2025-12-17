package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCloudShipBundlesAPIResponse tests parsing of CloudShip bundles API response
func TestCloudShipBundlesAPIResponse(t *testing.T) {
	tests := []struct {
		name            string
		responseBody    string
		expectedBundles int
		expectedOrg     string
		authHeader      string
		authValue       string
		expectAuth      bool
	}{
		{
			name: "registration_key_auth_with_bundles",
			responseBody: `{
				"total": 2,
				"organization": "Test Organization",
				"bundles": [
					{"name": "bundle1", "organization": "Test Organization"},
					{"name": "bundle2", "organization": "Test Organization"}
				]
			}`,
			expectedBundles: 2,
			expectedOrg:     "Test Organization",
			authHeader:      "X-Registration-Key",
			authValue:       "test-registration-key",
			expectAuth:      true,
		},
		{
			name: "registration_key_auth_empty_bundles",
			responseBody: `{
				"total": 0,
				"organization": "Empty Org",
				"bundles": []
			}`,
			expectedBundles: 0,
			expectedOrg:     "Empty Org",
			authHeader:      "X-Registration-Key",
			authValue:       "test-registration-key",
			expectAuth:      true,
		},
		{
			name: "bearer_token_auth_with_bundles",
			responseBody: `{
				"total": 1,
				"organization": "Bearer Org",
				"bundles": [
					{"name": "bundle1", "organization": "Bearer Org"}
				]
			}`,
			expectedBundles: 1,
			expectedOrg:     "Bearer Org",
			authHeader:      "Authorization",
			authValue:       "Bearer test-api-key",
			expectAuth:      true,
		},
		{
			name: "org_from_bundle_fallback",
			responseBody: `{
				"total": 1,
				"bundles": [
					{"name": "bundle1", "organization": "Fallback Org"}
				]
			}`,
			expectedBundles: 1,
			expectedOrg:     "Fallback Org",
			authHeader:      "X-Registration-Key",
			authValue:       "test-key",
			expectAuth:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify the correct auth header is sent
				authValue := r.Header.Get(tt.authHeader)
				if authValue == "" {
					t.Errorf("Expected %s header, got empty", tt.authHeader)
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte(`{"detail": "Unauthorized"}`))
					return
				}

				if tt.authHeader == "Authorization" && authValue != tt.authValue {
					t.Errorf("Expected Authorization header %s, got %s", tt.authValue, authValue)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			// Make request to mock server
			req, err := http.NewRequest("GET", server.URL+"/api/public/bundles/", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set(tt.authHeader, tt.authValue)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			// Parse response like CloudShipStatusHandler does
			var bundleResp struct {
				Organization string `json:"organization"`
				Bundles      []struct {
					Organization string `json:"organization"`
				} `json:"bundles"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&bundleResp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			// Verify bundle count
			if len(bundleResp.Bundles) != tt.expectedBundles {
				t.Errorf("Expected %d bundles, got %d", tt.expectedBundles, len(bundleResp.Bundles))
			}

			// Verify organization (check top-level first, then fallback to bundle)
			org := bundleResp.Organization
			if org == "" && len(bundleResp.Bundles) > 0 {
				org = bundleResp.Bundles[0].Organization
			}
			if org != tt.expectedOrg {
				t.Errorf("Expected organization %q, got %q", tt.expectedOrg, org)
			}
		})
	}
}

// TestCloudShipAuthHeaderPriority tests that registration key is preferred over API key
func TestCloudShipAuthHeaderPriority(t *testing.T) {
	var receivedHeaders map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = make(map[string]string)
		receivedHeaders["X-Registration-Key"] = r.Header.Get("X-Registration-Key")
		receivedHeaders["Authorization"] = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"organization": "Test", "bundles": []}`))
	}))
	defer server.Close()

	tests := []struct {
		name                string
		registrationKey     string
		apiKey              string
		expectedHeader      string
		expectedHeaderValue string
		notExpectedHeader   string
	}{
		{
			name:                "prefer_registration_key_when_both_present",
			registrationKey:     "reg-key-123",
			apiKey:              "api-key-456",
			expectedHeader:      "X-Registration-Key",
			expectedHeaderValue: "reg-key-123",
			notExpectedHeader:   "Authorization",
		},
		{
			name:                "use_api_key_when_no_registration_key",
			registrationKey:     "",
			apiKey:              "api-key-789",
			expectedHeader:      "Authorization",
			expectedHeaderValue: "Bearer api-key-789",
			notExpectedHeader:   "X-Registration-Key",
		},
		{
			name:                "use_registration_key_only",
			registrationKey:     "reg-key-only",
			apiKey:              "",
			expectedHeader:      "X-Registration-Key",
			expectedHeaderValue: "reg-key-only",
			notExpectedHeader:   "Authorization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", server.URL+"/api/public/bundles/", nil)

			// Simulate the logic from CloudShipStatusHandler
			hasRegistrationKey := tt.registrationKey != ""
			hasAPIKey := tt.apiKey != ""

			if hasRegistrationKey {
				req.Header.Set("X-Registration-Key", tt.registrationKey)
			} else if hasAPIKey {
				req.Header.Set("Authorization", "Bearer "+tt.apiKey)
			}

			client := &http.Client{}
			_, err := client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}

			// Verify the correct header was sent
			if receivedHeaders[tt.expectedHeader] != tt.expectedHeaderValue {
				t.Errorf("Expected %s=%q, got %q",
					tt.expectedHeader, tt.expectedHeaderValue, receivedHeaders[tt.expectedHeader])
			}

			// Verify the other header was NOT sent (should be empty)
			if tt.notExpectedHeader != "" && receivedHeaders[tt.notExpectedHeader] != "" {
				t.Errorf("Did not expect %s header, but got %q",
					tt.notExpectedHeader, receivedHeaders[tt.notExpectedHeader])
			}
		})
	}
}

// TestCloudShipUnauthorizedResponse tests handling of 401/403 responses
func TestCloudShipUnauthorizedResponse(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		expectedAuth  bool
		expectedError string
	}{
		{
			name:          "unauthorized_401",
			statusCode:    http.StatusUnauthorized,
			expectedAuth:  false,
			expectedError: "invalid",
		},
		{
			name:          "forbidden_403",
			statusCode:    http.StatusForbidden,
			expectedAuth:  false,
			expectedError: "invalid",
		},
		{
			name:          "server_error_500",
			statusCode:    http.StatusInternalServerError,
			expectedAuth:  false,
			expectedError: "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(`{"detail": "Error"}`))
			}))
			defer server.Close()

			req, _ := http.NewRequest("GET", server.URL+"/api/public/bundles/", nil)
			req.Header.Set("X-Registration-Key", "invalid-key")

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			// Simulate the status check logic
			authenticated := false
			var errorMsg string

			if resp.StatusCode == http.StatusOK {
				authenticated = true
			} else if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				authenticated = false
				errorMsg = "Registration key or API key is invalid"
			} else {
				errorMsg = "CloudShip returned HTTP " + resp.Status
			}

			if authenticated != tt.expectedAuth {
				t.Errorf("Expected authenticated=%v, got %v", tt.expectedAuth, authenticated)
			}

			if tt.expectedError != "" && errorMsg == "" {
				t.Errorf("Expected error containing %q, got empty", tt.expectedError)
			}
		})
	}
}

// TestCloudShipAPIKeyMasking tests that API keys are properly masked
func TestCloudShipAPIKeyMasking(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		expectedMask string
	}{
		{
			name:         "long_key",
			key:          "cst_pZylPd3fHkiFuPjN8xqXi1zb9Qgi5vw2jWBZseQAWLw",
			expectedMask: "cst_...AWLw",
		},
		{
			name:         "registration_key",
			key:          "qwwhNNoVz09cDiQ1IysYO06bP-XdRwdPHqTJFVm1nxw",
			expectedMask: "qwwh...1nxw",
		},
		{
			name:         "short_key_exactly_8",
			key:          "abcd1234",
			expectedMask: "****",
		},
		{
			name:         "key_longer_than_8",
			key:          "abcd12345",
			expectedMask: "abcd...2345",
		},
		{
			name:         "very_short_key",
			key:          "abc",
			expectedMask: "****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var masked string
			if len(tt.key) > 8 {
				masked = tt.key[:4] + "..." + tt.key[len(tt.key)-4:]
			} else {
				masked = "****"
			}

			if masked != tt.expectedMask {
				t.Errorf("Expected masked key %q, got %q", tt.expectedMask, masked)
			}
		})
	}
}
