package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"station/pkg/bundle"
)

// MockHTTPClient implements HTTPClient for testing
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

func TestHTTPRegistry_List(t *testing.T) {
	tests := []struct {
		name           string
		opts           bundle.ListOptions
		mockResponse   *http.Response
		mockError      error
		expectedURL    string
		expectedBundles int
		expectError    bool
	}{
		{
			name: "successful list with no filters",
			opts: bundle.ListOptions{},
			mockResponse: createJSONResponse(http.StatusOK, map[string]interface{}{
				"bundles": []map[string]interface{}{
					{
						"name":        "openai-assistant",
						"version":     "1.0.0",
						"description": "OpenAI assistant bundle",
						"author":      "Station Team",
					},
					{
						"name":        "github-tools",
						"version":     "2.1.0", 
						"description": "GitHub integration tools",
						"author":      "Station Team",
					},
				},
			}),
			expectedURL:     "https://registry.example.com/bundles",
			expectedBundles: 2,
			expectError:     false,
		},
		{
			name: "list with search filter",
			opts: bundle.ListOptions{Search: "openai"},
			mockResponse: createJSONResponse(http.StatusOK, map[string]interface{}{
				"bundles": []map[string]interface{}{
					{
						"name":        "openai-assistant",
						"version":     "1.0.0",
						"description": "OpenAI assistant bundle",
						"author":      "Station Team",
					},
				},
			}),
			expectedURL:     "https://registry.example.com/bundles?search=openai",
			expectedBundles: 1,
			expectError:     false,
		},
		{
			name: "list with tags filter",
			opts: bundle.ListOptions{Tags: []string{"ai", "llm"}},
			mockResponse: createJSONResponse(http.StatusOK, map[string]interface{}{
				"bundles": []map[string]interface{}{},
			}),
			expectedURL:     "https://registry.example.com/bundles?tags=ai%2Cllm",
			expectedBundles: 0,
			expectError:     false,
		},
		{
			name: "registry server error",
			opts: bundle.ListOptions{},
			mockResponse: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("Internal Server Error")),
			},
			expectedURL: "https://registry.example.com/bundles",
			expectError: true,
		},
		{
			name:         "network error",
			opts:         bundle.ListOptions{},
			mockError:    assert.AnError,
			expectedURL:  "https://registry.example.com/bundles",
			expectError:  true,
		},
		{
			name: "invalid JSON response",
			opts: bundle.ListOptions{},
			mockResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("invalid json")),
			},
			expectedURL: "https://registry.example.com/bundles",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockClient := new(MockHTTPClient)
			registry := NewHTTPRegistry("test-registry", "https://registry.example.com", nil)
			registry.SetHTTPClient(mockClient)

			// Setup expectations
			if tt.mockError != nil {
				mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.URL.String() == tt.expectedURL
				})).Return((*http.Response)(nil), tt.mockError)
			} else {
				mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.URL.String() == tt.expectedURL
				})).Return(tt.mockResponse, nil)
			}

			// Execute
			bundles, err := registry.List(context.Background(), tt.opts)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, bundles, tt.expectedBundles)
				if tt.expectedBundles > 0 {
					assert.NotEmpty(t, bundles[0].Name)
					assert.NotEmpty(t, bundles[0].Version)
				}
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestHTTPRegistry_Get(t *testing.T) {
	tests := []struct {
		name         string
		bundleName   string
		version      string
		mockResponse *http.Response
		mockError    error
		expectedURL  string
		expectError  bool
		checkBundle  func(t *testing.T, bundle *bundle.BundleManifest)
	}{
		{
			name:       "get latest version",
			bundleName: "openai-assistant",
			version:    "",
			mockResponse: createJSONResponse(http.StatusOK, map[string]interface{}{
				"name":        "openai-assistant",
				"version":     "1.0.0",
				"description": "OpenAI assistant bundle",
				"author":      "Station Team",
			}),
			expectedURL: "https://registry.example.com/bundles/openai-assistant",
			expectError: false,
			checkBundle: func(t *testing.T, bundle *bundle.BundleManifest) {
				assert.Equal(t, "openai-assistant", bundle.Name)
				assert.Equal(t, "1.0.0", bundle.Version)
			},
		},
		{
			name:       "get specific version",
			bundleName: "openai-assistant",
			version:    "1.2.0",
			mockResponse: createJSONResponse(http.StatusOK, map[string]interface{}{
				"name":        "openai-assistant",
				"version":     "1.2.0",
				"description": "OpenAI assistant bundle",
				"author":      "Station Team",
			}),
			expectedURL: "https://registry.example.com/bundles/openai-assistant/1.2.0",
			expectError: false,
			checkBundle: func(t *testing.T, bundle *bundle.BundleManifest) {
				assert.Equal(t, "1.2.0", bundle.Version)
			},
		},
		{
			name:       "bundle not found",
			bundleName: "nonexistent-bundle",
			version:    "",
			mockResponse: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("Not Found")),
			},
			expectedURL: "https://registry.example.com/bundles/nonexistent-bundle",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockClient := new(MockHTTPClient)
			registry := NewHTTPRegistry("test-registry", "https://registry.example.com", nil)
			registry.SetHTTPClient(mockClient)

			// Setup expectations
			if tt.mockError != nil {
				mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return((*http.Response)(nil), tt.mockError)
			} else {
				mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.URL.String() == tt.expectedURL
				})).Return(tt.mockResponse, nil)
			}

			// Execute
			bundle, err := registry.Get(context.Background(), tt.bundleName, tt.version)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, bundle)
			} else {
				require.NoError(t, err)
				require.NotNil(t, bundle)
				if tt.checkBundle != nil {
					tt.checkBundle(t, bundle)
				}
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestHTTPRegistry_Download(t *testing.T) {
	tests := []struct {
		name         string
		bundleName   string
		version      string
		mockResponse *http.Response
		mockError    error
		expectedURL  string
		expectedData []byte
		expectError  bool
	}{
		{
			name:       "successful download",
			bundleName: "openai-assistant",
			version:    "1.0.0",
			mockResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("mock-bundle-data"))),
			},
			expectedURL:  "https://registry.example.com/bundles/openai-assistant/download?version=1.0.0",
			expectedData: []byte("mock-bundle-data"),
			expectError:  false,
		},
		{
			name:       "download latest version",
			bundleName: "github-tools",
			version:    "",
			mockResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("latest-bundle-data"))),
			},
			expectedURL:  "https://registry.example.com/bundles/github-tools/download",
			expectedData: []byte("latest-bundle-data"),
			expectError:  false,
		},
		{
			name:       "bundle not found",
			bundleName: "missing-bundle",
			version:    "",
			mockResponse: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("Not Found")),
			},
			expectedURL: "https://registry.example.com/bundles/missing-bundle/download",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockClient := new(MockHTTPClient)
			registry := NewHTTPRegistry("test-registry", "https://registry.example.com", nil)
			registry.SetHTTPClient(mockClient)

			// Setup expectations
			if tt.mockError != nil {
				mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return((*http.Response)(nil), tt.mockError)
			} else {
				mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.URL.String() == tt.expectedURL
				})).Return(tt.mockResponse, nil)
			}

			// Execute
			data, err := registry.Download(context.Background(), tt.bundleName, tt.version)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, data)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedData, data)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestHTTPRegistry_GetVersions(t *testing.T) {
	tests := []struct {
		name             string
		bundleName       string
		mockResponse     *http.Response
		mockError        error
		expectedVersions []string
		expectError      bool
	}{
		{
			name:       "successful versions list",
			bundleName: "openai-assistant",
			mockResponse: createJSONResponse(http.StatusOK, map[string]interface{}{
				"versions": []string{"1.0.0", "1.1.0", "1.2.0"},
			}),
			expectedVersions: []string{"1.0.0", "1.1.0", "1.2.0"},
			expectError:      false,
		},
		{
			name:       "bundle not found",
			bundleName: "missing-bundle",
			mockResponse: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("Not Found")),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockClient := new(MockHTTPClient)
			registry := NewHTTPRegistry("test-registry", "https://registry.example.com", nil)
			registry.SetHTTPClient(mockClient)

			expectedURL := "https://registry.example.com/bundles/" + tt.bundleName + "/versions"

			// Setup expectations
			if tt.mockError != nil {
				mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return((*http.Response)(nil), tt.mockError)
			} else {
				mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.URL.String() == expectedURL
				})).Return(tt.mockResponse, nil)
			}

			// Execute
			versions, err := registry.GetVersions(context.Background(), tt.bundleName)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedVersions, versions)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestHTTPRegistry_Authentication(t *testing.T) {
	tests := []struct {
		name           string
		auth           map[string]string
		checkAuthHeader func(t *testing.T, req *http.Request)
	}{
		{
			name: "API key authentication",
			auth: map[string]string{"api_key": "test-api-key"},
			checkAuthHeader: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "Bearer test-api-key", req.Header.Get("Authorization"))
			},
		},
		{
			name: "Token authentication",
			auth: map[string]string{"token": "test-token"},
			checkAuthHeader: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "Token test-token", req.Header.Get("Authorization"))
			},
		},
		{
			name: "Basic authentication",
			auth: map[string]string{"username": "user", "password": "pass"},
			checkAuthHeader: func(t *testing.T, req *http.Request) {
				username, password, ok := req.BasicAuth()
				assert.True(t, ok)
				assert.Equal(t, "user", username)
				assert.Equal(t, "pass", password)
			},
		},
		{
			name: "No authentication",
			auth: map[string]string{},
			checkAuthHeader: func(t *testing.T, req *http.Request) {
				assert.Empty(t, req.Header.Get("Authorization"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockClient := new(MockHTTPClient)
			registry := NewHTTPRegistry("test-registry", "https://registry.example.com", tt.auth)
			registry.SetHTTPClient(mockClient)

			mockResponse := createJSONResponse(http.StatusOK, map[string]interface{}{
				"bundles": []map[string]interface{}{},
			})

			// Setup expectations with request inspection
			mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				// Check authentication headers
				tt.checkAuthHeader(t, req)
				
				// Check other headers
				assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
				assert.Equal(t, "station-bundle-client/1.0", req.Header.Get("User-Agent"))
				
				return true
			})).Return(mockResponse, nil)

			// Execute
			_, err := registry.List(context.Background(), bundle.ListOptions{})

			// Assert
			require.NoError(t, err)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestHTTPRegistry_ContextCancellation(t *testing.T) {
	mockClient := new(MockHTTPClient)
	registry := NewHTTPRegistry("test-registry", "https://registry.example.com", nil)
	registry.SetHTTPClient(mockClient)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Setup mock expectation for cancelled context
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(
		(*http.Response)(nil), 
		context.Canceled,
	)

	// The request should fail due to cancelled context
	_, err := registry.List(ctx, bundle.ListOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
	
	mockClient.AssertExpectations(t)
}

// Helper functions

func createJSONResponse(statusCode int, data interface{}) *http.Response {
	jsonData, _ := json.Marshal(data)
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader(jsonData)),
		Header:     make(http.Header),
	}
}