package lighthouse

import (
	"context"
	"fmt"
	"sync"
	"time"

	"station/internal/lighthouse/proto"
	"station/internal/logging"
)

// MemoryClient handles memory context requests via the ManagementChannel.
// It sends GetMemoryContext requests to Lighthouse which proxies to Django's memory API.
type MemoryClient struct {
	mu              sync.Mutex
	stream          proto.LighthouseService_ManagementChannelClient
	registrationKey string
	timeout         time.Duration

	// Response channels for async request handling
	responseChannels map[string]chan *proto.GetMemoryContextResponse
	responseMu       sync.RWMutex
}

// NewMemoryClient creates a new memory client.
// The stream and registrationKey can be set later using SetStream.
func NewMemoryClient(timeout time.Duration) *MemoryClient {
	if timeout == 0 {
		timeout = 2 * time.Second // Default 2 second timeout per PRD
	}
	return &MemoryClient{
		timeout:          timeout,
		responseChannels: make(map[string]chan *proto.GetMemoryContextResponse),
	}
}

// SetStream sets the active management channel stream for making requests.
// This should be called when the ManagementChannel connects.
func (mc *MemoryClient) SetStream(stream proto.LighthouseService_ManagementChannelClient, registrationKey string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.stream = stream
	mc.registrationKey = registrationKey
}

// ClearStream clears the active stream (called on disconnect).
func (mc *MemoryClient) ClearStream() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.stream = nil
}

// IsConnected returns true if a stream is available for memory requests.
func (mc *MemoryClient) IsConnected() bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return mc.stream != nil
}

// GetMemoryContext fetches memory context from CloudShip via the ManagementChannel.
// This is called by the execution engine when an agent has a memory_topic_key configured.
//
// Flow:
// 1. Station sends GetMemoryContextRequest via ManagementChannel
// 2. Lighthouse proxies to Django's memory API
// 3. Django queries mem0 and returns relevant memories
// 4. Context is returned for injection into agent prompt
//
// Returns empty string on error (never fails agent run due to memory issues)
func (mc *MemoryClient) GetMemoryContext(ctx context.Context, topicKey string, maxTokens int) (string, error) {
	mc.mu.Lock()
	stream := mc.stream
	registrationKey := mc.registrationKey
	mc.mu.Unlock()

	if stream == nil {
		return "", fmt.Errorf("no active management channel")
	}

	if topicKey == "" {
		return "", fmt.Errorf("topic_key is required")
	}

	// Use default max tokens if not specified
	if maxTokens <= 0 {
		maxTokens = 2000
	}

	// Generate unique request ID
	requestID := fmt.Sprintf("memory_%d", time.Now().UnixNano())

	// Create request message
	req := &proto.ManagementMessage{
		RequestId:       requestID,
		RegistrationKey: registrationKey,
		IsResponse:      false,
		Success:         false,
		Message: &proto.ManagementMessage_GetMemoryContextRequest{
			GetMemoryContextRequest: &proto.GetMemoryContextRequest{
				RegistrationKey: registrationKey,
				TopicKey:        topicKey,
				MaxTokens:       int32(maxTokens),
				Environment:     "",
			},
		},
	}

	// Create response channel
	responseChan := make(chan *proto.GetMemoryContextResponse, 1)
	mc.responseMu.Lock()
	mc.responseChannels[requestID] = responseChan
	mc.responseMu.Unlock()

	// Ensure cleanup
	defer func() {
		mc.responseMu.Lock()
		delete(mc.responseChannels, requestID)
		mc.responseMu.Unlock()
	}()

	logging.Debug("Sending GetMemoryContext request: topic_key=%s, max_tokens=%d, request_id=%s",
		topicKey, maxTokens, requestID)

	// Send request
	if err := stream.Send(req); err != nil {
		return "", fmt.Errorf("failed to send GetMemoryContext request: %w", err)
	}

	// Wait for response with timeout
	select {
	case resp := <-responseChan:
		if resp == nil {
			return "", fmt.Errorf("received nil response")
		}
		if !resp.Success {
			logging.Info("GetMemoryContext failed: %s", resp.ErrorMessage)
			return "", fmt.Errorf("memory fetch failed: %s", resp.ErrorMessage)
		}
		logging.Debug("GetMemoryContext succeeded: topic_key=%s, token_count=%d",
			resp.TopicKey, resp.TokenCount)
		return resp.Context, nil

	case <-time.After(mc.timeout):
		logging.Info("GetMemoryContext timeout for topic_key=%s after %v", topicKey, mc.timeout)
		return "", fmt.Errorf("timeout waiting for memory context")

	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// HandleResponse processes a GetMemoryContextResponse from Lighthouse.
// This should be called by the ManagementChannel message receiver.
func (mc *MemoryClient) HandleResponse(requestID string, resp *proto.GetMemoryContextResponse) {
	mc.responseMu.RLock()
	ch, ok := mc.responseChannels[requestID]
	mc.responseMu.RUnlock()

	if !ok {
		logging.Info("Received memory context response for unknown request: %s", requestID)
		return
	}

	// Send response to waiting goroutine (non-blocking)
	select {
	case ch <- resp:
		logging.Debug("Delivered memory context response for request: %s", requestID)
	default:
		logging.Info("Failed to deliver memory context response (channel full): %s", requestID)
	}
}
