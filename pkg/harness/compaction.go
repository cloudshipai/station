package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// HistoryBackend interface for persisting conversation history
type HistoryBackend interface {
	WriteFile(path string, content []byte) error
	ReadFile(path string) ([]byte, error)
}

type Compactor struct {
	genkitApp      *genkit.Genkit
	modelName      string
	config         CompactionConfig
	tokenCounter   TokenCounter
	contextWindow  int
	historyBackend HistoryBackend // For history offload
	historyPrefix  string         // e.g., "/conversation_history"
	sessionID      string         // For unique history file names
}

type TokenCounter interface {
	CountTokens(messages []*ai.Message) (int, error)
}

type SimpleTokenCounter struct{}

func (s *SimpleTokenCounter) CountTokens(messages []*ai.Message) (int, error) {
	total := 0
	for _, msg := range messages {
		for _, part := range msg.Content {
			if part.IsText() {
				total += len(strings.Fields(part.Text)) * 4 / 3
			}
		}
	}
	return total, nil
}

func NewCompactor(genkitApp *genkit.Genkit, modelName string, config CompactionConfig, contextWindow int) *Compactor {
	return &Compactor{
		genkitApp:     genkitApp,
		modelName:     modelName,
		config:        config,
		tokenCounter:  &SimpleTokenCounter{},
		contextWindow: contextWindow,
	}
}

func (c *Compactor) WithTokenCounter(counter TokenCounter) *Compactor {
	c.tokenCounter = counter
	return c
}

func (c *Compactor) WithHistoryBackend(backend HistoryBackend, prefix, sessionID string) *Compactor {
	c.historyBackend = backend
	c.historyPrefix = prefix
	c.sessionID = sessionID
	return c
}

func (c *Compactor) CompactIfNeeded(ctx context.Context, history []*ai.Message) ([]*ai.Message, bool, error) {
	if !c.config.Enabled {
		return history, false, nil
	}

	currentTokens, err := c.tokenCounter.CountTokens(history)
	if err != nil {
		return history, false, fmt.Errorf("failed to count tokens: %w", err)
	}

	threshold := int(float64(c.contextWindow) * c.config.Threshold)
	if currentTokens < threshold {
		return history, false, nil
	}

	compacted, err := c.compact(ctx, history)
	if err != nil {
		return history, false, fmt.Errorf("compaction failed: %w", err)
	}

	return compacted, true, nil
}

func (c *Compactor) compact(ctx context.Context, history []*ai.Message) ([]*ai.Message, error) {
	if len(history) <= 2 {
		return history, nil
	}

	protectedTokens := c.config.ProtectTokens
	if protectedTokens <= 0 {
		protectedTokens = 40000
	}

	currentTokens, _ := c.tokenCounter.CountTokens(history)
	tokensToPreserve := 0
	preserveFromIdx := len(history)

	for i := len(history) - 1; i >= 0; i-- {
		msgTokens, _ := c.tokenCounter.CountTokens([]*ai.Message{history[i]})
		if tokensToPreserve+msgTokens > protectedTokens {
			preserveFromIdx = i + 1
			break
		}
		tokensToPreserve += msgTokens
	}

	if preserveFromIdx <= 1 {
		return history, nil
	}

	toSummarize := history[1:preserveFromIdx]
	if len(toSummarize) == 0 {
		return history, nil
	}

	// Offload history before summarization (preserves full data)
	var historyFile string
	if c.config.HistoryOffload {
		var err error
		historyFile, err = c.offloadHistory(toSummarize)
		if err != nil {
			// Log but don't fail - offload is optional
			fmt.Printf("Warning: failed to offload history: %v\n", err)
		}
	}

	summary, err := c.summarizeHistory(ctx, toSummarize)
	if err != nil {
		return history, err
	}

	compacted := make([]*ai.Message, 0, 2+len(history)-preserveFromIdx)

	if len(history) > 0 {
		compacted = append(compacted, history[0])
	}

	// Build summary message with optional file reference
	summaryMsg := fmt.Sprintf(
		"[CONTEXT COMPACTION: The following is a summary of %d previous messages totaling ~%d tokens]\n\n%s\n\n",
		len(toSummarize),
		currentTokens-tokensToPreserve,
		summary,
	)

	if historyFile != "" {
		summaryMsg += fmt.Sprintf("[Full history preserved at: %s]\n\n", historyFile)
	}

	summaryMsg += "[END SUMMARY - Continuing from recent context]"

	compacted = append(compacted, ai.NewUserTextMessage(summaryMsg))

	compacted = append(compacted, history[preserveFromIdx:]...)

	return compacted, nil
}

