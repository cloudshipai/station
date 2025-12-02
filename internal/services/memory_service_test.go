package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewMemoryService(t *testing.T) {
	ms := NewMemoryService()

	if ms == nil {
		t.Fatal("NewMemoryService() returned nil")
	}

	if ms.localCache == nil {
		t.Error("localCache should be initialized")
	}

	if ms.maxTokens != 2000 {
		t.Errorf("Expected default maxTokens=2000, got %d", ms.maxTokens)
	}

	if ms.cacheExpiry != 5*time.Minute {
		t.Errorf("Expected cacheExpiry=5m, got %v", ms.cacheExpiry)
	}
}

func TestGetMemoryContext_EmptyTopicKey(t *testing.T) {
	ms := NewMemoryService()

	result := ms.GetMemoryContext(context.Background(), "", 2000)

	if result.Error == nil {
		t.Error("Expected error for empty topic key")
	}

	if result.TopicKey != "" {
		t.Errorf("Expected empty TopicKey, got '%s'", result.TopicKey)
	}
}

func TestGetMemoryContext_NonExistentFile(t *testing.T) {
	// Create temp directory for workspace
	tmpDir, err := os.MkdirTemp("", "memory_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ms := NewMemoryService()
	ms.workspaceDir = tmpDir

	result := ms.GetMemoryContext(context.Background(), "nonexistent-topic", 2000)

	// Should not error - just return empty content
	if result.Error != nil {
		t.Errorf("Unexpected error for non-existent file: %v", result.Error)
	}

	if result.Content != "" {
		t.Errorf("Expected empty content for non-existent file, got '%s'", result.Content)
	}

	if result.Source != "local_empty" {
		t.Errorf("Expected source='local_empty', got '%s'", result.Source)
	}

	if result.TopicKey != "nonexistent-topic" {
		t.Errorf("Expected TopicKey='nonexistent-topic', got '%s'", result.TopicKey)
	}
}

func TestGetMemoryContext_ValidFile(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "memory_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create memory/test-topic/context.md
	topicDir := filepath.Join(tmpDir, "memory", "test-topic")
	if err := os.MkdirAll(topicDir, 0755); err != nil {
		t.Fatalf("Failed to create topic dir: %v", err)
	}

	memoryContent := "# Test Memory\n\nThis is test memory content for the agent."
	memoryFile := filepath.Join(topicDir, "context.md")
	if err := os.WriteFile(memoryFile, []byte(memoryContent), 0644); err != nil {
		t.Fatalf("Failed to write memory file: %v", err)
	}

	ms := NewMemoryService()
	ms.workspaceDir = tmpDir

	result := ms.GetMemoryContext(context.Background(), "test-topic", 2000)

	if result.Error != nil {
		t.Errorf("Unexpected error: %v", result.Error)
	}

	if result.Content != memoryContent {
		t.Errorf("Expected content='%s', got '%s'", memoryContent, result.Content)
	}

	if result.Source != "local" {
		t.Errorf("Expected source='local', got '%s'", result.Source)
	}

	if result.TopicKey != "test-topic" {
		t.Errorf("Expected TopicKey='test-topic', got '%s'", result.TopicKey)
	}

	// Token count should be roughly len/4
	expectedTokens := len(memoryContent) / 4
	if result.TokenCount != expectedTokens {
		t.Errorf("Expected TokenCount~%d, got %d", expectedTokens, result.TokenCount)
	}
}

func TestGetMemoryContext_TokenTruncation(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "memory_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create memory/truncate-topic/context.md with large content
	topicDir := filepath.Join(tmpDir, "memory", "truncate-topic")
	if err := os.MkdirAll(topicDir, 0755); err != nil {
		t.Fatalf("Failed to create topic dir: %v", err)
	}

	// Create content that's ~1000 tokens (4000 chars)
	largeContent := ""
	for i := 0; i < 500; i++ {
		largeContent += "word "
	}

	memoryFile := filepath.Join(topicDir, "context.md")
	if err := os.WriteFile(memoryFile, []byte(largeContent), 0644); err != nil {
		t.Fatalf("Failed to write memory file: %v", err)
	}

	ms := NewMemoryService()
	ms.workspaceDir = tmpDir

	// Request only 100 tokens
	maxTokens := 100
	result := ms.GetMemoryContext(context.Background(), "truncate-topic", maxTokens)

	if result.Error != nil {
		t.Errorf("Unexpected error: %v", result.Error)
	}

	// Content should be truncated
	if len(result.Content) > maxTokens*4+100 { // Allow some margin for truncation message
		t.Errorf("Content not truncated properly: len=%d, expected max ~%d", len(result.Content), maxTokens*4)
	}

	// Should contain truncation notice
	if result.TokenCount != maxTokens {
		t.Errorf("TokenCount should be capped at maxTokens=%d, got %d", maxTokens, result.TokenCount)
	}
}

func TestGetMemoryContext_Caching(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "memory_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create memory file
	topicDir := filepath.Join(tmpDir, "memory", "cache-topic")
	if err := os.MkdirAll(topicDir, 0755); err != nil {
		t.Fatalf("Failed to create topic dir: %v", err)
	}

	initialContent := "Initial memory content"
	memoryFile := filepath.Join(topicDir, "context.md")
	if err := os.WriteFile(memoryFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to write memory file: %v", err)
	}

	ms := NewMemoryService()
	ms.workspaceDir = tmpDir

	// First request - should read from file
	result1 := ms.GetMemoryContext(context.Background(), "cache-topic", 2000)
	if result1.Error != nil {
		t.Errorf("First request error: %v", result1.Error)
	}
	if result1.Source != "local" {
		t.Errorf("First request should have source='local', got '%s'", result1.Source)
	}
	if result1.Content != initialContent {
		t.Errorf("First request content mismatch")
	}

	// Modify file content
	modifiedContent := "Modified memory content"
	if err := os.WriteFile(memoryFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("Failed to write modified memory file: %v", err)
	}

	// Second request - should read from cache (file change not detected)
	result2 := ms.GetMemoryContext(context.Background(), "cache-topic", 2000)
	if result2.Error != nil {
		t.Errorf("Second request error: %v", result2.Error)
	}
	if result2.Source != "local_cache" {
		t.Errorf("Second request should have source='local_cache', got '%s'", result2.Source)
	}
	// Should still have initial content (cached)
	if result2.Content != initialContent {
		t.Errorf("Second request should return cached content")
	}

	// Invalidate cache
	ms.InvalidateCache("cache-topic")

	// Third request - should read from file again
	result3 := ms.GetMemoryContext(context.Background(), "cache-topic", 2000)
	if result3.Error != nil {
		t.Errorf("Third request error: %v", result3.Error)
	}
	if result3.Source != "local" {
		t.Errorf("Third request should have source='local', got '%s'", result3.Source)
	}
	// Should now have modified content
	if result3.Content != modifiedContent {
		t.Errorf("Third request should return modified content")
	}
}

func TestWriteLocalMemory(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "memory_write_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ms := NewMemoryService()
	ms.workspaceDir = tmpDir

	// Write memory
	content := "# Written Memory\n\nThis was written via WriteLocalMemory."
	err = ms.WriteLocalMemory("write-topic", content)
	if err != nil {
		t.Errorf("WriteLocalMemory failed: %v", err)
	}

	// Verify file was created
	expectedPath := filepath.Join(tmpDir, "memory", "write-topic", "context.md")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Memory file not created at expected path: %s", expectedPath)
	}

	// Read back and verify content
	readContent, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Errorf("Failed to read written file: %v", err)
	}
	if string(readContent) != content {
		t.Errorf("Written content mismatch: got '%s', expected '%s'", string(readContent), content)
	}

	// Verify GetMemoryContext returns the written content
	result := ms.GetMemoryContext(context.Background(), "write-topic", 2000)
	if result.Error != nil {
		t.Errorf("GetMemoryContext after write error: %v", result.Error)
	}
	if result.Content != content {
		t.Errorf("GetMemoryContext content mismatch after write")
	}
}

