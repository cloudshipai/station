package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"station/internal/config"
	"station/internal/lighthouse"
	"station/internal/logging"
)

// MemoryService handles CloudShip memory context retrieval
// Supports both CloudShip API (via ManagementChannel), direct API calls, and local file fallback
type MemoryService struct {
	// For CloudShip integration via ManagementChannel (serve/stdio mode)
	memoryClient *lighthouse.MemoryClient

	// For CloudShip integration via direct API calls (CLI mode)
	memoryAPIClient *lighthouse.MemoryAPIClient

	// Local fallback cache
	localCache     map[string]*cachedMemory
	localCacheMu   sync.RWMutex
	cacheExpiry    time.Duration
	workspaceDir   string
	maxTokens      int // Default max tokens if not specified
}

// cachedMemory represents cached memory context
type cachedMemory struct {
	content   string
	loadedAt  time.Time
	tokenCount int
}

// MemoryContext represents the result of fetching memory
type MemoryContext struct {
	Content     string    // Memory context content
	TokenCount  int       // Approximate token count
	TopicKey    string    // The topic key used
	LastUpdated time.Time // When the memory was last updated
	Source      string    // "cloudship" or "local"
	Error       error     // Any error that occurred
}

// NewMemoryService creates a new memory service
func NewMemoryService() *MemoryService {
	cfg, err := config.Load()
	workspaceDir := "."
	if err == nil && cfg.Workspace != "" {
		workspaceDir = cfg.Workspace
	}

	return &MemoryService{
		localCache:   make(map[string]*cachedMemory),
		cacheExpiry:  5 * time.Minute, // Cache local memory for 5 minutes
		workspaceDir: workspaceDir,
		maxTokens:    2000, // Default max tokens
	}
}

// NewMemoryServiceWithClient creates a memory service with CloudShip integration
func NewMemoryServiceWithClient(memoryClient *lighthouse.MemoryClient) *MemoryService {
	ms := NewMemoryService()
	ms.memoryClient = memoryClient
	return ms
}

// SetMemoryClient sets the memory client for CloudShip integration
// This allows setting the client after construction (e.g., when ManagementChannel connects)
func (ms *MemoryService) SetMemoryClient(memoryClient *lighthouse.MemoryClient) {
	ms.memoryClient = memoryClient
}

// SetMemoryAPIClient sets the direct API memory client for CloudShip integration (CLI mode)
func (ms *MemoryService) SetMemoryAPIClient(apiClient *lighthouse.MemoryAPIClient) {
	ms.memoryAPIClient = apiClient
}

// GetMemoryContext retrieves memory context for a given topic key
// First tries CloudShip (if connected), then falls back to local file
func (ms *MemoryService) GetMemoryContext(ctx context.Context, topicKey string, maxTokens int) *MemoryContext {
	if topicKey == "" {
		return &MemoryContext{
			TopicKey: topicKey,
			Error:    fmt.Errorf("memory topic key is empty"),
		}
	}

	// Use default max tokens if not specified
	if maxTokens <= 0 {
		maxTokens = ms.maxTokens
	}

	// Try CloudShip via management channel first (serve/stdio mode)
	if ms.memoryClient != nil && ms.memoryClient.IsConnected() {
		logging.Debug("Attempting CloudShip memory fetch via management channel for topic '%s'", topicKey)
		content, err := ms.memoryClient.GetMemoryContext(ctx, topicKey, maxTokens)
		if err != nil {
			// CloudShip fetch failed - log and fall back to local
			logging.Info("CloudShip memory fetch failed for topic '%s': %v, falling back to local", topicKey, err)
		} else if content != "" {
			// Got content from CloudShip - use it
			approxTokens := len(content) / 4
			logging.Debug("Successfully fetched memory from CloudShip for topic '%s': %d chars, ~%d tokens", topicKey, len(content), approxTokens)
			return &MemoryContext{
				Content:     content,
				TokenCount:  approxTokens,
				TopicKey:    topicKey,
				LastUpdated: time.Now(), // CloudShip doesn't return last_updated in current implementation
				Source:      "cloudship",
			}
		} else {
			// CloudShip returned empty - fall back to local file
			logging.Info("CloudShip returned empty memory for topic '%s', falling back to local file", topicKey)
		}
	}

	// Try CloudShip via direct API calls (CLI mode)
	if ms.memoryAPIClient != nil && ms.memoryAPIClient.IsConnected() {
		logging.Debug("Attempting CloudShip memory fetch via direct API for topic '%s'", topicKey)
		content, err := ms.memoryAPIClient.GetMemoryContext(ctx, topicKey, maxTokens)
		if err != nil {
			// Direct API fetch failed - log and fall back to local
			logging.Info("CloudShip direct API memory fetch failed for topic '%s': %v, falling back to local", topicKey, err)
		} else if content != "" {
			// Got content from CloudShip API - use it
			approxTokens := len(content) / 4
			logging.Info("Successfully fetched memory from CloudShip API for topic '%s': %d chars, ~%d tokens", topicKey, len(content), approxTokens)
			return &MemoryContext{
				Content:     content,
				TokenCount:  approxTokens,
				TopicKey:    topicKey,
				LastUpdated: time.Now(),
				Source:      "cloudship-api",
			}
		} else {
			// CloudShip API returned empty - fall back to local file
			logging.Info("CloudShip API returned empty memory for topic '%s', falling back to local file", topicKey)
		}
	}

	// Fall back to local file
	return ms.getLocalMemory(ctx, topicKey, maxTokens)
}

