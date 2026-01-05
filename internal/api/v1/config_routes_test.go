package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// setupConfigTestRouter creates a minimal test router for config endpoints
func setupConfigTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create a minimal handler for config routes
	h := &APIHandlers{localMode: true}

	configGroup := router.Group("/api/v1/config")
	h.registerConfigRoutes(configGroup)

	return router
}

func TestGetConfigSchema(t *testing.T) {
	router := setupConfigTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/config/schema", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response struct {
		Sections []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Order       int    `json:"order"`
		} `json:"sections"`
		Fields []struct {
			Key         string   `json:"key"`
			Type        string   `json:"type"`
			Description string   `json:"description"`
			Section     string   `json:"section"`
			Options     []string `json:"options,omitempty"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify sections exist
	if len(response.Sections) == 0 {
		t.Error("Expected sections to be non-empty")
	}

	// Verify fields exist
	if len(response.Fields) == 0 {
		t.Error("Expected fields to be non-empty")
	}

	// Verify some expected fields
	expectedFields := []string{"workspace", "ai_provider", "ai_model", "coding.backend"}
	fieldKeys := make(map[string]bool)
	for _, f := range response.Fields {
		fieldKeys[f.Key] = true
	}

	for _, expected := range expectedFields {
		if !fieldKeys[expected] {
			t.Errorf("Expected field %q in schema", expected)
		}
	}

	for _, f := range response.Fields {
		if f.Key == "ai_provider" {
			if len(f.Options) == 0 {
				t.Error("Expected ai_provider to have options")
			}
			break
		}
	}
}

func TestStartConfigSession(t *testing.T) {
	router := setupConfigTestRouter()

	// Create temp config file
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `workspace: /test/workspace
ai_provider: openai
ai_model: gpt-4
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Start session
	reqBody := map[string]string{"config_path": configPath}
	reqBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/api/v1/config/session", bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		SessionID  string `json:"session_id"`
		ConfigPath string `json:"config_path"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.SessionID == "" {
		t.Error("Expected session_id to be non-empty")
	}
	if response.ConfigPath != configPath {
		t.Errorf("Expected config_path %q, got %q", configPath, response.ConfigPath)
	}
}

func TestGetConfigSession(t *testing.T) {
	router := setupConfigTestRouter()

	// Create temp config file
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `workspace: /test/workspace
ai_provider: anthropic
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Start a session first
	reqBody := map[string]string{"config_path": configPath}
	reqBytes, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/config/session", bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var startResponse struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &startResponse); err != nil {
		t.Fatalf("Failed to parse start response: %v", err)
	}

	// Get the session
	req, _ = http.NewRequest("GET", "/api/v1/config/session/"+startResponse.SessionID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var session ConfigSession
	if err := json.Unmarshal(w.Body.Bytes(), &session); err != nil {
		t.Fatalf("Failed to parse session response: %v", err)
	}

	if session.ID != startResponse.SessionID {
		t.Errorf("Expected session ID %q, got %q", startResponse.SessionID, session.ID)
	}
	if session.Status != "waiting" {
		t.Errorf("Expected status 'waiting', got %q", session.Status)
	}
	if session.ConfigPath != configPath {
		t.Errorf("Expected config path %q, got %q", configPath, session.ConfigPath)
	}

	// Verify values were loaded
	if session.Values["workspace"] != "/test/workspace" {
		t.Errorf("Expected workspace '/test/workspace', got %v", session.Values["workspace"])
	}
	if session.Values["ai_provider"] != "anthropic" {
		t.Errorf("Expected ai_provider 'anthropic', got %v", session.Values["ai_provider"])
	}
}

