package v1

import (
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"station/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type ConfigSession struct {
	ID         string                 `json:"id"`
	Status     string                 `json:"status"`
	ConfigPath string                 `json:"config_path"`
	Values     map[string]interface{} `json:"values,omitempty"`
	Error      string                 `json:"error,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	ExpiresAt  time.Time              `json:"expires_at"`
}

var (
	configSessions   = make(map[string]*ConfigSession)
	configSessionsMu sync.RWMutex
)

func (h *APIHandlers) registerConfigRoutes(group *gin.RouterGroup) {
	group.GET("/schema", h.getConfigSchema)
	group.POST("/session", h.startConfigSession)
	group.GET("/session/:id", h.getConfigSession)
	group.POST("/session/:id/save", h.saveConfigSession)
	group.DELETE("/session/:id", h.cancelConfigSession)
	group.GET("/values", h.getConfigValues)
}

func (h *APIHandlers) getConfigSchema(c *gin.Context) {
	sections := config.GetConfigSections()
	fields := config.GetConfigSchema()

	c.JSON(http.StatusOK, gin.H{
		"sections": sections,
		"fields":   fields,
	})
}

func (h *APIHandlers) startConfigSession(c *gin.Context) {
	var req struct {
		ConfigPath string `json:"config_path"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	configPath := req.ConfigPath
	if configPath == "" {
		configPath = viper.ConfigFileUsed()
	}
	if configPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No config path specified and no config file in use"})
		return
	}

	var values map[string]interface{}
	content, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read config file"})
			return
		}
		values = make(map[string]interface{})
	} else {
		if err := yaml.Unmarshal(content, &values); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse config file"})
			return
		}
	}

	sessionID := uuid.New().String()
	session := &ConfigSession{
		ID:         sessionID,
		Status:     "waiting",
		ConfigPath: configPath,
		Values:     values,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}

	configSessionsMu.Lock()
	configSessions[sessionID] = session
	configSessionsMu.Unlock()

	go cleanupExpiredConfigSessions()

	c.JSON(http.StatusOK, gin.H{
		"session_id":  sessionID,
		"config_path": configPath,
	})
}

func (h *APIHandlers) getConfigSession(c *gin.Context) {
	sessionID := c.Param("id")

	configSessionsMu.RLock()
	session, ok := configSessions[sessionID]
	configSessionsMu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, session)
}

func (h *APIHandlers) saveConfigSession(c *gin.Context) {
	sessionID := c.Param("id")

	configSessionsMu.Lock()
	session, ok := configSessions[sessionID]
	if !ok {
		configSessionsMu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}
	session.Status = "saving"
	configSessionsMu.Unlock()

	var req struct {
		Values map[string]interface{} `json:"values"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		configSessionsMu.Lock()
		session.Status = "failed"
		session.Error = "Invalid request body"
		configSessionsMu.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	output, err := yaml.Marshal(req.Values)
	if err != nil {
		configSessionsMu.Lock()
		session.Status = "failed"
		session.Error = "Failed to serialize config"
		configSessionsMu.Unlock()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize config"})
		return
	}

	if err := os.MkdirAll(filepath.Dir(session.ConfigPath), 0755); err != nil {
		configSessionsMu.Lock()
		session.Status = "failed"
		session.Error = "Failed to create config directory"
		configSessionsMu.Unlock()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create config directory"})
		return
	}

	if err := os.WriteFile(session.ConfigPath, output, 0644); err != nil {
		configSessionsMu.Lock()
		session.Status = "failed"
		session.Error = "Failed to write config file"
		configSessionsMu.Unlock()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write config file"})
		return
	}

	configSessionsMu.Lock()
	session.Status = "completed"
	session.Values = req.Values
	configSessionsMu.Unlock()

	go func() {
		time.Sleep(5 * time.Second)
		configSessionsMu.Lock()
		delete(configSessions, sessionID)
		configSessionsMu.Unlock()
	}()

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"config_path": session.ConfigPath,
		"message":     "Configuration saved. Restart Station to apply changes.",
	})
}

func (h *APIHandlers) cancelConfigSession(c *gin.Context) {
	sessionID := c.Param("id")

	configSessionsMu.Lock()
	session, ok := configSessions[sessionID]
	if ok {
		session.Status = "cancelled"
		delete(configSessions, sessionID)
	}
	configSessionsMu.Unlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *APIHandlers) getConfigValues(c *gin.Context) {
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "No config file in use"})
		return
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read config file"})
		return
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(content, &values); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse config file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config_path": configPath,
		"values":      values,
	})
}

func cleanupExpiredConfigSessions() {
	configSessionsMu.Lock()
	defer configSessionsMu.Unlock()

	now := time.Now()
	for id, session := range configSessions {
		if now.After(session.ExpiresAt) {
			delete(configSessions, id)
		}
	}
}