func (c *Compactor) summarizeHistory(ctx context.Context, messages []*ai.Message) (string, error) {
	if c.genkitApp == nil {
		return c.fallbackSummarize(messages), nil
	}

	var historyText strings.Builder
	for i, msg := range messages {
		historyText.WriteString(fmt.Sprintf("\n--- Message %d (%s) ---\n", i+1, msg.Role))
		for _, part := range msg.Content {
			if part.IsText() {
				historyText.WriteString(part.Text)
				historyText.WriteString("\n")
			} else if part.IsToolRequest() {
				historyText.WriteString(fmt.Sprintf("[Tool Call: %s]\n", part.ToolRequest.Name))
			} else if part.IsToolResponse() {
				historyText.WriteString(fmt.Sprintf("[Tool Response: %s]\n", part.ToolResponse.Name))
			}
		}
	}

	prompt := fmt.Sprintf(`Summarize the following conversation history concisely, preserving:
1. Key decisions made
2. Important information discovered
3. Tool calls and their outcomes
4. Any errors encountered and their resolutions

Be concise but comprehensive. Focus on information that would be needed to continue the task.

CONVERSATION HISTORY:
%s

SUMMARY:`, historyText.String())

	resp, err := genkit.Generate(ctx, c.genkitApp,
		ai.WithModelName(c.modelName),
		ai.WithPrompt(prompt),
	)
	if err != nil {
		return c.fallbackSummarize(messages), nil
	}

	return resp.Text(), nil
}

func (c *Compactor) fallbackSummarize(messages []*ai.Message) string {
	var summary strings.Builder
	summary.WriteString("Previous conversation summary:\n")

	toolCalls := make(map[string]int)
	for _, msg := range messages {
		for _, part := range msg.Content {
			if part.IsToolRequest() {
				toolCalls[part.ToolRequest.Name]++
			}
		}
	}

	if len(toolCalls) > 0 {
		summary.WriteString("\nTools used:\n")
		for tool, count := range toolCalls {
			summary.WriteString(fmt.Sprintf("- %s: %d times\n", tool, count))
		}
	}

	summary.WriteString(fmt.Sprintf("\n[%d messages compacted]\n", len(messages)))
	return summary.String()
}

func (c *Compactor) ShouldCompact(history []*ai.Message) (bool, int, error) {
	if !c.config.Enabled {
		return false, 0, nil
	}

	currentTokens, err := c.tokenCounter.CountTokens(history)
	if err != nil {
		return false, 0, err
	}

	threshold := int(float64(c.contextWindow) * c.config.Threshold)
	return currentTokens >= threshold, currentTokens, nil
}