func TestGetConfigSession_NotFound(t *testing.T) {
	router := setupConfigTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/config/session/nonexistent-id", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestSaveConfigSession(t *testing.T) {
	router := setupConfigTestRouter()

	// Create temp config file
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `workspace: /old/workspace
ai_provider: openai
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Start a session
	startReq := map[string]string{"config_path": configPath}
	startBytes, _ := json.Marshal(startReq)
	req, _ := http.NewRequest("POST", "/api/v1/config/session", bytes.NewReader(startBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var startResponse struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &startResponse); err != nil {
		t.Fatalf("Failed to parse start response: %v", err)
	}

	// Save new values
	newValues := map[string]interface{}{
		"workspace":   "/new/workspace",
		"ai_provider": "anthropic",
		"ai_model":    "claude-3-opus",
	}
	saveReq := map[string]interface{}{"values": newValues}
	saveBytes, _ := json.Marshal(saveReq)

	req, _ = http.NewRequest("POST", "/api/v1/config/session/"+startResponse.SessionID+"/save", bytes.NewReader(saveBytes))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var saveResponse struct {
		Success    bool   `json:"success"`
		ConfigPath string `json:"config_path"`
		Message    string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &saveResponse); err != nil {
		t.Fatalf("Failed to parse save response: %v", err)
	}

	if !saveResponse.Success {
		t.Error("Expected success to be true")
	}

	// Verify file was written
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var savedValues map[string]interface{}
	if err := yaml.Unmarshal(content, &savedValues); err != nil {
		t.Fatalf("Failed to parse saved config: %v", err)
	}

	if savedValues["workspace"] != "/new/workspace" {
		t.Errorf("Expected workspace '/new/workspace', got %v", savedValues["workspace"])
	}
	if savedValues["ai_provider"] != "anthropic" {
		t.Errorf("Expected ai_provider 'anthropic', got %v", savedValues["ai_provider"])
	}
	if savedValues["ai_model"] != "claude-3-opus" {
		t.Errorf("Expected ai_model 'claude-3-opus', got %v", savedValues["ai_model"])
	}
}

func TestSaveConfigSession_NotFound(t *testing.T) {
	router := setupConfigTestRouter()

	saveReq := map[string]interface{}{"values": map[string]interface{}{"test": "value"}}
	saveBytes, _ := json.Marshal(saveReq)

	req, _ := http.NewRequest("POST", "/api/v1/config/session/nonexistent/save", bytes.NewReader(saveBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestCancelConfigSession(t *testing.T) {
	router := setupConfigTestRouter()

	// Create temp config file
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("workspace: /test"), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Start a session
	startReq := map[string]string{"config_path": configPath}
	startBytes, _ := json.Marshal(startReq)
	req, _ := http.NewRequest("POST", "/api/v1/config/session", bytes.NewReader(startBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var startResponse struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &startResponse); err != nil {
		t.Fatalf("Failed to parse start response: %v", err)
	}

	// Cancel the session
	req, _ = http.NewRequest("DELETE", "/api/v1/config/session/"+startResponse.SessionID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify session is gone
	req, _ = http.NewRequest("GET", "/api/v1/config/session/"+startResponse.SessionID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 after cancel, got %d", w.Code)
	}
}

func TestCancelConfigSession_NotFound(t *testing.T) {
	router := setupConfigTestRouter()

	req, _ := http.NewRequest("DELETE", "/api/v1/config/session/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestStartConfigSession_CreatesNewFile(t *testing.T) {
	router := setupConfigTestRouter()

	// Create temp directory but no config file
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "new-config.yaml")

	// Start session with non-existent file path
	reqBody := map[string]string{"config_path": configPath}
	reqBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/api/v1/config/session", bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		SessionID  string `json:"session_id"`
		ConfigPath string `json:"config_path"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Get session to verify empty values
	req, _ = http.NewRequest("GET", "/api/v1/config/session/"+response.SessionID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var session ConfigSession
	if err := json.Unmarshal(w.Body.Bytes(), &session); err != nil {
		t.Fatalf("Failed to parse session response: %v", err)
	}

	// Values should be empty for new file
	if len(session.Values) != 0 {
		t.Errorf("Expected empty values for new file, got %v", session.Values)
	}

	// Now save values to create the file
	newValues := map[string]interface{}{
		"workspace":   "/new/workspace",
		"ai_provider": "openai",
	}
	saveReq := map[string]interface{}{"values": newValues}
	saveBytes, _ := json.Marshal(saveReq)

	req, _ = http.NewRequest("POST", "/api/v1/config/session/"+response.SessionID+"/save", bytes.NewReader(saveBytes))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected config file to be created")
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var savedValues map[string]interface{}
	if err := yaml.Unmarshal(content, &savedValues); err != nil {
		t.Fatalf("Failed to parse saved config: %v", err)
	}

	if savedValues["workspace"] != "/new/workspace" {
		t.Errorf("Expected workspace '/new/workspace', got %v", savedValues["workspace"])
	}
}

func TestConfigSessionExpiry(t *testing.T) {
	// Clean up any existing sessions
	configSessionsMu.Lock()
	configSessions = make(map[string]*ConfigSession)
	configSessionsMu.Unlock()

	// Create an expired session directly
	expiredSession := &ConfigSession{
		ID:         "expired-session",
		Status:     "waiting",
		ConfigPath: "/test/path",
		CreatedAt:  time.Now().Add(-20 * time.Minute),
		ExpiresAt:  time.Now().Add(-10 * time.Minute), // Expired 10 minutes ago
	}

	configSessionsMu.Lock()
	configSessions["expired-session"] = expiredSession
	configSessionsMu.Unlock()

	// Run cleanup
	cleanupExpiredConfigSessions()

	// Verify session was cleaned up
	configSessionsMu.RLock()
	_, exists := configSessions["expired-session"]
	configSessionsMu.RUnlock()

	if exists {
		t.Error("Expected expired session to be cleaned up")
	}
}

func TestConfigSessionStatusTransitions(t *testing.T) {
	router := setupConfigTestRouter()

	// Create temp config file
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("workspace: /test"), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Start a session - status should be "waiting"
	startReq := map[string]string{"config_path": configPath}
	startBytes, _ := json.Marshal(startReq)
	req, _ := http.NewRequest("POST", "/api/v1/config/session", bytes.NewReader(startBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var startResponse struct {
		SessionID string `json:"session_id"`
	}
	json.Unmarshal(w.Body.Bytes(), &startResponse)

	// Get session - verify waiting status
	req, _ = http.NewRequest("GET", "/api/v1/config/session/"+startResponse.SessionID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var session ConfigSession
	json.Unmarshal(w.Body.Bytes(), &session)
	if session.Status != "waiting" {
		t.Errorf("Expected status 'waiting' after start, got %q", session.Status)
	}

	// Save - status should transition to "completed"
	saveReq := map[string]interface{}{"values": map[string]interface{}{"test": "value"}}
	saveBytes, _ := json.Marshal(saveReq)
	req, _ = http.NewRequest("POST", "/api/v1/config/session/"+startResponse.SessionID+"/save", bytes.NewReader(saveBytes))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Get session again - verify completed status
	req, _ = http.NewRequest("GET", "/api/v1/config/session/"+startResponse.SessionID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &session)
	if session.Status != "completed" {
		t.Errorf("Expected status 'completed' after save, got %q", session.Status)
	}
}

func TestSaveConfigSession_InvalidJSON(t *testing.T) {
	router := setupConfigTestRouter()

	// Create temp config file
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("workspace: /test"), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Start a session
	startReq := map[string]string{"config_path": configPath}
	startBytes, _ := json.Marshal(startReq)
	req, _ := http.NewRequest("POST", "/api/v1/config/session", bytes.NewReader(startBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var startResponse struct {
		SessionID string `json:"session_id"`
	}
	json.Unmarshal(w.Body.Bytes(), &startResponse)

	// Try to save with invalid JSON
	req, _ = http.NewRequest("POST", "/api/v1/config/session/"+startResponse.SessionID+"/save", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid JSON, got %d", w.Code)
	}
}

func TestNestedConfigValues(t *testing.T) {
	router := setupConfigTestRouter()

	// Create temp config file with nested values
	tempDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `workspace: /test
coding:
  backend: claudecode
  claudecode:
    cli_path: /usr/bin/claude
cloudship:
  enabled: true
  endpoint: cloudship.ai
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Start session
	startReq := map[string]string{"config_path": configPath}
	startBytes, _ := json.Marshal(startReq)
	req, _ := http.NewRequest("POST", "/api/v1/config/session", bytes.NewReader(startBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var startResponse struct {
		SessionID string `json:"session_id"`
	}
	json.Unmarshal(w.Body.Bytes(), &startResponse)

	// Get session
	req, _ = http.NewRequest("GET", "/api/v1/config/session/"+startResponse.SessionID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var session ConfigSession
	json.Unmarshal(w.Body.Bytes(), &session)

	// Verify nested values
	coding, ok := session.Values["coding"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected coding to be a map, got %T", session.Values["coding"])
	}

	if coding["backend"] != "claudecode" {
		t.Errorf("Expected coding.backend 'claudecode', got %v", coding["backend"])
	}

	cloudship, ok := session.Values["cloudship"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected cloudship to be a map, got %T", session.Values["cloudship"])
	}

	if cloudship["enabled"] != true {
		t.Errorf("Expected cloudship.enabled true, got %v", cloudship["enabled"])
	}

	// Update nested values
	newValues := map[string]interface{}{
		"workspace": "/updated",
		"coding": map[string]interface{}{
			"backend": "aider",
			"aider": map[string]interface{}{
				"model": "gpt-4",
			},
		},
		"cloudship": map[string]interface{}{
			"enabled":  false,
			"endpoint": "new.cloudship.ai",
		},
	}
	saveReq := map[string]interface{}{"values": newValues}
	saveBytes, _ := json.Marshal(saveReq)

	req, _ = http.NewRequest("POST", "/api/v1/config/session/"+startResponse.SessionID+"/save", bytes.NewReader(saveBytes))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify saved file
	content, _ := os.ReadFile(configPath)
	var savedValues map[string]interface{}
	yaml.Unmarshal(content, &savedValues)

	savedCoding := savedValues["coding"].(map[string]interface{})
	if savedCoding["backend"] != "aider" {
		t.Errorf("Expected coding.backend 'aider', got %v", savedCoding["backend"])
	}

	savedCloudship := savedValues["cloudship"].(map[string]interface{})
	if savedCloudship["enabled"] != false {
		t.Errorf("Expected cloudship.enabled false, got %v", savedCloudship["enabled"])
	}
}
