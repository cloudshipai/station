package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/firebase/genkit/go/ai"
)

// HistoryStore persists message history for sessions.
// This enables REPL-style prolonged conversations with the agent.
type HistoryStore struct {
	basePath string
	mu       sync.RWMutex
}

// StoredMessage is a serializable version of ai.Message
type StoredMessage struct {
	Role      string                 `json:"role"`
	Content   string                 `json:"content"`
	ToolCalls []StoredToolCall       `json:"tool_calls,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// StoredToolCall is a serializable version of tool call data
type StoredToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
	Result    string                 `json:"result,omitempty"`
}

// SessionHistory contains all messages for a session
type SessionHistory struct {
	SessionID   string           `json:"session_id"`
	Messages    []StoredMessage  `json:"messages"`
	TotalTokens int              `json:"total_tokens"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// NewHistoryStore creates a new history store
func NewHistoryStore(basePath string) *HistoryStore {
	return &HistoryStore{
		basePath: basePath,
	}
}

// historyPath returns the path for a session's history file
func (h *HistoryStore) historyPath(sessionID string) string {
	return filepath.Join(h.basePath, "session", sessionID, ".history.json")
}

// Load loads message history for a session
func (h *HistoryStore) Load(sessionID string) (*SessionHistory, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	path := h.historyPath(sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty history for new sessions
			return &SessionHistory{
				SessionID: sessionID,
				Messages:  []StoredMessage{},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("failed to read history: %w", err)
	}

	var history SessionHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("failed to parse history: %w", err)
	}

	return &history, nil
}

// Save persists message history for a session
func (h *HistoryStore) Save(sessionID string, history *SessionHistory) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	history.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	path := h.historyPath(sessionID)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write history: %w", err)
	}

	return nil
}

// Append adds messages to a session's history
func (h *HistoryStore) Append(sessionID string, messages []StoredMessage) error {
	history, err := h.Load(sessionID)
	if err != nil {
		return err
	}

	history.Messages = append(history.Messages, messages...)
	return h.Save(sessionID, history)
}

// Clear removes all messages from a session's history
func (h *HistoryStore) Clear(sessionID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	return os.Remove(h.historyPath(sessionID))
}

// ToAIMessages converts stored messages back to ai.Message format
func (h *SessionHistory) ToAIMessages() []*ai.Message {
	messages := make([]*ai.Message, len(h.Messages))
	for i, msg := range h.Messages {
		var parts []*ai.Part
		if msg.Content != "" {
			parts = append(parts, ai.NewTextPart(msg.Content))
		}

		// Note: Tool calls would need to be reconstructed from StoredToolCall
		// This is a simplified conversion

		messages[i] = &ai.Message{
			Role:    ai.Role(msg.Role),
			Content: parts,
		}
	}
	return messages
}

// FromAIMessage converts an ai.Message to StoredMessage
func FromAIMessage(msg *ai.Message) StoredMessage {
	stored := StoredMessage{
		Role:      string(msg.Role),
		Timestamp: time.Now(),
	}

	// Extract text content
	for _, part := range msg.Content {
		if part.IsText() {
			stored.Content += part.Text
		}
		// TODO: Handle tool request parts
	}

	return stored
}

// FromAIMessages converts multiple ai.Messages to StoredMessages
func FromAIMessages(msgs []*ai.Message) []StoredMessage {
	stored := make([]StoredMessage, len(msgs))
	for i, msg := range msgs {
		stored[i] = FromAIMessage(msg)
	}
	return stored
}