func TestWriteLocalMemory_EmptyTopicKey(t *testing.T) {
	ms := NewMemoryService()

	err := ms.WriteLocalMemory("", "content")
	if err == nil {
		t.Error("Expected error for empty topic key")
	}
}

func TestInvalidateCache_All(t *testing.T) {
	ms := NewMemoryService()

	// Pre-populate cache
	ms.localCache["topic1"] = &cachedMemory{content: "content1", loadedAt: time.Now()}
	ms.localCache["topic2"] = &cachedMemory{content: "content2", loadedAt: time.Now()}
	ms.localCache["topic3"] = &cachedMemory{content: "content3", loadedAt: time.Now()}

	if len(ms.localCache) != 3 {
		t.Errorf("Expected 3 cached items, got %d", len(ms.localCache))
	}

	// Invalidate all (empty string = clear all)
	ms.InvalidateCache("")

	if len(ms.localCache) != 0 {
		t.Errorf("Expected 0 cached items after InvalidateCache(''), got %d", len(ms.localCache))
	}
}

func TestInvalidateCache_Specific(t *testing.T) {
	ms := NewMemoryService()

	// Pre-populate cache
	ms.localCache["topic1"] = &cachedMemory{content: "content1", loadedAt: time.Now()}
	ms.localCache["topic2"] = &cachedMemory{content: "content2", loadedAt: time.Now()}

	// Invalidate specific topic
	ms.InvalidateCache("topic1")

	if len(ms.localCache) != 1 {
		t.Errorf("Expected 1 cached item after specific invalidation, got %d", len(ms.localCache))
	}

	if _, exists := ms.localCache["topic2"]; !exists {
		t.Error("topic2 should still be in cache")
	}

	if _, exists := ms.localCache["topic1"]; exists {
		t.Error("topic1 should be removed from cache")
	}
}

func TestGetMemoryContext_DefaultMaxTokens(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "memory_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create memory file
	topicDir := filepath.Join(tmpDir, "memory", "default-tokens-topic")
	if err := os.MkdirAll(topicDir, 0755); err != nil {
		t.Fatalf("Failed to create topic dir: %v", err)
	}

	content := "Test content"
	memoryFile := filepath.Join(topicDir, "context.md")
	if err := os.WriteFile(memoryFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write memory file: %v", err)
	}

	ms := NewMemoryService()
	ms.workspaceDir = tmpDir

	// Request with maxTokens=0 should use default
	result := ms.GetMemoryContext(context.Background(), "default-tokens-topic", 0)

	if result.Error != nil {
		t.Errorf("Unexpected error: %v", result.Error)
	}

	if result.Content != content {
		t.Errorf("Content mismatch")
	}
}
