package session

import (
	"encoding/json"
	"fmt"
)

// HistoryBuilder builds formatted history for prompts
type HistoryBuilder struct {
	events []*Event
}

// NewHistoryBuilder creates a new history builder
func NewHistoryBuilder(events []*Event) *HistoryBuilder {
	return &HistoryBuilder{events: events}
}

// BuildWriteHistoryPrompt formats write history for AI prompts
func (hb *HistoryBuilder) BuildWriteHistoryPrompt() string {
	if len(hb.events) == 0 {
		return "No previous write operations."
	}

	var prompt string
	prompt += "Previous Write Operations (in chronological order):\n\n"

	for i, event := range hb.events {
		argsJSON, _ := json.MarshalIndent(event.Arguments, "", "  ")
		responseJSON, _ := json.MarshalIndent(event.Response, "", "  ")

		prompt += fmt.Sprintf("%d. [%s] %s\n",
			i+1,
			event.Timestamp.Format("2006-01-02 15:04:05"),
			event.ToolName,
		)
		prompt += fmt.Sprintf("   Arguments: %s\n", string(argsJSON))
		prompt += fmt.Sprintf("   Response: %s\n\n", string(responseJSON))
	}

	return prompt
}

// BuildAllEventsPrompt formats all events (read and write) for AI prompts
func (hb *HistoryBuilder) BuildAllEventsPrompt() string {
	if len(hb.events) == 0 {
		return "No previous operations."
	}

	var prompt string
	prompt += "Previous Operations (chronological):\n\n"

	for i, event := range hb.events {
		argsJSON, _ := json.MarshalIndent(event.Arguments, "", "  ")
		responseJSON, _ := json.MarshalIndent(event.Response, "", "  ")

		prompt += fmt.Sprintf("%d. [%s] %s\n",
			i+1,
			event.OperationType,
			event.ToolName)
		prompt += fmt.Sprintf("   Arguments: %s\n", string(argsJSON))
		prompt += fmt.Sprintf("   Response: %s\n", string(responseJSON))
		prompt += fmt.Sprintf("   Timestamp: %s\n\n",
			event.Timestamp.Format("2006-01-02 15:04:05"))
	}

	return prompt
}

// BuildSummary creates a summary of operations
func (hb *HistoryBuilder) BuildSummary() string {
	if len(hb.events) == 0 {
		return "No operations recorded."
	}

	writeCount := 0
	readCount := 0
	toolCounts := make(map[string]int)

	for _, event := range hb.events {
		if event.OperationType == OperationWrite {
			writeCount++
		} else {
			readCount++
		}
		toolCounts[event.ToolName]++
	}

	summary := fmt.Sprintf("Session Summary:\n")
	summary += fmt.Sprintf("  Total operations: %d\n", len(hb.events))
	summary += fmt.Sprintf("  Write operations: %d\n", writeCount)
	summary += fmt.Sprintf("  Read operations: %d\n\n", readCount)
	summary += fmt.Sprintf("Tools used:\n")

	for tool, count := range toolCounts {
		summary += fmt.Sprintf("  - %s: %d calls\n", tool, count)
	}

	return summary
}