// offloadHistory saves conversation history to the backend before summarization
// This preserves the full history for debugging and retrieval
func (c *Compactor) offloadHistory(messages []*ai.Message) (string, error) {
	if c.historyBackend == nil {
		return "", nil
	}

	// Create serializable structure for history
	type HistoryEntry struct {
		Timestamp time.Time `json:"timestamp"`
		SessionID string    `json:"session_id"`
		Messages  []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
			Tools   []struct {
				Name   string `json:"name"`
				Type   string `json:"type"` // "request" or "response"
				Input  any    `json:"input,omitempty"`
				Output any    `json:"output,omitempty"`
			} `json:"tools,omitempty"`
		} `json:"messages"`
	}

	entry := HistoryEntry{
		Timestamp: time.Now(),
		SessionID: c.sessionID,
	}

	for _, msg := range messages {
		msgEntry := struct {
			Role    string `json:"role"`
			Content string `json:"content"`
			Tools   []struct {
				Name   string `json:"name"`
				Type   string `json:"type"`
				Input  any    `json:"input,omitempty"`
				Output any    `json:"output,omitempty"`
			} `json:"tools,omitempty"`
		}{
			Role: string(msg.Role),
		}

		for _, part := range msg.Content {
			if part.IsText() {
				msgEntry.Content += part.Text
			} else if part.IsToolRequest() {
				msgEntry.Tools = append(msgEntry.Tools, struct {
					Name   string `json:"name"`
					Type   string `json:"type"`
					Input  any    `json:"input,omitempty"`
					Output any    `json:"output,omitempty"`
				}{
					Name:  part.ToolRequest.Name,
					Type:  "request",
					Input: part.ToolRequest.Input,
				})
			} else if part.IsToolResponse() {
				msgEntry.Tools = append(msgEntry.Tools, struct {
					Name   string `json:"name"`
					Type   string `json:"type"`
					Input  any    `json:"input,omitempty"`
					Output any    `json:"output,omitempty"`
				}{
					Name:   part.ToolResponse.Name,
					Type:   "response",
					Output: part.ToolResponse.Output,
				})
			}
		}

		entry.Messages = append(entry.Messages, msgEntry)
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal history: %w", err)
	}

	// Generate unique filename with timestamp
	filename := fmt.Sprintf("%s/%s_%s.json",
		c.historyPrefix,
		c.sessionID,
		time.Now().Format("20060102_150405"),
	)

	if err := c.historyBackend.WriteFile(filename, data); err != nil {
		return "", fmt.Errorf("failed to write history: %w", err)
	}

	return filename, nil
}

// TruncateOldArgs truncates large tool arguments in older messages to reduce token count
// This is useful for tools like write_file and edit_file that have large inputs
func (c *Compactor) TruncateOldArgs(messages []*ai.Message, maxArgLength int) []*ai.Message {
	if maxArgLength <= 0 {
		maxArgLength = 500 // Default max length for truncated args
	}

	// Don't modify the last few messages (keep them intact)
	preserveCount := 4
	if len(messages) <= preserveCount {
		return messages
	}

	result := make([]*ai.Message, len(messages))
	copy(result[len(messages)-preserveCount:], messages[len(messages)-preserveCount:])

	for i := 0; i < len(messages)-preserveCount; i++ {
		msg := messages[i]
		newContent := make([]*ai.Part, len(msg.Content))

		for j, part := range msg.Content {
			if part.IsToolRequest() {
				// Check if this is a tool with potentially large args
				toolName := part.ToolRequest.Name
				if shouldTruncateArgs(toolName) {
					// Create truncated version
					truncatedInput := truncateInput(part.ToolRequest.Input, maxArgLength)
					newPart := ai.NewToolRequestPart(&ai.ToolRequest{
						Name:  toolName,
						Input: truncatedInput,
					})
					newContent[j] = newPart
				} else {
					newContent[j] = part
				}
			} else {
				newContent[j] = part
			}
		}

		result[i] = &ai.Message{
			Role:    msg.Role,
			Content: newContent,
		}
	}

	return result
}

// shouldTruncateArgs returns true for tools that typically have large arguments
func shouldTruncateArgs(toolName string) bool {
	largeArgTools := map[string]bool{
		"write_file":      true,
		"edit_file":       true,
		"bash":            true,
		"create_file":     true,
		"update_file":     true,
		"patch_file":      true,
		"__write_file":    true,
		"__edit_file":     true,
		"__create_file":   true,
	}
	return largeArgTools[toolName]
}

// truncateInput truncates the input map values that are strings
func truncateInput(input any, maxLength int) any {
	if input == nil {
		return nil
	}

	switch v := input.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, val := range v {
			if str, ok := val.(string); ok && len(str) > maxLength {
				result[key] = str[:maxLength] + fmt.Sprintf("... [truncated %d chars]", len(str)-maxLength)
			} else {
				result[key] = val
			}
		}
		return result
	default:
		return input
	}
}