// getLocalMemory retrieves memory from local file system
// File path: {workspace}/memory/{topic_key}/context.md
func (ms *MemoryService) getLocalMemory(ctx context.Context, topicKey string, maxTokens int) *MemoryContext {
	// Check cache first
	ms.localCacheMu.RLock()
	cached, exists := ms.localCache[topicKey]
	ms.localCacheMu.RUnlock()

	if exists && time.Since(cached.loadedAt) < ms.cacheExpiry {
		logging.Debug("Using cached memory for topic '%s' (cached %v ago)", topicKey, time.Since(cached.loadedAt))
		return &MemoryContext{
			Content:     cached.content,
			TokenCount:  cached.tokenCount,
			TopicKey:    topicKey,
			LastUpdated: cached.loadedAt,
			Source:      "local_cache",
		}
	}

	// Build file path: memory/{topic_key}/context.md
	memoryFilePath := filepath.Join(ms.workspaceDir, "memory", topicKey, "context.md")

	// Check if file exists
	fileInfo, err := os.Stat(memoryFilePath)
	if os.IsNotExist(err) {
		logging.Debug("No local memory file found for topic '%s' at %s", topicKey, memoryFilePath)
		return &MemoryContext{
			Content:     "", // Empty string, not an error - memory simply doesn't exist yet
			TokenCount:  0,
			TopicKey:    topicKey,
			LastUpdated: time.Time{},
			Source:      "local_empty",
		}
	}
	if err != nil {
		return &MemoryContext{
			TopicKey: topicKey,
			Error:    fmt.Errorf("failed to stat memory file: %w", err),
		}
	}

	// Read file content
	content, err := os.ReadFile(memoryFilePath)
	if err != nil {
		return &MemoryContext{
			TopicKey: topicKey,
			Error:    fmt.Errorf("failed to read memory file: %w", err),
		}
	}

	contentStr := string(content)

	// Truncate if needed to stay within token limits
	// Rough approximation: 1 token ≈ 4 characters for English text
	approxTokens := len(contentStr) / 4
	if approxTokens > maxTokens {
		// Truncate content to fit within token limit
		maxChars := maxTokens * 4
		if maxChars < len(contentStr) {
			contentStr = contentStr[:maxChars]
			// Try to truncate at a word boundary
			lastSpace := strings.LastIndex(contentStr, " ")
			if lastSpace > maxChars-100 { // Don't go back too far
				contentStr = contentStr[:lastSpace]
			}
			contentStr += "\n\n[Memory truncated to fit token limit]"
		}
		approxTokens = maxTokens
	}

	// Update cache
	ms.localCacheMu.Lock()
	ms.localCache[topicKey] = &cachedMemory{
		content:    contentStr,
		loadedAt:   time.Now(),
		tokenCount: approxTokens,
	}
	ms.localCacheMu.Unlock()

	logging.Debug("Loaded local memory for topic '%s': %d chars, ~%d tokens", topicKey, len(contentStr), approxTokens)

	return &MemoryContext{
		Content:     contentStr,
		TokenCount:  approxTokens,
		TopicKey:    topicKey,
		LastUpdated: fileInfo.ModTime(),
		Source:      "local",
	}
}

// InvalidateCache clears the cache for a specific topic or all topics
func (ms *MemoryService) InvalidateCache(topicKey string) {
	ms.localCacheMu.Lock()
	defer ms.localCacheMu.Unlock()

	if topicKey == "" {
		// Clear all cache
		ms.localCache = make(map[string]*cachedMemory)
		logging.Debug("Cleared all memory cache")
	} else {
		delete(ms.localCache, topicKey)
		logging.Debug("Cleared memory cache for topic '%s'", topicKey)
	}
}

// WriteLocalMemory writes memory context to local file (for testing/local mode)
func (ms *MemoryService) WriteLocalMemory(topicKey, content string) error {
	if topicKey == "" {
		return fmt.Errorf("topic key cannot be empty")
	}

	// Build file path: memory/{topic_key}/context.md
	memoryDir := filepath.Join(ms.workspaceDir, "memory", topicKey)
	memoryFilePath := filepath.Join(memoryDir, "context.md")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	// Write content
	if err := os.WriteFile(memoryFilePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write memory file: %w", err)
	}

	// Invalidate cache
	ms.InvalidateCache(topicKey)

	logging.Info("Wrote local memory for topic '%s' to %s", topicKey, memoryFilePath)
	return nil
}
